package api

import (
	"encoding/json"
	"net/http"

	"be-modami-no-service/config"
	"be-modami-no-service/pkg/centrifugo"

	"gitlab.com/lifegoeson-libs/pkg-gokit/response"
	"gitlab.com/lifegoeson-libs/pkg-logging/logger"
)

// AuthHandler groups auth-related HTTP handlers.
type AuthHandler struct {
	cfg *configs.Config
}

func NewAuthHandler(cfg *configs.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/noti-services/auth/centrifugo-token", h.CentrifugoToken)
}

// CentrifugoToken godoc
// @Summary Generate Centrifugo connection token
// @Description Generate a JWT token for client WebSocket connection to Centrifugo
// @Tags auth
// @Accept json
// @Produce json
// @Param body body object{user_id=string} true "User ID"
// @Success 200 {object} object{data=object{token=string}}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /auth/centrifugo-token [post]
func (h *AuthHandler) CentrifugoToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		response.BadRequest(w, "user_id is required")
		return
	}
	token, err := centrifugo.GenerateConnectionToken(h.cfg.Centrifugo.HMACSecret, req.UserID, h.cfg.Centrifugo.TokenTTL)
	if err != nil {
		l := logger.FromContext(r.Context())
		l.Error("failed to generate centrifugo token", err)
		response.InternalError(w, "failed to generate token")
		return
	}
	response.OK(w, map[string]string{"token": token})
}
