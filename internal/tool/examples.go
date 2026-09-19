package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

type Calculator struct{}

func (Calculator) Definition() Definition {
	return Definition{
		Name:        "calculator",
		Description: "Perform a basic arithmetic operation.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"operation": map[string]any{"type": "string", "enum": []string{"add", "subtract", "multiply", "divide"}},
				"a":         map[string]any{"type": "number"},
				"b":         map[string]any{"type": "number"},
			},
			"required": []string{"operation", "a", "b"},
		},
	}
}

func (Calculator) Execute(_ context.Context, input json.RawMessage) (any, error) {
	var request struct {
		Operation string  `json:"operation"`
		A         float64 `json:"a"`
		B         float64 `json:"b"`
	}
	if err := json.Unmarshal(input, &request); err != nil {
		return nil, fmt.Errorf("decode calculator input: %w", err)
	}

	var result float64
	switch request.Operation {
	case "add":
		result = request.A + request.B
	case "subtract":
		result = request.A - request.B
	case "multiply":
		result = request.A * request.B
	case "divide":
		if request.B == 0 {
			return nil, errors.New("division by zero")
		}
		result = request.A / request.B
	default:
		return nil, fmt.Errorf("unsupported operation %q", request.Operation)
	}
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return nil, errors.New("result is not finite")
	}
	return map[string]float64{"result": result}, nil
}

type MockLookup struct {
	Name        string
	Description string
	Results     map[string]string
}

func (m MockLookup) Definition() Definition {
	return Definition{
		Name:        m.Name,
		Description: m.Description,
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"query": map[string]any{"type": "string"}},
			"required":   []string{"query"},
		},
	}
}

func (m MockLookup) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var request struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(input, &request); err != nil {
		return nil, fmt.Errorf("decode %s input: %w", m.Name, err)
	}
	query := strings.TrimSpace(strings.ToLower(request.Query))
	if query == "" {
		return nil, errors.New("query is required")
	}
	if result, ok := m.Results[query]; ok {
		return map[string]string{"result": result, "source": "mock"}, nil
	}
	return map[string]string{"result": "No matching mock result", "source": "mock"}, nil
}

func DefaultTools() []Tool {
	return []Tool{
		Calculator{},
		MockLookup{Name: "web_search", Description: "Search a deterministic local web index.", Results: map[string]string{"agent observability": "Tracing, metrics, logs, evaluations, and cost attribution for agent runs."}},
		MockLookup{Name: "weather", Description: "Get mock weather data.", Results: map[string]string{"mumbai": "29C, partly cloudy"}},
		MockLookup{Name: "database_query", Description: "Query an approved read-only mock dataset.", Results: map[string]string{"active agents": "12"}},
		MockLookup{Name: "company_api", Description: "Call an internal company API mock.", Results: map[string]string{"service status": "all systems operational"}},
	}
}
