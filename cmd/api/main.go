// @title Modami Notification Service API
// @version 1.0
// @description REST API for the Modami notification service. Manages notifications, user preferences, device subscribers, and Centrifugo WebSocket tokens.
// @BasePath /v1/noti-services
// @schemes http https
// @produce json
// @consume json

// API: REST for core service — notifications, preferences, subscribers, Centrifugo token.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"be-modami-no-service/config"
	"be-modami-no-service/internal/api"
	mongostore "be-modami-no-service/internal/store/mongo"
	"be-modami-no-service/pkg/health"
	"be-modami-no-service/pkg/httputil"
	database "be-modami-no-service/pkg/storage/database/mongodb"

	"gitlab.com/lifegoeson-libs/pkg-logging/logger"

	"be-modami-no-service/docs"

	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
	ctx := context.Background()

	cfg, err := configs.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	docs.SwaggerInfo.Host = cfg.App.SwaggerHost

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

	// Register all API route groups
	api.RegisterAll(mux,
		api.NewAuthHandler(cfg),
		api.NewNotificationHandler(notificationStore),
		api.NewPreferenceHandler(preferenceStore),
		api.NewSubscriberHandler(subscriberStore),
	)

	// Apply middleware
	handler := httputil.Chain(mux,
		httputil.Recovery,
		httputil.RequestID,
		httputil.RequestLogging,
		httputil.CORS(cfg.App.AllowedOrigins),
	)

	srv := &http.Server{
		Addr:         cfg.Servers.APIAddr,
		Handler:      handler,
		ReadTimeout:  cfg.App.ParseReadTimeout(),
		WriteTimeout: cfg.App.ParseWriteTimeout(),
		IdleTimeout:  cfg.App.ParseIdleTimeout(),
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ParseShutdownTimeout())
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	l.Info("api stopped")
}
