package tool

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCalculator(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		want      float64
		wantError bool
	}{
		{name: "add", input: `{"operation":"add","a":2,"b":3}`, want: 5},
		{name: "multiply", input: `{"operation":"multiply","a":4,"b":2.5}`, want: 10},
		{name: "division by zero", input: `{"operation":"divide","a":4,"b":0}`, wantError: true},
		{name: "unknown operation", input: `{"operation":"power","a":2,"b":3}`, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := (Calculator{}).Execute(context.Background(), json.RawMessage(test.input))
			if test.wantError {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := result.(map[string]float64)["result"]; got != test.want {
				t.Fatalf("result = %v; want %v", got, test.want)
			}
		})
	}
}
