package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTP              HTTPConfig
	DatabaseURL       string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	QueueKey          string
	WorkerConcurrency int
	JWTSecret         string
	APIKey            string
	APIKeyUser        string
	LLMBaseURL        string
	LLMAPIKey         string
	LLMProvider       string
	RunMode           string
	OTLPEndpoint      string
}

type HTTPConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func Load() (Config, error) {
	readHeaderTimeout, err := durationFromEnv("HTTP_READ_HEADER_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	readTimeout, err := durationFromEnv("HTTP_READ_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := durationFromEnv("HTTP_WRITE_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	idleTimeout, err := durationFromEnv("HTTP_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return Config{}, err
	}
	redisDB, err := intFromEnv("REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}
	workerConcurrency, err := intFromEnv("WORKER_CONCURRENCY", 4)
	if err != nil || workerConcurrency < 1 {
		return Config{}, fmt.Errorf("WORKER_CONCURRENCY must be a positive integer")
	}

	return Config{
		HTTP: HTTPConfig{
			Addr:              stringFromEnv("HTTP_ADDR", ":8080"),
			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
		},
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		RedisAddr:         os.Getenv("REDIS_ADDR"),
		RedisPassword:     os.Getenv("REDIS_PASSWORD"),
		RedisDB:           redisDB,
		QueueKey:          stringFromEnv("REDIS_QUEUE_KEY", "agentflow:jobs"),
		WorkerConcurrency: workerConcurrency,
		JWTSecret:         stringFromEnv("JWT_SECRET", "development-only-change-me"),
		APIKey:            stringFromEnv("AGENTFLOW_API_KEY", "dev-agentflow-key"),
		APIKeyUser:        stringFromEnv("AGENTFLOW_API_KEY_USER", "00000000-0000-0000-0000-000000000001"),
		LLMBaseURL:        stringFromEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:         os.Getenv("LLM_API_KEY"),
		LLMProvider:       stringFromEnv("LLM_PROVIDER", "mock"),
		RunMode:           stringFromEnv("RUN_MODE", "all"),
		OTLPEndpoint:      os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}, nil
}

func stringFromEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return duration, nil
}

func intFromEnv(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}
