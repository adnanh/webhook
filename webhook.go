// This compatibility entrypoint preserves go build and go run . at the module root.
// Command webhook serves HTTP hooks and optional network tunnels.
package main

import (
	"github.com/xtulnx/webhook/internal/app"
	"github.com/xtulnx/webhook/internal/buildinfo"
)

// Keep these in main so existing release -ldflags continue to work.
var version = buildinfo.Version
var updateManifestPublicKey string

func main() {
	app.Main(version, updateManifestPublicKey)
}
