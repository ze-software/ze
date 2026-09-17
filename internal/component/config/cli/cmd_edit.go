// Design: docs/architecture/config/syntax.md — config edit command
// Overview: main.go — dispatch and exit codes

package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/archive"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/resolve"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
)

// ephemeralPollInterval is the interval between SSH port readiness checks.
const ephemeralPollInterval = 100 * time.Millisecond

// ephemeralPollTimeout is the maximum time to wait for the ephemeral daemon to start.
const ephemeralPollTimeout = 10 * time.Second

// startEphemeralDaemon starts a background ze daemon for the given config.
// Waits for the SSH port to become reachable before returning.
// Returns the process (caller must stop and wait) or an error.
func startEphemeralDaemon(configPath, host, port string, extraArgs []string) (*os.Process, string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, "", fmt.Errorf("find ze binary: %w", err)
	}

	devnull, err := os.Open(os.DevNull)
	if err != nil {
		return nil, "", fmt.Errorf("open devnull: %w", err)
	}

	// Ephemeral SSH address file: the daemon starts SSH on port 0 (OS-assigned)
	// and writes the actual address here so we can connect.
	sshAddrFile := filepath.Join("tmp", fmt.Sprintf("ephemeral-ssh-%d.addr", os.Getpid()))

	daemonEnv := append(os.Environ(), "ZE_SSH_EPHEMERAL="+sshAddrFile, "ZE_CONFIG_DIR="+resolve.StoreDir(configPath))

	argv := make([]string, 0, 1+len(extraArgs)+1)
	argv = append(argv, exe, "start")
	argv = append(argv, extraArgs...)
	if info, err := os.Stat(configPath); err == nil && info.Mode().IsRegular() {
		argv = append(argv, configPath)
	}

	proc, err := os.StartProcess(exe, argv, &os.ProcAttr{
		Env:   daemonEnv,
		Files: []*os.File{devnull, devnull, os.Stderr},
	})
	devnull.Close() //nolint:errcheck // devnull close is non-fatal
	if err != nil {
		return nil, "", fmt.Errorf("start ephemeral daemon: %w", err)
	}

	// Poll for SSH: try configured port and ephemeral addr file.
	configuredAddr := net.JoinHostPort(host, port)
	deadline := time.Now().Add(ephemeralPollTimeout)
	for time.Now().Before(deadline) {
		if probeSSHWithTimeout(host, port, 200*time.Millisecond) {
			return proc, configuredAddr, nil
		}
		if data, readErr := os.ReadFile(sshAddrFile); readErr == nil && len(data) > 0 { //nolint:gosec // path is constructed by us, not user input
			addr := string(data)
			if probeAddr(addr, 200*time.Millisecond) {
				return proc, addr, nil
			}
		}
		time.Sleep(ephemeralPollInterval)
	}

	if killErr := proc.Kill(); killErr != nil {
		fmt.Fprintf(os.Stderr, "warning: kill ephemeral daemon: %v\n", killErr)
	}
	if _, waitErr := proc.Wait(); waitErr != nil {
		fmt.Fprintf(os.Stderr, "warning: wait ephemeral daemon: %v\n", waitErr)
	}
	os.Remove(sshAddrFile) //nolint:errcheck // best-effort cleanup of temp file
	return nil, "", fmt.Errorf("ephemeral daemon failed to start within %v", ephemeralPollTimeout)
}

// probeAddr dials a host:port address to check reachability.
func probeAddr(addr string, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	conn, dialErr := d.Dial("tcp", addr)
	if dialErr != nil {
		return false
	}
	conn.Close() //nolint:errcheck // probe connection
	return true
}

// stopEphemeralDaemon sends a stop command via SSH and waits for the process to exit.
// If the process doesn't exit within 5 seconds, it is killed.
func stopEphemeralDaemon(proc *os.Process, creds sshclient.Credentials) {
	// Best-effort stop via SSH
	if _, err := sshclient.ExecCommand(creds, "stop"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: stop ephemeral daemon: %v\n", err)
		// SSH failed (web-only daemon or unreachable) — send SIGINT so the
		// process shuts down via its signal handler instead of waiting for kill.
		_ = proc.Signal(os.Interrupt)
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

// execEditorCommand indirects over the SSH client so a test can read the command
// string the editor's executor put on the channel, `| raw` included, without
// opening a connection. The same pattern is in reload_notify.go, for the same
// reason.
var execEditorCommand = sshclient.ExecCommand

// sshCommandExecutor is the operational-command executor the editor's Model runs.
//
// The Model splits the pipe chain and renders the answer itself
// (internal/component/cli/model_mode.go, executeOperationalCommand), so this
// executor asks for the dispatcher's JSON. An answer the daemon already
// rendered in the configured format cannot be rendered a second time.
func sshCommandExecutor(creds sshclient.Credentials) cli.CommandExecutor {
	return func(input string) (cli.CommandOutput, error) {
		output, err := execEditorCommand(creds, sshclient.RawCommand(input))
		return cli.CommandOutput{Text: output}, err
	}
}

// wireSSHCommandExecutor sets up a command executor that dispatches via SSH exec,
// optionally wrapping with transcript recording if enabled. Returns the
// TranscriptWriter (nil if disabled) so the caller can defer Close.
func wireSSHCommandExecutor(m *cli.Model, creds sshclient.Credentials, username, remoteHost string) *cli.TranscriptWriter {
	executor := sshCommandExecutor(creds)

	var tw *cli.TranscriptWriter
	if tf := openTranscriptFile(); tf != nil {
		tw = cli.NewTranscriptWriter(tf, username, remoteHost)
		executor = cli.WrapExecutorWithTranscript(executor, tw)
	}

	m.SetCommandExecutor(executor)
	return tw
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
	// between the Stat check and file creation. `-` is rejected upstream in
	// cmdEditWithStorage, so path is always a real file (never stdin), and cliio
	// cannot express O_EXCL; keep the raw create.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // cliio:allow config path is a real file; edit rejects "-" upstream, and O_EXCL atomic-create has no cliio form
	if err != nil {
		fmt.Fprintf(errw, "error: cannot create file: %v\n", err) //nolint:errcheck // terminal output
		return false
	}
	f.Close() //nolint:errcheck,gosec // empty file, close error is non-fatal

	return true
}

// selectConfig prompts the user to select a config from the live store.
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
		fmt.Fprintf(errw, "cannot list configurations: %v\n", err) //nolint:errcheck // terminal output
		return ""
	}

	// Filter to .conf files (excludes .draft, .lock, ssh_host_*, etc.)
	var configs []string
	for _, f := range files {
		if strings.HasSuffix(f, ".conf") {
			configs = append(configs, f)
		}
	}
	slices.Sort(configs)

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
// The -f flag edits a loose file. Otherwise the editor connects to its owning daemon.
func cmdEditWithStorage(store storage.Storage, args []string) int {
	fs := flag.NewFlagSet("config edit", flag.ExitOnError)
	fileOverride := fs.Bool("f", false, "Edit a loose file without the live store")
	user := fs.String("user", "", "SSH login username (overrides zefs super-admin)")
	fs.StringVar(user, "u", "", "Short alias for --user")
	webPort := fs.String("web", "", "Start web UI on given port (passed to ephemeral daemon)")
	insecureWeb := fs.Bool("insecure-web", false, "Disable web auth (implies localhost bind)")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze config edit",
			ShortHelp: "Interactive configuration editor with VyOS-like set commands",
			Usage:     []string{"ze config edit [options] [config-file]"},
			Sections: []helpfmt.HelpSection{
				{Title: helpSectionOptions, Entries: []helpfmt.HelpEntry{
					{Name: "-f", Desc: "Edit a loose file without the live store"},
					{Name: "--web <port>", Desc: "Start web UI on given port (ephemeral daemon)"},
					{Name: "--insecure-web", Desc: "Disable web auth (binds localhost)"},
				}},
				{Title: "Commands", Entries: []helpfmt.HelpEntry{
					{Name: "set <path> <value>", Desc: "Set a configuration value"},
					{Name: "delete <path>", Desc: "Delete a configuration value"},
					{Name: "edit <path>", Desc: "Enter a subsection (narrowed context)"},
					{Name: "edit <list> *", Desc: "Edit template for all entries (inheritance)"},
					{Name: "top", Desc: "Return to root context"},
					{Name: "up", Desc: "Go up one level"},
					{Name: "show [section]", Desc: "Display current configuration"},
					{Name: "show | <filter>", Desc: "Pipe: blame, changes, compare, errors, history"},
					{Name: "commit", Desc: "Save changes (creates backup)"},
					{Name: "discard", Desc: "Revert all changes"},
					{Name: "rollback <N>", Desc: "Restore backup N"},
					{Name: "run <command>", Desc: "Execute operational command"},
					{Name: "exit/quit", Desc: "Exit (prompts if unsaved changes)"},
				}},
				{Title: "Mode switching", Entries: []helpfmt.HelpEntry{
					{Name: "command", Desc: "Switch to operational command mode"},
					{Name: "edit", Desc: "Switch back to config edit mode (in command mode)"},
				}},
				{Title: "Tab completion", Entries: []helpfmt.HelpEntry{
					{Name: "Tab", Desc: "Type partial text + Tab for completion"},
					{Name: "Multiple matches", Desc: "Show dropdown, Tab cycles through"},
					{Name: "Ghost text", Desc: "Shows best match in gray"},
				}},
			},
			Examples: []string{
				"ze config edit                         Edit default config (<identity>.conf)",
				"ze config edit router.conf             Edit specific config",
				"ze config edit -f /etc/ze/config.conf  Edit from filesystem",
			},
		}
		p.WriteErr()
		fmt.Fprintf(os.Stderr, "\nConfig file defaults to <name>.conf (from meta/instance/name) or ze.conf.\n")
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "error: config edit takes at most one config file")
		return exitError
	}
	if *fileOverride && (*webPort != "" || *insecureWeb) {
		fmt.Fprintln(os.Stderr, "error: -f is an offline file editor; use config edit --web without -f for a daemon web session")
		return exitError
	}

	// The interactive editor needs a real file identity (draft/backup/lock) and a
	// TTY; a config on stdin ("-") has neither, and stdin is consumed by the pipe.
	// Reject early, independent of the storage backend.
	if fs.NArg() >= 1 && cliio.IsStdin(fs.Arg(0)) {
		fmt.Fprintf(os.Stderr, "error: interactive edit cannot read a config from stdin (\"-\"); use `ze config set - ...` for a pipeline, or edit a file\n")
		return 1
	}

	configPath := ""
	userProvided := fs.NArg() >= 1
	if userProvided {
		configPath = config.ResolveConfigPath(fs.Arg(0))
	}
	if *fileOverride {
		if configPath == "" {
			configPath = resolve.DefaultConfig(nil)
		}
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if !promptCreateConfig(configPath) {
				return exitError
			}
		}
		ed, err := openEditableConfig(nil, configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		return runEditor(ed, configPath, *user)
	}
	openedStore := false
	if store == nil {
		var err error
		store, err = storage.Open(resolve.StoreDir(configPath))
		if errors.Is(err, storage.ErrBusy) {
			store, err = storage.OpenReadOnly(resolve.StoreDir(configPath))
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: config edit: %v\n", err)
			return exitError
		}
		openedStore = true
		defer func() {
			if store != nil {
				store.Close()
			} //nolint:errcheck // Earlier command error takes precedence.
		}()
	}
	if configPath == "" {
		configPath = resolve.DefaultConfig(store)
		if !store.Exists(configPath) {
			configPath = selectConfig(store, "file/active", configPath)
			if configPath == "" {
				return exitError
			}
		}
		configPath = filepath.Join(resolve.StoreDir(""), configPath)
	}
	// Selection may create the first config in an otherwise empty offline store.
	// Release that owner before an ephemeral daemon opens the same store.
	if openedStore {
		err := store.Close()
		store = nil
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: release editor discovery store: %v\n", err)
			return exitError
		}
	}

	var daemonArgs []string
	if *webPort != "" {
		daemonArgs = append(daemonArgs, "--web", *webPort)
	}
	if *insecureWeb {
		daemonArgs = append(daemonArgs, "--insecure-web")
	}

	return runStoredEditor(configPath, *user, daemonArgs)
}

// runStoredEditor leaves drafts, commits, and history inside the owning daemon.
func runStoredEditor(configPath, user string, daemonArgs []string) int {
	creds, credsErr := sshclient.ReadCredentialsWithFlags(sshclient.ResolveStoreDir(configPath), user)
	if credsErr != nil {
		fmt.Fprintf(os.Stderr, "error: editor connection: %v\n", credsErr)
		return exitError
	}
	if !probeDaemonSSH(creds.Host, creds.Port) {
		proc, address, err := startEphemeralDaemon(configPath, creds.Host, creds.Port, daemonArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: ephemeral daemon: %v\n", err)
			return exitError
		}
		if host, port, err := net.SplitHostPort(address); err == nil {
			creds.Host, creds.Port = host, port
		}
		defer stopEphemeralDaemon(proc, creds)
	}
	if err := sshclient.RunInteractive(creds, filepath.Base(configPath), false); err != nil {
		fmt.Fprintf(os.Stderr, "error: editor session: %v\n", err)
		return exitError
	}
	return exitOK
}

// runEditor keeps explicit loose-file editing local.
func runEditor(ed *cli.Editor, configPath, user string) int {
	defer ed.Close() //nolint:errcheck // Best effort cleanup.
	creds, credsErr := sshclient.ReadCredentialsWithFlags(sshclient.ResolveStoreDir(configPath), user)
	if ed.HasPendingEdit() {
		switch ed.PromptPendingEdit() {
		case cli.PendingEditContinue:
			if err := ed.LoadPendingEdit(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return exitError
			}
		case cli.PendingEditDiscard:
			if err := ed.Discard(); err != nil {
				return exitError
			}
		case cli.PendingEditQuit:
			return exitOK
		}
	}
	if ed.Tree() != nil {
		sys := system.ExtractSystemConfig(ed.Tree())
		configs := archive.FilterByTrigger(archive.ExtractConfigs(ed.Tree()), archive.TriggerCommit)
		if len(configs) > 0 {
			ed.SetArchiveNotifier(archive.NewNotifier(configPath, configs, &sys, nil))
		}
	}
	m, err := cli.NewModel(ed, cli.FilesystemAuthorityOperatorLocal)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	m.SetCommandCompleter(cli.NewCommandCompleter(buildEditorCommandTree()))
	if credsErr == nil {
		if tw := wireSSHCommandExecutor(&m, creds, os.Getenv("USER"), net.JoinHostPort(creds.Host, creds.Port)); tw != nil {
			defer tw.Close() //nolint:errcheck // Best effort transcript.
		}
	}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		if errors.Is(err, tea.ErrProgramPanic) {
			crashlog.HandleCaughtPanic(err)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	return exitOK
}
