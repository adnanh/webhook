package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/xtulnx/webhook/internal/network"
)

func registerNetworkAdminRoutes(r *mux.Router, hub *network.Hub) {
	if !*adminEnabled || hub == nil {
		return
	}
	base := currentAdminAuth.basePath + "/api/network/nodes"
	r.HandleFunc(base, adminRequireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		writeAdminJSON(w, http.StatusOK, struct {
			HubVersion string `json:"hub_version"`
			network.HubStatus
		}{version, hub.Status()})
	})).Methods(http.MethodGet)
	r.HandleFunc(base+"/{id}", adminRequireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		for _, node := range hub.Status().Nodes {
			if node.ID == mux.Vars(r)["id"] {
				writeAdminJSON(w, http.StatusOK, node)
				return
			}
		}
		writeAdminError(w, http.StatusNotFound, "unknown network node")
	})).Methods(http.MethodGet)
}
