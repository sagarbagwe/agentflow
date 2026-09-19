package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/sagarbagwe/agentflow/internal/queue"
	"github.com/sagarbagwe/agentflow/internal/store"
	"github.com/sagarbagwe/agentflow/internal/workflow"
)

type Worker struct {
	queue  queue.Queue
	store  store.Store
	engine *workflow.Engine
	logger *slog.Logger
}

func New(jobQueue queue.Queue, storage store.Store, engine *workflow.Engine, logger *slog.Logger) *Worker {
	return &Worker{queue: jobQueue, store: storage, engine: engine, logger: logger}
}

func (w *Worker) Run(ctx context.Context, concurrency int) error {
	if concurrency < 1 {
		concurrency = 1
	}
	var wait sync.WaitGroup
	for index := 0; index < concurrency; index++ {
		wait.Add(1)
		go func(workerID int) {
			defer wait.Done()
			w.consume(ctx, workerID)
		}(index + 1)
	}
	<-ctx.Done()
	wait.Wait()
	return ctx.Err()
}

func (w *Worker) consume(ctx context.Context, workerID int) {
	logger := w.logger.With("worker_id", workerID)
	for {
		job, err := w.queue.Dequeue(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logger.Error("failed to dequeue job", "error", err)
			continue
		}

		execution, err := w.store.GetExecution(ctx, job.OwnerID, job.ExecutionID)
		if err != nil {
			logger.Error("failed to load execution", "execution_id", job.ExecutionID, "error", err)
			continue
		}
		agent, err := w.store.GetAgent(ctx, job.OwnerID, job.AgentID)
		if err != nil {
			logger.Error("failed to load agent", "agent_id", job.AgentID, "error", err)
			continue
		}

		logger.Info("executing agent job", "execution_id", job.ExecutionID, "agent_id", job.AgentID)
		if _, err := w.engine.Execute(ctx, agent, execution); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("agent execution failed", "execution_id", job.ExecutionID, "error", err)
		}
	}
}
