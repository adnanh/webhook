package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adnanh/webhook/internal/hook"
	"github.com/ghodss/yaml"
)

var (
	loadedHooksMu sync.RWMutex

	errAdminReadOnly   = errors.New("admin writes are disabled when -template is enabled")
	errUnknownHookFile = errors.New("unknown hooks file")
)

type adminHooksFile struct {
	Path     string     `json:"path"`
	Format   string     `json:"format"`
	Writable bool       `json:"writable"`
	Hooks    hook.Hooks `json:"hooks"`
}

type adminHooksState struct {
	Files          []adminHooksFile `json:"files"`
	ReadOnly       bool             `json:"readOnly"`
	ReadOnlyReason string           `json:"readOnlyReason,omitempty"`
}

func adminWritesDisabled() bool {
	return *asTemplate
}

func adminReadOnlyReason() string {
	if !adminWritesDisabled() {
		return ""
	}

	return "editing is disabled while webhook is running with -template because the original template source cannot be preserved safely"
}

func cloneHook(src hook.Hook) hook.Hook {
	dst := src

	if src.ResponseHeaders != nil {
		dst.ResponseHeaders = append(hook.ResponseHeaders(nil), src.ResponseHeaders...)
	}
	if src.PassEnvironmentToCommand != nil {
		dst.PassEnvironmentToCommand = append([]hook.Argument(nil), src.PassEnvironmentToCommand...)
	}
	if src.PassArgumentsToCommand != nil {
		dst.PassArgumentsToCommand = append([]hook.Argument(nil), src.PassArgumentsToCommand...)
	}
	if src.PassFileToCommand != nil {
		dst.PassFileToCommand = append([]hook.Argument(nil), src.PassFileToCommand...)
	}
	if src.JSONStringParameters != nil {
		dst.JSONStringParameters = append([]hook.Argument(nil), src.JSONStringParameters...)
	}
	if src.HTTPMethods != nil {
		dst.HTTPMethods = append([]string(nil), src.HTTPMethods...)
	}
	if src.MaxConcurrency != nil {
		value := *src.MaxConcurrency
		dst.MaxConcurrency = &value
	}

	return dst
}

func cloneHooks(src hook.Hooks) hook.Hooks {
	if src == nil {
		return nil
	}

	dst := make(hook.Hooks, len(src))
	for i := range src {
		dst[i] = cloneHook(src[i])
	}

	return dst
}

func cloneLoadedHooksMapLocked() map[string]hook.Hooks {
	dst := make(map[string]hook.Hooks, len(loadedHooksFromFiles))
	for path, hooksInFile := range loadedHooksFromFiles {
		dst[path] = cloneHooks(hooksInFile)
	}

	return dst
}

func hooksFilesSnapshot() []string {
	loadedHooksMu.RLock()
	defer loadedHooksMu.RUnlock()

	return append([]string(nil), []string(hooksFiles)...)
}

func hooksStateSnapshot() adminHooksState {
	loadedHooksMu.RLock()
	files := append([]string(nil), []string(hooksFiles)...)
	loaded := cloneLoadedHooksMapLocked()
	loadedHooksMu.RUnlock()

	state := adminHooksState{
		ReadOnly:       adminWritesDisabled(),
		ReadOnlyReason: adminReadOnlyReason(),
		Files:          make([]adminHooksFile, 0, len(files)),
	}

	for _, path := range files {
		hooksInFile := cloneHooks(loaded[path])
		if hooksInFile == nil {
			hooksInFile = hook.Hooks{}
		}
		sort.Slice(hooksInFile, func(i, j int) bool {
			return hooksInFile[i].ID < hooksInFile[j].ID
		})

		state.Files = append(state.Files, adminHooksFile{
			Path:     path,
			Format:   hooksFileFormat(path),
			Writable: !state.ReadOnly,
			Hooks:    hooksInFile,
		})
	}

	return state
}

func hooksFileFormat(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return "yaml"
	default:
		return "json"
	}
}

func marshalHooksFile(path string, hooksInFile hook.Hooks) ([]byte, error) {
	var (
		data []byte
		err  error
	)

	switch hooksFileFormat(path) {
	case "yaml":
		data, err = yaml.Marshal(hooksInFile)
	default:
		data, err = json.MarshalIndent(hooksInFile, "", "  ")
	}
	if err != nil {
		return nil, err
	}

	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	return data, nil
}

func writeHooksFile(path string, hooksInFile hook.Hooks) error {
	data, err := marshalHooksFile(path, hooksInFile)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}

	tmp, err := ioutil.TempFile(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	mode := os.FileMode(0o644)
	if stat, statErr := os.Stat(path); statErr == nil {
		mode = stat.Mode()
	}

	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	renameErr := os.Rename(tmpName, path)
	if renameErr == nil {
		return nil
	}

	if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
		return renameErr
	}

	return os.Rename(tmpName, path)
}

func validateUniqueHookIDs(candidate map[string]hook.Hooks) error {
	seen := make(map[string]string)

	for path, hooksInFile := range candidate {
		for _, currentHook := range hooksInFile {
			id := strings.TrimSpace(currentHook.ID)
			if id == "" {
				return fmt.Errorf("hook id is required for hooks file %s", path)
			}

			if prevPath, ok := seen[id]; ok {
				return fmt.Errorf("hook with ID %s is already defined in %s", id, prevPath)
			}

			seen[id] = path
		}
	}

	return nil
}

func hookIndexByID(hooksInFile hook.Hooks, id string) int {
	for i := range hooksInFile {
		if hooksInFile[i].ID == id {
			return i
		}
	}

	return -1
}

func normalizeAdminHook(current hook.Hook) hook.Hook {
	current.ID = strings.TrimSpace(current.ID)
	current.ExecuteCommand = strings.TrimSpace(current.ExecuteCommand)
	current.CommandWorkingDirectory = strings.TrimSpace(current.CommandWorkingDirectory)
	current.CommandTimeout = strings.TrimSpace(current.CommandTimeout)
	current.ResponseMessage = strings.TrimSpace(current.ResponseMessage)

	if len(current.HTTPMethods) != 0 {
		methods := make([]string, 0, len(current.HTTPMethods))
		for _, method := range current.HTTPMethods {
			method = strings.ToUpper(strings.TrimSpace(method))
			if method != "" {
				methods = append(methods, method)
			}
		}
		current.HTTPMethods = methods
	}

	return current
}

func upsertHookInFile(path, currentID string, updated hook.Hook) error {
	if adminWritesDisabled() {
		return errAdminReadOnly
	}

	updated = normalizeAdminHook(updated)
	if updated.ID == "" {
		return errors.New("hook id is required")
	}
	if err := updated.ValidateExecutionSettings(); err != nil {
		return err
	}

	loadedHooksMu.Lock()
	defer loadedHooksMu.Unlock()

	existingHooks, ok := loadedHooksFromFiles[path]
	if !ok {
		return fmt.Errorf("%w: %s", errUnknownHookFile, path)
	}

	nextHooks := cloneHooks(existingHooks)
	if currentID == "" {
		if hookIndexByID(nextHooks, updated.ID) != -1 {
			return fmt.Errorf("hook with ID %s is already defined", updated.ID)
		}
		nextHooks = append(nextHooks, updated)
	} else {
		index := hookIndexByID(nextHooks, currentID)
		if index == -1 {
			return fmt.Errorf("hook with ID %s was not found in %s", currentID, path)
		}
		nextHooks[index] = updated
	}

	candidate := cloneLoadedHooksMapLocked()
	candidate[path] = nextHooks

	if err := validateUniqueHookIDs(candidate); err != nil {
		return err
	}
	if err := writeHooksFile(path, nextHooks); err != nil {
		return err
	}

	loadedHooksFromFiles[path] = nextHooks
	markHookLoadedLocked(path, time.Now())
	return nil
}

func deleteHookFromFile(path, id string) error {
	if adminWritesDisabled() {
		return errAdminReadOnly
	}

	loadedHooksMu.Lock()
	defer loadedHooksMu.Unlock()

	existingHooks, ok := loadedHooksFromFiles[path]
	if !ok {
		return fmt.Errorf("%w: %s", errUnknownHookFile, path)
	}

	index := hookIndexByID(existingHooks, id)
	if index == -1 {
		return fmt.Errorf("hook with ID %s was not found in %s", id, path)
	}

	nextHooks := cloneHooks(existingHooks[:index])
	nextHooks = append(nextHooks, cloneHooks(existingHooks[index+1:])...)

	candidate := cloneLoadedHooksMapLocked()
	candidate[path] = nextHooks

	if err := validateUniqueHookIDs(candidate); err != nil {
		return err
	}
	if err := writeHooksFile(path, nextHooks); err != nil {
		return err
	}

	loadedHooksFromFiles[path] = nextHooks
	markHookLoadedLocked(path, time.Now())
	return nil
}
