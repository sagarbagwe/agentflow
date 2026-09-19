package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sagarbagwe/agentflow/internal/domain"
	"github.com/sagarbagwe/agentflow/internal/id"
	"github.com/sagarbagwe/agentflow/internal/queue"
	"github.com/sagarbagwe/agentflow/internal/store"
)

type Service struct {
	store store.Store
	queue queue.Queue
}

func New(storage store.Store, jobQueue queue.Queue) *Service {
	return &Service{store: storage, queue: jobQueue}
}

type AgentInput struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	SystemPrompt string   `json:"system_prompt"`
	Model        string   `json:"model"`
	Temperature  float64  `json:"temperature"`
	MaxTokens    int      `json:"max_tokens"`
	Tools        []string `json:"tools"`
}

func (s *Service) CreateAgent(ctx context.Context, ownerID string, input AgentInput) (domain.Agent, error) {
	if err := validateAgent(input); err != nil {
		return domain.Agent{}, err
	}
	agentID, err := id.New()
	if err != nil {
		return domain.Agent{}, err
	}
	now := time.Now().UTC()
	agent := domain.Agent{
		ID:           agentID,
		OwnerID:      ownerID,
		Name:         strings.TrimSpace(input.Name),
		Description:  strings.TrimSpace(input.Description),
		SystemPrompt: input.SystemPrompt,
		Model:        input.Model,
		Temperature:  input.Temperature,
		MaxTokens:    input.MaxTokens,
		Tools:        input.Tools,
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.store.CreateAgent(ctx, agent); err != nil {
		return domain.Agent{}, fmt.Errorf("create agent: %w", err)
	}
	return agent, nil
}

func (s *Service) GetAgent(ctx context.Context, ownerID, agentID string) (domain.Agent, error) {
	return s.store.GetAgent(ctx, ownerID, agentID)
}

func (s *Service) UpdateAgent(ctx context.Context, ownerID, agentID string, input AgentInput) (domain.Agent, error) {
	if err := validateAgent(input); err != nil {
		return domain.Agent{}, err
	}
	agent, err := s.store.GetAgent(ctx, ownerID, agentID)
	if err != nil {
		return domain.Agent{}, err
	}
	agent.Name = strings.TrimSpace(input.Name)
	agent.Description = strings.TrimSpace(input.Description)
	agent.SystemPrompt = input.SystemPrompt
	agent.Model = input.Model
	agent.Temperature = input.Temperature
	agent.MaxTokens = input.MaxTokens
	agent.Tools = input.Tools
	agent.Version++
	agent.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateAgent(ctx, agent); err != nil {
		return domain.Agent{}, err
	}
	return agent, nil
}

func (s *Service) DeleteAgent(ctx context.Context, ownerID, agentID string) error {
	return s.store.DeleteAgent(ctx, ownerID, agentID)
}

type ExecutionInput struct {
	Input          string `json:"input"`
	ConversationID string `json:"conversation_id"`
	IdempotencyKey string `json:"-"`
	RequestID      string `json:"-"`
}

func (s *Service) StartExecution(ctx context.Context, ownerID, agentID string, input ExecutionInput) (domain.Execution, bool, error) {
	if strings.TrimSpace(input.Input) == "" {
		return domain.Execution{}, false, fmt.Errorf("input is required")
	}
	if input.IdempotencyKey != "" {
		existing, err := s.store.GetByIdempotencyKey(ctx, ownerID, input.IdempotencyKey)
		if err == nil {
			return existing, true, nil
		}
		if !errors.Is(err, store.ErrNotFound) {
			return domain.Execution{}, false, err
		}
	}
	agent, err := s.store.GetAgent(ctx, ownerID, agentID)
	if err != nil {
		return domain.Execution{}, false, err
	}
	conversationID := input.ConversationID
	if conversationID == "" {
		conversationID, err = id.New()
		if err != nil {
			return domain.Execution{}, false, err
		}
		now := time.Now().UTC()
		conversation := domain.Conversation{ID: conversationID, OwnerID: ownerID, AgentID: agentID, Title: truncate(input.Input, 80), CreatedAt: now, UpdatedAt: now}
		if err := s.store.CreateConversation(ctx, conversation); err != nil {
			return domain.Execution{}, false, fmt.Errorf("create conversation: %w", err)
		}
	} else {
		conversation, err := s.store.GetConversation(ctx, ownerID, conversationID)
		if err != nil {
			return domain.Execution{}, false, err
		}
		if conversation.AgentID != agentID {
			return domain.Execution{}, false, fmt.Errorf("conversation belongs to another agent")
		}
	}
	executionID, err := id.New()
	if err != nil {
		return domain.Execution{}, false, err
	}
	requestID := input.RequestID
	if requestID == "" {
		requestID, err = id.New()
		if err != nil {
			return domain.Execution{}, false, err
		}
	}
	execution := domain.Execution{
		ID:             executionID,
		RequestID:      requestID,
		IdempotencyKey: input.IdempotencyKey,
		AgentID:        agent.ID,
		OwnerID:        ownerID,
		ConversationID: conversationID,
		Input:          input.Input,
		Status:         domain.ExecutionQueued,
		Model:          agent.Model,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.store.CreateExecution(ctx, execution); err != nil {
		return domain.Execution{}, false, err
	}
	if err := s.queue.Enqueue(ctx, queue.Job{ExecutionID: execution.ID, AgentID: agent.ID, OwnerID: ownerID}); err != nil {
		execution.Status = domain.ExecutionFailed
		execution.Error = "dispatch execution: " + err.Error()
		now := time.Now().UTC()
		execution.CompletedAt = &now
		_ = s.store.UpdateExecution(ctx, execution)
		return execution, false, fmt.Errorf("dispatch execution: %w", err)
	}
	return execution, false, nil
}

func (s *Service) GetExecution(ctx context.Context, ownerID, executionID string) (domain.Execution, error) {
	return s.store.GetExecution(ctx, ownerID, executionID)
}

func (s *Service) GetTrace(ctx context.Context, ownerID, executionID string) (domain.Trace, error) {
	return s.store.Trace(ctx, ownerID, executionID)
}

func (s *Service) ListExecutions(ctx context.Context, ownerID, agentID string, limit int) ([]domain.Execution, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.store.ListAgentExecutions(ctx, ownerID, agentID, limit)
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func validateAgent(input AgentInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(input.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if input.Temperature < 0 || input.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}
	if input.MaxTokens < 1 || input.MaxTokens > 1_000_000 {
		return fmt.Errorf("max_tokens must be between 1 and 1000000")
	}
	return nil
}
