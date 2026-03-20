// Design: docs/architecture/config/syntax.md — config edit command
// Overview: main.go — dispatch and exit codes

package config

import (
	"bufio"
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

	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/archive"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/config/system"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

// fallbackConfigName is used when meta/instance/name is not set.
const fallbackConfigName = "ze.conf"

// identityNameKey is the blob key for the instance name.
const identityNameKey = "meta/instance/name"

// defaultConfigName returns the default config filename from the blob's
// meta/instance/name (e.g. "ze-first" -> "ze-first.conf"), falling back
// to "ze.conf" when the key is absent or the storage is not blob-backed.
func defaultConfigName(store storage.Storage) string {
	if !storage.IsBlobStorage(store) {
		return fallbackConfigName
	}
	data, err := store.ReadFile(identityNameKey)
	if err != nil || len(data) == 0 {
		return fallbackConfigName
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return fallbackConfigName
	}
	return name + ".conf"
}

// ephemeralPollInterval is the interval between SSH port readiness checks.
const ephemeralPollInterval = 100 * time.Millisecond

// ephemeralPollTimeout is the maximum time to wait for the ephemeral daemon to start.
const ephemeralPollTimeout = 10 * time.Second

// startEphemeralDaemon starts a background ze daemon for the given config.
// Waits for the SSH port to become reachable before returning.
// Returns the process (caller must stop and wait) or an error.
func startEphemeralDaemon(configPath, host, port string) (*os.Process, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("find ze binary: %w", err)
	}

	// Open devnull for the daemon's stdin so it doesn't compete with the editor's terminal.
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		return nil, fmt.Errorf("open devnull: %w", err)
	}

	// Start ze with the config file as a background daemon.
	proc, err := os.StartProcess(exe, []string{exe, configPath}, &os.ProcAttr{
		Env:   os.Environ(),
		Files: []*os.File{devnull, os.Stderr, os.Stderr},
	})
	devnull.Close() //nolint:errcheck // devnull close is non-fatal
	if err != nil {
		return nil, fmt.Errorf("start ephemeral daemon: %w", err)
	}

	// Wait for SSH port to become reachable (short timeout per probe for fast polling)
	deadline := time.Now().Add(ephemeralPollTimeout)
	for time.Now().Before(deadline) {
		if probeSSHWithTimeout(host, port, 200*time.Millisecond) {
			return proc, nil
		}
		time.Sleep(ephemeralPollInterval)
	}

	// Timeout — kill the process and report failure
	if killErr := proc.Kill(); killErr != nil {
		fmt.Fprintf(os.Stderr, "warning: kill ephemeral daemon: %v\n", killErr)
	}
	if _, waitErr := proc.Wait(); waitErr != nil {
		fmt.Fprintf(os.Stderr, "warning: wait ephemeral daemon: %v\n", waitErr)
	}
	return nil, fmt.Errorf("ephemeral daemon failed to start within %v", ephemeralPollTimeout)
}

// stopEphemeralDaemon sends a stop command via SSH and waits for the process to exit.
// If the process doesn't exit within 5 seconds, it is killed.
func stopEphemeralDaemon(proc *os.Process, creds sshclient.Credentials) {
	// Best-effort stop via SSH
	if _, err := sshclient.ExecCommand(creds, "stop"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: stop ephemeral daemon: %v\n", err)
	}

	// Wait for process to exit with timeout.
	// Single goroutine owns proc.Wait to avoid race between Wait and Kill.
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Wait blocks until the process exits (from SSH stop or kill below).
		if _, err := proc.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: wait ephemeral daemon: %v\n", err)
		}
	}()

	select {
	case <-done:
		return
	case <-time.After(5 * time.Second):
		// Process didn't exit after SSH stop — force kill, then wait for goroutine.
		if err := proc.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: kill ephemeral daemon: %v\n", err)
		}
		<-done // wait for goroutine to finish after kill
	}
}

// probeDaemonSSH checks if a daemon is reachable at host:port via TCP dial.
// Uses the provided timeout (0 means default 2s).
func probeDaemonSSH(host, port string) bool {
	return probeSSHWithTimeout(host, port, 2*time.Second)
}

func probeSSHWithTimeout(host, port string, timeout time.Duration) bool {
	addr := net.JoinHostPort(host, port)
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return false
	}
	conn.Close() //nolint:errcheck // probe connection
	return true
}

// wireSSHCommandExecutor sets up a command executor that dispatches via SSH exec.
// If credentials are unavailable, command mode will show an error on Enter (best-effort).
func wireSSHCommandExecutor(m *cli.Model, creds sshclient.Credentials) {
	m.SetCommandExecutor(func(input string) (string, error) {
		return sshclient.ExecCommand(creds, input)
	})
}

const createPromptTimeout = 10 * time.Second

// buildEditorCommandTree builds a command.Node tree from YANG command modules.
func buildEditorCommandTree() *command.Node {
	loader, _ := yang.DefaultLoader()
	return yang.BuildCommandTree(loader)
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

	// AC-7: no configs exist, create default config
	if len(configs) == 0 {
		fmt.Fprintf(errw, "no configs found, creating %s\n", filepath.Base(defaultPath)) //nolint:errcheck // terminal output
		if writeErr := store.WriteFile(defaultPath, []byte{}, 0o600); writeErr != nil {
			fmt.Fprintf(errw, "error: cannot create %s: %v\n", filepath.Base(defaultPath), writeErr) //nolint:errcheck // terminal output
			return ""
		}
		return defaultPath
	}

	// AC-6: list available configs and prompt for selection
	fmt.Fprintf(errw, "%s not found in store. Available configs:\n", filepath.Base(defaultPath)) //nolint:errcheck // terminal output
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
// Supports -f flag to override to filesystem. Defaults to <identity>.conf from blob.
func cmdEditWithStorage(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config edit", flag.ExitOnError)
	fileOverride := fs.Bool("f", false, "Use filesystem directly, bypass blob store")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze config edit [options] [config-file]

Interactive configuration editor with VyOS-like set commands.
Config file defaults to <name>.conf (from meta/instance/name) or ze.conf.

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
  ze config edit                         Edit default config (<identity>.conf)
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

	// Default config name from meta/instance/name, fall back to ze.conf
	configPath := defaultConfigName(store)
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

	// For blob storage with default config name, handle missing config (AC-6/AC-7)
	if storage.IsBlobStorage(store) && !userProvided && !store.Exists(configPath) {
		selected := selectConfig(store, "file/active", configPath)
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

	return runEditor(ed, store, configPath)
}

// runEditor runs the interactive editor TUI after the Editor is created.
func runEditor(ed *cli.Editor, store storage.Storage, configPath string) int {
	defer ed.Close() //nolint:errcheck // Best effort cleanup

	// Probe daemon SSH port at startup.
	// If no daemon is running and credentials are available, start an ephemeral daemon.
	// Skip ephemeral daemon when config has no recognized block (bgp, plugin) — the
	// daemon would reject it with "no recognized block" and exit immediately.
	creds, credsErr := sshclient.LoadCredentials()
	var ephemeralProc *os.Process
	daemonReachable := false
	if credsErr == nil {
		if probeDaemonSSH(creds.Host, creds.Port) {
			daemonReachable = true
		} else if config.ProbeConfigType(ed.OriginalContent()) != config.ConfigTypeUnknown {
			// Only start ephemeral daemon when config has a recognized block
			proc, ephErr := startEphemeralDaemon(configPath, creds.Host, creds.Port)
			if ephErr != nil {
				fmt.Fprintf(os.Stderr, "warning: ephemeral daemon: %v\n", ephErr)
			} else {
				ephemeralProc = proc
				daemonReachable = true
			}
		}
		if daemonReachable {
			ed.SetReloadNotifier(func() error {
				_, err := sshclient.ExecCommand(creds, "reload")
				return err
			})
		}
	}
	// Stop ephemeral daemon when editor exits
	if ephemeralProc != nil {
		defer stopEphemeralDaemon(ephemeralProc, creds)
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
	if err := cli.ValidateUser(username); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	session := cli.NewEditSession(username, "local")
	ed.SetSession(session)

	// Auto-load draft if it exists.
	draftPath := cli.DraftPath(configPath)
	if store.Exists(draftPath) {
		// Load draft first so ActiveSessions can see draft sessions.
		if !ed.LoadDraft() {
			fmt.Fprintf(os.Stderr, "warning: draft file exists but could not be loaded\n") //nolint:errcheck // terminal output
		}

		// Draft exists: check for same-user orphaned sessions.
		activeSessions := ed.ActiveSessions()
		orphaned := session.OrphanedSessions(activeSessions)
		stdinScanner := bufio.NewScanner(os.Stdin)
		for _, sid := range orphaned {
			// Same user, different session -- offer adoption.
			entries := ed.SessionChanges(sid)
			fmt.Fprintf(os.Stderr, "Found pending changes from previous session (%s, %d changes):\n", sid, len(entries)) //nolint:errcheck // terminal output
			for _, e := range entries {
				fmt.Fprintf(os.Stderr, "  %s\n", e.Path) //nolint:errcheck // terminal output
			}
			fmt.Fprintf(os.Stderr, "Adopt these changes? (yes/no) ") //nolint:errcheck // terminal output

			if !stdinScanner.Scan() {
				break // stdin closed or error
			}
			if strings.TrimSpace(stdinScanner.Text()) == "yes" {
				if adoptErr := ed.AdoptSession(sid); adoptErr != nil {
					fmt.Fprintf(os.Stderr, "error adopting session: %v\n", adoptErr) //nolint:errcheck // terminal output
				}
			}
		}

		// Display remaining active sessions (other users).
		remaining := ed.ActiveSessions()
		otherSessions := make([]string, 0)
		for _, sid := range remaining {
			if sid != session.ID {
				otherSessions = append(otherSessions, sid)
			}
		}
		if len(otherSessions) > 0 {
			fmt.Fprintf(os.Stderr, "Active editing sessions:\n") //nolint:errcheck // terminal output
			for _, sid := range otherSessions {
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

	// Wire persistent command history from blob storage (graceful no-op for filesystem).
	if storage.IsBlobStorage(store) {
		m.SetHistory(cli.NewHistory(store, username))
	}

	// Wire command mode: completer from RPC registrations, executor via SSH.
	// Command mode is best-effort - works without a running daemon (completions only).
	m.SetCommandCompleter(cli.NewCommandCompleter(buildEditorCommandTree()))
	if credsErr == nil && daemonReachable {
		wireSSHCommandExecutor(&m, creds)
	}

	// Run Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}
