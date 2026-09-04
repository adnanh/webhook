package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultRepository = "xtulnx/webhook"
	manifestName      = "update-manifest.json"
	signatureName     = "update-manifest.json.sig"
	maxManifestSize   = 2 << 20
	maxArchiveSize    = 512 << 20
	maxBinarySize     = 256 << 20
)

var ErrSignatureUnavailable = errors.New("update signature verification is not configured")

type Asset struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion int       `json:"schemaVersion"`
	Version       string    `json:"version"`
	PublishedAt   time.Time `json:"publishedAt"`
	Commit        string    `json:"commit,omitempty"`
	ReleaseURL    string    `json:"releaseURL,omitempty"`
	Assets        []Asset   `json:"assets"`
}

type Result struct {
	CurrentVersion string    `json:"currentVersion"`
	LatestVersion  string    `json:"latestVersion,omitempty"`
	Available      bool      `json:"available"`
	Verified       bool      `json:"verified"`
	PublishedAt    time.Time `json:"publishedAt,omitempty"`
	ReleaseURL     string    `json:"releaseURL,omitempty"`
	CheckedAt      time.Time `json:"checkedAt"`
	Manifest       Manifest  `json:"-"`
}

type State struct {
	CurrentVersion   string    `json:"currentVersion"`
	InstalledVersion string    `json:"installedVersion"`
	Target           string    `json:"target"`
	Backup           string    `json:"backup"`
	SHA256           string    `json:"sha256"`
	AppliedAt        time.Time `json:"appliedAt"`
	RolledBackAt     time.Time `json:"rolledBackAt,omitempty"`
}

type Client struct {
	Repository string
	Version    string
	PublicKey  string
	HTTPClient *http.Client
	BaseURL    string
}

type ApplyOptions struct {
	Version       string
	Target        string
	StateDir      string
	GOOS          string
	GOARCH        string
	SkipProbe     bool
	AllowUnsigned bool
}

func (c *Client) Check(ctx context.Context, requestedVersion string) (Result, error) {
	if err := validateRepository(c.repository()); err != nil {
		return Result{}, err
	}
	if err := validateRequestedVersion(requestedVersion); err != nil {
		return Result{}, err
	}

	manifestURL := c.releaseAssetURL(requestedVersion, manifestName)
	manifestBytes, err := c.download(ctx, manifestURL, maxManifestSize)
	if err != nil {
		return Result{}, fmt.Errorf("download update manifest: %w", err)
	}

	verified := false
	if strings.TrimSpace(c.PublicKey) != "" {
		signatureBytes, err := c.download(ctx, c.releaseAssetURL(requestedVersion, signatureName), maxManifestSize)
		if err != nil {
			return Result{}, fmt.Errorf("download update manifest signature: %w", err)
		}
		if err := verifyManifestSignature(manifestBytes, signatureBytes, c.PublicKey); err != nil {
			return Result{}, err
		}
		verified = true
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Result{}, fmt.Errorf("decode update manifest: %w", err)
	}
	if err := validateManifest(manifest); err != nil {
		return Result{}, err
	}
	if requestedVersion != "" && requestedVersion != "latest" && CompareVersions(manifest.Version, requestedVersion) != 0 {
		return Result{}, fmt.Errorf("update manifest version %s does not match requested version %s", manifest.Version, requestedVersion)
	}

	return Result{
		CurrentVersion: c.Version,
		LatestVersion:  manifest.Version,
		Available:      CompareVersions(manifest.Version, c.Version) > 0,
		Verified:       verified,
		PublishedAt:    manifest.PublishedAt,
		ReleaseURL:     manifest.ReleaseURL,
		CheckedAt:      time.Now().UTC(),
		Manifest:       manifest,
	}, nil
}

func (c *Client) Apply(ctx context.Context, options ApplyOptions) (State, error) {
	if strings.TrimSpace(c.PublicKey) == "" && !options.AllowUnsigned {
		return State{}, ErrSignatureUnavailable
	}

	result, err := c.Check(ctx, options.Version)
	if err != nil {
		return State{}, err
	}
	if !result.Verified && !options.AllowUnsigned {
		return State{}, ErrSignatureUnavailable
	}
	if options.Version == "" && !result.Available {
		return State{}, fmt.Errorf("version %s is not newer than %s", result.LatestVersion, c.Version)
	}

	goos := options.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := options.GOARCH
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	asset, err := findAsset(result.Manifest, goos, goarch)
	if err != nil {
		return State{}, err
	}

	target, err := resolveTarget(options.Target)
	if err != nil {
		return State{}, err
	}
	stateDir := strings.TrimSpace(options.StateDir)
	if stateDir == "" {
		stateDir, err = os.Getwd()
		if err != nil {
			return State{}, fmt.Errorf("locate update state directory: %w", err)
		}
	}
	stateDir, err = filepath.Abs(stateDir)
	if err != nil {
		return State{}, fmt.Errorf("resolve update state directory: %w", err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return State{}, fmt.Errorf("create update state directory: %w", err)
	}

	archive, err := os.CreateTemp(stateDir, ".webhook-update-*.tar.gz")
	if err != nil {
		return State{}, fmt.Errorf("create update archive: %w", err)
	}
	archivePath := archive.Name()
	defer os.Remove(archivePath)

	if err := c.downloadFile(ctx, c.releaseAssetURL(result.Manifest.Version, asset.Name), archive, maxArchiveSize); err != nil {
		archive.Close()
		return State{}, fmt.Errorf("download update archive: %w", err)
	}
	if err := archive.Close(); err != nil {
		return State{}, fmt.Errorf("close update archive: %w", err)
	}
	if err := verifyFile(archivePath, asset); err != nil {
		return State{}, err
	}

	newBinary, err := extractBinary(archivePath, filepath.Dir(target), asset, goos, goarch)
	if err != nil {
		return State{}, err
	}
	defer os.Remove(newBinary)

	if info, statErr := os.Stat(target); statErr == nil {
		if err := os.Chmod(newBinary, info.Mode().Perm()); err != nil {
			return State{}, fmt.Errorf("set update binary permissions: %w", err)
		}
	} else if err := os.Chmod(newBinary, 0o755); err != nil {
		return State{}, fmt.Errorf("set update binary permissions: %w", err)
	}
	if !options.SkipProbe {
		if err := probeBinary(newBinary, result.Manifest.Version); err != nil {
			return State{}, err
		}
	}

	backup := target + ".previous"
	if err := copyFile(target, backup); err != nil {
		return State{}, fmt.Errorf("backup current binary: %w", err)
	}
	if err := replaceExecutable(newBinary, target); err != nil {
		return State{}, fmt.Errorf("replace executable: %w", err)
	}

	state := State{
		CurrentVersion:   c.Version,
		InstalledVersion: result.Manifest.Version,
		Target:           target,
		Backup:           backup,
		SHA256:           strings.ToLower(asset.SHA256),
		AppliedAt:        time.Now().UTC(),
	}
	if err := writeState(stateDir, state); err != nil {
		return state, fmt.Errorf("update installed but state could not be written: %w", err)
	}
	return state, nil
}

func Rollback(target, stateDir string) (State, error) {
	resolvedTarget, err := resolveTarget(target)
	if err != nil {
		return State{}, err
	}
	if strings.TrimSpace(stateDir) == "" {
		stateDir, err = os.Getwd()
		if err != nil {
			return State{}, fmt.Errorf("locate update state directory: %w", err)
		}
	}
	state, err := readState(stateDir)
	if err != nil {
		return State{}, err
	}
	if state.Target != resolvedTarget {
		return State{}, fmt.Errorf("update state belongs to %s, not %s", state.Target, resolvedTarget)
	}
	if _, err := os.Stat(state.Backup); err != nil {
		return State{}, fmt.Errorf("access rollback binary: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(resolvedTarget), ".webhook-rollback-*")
	if err != nil {
		return State{}, fmt.Errorf("create rollback file: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return State{}, err
	}
	defer os.Remove(tmpPath)
	if err := copyFile(state.Backup, tmpPath); err != nil {
		return State{}, fmt.Errorf("prepare rollback binary: %w", err)
	}
	if err := replaceExecutable(tmpPath, resolvedTarget); err != nil {
		return State{}, fmt.Errorf("restore previous binary: %w", err)
	}

	state.RolledBackAt = time.Now().UTC()
	if err := writeState(stateDir, state); err != nil {
		return state, fmt.Errorf("rollback completed but state could not be written: %w", err)
	}
	return state, nil
}

func (c *Client) repository() string {
	repository := strings.TrimSpace(c.Repository)
	if repository == "" {
		return DefaultRepository
	}
	return repository
}

func (c *Client) baseURL() string {
	if strings.TrimSpace(c.BaseURL) != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://github.com/" + c.repository()
}

func (c *Client) releaseAssetURL(version, asset string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "latest" {
		return c.baseURL() + "/releases/latest/download/" + asset
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return c.baseURL() + "/releases/download/" + version + "/" + asset
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *Client) download(ctx context.Context, url string, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "webhook-updater/"+c.Version)
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return data, nil
}

func (c *Client) downloadFile(ctx context.Context, url string, destination io.Writer, limit int64) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "webhook-updater/"+c.Version)
	response, err := c.httpClient().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %s", response.Status)
	}
	if response.ContentLength > limit {
		return fmt.Errorf("archive exceeds %d bytes", limit)
	}
	written, err := io.Copy(destination, io.LimitReader(response.Body, limit+1))
	if err != nil {
		return err
	}
	if written > limit {
		return fmt.Errorf("archive exceeds %d bytes", limit)
	}
	return nil
}

func verifyManifestSignature(manifest, encodedSignature []byte, encodedPublicKey string) error {
	publicKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedPublicKey))
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return errors.New("invalid update manifest public key")
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encodedSignature)))
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("invalid update manifest signature encoding")
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), manifest, signature) {
		return errors.New("update manifest signature verification failed")
	}
	return nil
}

func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("unsupported update manifest schema %d", manifest.SchemaVersion)
	}
	if _, ok := parseVersion(manifest.Version); !ok {
		return fmt.Errorf("invalid release version %q", manifest.Version)
	}
	if len(manifest.Assets) == 0 {
		return errors.New("update manifest contains no assets")
	}
	for _, asset := range manifest.Assets {
		if asset.OS == "" || asset.Arch == "" || filepath.Base(asset.Name) != asset.Name || asset.Size <= 0 {
			return fmt.Errorf("invalid update asset %q", asset.Name)
		}
		decoded, err := hex.DecodeString(asset.SHA256)
		if err != nil || len(decoded) != sha256.Size {
			return fmt.Errorf("invalid SHA256 for update asset %q", asset.Name)
		}
	}
	return nil
}

func validateRepository(repository string) error {
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid update repository %q", repository)
	}
	for _, part := range parts {
		for _, ch := range part {
			if !(ch >= 'a' && ch <= 'z') && !(ch >= 'A' && ch <= 'Z') && !(ch >= '0' && ch <= '9') && ch != '-' && ch != '_' && ch != '.' {
				return fmt.Errorf("invalid update repository %q", repository)
			}
		}
	}
	return nil
}

func validateRequestedVersion(version string) error {
	version = strings.TrimSpace(version)
	if version == "" || version == "latest" {
		return nil
	}
	if _, ok := parseVersion(version); !ok {
		return fmt.Errorf("invalid requested update version %q", version)
	}
	return nil
}

func findAsset(manifest Manifest, goos, goarch string) (Asset, error) {
	for _, asset := range manifest.Assets {
		if asset.OS == goos && asset.Arch == goarch {
			return asset, nil
		}
	}
	return Asset{}, fmt.Errorf("release %s has no asset for %s/%s", manifest.Version, goos, goarch)
}

func verifyFile(path string, asset Asset) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return err
	}
	if size != asset.Size {
		return fmt.Errorf("update archive size mismatch: got %d, want %d", size, asset.Size)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, asset.SHA256) {
		return errors.New("update archive SHA256 mismatch")
	}
	return nil
}

func extractBinary(archivePath, destinationDir string, asset Asset, goos, goarch string) (string, error) {
	archive, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return "", fmt.Errorf("open update archive: %w", err)
	}
	defer gzipReader.Close()

	binaryName := "webhook"
	if goos == "windows" {
		binaryName += ".exe"
	}
	wanted := filepath.ToSlash("webhook-" + goos + "-" + goarch + "/" + binaryName)
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read update archive: %w", err)
		}
		cleanName := filepath.ToSlash(filepath.Clean(header.Name))
		if cleanName != wanted {
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > maxBinarySize {
			return "", errors.New("update archive contains an invalid executable")
		}
		tmp, err := os.CreateTemp(destinationDir, ".webhook-new-*")
		if err != nil {
			return "", err
		}
		tmpPath := tmp.Name()
		written, copyErr := io.Copy(tmp, io.LimitReader(tarReader, maxBinarySize+1))
		closeErr := tmp.Close()
		if copyErr != nil || closeErr != nil || written != header.Size {
			os.Remove(tmpPath)
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
			return "", errors.New("update executable size mismatch")
		}
		return tmpPath, nil
	}
	return "", fmt.Errorf("update archive does not contain %s", wanted)
}

func resolveTarget(target string) (string, error) {
	var err error
	if strings.TrimSpace(target) == "" {
		target, err = os.Executable()
		if err != nil {
			return "", fmt.Errorf("locate current executable: %w", err)
		}
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve update target: %w", err)
	}
	if resolved, evalErr := filepath.EvalSymlinks(target); evalErr == nil {
		target = resolved
	}
	return target, nil
}

func probeBinary(path, expectedVersion string) error {
	command := exec.Command(path, "-version")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run downloaded executable: %w", err)
	}
	if !strings.Contains(string(output), strings.TrimPrefix(expectedVersion, "v")) {
		return fmt.Errorf("downloaded executable reported an unexpected version: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func copyFile(source, destination string) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), filepath.Base(destination)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceFile(tmpPath, destination)
}

func statePath(stateDir string) string {
	return filepath.Join(stateDir, ".webhook-update.json")
}

func writeState(stateDir string, state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(stateDir, ".webhook-update-state-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceFile(tmpPath, statePath(stateDir))
}

func readState(stateDir string) (State, error) {
	data, err := os.ReadFile(statePath(stateDir))
	if err != nil {
		return State{}, fmt.Errorf("read update state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decode update state: %w", err)
	}
	return state, nil
}

type parsedVersion struct {
	major int
	minor int
	patch int
	pre   []string
}

func CompareVersions(left, right string) int {
	l, lok := parseVersion(left)
	r, rok := parseVersion(right)
	if !lok && !rok {
		return strings.Compare(left, right)
	}
	if !lok {
		return -1
	}
	if !rok {
		return 1
	}
	for _, pair := range [][2]int{{l.major, r.major}, {l.minor, r.minor}, {l.patch, r.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if len(l.pre) == 0 && len(r.pre) != 0 {
		return 1
	}
	if len(l.pre) != 0 && len(r.pre) == 0 {
		return -1
	}
	for i := 0; i < len(l.pre) && i < len(r.pre); i++ {
		if l.pre[i] == r.pre[i] {
			continue
		}
		li, lerr := strconv.Atoi(l.pre[i])
		ri, rerr := strconv.Atoi(r.pre[i])
		switch {
		case lerr == nil && rerr == nil:
			if li < ri {
				return -1
			}
			return 1
		case lerr == nil:
			return -1
		case rerr == nil:
			return 1
		default:
			return strings.Compare(l.pre[i], r.pre[i])
		}
	}
	if len(l.pre) < len(r.pre) {
		return -1
	}
	if len(l.pre) > len(r.pre) {
		return 1
	}
	return 0
}

func parseVersion(value string) (parsedVersion, bool) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	if buildIndex := strings.IndexByte(value, '+'); buildIndex >= 0 {
		value = value[:buildIndex]
	}
	var pre []string
	if preIndex := strings.IndexByte(value, '-'); preIndex >= 0 {
		pre = strings.Split(value[preIndex+1:], ".")
		value = value[:preIndex]
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return parsedVersion{}, false
	}
	numbers := make([]int, 3)
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return parsedVersion{}, false
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return parsedVersion{}, false
		}
		numbers[i] = number
	}
	for _, identifier := range pre {
		if identifier == "" {
			return parsedVersion{}, false
		}
	}
	return parsedVersion{major: numbers[0], minor: numbers[1], patch: numbers[2], pre: pre}, true
}
