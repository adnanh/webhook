package app

import (
	"fmt"
	"log"
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/mux"
	"github.com/xtulnx/webhook/internal/middleware"
	"github.com/xtulnx/webhook/internal/network"
)

// newHTTPHandler assembles service, management, and business routes. Network
// transport uses its own authentication and is dispatched outside the HTTP
// access-control middleware, as it was before these groups were extracted.
func newHTTPHandler(hub *network.Hub) http.Handler {
	r := mux.NewRouter()
	r.Use(middleware.RequestID(
		middleware.UseXRequestIDHeaderOption(*useXRequestID),
		middleware.XRequestIDLimitOption(*xRequestIDLimit),
	))
	r.Use(middleware.NewLogger())
	r.Use(chimiddleware.Recoverer)
	r.Use(accessControlMiddleware)
	if *debug {
		r.Use(middleware.Dumper(log.Writer()))
	}

	registerServiceRoutes(r)
	registerManagementRoutes(r, hub)
	registerBusinessRoutes(r, hub)
	return networkTransportHandler(r, hub)
}

func registerServiceRoutes(r *mux.Router) {
	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		for _, responseHeader := range responseHeaders {
			w.Header().Set(responseHeader.Name, responseHeader.Value)
		}
		fmt.Fprint(w, "OK")
	})
	registerStatusRoute(r)
}

func registerManagementRoutes(r *mux.Router, hub *network.Hub) {
	registerAdminRoutes(r)
	registerNetworkAdminRoutes(r, hub)
}

func registerBusinessRoutes(r *mux.Router, hub *network.Hub) {
	// Explicit Hub routes take precedence over the local /hooks/{id} pattern.
	registerNetworkHookRoutes(r, hub)
	r.HandleFunc(makeRoutePattern(hooksURLPrefix), hookHandler)
}
