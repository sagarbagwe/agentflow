package observability

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	AgentRequests prometheus.Counter
	AgentErrors   prometheus.Counter
	AgentDuration prometheus.Histogram
	LLMLatency    *prometheus.HistogramVec
	ToolDuration  *prometheus.HistogramVec
	TokenUsage    *prometheus.CounterVec
	registry      *prometheus.Registry
}

func NewMetrics() *Metrics {
	metrics := &Metrics{
		AgentRequests: prometheus.NewCounter(prometheus.CounterOpts{Name: "agent_requests_total", Help: "Total agent execution requests."}),
		AgentErrors:   prometheus.NewCounter(prometheus.CounterOpts{Name: "agent_errors_total", Help: "Total failed agent executions."}),
		AgentDuration: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "agent_execution_duration_seconds", Help: "Agent execution duration.", Buckets: prometheus.DefBuckets}),
		LLMLatency:    prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "llm_latency_seconds", Help: "LLM provider request latency.", Buckets: prometheus.DefBuckets}, []string{"model"}),
		ToolDuration:  prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "tool_execution_duration_seconds", Help: "Tool execution duration.", Buckets: prometheus.DefBuckets}, []string{"tool"}),
		TokenUsage:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "token_usage_total", Help: "Tokens consumed by direction and model."}, []string{"model", "direction"}),
		registry:      prometheus.NewRegistry(),
	}
	metrics.registry.MustRegister(metrics.AgentRequests, metrics.AgentErrors, metrics.AgentDuration, metrics.LLMLatency, metrics.ToolDuration, metrics.TokenUsage, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return metrics
}

func (m *Metrics) Handler() gin.HandlerFunc {
	handler := promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
	return gin.WrapH(handler)
}

func (m *Metrics) HTTPMiddleware() gin.HandlerFunc {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total", Help: "HTTP requests by method, route, and status."}, []string{"method", "route", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP request duration.", Buckets: prometheus.DefBuckets}, []string{"method", "route"})
	m.registry.MustRegister(requests, duration)
	return func(ctx *gin.Context) {
		started := time.Now()
		ctx.Next()
		route := ctx.FullPath()
		if route == "" {
			route = "unmatched"
		}
		requests.WithLabelValues(ctx.Request.Method, route, strconv.Itoa(ctx.Writer.Status())).Inc()
		duration.WithLabelValues(ctx.Request.Method, route).Observe(time.Since(started).Seconds())
	}
}
