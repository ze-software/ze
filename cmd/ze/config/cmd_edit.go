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

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/archive"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// wireCommandExecutor tries to connect to the daemon socket and sets up the command executor.
// If the daemon is not running, command mode will show an error on Enter (best-effort).
// Returns the connection (caller must close) or nil if no daemon.
func wireCommandExecutor(m *cli.Model, socketPath string) net.Conn {
	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", socketPath)
	if err != nil {
		return nil // No daemon — command mode will report "no daemon connection"
	}

	reader := rpc.NewFrameReader(conn)
	writer := rpc.NewFrameWriter(conn)

	// Build command map for resolving CLI text → wire method
	cmdMap, cmdKeys := buildCommandMap()

	// Pipe processing (| table, | json, etc.) is handled by the unified model's
	// executeOperationalCommand — executor receives pre-pipe commands, returns raw JSON.
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
			Params json.RawMessage `json:"params,omitempty"`
		}
		if err := json.Unmarshal(respBytes, &resp); err != nil {
			return "", fmt.Errorf("parse response: %w", err)
		}

		if resp.Error != "" {
			if msg := rpc.ExtractMessage(resp.Params); msg != "" {
				return "", fmt.Errorf("%s", msg)
			}
			return "", fmt.Errorf("%s", resp.Error)
		}

		if len(resp.Result) == 0 || string(resp.Result) == "null" {
			return "OK", nil
		}

		return string(resp.Result), nil
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
	return pluginserver.AllBuiltinRPCs()
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

// buildEditorCommandTree builds a command.Node tree from all registered RPCs.
// Strips the "bgp " prefix for BGP commands so the user types "peer list" not "bgp peer list".
func buildEditorCommandTree() *command.Node {
	rpcs := pluginserver.AllBuiltinRPCs()
	infos := make([]command.RPCInfo, len(rpcs))
	for i, reg := range rpcs {
		infos[i] = command.RPCInfo{
			CLICommand: reg.CLICommand,
			Help:       reg.Help,
			ReadOnly:   reg.ReadOnly,
		}
	}
	return command.BuildTree(infos, false)
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

	// Use O_CREATE|O_EXCL for atomic create — prevents TOCTOU symlink attacks
	// between the Stat check and file creation.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // config path from user
	if err != nil {
		fmt.Fprintf(errw, "error: cannot create file: %v\n", err) //nolint:errcheck // terminal output
		return false
	}
	f.Close() //nolint:errcheck,gosec // empty file, close error is non-fatal

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
  command               Switch to operational command mode
  edit                  Switch back to config edit mode (in command mode)

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
	ed, err := cli.NewEditor(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer ed.Close() //nolint:errcheck // Best effort cleanup

	// Probe daemon socket at startup: only wire reload if daemon is reachable
	socketPath := config.DefaultSocketPath()
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer probeCancel()
	if conn, err := (&net.Dialer{}).DialContext(probeCtx, "unix", socketPath); err == nil {
		conn.Close() //nolint:errcheck,gosec // Probe connection, close error is irrelevant
		ed.SetReloadNotifier(cli.NewSocketReloadNotifier(socketPath))
	}

	// Wire archive notifier if config has commit-triggered archive blocks
	if ed.Tree() != nil {
		sys := system.ExtractSystemConfig(ed.Tree())
		allConfigs := archive.ExtractConfigs(ed.Tree())
		commitConfigs := archive.FilterByTrigger(allConfigs, archive.TriggerCommit)
		if len(commitConfigs) > 0 {
			ed.SetArchiveNotifier(archive.NewNotifier(configPath, commitConfigs, sys))
		}
	}

	// Create session for concurrent editing.
	username := os.Getenv("USER")
	if username == "" {
		username = "unknown"
	}
	session := cli.NewEditSession(username, "local")
	ed.SetSession(session)

	// Auto-load draft if it exists (replaces PromptPendingEdit).
	draftPath := cli.DraftPath(configPath)
	if _, statErr := os.Stat(draftPath); statErr == nil {
		// Draft exists: display active sessions.
		activeSessions := ed.ActiveSessions()
		if len(activeSessions) > 0 {
			fmt.Fprintf(os.Stderr, "Active sessions:\n") //nolint:errcheck // terminal output
			for _, sid := range activeSessions {
				fmt.Fprintf(os.Stderr, "  %s\n", sid) //nolint:errcheck // terminal output
			}
		}
	} else if ed.HasPendingEdit() {
		// Legacy pending edit file (pre-session format).
		switch ed.PromptPendingEdit() {
		case cli.PendingEditContinue:
			if err := ed.LoadPendingEdit(); err != nil {
				fmt.Fprintf(os.Stderr, "error loading edit file: %v\n", err)
				return 1
			}
		case cli.PendingEditDiscard:
			if err := ed.Discard(); err != nil {
				fmt.Fprintf(os.Stderr, "error discarding edit file: %v\n", err)
				return 1
			}
		case cli.PendingEditQuit:
			return 0
		}
	}

	// Create model
	m, err := cli.NewModel(ed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Wire command mode: completer from RPC registrations, executor from daemon socket.
	// Command mode is best-effort — works without a running daemon (completions only).
	m.SetCommandCompleter(cli.NewCommandCompleter(buildEditorCommandTree()))
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
