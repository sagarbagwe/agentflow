package llm

import (
	"context"
	"fmt"
)

type Mock struct{}

func (Mock) Complete(ctx context.Context, request Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	if len(request.Messages) == 0 {
		return Response{}, fmt.Errorf("at least one message is required")
	}
	last := request.Messages[len(request.Messages)-1]
	return Response{
		Content: "Mock response: " + last.Content,
		Model:   request.Model,
		Usage:   Usage{PromptTokens: int64(len(last.Content) / 4), OutputTokens: int64(len(last.Content)/4 + 3)},
	}, nil
}
