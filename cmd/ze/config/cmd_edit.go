// Design: docs/architecture/config/syntax.md — config edit command
// Overview: main.go — dispatch and exit codes

package config

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/editor"
)

const createPromptTimeout = 10 * time.Second

// promptCreateConfig asks the user whether to create a missing config file.
// Returns true if the file was created, false otherwise.
func promptCreateConfig(path string) bool {
	return doPromptCreateConfig(path, os.Stdin, os.Stderr, createPromptTimeout)
}

// doPromptCreateConfig is the testable core of promptCreateConfig.
func doPromptCreateConfig(path string, in io.Reader, errw io.Writer, timeout time.Duration) bool { //nolint:cyclop // linear flow with early returns
	fmt.Fprintf(errw, "config file not found: %s\n", path) //nolint:errcheck // terminal output
	fmt.Fprintf(errw, "create it? [y/N] ")                 //nolint:errcheck // terminal output

	ch := make(chan string, 1)
	go func() {
		reader := bufio.NewReader(in)
		line, _ := reader.ReadString('\n') //nolint:errcheck // EOF returns empty string, handled below
		ch <- strings.ToLower(strings.TrimSpace(line))
	}()

	var answer string
	select {
	case answer = <-ch:
	case <-time.After(timeout):
		fmt.Fprintln(errw)                                 //nolint:errcheck // terminal output
		fmt.Fprintf(errw, "error: no response, exiting\n") //nolint:errcheck // terminal output
		return false
	}

	if answer != "y" && answer != "yes" {
		return false
	}

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			fmt.Fprintf(errw, "error: cannot create directory: %v\n", err) //nolint:errcheck // terminal output
			return false
		}
	}

	if err := os.WriteFile(path, nil, 0o600); err != nil {
		fmt.Fprintf(errw, "error: cannot create file: %v\n", err) //nolint:errcheck // terminal output
		return false
	}

	return true
}

func cmdEdit(args []string) int {
	fs := flag.NewFlagSet("config edit", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config edit [options] <config-file>

Interactive configuration editor with VyOS-like set commands.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Commands:
  set <path> <value>    Set a configuration value
  delete <path>         Delete a configuration value
  edit <path>           Enter a subsection (narrowed context)
  edit <list> *         Edit template for all entries (inheritance)
  top                   Return to root context
  up                    Go up one level
  show [section]        Display current configuration
  compare               Show diff vs original
  commit                Save changes (creates backup)
  discard               Revert all changes
  history               List backup files
  rollback <N>          Restore backup N
  exit/quit             Exit (prompts if unsaved changes)

Tab completion:
  Type partial text + Tab for completion
  Multiple matches show dropdown, Tab cycles through
  Ghost text shows best match in gray

Examples:
  ze config edit /etc/ze/config.conf
  ze config edit ./myconfig.conf
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file\n")
		fs.Usage()
		return 1
	}

	configPath := fs.Arg(0)

	// Offer to create if file doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if !promptCreateConfig(configPath) {
			return 1
		}
	}

	// Create editor
	ed, err := editor.NewEditor(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer ed.Close() //nolint:errcheck // Best effort cleanup

	// Wire reload notification: commit will notify daemon via API socket
	ed.SetReloadNotifier(editor.NewSocketReloadNotifier(config.DefaultSocketPath()))

	// Check for pending edit file from previous session
	if ed.HasPendingEdit() {
		switch ed.PromptPendingEdit() {
		case editor.PendingEditContinue:
			if err := ed.LoadPendingEdit(); err != nil {
				fmt.Fprintf(os.Stderr, "error loading edit file: %v\n", err)
				return 1
			}
		case editor.PendingEditDiscard:
			if err := ed.Discard(); err != nil {
				fmt.Fprintf(os.Stderr, "error discarding edit file: %v\n", err)
				return 1
			}
		case editor.PendingEditQuit:
			return 0
		}
	}

	// Create model
	m, err := editor.NewModel(ed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Run Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}
