// Design: docs/architecture/host/inventory.md -- offline `show host` fallback

// Package host is the offline fallback for `show host [section]`. It reads the
// hardware inventory directly from sysfs/procfs (no daemon required) and writes
// JSON to stdout, the same shape as the daemon `show host *` RPC. There is no
// `--text` mode: the verb-first daemon grammar has no such flag, so keeping one
// only offline would be an online/offline inconsistency. For a human-readable
// view, pipe the JSON through `ze pipe table` or `jq`.
//
// Sibling consumers of the same inventory library:
//
//	internal/plugins/host-cmd/cmd/show_host.go  online `show host *` RPCs
//	internal/component/host/inventory.go         detection library
package host

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"

	hostinv "github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// RunShow implements `show host [section]` as the offline fallback. Output is
// JSON (machine-parseable, identical to the daemon RPC). Returns 0 on success,
// 1 on argument / IO error.
func RunShow(args []string) int {
	fs := flag.NewFlagSet("show host", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		var tb textbuf.Buffer
		os.Stderr.WriteString(tb.Str("Usage: ze show host [section]\n\nSections: ").Str(sectionList()).Str("\n\nDefault section is 'all'. Output is JSON.\n").String()) //nolint:errcheck // usage output
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	positional := fs.Args()
	section := "all"
	if len(positional) > 0 {
		section = strings.ToLower(positional[0])
	}

	data, err := hostinv.DetectSection(section)
	if err != nil {
		var tb textbuf.Buffer
		if errors.Is(err, hostinv.ErrUnknownSection) {
			os.Stderr.WriteString(tb.Str("error: unknown section ").Quoted(section).Str("; valid: ").Str(sectionList()).Str("\n").String()) //nolint:errcheck // CLI error
			return 1
		}
		os.Stderr.WriteString(tb.Str("error: ").Err(err).Str("\n").String()) //nolint:errcheck // CLI error
		return 1
	}

	return renderJSON(data)
}

// renderJSON writes the inventory as indented JSON to stdout.
func renderJSON(data any) int {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		var tb textbuf.Buffer
		os.Stderr.WriteString(tb.Str("error: marshal: ").Err(err).Str("\n").String()) //nolint:errcheck // CLI error
		return 1
	}
	if _, err := os.Stdout.Write(append(b, '\n')); err != nil {
		var tb textbuf.Buffer
		os.Stderr.WriteString(tb.Str("error: write: ").Err(err).Str("\n").String()) //nolint:errcheck // CLI error
		return 1
	}
	return 0
}

// sectionList returns the sorted, comma-separated list of valid section names
// for error messages and help output. Thin alias over host.SectionList() so the
// canonical source stays in the host package (rules/derive-not-hardcode.md).
func sectionList() string {
	return hostinv.SectionList()
}
