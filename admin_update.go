package main

import (
	"context"
	"net/http"
	"sync"
	"time"

	webhookupdate "github.com/adnanh/webhook/internal/update"
)

type adminUpdateStatus struct {
	Enabled        bool   `json:"enabled"`
	Repository     string `json:"repository"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion,omitempty"`
	Available      bool   `json:"available"`
	Verified       bool   `json:"verified"`
	PublishedAt    string `json:"publishedAt,omitempty"`
	ReleaseURL     string `json:"releaseURL,omitempty"`
	CheckedAt      string `json:"checkedAt,omitempty"`
	Error          string `json:"error,omitempty"`
}

var adminUpdateState = struct {
	sync.RWMutex
	status adminUpdateStatus
}{}

var checkAdminUpdate = func(ctx context.Context) (webhookupdate.Result, error) {
	return newUpdateClient(*updateRepository).Check(ctx, "")
}

func adminUpdateStatusSnapshot() adminUpdateStatus {
	adminUpdateState.RLock()
	status := adminUpdateState.status
	adminUpdateState.RUnlock()
	status.Enabled = *updateEnabled
	status.Repository = *updateRepository
	status.CurrentVersion = version
	return status
}

func adminUpdateStatusHandler(w http.ResponseWriter, r *http.Request) {
	writeAdminJSON(w, http.StatusOK, adminUpdateStatusSnapshot())
}

func adminUpdateCheckHandler(w http.ResponseWriter, r *http.Request) {
	if !*updateEnabled {
		writeAdminError(w, http.StatusForbidden, "update checks are disabled")
		return
	}

	result, err := checkAdminUpdate(r.Context())
	if err != nil {
		adminUpdateState.Lock()
		adminUpdateState.status = adminUpdateStatus{
			CurrentVersion: version,
			CheckedAt:      time.Now().UTC().Format(time.RFC3339Nano),
			Error:          err.Error(),
		}
		adminUpdateState.Unlock()
		writeAdminError(w, http.StatusBadGateway, "update check failed")
		return
	}

	status := adminUpdateStatus{
		CurrentVersion: result.CurrentVersion,
		LatestVersion:  result.LatestVersion,
		Available:      result.Available,
		Verified:       result.Verified,
		PublishedAt:    result.PublishedAt.Format(time.RFC3339Nano),
		ReleaseURL:     result.ReleaseURL,
		CheckedAt:      result.CheckedAt.Format(time.RFC3339Nano),
	}
	adminUpdateState.Lock()
	adminUpdateState.status = status
	adminUpdateState.Unlock()
	writeAdminJSON(w, http.StatusOK, adminUpdateStatusSnapshot())
}
