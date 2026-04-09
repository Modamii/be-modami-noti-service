// Ingest: Kafka consumer + HTTP webhook. Parse NotificationEvent, dispatch by Identity.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"be-modami-no-service/config"
	"be-modami-no-service/internal/handlers"
	"be-modami-no-service/internal/queue"
	"be-modami-no-service/internal/service"
	mongostore "be-modami-no-service/internal/store/mongo"
	"be-modami-no-service/pkg/contract"
	"be-modami-no-service/pkg/health"
	"be-modami-no-service/pkg/kafka"
	database "be-modami-no-service/pkg/storage/database/mongodb"

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

	// MongoDB
	mongoDB, err := database.NewMongoDB(database.MongoConfig{
		URI:      cfg.MongoDB.URI,
		Database: cfg.MongoDB.Database,
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
	preferenceStore := mongostore.NewPreferenceStore(mongoDB.Database)
	subscriberStore := mongostore.NewSubscriberStore(mongoDB.Database)

	// Redis
	rdb := redis.NewClient(configs.RedisOptions(cfg))
	if err := rdb.Ping(ctx).Err(); err != nil {
		l.Error("redis ping failed", err)
		os.Exit(1)
	}
	defer rdb.Close()

	q := queue.New(rdb)

	// Build notification service with dispatchers (Strategy Pattern)
	inAppDispatcher := service.NewInAppDispatcher(q, cfg.Queue.WSKey)
	pushDispatcher := service.NewPushDispatcher(q, cfg.Queue.PushKey)
	notifSvc := service.NewNotificationService(
		notificationStore,
		preferenceStore,
		subscriberStore,
		inAppDispatcher,
		pushDispatcher,
	)

	reg := newRegistryWithHandlers(notifSvc)

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Kafka consumer using KafkaService with retry, DLQ, and trace propagation
	if cfg.Kafka.Enable && cfg.Kafka.BrokerList != "" {
		kafkaSvc, err := kafka.NewKafkaService(nil, cfg)
		if err != nil {
			l.Error("failed to create kafka service", err)
			os.Exit(1)
		}
		defer kafkaSvc.Close()

		if err := kafkaSvc.EnsureTopics(ctx); err != nil {
			l.Error("failed to ensure kafka topics", err)
		}

		consumer := buildNotificationConsumer(kafkaSvc, reg)

		go func() {
			l.Info("starting kafka consumer with topic handlers")
			if err := kafkaSvc.StartConsumer(sigCtx, []kafka.ConsumerHandler{consumer}); err != nil {
				l.Error("kafka consumer exited", err)
			}
		}()
	}

	// HTTP webhook (fallback / direct integration)
	mux := http.NewServeMux()

	checker := health.NewHandler(health.NewRedisChecker(rdb))
	mux.HandleFunc("GET /healthz", checker.Healthz)
	mux.HandleFunc("GET /readyz", checker.Readyz)

	mux.HandleFunc("POST /webhook", webhookHandler(reg))

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

// newRegistryWithHandlers creates the handler registry with all known identities.
func newRegistryWithHandlers(svc *service.NotificationService) handlers.Registry {
	reg := handlers.NewRegistry()
	reg.Register(contract.ContentPublished, handlers.ContentPublished(svc))
	reg.Register(contract.CommentCreated, handlers.CommentCreated(svc))
	return reg
}

// buildNotificationConsumer creates a Kafka consumer with TopicHandler per notification-triggering topic.
// Each handler: deserialize → map topic to identity → validate → dispatch via registry.
func buildNotificationConsumer(kafkaSvc *kafka.KafkaService, reg handlers.Registry) *kafka.BaseConsumer {
	consumer := kafka.NewBaseConsumer("notification-ingest", kafkaSvc)

	opts := &kafka.ConsumerOptions{
		RetryCount:      3,
		RetryDelay:      time.Second,
		DeadLetterTopic: "notification.dead-letter",
		EnableLogging:   true,
	}

	// Register a TopicHandler for each topic that maps to a notification identity
	for baseTopic, identity := range contract.TopicToIdentity {
		id := identity // capture for closure
		handler := kafka.NewTopicHandler(
			baseTopic,
			func(ctx context.Context, payload interface{}) error {
				evt, ok := payload.(contract.NotificationEvent)
				if !ok {
					logger.FromContext(ctx).Error("unexpected payload type", nil)
					return nil
				}
				// Override identity from topic mapping (source of truth)
				evt.Identity = id

				if err := evt.Validate(); err != nil {
					logger.FromContext(ctx).Error("event validation failed", err)
					return nil // skip invalid events, don't retry
				}

				h, ok := reg.Get(evt.Identity)
				if !ok {
					return nil
				}
				return h(ctx, &evt)
			},
			reflect.TypeOf(contract.NotificationEvent{}),
			opts,
		)
		consumer.WithHandler(handler)
	}

	return consumer
}

// webhookHandler handles HTTP webhook ingestion with validation.
func webhookHandler(reg handlers.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var e contract.NotificationEvent
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := e.Validate(); err != nil {
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
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}
