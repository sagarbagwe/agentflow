package realtime

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/sagarbagwe/agentflow/internal/workflow"
)

type RedisBroker struct {
	client *redis.Client
	prefix string
}

func NewRedisBroker(client *redis.Client, prefix string) *RedisBroker {
	return &RedisBroker{client: client, prefix: prefix}
}

func (b *RedisBroker) Publish(ctx context.Context, event workflow.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return b.client.Publish(ctx, b.prefix+event.ExecutionID, payload).Err()
}

func (b *RedisBroker) Subscribe(executionID string) (<-chan workflow.Event, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	pubsub := b.client.Subscribe(ctx, b.prefix+executionID)
	events := make(chan workflow.Event, 32)
	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			cancel()
			_ = pubsub.Close()
		})
	}
	go func() {
		defer close(events)
		defer unsubscribe()
		channel := pubsub.Channel()
		for {
			select {
			case message, ok := <-channel:
				if !ok {
					return
				}
				var event workflow.Event
				if json.Unmarshal([]byte(message.Payload), &event) != nil {
					continue
				}
				select {
				case events <- event:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return events, unsubscribe
}
