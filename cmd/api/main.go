// @title TechInsight Notification Service API
// @version 1.0
// @description REST API for the TechInsight notification service. Manages notifications, user preferences, device subscribers, and Centrifugo WebSocket tokens.
// @BasePath /api/v1
// @schemes http https
// @produce json
// @consume json

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
	"github.com/techinsight/be-techinsights-notification-service/pkg/httputil"
	database "github.com/techinsight/be-techinsights-notification-service/pkg/storage/database/mongodb"
	"gitlab.com/lifegoeson-libs/pkg-logging/logger"

	httpSwagger "github.com/swaggo/http-swagger"
	_ "github.com/techinsight/be-techinsights-notification-service/docs"
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

	// Swagger
	mux.Handle("GET /swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// Centrifugo JWT token endpoint
	mux.HandleFunc("POST /api/v1/auth/centrifugo-token", centrifugoTokenHandler(cfg))

	// Notifications
	mux.HandleFunc("GET /api/v1/users/{userId}/notifications", listNotifications(notificationStore))
	mux.HandleFunc("GET /api/v1/users/{userId}/notifications/unread-count", countUnread(notificationStore))
	mux.HandleFunc("PATCH /api/v1/users/{userId}/notifications/read-all", markAllRead(notificationStore))
	mux.HandleFunc("GET /api/v1/notifications/{id}", getNotification(notificationStore))
	mux.HandleFunc("PATCH /api/v1/notifications/{id}/read", markRead(notificationStore))
	mux.HandleFunc("DELETE /api/v1/notifications/{id}", deleteNotification(notificationStore))

	// Preferences
	mux.HandleFunc("GET /api/v1/users/{userId}/preferences", getPreferences(preferenceStore))
	mux.HandleFunc("PUT /api/v1/users/{userId}/preferences", setPreferences(preferenceStore))

	// Subscribers
	mux.HandleFunc("POST /api/v1/users/{userId}/subscribers", registerSubscriber(subscriberStore))
	mux.HandleFunc("DELETE /api/v1/users/{userId}/subscribers/{token}", deleteSubscriber(subscriberStore))

	// Apply middleware
	handler := httputil.Chain(mux,
		httputil.Recovery,
		httputil.RequestID,
		httputil.RequestLogging,
		httputil.CORS(cfg.App.CORSOrigins),
	)

	srv := &http.Server{
		Addr:         cfg.Servers.APIAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
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

// centrifugoTokenHandler godoc
// @Summary Generate Centrifugo connection token
// @Description Generate a JWT token for client WebSocket connection to Centrifugo
// @Tags auth
// @Accept json
// @Produce json
// @Param body body object{user_id=string} true "User ID"
// @Success 200 {object} object{data=object{token=string}}
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /auth/centrifugo-token [post]
func centrifugoTokenHandler(cfg *configs.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID string `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
			httputil.ErrBadRequest(w, "user_id is required")
			return
		}
		token, err := centrifugo.GenerateConnectionToken(cfg.Centrifugo.HMACSecret, req.UserID, cfg.Centrifugo.TokenTTL)
		if err != nil {
			l := logger.FromContext(r.Context())
			l.Error("failed to generate centrifugo token", err)
			httputil.ErrInternal(w, "failed to generate token")
			return
		}
		httputil.RespondJSON(w, http.StatusOK, map[string]string{"token": token}, nil)
	}
}

// listNotifications godoc
// @Summary List user notifications
// @Description Get paginated list of notifications for a user
// @Tags notifications
// @Produce json
// @Param userId path string true "User ID"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param unread_only query bool false "Filter unread only"
// @Success 200 {object} httputil.Response{data=[]domain.Notification,meta=httputil.Meta}
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /users/{userId}/notifications [get]
func listNotifications(ns store.NotificationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		if userID == "" {
			httputil.ErrBadRequest(w, "missing userId")
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page <= 0 {
			page = 1
		}
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if perPage <= 0 {
			perPage = 20
		}
		unreadOnly := r.URL.Query().Get("unread_only") == "1" || r.URL.Query().Get("unread_only") == "true"

		result, err := ns.ListByUserIDPaginated(r.Context(), userID, store.PaginationParams{
			Page:       page,
			PerPage:    perPage,
			UnreadOnly: unreadOnly,
		})
		if err != nil {
			httputil.ErrInternal(w, "failed to list notifications")
			return
		}

		httputil.RespondJSON(w, http.StatusOK, result.Items, &httputil.Meta{
			Total:      result.Total,
			Page:       result.Page,
			PerPage:    result.PerPage,
			TotalPages: result.TotalPages,
			HasMore:    result.HasMore,
		})
	}
}

// getNotification godoc
// @Summary Get a notification by ID
// @Description Get a single notification by its ID
// @Tags notifications
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} httputil.Response{data=domain.Notification}
// @Failure 400 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /notifications/{id} [get]
func getNotification(ns store.NotificationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			httputil.ErrBadRequest(w, "missing id")
			return
		}
		n, err := ns.GetByID(r.Context(), id)
		if err != nil {
			httputil.ErrInternal(w, "failed to get notification")
			return
		}
		if n == nil {
			httputil.ErrNotFound(w, "notification not found")
			return
		}
		httputil.RespondJSON(w, http.StatusOK, n, nil)
	}
}

// markRead godoc
// @Summary Mark a notification as read
// @Description Mark a single notification as read by ID
// @Tags notifications
// @Param id path string true "Notification ID"
// @Success 204 "No Content"
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /notifications/{id}/read [patch]
func markRead(ns store.NotificationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			httputil.ErrBadRequest(w, "missing id")
			return
		}
		if err := ns.MarkRead(r.Context(), id); err != nil {
			httputil.ErrInternal(w, "failed to mark notification as read")
			return
		}
		httputil.RespondNoContent(w)
	}
}

// markAllRead godoc
// @Summary Mark all notifications as read
// @Description Mark all unread notifications as read for a user
// @Tags notifications
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} httputil.Response{data=object{updated=int64}}
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /users/{userId}/notifications/read-all [patch]
func markAllRead(ns store.NotificationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		if userID == "" {
			httputil.ErrBadRequest(w, "missing userId")
			return
		}
		count, err := ns.MarkAllRead(r.Context(), userID)
		if err != nil {
			httputil.ErrInternal(w, "failed to mark all as read")
			return
		}
		httputil.RespondJSON(w, http.StatusOK, map[string]int64{"updated": count}, nil)
	}
}

// deleteNotification godoc
// @Summary Delete a notification
// @Description Delete a notification by ID
// @Tags notifications
// @Param id path string true "Notification ID"
// @Success 204 "No Content"
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /notifications/{id} [delete]
func deleteNotification(ns store.NotificationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			httputil.ErrBadRequest(w, "missing id")
			return
		}
		if err := ns.Delete(r.Context(), id); err != nil {
			httputil.ErrInternal(w, "failed to delete notification")
			return
		}
		httputil.RespondNoContent(w)
	}
}

// countUnread godoc
// @Summary Count unread notifications
// @Description Get the count of unread notifications for a user
// @Tags notifications
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} httputil.Response{data=object{count=int64}}
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /users/{userId}/notifications/unread-count [get]
func countUnread(ns store.NotificationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		if userID == "" {
			httputil.ErrBadRequest(w, "missing userId")
			return
		}
		count, err := ns.CountUnread(r.Context(), userID)
		if err != nil {
			httputil.ErrInternal(w, "failed to count unread notifications")
			return
		}
		httputil.RespondJSON(w, http.StatusOK, map[string]int64{"count": count}, nil)
	}
}

// getPreferences godoc
// @Summary Get user preferences
// @Description Get notification preferences for a user
// @Tags preferences
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} httputil.Response{data=domain.Preference}
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /users/{userId}/preferences [get]
func getPreferences(ps store.PreferenceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		if userID == "" {
			httputil.ErrBadRequest(w, "missing userId")
			return
		}
		pref, err := ps.Get(r.Context(), userID)
		if err != nil {
			httputil.ErrInternal(w, "failed to get preferences")
			return
		}
		httputil.RespondJSON(w, http.StatusOK, pref, nil)
	}
}

// setPreferences godoc
// @Summary Update user preferences
// @Description Update notification preferences for a user
// @Tags preferences
// @Accept json
// @Param userId path string true "User ID"
// @Param body body domain.Preference true "Preference settings"
// @Success 204 "No Content"
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /users/{userId}/preferences [put]
func setPreferences(ps store.PreferenceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		if userID == "" {
			httputil.ErrBadRequest(w, "missing userId")
			return
		}
		var pref domain.Preference
		if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
			httputil.ErrBadRequest(w, "invalid request body")
			return
		}
		pref.UserID = userID
		if err := ps.Set(r.Context(), &pref); err != nil {
			httputil.ErrInternal(w, "failed to update preferences")
			return
		}
		httputil.RespondNoContent(w)
	}
}

// registerSubscriber godoc
// @Summary Register a device subscriber
// @Description Register a device for push notifications (FCM/Web Push)
// @Tags subscribers
// @Accept json
// @Param userId path string true "User ID"
// @Param body body domain.Subscriber true "Subscriber details"
// @Success 201 "Created"
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /users/{userId}/subscribers [post]
func registerSubscriber(ss store.SubscriberStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		if userID == "" {
			httputil.ErrBadRequest(w, "missing userId")
			return
		}
		var sub domain.Subscriber
		if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
			httputil.ErrBadRequest(w, "invalid request body")
			return
		}
		sub.UserID = userID
		if err := ss.Upsert(r.Context(), &sub); err != nil {
			httputil.ErrInternal(w, "failed to register subscriber")
			return
		}
		w.WriteHeader(http.StatusCreated)
	}
}

// deleteSubscriber godoc
// @Summary Delete a device subscriber
// @Description Unregister a device from push notifications
// @Tags subscribers
// @Param userId path string true "User ID"
// @Param token path string true "Device token"
// @Success 204 "No Content"
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /users/{userId}/subscribers/{token} [delete]
func deleteSubscriber(ss store.SubscriberStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("userId")
		token := r.PathValue("token")
		if userID == "" || token == "" {
			httputil.ErrBadRequest(w, "missing userId or token")
			return
		}
		if err := ss.DeleteByToken(r.Context(), userID, token); err != nil {
			httputil.ErrInternal(w, "failed to delete subscriber")
			return
		}
		httputil.RespondNoContent(w)
	}
}
