// Design: docs/architecture/config/syntax.md — config deactivate command
// Detail: cmd_set.go — same one-shot pattern (flags, editor, save, notify)
// Detail: ../../cli/editor_activation.go — ApplyActivation, the dispatch every editor shares

package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func cmdDeactivateWithStorage(store storage.Storage, args []string) int {
	return cmdDeactivateImpl(store, args)
}

func cmdActivateWithStorage(store storage.Storage, args []string) int {
	return cmdActivateImpl(store, args)
}

func cmdDeactivateImpl(store storage.Storage, args []string) int {
	return runDeactivateLike(store, args, false)
}

func cmdActivateImpl(store storage.Storage, args []string) int {
	return runDeactivateLike(store, args, true)
}

// runDeactivateLike implements both `deactivate` and `activate`. The two
// verbs share flag parsing, path resolution, and post-save notification;
// only the Editor mutation method differs, so factoring keeps the diff
// small and the help texts comparable.
func runDeactivateLike(store storage.Storage, args []string, activate bool) int {
	verb := "deactivate"
	if activate {
		verb = "activate"
	}

	fs := flag.NewFlagSet("config "+verb, flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "show what would change without writing")
	reload := fs.Bool("reload", false, "notify the running daemon to reload after save")
	user := fs.String("user", "", "SSH login username (overrides zefs super-admin)")
	fs.StringVar(user, "u", "", "Short alias for --user")

	fs.Usage = func() {
		summary := "Mark a configuration node inactive (kept in file, skipped at apply)"
		if activate {
			summary = "Clear the inactive flag on a configuration node"
		}
		var tb textbuf.Buffer
		p := helpfmt.Page{
			Command:   tb.Str("ze config ").Str(verb).String(),
			ShortHelp: summary,
			Usage:     []string{tb.Reset().Str("ze config ").Str(verb).Str(" [options] <config-file> <path...>").String()},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionDescription, Entries: []helpfmt.HelpEntry{
					{Name: "", Desc: "Targets a leaf, container, list entry, or leaf-list value."},
					{Name: "", Desc: "The deactivated node round-trips through save/load and is skipped at apply time."},
				}},
				{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
					{Name: helpFlagDryRun, Desc: "Show what would change without writing"},
					{Name: "--reload", Desc: "Notify the running daemon to reload after save"},
				}},
			},
			Examples: []string{
				tb.Reset().Str("ze config ").Str(verb).Str(" router.conf bgp router-id").String(),
				tb.Reset().Str("ze config ").Str(verb).Str(" router.conf bgp peer peer1").String(),
				tb.Reset().Str("ze config ").Str(verb).Str(" router.conf bgp filter import no-self-as").String(),
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}
	if fs.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "error: requires <config-file> <path...>\n")
		fs.Usage()
		return exitError
	}

	configPath := fs.Arg(0)
	path := fs.Args()[1:]

	ed, err := openEditableConfig(store, configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	defer ed.Close() //nolint:errcheck // best-effort cleanup

	displayPath := textbuf.Join(path, " ")
	// ApplyActivation is the dispatch the SSH and web editors use, so the
	// offline verb accepts and refuses the same paths they do.
	status, err := ed.ApplyActivation(path, activate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	if !ed.Dirty() {
		// The node already held the requested state: idempotent, AC-8.
		fmt.Fprintf(os.Stderr, "%s; nothing to do\n", status)
		return exitOK
	}

	if *dryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would %s %s\n", verb, displayPath)
		if diff := ed.Diff(); diff != "" {
			fmt.Fprint(os.Stderr, diff)
		}
		return exitOK
	}

	warnings, err := ed.Save()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: save failed: %v\n", err)
		return exitError
	}

	printCommitWarnings(warnings)
	noticeUnrecordedVersion(ed, configPath)
	fmt.Fprintf(os.Stderr, "%s\n", status)

	// Editing a stored config does not contact the daemon by default; --reload
	// opts in. See notifyDaemonReload.
	notifyDaemonReload(ed, *reload, configPath, *user)

	return exitOK
}
