package api

import (
	"encoding/json"
	"net/http"

	"github.com/techinsight/be-techinsights-notification-service/internal/domain"
	"github.com/techinsight/be-techinsights-notification-service/internal/store"
	"github.com/techinsight/be-techinsights-notification-service/pkg/httputil"
)

// PreferenceHandler groups preference-related HTTP handlers.
type PreferenceHandler struct {
	store store.PreferenceStore
}

func NewPreferenceHandler(s store.PreferenceStore) *PreferenceHandler {
	return &PreferenceHandler{store: s}
}

func (h *PreferenceHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/users/{userId}/preferences", h.Get)
	mux.HandleFunc("PUT /api/v1/users/{userId}/preferences", h.Set)
}

// Get godoc
// @Summary Get user preferences
// @Description Get notification preferences for a user
// @Tags preferences
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} httputil.Response{data=domain.Preference}
// @Failure 500 {object} httputil.Response
// @Router /users/{userId}/preferences [get]
func (h *PreferenceHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		httputil.ErrBadRequest(w, "missing userId")
		return
	}
	pref, err := h.store.Get(r.Context(), userID)
	if err != nil {
		httputil.ErrInternal(w, "failed to get preferences")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, pref, nil)
}

// Set godoc
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
func (h *PreferenceHandler) Set(w http.ResponseWriter, r *http.Request) {
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
	if err := h.store.Set(r.Context(), &pref); err != nil {
		httputil.ErrInternal(w, "failed to update preferences")
		return
	}
	httputil.RespondNoContent(w)
}
