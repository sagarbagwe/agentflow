package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OpenAICompatible struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewOpenAICompatible(baseURL, apiKey string, client *http.Client) *OpenAICompatible {
	return &OpenAICompatible{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  client,
	}
}

func (p *OpenAICompatible) Complete(ctx context.Context, request Request) (Response, error) {
	messages := make([]map[string]any, 0, len(request.Messages)+1)
	if request.SystemPrompt != "" {
		messages = append(messages, map[string]any{"role": "system", "content": request.SystemPrompt})
	}
	for _, message := range request.Messages {
		item := map[string]any{"role": message.Role, "content": message.Content}
		if message.Name != "" {
			item["name"] = message.Name
		}
		if message.ToolCallID != "" {
			item["tool_call_id"] = message.ToolCallID
		}
		messages = append(messages, item)
	}

	tools := make([]map[string]any, 0, len(request.Tools))
	for _, definition := range request.Tools {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        definition.Name,
				"description": definition.Description,
				"parameters":  definition.Schema,
			},
		})
	}

	body, err := json.Marshal(map[string]any{
		"model":       request.Model,
		"messages":    messages,
		"tools":       tools,
		"temperature": request.Temperature,
		"max_tokens":  request.MaxTokens,
	})
	if err != nil {
		return Response{}, fmt.Errorf("encode LLM request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create LLM request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := p.client.Do(httpRequest)
	if err != nil {
		return Response{}, fmt.Errorf("send LLM request: %w", err)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 4<<20))
	if err != nil {
		return Response{}, fmt.Errorf("read LLM response: %w", err)
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return Response{}, fmt.Errorf("LLM response status %d: %s", httpResponse.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var payload struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return Response{}, fmt.Errorf("decode LLM response: %w", err)
	}
	if len(payload.Choices) == 0 {
		return Response{}, fmt.Errorf("LLM response contains no choices")
	}

	result := Response{
		Content: payload.Choices[0].Message.Content,
		Model:   payload.Model,
		Usage: Usage{
			PromptTokens: payload.Usage.PromptTokens,
			OutputTokens: payload.Usage.CompletionTokens,
		},
	}
	for _, call := range payload.Choices[0].Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{ID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
	}
	return result, nil
}
