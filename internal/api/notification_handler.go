package api

import (
	"net/http"
	"strconv"

	"be-modami-no-service/internal/store"

	"gitlab.com/lifegoeson-libs/pkg-gokit/response"
)

// NotificationHandler groups all notification-related HTTP handlers.
type NotificationHandler struct {
	store    store.NotificationStore
	readSync ReadSyncPublisher // nil-safe: without it, other devices catch up on refetch
}

func NewNotificationHandler(s store.NotificationStore) *NotificationHandler {
	return &NotificationHandler{store: s}
}

// WithReadSync makes read state propagate to the user's other devices.
func (h *NotificationHandler) WithReadSync(p ReadSyncPublisher) *NotificationHandler {
	h.readSync = p
	return h
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
// @Success 200 {object} response.Response{data=[]domain.Notification}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /users/{userId}/notifications [get]
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
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
		response.InternalError(w, "failed to list notifications")
		return
	}

	response.OKWithPagination(w, result.Items, response.Pagination{
		TotalItems: int(result.Total),
		Page:       result.Page,
		PerPage:    result.PerPage,
		TotalPages: result.TotalPages,
		HasNext:    result.HasMore,
	})
}

// GetByID godoc
// @Summary Get a notification by ID
// @Description Get a single notification by its ID
// @Tags notifications
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} response.Response{data=domain.Notification}
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /notifications/{id} [get]
func (h *NotificationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		response.BadRequest(w, "missing id")
		return
	}
	n, err := h.store.GetByID(r.Context(), userID, id)
	if err != nil {
		response.InternalError(w, "failed to get notification")
		return
	}
	// A notification owned by someone else is reported as missing rather than
	// forbidden, so ids cannot be probed for existence.
	if n == nil {
		response.NotFound(w, "notification not found")
		return
	}
	response.OK(w, n)
}

// MarkRead godoc
// @Summary Mark a notification as read
// @Description Mark a single notification as read by ID
// @Tags notifications
// @Param id path string true "Notification ID"
// @Success 204 "No Content"
// @Failure 500 {object} response.Response
// @Router /notifications/{id}/read [patch]
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		response.BadRequest(w, "missing id")
		return
	}
	updated, err := h.store.MarkRead(r.Context(), userID, id)
	if err != nil {
		response.InternalError(w, "failed to mark notification as read")
		return
	}
	if !updated {
		response.NotFound(w, "notification not found")
		return
	}
	if h.readSync != nil {
		unread, cErr := h.store.CountUnread(r.Context(), userID)
		if cErr == nil {
			h.readSync.NotificationRead(r.Context(), userID, id, unread)
		}
	}
	response.NoContent(w)
}

// MarkAllRead godoc
// @Summary Mark all notifications as read
// @Description Mark all unread notifications as read for a user
// @Tags notifications
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} response.Response{data=object{updated=int64}}
// @Failure 500 {object} response.Response
// @Router /users/{userId}/notifications/read-all [patch]
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	count, err := h.store.MarkAllRead(r.Context(), userID)
	if err != nil {
		response.InternalError(w, "failed to mark all as read")
		return
	}
	if h.readSync != nil {
		h.readSync.NotificationReadAll(r.Context(), userID)
	}
	response.OK(w, map[string]int64{"updated": count})
}

// Delete godoc
// @Summary Delete a notification
// @Description Delete a notification by ID
// @Tags notifications
// @Param id path string true "Notification ID"
// @Success 204 "No Content"
// @Failure 500 {object} response.Response
// @Router /notifications/{id} [delete]
func (h *NotificationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		response.BadRequest(w, "missing id")
		return
	}
	deleted, err := h.store.Delete(r.Context(), userID, id)
	if err != nil {
		response.InternalError(w, "failed to delete notification")
		return
	}
	if !deleted {
		response.NotFound(w, "notification not found")
		return
	}
	response.NoContent(w)
}

// CountUnread godoc
// @Summary Count unread notifications
// @Description Get the count of unread notifications for a user
// @Tags notifications
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} response.Response{data=object{count=int64}}
// @Failure 500 {object} response.Response
// @Router /users/{userId}/notifications/unread-count [get]
func (h *NotificationHandler) CountUnread(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	count, err := h.store.CountUnread(r.Context(), userID)
	if err != nil {
		response.InternalError(w, "failed to count unread notifications")
		return
	}
	response.OK(w, map[string]int64{"count": count})
}
