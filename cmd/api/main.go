// API: REST for core service — notifications, preferences, subscribers, Centrifugo token.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/techinsight/be-techinsights-notification-service/configs"
	"github.com/techinsight/be-techinsights-notification-service/internal/domain"
	"github.com/techinsight/be-techinsights-notification-service/internal/store"
	mongostore "github.com/techinsight/be-techinsights-notification-service/internal/store/mongo"
	"github.com/techinsight/be-techinsights-notification-service/pkg/centrifugo"
	"github.com/techinsight/be-techinsights-notification-service/pkg/health"
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

	// MongoDB connection
	mongoDB, err := database.NewMongoDB(database.MongoConfig{
		URI:      cfg.Database.MongoDB.URI,
		Database: cfg.Database.MongoDB.Database,
	})
	if err != nil {
		l.Error("failed to connect to MongoDB", err)
		os.Exit(1)
	}
	defer mongoDB.Close(context.Background())

	// Ensure indexes
	if err := mongostore.EnsureIndexes(ctx, mongoDB.Database); err != nil {
		l.Error("failed to ensure indexes", err)
	}

	// Stores
	notificationStore := mongostore.NewNotificationStore(mongoDB.Database)
	subscriberStore := mongostore.NewSubscriberStore(mongoDB.Database)
	preferenceStore := mongostore.NewPreferenceStore(mongoDB.Database)

	mux := http.NewServeMux()

	// Health endpoints
	checker := health.NewHandler(
		health.NewMongoChecker(mongoDB.Client),
	)
	mux.HandleFunc("GET /healthz", checker.Healthz)
	mux.HandleFunc("GET /readyz", checker.Readyz)

	// Centrifugo JWT token endpoint
	mux.HandleFunc("POST /api/v1/auth/centrifugo-token", centrifugoTokenHandler(cfg))

	// Notifications
	mux.HandleFunc("GET /api/v1/users/{userId}/notifications", listNotifications(notificationStore))
	mux.HandleFunc("GET /api/v1/notifications/{id}", getNotification(notificationStore))
	mux.HandleFunc("PATCH /api/v1/notifications/{id}/read", markRead(notificationStore))

	// Preferences
	mux.HandleFunc("GET /api/v1/users/{userId}/preferences", getPreferences(preferenceStore))
	mux.HandleFunc("PUT /api/v1/users/{userId}/preferences", setPreferences(preferenceStore))

	// Subscribers
	mux.HandleFunc("POST /api/v1/users/{userId}/subscribers", registerSubscriber(subscriberStore))
	mux.HandleFunc("DELETE /api/v1/users/{userId}/subscribers/{token}", deleteSubscriber(subscriberStore))

	srv := &http.Server{Addr: cfg.Servers.APIAddr, Handler: mux}
	go func() {
		l.Info("api listening on " + cfg.Servers.APIAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.Error("server error", err)
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	<-sigCtx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	l.Info("api stopped")
}

func centrifugoTokenHandler(cfg *configs.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID string `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
			http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
			return
		}
		token, err := centrifugo.GenerateConnectionToken(cfg.Centrifugo.HMACSecret, req.UserID, cfg.Centrifugo.TokenTTL)
		if err != nil {
			l := logger.FromContext(r.Context())
			l.Error("failed to generate centrifugo token", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": token})
	}
}

func listNotifications(ns store.NotificationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		list, err := ns.ListByUserID(r.Context(), userID, limit)
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
		json.NewEncoder(w).Encode(list)
	}
}

func getNotification(ns store.NotificationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		n, err := ns.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if n == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(n)
	}
}

func markRead(ns store.NotificationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		if err := ns.MarkRead(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func getPreferences(ps store.PreferenceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		if userID == "" {
			http.Error(w, "missing userId", http.StatusBadRequest)
			return
		}
		pref, err := ps.Get(r.Context(), userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pref)
	}
}

func setPreferences(ps store.PreferenceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		if userID == "" {
			http.Error(w, "missing userId", http.StatusBadRequest)
			return
		}
		var pref domain.Preference
		if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		pref.UserID = userID
		if err := ps.Set(r.Context(), &pref); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func registerSubscriber(ss store.SubscriberStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		if userID == "" {
			http.Error(w, "missing userId", http.StatusBadRequest)
			return
		}
		var sub domain.Subscriber
		if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sub.UserID = userID
		if err := ss.Upsert(r.Context(), &sub); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}
}

func deleteSubscriber(ss store.SubscriberStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		token := r.PathValue("token")
		if userID == "" || token == "" {
			http.Error(w, "missing userId or token", http.StatusBadRequest)
			return
		}
		if err := ss.DeleteByToken(r.Context(), userID, token); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
