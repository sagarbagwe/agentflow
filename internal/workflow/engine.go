package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sagarbagwe/agentflow/internal/domain"
	"github.com/sagarbagwe/agentflow/internal/id"
	"github.com/sagarbagwe/agentflow/internal/llm"
	"github.com/sagarbagwe/agentflow/internal/reliability"
	"github.com/sagarbagwe/agentflow/internal/store"
	"github.com/sagarbagwe/agentflow/internal/tool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type Engine struct {
	provider    llm.Provider
	tools       *tool.Registry
	store       store.ExecutionStore
	events      EventSink
	retryPolicy reliability.RetryPolicy
	breaker     *reliability.CircuitBreaker
	stepTimeout time.Duration
	maxSteps    int
}

func NewEngine(provider llm.Provider, tools *tool.Registry, storage store.ExecutionStore, events EventSink) *Engine {
	return &Engine{
		provider: provider,
		tools:    tools,
		store:    storage,
		events:   events,
		retryPolicy: reliability.RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   200 * time.Millisecond,
			MaxDelay:    2 * time.Second,
		},
		breaker:     reliability.NewCircuitBreaker(5, 30*time.Second),
		stepTimeout: 30 * time.Second,
		maxSteps:    8,
	}
}

func (e *Engine) Execute(ctx context.Context, agent domain.Agent, execution domain.Execution) (domain.Execution, error) {
	ctx, span := otel.Tracer("agentflow/workflow").Start(ctx, "agent.execute")
	span.SetAttributes(attribute.String("agent.id", agent.ID), attribute.String("execution.id", execution.ID), attribute.String("llm.model", agent.Model))
	defer span.End()

	now := time.Now().UTC()
	execution.Status = domain.ExecutionRunning
	execution.StartedAt = &now
	if err := e.store.UpdateExecution(ctx, execution); err != nil {
		return execution, fmt.Errorf("mark execution running: %w", err)
	}
	_ = e.events.Publish(ctx, Event{ExecutionID: execution.ID, Type: "agent.started", Timestamp: now})

	messages := []llm.Message{{Role: "user", Content: execution.Input}}
	for stepNumber := 0; stepNumber < e.maxSteps; stepNumber++ {
		if err := ctx.Err(); err != nil {
			return e.fail(ctx, execution, err)
		}
		if !e.breaker.Allow() {
			return e.fail(ctx, execution, reliability.ErrCircuitOpen)
		}

		stepID, err := id.New()
		if err != nil {
			return e.fail(ctx, execution, err)
		}
		startedAt := time.Now().UTC()
		step := domain.Step{ID: stepID, ExecutionID: execution.ID, Type: "llm", Name: "completion", StartedAt: startedAt}
		_ = e.events.Publish(ctx, Event{ExecutionID: execution.ID, Type: "agent.thinking", StepID: stepID, Timestamp: startedAt})

		stepCtx, cancel := context.WithTimeout(ctx, e.stepTimeout)
		stepCtx, llmSpan := otel.Tracer("agentflow/workflow").Start(stepCtx, "llm.complete")
		llmSpan.SetAttributes(attribute.String("llm.model", agent.Model), attribute.String("execution.id", execution.ID))
		response, retries, callErr := reliability.Retry(stepCtx, e.retryPolicy, func(callCtx context.Context) (llm.Response, error) {
			return e.provider.Complete(callCtx, llm.Request{
				Model:        agent.Model,
				SystemPrompt: agent.SystemPrompt,
				Messages:     messages,
				Tools:        e.tools.Definitions(agent.Tools),
				Temperature:  agent.Temperature,
				MaxTokens:    agent.MaxTokens,
			})
		})
		if callErr != nil {
			llmSpan.RecordError(callErr)
			llmSpan.SetStatus(codes.Error, callErr.Error())
		} else {
			llmSpan.SetStatus(codes.Ok, "completed")
		}
		llmSpan.End()
		cancel()
		step.RetryCount = retries
		execution.RetryCount += retries
		completedAt := time.Now().UTC()
		step.CompletedAt = &completedAt
		if callErr != nil {
			e.breaker.Failure()
			step.Error = callErr.Error()
			_ = e.store.AppendStep(ctx, step)
			return e.fail(ctx, execution, callErr)
		}
		e.breaker.Success()
		execution.Model = response.Model
		execution.PromptTokens += response.Usage.PromptTokens
		execution.OutputTokens += response.Usage.OutputTokens
		step.Output = map[string]any{"content": response.Content, "tool_call_count": len(response.ToolCalls)}
		if err := e.store.AppendStep(ctx, step); err != nil {
			return e.fail(ctx, execution, fmt.Errorf("store LLM step: %w", err))
		}

		if len(response.ToolCalls) == 0 {
			execution.Output = response.Content
			return e.succeed(ctx, execution)
		}

		messages = append(messages, llm.Message{Role: "assistant", Content: response.Content})
		toolMessages, err := e.executeTools(ctx, execution.ID, response.ToolCalls)
		if err != nil {
			return e.fail(ctx, execution, err)
		}
		messages = append(messages, toolMessages...)
	}
	return e.fail(ctx, execution, errors.New("workflow exceeded maximum steps"))
}

func (e *Engine) executeTools(ctx context.Context, executionID string, calls []llm.ToolCall) ([]llm.Message, error) {
	messages := make([]llm.Message, len(calls))
	errorsByIndex := make([]error, len(calls))
	var wait sync.WaitGroup

	for index, call := range calls {
		index, call := index, call
		wait.Add(1)
		go func() {
			defer wait.Done()
			messages[index], errorsByIndex[index] = e.executeTool(ctx, executionID, call)
		}()
	}
	wait.Wait()

	for _, err := range errorsByIndex {
		if err != nil {
			return nil, err
		}
	}
	return messages, nil
}

func (e *Engine) executeTool(ctx context.Context, executionID string, call llm.ToolCall) (llm.Message, error) {
	ctx, span := otel.Tracer("agentflow/workflow").Start(ctx, "tool.execute")
	span.SetAttributes(attribute.String("tool.name", call.Name), attribute.String("execution.id", executionID))
	defer span.End()

	item, ok := e.tools.Get(call.Name)
	if !ok {
		return llm.Message{}, fmt.Errorf("tool %q is not registered", call.Name)
	}
	stepID, err := id.New()
	if err != nil {
		return llm.Message{}, err
	}
	startedAt := time.Now().UTC()
	step := domain.Step{ID: stepID, ExecutionID: executionID, Type: "tool", Name: call.Name, Input: map[string]any{"arguments": json.RawMessage(call.Arguments)}, StartedAt: startedAt}
	_ = e.events.Publish(ctx, Event{ExecutionID: executionID, Type: "tool.started", StepID: stepID, Data: map[string]any{"tool": call.Name}, Timestamp: startedAt})

	toolCtx, cancel := context.WithTimeout(ctx, e.stepTimeout)
	output, callErr := item.Execute(toolCtx, call.Arguments)
	cancel()
	completedAt := time.Now().UTC()
	step.CompletedAt = &completedAt
	if callErr != nil {
		step.Error = callErr.Error()
		_ = e.store.AppendStep(ctx, step)
		_ = e.events.Publish(ctx, Event{ExecutionID: executionID, Type: "tool.failed", StepID: stepID, Data: map[string]any{"error": callErr.Error()}, Timestamp: completedAt})
		return llm.Message{}, fmt.Errorf("execute tool %s: %w", call.Name, callErr)
	}

	encoded, err := json.Marshal(output)
	if err != nil {
		return llm.Message{}, fmt.Errorf("encode tool %s output: %w", call.Name, err)
	}
	step.Output = map[string]any{"result": output}
	if err := e.store.AppendStep(ctx, step); err != nil {
		return llm.Message{}, fmt.Errorf("store tool step: %w", err)
	}
	_ = e.events.Publish(ctx, Event{ExecutionID: executionID, Type: "tool.completed", StepID: stepID, Data: map[string]any{"tool": call.Name}, Timestamp: completedAt})
	return llm.Message{Role: "tool", Name: call.Name, ToolCallID: call.ID, Content: string(encoded)}, nil
}

func (e *Engine) succeed(ctx context.Context, execution domain.Execution) (domain.Execution, error) {
	oteltrace.SpanFromContext(ctx).SetStatus(codes.Ok, "completed")
	now := time.Now().UTC()
	execution.Status = domain.ExecutionSucceeded
	execution.CompletedAt = &now
	if err := e.store.UpdateExecution(ctx, execution); err != nil {
		return execution, err
	}
	_ = e.events.Publish(ctx, Event{ExecutionID: execution.ID, Type: "agent.completed", Data: map[string]any{"output": execution.Output}, Timestamp: now})
	return execution, nil
}

func (e *Engine) fail(ctx context.Context, execution domain.Execution, cause error) (domain.Execution, error) {
	span := oteltrace.SpanFromContext(ctx)
	span.RecordError(cause)
	span.SetStatus(codes.Error, cause.Error())
	now := time.Now().UTC()
	execution.Status = domain.ExecutionFailed
	if errors.Is(cause, context.Canceled) {
		execution.Status = domain.ExecutionCancelled
	}
	execution.Error = cause.Error()
	execution.CompletedAt = &now
	if err := e.store.UpdateExecution(ctx, execution); err != nil {
		return execution, errors.Join(cause, err)
	}
	_ = e.events.Publish(context.Background(), Event{ExecutionID: execution.ID, Type: "agent.failed", Data: map[string]any{"error": cause.Error()}, Timestamp: now})
	return execution, cause
}
