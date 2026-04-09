package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"be-modami-no-service/pkg/centrifugo"

	"github.com/golang-jwt/jwt/v5"
	"gitlab.com/lifegoeson-libs/pkg-logging/logger"
)

// ProxyHandler handles Centrifugo proxy callbacks: connect, subscribe, publish.
type ProxyHandler struct {
	hmacSecret     string
	publishLimiter *RateLimiter
}

func NewProxyHandler(hmacSecret string) *ProxyHandler {
	return &ProxyHandler{
		hmacSecret: hmacSecret,
		// 10 publishes per second, burst of 20 per user
		publishLimiter: NewRateLimiter(10, 20, time.Second),
	}
}

// RegisterRoutes registers proxy callback endpoints.
// These URLs must match the Centrifugo proxy config.
func (h *ProxyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /centrifugo/connect", h.Connect)
	mux.HandleFunc("POST /centrifugo/subscribe", h.Subscribe)
	mux.HandleFunc("POST /centrifugo/publish", h.Publish)
}

// Connect validates the client's JWT and returns the user ID.
// Centrifugo calls this when a client opens a WebSocket connection.
func (h *ProxyHandler) Connect(w http.ResponseWriter, r *http.Request) {
	var req centrifugo.ProxyConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProxyError(w, centrifugo.ProxyErrorInternal, "invalid request")
		return
	}

	// Extract JWT from the connect data
	var connectData struct {
		Token string `json:"token"`
	}
	if req.Data != nil {
		_ = json.Unmarshal(req.Data, &connectData)
	}

	if connectData.Token == "" {
		writeProxyError(w, centrifugo.ProxyErrorUnauthorized, "token required")
		return
	}

	claims, err := h.parseJWT(connectData.Token)
	if err != nil {
		logger.FromContext(r.Context()).Error("jwt validation failed", err)
		writeProxyError(w, centrifugo.ProxyErrorUnauthorized, "invalid token")
		return
	}

	userID, _ := claims["sub"].(string)
	if userID == "" {
		writeProxyError(w, centrifugo.ProxyErrorUnauthorized, "missing sub claim")
		return
	}

	// Auto-subscribe user to their personal notification channel
	personalChannel := centrifugo.ChannelFromRoomID("user:" + userID)

	expireAt := int64(0)
	if exp, ok := claims["exp"].(float64); ok {
		expireAt = int64(exp)
	}

	writeProxyResult(w, &centrifugo.ProxyConnectResult{
		User:     userID,
		ExpireAt: expireAt,
		Channels: []string{personalChannel},
	})
}

// Subscribe checks whether the user is allowed to subscribe to the requested channel.
// Channel format: "notifications:user:{userID}" — user can only subscribe to their own channel.
func (h *ProxyHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req centrifugo.ProxySubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProxyError(w, centrifugo.ProxyErrorInternal, "invalid request")
		return
	}

	if !isChannelAllowed(req.User, req.Channel) {
		writeProxyError(w, centrifugo.ProxyErrorForbidden, "not allowed to subscribe to this channel")
		return
	}

	writeProxyResult(w, &centrifugo.ProxySubscribeResult{})
}

// Publish rate-limits client-side publishes.
// Most notifications are server-initiated, but this guards against abuse if client publish is enabled.
func (h *ProxyHandler) Publish(w http.ResponseWriter, r *http.Request) {
	var req centrifugo.ProxyPublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProxyError(w, centrifugo.ProxyErrorInternal, "invalid request")
		return
	}

	if !h.publishLimiter.Allow(req.User) {
		writeProxyError(w, centrifugo.ProxyErrorTooMany, "rate limit exceeded")
		return
	}

	if !isChannelAllowed(req.User, req.Channel) {
		writeProxyError(w, centrifugo.ProxyErrorForbidden, "not allowed to publish to this channel")
		return
	}

	writeProxyResult(w, &centrifugo.ProxyPublishResult{})
}

// isChannelAllowed checks if a user can access the given channel.
// Rule: "notifications:user:{userID}" — only the owner can subscribe/publish.
func isChannelAllowed(userID, channel string) bool {
	// Personal notification channel
	expected := centrifugo.ChannelFromRoomID("user:" + userID)
	if channel == expected {
		return true
	}

	// Public/topic channels (e.g. "notifications:topic:announcements") are open
	if strings.HasPrefix(channel, "notifications:topic:") {
		return true
	}

	return false
}

func (h *ProxyHandler) parseJWT(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.hmacSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	return claims, nil
}

func writeProxyResult(w http.ResponseWriter, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(centrifugo.ProxyResponse{Result: result})
}

func writeProxyError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(centrifugo.ProxyResponse{
		Error: &centrifugo.ProxyError{Code: code, Message: message},
	})
}
