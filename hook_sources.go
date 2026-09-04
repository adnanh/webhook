package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/adnanh/webhook/internal/hook"
	"github.com/fsnotify/fsnotify"
)

type hookLoadMetadata struct {
	LoadedAt time.Time
}

type hookLoadFailure struct {
	FailedAt time.Time
	Err      string
}

type hookApplyError struct {
	err error
}

func (e *hookApplyError) Error() string { return e.err.Error() }
func (e *hookApplyError) Unwrap() error { return e.err }

var (
	watcher *fsnotify.Watcher

	configuredHookFiles    []string
	explicitHookFiles      = make(map[string]struct{})
	hookDirectoryByFile    = make(map[string]string)
	hookLoadMetadataByFile = make(map[string]hookLoadMetadata)
	hookLoadFailures       = make(map[string]hookLoadFailure)
	lastHooksLoadTime      time.Time
)

func initializeHookSources() error {
	explicit := append([]string(nil), hooksFiles...)
	hooksFiles = nil
	configuredHookFiles = nil
	explicitHookFiles = make(map[string]struct{})
	hookDirectoryByFile = make(map[string]string)

	for _, path := range explicit {
		normalized, err := normalizeSourcePath(path)
		if err != nil {
			return fmt.Errorf("invalid hooks file path %q: %w", path, err)
		}
		if _, exists := explicitHookFiles[normalized]; exists {
			continue
		}
		explicitHookFiles[normalized] = struct{}{}
		configuredHookFiles = append(configuredHookFiles, normalized)
	}

	normalizedDirectories := make(hook.HooksFiles, 0, len(hooksDirectories))
	seenDirectories := make(map[string]struct{}, len(hooksDirectories))
	for _, directory := range hooksDirectories {
		normalized, err := normalizeSourcePath(directory)
		if err != nil {
			return fmt.Errorf("invalid hooks directory path %q: %w", directory, err)
		}
		info, err := os.Stat(normalized)
		if err != nil {
			return fmt.Errorf("cannot access hooks directory %q: %w", directory, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("hooks directory %q is not a directory", directory)
		}
		if _, exists := seenDirectories[normalized]; exists {
			continue
		}
		seenDirectories[normalized] = struct{}{}
		normalizedDirectories = append(normalizedDirectories, normalized)
	}
	hooksDirectories = normalizedDirectories

	for _, path := range configuredHookFiles {
		if err := reloadHooks(path); err != nil {
			var applyErr *hookApplyError
			if errors.As(err, &applyErr) {
				return applyErr
			}
		}
	}
	for _, directory := range hooksDirectories {
		if err := syncHooksDirectory(directory); err != nil {
			return err
		}
	}

	return nil
}

func normalizeSourcePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path must not be empty")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	return abs, nil
}

func isHooksConfigFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func directoryHooksFiles(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("cannot read hooks directory %q: %w", directory, err)
	}

	resolvedDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve hooks directory %q: %w", directory, err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !isHooksConfigFile(entry.Name()) {
			continue
		}

		path := filepath.Join(directory, entry.Name())
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			log.Printf("ignoring hooks file %s: %v", path, err)
			continue
		}
		relative, err := filepath.Rel(resolvedDirectory, resolvedPath)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			log.Printf("ignoring hooks file %s because it resolves outside its configured directory", path)
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			log.Printf("ignoring hooks file %s because it is not a regular file", path)
			continue
		}
		files = append(files, path)
	}

	sort.Strings(files)
	return files, nil
}

func syncHooksDirectory(directory string) error {
	files, err := directoryHooksFiles(directory)
	if err != nil {
		return err
	}

	found := make(map[string]struct{}, len(files))
	for _, path := range files {
		found[path] = struct{}{}
	}

	loadedHooksMu.RLock()
	removed := make([]string, 0)
	for path, sourceDirectory := range hookDirectoryByFile {
		if sourceDirectory == directory {
			if _, exists := found[path]; !exists {
				removed = append(removed, path)
			}
		}
	}
	loadedHooksMu.RUnlock()
	for _, path := range removed {
		removeHooks(path)
	}

	parsed := make(map[string]hook.Hooks, len(files))
	parseFailures := make(map[string]error)
	for _, path := range files {
		hooksInFile := hook.Hooks{}
		if err := hooksInFile.LoadFromFile(path, *asTemplate); err != nil {
			parseFailures[path] = err
			continue
		}
		parsed[path] = hooksInFile
	}

	now := time.Now().UTC()
	loadedHooksMu.Lock()
	for _, path := range files {
		hookDirectoryByFile[path] = directory
	}

	candidate := cloneLoadedHooksMapLocked()
	for path, hooksInFile := range parsed {
		candidate[path] = cloneHooks(hooksInFile)
	}
	if err := validateUniqueHookIDs(candidate); err != nil {
		for path, hooksInFile := range parsed {
			if !reflect.DeepEqual(loadedHooksFromFiles[path], hooksInFile) {
				hookLoadFailures[path] = hookLoadFailure{FailedAt: now, Err: err.Error()}
			}
		}
		for path, parseErr := range parseFailures {
			hookLoadFailures[path] = hookLoadFailure{FailedAt: now, Err: parseErr.Error()}
		}
		loadedHooksMu.Unlock()
		log.Printf("couldn't apply hooks directory %s: %v; keeping the previous configuration", directory, err)
		return nil
	}

	for path, hooksInFile := range parsed {
		loadedHooksFromFiles[path] = hooksInFile
		if !containsString(hooksFiles, path) {
			hooksFiles = append(hooksFiles, path)
		}
		markHookLoadedLocked(path, now)
	}
	for path, parseErr := range parseFailures {
		hookLoadFailures[path] = hookLoadFailure{FailedAt: now, Err: parseErr.Error()}
	}
	loadedHooksMu.Unlock()

	for path, hooksInFile := range parsed {
		log.Printf("loaded %d hook(s) from %s", len(hooksInFile), path)
	}
	for path, parseErr := range parseFailures {
		log.Printf("couldn't load hooks from file %s: %v; keeping the previous configuration", path, parseErr)
	}

	return nil
}

func reloadHooks(path string) error {
	log.Printf("attempting to load hooks from %s", path)

	info, err := os.Stat(path)
	if err != nil {
		recordHookLoadFailure(path, err)
		log.Printf("couldn't load hooks from file! %v", err)
		return err
	}
	if !info.Mode().IsRegular() {
		err = errors.New("hooks source is not a regular file")
		recordHookLoadFailure(path, err)
		log.Printf("couldn't load hooks from file! %v", err)
		return err
	}

	hooksInFile := hook.Hooks{}
	if err := hooksInFile.LoadFromFile(path, *asTemplate); err != nil {
		recordHookLoadFailure(path, err)
		log.Printf("couldn't load hooks from file! %v", err)
		return err
	}

	now := time.Now().UTC()
	loadedHooksMu.Lock()
	candidate := cloneLoadedHooksMapLocked()
	candidate[path] = cloneHooks(hooksInFile)
	if err := validateUniqueHookIDs(candidate); err != nil {
		hookLoadFailures[path] = hookLoadFailure{FailedAt: now, Err: err.Error()}
		loadedHooksMu.Unlock()
		log.Printf("couldn't apply hooks from file %s: %v; keeping the previous configuration", path, err)
		return &hookApplyError{err: err}
	}

	loadedHooksFromFiles[path] = hooksInFile
	if !containsString(hooksFiles, path) {
		hooksFiles = append(hooksFiles, path)
	}
	markHookLoadedLocked(path, now)
	loadedHooksMu.Unlock()

	log.Printf("found %d hook(s) in file", len(hooksInFile))
	for _, currentHook := range hooksInFile {
		log.Printf("\tloaded: %s", currentHook.ID)
	}
	return nil
}

func recordHookLoadFailure(path string, err error) {
	loadedHooksMu.Lock()
	hookLoadFailures[path] = hookLoadFailure{FailedAt: time.Now().UTC(), Err: err.Error()}
	loadedHooksMu.Unlock()
}

func markHookLoadedLocked(path string, loadedAt time.Time) {
	loadedAt = loadedAt.UTC()
	hookLoadMetadataByFile[path] = hookLoadMetadata{LoadedAt: loadedAt}
	delete(hookLoadFailures, path)
	lastHooksLoadTime = loadedAt
}

func removeHooks(path string) {
	loadedHooksMu.Lock()
	removedCount := len(loadedHooksFromFiles[path])
	delete(loadedHooksFromFiles, path)
	delete(hookLoadMetadataByFile, path)
	delete(hookLoadFailures, path)
	delete(hookDirectoryByFile, path)

	remaining := hooksFiles[:0]
	for _, current := range hooksFiles {
		if current != path {
			remaining = append(remaining, current)
		}
	}
	hooksFiles = remaining
	loadedHooksMu.Unlock()

	if removedCount != 0 {
		log.Printf("removed %d hook(s) loaded from %s", removedCount, path)
	}
}

func reloadAllHooks() {
	for _, path := range configuredHookFiles {
		_ = reloadHooks(path)
	}
	for _, directory := range hooksDirectories {
		if err := syncHooksDirectory(directory); err != nil {
			log.Printf("couldn't rescan hooks directory %s: %v", directory, err)
		}
	}
}

func startHookWatcher() error {
	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	targets := make(map[string]struct{})
	if *hotReload {
		for _, path := range configuredHookFiles {
			targets[filepath.Dir(path)] = struct{}{}
		}
	}
	for _, directory := range hooksDirectories {
		targets[directory] = struct{}{}
	}

	for target := range targets {
		log.Printf("setting up hooks watcher for %s", target)
		if err := watcher.Add(target); err != nil {
			_ = watcher.Close()
			watcher = nil
			return fmt.Errorf("cannot watch %q: %w", target, err)
		}
	}
	return nil
}

func watchForFileChange() {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			handleHookFileEvent(event)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("watcher error:", err)
		}
	}
}

func handleHookFileEvent(event fsnotify.Event) {
	eventPath, err := normalizeSourcePath(event.Name)
	if err != nil {
		return
	}
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) != 0 {
		// Editors and deployment tools often emit an event before an atomic
		// replacement is fully visible through the directory entry.
		time.Sleep(50 * time.Millisecond)
	}

	for _, directory := range hooksDirectories {
		if filepath.Dir(eventPath) == directory {
			if err := syncHooksDirectory(directory); err != nil {
				log.Printf("couldn't rescan hooks directory %s: %v", directory, err)
			}
		}
	}

	if _, configured := explicitHookFiles[eventPath]; !configured || !*hotReload {
		return
	}
	if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		if _, err := os.Stat(eventPath); os.IsNotExist(err) {
			removeHooks(eventPath)
			recordHookLoadFailure(eventPath, err)
			return
		}
	}
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) != 0 {
		_ = reloadHooks(eventPath)
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
