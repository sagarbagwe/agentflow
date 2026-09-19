package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sagarbagwe/agentflow/internal/app"
	"github.com/sagarbagwe/agentflow/internal/auth"
	"github.com/sagarbagwe/agentflow/internal/config"
	"github.com/sagarbagwe/agentflow/internal/observability"
	"github.com/sagarbagwe/agentflow/internal/realtime"
	"github.com/sagarbagwe/agentflow/internal/store"
	"github.com/sagarbagwe/agentflow/internal/tool"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Dependencies struct {
	Config  config.HTTPConfig
	Logger  *slog.Logger
	Service *app.Service
	Auth    *auth.Authenticator
	Metrics *observability.Metrics
	Broker  *realtime.Broker
	Tools   *tool.Registry
}

func init() {
	gin.SetMode(gin.ReleaseMode)
}

type API struct {
	service  *app.Service
	logger   *slog.Logger
	broker   *realtime.Broker
	tools    *tool.Registry
	upgrader websocket.Upgrader
}

func New(dependencies Dependencies) *http.Server {
	router := gin.New()
	api := &API{
		service: dependencies.Service,
		logger:  dependencies.Logger,
		broker:  dependencies.Broker,
		tools:   dependencies.Tools,
		upgrader: websocket.Upgrader{CheckOrigin: func(request *http.Request) bool {
			origin := request.Header.Get("Origin")
			if origin == "" {
				return true
			}
			parsed, err := url.Parse(origin)
			return err == nil && parsed.Host == request.Host
		}},
	}

	router.Use(gin.Recovery(), otelgin.Middleware("agentflow-api"), dependencies.Metrics.HTTPMiddleware(), requestLogger(dependencies.Logger), requestID(), bodyLimit(1<<20))
	router.GET("/healthz", func(ctx *gin.Context) { ctx.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	router.GET("/readyz", func(ctx *gin.Context) { ctx.JSON(http.StatusOK, gin.H{"status": "ready"}) })
	router.GET("/metrics", dependencies.Metrics.Handler())

	v1 := router.Group("/api/v1")
	v1.Use(dependencies.Auth.Middleware(), auth.NewLimiter(120, time.Minute).Middleware())
	v1.POST("/agents", auth.RequireRoles("admin", "developer"), api.createAgent)
	v1.GET("/agents/:id", api.getAgent)
	v1.PUT("/agents/:id", auth.RequireRoles("admin", "developer"), api.updateAgent)
	v1.DELETE("/agents/:id", auth.RequireRoles("admin", "developer"), api.deleteAgent)
	v1.POST("/agents/:id/execute", auth.RequireRoles("admin", "developer"), api.executeAgent)
	v1.GET("/agents/:id/executions", api.listExecutions)
	v1.GET("/executions/:id", api.getExecution)
	v1.GET("/executions/:id/trace", api.getTrace)
	v1.GET("/executions/:id/stream", api.streamExecution)
	v1.POST("/tools", auth.RequireRoles("admin"), api.createTool)

	return &http.Server{
		Addr:              dependencies.Config.Addr,
		Handler:           router,
		ReadHeaderTimeout: dependencies.Config.ReadHeaderTimeout,
		ReadTimeout:       dependencies.Config.ReadTimeout,
		WriteTimeout:      dependencies.Config.WriteTimeout,
		IdleTimeout:       dependencies.Config.IdleTimeout,
	}
}

func (a *API) createAgent(ctx *gin.Context) {
	var input app.AgentInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	agent, err := a.service.CreateAgent(ctx, auth.PrincipalFrom(ctx).ID, input)
	if err != nil {
		writeError(ctx, err, http.StatusBadRequest)
		return
	}
	ctx.JSON(http.StatusCreated, agent)
}

func (a *API) getAgent(ctx *gin.Context) {
	agent, err := a.service.GetAgent(ctx, auth.PrincipalFrom(ctx).ID, ctx.Param("id"))
	if err != nil {
		writeError(ctx, err, http.StatusInternalServerError)
		return
	}
	ctx.JSON(http.StatusOK, agent)
}

func (a *API) updateAgent(ctx *gin.Context) {
	var input app.AgentInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	agent, err := a.service.UpdateAgent(ctx, auth.PrincipalFrom(ctx).ID, ctx.Param("id"), input)
	if err != nil {
		writeError(ctx, err, http.StatusBadRequest)
		return
	}
	ctx.JSON(http.StatusOK, agent)
}

func (a *API) deleteAgent(ctx *gin.Context) {
	if err := a.service.DeleteAgent(ctx, auth.PrincipalFrom(ctx).ID, ctx.Param("id")); err != nil {
		writeError(ctx, err, http.StatusInternalServerError)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (a *API) executeAgent(ctx *gin.Context) {
	var input app.ExecutionInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	input.IdempotencyKey = ctx.GetHeader("Idempotency-Key")
	input.RequestID = ctx.GetString("request_id")
	execution, replayed, err := a.service.StartExecution(ctx, auth.PrincipalFrom(ctx).ID, ctx.Param("id"), input)
	if err != nil {
		writeError(ctx, err, http.StatusBadRequest)
		return
	}
	if replayed {
		ctx.Header("Idempotent-Replayed", "true")
	}
	ctx.JSON(http.StatusAccepted, execution)
}

func (a *API) listExecutions(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	executions, err := a.service.ListExecutions(ctx, auth.PrincipalFrom(ctx).ID, ctx.Param("id"), limit)
	if err != nil {
		writeError(ctx, err, http.StatusInternalServerError)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"items": executions})
}

func (a *API) getExecution(ctx *gin.Context) {
	execution, err := a.service.GetExecution(ctx, auth.PrincipalFrom(ctx).ID, ctx.Param("id"))
	if err != nil {
		writeError(ctx, err, http.StatusInternalServerError)
		return
	}
	ctx.JSON(http.StatusOK, execution)
}

func (a *API) getTrace(ctx *gin.Context) {
	trace, err := a.service.GetTrace(ctx, auth.PrincipalFrom(ctx).ID, ctx.Param("id"))
	if err != nil {
		writeError(ctx, err, http.StatusInternalServerError)
		return
	}
	ctx.JSON(http.StatusOK, trace)
}

func (a *API) streamExecution(ctx *gin.Context) {
	executionID := ctx.Param("id")
	if _, err := a.service.GetExecution(ctx, auth.PrincipalFrom(ctx).ID, executionID); err != nil {
		writeError(ctx, err, http.StatusInternalServerError)
		return
	}
	connection, err := a.upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		a.logger.Warn("WebSocket upgrade failed", "error", err)
		return
	}
	defer connection.Close()
	events, unsubscribe := a.broker.Subscribe(executionID)
	defer unsubscribe()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := connection.WriteJSON(event); err != nil {
				return
			}
		case <-ctx.Request.Context().Done():
			return
		}
	}
}

func (a *API) createTool(ctx *gin.Context) {
	var input struct {
		Name        string            `json:"name" binding:"required"`
		Description string            `json:"description" binding:"required"`
		Responses   map[string]string `json:"responses" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&input); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "name, description, and responses are required"})
		return
	}
	created := tool.MockLookup{Name: input.Name, Description: input.Description, Results: input.Responses}
	if err := a.tools.Register(created); err != nil {
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, created.Definition())
}

func writeError(ctx *gin.Context, err error, fallback int) {
	status := fallback
	if errors.Is(err, store.ErrNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, store.ErrConflict) {
		status = http.StatusConflict
	}
	ctx.JSON(status, gin.H{"error": err.Error()})
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		started := time.Now()
		ctx.Next()
		logger.InfoContext(ctx, "HTTP request", "method", ctx.Request.Method, "path", ctx.Request.URL.Path, "status", ctx.Writer.Status(), "duration", time.Since(started), "request_id", ctx.GetString("request_id"))
	}
}

func requestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		value := ctx.GetHeader("X-Request-ID")
		if value == "" {
			value = strconv.FormatInt(time.Now().UnixNano(), 36)
		}
		ctx.Set("request_id", value)
		ctx.Header("X-Request-ID", value)
		ctx.Next()
	}
}

func bodyLimit(limit int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, limit)
		ctx.Next()
	}
}
