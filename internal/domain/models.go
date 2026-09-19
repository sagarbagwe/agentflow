package domain

import "time"

type Agent struct {
	ID           string         `json:"id"`
	OwnerID      string         `json:"owner_id"`
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	SystemPrompt string         `json:"system_prompt"`
	Model        string         `json:"model"`
	Temperature  float64        `json:"temperature"`
	MaxTokens    int            `json:"max_tokens"`
	Tools        []string       `json:"tools"`
	Version      int            `json:"version"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type ExecutionStatus string

const (
	ExecutionQueued    ExecutionStatus = "queued"
	ExecutionRunning   ExecutionStatus = "running"
	ExecutionSucceeded ExecutionStatus = "succeeded"
	ExecutionFailed    ExecutionStatus = "failed"
	ExecutionCancelled ExecutionStatus = "cancelled"
)

type Execution struct {
	ID             string          `json:"id"`
	RequestID      string          `json:"request_id"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	AgentID        string          `json:"agent_id"`
	OwnerID        string          `json:"owner_id"`
	ConversationID string          `json:"conversation_id,omitempty"`
	Input          string          `json:"input"`
	Output         string          `json:"output,omitempty"`
	Status         ExecutionStatus `json:"status"`
	Model          string          `json:"model"`
	PromptTokens   int64           `json:"prompt_tokens"`
	OutputTokens   int64           `json:"output_tokens"`
	Error          string          `json:"error,omitempty"`
	RetryCount     int             `json:"retry_count"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type Step struct {
	ID          string         `json:"id"`
	ExecutionID string         `json:"execution_id"`
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Input       map[string]any `json:"input,omitempty"`
	Output      map[string]any `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	RetryCount  int            `json:"retry_count"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

type Trace struct {
	Execution Execution `json:"execution"`
	Steps     []Step     `json:"steps"`
}
