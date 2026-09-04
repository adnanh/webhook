package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{left: "v2.8.4", right: "2.8.3", want: 1},
		{left: "2.8.3", right: "v2.8.3", want: 0},
		{left: "2.8.3-rc.1", right: "2.8.3", want: -1},
		{left: "2.8.3-rc.2", right: "2.8.3-rc.1", want: 1},
		{left: "dev", right: "2.8.3", want: -1},
	}
	for _, test := range tests {
		if got := CompareVersions(test.left, test.right); normalizeComparison(got) != test.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestCheckVerifiesSignedManifest(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testManifest(t, Asset{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		Name:   "webhook-test.tar.gz",
		Size:   1,
		SHA256: hex.EncodeToString(make([]byte, sha256.Size)),
	})
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, manifest))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch filepath.Base(r.URL.Path) {
		case manifestName:
			_, _ = w.Write(manifest)
		case signatureName:
			_, _ = w.Write([]byte(signature))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := Client{
		Repository: DefaultRepository,
		Version:    "2.8.3",
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}
	result, err := client.Check(context.Background(), "")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !result.Verified || !result.Available || result.LatestVersion != "v2.8.4" {
		t.Fatalf("unexpected check result: %+v", result)
	}
}

func TestApplyAndRollback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test executable is a POSIX shell script")
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	binary := []byte("#!/bin/sh\necho 'webhook version 2.8.4'\n")
	archive := testArchive(t, runtime.GOOS, runtime.GOARCH, binary)
	digest := sha256.Sum256(archive)
	asset := Asset{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		Name:   "webhook-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz",
		Size:   int64(len(archive)),
		SHA256: hex.EncodeToString(digest[:]),
	}
	manifest := testManifest(t, asset)
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, manifest))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch filepath.Base(r.URL.Path) {
		case manifestName:
			_, _ = w.Write(manifest)
		case signatureName:
			_, _ = w.Write([]byte(signature))
		case asset.Name:
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	directory := t.TempDir()
	target := filepath.Join(directory, "webhook")
	oldBinary := []byte("#!/bin/sh\necho 'webhook version 2.8.3'\n")
	if err := os.WriteFile(target, oldBinary, 0o755); err != nil {
		t.Fatal(err)
	}

	client := Client{
		Repository: DefaultRepository,
		Version:    "2.8.3",
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}
	state, err := client.Apply(context.Background(), ApplyOptions{Target: target, StateDir: directory})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if state.InstalledVersion != "v2.8.4" {
		t.Fatalf("installed version = %q", state.InstalledVersion)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, binary) {
		t.Fatal("target does not contain the new binary")
	}

	if _, err := Rollback(target, directory); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	got, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, oldBinary) {
		t.Fatal("target does not contain the restored binary")
	}
}

func testManifest(t *testing.T, asset Asset) []byte {
	t.Helper()
	manifest := Manifest{
		SchemaVersion: 1,
		Version:       "v2.8.4",
		PublishedAt:   time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC),
		Commit:        "abc123",
		ReleaseURL:    "https://github.com/xtulnx/webhook/releases/tag/v2.8.4",
		Assets:        []Asset{asset},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testArchive(t *testing.T, goos, goarch string, binary []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	name := "webhook-" + goos + "-" + goarch + "/webhook"
	if goos == "windows" {
		name += ".exe"
	}
	if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func normalizeComparison(value int) int {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}
