package api

import (
	"encoding/json"
	"net/http"

	"be-modami-no-service/internal/domain"
	"be-modami-no-service/internal/store"

	"gitlab.com/lifegoeson-libs/pkg-gokit/response"
)

// PreferenceHandler groups preference-related HTTP handlers.
type PreferenceHandler struct {
	store store.PreferenceStore
}

func NewPreferenceHandler(s store.PreferenceStore) *PreferenceHandler {
	return &PreferenceHandler{store: s}
}

func (h *PreferenceHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/noti-services/users/{userId}/preferences", h.Get)
	mux.HandleFunc("PUT /v1/noti-services/users/{userId}/preferences", h.Set)
}

// Get godoc
// @Summary Get user preferences
// @Description Get notification preferences for a user
// @Tags preferences
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} response.Response{data=domain.Preference}
// @Failure 500 {object} response.Response
// @Router /users/{userId}/preferences [get]
func (h *PreferenceHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		response.BadRequest(w, "missing userId")
		return
	}
	pref, err := h.store.Get(r.Context(), userID)
	if err != nil {
		response.InternalError(w, "failed to get preferences")
		return
	}
	response.OK(w, pref)
}

// Set godoc
// @Summary Update user preferences
// @Description Update notification preferences for a user
// @Tags preferences
// @Accept json
// @Param userId path string true "User ID"
// @Param body body domain.Preference true "Preference settings"
// @Success 204 "No Content"
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /users/{userId}/preferences [put]
func (h *PreferenceHandler) Set(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		response.BadRequest(w, "missing userId")
		return
	}
	var pref domain.Preference
	if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	pref.UserID = userID
	if err := h.store.Set(r.Context(), &pref); err != nil {
		response.InternalError(w, "failed to update preferences")
		return
	}
	response.NoContent(w)
}
