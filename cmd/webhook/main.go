// Command webhook serves HTTP hooks and optional network tunnels.
package main

import "github.com/adnanh/webhook/internal/app"

// Keep these in main so existing release -ldflags continue to work.
var version = "2.8.3"
var updateManifestPublicKey string

func main() {
	app.Main(version, updateManifestPublicKey)
}
