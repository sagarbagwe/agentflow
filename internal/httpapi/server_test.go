package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sagarbagwe/agentflow/internal/config"
)

func TestHealthHandler(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := New(config.HTTPConfig{}, logger)

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d; want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q; want %q", got, "application/json")
	}
	if got := response.Body.String(); got != `{"status":"ok"}` {
		t.Fatalf("body = %q; want %q", got, `{"status":"ok"}`)
	}
}

func TestHealthHandlerRejectsUnsupportedMethod(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := New(config.HTTPConfig{}, logger)

	request := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d; want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
