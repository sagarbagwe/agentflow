package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sagarbagwe/agentflow/internal/domain"
	"github.com/sagarbagwe/agentflow/internal/store"
)

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) CreateAgent(ctx context.Context, agent domain.Agent) error {
	tools, err := json.Marshal(agent.Tools)
	if err != nil {
		return fmt.Errorf("encode tools: %w", err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create agent: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `INSERT INTO agents (id, owner_id, name, description, current_version, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, agent.ID, agent.OwnerID, agent.Name, agent.Description, agent.Version, agent.CreatedAt, agent.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert agent: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO agent_versions (agent_id, version, system_prompt, model, temperature, max_tokens, tool_names) VALUES ($1,$2,$3,$4,$5,$6,$7)`, agent.ID, agent.Version, agent.SystemPrompt, agent.Model, agent.Temperature, agent.MaxTokens, tools)
	if err != nil {
		return fmt.Errorf("insert agent version: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create agent: %w", err)
	}
	return nil
}

func (s *Store) GetAgent(ctx context.Context, ownerID, agentID string) (domain.Agent, error) {
	var agent domain.Agent
	var tools []byte
	err := s.pool.QueryRow(ctx, `
SELECT a.id, a.owner_id, a.name, a.description, v.system_prompt, v.model, v.temperature, v.max_tokens, v.tool_names, a.current_version, a.created_at, a.updated_at
FROM agents a JOIN agent_versions v ON v.agent_id = a.id AND v.version = a.current_version
WHERE a.owner_id = $1 AND a.id = $2`, ownerID, agentID).Scan(&agent.ID, &agent.OwnerID, &agent.Name, &agent.Description, &agent.SystemPrompt, &agent.Model, &agent.Temperature, &agent.MaxTokens, &tools, &agent.Version, &agent.CreatedAt, &agent.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Agent{}, store.ErrNotFound
	}
	if err != nil {
		return domain.Agent{}, fmt.Errorf("get agent: %w", err)
	}
	if err := json.Unmarshal(tools, &agent.Tools); err != nil {
		return domain.Agent{}, fmt.Errorf("decode tools: %w", err)
	}
	return agent, nil
}

func (s *Store) UpdateAgent(ctx context.Context, agent domain.Agent) error {
	tools, err := json.Marshal(agent.Tools)
	if err != nil {
		return fmt.Errorf("encode tools: %w", err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := tx.Exec(ctx, `UPDATE agents SET name=$3, description=$4, current_version=$5, updated_at=$6 WHERE owner_id=$1 AND id=$2`, agent.OwnerID, agent.ID, agent.Name, agent.Description, agent.Version, agent.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update agent: %w", err)
	}
	if result.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	_, err = tx.Exec(ctx, `INSERT INTO agent_versions (agent_id, version, system_prompt, model, temperature, max_tokens, tool_names) VALUES ($1,$2,$3,$4,$5,$6,$7)`, agent.ID, agent.Version, agent.SystemPrompt, agent.Model, agent.Temperature, agent.MaxTokens, tools)
	if err != nil {
		return fmt.Errorf("insert agent version: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *Store) DeleteAgent(ctx context.Context, ownerID, agentID string) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM agents WHERE owner_id=$1 AND id=$2`, ownerID, agentID)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	if result.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) CreateExecution(ctx context.Context, execution domain.Execution) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO executions (id, request_id, idempotency_key, agent_id, agent_version, owner_id, conversation_id, input, output, status, model, prompt_tokens, output_tokens, retry_count, error, started_at, completed_at, created_at)
VALUES ($1,$2,NULLIF($3,''),$4,(SELECT current_version FROM agents WHERE id=$4),$5,NULLIF($6,'')::uuid,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`, execution.ID, execution.RequestID, execution.IdempotencyKey, execution.AgentID, execution.OwnerID, execution.ConversationID, execution.Input, execution.Output, execution.Status, execution.Model, execution.PromptTokens, execution.OutputTokens, execution.RetryCount, execution.Error, execution.StartedAt, execution.CompletedAt, execution.CreatedAt)
	if err != nil {
		return fmt.Errorf("create execution: %w", err)
	}
	return nil
}

func (s *Store) GetExecution(ctx context.Context, ownerID, executionID string) (domain.Execution, error) {
	return s.getExecution(ctx, `WHERE owner_id=$1 AND id=$2`, ownerID, executionID)
}

func (s *Store) GetByIdempotencyKey(ctx context.Context, ownerID, key string) (domain.Execution, error) {
	return s.getExecution(ctx, `WHERE owner_id=$1 AND idempotency_key=$2`, ownerID, key)
}

func (s *Store) getExecution(ctx context.Context, where string, first, second any) (domain.Execution, error) {
	var execution domain.Execution
	err := s.pool.QueryRow(ctx, `SELECT id, request_id, COALESCE(idempotency_key,''), agent_id, owner_id, COALESCE(conversation_id::text,''), input, output, status, model, prompt_tokens, output_tokens, error, retry_count, started_at, completed_at, created_at FROM executions `+where, first, second).Scan(&execution.ID, &execution.RequestID, &execution.IdempotencyKey, &execution.AgentID, &execution.OwnerID, &execution.ConversationID, &execution.Input, &execution.Output, &execution.Status, &execution.Model, &execution.PromptTokens, &execution.OutputTokens, &execution.Error, &execution.RetryCount, &execution.StartedAt, &execution.CompletedAt, &execution.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Execution{}, store.ErrNotFound
	}
	if err != nil {
		return domain.Execution{}, fmt.Errorf("get execution: %w", err)
	}
	return execution, nil
}

func (s *Store) UpdateExecution(ctx context.Context, execution domain.Execution) error {
	result, err := s.pool.Exec(ctx, `UPDATE executions SET output=$2,status=$3,model=$4,prompt_tokens=$5,output_tokens=$6,error=$7,retry_count=$8,started_at=$9,completed_at=$10 WHERE id=$1`, execution.ID, execution.Output, execution.Status, execution.Model, execution.PromptTokens, execution.OutputTokens, execution.Error, execution.RetryCount, execution.StartedAt, execution.CompletedAt)
	if err != nil {
		return fmt.Errorf("update execution: %w", err)
	}
	if result.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) AppendStep(ctx context.Context, step domain.Step) error {
	input, err := json.Marshal(step.Input)
	if err != nil {
		return err
	}
	output, err := json.Marshal(step.Output)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO execution_steps (id,execution_id,step_type,name,input,output,error,retry_count,started_at,completed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, step.ID, step.ExecutionID, step.Type, step.Name, input, output, step.Error, step.RetryCount, step.StartedAt, step.CompletedAt)
	return err
}

func (s *Store) Trace(ctx context.Context, ownerID, executionID string) (domain.Trace, error) {
	execution, err := s.GetExecution(ctx, ownerID, executionID)
	if err != nil {
		return domain.Trace{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,execution_id,step_type,name,input,output,error,retry_count,started_at,completed_at FROM execution_steps WHERE execution_id=$1 ORDER BY started_at`, executionID)
	if err != nil {
		return domain.Trace{}, err
	}
	defer rows.Close()
	trace := domain.Trace{Execution: execution}
	for rows.Next() {
		var step domain.Step
		var input, output []byte
		if err := rows.Scan(&step.ID, &step.ExecutionID, &step.Type, &step.Name, &input, &output, &step.Error, &step.RetryCount, &step.StartedAt, &step.CompletedAt); err != nil {
			return domain.Trace{}, err
		}
		_ = json.Unmarshal(input, &step.Input)
		_ = json.Unmarshal(output, &step.Output)
		trace.Steps = append(trace.Steps, step)
	}
	return trace, rows.Err()
}

func (s *Store) ListAgentExecutions(ctx context.Context, ownerID, agentID string, limit int) ([]domain.Execution, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, request_id, COALESCE(idempotency_key,''), agent_id, owner_id, COALESCE(conversation_id::text,''), input, output, status, model, prompt_tokens, output_tokens, error, retry_count, started_at, completed_at, created_at FROM executions WHERE owner_id=$1 AND agent_id=$2 ORDER BY created_at DESC LIMIT $3`, ownerID, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var executions []domain.Execution
	for rows.Next() {
		var execution domain.Execution
		if err := rows.Scan(&execution.ID, &execution.RequestID, &execution.IdempotencyKey, &execution.AgentID, &execution.OwnerID, &execution.ConversationID, &execution.Input, &execution.Output, &execution.Status, &execution.Model, &execution.PromptTokens, &execution.OutputTokens, &execution.Error, &execution.RetryCount, &execution.StartedAt, &execution.CompletedAt, &execution.CreatedAt); err != nil {
			return nil, err
		}
		executions = append(executions, execution)
	}
	return executions, rows.Err()
}
