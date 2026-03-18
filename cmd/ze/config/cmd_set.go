// Design: docs/architecture/config/syntax.md — config set command
// Overview: main.go — dispatch and exit codes

package config

import (
	"flag"
	"fmt"
	"os"
	"strings"

	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
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
	noReload := fs.Bool("no-reload", false, "do not notify running daemon after save")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config set [options] <config-file> <path...> <value>

Set a configuration value in a config file.

The last argument is the value, the second-to-last is the leaf name,
and everything between the config file and the leaf is the container path.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  ze config set config.conf bgp local-as 65000
  ze config set config.conf bgp peer 1.1.1.1 local-as 65001
  ze config set config.conf bgp peer 1.1.1.1 description "my peer"
  ze config set --dry-run config.conf bgp hold-time 90
`)
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

	// For filesystem storage, check file exists
	if !storage.IsBlobStorage(store) {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error: config file not found: %s\n", configPath)
			return exitError
		}
	}

	ed, err := cli.NewEditorWithStorage(store, configPath)
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

	displayPath := strings.Join(path, " ")

	if *dryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would set %s = %s\n", displayPath, value)
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

	fmt.Fprintf(os.Stderr, "set %s = %s\n", displayPath, value)

	// Notify daemon (best-effort) via SSH
	if !*noReload {
		creds, credErr := sshclient.LoadCredentials()
		if credErr == nil {
			ed.SetReloadNotifier(func() error {
				_, reloadErr := sshclient.ExecCommand(creds, "reload")
				return reloadErr
			})
			if notifyErr := ed.NotifyReload(); notifyErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not notify daemon: %v\n", notifyErr)
			}
		}
	}

	return exitOK
}
