// Worker WS: BRPOP notif:ws → publish to Redis PubSub channel → ws-gateway subscribes and broadcasts.
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

	err := q.Consume(ctx, cfg.Queue.WSKey, 5*time.Second, func(b []byte) error {
		var msg event.WSMessage
		if err := json.Unmarshal(b, &msg); err != nil {
			log.Printf("worker-ws unmarshal: %v", err)
			return nil
		}
		// Publish to channel so ws-gateway can broadcast
		if err := q.PublishWSDispatch(ctx, cfg.Queue.PubSubWSChan, msg); err != nil {
			log.Printf("worker-ws publish: %v", err)
			return err
		}
		return nil
	})
	if err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
