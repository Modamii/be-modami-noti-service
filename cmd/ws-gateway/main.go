// WS Gateway: Centrifugo proxy handler for connect/subscribe/publish callbacks.
//
// Architecture:
//   Client ──WebSocket──▶ Centrifugo ──proxy HTTP──▶ this service
//
// Centrifugo delegates auth, channel authorization, and publish validation
// to this service via its proxy protocol. This avoids a double-hop WebSocket
// proxy and lets Centrifugo handle connection management and fanout natively.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/techinsight/be-techinsights-notification-service/configs"
	"github.com/techinsight/be-techinsights-notification-service/internal/gateway"
	"github.com/techinsight/be-techinsights-notification-service/pkg/centrifugo"
	"github.com/techinsight/be-techinsights-notification-service/pkg/health"
	"github.com/techinsight/be-techinsights-notification-service/pkg/httputil"
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

	cfgoClient := centrifugo.NewClient(cfg.Centrifugo.APIURL, cfg.Centrifugo.APIKey)

	mux := http.NewServeMux()

	// Health endpoints
	checker := health.NewHandler(
		health.NewCentrifugoChecker(cfgoClient),
	)
	mux.HandleFunc("GET /healthz", checker.Healthz)
	mux.HandleFunc("GET /readyz", checker.Readyz)

	// Centrifugo proxy callbacks
	proxyHandler := gateway.NewProxyHandler(cfg.Centrifugo.HMACSecret)
	proxyHandler.RegisterRoutes(mux)

	handler := httputil.Chain(mux,
		httputil.Recovery,
		httputil.RequestID,
		httputil.RequestLogging,
	)

	srv := &http.Server{
		Addr:         cfg.Servers.GatewayAddr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		l.Info("ws-gateway listening on " + cfg.Servers.GatewayAddr)
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
	l.Info("ws-gateway stopped")
}
