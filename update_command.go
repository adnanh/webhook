package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	webhookupdate "github.com/adnanh/webhook/internal/update"
)

var (
	updateEnabled    = flag.Bool("update-enabled", true, "enable update checks in the admin API")
	updateRepository = flag.String("update-repository", webhookupdate.DefaultRepository, "GitHub repository used for updates")
	updateStateDir   = flag.String("update-state-dir", "", "directory for update state; defaults to the executable directory")

	updateManifestPublicKey string
)

func isUpdateCommand(args []string) bool {
	return len(args) > 1 && args[1] == "update"
}

func newUpdateClient(repository string) *webhookupdate.Client {
	return &webhookupdate.Client{
		Repository: repository,
		Version:    version,
		PublicKey:  updateManifestPublicKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func runUpdateCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeUpdateUsage(stderr)
		return 2
	}
	if args[0] == "-h" || args[0] == "--help" {
		writeUpdateUsage(stdout)
		return 0
	}

	switch args[0] {
	case "check":
		return runUpdateCheck(args[1:], stdout, stderr)
	case "apply":
		return runUpdateApply(args[1:], stdin, stdout, stderr)
	case "rollback":
		return runUpdateRollback(args[1:], stdout, stderr)
	case "help":
		writeUpdateUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown update command %q\n", args[0])
		writeUpdateUsage(stderr)
		return 2
	}
}

func runUpdateCheck(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("webhook update check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repository := flags.String("repository", webhookupdate.DefaultRepository, "GitHub repository")
	requestedVersion := flags.String("version", "", "release version; defaults to latest")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "update check does not accept positional arguments")
		return 2
	}

	result, err := newUpdateClient(*repository).Check(context.Background(), *requestedVersion)
	if err != nil {
		fmt.Fprintln(stderr, "update check failed:", err)
		return 1
	}
	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintln(stderr, "encode update result:", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "Current version: %s\n", result.CurrentVersion)
	fmt.Fprintf(stdout, "Latest version:  %s\n", result.LatestVersion)
	if result.Available {
		fmt.Fprintln(stdout, "Update available: yes")
	} else {
		fmt.Fprintln(stdout, "Update available: no")
	}
	if result.Verified {
		fmt.Fprintln(stdout, "Manifest signature: verified")
	} else {
		fmt.Fprintln(stdout, "Manifest signature: unavailable in this build")
	}
	return 0
}

func runUpdateApply(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("webhook update apply", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repository := flags.String("repository", webhookupdate.DefaultRepository, "GitHub repository")
	requestedVersion := flags.String("version", "", "release version; defaults to latest")
	target := flags.String("target", "", "executable to replace; defaults to the current executable")
	stateDir := flags.String("state-dir", "", "update state directory; defaults to the current working directory")
	yes := flags.Bool("yes", false, "apply without interactive confirmation")
	allowUnsigned := flags.Bool("allow-unsigned", false, "allow applying a manifest without a configured signature key")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "update apply does not accept positional arguments")
		return 2
	}

	client := newUpdateClient(*repository)
	result, err := client.Check(context.Background(), *requestedVersion)
	if err != nil {
		fmt.Fprintln(stderr, "update check failed:", err)
		return 1
	}
	if !result.Verified && !*allowUnsigned {
		fmt.Fprintln(stderr, "update cannot be applied because this build has no trusted manifest public key")
		return 1
	}
	if *requestedVersion == "" && !result.Available {
		fmt.Fprintf(stdout, "webhook %s is already up to date.\n", version)
		return 0
	}
	if !*yes {
		fmt.Fprintf(stdout, "Replace webhook %s with %s? [y/N] ", version, result.LatestVersion)
		answer, _ := bufio.NewReader(stdin).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(stdout, "Update cancelled.")
			return 0
		}
	}

	state, err := client.Apply(context.Background(), webhookupdate.ApplyOptions{
		Version:       result.LatestVersion,
		Target:        *target,
		StateDir:      *stateDir,
		AllowUnsigned: *allowUnsigned,
	})
	if err != nil {
		fmt.Fprintln(stderr, "update failed:", err)
		return 1
	}
	fmt.Fprintf(stdout, "Updated webhook from %s to %s.\n", state.CurrentVersion, state.InstalledVersion)
	fmt.Fprintln(stdout, "Restart the running webhook service to activate the new version.")
	return 0
}

func runUpdateRollback(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("webhook update rollback", flag.ContinueOnError)
	flags.SetOutput(stderr)
	target := flags.String("target", "", "executable to restore; defaults to the current executable")
	stateDir := flags.String("state-dir", "", "update state directory; defaults to the current working directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "update rollback does not accept positional arguments")
		return 2
	}

	state, err := webhookupdate.Rollback(*target, *stateDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(stderr, "rollback is unavailable: no previous update state was found")
		} else {
			fmt.Fprintln(stderr, "rollback failed:", err)
		}
		return 1
	}
	fmt.Fprintf(stdout, "Restored webhook %s over %s.\n", state.CurrentVersion, state.InstalledVersion)
	fmt.Fprintln(stdout, "Restart the running webhook service to activate the restored version.")
	return 0
}

func writeUpdateUsage(output io.Writer) {
	fmt.Fprintln(output, "Usage:")
	fmt.Fprintln(output, "  webhook update check [--repository owner/name] [--version vX.Y.Z] [--json]")
	fmt.Fprintln(output, "  webhook update apply [--repository owner/name] [--version vX.Y.Z] [--state-dir path] [--yes] [--allow-unsigned]")
	fmt.Fprintln(output, "  webhook update rollback [--state-dir path]")
}
