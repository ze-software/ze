// Design: plan/spec-diag-crash-capture.md -- offline crash file viewer
//
// Package crashes provides `ze crashes show [latest|<name>]`. Works offline
// (no daemon required) by reading crash files directly from disk.

package crashes

import (
	"encoding/json"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/core/crashlog"
)

func RunHint(_ []string) int {
	os.Stdout.WriteString("Usage: ze crashes show [latest|<filename>]\n") //nolint:errcheck // CLI hint
	return 0
}

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

func showList() int {
	summaries := crashlog.ListCrashes()
	if len(summaries) == 0 {
		os.Stdout.WriteString("no crashes recorded (dir: " + crashlog.CrashDir() + ")\n") //nolint:errcheck // CLI output
		return 0
	}

	entries := make([]map[string]any, 0, len(summaries))
	for _, s := range summaries {
		entries = append(entries, map[string]any{
			"name": s.Name,
			"size": s.Size,
		})
	}

	out := map[string]any{
		"crashes": entries,
		"count":   len(entries),
		"dir":     crashlog.CrashDir(),
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
