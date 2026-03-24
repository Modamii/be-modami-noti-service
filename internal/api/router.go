package api

import "net/http"

// RouteRegistrar allows handler groups to self-register their routes.
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

// RegisterAll registers all route groups on the given mux.
func RegisterAll(mux *http.ServeMux, registrars ...RouteRegistrar) {
	for _, r := range registrars {
		r.RegisterRoutes(mux)
	}
}
