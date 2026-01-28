// API: REST for core service — notifications by userId, preferences, subscribers.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"

	"github.com/techinsight/be-techinsights-notification-service/configs"
	"github.com/techinsight/be-techinsights-notification-service/internal/store/memory"
)

func main() {
	cfg, err := configs.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	notificationStore := memory.NewNotificationStore()

	mux := http.NewServeMux()

	// GET list notifications by userId (for core service)
	mux.HandleFunc("GET /api/v1/users/{userId}/notifications", func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		if userID == "" {
			http.Error(w, "missing userId", http.StatusBadRequest)
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 20
		}
		unreadOnly := r.URL.Query().Get("unread_only") == "1" || r.URL.Query().Get("unread_only") == "true"

		list, err := notificationStore.ListByUserID(r.Context(), userID, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if unreadOnly {
			filtered := list[:0]
			for _, n := range list {
				if !n.Read {
					filtered = append(filtered, n)
				}
			}
			list = filtered
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	})

	// GET single notification
	mux.HandleFunc("GET /api/v1/notifications/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		n, err := notificationStore.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if n == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(n)
	})

	// PATCH mark notification as read (optional)
	mux.HandleFunc("PATCH /api/v1/notifications/{id}/read", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		if err := notificationStore.MarkRead(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Preferences (placeholder; wire PreferenceStore when ready)
	mux.HandleFunc("GET /api/v1/users/{userId}/preferences", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})
	mux.HandleFunc("PUT /api/v1/users/{userId}/preferences", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Subscribers (placeholder; wire SubscriberStore when ready)
	mux.HandleFunc("POST /api/v1/users/{userId}/subscribers", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("DELETE /api/v1/users/{userId}/subscribers/{token}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	srv := &http.Server{Addr: cfg.Servers.APIAddr, Handler: mux}
	go func() {
		log.Printf("api listening on %s", cfg.Servers.APIAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	<-ctx.Done()
	stop()
	_ = srv.Shutdown(context.Background())
}
