// Package api assembles the HTTP router, controllers, and middleware.
package api

import (
	"net/http"

	"github.com/kazemisoroush/vault/backend/internal/api/controller"
)

// Router maps HTTP endpoints to their controllers.
type Router struct {
	mux *http.ServeMux
}

// NewRouter wires each endpoint to a controller method.
func NewRouter(files *controller.File, health *controller.Health) *Router {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /files", files.Drop)
	mux.HandleFunc("GET /files", files.List)
	mux.HandleFunc("GET /files/{id}", files.Get)
	mux.HandleFunc("PATCH /files/{id}", files.Update)
	mux.HandleFunc("DELETE /files/{id}", files.Delete)
	mux.Handle("GET /health", health)
	return &Router{mux: mux}
}

// ServeHTTP dispatches a request to the matching controller.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
