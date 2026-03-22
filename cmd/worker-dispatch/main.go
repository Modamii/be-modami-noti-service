// Worker Dispatch: BRPOP notif:ws → publish to Centrifugo via HTTP API.
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
	"github.com/techinsight/be-techinsights-notification-service/internal/queue"
	"github.com/techinsight/be-techinsights-notification-service/pkg/centrifugo"
	"github.com/techinsight/be-techinsights-notification-service/pkg/event"
	"github.com/techinsight/be-techinsights-notification-service/pkg/health"
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
	cfgo := centrifugo.NewClient(cfg.Centrifugo.APIURL, cfg.Centrifugo.APIKey)

	// Health endpoint for Kubernetes probes
	checker := health.NewHandler(
		health.NewRedisChecker(rdb),
		health.NewCentrifugoChecker(cfgo),
	)
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("GET /healthz", checker.Healthz)
	healthMux.HandleFunc("GET /readyz", checker.Readyz)
	healthSrv := &http.Server{Addr: ":9090", Handler: healthMux}
	go func() {
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("health server error", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("worker-dispatch started", "queue", cfg.Queue.WSKey)

	err = q.Consume(ctx, cfg.Queue.WSKey, 5*time.Second, func(b []byte) error {
		var msg event.WSMessage
		if err := json.Unmarshal(b, &msg); err != nil {
			slog.Warn("unmarshal WSMessage failed", "error", err)
			return nil // skip malformed messages
		}

		channel := centrifugo.ChannelFromRoomID(msg.RoomID)
		payload := map[string]interface{}{
			"event":   msg.Event,
			"payload": msg.Payload,
		}

		if err := cfgo.Publish(ctx, channel, payload); err != nil {
			slog.Error("centrifugo publish failed", "channel", channel, "error", err)
			return err
		}
		slog.Debug("published to centrifugo", "channel", channel, "event", msg.Event)
		return nil
	})
	if err != nil && err != context.Canceled {
		slog.Error("consume loop exited", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = healthSrv.Shutdown(shutdownCtx)
	slog.Info("worker-dispatch stopped")
}
