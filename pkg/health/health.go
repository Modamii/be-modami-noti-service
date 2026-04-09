package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"be-modami-no-service/pkg/centrifugo"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Checker verifies a dependency is healthy.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// Handler serves /healthz and /readyz endpoints.
type Handler struct {
	checkers []Checker
}

// NewHandler creates a health handler with the given checkers.
func NewHandler(checkers ...Checker) *Handler {
	return &Handler{checkers: checkers}
}

// Healthz is a liveness probe — always returns 200.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

// Readyz is a readiness probe — checks all dependencies.
func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	status := "ready"
	code := http.StatusOK
	checks := make(map[string]string, len(h.checkers))

	for _, c := range h.checkers {
		if err := c.Check(ctx); err != nil {
			checks[c.Name()] = err.Error()
			status = "not_ready"
			code = http.StatusServiceUnavailable
		} else {
			checks[c.Name()] = "ok"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": status,
		"checks": checks,
	})
}

// RedisChecker checks Redis connectivity.
type RedisChecker struct {
	client *redis.Client
}

func NewRedisChecker(client *redis.Client) *RedisChecker {
	return &RedisChecker{client: client}
}

func (c *RedisChecker) Name() string                        { return "redis" }
func (c *RedisChecker) Check(ctx context.Context) error     { return c.client.Ping(ctx).Err() }

// MongoChecker checks MongoDB connectivity.
type MongoChecker struct {
	client *mongo.Client
}

func NewMongoChecker(client *mongo.Client) *MongoChecker {
	return &MongoChecker{client: client}
}

func (c *MongoChecker) Name() string                    { return "mongodb" }
func (c *MongoChecker) Check(ctx context.Context) error { return c.client.Ping(ctx, readpref.Primary()) }

// CentrifugoChecker checks Centrifugo API connectivity.
type CentrifugoChecker struct {
	client *centrifugo.Client
}

func NewCentrifugoChecker(client *centrifugo.Client) *CentrifugoChecker {
	return &CentrifugoChecker{client: client}
}

func (c *CentrifugoChecker) Name() string                    { return "centrifugo" }
func (c *CentrifugoChecker) Check(ctx context.Context) error { return c.client.Ping(ctx) }
