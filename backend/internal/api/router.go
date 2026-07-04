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

// NewRouter wires each endpoint to its controller. Distinct types make a mis-order a compile error.
func NewRouter(drop *controller.Drop, list *controller.List, get *controller.Get, update *controller.Update, remove *controller.Delete, health *controller.Health) *Router {
	mux := http.NewServeMux()
	mux.Handle("POST /files", drop)
	mux.Handle("GET /files", list)
	mux.Handle("GET /files/{id}", get)
	mux.Handle("PATCH /files/{id}", update)
	mux.Handle("DELETE /files/{id}", remove)
	mux.Handle("GET /health", health)
	return &Router{mux: mux}
}

// ServeHTTP dispatches a request to the matching controller.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
