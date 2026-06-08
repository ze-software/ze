// Design: docs/architecture/zefs-format.md -- debug flags stored as zefs keys
// Overview: register.go -- CLI command registration

package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"codeberg.org/thomas-mangin/ze/internal/core/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

const (
	defaultBlobName = "database.zefs"
	nameAll         = "all"
)

// blobPathOverride allows tests to redirect storage to a temp file.
// Empty means use the default path.
var blobPathOverride string

// Run executes the debug subcommand. Returns exit code.
func Run(args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "enable":
		return cmdEnable(subArgs)
	case "disable":
		return cmdDisable(subArgs)
	case "show":
		return cmdShow(subArgs)
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown debug subcommand: %s\n", subcmd)
		usage()
		return 1
	}
}

func cmdEnable(args []string) int {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: ze debug enable <subsystem|all>\n")
		return 1
	}
	name := args[0]

	if !slogutil.ValidateSubsystem(name) {
		return printInvalidSubsystem(name)
	}

	store, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer store.Close() //nolint:errcheck // best-effort close

	key := debugKey(name)
	if err := store.WriteFile(key, []byte("on"), 0); err != nil {
		fmt.Fprintf(os.Stderr, "error: write %s: %v\n", key, err)
		return 2
	}

	applyEnable(name)
	fmt.Fprintf(os.Stdout, "debug %s: enabled\n", name) //nolint:errcheck // stdout
	return 0
}

func cmdDisable(args []string) int {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: ze debug disable <subsystem|all>\n")
		return 1
	}
	name := args[0]

	if !slogutil.ValidateSubsystem(name) {
		return printInvalidSubsystem(name)
	}

	store, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer store.Close() //nolint:errcheck // best-effort close

	key := debugKey(name)
	if err := store.WriteFile(key, []byte("off"), 0); err != nil {
		fmt.Fprintf(os.Stderr, "error: write %s: %v\n", key, err)
		return 2
	}

	applyDisable(name, store)
	fmt.Fprintf(os.Stdout, "debug %s: disabled\n", name) //nolint:errcheck // stdout
	return 0
}

func cmdShow(_ []string) int {
	store, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer store.Close() //nolint:errcheck // best-effort close

	states := slogutil.ResolveDebugStates(store)

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	printRow(w, "SUBSYSTEM", "DEBUG", "SOURCE")
	printRow(w, "---------", "-----", "------")
	for _, s := range states {
		state := "off"
		if s.Enabled {
			state = "on"
		}
		printRow(w, s.Name, state, string(s.Source))
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func printRow(w *tabwriter.Writer, cols ...string) {
	if _, err := fmt.Fprintln(w, textbuf.Join(cols, "\t")); err != nil { //nolint:errcheck // output
		return
	}
}

func debugKey(name string) string {
	if name == nameAll {
		return zefs.KeyDebugAll.Key()
	}
	return zefs.KeyDebugSubsystem.Key(name)
}

func applyEnable(name string) {
	if name == nameAll {
		for _, info := range slogutil.Subsystems() {
			_ = slogutil.SetLevel(info.Name, "debug")
		}
		return
	}
	for _, sub := range slogutil.SubsystemsMatching(name) {
		_ = slogutil.SetLevel(sub, "debug")
	}
}

func applyDisable(name string, store *zefs.BlobStore) {
	if name == nameAll {
		// Re-resolve: per-subsystem "on" keys should stay at debug after global disable.
		states := slogutil.ResolveDebugStates(store)
		for _, s := range states {
			if s.Enabled {
				_ = slogutil.SetLevel(s.Name, "debug")
			} else {
				slogutil.RestoreLevel(s.Name)
			}
		}
		return
	}
	for _, sub := range slogutil.SubsystemsMatching(name) {
		slogutil.RestoreLevel(sub)
	}
}

func printInvalidSubsystem(name string) int {
	fmt.Fprintf(os.Stderr, "error: unknown subsystem %q\n", name)
	fmt.Fprintf(os.Stderr, "valid subsystems:\n")
	for _, info := range slogutil.Subsystems() {
		if info.Description != "" {
			fmt.Fprintf(os.Stderr, "  %-30s %s\n", info.Name, info.Description)
		} else {
			fmt.Fprintf(os.Stderr, "  %s\n", info.Name)
		}
	}
	fmt.Fprintf(os.Stderr, "\nhierarchical prefixes also work (e.g., \"bgp\" enables all bgp.* subsystems)\n")
	return 1
}

func openStore() (*zefs.BlobStore, error) {
	storePath := blobPath()
	if _, err := os.Stat(storePath); err == nil {
		return zefs.Open(storePath)
	}
	return zefs.Create(storePath)
}

func blobPath() string {
	if blobPathOverride != "" {
		return blobPathOverride
	}
	configDir := paths.DefaultConfigDir()
	if configDir == "" {
		return defaultBlobName
	}
	return filepath.Join(configDir, defaultBlobName)
}

func usage() {
	p := helpfmt.Page{
		Command: "ze debug",
		Summary: "Runtime debug flags (persistent, stored in ZeFS)",
		Usage:   []string{"ze debug <command> [args...]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: "enable <subsystem|all>", Desc: "Enable debug logging for a subsystem or all"},
				{Name: "disable <subsystem|all>", Desc: "Disable debug logging for a subsystem or all"},
				{Name: "show", Desc: "Show debug state for all subsystems"},
			}},
		},
		Examples: []string{
			"ze debug enable bgp",
			"ze debug enable all",
			"ze debug disable bgp",
			"ze debug show",
		},
	}
	p.Write()
}
