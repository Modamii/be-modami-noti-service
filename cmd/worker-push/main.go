// Worker Push: BRPOP notif:push → FCM/Web Push. Stub: log payload; wire FCM later.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"be-modami-no-service/config"
	"be-modami-no-service/internal/queue"
	"be-modami-no-service/pkg/event"
	"be-modami-no-service/pkg/health"

	"github.com/redis/go-redis/v9"
	"gitlab.com/lifegoeson-libs/pkg-logging/logger"
)

func main() {
	ctx := context.Background()

	cfg, err := configs.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	loggingCfg := cfg.ToLoggingConfig()
	if err := logger.Init(loggingCfg); err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	l := logger.FromContext(ctx)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = logger.Shutdown(shutdownCtx)
	}()

	rdb := redis.NewClient(configs.RedisOptions(cfg))
	if err := rdb.Ping(ctx).Err(); err != nil {
		l.Error("redis ping failed", err)
		os.Exit(1)
	}
	defer rdb.Close()

	q := queue.New(rdb)

	// Health endpoint for Kubernetes probes
	checker := health.NewHandler(health.NewRedisChecker(rdb))
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("GET /healthz", checker.Healthz)
	healthMux.HandleFunc("GET /readyz", checker.Readyz)
	healthSrv := &http.Server{Addr: ":9091", Handler: healthMux}
	go func() {
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.Error("health server error", err)
		}
	}()

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	l.Info("worker-push started, queue: " + cfg.Queue.PushKey)

	err = q.Consume(sigCtx, cfg.Queue.PushKey, 5*time.Second, func(b []byte) error {
		var msg event.PushMessage
		if err := json.Unmarshal(b, &msg); err != nil {
			l.Error("unmarshal PushMessage failed", err)
			return nil
		}
		// Stub: log payload. Replace with FCM + Web Push when ready.
		l.Info("push stub: title=" + msg.Title + " body=" + msg.Body)
		return nil
	})
	if err != nil && err != context.Canceled {
		l.Error("consume loop exited", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = healthSrv.Shutdown(shutdownCtx)
	l.Info("worker-push stopped")
}
