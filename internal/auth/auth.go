package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Principal struct {
	ID   string
	Role string
}

type Authenticator struct {
	jwtSecret  []byte
	apiKeyHash [32]byte
	apiKeyUser string
}

func New(jwtSecret, apiKey, apiKeyUser string) *Authenticator {
	return &Authenticator{jwtSecret: []byte(jwtSecret), apiKeyHash: sha256.Sum256([]byte(apiKey)), apiKeyUser: apiKeyUser}
}

func (a *Authenticator) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		principal, err := a.authenticate(ctx.GetHeader("Authorization"), ctx.GetHeader("X-API-Key"))
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		ctx.Set("principal", principal)
		ctx.Next()
	}
}

func (a *Authenticator) authenticate(authorization, apiKey string) (Principal, error) {
	if apiKey != "" {
		candidate := sha256.Sum256([]byte(apiKey))
		if subtle.ConstantTimeCompare(candidate[:], a.apiKeyHash[:]) == 1 {
			return Principal{ID: a.apiKeyUser, Role: "admin"}, nil
		}
	}
	if !strings.HasPrefix(authorization, "Bearer ") {
		return Principal{}, errors.New("credentials are required")
	}
	token, err := jwt.Parse(strings.TrimPrefix(authorization, "Bearer "), func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return a.jwtSecret, nil
	}, jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return Principal{}, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Principal{}, errors.New("invalid claims")
	}
	subject, err := claims.GetSubject()
	if err != nil || subject == "" {
		return Principal{}, errors.New("subject is required")
	}
	role, _ := claims["role"].(string)
	if role == "" {
		role = "developer"
	}
	return Principal{ID: subject, Role: role}, nil
}

func PrincipalFrom(ctx *gin.Context) Principal {
	value, _ := ctx.Get("principal")
	principal, _ := value.(Principal)
	return principal
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(ctx *gin.Context) {
		if _, ok := allowed[PrincipalFrom(ctx).Role]; !ok {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		ctx.Next()
	}
}

type visitor struct {
	windowStart time.Time
	count       int
}

type Limiter struct {
	mu       sync.Mutex
	visitors map[string]visitor
	limit    int
	window   time.Duration
}

func NewLimiter(limit int, window time.Duration) *Limiter {
	return &Limiter{visitors: make(map[string]visitor), limit: limit, window: window}
}

func (l *Limiter) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		key := ctx.ClientIP()
		if principal := PrincipalFrom(ctx); principal.ID != "" {
			key = principal.ID
		}
		now := time.Now()
		l.mu.Lock()
		entry := l.visitors[key]
		if now.Sub(entry.windowStart) >= l.window {
			entry = visitor{windowStart: now}
		}
		entry.count++
		l.visitors[key] = entry
		allowed := entry.count <= l.limit
		l.mu.Unlock()
		if !allowed {
			ctx.Header("Retry-After", "60")
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		ctx.Next()
	}
}
