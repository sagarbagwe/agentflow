package workflow

import (
	"context"
	"time"
)

type Event struct {
	ExecutionID string         `json:"execution_id"`
	Type        string         `json:"type"`
	StepID      string         `json:"step_id,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
	Timestamp   time.Time      `json:"timestamp"`
}

type EventSink interface {
	Publish(context.Context, Event) error
}

type NopEventSink struct{}

func (NopEventSink) Publish(context.Context, Event) error { return nil }
