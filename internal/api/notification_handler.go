package api

import (
	"net/http"
	"strconv"

	"be-modami-no-service/internal/domain"
	"be-modami-no-service/internal/store"
	"be-modami-no-service/pkg/httputil"
)

// NotificationHandler groups all notification-related HTTP handlers.
type NotificationHandler struct {
	store store.NotificationStore
}

func NewNotificationHandler(s store.NotificationStore) *NotificationHandler {
	return &NotificationHandler{store: s}
}

// RegisterRoutes registers notification routes on the given mux.
func (h *NotificationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/noti-services/users/{userId}/notifications", h.List)
	mux.HandleFunc("GET /v1/noti-services/users/{userId}/notifications/unread-count", h.CountUnread)
	mux.HandleFunc("PATCH /v1/noti-services/users/{userId}/notifications/read-all", h.MarkAllRead)
	mux.HandleFunc("GET /v1/noti-services/notifications/{id}", h.GetByID)
	mux.HandleFunc("PATCH /v1/noti-services/notifications/{id}/read", h.MarkRead)
	mux.HandleFunc("DELETE /v1/noti-services/notifications/{id}", h.Delete)
}

// List godoc
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
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.store.ListByUserIDPaginated(r.Context(), userID, store.PaginationParams{
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

// GetByID godoc
// @Summary Get a notification by ID
// @Description Get a single notification by its ID
// @Tags notifications
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} httputil.Response{data=domain.Notification}
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /notifications/{id} [get]
func (h *NotificationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.ErrBadRequest(w, "missing id")
		return
	}
	var n *domain.Notification
	var err error
	n, err = h.store.GetByID(r.Context(), id)
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

// MarkRead godoc
// @Summary Mark a notification as read
// @Description Mark a single notification as read by ID
// @Tags notifications
// @Param id path string true "Notification ID"
// @Success 204 "No Content"
// @Failure 500 {object} httputil.Response
// @Router /notifications/{id}/read [patch]
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.ErrBadRequest(w, "missing id")
		return
	}
	if err := h.store.MarkRead(r.Context(), id); err != nil {
		httputil.ErrInternal(w, "failed to mark notification as read")
		return
	}
	httputil.RespondNoContent(w)
}

// MarkAllRead godoc
// @Summary Mark all notifications as read
// @Description Mark all unread notifications as read for a user
// @Tags notifications
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} httputil.Response{data=object{updated=int64}}
// @Failure 500 {object} httputil.Response
// @Router /users/{userId}/notifications/read-all [patch]
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		httputil.ErrBadRequest(w, "missing userId")
		return
	}
	count, err := h.store.MarkAllRead(r.Context(), userID)
	if err != nil {
		httputil.ErrInternal(w, "failed to mark all as read")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, map[string]int64{"updated": count}, nil)
}

// Delete godoc
// @Summary Delete a notification
// @Description Delete a notification by ID
// @Tags notifications
// @Param id path string true "Notification ID"
// @Success 204 "No Content"
// @Failure 500 {object} httputil.Response
// @Router /notifications/{id} [delete]
func (h *NotificationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httputil.ErrBadRequest(w, "missing id")
		return
	}
	if err := h.store.Delete(r.Context(), id); err != nil {
		httputil.ErrInternal(w, "failed to delete notification")
		return
	}
	httputil.RespondNoContent(w)
}

// CountUnread godoc
// @Summary Count unread notifications
// @Description Get the count of unread notifications for a user
// @Tags notifications
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} httputil.Response{data=object{count=int64}}
// @Failure 500 {object} httputil.Response
// @Router /users/{userId}/notifications/unread-count [get]
func (h *NotificationHandler) CountUnread(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		httputil.ErrBadRequest(w, "missing userId")
		return
	}
	count, err := h.store.CountUnread(r.Context(), userID)
	if err != nil {
		httputil.ErrInternal(w, "failed to count unread notifications")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, map[string]int64{"count": count}, nil)
}
