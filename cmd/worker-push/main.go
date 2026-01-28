// Worker Push: BRPOP notif:push → FCM/Web Push. Stub: log payload; wire FCM later.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/techinsight/be-techinsights-notification-service/configs"
	"github.com/techinsight/be-techinsights-notification-service/internal/queue"
	"github.com/techinsight/be-techinsights-notification-service/pkg/event"
)

func main() {
	cfg, err := configs.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	rdb := redis.NewClient(configs.RedisOptions(cfg))
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis ping: %v", err)
	}
	q := queue.New(rdb)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := q.Consume(ctx, cfg.Queue.PushKey, 5*time.Second, func(b []byte) error {
		var msg event.PushMessage
		if err := json.Unmarshal(b, &msg); err != nil {
			log.Printf("worker-push unmarshal: %v", err)
			return nil
		}
		// Stub: log payload. Replace with FCM + Web Push when ready.
		log.Printf("[push stub] title=%q body=%q link=%q tokens=%v", msg.Title, msg.Body, msg.Link, msg.DeviceTokens)
		return nil
	})
	if err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
