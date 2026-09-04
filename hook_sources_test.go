package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/adnanh/webhook/internal/hook"
)

func resetHookSourceTestState(t *testing.T) {
	t.Helper()

	savedHooksFiles := append(hook.HooksFiles(nil), hooksFiles...)
	savedDirectories := append(hook.HooksFiles(nil), hooksDirectories...)
	savedConfigured := append([]string(nil), configuredHookFiles...)
	savedExplicit := explicitHookFiles
	savedDirectoryByFile := hookDirectoryByFile
	savedMetadata := hookLoadMetadataByFile
	savedFailures := hookLoadFailures
	savedLastLoad := lastHooksLoadTime
	savedLoaded := loadedHooksFromFiles
	savedTemplate := *asTemplate
	savedHotReload := *hotReload
	savedWatcher := watcher

	hooksFiles = nil
	hooksDirectories = nil
	configuredHookFiles = nil
	explicitHookFiles = make(map[string]struct{})
	hookDirectoryByFile = make(map[string]string)
	hookLoadMetadataByFile = make(map[string]hookLoadMetadata)
	hookLoadFailures = make(map[string]hookLoadFailure)
	lastHooksLoadTime = time.Time{}
	loadedHooksFromFiles = make(map[string]hook.Hooks)
	*asTemplate = false
	*hotReload = false
	watcher = nil

	t.Cleanup(func() {
		hooksFiles = savedHooksFiles
		hooksDirectories = savedDirectories
		configuredHookFiles = savedConfigured
		explicitHookFiles = savedExplicit
		hookDirectoryByFile = savedDirectoryByFile
		hookLoadMetadataByFile = savedMetadata
		hookLoadFailures = savedFailures
		lastHooksLoadTime = savedLastLoad
		loadedHooksFromFiles = savedLoaded
		*asTemplate = savedTemplate
		*hotReload = savedHotReload
		watcher = savedWatcher
	})
}

func writeHookIDs(t *testing.T, path string, ids ...string) {
	t.Helper()

	hooks := make([]hook.Hook, 0, len(ids))
	for _, id := range ids {
		hooks = append(hooks, hook.Hook{ID: id, ExecuteCommand: "/bin/true"})
	}
	data, err := json.Marshal(hooks)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeHookSourcesLoadsDirectory(t *testing.T) {
	resetHookSourceTestState(t)

	directory := t.TempDir()
	writeHookIDs(t, filepath.Join(directory, "one.json"), "alpha")
	writeHookIDs(t, filepath.Join(directory, "two.YAML"), "beta")
	writeHookIDs(t, filepath.Join(directory, "ignored.txt"), "ignored")
	hooksDirectories = hook.HooksFiles{directory}

	if err := initializeHookSources(); err != nil {
		t.Fatalf("initializeHookSources: %v", err)
	}
	if matchLoadedHook("alpha") == nil || matchLoadedHook("beta") == nil {
		t.Fatalf("directory hooks were not loaded: %#v", loadedHooksFromFiles)
	}
	if matchLoadedHook("ignored") != nil {
		t.Fatal("unsupported file extension was loaded")
	}
	if len(hooksFilesSnapshot()) != 2 {
		t.Fatalf("loaded file count = %d, want 2", len(hooksFilesSnapshot()))
	}
}

func TestSyncHooksDirectoryAppliesChangesSafely(t *testing.T) {
	resetHookSourceTestState(t)

	directory := t.TempDir()
	first := filepath.Join(directory, "first.json")
	second := filepath.Join(directory, "second.yaml")
	writeHookIDs(t, first, "alpha")
	hooksDirectories = hook.HooksFiles{directory}
	if err := initializeHookSources(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(first, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syncHooksDirectory(directory); err != nil {
		t.Fatal(err)
	}
	if matchLoadedHook("alpha") == nil {
		t.Fatal("invalid update replaced the last valid hooks")
	}
	if len(hookLoadFailures) != 1 {
		t.Fatalf("load failure count = %d, want 1", len(hookLoadFailures))
	}

	writeHookIDs(t, first, "beta")
	writeHookIDs(t, second, "gamma")
	if err := syncHooksDirectory(directory); err != nil {
		t.Fatal(err)
	}
	if matchLoadedHook("alpha") != nil || matchLoadedHook("beta") == nil || matchLoadedHook("gamma") == nil {
		t.Fatal("valid update and addition were not applied")
	}

	if err := os.Remove(first); err != nil {
		t.Fatal(err)
	}
	if err := syncHooksDirectory(directory); err != nil {
		t.Fatal(err)
	}
	if matchLoadedHook("beta") != nil || matchLoadedHook("gamma") == nil {
		t.Fatal("removed file hooks were not removed independently")
	}
}

func TestSyncHooksDirectoryRejectsDuplicateIDs(t *testing.T) {
	resetHookSourceTestState(t)

	directory := t.TempDir()
	writeHookIDs(t, filepath.Join(directory, "first.json"), "shared")
	hooksDirectories = hook.HooksFiles{directory}
	if err := initializeHookSources(); err != nil {
		t.Fatal(err)
	}

	writeHookIDs(t, filepath.Join(directory, "second.json"), "shared")
	if err := syncHooksDirectory(directory); err != nil {
		t.Fatal(err)
	}
	if lenLoadedHooks() != 1 {
		t.Fatalf("loaded hooks = %d, want 1", lenLoadedHooks())
	}
	if len(hookLoadFailures) != 1 {
		t.Fatalf("load failure count = %d, want 1", len(hookLoadFailures))
	}
}

func TestSyncHooksDirectoryAllowsIDTransferBetweenFiles(t *testing.T) {
	resetHookSourceTestState(t)

	directory := t.TempDir()
	oldPath := filepath.Join(directory, "old.json")
	newPath := filepath.Join(directory, "new.json")
	writeHookIDs(t, oldPath, "transferred")
	hooksDirectories = hook.HooksFiles{directory}
	if err := initializeHookSources(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	if err := syncHooksDirectory(directory); err != nil {
		t.Fatal(err)
	}

	if matchLoadedHook("transferred") == nil {
		t.Fatal("hook was lost while its source file was renamed")
	}
	files := hooksFilesSnapshot()
	if len(files) != 1 || files[0] != newPath {
		t.Fatalf("loaded sources = %v, want [%s]", files, newPath)
	}
}

func TestInitializeHookSourcesRejectsDuplicateExplicitFiles(t *testing.T) {
	resetHookSourceTestState(t)

	directory := t.TempDir()
	first := filepath.Join(directory, "first.json")
	second := filepath.Join(directory, "second.json")
	writeHookIDs(t, first, "shared")
	writeHookIDs(t, second, "shared")
	hooksFiles = hook.HooksFiles{first, second}

	if err := initializeHookSources(); err == nil {
		t.Fatal("duplicate IDs in explicitly configured files were accepted")
	}
}

func TestDirectoryWatcherHandlesAddAndRemove(t *testing.T) {
	resetHookSourceTestState(t)

	directory := t.TempDir()
	hooksDirectories = hook.HooksFiles{directory}
	if err := initializeHookSources(); err != nil {
		t.Fatal(err)
	}
	if err := startHookWatcher(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		watchForFileChange()
		close(done)
	}()

	path := filepath.Join(directory, "dynamic.json")
	writeHookIDs(t, path, "dynamic")
	waitForHook(t, "dynamic", true)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	waitForHook(t, "dynamic", false)

	if err := watcher.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("watcher goroutine did not stop")
	}
	watcher = nil
}

func waitForHook(t *testing.T, id string, present bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if (matchLoadedHook(id) != nil) == present {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("hook %q presence did not become %t", id, present)
}

func TestDirectoryHooksFilesRejectsEscapingSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges on Windows")
	}

	directory := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.json")
	writeHookIDs(t, outside, "outside")
	link := filepath.Join(directory, "escape.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	files, err := directoryHooksFiles(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("escaping symlink was accepted: %v", files)
	}
}

func TestStatusHandlerReportsHooksWithoutSensitiveConfiguration(t *testing.T) {
	resetHookSourceTestState(t)

	path := filepath.Join(t.TempDir(), "private-hooks.json")
	loadedAt := time.Date(2026, 8, 24, 10, 30, 0, 0, time.UTC)
	loadedHooksFromFiles[path] = hook.Hooks{{
		ID:             "deploy",
		ExecuteCommand: "/secret/bin/deploy --token private-value",
	}}
	hooksFiles = hook.HooksFiles{path}
	hookLoadMetadataByFile[path] = hookLoadMetadata{LoadedAt: loadedAt}
	lastHooksLoadTime = loadedAt

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	statusHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, sensitive := range []string{filepath.Dir(path), "/secret/bin/deploy", "private-value"} {
		if strings.Contains(body, sensitive) {
			t.Fatalf("status response leaked %q: %s", sensitive, body)
		}
	}
	if !strings.Contains(body, `"source":"private-hooks.json"`) || !strings.Contains(body, `"hooks":["deploy"]`) {
		t.Fatalf("status response does not identify loaded hooks: %s", body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}
}
