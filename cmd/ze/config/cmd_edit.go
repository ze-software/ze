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
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/archive"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	rpc "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// defaultConfigName is the config name used when no argument is given.
const defaultConfigName = "ze.conf"

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

// selectConfig prompts the user to select a config from available configs in blob storage.
// Returns the selected config path, or empty string if canceled/error.
func selectConfig(store storage.Storage, configDir, defaultPath string) string {
	return doSelectConfig(store, configDir, defaultPath, os.Stdin, os.Stderr, createPromptTimeout)
}

// doSelectConfig is the testable core of selectConfig.
// Lists .conf files in configDir via storage; if none exist (AC-7), creates defaultPath.
// If multiple exist (AC-6), presents numbered list and accepts selection.
func doSelectConfig(store storage.Storage, configDir, defaultPath string, in io.Reader, errw io.Writer, timeout time.Duration) string { //nolint:cyclop // linear flow with early returns
	files, err := store.List(configDir)
	if err != nil {
		// Empty blob has no directory entries - treat as "no configs"
		files = nil
	}

	// Filter to .conf files (excludes .draft, .lock, ssh_host_*, etc.)
	var configs []string
	for _, f := range files {
		if strings.HasSuffix(f, ".conf") {
			configs = append(configs, f)
		}
	}
	sort.Strings(configs)

	// AC-7: no configs exist, create ze.conf
	if len(configs) == 0 {
		fmt.Fprintf(errw, "no configs found, creating %s\n", filepath.Base(defaultPath)) //nolint:errcheck // terminal output
		if writeErr := store.WriteFile(defaultPath, []byte{}, 0o600); writeErr != nil {
			fmt.Fprintf(errw, "error: cannot create %s: %v\n", filepath.Base(defaultPath), writeErr) //nolint:errcheck // terminal output
			return ""
		}
		return defaultPath
	}

	// AC-6: list available configs and prompt for selection
	fmt.Fprintf(errw, "ze.conf not found in store. Available configs:\n") //nolint:errcheck // terminal output
	for i, c := range configs {
		fmt.Fprintf(errw, "  %d) %s\n", i+1, filepath.Base(c)) //nolint:errcheck // terminal output
	}
	fmt.Fprintf(errw, "select [1-%d]: ", len(configs)) //nolint:errcheck // terminal output

	ch := make(chan string, 1)
	go func() {
		reader := bufio.NewReader(in)
		line, _ := reader.ReadString('\n') //nolint:errcheck // EOF returns empty string, handled below
		ch <- strings.TrimSpace(line)
	}()

	var answer string
	select {
	case answer = <-ch:
	case <-time.After(timeout):
		fmt.Fprintln(errw)                                 //nolint:errcheck // terminal output
		fmt.Fprintf(errw, "error: no response, exiting\n") //nolint:errcheck // terminal output
		return ""
	}

	if answer == "" {
		return ""
	}

	n, parseErr := strconv.Atoi(answer)
	if parseErr != nil || n < 1 || n > len(configs) {
		fmt.Fprintf(errw, "error: invalid selection\n") //nolint:errcheck // terminal output
		return ""
	}

	return configs[n-1]
}

// cmdEditWithStorage handles the edit command with a given storage backend.
// Supports -f flag to override to filesystem and defaults to ze.conf.
func cmdEditWithStorage(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config edit", flag.ExitOnError)
	fileOverride := fs.Bool("f", false, "Use filesystem directly, bypass blob store")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config edit [options] [config-file]

Interactive configuration editor with VyOS-like set commands.
Config file defaults to ze.conf when not specified.

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
  ze config edit                         Edit default ze.conf
  ze config edit router.conf             Edit specific config
  ze config edit -f /etc/ze/config.conf  Edit from filesystem
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Override to filesystem if -f flag is set
	if *fileOverride {
		store.Close() //nolint:errcheck // closing blob before switching to filesystem
		store = storage.NewFilesystem()
	}

	// Default to ze.conf when no config name given
	configPath := defaultConfigName
	userProvided := fs.NArg() >= 1
	if userProvided {
		configPath = fs.Arg(0)
	}

	configPath = config.ResolveConfigPath(configPath)

	// For filesystem storage, offer to create if file doesn't exist
	if !storage.IsBlobStorage(store) {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if !promptCreateConfig(configPath) {
				return 1
			}
		}
	}

	// For blob storage with default config name, handle missing ze.conf (AC-6/AC-7)
	if storage.IsBlobStorage(store) && !userProvided && !store.Exists(configPath) {
		selected := selectConfig(store, filepath.Dir(configPath), configPath)
		if selected == "" {
			return 1
		}
		configPath = selected
	}

	// Create editor with storage backend
	ed, err := cli.NewEditorWithStorage(store, configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return runEditor(ed, configPath)
}

// runEditor runs the interactive editor TUI after the Editor is created.
func runEditor(ed *cli.Editor, configPath string) int {
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
