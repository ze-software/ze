// Design: docs/architecture/config/syntax.md — config set command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func cmdSetWithStorage(store storage.Storage, args []string) int {
	return cmdSetImpl(store, args)
}

func cmdSet(args []string) int {
	return cmdSetImpl(storage.NewFilesystem(), args)
}

func cmdSetImpl(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config set", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "show what would change without writing")
	reload := fs.Bool("reload", false, "notify the running daemon to reload after save")
	user := fs.String("user", "", "SSH login username (overrides zefs super-admin)")
	fs.StringVar(user, "u", "", "Short alias for --user")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config set",
			Summary: "Set a configuration value in a config file",
			Usage:   []string{"ze config set [options] <config-file> <path...> <value>"},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionDescription, Entries: []helpfmt.HelpEntry{
					{Name: "", Desc: "The last argument is the value, the second-to-last is the leaf name,"},
					{Name: "", Desc: "and everything between the config file and the leaf is the container path."},
				}},
				{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
					{Name: helpFlagDryRun, Desc: "Show what would change without writing"},
					{Name: "--reload", Desc: "Notify the running daemon to reload after save"},
				}},
			},
			Examples: []string{
				"ze config set config.conf bgp local as 65000",
				"ze config set config.conf bgp peer peer1 remote as 65001",
				"ze config set config.conf bgp peer peer1 description \"my peer\"",
				"ze config set --dry-run config.conf bgp peer peer1 timer receive-hold-time 90",
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	// Need at least: <file> <key> <value> (minimum 3 positional args)
	if fs.NArg() < 3 {
		fmt.Fprintf(os.Stderr, "error: requires <config-file> <path...> <value>\n")
		fs.Usage()
		return exitError
	}

	configPath := fs.Arg(0)
	setArgs := fs.Args()[1:] // everything after config file

	// Parse path: last = value, second-to-last = key, rest = container path
	value := setArgs[len(setArgs)-1]
	path := setArgs[:len(setArgs)-1]
	key := path[len(path)-1]
	containerPath := path[:len(path)-1]

	// For filesystem storage, check file exists (stdin "-" has no path to stat).
	if !cliio.IsStdin(configPath) && !storage.IsBlobStorage(store) {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error: config file not found: %s\n", configPath)
			return exitError
		}
	}

	ed, err := openEditableConfig(store, configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	defer ed.Close() //nolint:errcheck // best-effort cleanup

	// Validate value against YANG schema
	completer := cli.NewCompleter()
	completer.SetTree(ed.Tree())
	if err := completer.ValidateValueAtPath(path, value); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	// Apply the set
	if err := ed.SetValue(containerPath, key, value); err != nil {
		fmt.Fprintf(os.Stderr, "error: set failed: %v\n", err)
		return exitError
	}

	displayPath := textbuf.Join(path, " ")

	if *dryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would set %s %s\n", displayPath, value)
		diff := ed.Diff()
		if diff != "" {
			fmt.Fprint(os.Stderr, diff)
		}
		return exitOK
	}

	// Save (creates backup automatically)
	if err := ed.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "error: save failed: %v\n", err)
		return exitError
	}

	fmt.Fprintf(os.Stderr, "set %s %s\n", displayPath, value)

	// Editing a stored config does not contact the daemon by default; --reload
	// opts in. See notifyDaemonReload.
	notifyDaemonReload(ed, *reload, configPath, *user)

	return exitOK
}
