// Worker Push: BRPOP notif:push → FCM. iOS is reached through FCM as well,
// with the APNs key uploaded to the Firebase console.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"be-modami-no-service/config"
	"be-modami-no-service/internal/queue"
	mongostore "be-modami-no-service/internal/store/mongo"
	"be-modami-no-service/pkg/event"
	"be-modami-no-service/pkg/health"
	"be-modami-no-service/pkg/metrics"
	"be-modami-no-service/pkg/push"
	database "be-modami-no-service/pkg/storage/database/mongodb"

	"github.com/redis/go-redis/v9"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
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
	reg := metrics.NewRegistry()
	pushSent := reg.NewCounter("notif_push_sent_total", "Pushes accepted by FCM")
	pushFailed := reg.NewCounter("notif_push_failed_total", "Pushes FCM rejected for a retryable reason")
	pushPruned := reg.NewCounter("notif_push_pruned_total", "Device tokens deleted after FCM reported them unregistered")
	reg.RegisterGauge("notif_queue_depth", "Messages waiting in the Redis queue",
		map[string]string{"queue": "push"},
		func() (float64, error) {
			n, err := rdb.LLen(context.Background(), cfg.Queue.PushKey).Result()
			return float64(n), err
		})

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("GET /healthz", checker.Healthz)
	healthMux.HandleFunc("GET /readyz", checker.Readyz)
	healthMux.HandleFunc("GET /metrics", reg.Handler())
	healthSrv := &http.Server{Addr: ":7074", Handler: healthMux}
	go func() {
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.Error("health server error", err)
		}
	}()

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	sender, err := push.NewFCMSender(ctx, cfg.FCM.CredentialsPath)
	if err != nil {
		// Without credentials the worker still drains the queue, so messages do
		// not pile up in Redis — but it says so loudly at every start.
		l.Error("push disabled: FCM not configured, messages will be dropped", err)
	}

	// Mongo is needed to delete tokens FCM rejects. Without it the worker still
	// sends; dead tokens just accumulate.
	var subscriberStore interface {
		DeleteByToken(ctx context.Context, userID, token string) error
	}
	if cfg.MongoDB.URI != "" {
		mongoDB, mErr := database.NewMongoDB(database.MongoConfig{
			URI:      cfg.MongoDB.URI,
			Database: cfg.MongoDB.Database,
		})
		if mErr != nil {
			l.Error("worker-push: mongo unavailable, dead tokens will not be pruned", mErr)
		} else {
			defer mongoDB.Close(context.Background())
			subscriberStore = mongostore.NewSubscriberStore(mongoDB.Database)
		}
	}

	l.Info("worker-push started, queue: " + cfg.Queue.PushKey)

	err = q.Consume(sigCtx, cfg.Queue.PushKey, 5*time.Second, func(b []byte) error {
		var msg event.PushMessage
		if err := json.Unmarshal(b, &msg); err != nil {
			l.Error("unmarshal PushMessage failed", err)
			return nil
		}
		if sender == nil {
			l.Info("push dropped (FCM not configured): " + msg.Title)
			return nil
		}

		sendCtx, cancel := context.WithTimeout(sigCtx, 30*time.Second)
		defer cancel()

		result, err := sender.Send(sendCtx, msg.DeviceTokens, push.Notification{
			Title: msg.Title,
			Body:  msg.Body,
			Data:  msg.Data,
		})
		if err != nil {
			// Returning an error stops the consume loop, which would stall the
			// whole queue for one bad message. Log and move on.
			l.Error("push send failed for user "+msg.UserID, err)
			return nil
		}

		pushSent.Add(int64(result.Sent))
		pushFailed.Add(int64(len(result.Failed)))

		for token, reason := range result.Failed {
			l.Error("push rejected for user "+msg.UserID, errors.New(reason),
				logging.String("token", truncate(token)))
		}
		for _, token := range result.Unregistered {
			if subscriberStore == nil {
				continue
			}
			if dErr := subscriberStore.DeleteByToken(sendCtx, msg.UserID, token); dErr != nil {
				l.Error("failed to prune unregistered token for user "+msg.UserID, dErr)
				continue
			}
			pushPruned.Inc()
			l.Info("pruned unregistered device token for user " + msg.UserID)
		}
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

// truncate keeps a token recognisable in logs without writing the whole
// credential to disk.
func truncate(token string) string {
	const keep = 12
	if len(token) <= keep {
		return token
	}
	return token[:keep] + "…"
}
