package app

import (
	"context"
	"testing"

	"github.com/sagarbagwe/agentflow/internal/queue"
	"github.com/sagarbagwe/agentflow/internal/store"
)

func TestStartExecutionIsIdempotent(t *testing.T) {
	t.Parallel()
	storage := store.NewMemory()
	service := New(storage, queue.NewImmediate(10))
	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, "owner", AgentInput{Name: "assistant", Model: "mock", Temperature: 0.2, MaxTokens: 100})
	if err != nil {
		t.Fatal(err)
	}
	input := ExecutionInput{Input: "hello", IdempotencyKey: "request-1"}
	first, replayed, err := service.StartExecution(ctx, "owner", agent.ID, input)
	if err != nil || replayed {
		t.Fatalf("first request: replayed=%v error=%v", replayed, err)
	}
	second, replayed, err := service.StartExecution(ctx, "owner", agent.ID, input)
	if err != nil || !replayed {
		t.Fatalf("second request: replayed=%v error=%v", replayed, err)
	}
	if first.ID != second.ID {
		t.Fatalf("execution IDs differ: %s and %s", first.ID, second.ID)
	}
}
