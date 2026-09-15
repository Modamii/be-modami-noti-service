package api

import (
	"net/http"

	"be-modami-no-service/internal/api/middleware"

	"gitlab.com/lifegoeson-libs/pkg-gokit/response"
)

// callerID returns the authenticated user ID for the request.
//
// Routes shaped /users/{userId}/... keep the path segment for readability, but it
// is never trusted: when it disagrees with the token subject the request is
// refused rather than silently answered with the caller's own data. Handlers must
// stop when ok is false — the response has already been written.
func callerID(w http.ResponseWriter, r *http.Request) (string, bool) {
	uid := middleware.UserID(r)
	if uid == "" {
		response.Unauthorized(w, "missing bearer token")
		return "", false
	}
	if pathID := r.PathValue("userId"); pathID != "" && pathID != uid {
		response.Forbidden(w, "cannot access another user's notifications")
		return "", false
	}
	return uid, true
}
