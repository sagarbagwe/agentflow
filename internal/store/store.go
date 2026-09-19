package store

import (
	"context"
	"errors"

	"github.com/sagarbagwe/agentflow/internal/domain"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type AgentStore interface {
	CreateAgent(context.Context, domain.Agent) error
	GetAgent(context.Context, string, string) (domain.Agent, error)
	UpdateAgent(context.Context, domain.Agent) error
	DeleteAgent(context.Context, string, string) error
}

type ExecutionStore interface {
	CreateExecution(context.Context, domain.Execution) error
	GetExecution(context.Context, string, string) (domain.Execution, error)
	GetByIdempotencyKey(context.Context, string, string) (domain.Execution, error)
	UpdateExecution(context.Context, domain.Execution) error
	AppendStep(context.Context, domain.Step) error
	Trace(context.Context, string, string) (domain.Trace, error)
	ListAgentExecutions(context.Context, string, string, int) ([]domain.Execution, error)
}

type Store interface {
	AgentStore
	ExecutionStore
}
