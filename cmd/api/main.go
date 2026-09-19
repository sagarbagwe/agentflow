package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/sagarbagwe/agentflow/internal/app"
	"github.com/sagarbagwe/agentflow/internal/auth"
	"github.com/sagarbagwe/agentflow/internal/config"
	"github.com/sagarbagwe/agentflow/internal/httpapi"
	"github.com/sagarbagwe/agentflow/internal/llm"
	"github.com/sagarbagwe/agentflow/internal/observability"
	"github.com/sagarbagwe/agentflow/internal/queue"
	"github.com/sagarbagwe/agentflow/internal/realtime"
	storecontract "github.com/sagarbagwe/agentflow/internal/store"
	pgstore "github.com/sagarbagwe/agentflow/internal/store/postgres"
	"github.com/sagarbagwe/agentflow/internal/tool"
	"github.com/sagarbagwe/agentflow/internal/worker"
	"github.com/sagarbagwe/agentflow/internal/workflow"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration is invalid", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	storage, closeStorage, err := buildStore(ctx, cfg)
	if err != nil {
		logger.Error("storage initialization failed", "error", err)
		os.Exit(1)
	}
	defer closeStorage()

	jobQueue, closeQueue, err := buildQueue(ctx, cfg)
	if err != nil {
		logger.Error("queue initialization failed", "error", err)
		os.Exit(1)
	}
	defer closeQueue()

	registry, err := tool.NewRegistry(tool.DefaultTools()...)
	if err != nil {
		logger.Error("tool registry initialization failed", "error", err)
		os.Exit(1)
	}
	broker := realtime.NewBroker()
	provider := buildProvider(cfg)
	engine := workflow.NewEngine(provider, registry, storage, broker)
	jobWorker := worker.New(jobQueue, storage, engine, logger)
	service := app.New(storage, jobQueue)
	metrics := observability.NewMetrics()
	server := httpapi.New(httpapi.Dependencies{
		Config:  cfg.HTTP,
		Logger:  logger,
		Service: service,
		Auth:    auth.New(cfg.JWTSecret, cfg.APIKey, cfg.APIKeyUser),
		Metrics: metrics,
		Broker:  broker,
		Tools:   registry,
	})

	if cfg.RunMode != "api" && cfg.RunMode != "worker" && cfg.RunMode != "all" {
		logger.Error("RUN_MODE must be api, worker, or all", "run_mode", cfg.RunMode)
		os.Exit(1)
	}

	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	if cfg.RunMode == "worker" || cfg.RunMode == "all" {
		go func() {
			logger.Info("worker pool started", "concurrency", cfg.WorkerConcurrency)
			if err := jobWorker.Run(workerCtx, cfg.WorkerConcurrency); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("worker pool stopped", "error", err)
			}
		}()
	}

	var serverErrors <-chan error
	if cfg.RunMode == "api" || cfg.RunMode == "all" {
		errorsChannel := make(chan error, 1)
		serverErrors = errorsChannel
		go func() {
			logger.Info("API server started", "address", server.Addr)
			errorsChannel <- server.ListenAndServe()
		}()
	}

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("API server failed", "error", err)
		}
	}

	cancelWorkers()
	if cfg.RunMode == "api" || cfg.RunMode == "all" {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
	}
	logger.Info("AgentFlow stopped")
}

func buildStore(ctx context.Context, cfg config.Config) (storecontract.Store, func(), error) {
	if cfg.DatabaseURL == "" {
		return storecontract.NewMemory(), func() {}, nil
	}
	storage, err := pgstore.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, func() {}, err
	}
	return storage, storage.Close, nil
}

func buildQueue(ctx context.Context, cfg config.Config) (queue.Queue, func(), error) {
	if cfg.RedisAddr == "" {
		return queue.NewImmediate(1024), func() {}, nil
	}
	client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, func() {}, err
	}
	return queue.NewRedis(client, cfg.QueueKey), func() { _ = client.Close() }, nil
}

func buildProvider(cfg config.Config) llm.Provider {
	if cfg.LLMProvider == "mock" {
		return llm.Mock{}
	}
	client := &http.Client{Timeout: 60 * time.Second, Transport: otelhttp.NewTransport(http.DefaultTransport)}
	return llm.NewOpenAICompatible(cfg.LLMBaseURL, cfg.LLMAPIKey, client)
}
