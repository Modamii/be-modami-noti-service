// Ingest: Kafka consumer + HTTP webhook. Parse NotificationEvent, dispatch by Identity.
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

	"github.com/redis/go-redis/v9"
	"github.com/techinsight/be-techinsights-notification-service/configs"
	"github.com/techinsight/be-techinsights-notification-service/internal/handlers"
	"github.com/techinsight/be-techinsights-notification-service/internal/queue"
	"github.com/techinsight/be-techinsights-notification-service/internal/store"
	mongostore "github.com/techinsight/be-techinsights-notification-service/internal/store/mongo"
	"github.com/techinsight/be-techinsights-notification-service/pkg/contract"
	"github.com/techinsight/be-techinsights-notification-service/pkg/health"
	"github.com/techinsight/be-techinsights-notification-service/pkg/kafka"
	database "github.com/techinsight/be-techinsights-notification-service/pkg/storage/database/mongodb"
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

	// MongoDB connection for persisting notifications
	mongoDB, err := database.NewMongoDB(database.MongoConfig{
		URI:      cfg.Database.MongoDB.URI,
		Database: cfg.Database.MongoDB.Database,
	})
	if err != nil {
		l.Error("failed to connect to MongoDB", err)
		os.Exit(1)
	}
	defer mongoDB.Close(context.Background())

	if err := mongostore.EnsureIndexes(ctx, mongoDB.Database); err != nil {
		l.Error("failed to ensure indexes", err)
	}

	notificationStore := mongostore.NewNotificationStore(mongoDB.Database)

	rdb := redis.NewClient(configs.RedisOptions(cfg))
	if err := rdb.Ping(ctx).Err(); err != nil {
		l.Error("redis ping failed", err)
		os.Exit(1)
	}
	defer rdb.Close()

	q := queue.New(rdb)
	reg := NewRegistryWithHandlers(notificationStore, q, cfg.Queue.WSKey, cfg.Queue.PushKey)

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Kafka consumer
	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		go func() {
			l.Info("starting kafka consumer for topic: " + cfg.Kafka.NotificationTopic)
			err := kafka.RunConsumer(sigCtx, cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroupID, cfg.Kafka.NotificationTopic,
				func(ctx context.Context, key, value []byte) error {
					var e contract.NotificationEvent
					if err := json.Unmarshal(value, &e); err != nil {
						logger.FromContext(ctx).Error("kafka: unmarshal event failed", err)
						return nil
					}
					h, ok := reg.Get(e.Identity)
					if !ok {
						return nil
					}
					return h(ctx, &e)
				},
			)
			if err != nil && err != context.Canceled {
				l.Error("kafka consumer exited", err)
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
			logger.FromContext(r.Context()).Error("webhook handler error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})

	srv := &http.Server{Addr: cfg.Servers.IngestAddr, Handler: mux}
	go func() {
		l.Info("ingest listening on " + cfg.Servers.IngestAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.Error("server error", err)
		}
	}()

	<-sigCtx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	l.Info("ingest stopped")
}

func NewRegistryWithHandlers(ns store.NotificationStore, q *queue.Queue, queueWS, queuePush string) handlers.Registry {
	reg := handlers.NewRegistry()
	reg.Register(contract.ContentPublished, handlers.ContentPublished(ns, q, queueWS, queuePush))
	reg.Register(contract.CommentCreated, handlers.CommentCreated(ns, q, queueWS, queuePush))
	return reg
}
