package llm

import (
	"context"
	"encoding/json"

	"github.com/sagarbagwe/agentflow/internal/tool"
)

type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content,omitempty"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type Request struct {
	Model        string
	SystemPrompt string
	Messages     []Message
	Tools        []tool.Definition
	Temperature  float64
	MaxTokens    int
}

type Usage struct {
	PromptTokens int64 `json:"prompt_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type Response struct {
	Content   string
	ToolCalls []ToolCall
	Usage     Usage
	Model     string
}

type Provider interface {
	Complete(context.Context, Request) (Response, error)
}
