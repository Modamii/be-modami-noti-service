package api

import (
	"encoding/json"
	"net/http"

	"be-modami-no-service/internal/domain"
	"be-modami-no-service/internal/store"

	"gitlab.com/lifegoeson-libs/pkg-gokit/response"
)

// SubscriberHandler groups subscriber-related HTTP handlers.
type SubscriberHandler struct {
	store store.SubscriberStore
}

func NewSubscriberHandler(s store.SubscriberStore) *SubscriberHandler {
	return &SubscriberHandler{store: s}
}

func (h *SubscriberHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/noti-services/users/{userId}/subscribers", h.Register)
	mux.HandleFunc("DELETE /v1/noti-services/users/{userId}/subscribers/{token}", h.Delete)
}

// Register godoc
// @Summary Register a device subscriber
// @Description Register a device for push notifications (FCM/Web Push)
// @Tags subscribers
// @Accept json
// @Param userId path string true "User ID"
// @Param body body domain.Subscriber true "Subscriber details"
// @Success 201 "Created"
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /users/{userId}/subscribers [post]
func (h *SubscriberHandler) Register(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		response.BadRequest(w, "missing userId")
		return
	}
	var sub domain.Subscriber
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	sub.UserID = userID
	if err := h.store.Upsert(r.Context(), &sub); err != nil {
		response.InternalError(w, "failed to register subscriber")
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// Delete godoc
// @Summary Delete a device subscriber
// @Description Unregister a device from push notifications
// @Tags subscribers
// @Param userId path string true "User ID"
// @Param token path string true "Device token"
// @Success 204 "No Content"
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /users/{userId}/subscribers/{token} [delete]
func (h *SubscriberHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	token := r.PathValue("token")
	if userID == "" || token == "" {
		response.BadRequest(w, "missing userId or token")
		return
	}
	if err := h.store.DeleteByToken(r.Context(), userID, token); err != nil {
		response.InternalError(w, "failed to delete subscriber")
		return
	}
	response.NoContent(w)
}
