package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	queuepkg "github.com/sagarbagwe/agentflow/internal/queue"
)

func TestRedisQueueRoundTrip(t *testing.T) {
	address := os.Getenv("INTEGRATION_REDIS_ADDR")
	if address == "" {
		t.Skip("INTEGRATION_REDIS_ADDR is not set")
	}
	client := redis.NewClient(&redis.Options{Addr: address})
	defer client.Close()
	key := "agentflow:test:queue"
	defer client.Del(context.Background(), key)
	jobQueue := queuepkg.NewRedis(client, key)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	want := queuepkg.Job{ExecutionID: "execution", AgentID: "agent", OwnerID: "owner"}
	if err := jobQueue.Enqueue(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := jobQueue.Dequeue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("job = %#v; want %#v", got, want)
	}
}
