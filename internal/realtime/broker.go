package realtime

import (
	"context"
	"sync"

	"github.com/sagarbagwe/agentflow/internal/workflow"
)

type Stream interface {
	workflow.EventSink
	Subscribe(string) (<-chan workflow.Event, func())
}

type Broker struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan workflow.Event]struct{}
}

func NewBroker() *Broker {
	return &Broker{subscribers: make(map[string]map[chan workflow.Event]struct{})}
}

func (b *Broker) Publish(_ context.Context, event workflow.Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for subscriber := range b.subscribers[event.ExecutionID] {
		select {
		case subscriber <- event:
		default:
		}
	}
	return nil
}

func (b *Broker) Subscribe(executionID string) (<-chan workflow.Event, func()) {
	channel := make(chan workflow.Event, 32)
	b.mu.Lock()
	if b.subscribers[executionID] == nil {
		b.subscribers[executionID] = make(map[chan workflow.Event]struct{})
	}
	b.subscribers[executionID][channel] = struct{}{}
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if subscribers := b.subscribers[executionID]; subscribers != nil {
			delete(subscribers, channel)
			close(channel)
			if len(subscribers) == 0 {
				delete(b.subscribers, executionID)
			}
		}
	}
	return channel, unsubscribe
}
