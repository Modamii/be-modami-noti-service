// Ingest: Kafka consumer + HTTP webhook. Parse NotificationEvent, dispatch by Identity.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/techinsight/be-techinsights-notification-service/configs"
	"github.com/techinsight/be-techinsights-notification-service/internal/handlers"
	"github.com/techinsight/be-techinsights-notification-service/internal/queue"
	"github.com/techinsight/be-techinsights-notification-service/pkg/contract"
	"github.com/techinsight/be-techinsights-notification-service/pkg/health"
	"github.com/techinsight/be-techinsights-notification-service/pkg/kafka"
	"github.com/techinsight/be-techinsights-notification-service/pkg/logging"
)

func main() {
	cfg, err := configs.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	logging.Setup(cfg.App.Debug)

	rdb := redis.NewClient(configs.RedisOptions(cfg))
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Error("redis ping failed", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	q := queue.New(rdb)
	reg := NewRegistryWithHandlers(q, cfg.Queue.WSKey, cfg.Queue.PushKey)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Kafka consumer
	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		go func() {
			slog.Info("starting kafka consumer", "topic", cfg.Kafka.NotificationTopic)
			err := kafka.RunConsumer(ctx, cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroupID, cfg.Kafka.NotificationTopic,
				func(ctx context.Context, key, value []byte) error {
					var e contract.NotificationEvent
					if err := json.Unmarshal(value, &e); err != nil {
						slog.Warn("kafka: unmarshal event failed", "error", err)
						return nil
					}
					h, ok := reg.Get(e.Identity)
					if !ok {
						slog.Debug("kafka: no handler for identity", "identity", e.Identity)
						return nil
					}
					return h(ctx, &e)
				},
			)
			if err != nil && err != context.Canceled {
				slog.Error("kafka consumer exited", "error", err)
			}
		}()
	}

	// HTTP webhook
	mux := http.NewServeMux()

	// Health endpoints
	checker := health.NewHandler(health.NewRedisChecker(rdb))
	mux.HandleFunc("GET /healthz", checker.Healthz)
	mux.HandleFunc("GET /readyz", checker.Readyz)

	mux.HandleFunc("POST /webhook", func(w http.ResponseWriter, r *http.Request) {
		var e contract.NotificationEvent
		if err := decodeJSON(r.Body, &e); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h, ok := reg.Get(e.Identity)
		if !ok {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if err := h(r.Context(), &e); err != nil {
			slog.Error("webhook handler error", "identity", e.Identity, "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})

	srv := &http.Server{Addr: cfg.Servers.IngestAddr, Handler: mux}
	go func() {
		slog.Info("ingest listening", "addr", cfg.Servers.IngestAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	slog.Info("ingest stopped")
}

func NewRegistryWithHandlers(q *queue.Queue, queueWS, queuePush string) handlers.Registry {
	reg := handlers.NewRegistry()
	reg.Register(contract.ContentPublished, handlers.ContentPublished(q, queueWS, queuePush))
	reg.Register(contract.CommentCreated, handlers.CommentCreated(q, queueWS, queuePush))
	return reg
}
