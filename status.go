package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

var serviceStartedAt = time.Now().UTC()

type serviceStatusHookFile struct {
	Source   string   `json:"source"`
	LoadedAt string   `json:"loadedAt,omitempty"`
	Hooks    []string `json:"hooks"`
}

type serviceStatusHooks struct {
	Count        int                     `json:"count"`
	FileCount    int                     `json:"fileCount"`
	LastLoadedAt string                  `json:"lastLoadedAt,omitempty"`
	LoadErrors   int                     `json:"loadErrors"`
	Files        []serviceStatusHookFile `json:"files"`
}

type serviceStatusWatcher struct {
	Enabled        bool `json:"enabled"`
	DirectoryCount int  `json:"directoryCount"`
}

type serviceStatusResponse struct {
	Status        string               `json:"status"`
	Version       string               `json:"version"`
	StartedAt     string               `json:"startedAt"`
	UptimeSeconds int64                `json:"uptimeSeconds"`
	Hooks         serviceStatusHooks   `json:"hooks"`
	Watcher       serviceStatusWatcher `json:"watcher"`
}

func initStatus() error {
	path := strings.Trim(strings.TrimSpace(*statusURLPrefix), "/")
	if path == "" {
		return errors.New("status-path must not be empty")
	}
	if strings.ContainsAny(path, "{}?#\\") {
		return errors.New("status-path contains unsupported URL characters")
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return errors.New("status-path contains an invalid URL segment")
		}
	}

	hooksPath := strings.Trim(strings.TrimSpace(*hooksURLPrefix), "/")
	if path == hooksPath || strings.HasPrefix(path, hooksPath+"/") {
		return errors.New("status-path must not be inside urlprefix")
	}
	if *adminEnabled {
		adminPath := strings.Trim(strings.TrimSpace(*adminURLPrefix), "/")
		if path == adminPath || strings.HasPrefix(path, adminPath+"/") || strings.HasPrefix(adminPath, path+"/") {
			return errors.New("status-path must not overlap admin-path")
		}
	}

	*statusURLPrefix = path
	return nil
}

func registerStatusRoute(router *mux.Router) {
	router.HandleFunc(makeBaseURL(statusURLPrefix), statusHandler).Methods(http.MethodGet, http.MethodHead)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	response := serviceStatusSnapshot(now)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	if err := encoder.Encode(response); err != nil {
		http.Error(w, "failed to encode service status", http.StatusInternalServerError)
	}
}

func serviceStatusSnapshot(now time.Time) serviceStatusResponse {
	loadedHooksMu.RLock()
	files := append([]string(nil), hooksFiles...)
	loaded := cloneLoadedHooksMapLocked()
	metadata := make(map[string]hookLoadMetadata, len(hookLoadMetadataByFile))
	for path, value := range hookLoadMetadataByFile {
		metadata[path] = value
	}
	failureCount := len(hookLoadFailures)
	lastLoaded := lastHooksLoadTime
	loadedHooksMu.RUnlock()

	sort.Slice(files, func(i, j int) bool {
		left, right := filepath.Base(files[i]), filepath.Base(files[j])
		if left == right {
			return files[i] < files[j]
		}
		return left < right
	})

	hookFiles := make([]serviceStatusHookFile, 0, len(files))
	hookCount := 0
	for _, path := range files {
		ids := make([]string, 0, len(loaded[path]))
		for _, currentHook := range loaded[path] {
			ids = append(ids, currentHook.ID)
		}
		sort.Strings(ids)
		hookCount += len(ids)

		fileStatus := serviceStatusHookFile{
			Source: filepath.Base(path),
			Hooks:  ids,
		}
		if value, exists := metadata[path]; exists && !value.LoadedAt.IsZero() {
			fileStatus.LoadedAt = value.LoadedAt.Format(time.RFC3339Nano)
		}
		hookFiles = append(hookFiles, fileStatus)
	}

	status := "ok"
	if hookCount == 0 || failureCount != 0 {
		status = "degraded"
	}
	result := serviceStatusResponse{
		Status:        status,
		Version:       version,
		StartedAt:     serviceStartedAt.Format(time.RFC3339Nano),
		UptimeSeconds: int64(now.Sub(serviceStartedAt).Seconds()),
		Hooks: serviceStatusHooks{
			Count:      hookCount,
			FileCount:  len(hookFiles),
			LoadErrors: failureCount,
			Files:      hookFiles,
		},
		Watcher: serviceStatusWatcher{
			Enabled:        watcher != nil,
			DirectoryCount: len(hooksDirectories),
		},
	}
	if !lastLoaded.IsZero() {
		result.Hooks.LastLoadedAt = lastLoaded.Format(time.RFC3339Nano)
	}
	return result
}
