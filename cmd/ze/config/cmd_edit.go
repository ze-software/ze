// Design: docs/architecture/config/syntax.md — config edit command
// Overview: main.go — dispatch and exit codes

package config

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/handler"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/editor"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// wireCommandExecutor tries to connect to the daemon socket and sets up the command executor.
// If the daemon is not running, command mode will show an error on Enter (best-effort).
// Returns the connection (caller must close) or nil if no daemon.
func wireCommandExecutor(m *editor.Model, socketPath string) net.Conn {
	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", socketPath)
	if err != nil {
		return nil // No daemon — command mode will report "no daemon connection"
	}

	reader := rpc.NewFrameReader(conn)
	writer := rpc.NewFrameWriter(conn)

	// Build command map for resolving CLI text → wire method
	cmdMap, cmdKeys := buildCommandMap()

	m.SetCommandExecutor(func(input string) (string, error) {
		method, args := resolveEditorCommand(cmdMap, cmdKeys, input)
		if method == "" {
			return "", fmt.Errorf("unknown command: %s", input)
		}

		req := rpc.Request{Method: method}
		if len(args) > 0 {
			params := struct {
				Args []string `json:"args,omitempty"`
			}{Args: args}
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return "", fmt.Errorf("marshal params: %w", err)
			}
			req.Params = paramBytes
		}

		reqBytes, err := json.Marshal(req)
		if err != nil {
			return "", fmt.Errorf("marshal request: %w", err)
		}
		if err := writer.Write(reqBytes); err != nil {
			return "", fmt.Errorf("send: %w", err)
		}

		respBytes, err := reader.Read()
		if err != nil {
			return "", fmt.Errorf("receive: %w", err)
		}

		var resp struct {
			Result json.RawMessage `json:"result,omitempty"`
			Error  string          `json:"error,omitempty"`
		}
		if err := json.Unmarshal(respBytes, &resp); err != nil {
			return "", fmt.Errorf("parse response: %w", err)
		}

		if resp.Error != "" {
			return "", fmt.Errorf("%s", resp.Error)
		}

		if len(resp.Result) == 0 || string(resp.Result) == "null" {
			return "OK", nil
		}

		return formatJSONResult(resp.Result), nil
	})

	return conn
}

// buildCommandMap builds the CLI command → wire method mapping from registered RPCs.
func buildCommandMap() (map[string]string, []string) {
	cmdMap := make(map[string]string)
	for _, reg := range allEditorRPCs() {
		cmdMap[strings.ToLower(reg.CLICommand)] = reg.WireMethod
	}

	keys := make([]string, 0, len(cmdMap))
	for k := range cmdMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})

	return cmdMap, keys
}

// allEditorRPCs returns all RPCs for command resolution.
func allEditorRPCs() []pluginserver.RPCRegistration {
	rpcs := pluginserver.AllBuiltinRPCs()
	rpcs = append(rpcs, handler.BgpHandlerRPCs()...)
	return rpcs
}

// formatJSONResult pretty-prints a JSON result, falling back to raw string.
func formatJSONResult(raw json.RawMessage) string {
	var data any
	if json.Unmarshal(raw, &data) != nil {
		return string(raw)
	}
	formatted, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(formatted)
}

// resolveEditorCommand finds the wire method for a text command.
// Tries "bgp <input>" first (default subsystem), then raw input.
func resolveEditorCommand(cmdMap map[string]string, cmdKeys []string, input string) (string, []string) {
	if m, a := matchEditorCommand(cmdMap, cmdKeys, "bgp "+input); m != "" {
		return m, a
	}
	return matchEditorCommand(cmdMap, cmdKeys, input)
}

// matchEditorCommand does longest-prefix matching against registered CLI commands.
func matchEditorCommand(cmdMap map[string]string, cmdKeys []string, input string) (string, []string) {
	lower := strings.ToLower(strings.TrimSpace(input))
	for _, key := range cmdKeys {
		if strings.HasPrefix(lower, key) {
			if len(lower) == len(key) || lower[len(key)] == ' ' {
				remaining := strings.TrimSpace(input[len(key):])
				var args []string
				if remaining != "" {
					args = strings.Fields(remaining)
				}
				return cmdMap[key], args
			}
		}
	}
	return "", nil
}

const createPromptTimeout = 10 * time.Second

// buildEditorCommandTree builds a CommandNode tree from all registered RPCs.
// Strips the "bgp " prefix for BGP commands so the user types "peer list" not "bgp peer list".
func buildEditorCommandTree() *editor.CommandNode {
	rpcs := pluginserver.AllBuiltinRPCs()
	rpcs = append(rpcs, handler.BgpHandlerRPCs()...)

	root := &editor.CommandNode{Children: make(map[string]*editor.CommandNode)}
	for _, reg := range rpcs {
		cmd := strings.TrimPrefix(reg.CLICommand, "bgp ")
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			continue
		}

		current := root
		for _, part := range parts {
			if current.Children == nil {
				current.Children = make(map[string]*editor.CommandNode)
			}
			child, ok := current.Children[part]
			if !ok {
				child = &editor.CommandNode{Name: part}
				current.Children[part] = child
			}
			current = child
		}
		current.Description = reg.Help
	}

	return root
}

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

Mode switching:
  /command              Switch to operational command mode
  /edit                 Switch back to config edit mode

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

	// Wire command mode: completer from RPC registrations, executor from daemon socket.
	// Command mode is best-effort — works without a running daemon (completions only).
	m.SetCommandCompleter(editor.NewCommandCompleter(buildEditorCommandTree()))
	if conn := wireCommandExecutor(&m, config.DefaultSocketPath()); conn != nil {
		defer conn.Close() //nolint:errcheck // best-effort cleanup
	}

	// Run Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}
