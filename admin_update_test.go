package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	webhookupdate "github.com/adnanh/webhook/internal/update"
)

func TestAdminUpdateStatusRequiresAuth(t *testing.T) {
	restore := setupAdminTestState(t)
	defer restore()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/api/update/status", nil)
	adminRequireAuth(adminUpdateStatusHandler)(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestAdminUpdateCheck(t *testing.T) {
	restoreAdmin := setupAdminTestState(t)
	defer restoreAdmin()

	savedEnabled := *updateEnabled
	savedRepository := *updateRepository
	savedCheck := checkAdminUpdate
	savedState := adminUpdateStatusSnapshot()
	defer func() {
		*updateEnabled = savedEnabled
		*updateRepository = savedRepository
		checkAdminUpdate = savedCheck
		adminUpdateState.Lock()
		adminUpdateState.status = savedState
		adminUpdateState.Unlock()
	}()

	*updateEnabled = true
	*updateRepository = "xtulnx/webhook"
	checkedAt := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	checkAdminUpdate = func(context.Context) (webhookupdate.Result, error) {
		return webhookupdate.Result{
			CurrentVersion: version,
			LatestVersion:  "v2.8.4",
			Available:      true,
			Verified:       true,
			CheckedAt:      checkedAt,
		}, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/api/update/check", nil)
	adminUpdateCheckHandler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var status adminUpdateStatus
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.LatestVersion != "v2.8.4" || !status.Available || !status.Verified {
		t.Fatalf("unexpected update status: %+v", status)
	}
}

func TestAdminUpdateCheckFailureDoesNotExposeDetails(t *testing.T) {
	restoreAdmin := setupAdminTestState(t)
	defer restoreAdmin()

	savedEnabled := *updateEnabled
	savedCheck := checkAdminUpdate
	defer func() {
		*updateEnabled = savedEnabled
		checkAdminUpdate = savedCheck
	}()
	*updateEnabled = true
	checkAdminUpdate = func(context.Context) (webhookupdate.Result, error) {
		return webhookupdate.Result{}, errors.New("private upstream detail")
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/api/update/check", nil)
	adminUpdateCheckHandler(recorder, request)
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadGateway)
	}
	if recorder.Body.String() == "" {
		t.Fatal("expected an error response")
	}
}
