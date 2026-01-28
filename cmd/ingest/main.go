// Ingest: Kafka consumer (or HTTP webhook). Parse NotificationEvent, dispatch by Identity.
// Copy your Kafka consumer from config/kafka and wire RunConsumer here.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/redis/go-redis/v9"
	"github.com/techinsight/be-techinsights-notification-service/configs"
	"github.com/techinsight/be-techinsights-notification-service/internal/handlers"
	"github.com/techinsight/be-techinsights-notification-service/internal/queue"
	"github.com/techinsight/be-techinsights-notification-service/pkg/contract"
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
	reg := NewRegistryWithHandlers(q, cfg.Queue.WSKey, cfg.Queue.PushKey)

	// HTTP webhook for testing (contract envelope)
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})

	srv := &http.Server{Addr: cfg.Servers.IngestAddr, Handler: mux}
	go func() {
		log.Printf("ingest listening on %s", cfg.Servers.IngestAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// TODO: start Kafka consumer from config/kafka here, e.g.:
	// go kafka.RunConsumer(ctx, brokers, topic, func(msg []byte) error {
	//   var e contract.NotificationEvent
	//   if err := json.Unmarshal(msg, &e); err != nil { return err }
	//   h, ok := reg.Get(e.Identity); if !ok { return nil }
	//   return h(ctx, &e)
	// })

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	<-ctx.Done()
	stop()
	_ = srv.Shutdown(context.Background())
}

func NewRegistryWithHandlers(q *queue.Queue, queueWS, queuePush string) handlers.Registry {
	reg := handlers.NewRegistry()
	reg.Register(contract.ContentPublished, handlers.ContentPublished(q, queueWS, queuePush))
	reg.Register(contract.CommentCreated, handlers.CommentCreated(q, queueWS, queuePush))
	return reg
}
