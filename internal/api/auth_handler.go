package api

import (
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
// @Summary Generate a Centrifugo connection token for the caller
// @Description Mints a short-lived WebSocket token for the authenticated user. The subject
// @Description comes from the access token, so a token can never be requested for someone else.
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{data=object{token=string,expires_in=int}}
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /auth/centrifugo-token [post]
func (h *AuthHandler) CentrifugoToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}

	token, err := centrifugo.GenerateConnectionToken(h.cfg.Centrifugo.HMACSecret, userID, h.cfg.Centrifugo.TokenTTL)
	if err != nil {
		logger.FromContext(r.Context()).Error("failed to generate centrifugo token", err)
		response.InternalError(w, "failed to generate token")
		return
	}

	// expires_in lets the client schedule a refresh before Centrifugo drops the socket.
	response.OK(w, map[string]any{
		"token":      token,
		"expires_in": h.cfg.Centrifugo.TokenTTL,
	})
}
