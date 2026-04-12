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

// ProxyHandler handles Centrifugo proxy callbacks for the "noti" namespace only.
// The "chat" namespace is proxied to the chat service's own ws-gateway.
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
// Centrifugo calls this on every new WebSocket connection (shared across namespaces).
// On connect, the user is auto-subscribed to their personal noti channel.
func (h *ProxyHandler) Connect(w http.ResponseWriter, r *http.Request) {
	var req centrifugo.ProxyConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProxyError(w, centrifugo.ProxyErrorInternal, "invalid request")
		return
	}

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

	expireAt := int64(0)
	if exp, ok := claims["exp"].(float64); ok {
		expireAt = int64(exp)
	}

	// Auto-subscribe to personal noti channel: "noti:user:{userID}"
	// Client separately subscribes to chat channels after connect.
	writeProxyResult(w, &centrifugo.ProxyConnectResult{
		User:     userID,
		ExpireAt: expireAt,
		Channels: []string{centrifugo.NotiChannel(userID)},
	})
}

// Subscribe checks whether the user is allowed to subscribe to the requested noti channel.
// Only called for "noti:*" channels — chat namespace has its own proxy in the chat service.
func (h *ProxyHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req centrifugo.ProxySubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProxyError(w, centrifugo.ProxyErrorInternal, "invalid request")
		return
	}

	if !isNotiChannelAllowed(req.User, req.Channel) {
		writeProxyError(w, centrifugo.ProxyErrorForbidden, "not allowed to subscribe to this channel")
		return
	}

	writeProxyResult(w, &centrifugo.ProxySubscribeResult{})
}

// Publish rate-limits client-side publishes to noti channels.
// Notifications are server-initiated; this guards against abuse.
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

	if !isNotiChannelAllowed(req.User, req.Channel) {
		writeProxyError(w, centrifugo.ProxyErrorForbidden, "not allowed to publish to this channel")
		return
	}

	writeProxyResult(w, &centrifugo.ProxyPublishResult{})
}

// isNotiChannelAllowed checks noti namespace access rules:
//   - "noti:user:{userID}"  — only the owner
//   - "noti:topic:*"        — public broadcast channels
func isNotiChannelAllowed(userID, channel string) bool {
	if channel == centrifugo.NotiChannel(userID) {
		return true
	}
	if strings.HasPrefix(channel, centrifugo.NamespaceNoti+":topic:") {
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

func writeProxyResult(w http.ResponseWriter, result any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(centrifugo.ProxyResponse{Result: result})
}

func writeProxyError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(centrifugo.ProxyResponse{
		Error: &centrifugo.ProxyError{Code: code, Message: message},
	})
}
