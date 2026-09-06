// Design: docs/architecture/diagnostics/crash-capture.md -- offline crash file viewer
//
// Package crashes provides the in-process offline fallback for
// `show crashes [latest | name <file>]`. It reads crash files directly from
// disk, so it works when no daemon is reachable -- the situation you are in
// when inspecting a crash, since the daemon has died.

package crashes

import (
	"encoding/json"
	"os"

	"github.com/ze-software/ze/internal/core/crashlog"
)

func RunShow(args []string) int {
	crashlog.Init()

	if len(args) == 0 {
		return showList()
	}

	switch args[0] {
	case "latest":
		return showFile(crashlog.LatestCrash())
	default:
		return showFile(crashlog.ReadCrash(args[0]))
	}
}

// showList answers the same shape the daemon answers, built from the same two
// calls. The fallback runs when the daemon has died, which is exactly when an
// operator reads a crash report, so a different shape here would mean the answer
// changes with the health of the box.
func showList() int {
	entries := crashlog.CrashListFields()
	out := map[string]any{
		componentName: entries,
		"count":       len(entries),
		"dir":         crashlog.CrashDir(),
		"readiness":   Readiness("").Fields(),
	}
	if len(entries) == 0 {
		out["message"] = "no crashes recorded"
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		os.Stderr.WriteString("error: " + err.Error() + "\n") //nolint:errcheck // CLI error
		return 1
	}
	return 0
}

func showFile(content string) int {
	if content == "" {
		os.Stdout.WriteString("no crash report found\n") //nolint:errcheck // CLI output
		return 1
	}
	os.Stdout.WriteString(content) //nolint:errcheck // CLI output
	return 0
}
