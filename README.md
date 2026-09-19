# AgentFlow

AgentFlow is a production-oriented AI agent execution and observability platform written in Go.

## Status

The project is being built incrementally through small, reviewable commits. The first milestone establishes the API process, health checks, structured logging, configuration, and graceful shutdown.

## Run locally

Requires Go 1.24 or newer.

```bash
go run ./cmd/api
```

The API listens on `:8080` by default. Override it with `HTTP_ADDR`.

```bash
curl http://localhost:8080/healthz
```

Expected response:

```json
{"status":"ok"}
```

## Planned architecture

```text
Client
  -> API / Authentication
  -> Redis job queue
  -> Distributed workers
  -> Workflow engine
  -> Tools and LLM providers
  -> PostgreSQL / Redis
  -> OpenTelemetry / Prometheus / Grafana
```

## Delivery milestones

1. Service foundation and configuration
2. HTTP API and authentication
3. PostgreSQL schema and repositories
4. Redis queue and distributed workers
5. Agent workflow and tool framework
6. LLM provider adapters
7. WebSocket execution streaming
8. Reliability controls and idempotency
9. Metrics, traces, and dashboards
10. Containers, Kubernetes, CI/CD, and documentation

## License

To be decided.
