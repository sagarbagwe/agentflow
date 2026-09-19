package queue

import (
	"context"

	"github.com/sagarbagwe/agentflow/internal/domain"
)

type Job struct {
	ExecutionID string `json:"execution_id"`
	AgentID     string `json:"agent_id"`
	OwnerID     string `json:"owner_id"`
}

type Queue interface {
	Enqueue(context.Context, Job) error
	Dequeue(context.Context) (Job, error)
}

type Immediate struct {
	jobs chan Job
}

func NewImmediate(size int) *Immediate {
	return &Immediate{jobs: make(chan Job, size)}
}

func (q *Immediate) Enqueue(ctx context.Context, job Job) error {
	select {
	case q.jobs <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *Immediate) Dequeue(ctx context.Context) (Job, error) {
	select {
	case job := <-q.jobs:
		return job, nil
	case <-ctx.Done():
		return Job{}, ctx.Err()
	}
}

var _ = domain.Execution{}
