# AgentFlow

AgentFlow is a production-oriented AI agent execution and observability platform built with Go 1.24. It separates request handling from asynchronous execution, records durable step-level traces, streams live events over WebSockets, and exposes metrics and OTLP traces for operating agent workloads.

## Highlights

- Versioned agent definitions with replaceable LLM and tool interfaces
- Multi-step LLM → tool → LLM workflows with parallel tool calls
- Redis-backed distributed workers with independent API/worker scaling
- PostgreSQL execution history, step traces, conversations, tools, and API-key schema
- Context cancellation, deadlines, jittered retries, circuit breaker, and idempotency keys
- JWT/API-key authentication, RBAC, owner scoping, validation, and rate limiting
- WebSocket execution events and Prometheus/Grafana observability
- OpenTelemetry HTTP, LLM, tool, and execution traces
- Docker Compose, hardened Kubernetes manifests, race-tested CI, and GHCR releases

## Architecture

```text
Client → Gin API → authentication/RBAC/rate limit → PostgreSQL
                 └→ Redis queue → concurrent Go workers
                                  → workflow engine
                                  → LLM provider / tools
                                  → PostgreSQL traces
                                  → WebSocket events
API + workers → OpenTelemetry Collector / Prometheus → Grafana
```

See [the architecture guide](docs/architecture.md) and [OpenAPI specification](docs/openapi.yaml).

## Quick start

Requirements: Docker with Compose.

```bash
cp .env.example .env
docker compose up --build
```

Local endpoints:

- API: `http://localhost:8080`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000` (`admin` / `admin` for local development)
- OTLP HTTP: `http://localhost:4318`

The local bootstrap API key is `dev-agentflow-key`. Replace it in `.env` outside disposable development environments.

### Create an agent

```bash
curl -sS http://localhost:8080/api/v1/agents \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: dev-agentflow-key' \
  -d '{
    "name": "operations-assistant",
    "system_prompt": "Answer concisely and use tools when useful.",
    "model": "gpt-4o-mini",
    "temperature": 0.2,
    "max_tokens": 1024,
    "tools": ["calculator", "weather", "web_search"]
  }'
```

### Queue an execution

```bash
curl -sS http://localhost:8080/api/v1/agents/AGENT_ID/execute \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: dev-agentflow-key' \
  -H 'Idempotency-Key: demo-request-001' \
  -d '{"input":"What is 125 multiplied by 8?"}'
```

Poll `GET /api/v1/executions/{id}`, inspect `GET /api/v1/executions/{id}/trace`, or connect to `WS /api/v1/executions/{id}/stream`.

## Run without infrastructure

When `DATABASE_URL` and `REDIS_ADDR` are empty, AgentFlow uses in-memory adapters and runs API plus workers in one process:

```bash
go run ./cmd/api
```

Use `RUN_MODE=api`, `RUN_MODE=worker`, or `RUN_MODE=all`. Production API and worker deployments must share PostgreSQL and Redis.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `RUN_MODE` | `all` | `api`, `worker`, or `all` |
| `HTTP_ADDR` | `:8080` | HTTP bind address |
| `DATABASE_URL` | empty | PostgreSQL connection URL |
| `REDIS_ADDR` | empty | Redis address |
| `REDIS_PASSWORD` | empty | Redis password |
| `REDIS_QUEUE_KEY` | `agentflow:jobs` | Job queue list key |
| `WORKER_CONCURRENCY` | `4` | Consumers per worker process |
| `JWT_SECRET` | development value | HS256 verification secret |
| `AGENTFLOW_API_KEY` | development value | Bootstrap API key |
| `AGENTFLOW_API_KEY_USER` | local UUID | Owner represented by bootstrap key |
| `LLM_PROVIDER` | `mock` | `mock` or OpenAI-compatible mode |
| `LLM_BASE_URL` | OpenAI v1 URL | Compatible provider base URL |
| `LLM_API_KEY` | empty | Provider secret |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | empty | OTLP/HTTP collector endpoint |

All HTTP timeouts are independently configurable; see `.env.example` and `internal/config`.

## Built-in tools

Every tool implements:

```go
type Tool interface {
    Definition() Definition
    Execute(context.Context, json.RawMessage) (any, error)
}
```

The repository includes a calculator and deterministic mocks for web search, weather, read-only database queries, and an internal company API. The registry is replaceable and safe for concurrent reads and registration.

## Observability

Prometheus metrics include:

- `agent_requests_total`
- `agent_errors_total`
- `agent_execution_duration_seconds`
- `llm_latency_seconds`
- `tool_execution_duration_seconds`
- `token_usage_total`
- HTTP request counts and durations

Execution spans include agent ID, execution ID, model, tool name, errors, and nested LLM/tool spans. Grafana provisioning and a starter dashboard are under `deploy/grafana`.

## Database migrations

Migrations live in `migrations/`. Docker Compose runs them automatically. For other environments, run a migration job before rolling out API and workers:

```bash
migrate -path migrations -database "$DATABASE_URL" up
```

The schema covers users, agents, immutable agent versions, executions, execution steps, conversations, messages, tools, tool executions, and API keys with owner/status/time indexes.

## Testing

```bash
make test-race
```

Integration tests activate when these variables are present:

```bash
INTEGRATION_DATABASE_URL='postgres://...' \
INTEGRATION_REDIS_ADDR='localhost:6379' \
go test -race ./integration/...
```

CI verifies formatting, vetting, race safety, integration connectivity, vulnerability scanning, binary builds, and container builds.

## Kubernetes

Base manifests are in `deploy/kubernetes/base` and include:

- Independent API and worker Deployments
- Readiness/liveness probes
- CPU autoscaling
- Pod disruption budget
- Non-root, read-only, capability-dropped containers
- Network policy and resource limits

Create `agentflow-secrets` from `deploy/kubernetes/secret.example.yaml`, apply migrations, pin the image tag, then deploy:

```bash
kubectl apply -k deploy/kubernetes/base
```

## Release

Push a semantic version tag such as `v0.1.0`. GitHub Actions builds multi-platform images with SBOM and provenance metadata and publishes versioned tags to GHCR.

## Current production considerations

Before internet-facing use:

- Replace the bootstrap API key with database-backed, hashed API-key lookup and rotation.
- Use a managed PostgreSQL/Redis setup with TLS, backups, and queue monitoring.
- Put the API behind an ingress/API gateway with TLS and WebSocket support.
- Export OTLP traces to a durable backend such as Tempo, Jaeger, or a hosted service.
- Add provider-specific retry classification, budgets, evaluations, and content controls.
