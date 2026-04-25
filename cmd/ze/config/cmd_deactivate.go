// Design: docs/architecture/config/syntax.md — config deactivate command
// Detail: cmd_set.go — same one-shot pattern (flags, editor, save, notify)
// Detail: ../../../internal/component/cli/model_commands.go — TUI cmdDeactivate dispatch we mirror

package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// alreadyInRequestedState reports whether err signals that the target
// node was already in the state we asked for. The CLI verb treats this
// as success (idempotent, AC-8) and emits a "no change" status. The
// editor exports sentinel errors specifically for this matching so we
// don't have to compare error strings.
func alreadyInRequestedState(err error) bool {
	return errors.Is(err, cli.ErrLeafAlreadyInactive) ||
		errors.Is(err, cli.ErrLeafNotInactive) ||
		errors.Is(err, cli.ErrPathAlreadyInactive) ||
		errors.Is(err, cli.ErrPathNotInactive)
}

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
	noReload := fs.Bool("no-reload", false, "do not notify running daemon after save")
	user := fs.String("user", "", "SSH login username (overrides zefs super-admin)")
	fs.StringVar(user, "u", "", "Short alias for --user")

	fs.Usage = func() {
		summary := "Mark a configuration node inactive (kept in file, skipped at apply)"
		if activate {
			summary = "Clear the inactive flag on a configuration node"
		}
		p := helpfmt.Page{
			Command: "ze config " + verb,
			Summary: summary,
			Usage:   []string{"ze config " + verb + " [options] <config-file> <path...>"},
			Sections: []helpfmt.HelpSection{
				{Title: "Description", Entries: []helpfmt.HelpEntry{
					{Name: "", Desc: "Targets a leaf, container, list entry, or leaf-list value."},
					{Name: "", Desc: "The deactivated node round-trips through save/load and is skipped at apply time."},
				}},
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--dry-run", Desc: "Show what would change without writing"},
					{Name: "--no-reload", Desc: "Do not notify running daemon after save"},
				}},
			},
			Examples: []string{
				"ze config " + verb + " router.conf bgp router-id",
				"ze config " + verb + " router.conf bgp peer peer1",
				"ze config " + verb + " router.conf bgp filter import no-self-as",
			},
		}
		p.Write()
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

	displayPath := strings.Join(path, " ")
	if err := dispatchDeactivate(ed, path, activate); err != nil {
		if alreadyInRequestedState(err) {
			alreadyState := "deactivated"
			if activate {
				alreadyState = "active"
			}
			fmt.Fprintf(os.Stderr, "%s already %s; nothing to do\n", displayPath, alreadyState)
			return exitOK
		}
		fmt.Fprintf(os.Stderr, "error: %s failed: %v\n", verb, err)
		return exitError
	}

	pastTense := "Deactivated"
	if activate {
		pastTense = "Activated"
	}

	if *dryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would %s %s\n", verb, displayPath)
		if diff := ed.Diff(); diff != "" {
			fmt.Fprint(os.Stderr, diff)
		}
		return exitOK
	}

	if err := ed.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "error: save failed: %v\n", err)
		return exitError
	}

	fmt.Fprintf(os.Stderr, "%s %s\n", pastTense, displayPath)

	if !*noReload {
		creds, credErr := sshclient.LoadCredentialsWithFlags(*user)
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

// dispatchDeactivate routes the path to the appropriate Editor mutation
// method based on the schema node at the target. The dispatch mirrors
// the TUI Model.cmdDeactivate logic so behavior is consistent across
// the TUI and CLI surfaces.
func dispatchDeactivate(ed *cli.Editor, path []string, activate bool) error {
	if len(path) == 0 {
		return fmt.Errorf("path is empty")
	}

	// Leaf-list value: `... import no-self-as` (path ends at a value
	// that is not itself a schema child of the leaf-list).
	if len(path) >= 2 {
		parentPath, leafListName, isLeafList := ed.ResolveLeafListValue(path)
		if isLeafList {
			value := path[len(path)-1]
			if activate {
				return ed.ActivateLeafListValue(parentPath, leafListName, value)
			}
			return ed.DeactivateLeafListValue(parentPath, leafListName, value)
		}
	}

	schemaNode := ed.LookupSchemaNode(path)
	if schemaNode == nil {
		return fmt.Errorf("no such path: %s", strings.Join(path, " "))
	}

	switch schemaNode.(type) {
	case *config.LeafNode, *config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
		parentPath := path[:len(path)-1]
		leafName := path[len(path)-1]
		if activate {
			return ed.ActivateLeaf(parentPath, leafName)
		}
		return ed.DeactivateLeaf(parentPath, leafName)
	case *config.ContainerNode, *config.ListNode:
		// Container or list entry: route through DeactivatePath, which
		// rejects non-existent paths and surfaces idempotent "already in
		// state" via sentinel errors. Positional lists with all-leaf
		// children (e.g. nlri, nexthop, add-path) skip inactive
		// injection -- reject those explicitly per AC-12.
		if listNode, ok := schemaNode.(*config.ListNode); ok && !listNode.Has(config.InactiveLeafName) {
			return fmt.Errorf("path %q is a positional list entry; deactivate the parent container instead", strings.Join(path, " "))
		}
		if activate {
			return ed.ActivatePath(path)
		}
		return ed.DeactivatePath(path)
	default:
		return fmt.Errorf("path %q resolves to a node type that does not support deactivation", strings.Join(path, " "))
	}
}
