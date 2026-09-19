package store

import (
	"context"
	"sync"

	"github.com/sagarbagwe/agentflow/internal/domain"
)

type Memory struct {
	mu            sync.RWMutex
	agents        map[string]domain.Agent
	executions    map[string]domain.Execution
	steps         map[string][]domain.Step
	conversations map[string]domain.Conversation
	messages      map[string][]domain.Message
}

func NewMemory() *Memory {
	return &Memory{
		agents:        make(map[string]domain.Agent),
		executions:    make(map[string]domain.Execution),
		steps:         make(map[string][]domain.Step),
		conversations: make(map[string]domain.Conversation),
		messages:      make(map[string][]domain.Message),
	}
}

func (m *Memory) CreateAgent(_ context.Context, agent domain.Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.agents[agent.ID]; exists {
		return ErrConflict
	}
	m.agents[agent.ID] = agent
	return nil
}

func (m *Memory) GetAgent(_ context.Context, ownerID, agentID string) (domain.Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agent, ok := m.agents[agentID]
	if !ok || agent.OwnerID != ownerID {
		return domain.Agent{}, ErrNotFound
	}
	return agent, nil
}

func (m *Memory) UpdateAgent(_ context.Context, agent domain.Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.agents[agent.ID]
	if !ok || current.OwnerID != agent.OwnerID {
		return ErrNotFound
	}
	m.agents[agent.ID] = agent
	return nil
}

func (m *Memory) DeleteAgent(_ context.Context, ownerID, agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	agent, ok := m.agents[agentID]
	if !ok || agent.OwnerID != ownerID {
		return ErrNotFound
	}
	delete(m.agents, agentID)
	return nil
}

func (m *Memory) CreateConversation(_ context.Context, conversation domain.Conversation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.conversations[conversation.ID]; exists {
		return ErrConflict
	}
	m.conversations[conversation.ID] = conversation
	return nil
}

func (m *Memory) GetConversation(_ context.Context, ownerID, conversationID string) (domain.Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conversation, ok := m.conversations[conversationID]
	if !ok || conversation.OwnerID != ownerID {
		return domain.Conversation{}, ErrNotFound
	}
	return conversation, nil
}

func (m *Memory) AppendMessage(_ context.Context, message domain.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.conversations[message.ConversationID]; !ok {
		return ErrNotFound
	}
	m.messages[message.ConversationID] = append(m.messages[message.ConversationID], message)
	return nil
}

func (m *Memory) ListMessages(_ context.Context, ownerID, conversationID string, limit int) ([]domain.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conversation, ok := m.conversations[conversationID]
	if !ok || conversation.OwnerID != ownerID {
		return nil, ErrNotFound
	}
	messages := m.messages[conversationID]
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	return append([]domain.Message(nil), messages...), nil
}

func (m *Memory) CreateExecution(_ context.Context, execution domain.Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.executions[execution.ID]; exists {
		return ErrConflict
	}
	m.executions[execution.ID] = execution
	return nil
}

func (m *Memory) GetExecution(_ context.Context, ownerID, executionID string) (domain.Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	execution, ok := m.executions[executionID]
	if !ok || execution.OwnerID != ownerID {
		return domain.Execution{}, ErrNotFound
	}
	return execution, nil
}

func (m *Memory) GetByIdempotencyKey(_ context.Context, ownerID, key string) (domain.Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, execution := range m.executions {
		if execution.OwnerID == ownerID && execution.IdempotencyKey == key {
			return execution, nil
		}
	}
	return domain.Execution{}, ErrNotFound
}

func (m *Memory) UpdateExecution(_ context.Context, execution domain.Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.executions[execution.ID]; !ok {
		return ErrNotFound
	}
	m.executions[execution.ID] = execution
	return nil
}

func (m *Memory) AppendStep(_ context.Context, step domain.Step) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.executions[step.ExecutionID]; !ok {
		return ErrNotFound
	}
	m.steps[step.ExecutionID] = append(m.steps[step.ExecutionID], step)
	return nil
}

func (m *Memory) Trace(ctx context.Context, ownerID, executionID string) (domain.Trace, error) {
	execution, err := m.GetExecution(ctx, ownerID, executionID)
	if err != nil {
		return domain.Trace{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	steps := append([]domain.Step(nil), m.steps[executionID]...)
	return domain.Trace{Execution: execution, Steps: steps}, nil
}

func (m *Memory) ListAgentExecutions(_ context.Context, ownerID, agentID string, limit int) ([]domain.Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Execution, 0)
	for _, execution := range m.executions {
		if execution.OwnerID == ownerID && execution.AgentID == agentID {
			result = append(result, execution)
			if len(result) == limit {
				break
			}
		}
	}
	return result, nil
}
