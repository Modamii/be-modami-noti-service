// WS Gateway: gorilla/websocket, join room by userId/topic. Subscribes to Redis PubSub and broadcasts.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/techinsight/be-techinsights-notification-service/configs"
	"github.com/techinsight/be-techinsights-notification-service/internal/queue"
	"github.com/techinsight/be-techinsights-notification-service/internal/ws"
	"github.com/techinsight/be-techinsights-notification-service/pkg/event"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

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
	hub := ws.NewHub()

	// Subscribe to Redis: worker-ws publishes here; we broadcast to room
	go func() {
		ctx := context.Background()
		_ = q.SubscribeWSDispatch(ctx, cfg.Queue.PubSubWSChan, func(b []byte) error {
			var msg event.WSMessage
			if err := json.Unmarshal(b, &msg); err != nil {
				return err
			}
			hub.Broadcast(msg.RoomID, msg.Event, msg.Payload)
			return nil
		})
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade: %v", err)
			return
		}
		// Expect first message: {"action":"subscribe","user_id":"...","topic":""}
		var sub struct {
			Action string `json:"action"`
			UserID string `json:"user_id"`
			Topic  string `json:"topic"`
		}
		if err := conn.ReadJSON(&sub); err != nil {
			conn.Close()
			return
		}
		if sub.Action != "subscribe" {
			conn.Close()
			return
		}
		c := hub.Register(conn, sub.UserID, sub.Topic)
		defer hub.Unregister(c)
		// Block reading (close on client disconnect)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

	srv := &http.Server{Addr: cfg.Servers.WSGatewayAddr, Handler: mux}
	go func() {
		log.Printf("ws-gateway listening on %s", cfg.Servers.WSGatewayAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	<-ctx.Done()
	stop()
	_ = srv.Shutdown(context.Background())
}
