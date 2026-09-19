package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sagarbagwe/agentflow/internal/app"
	"github.com/sagarbagwe/agentflow/internal/auth"
	"github.com/sagarbagwe/agentflow/internal/config"
	"github.com/sagarbagwe/agentflow/internal/observability"
	"github.com/sagarbagwe/agentflow/internal/queue"
	"github.com/sagarbagwe/agentflow/internal/realtime"
	"github.com/sagarbagwe/agentflow/internal/store"
	"github.com/sagarbagwe/agentflow/internal/tool"
)

func TestHealthHandler(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d; want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q; want JSON", got)
	}
	if got := response.Body.String(); got != `{"status":"ok"}` {
		t.Fatalf("body = %q", got)
	}
}

func TestProtectedRouteRequiresAuthentication(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/executions/missing", nil)
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d; want %d", response.Code, http.StatusUnauthorized)
	}
}

func testServer(t *testing.T) *http.Server {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	storage := store.NewMemory()
	registry, err := tool.NewRegistry(tool.DefaultTools()...)
	if err != nil {
		t.Fatal(err)
	}
	return New(Dependencies{
		Config:  config.HTTPConfig{ReadHeaderTimeout: time.Second},
		Logger:  logger,
		Service: app.New(storage, queue.NewImmediate(10)),
		Auth:    auth.New("test-secret", "test-key", "test-user"),
		Metrics: observability.NewMetrics(),
		Broker:  realtime.NewBroker(),
		Tools:   registry,
	})
}
