// Design: docs/architecture/system-architecture.md — SSH client helper for CLI tools
// Related: ../../../../pkg/zefs/store.go — BlobStore reads credentials (meta/ssh/*)

// Package client provides SSH client connectivity for ze CLI tools.
// CLI tools connect to the daemon via SSH instead of Unix sockets.
package client

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// envTypeString is the EnvEntry type name for a string-valued variable.
const envTypeString = "string"

var errCannotDetermineDatabaseLocation = errors.New("cannot determine database location")

var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.host", Type: envTypeString, Default: "127.0.0.1", Description: "Override SSH host"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.port", Type: envTypeString, Default: "2222", Description: "Override SSH port"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.username", Type: envTypeString, Description: "Override SSH username (default: zefs super-admin)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.password", Type: envTypeString, Description: "SSH password (zefs stores bcrypt hash)", Secret: true})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.insecure", Type: "bool", Default: "false", Description: "Skip host key verification for remote SSH (INSECURE)"})
)

const (
	dialTimeout = 10 * time.Second
	defaultHost = "127.0.0.1"
	defaultPort = "2222"
)

// ServerSoftwareVersion is ze's SSH softwareversion token (RFC 4253 §4.2).
// The ze SSH server announces the identification string "SSH-2.0-" + this
// token (see internal/component/ssh, which passes it to wish.WithVersion).
// It is the shared source of truth so the server's banner and any client-side
// recognition (the `ze init` daemon-liveness probe) cannot drift apart.
const ServerSoftwareVersion = "ze"

// ServerVersionBanner is the full RFC 4253 §4.2 identification-string prefix
// that ze's SSH server announces. The daemon-liveness probe requires this
// prefix to positively distinguish a live ze daemon from a generic SSH server
// (e.g. host OpenSSH, "SSH-2.0-OpenSSH_*") or a bare TCP listener.
const ServerVersionBanner = "SSH-2.0-" + ServerSoftwareVersion

// Credentials holds SSH connection parameters.
type Credentials struct {
	Host     string
	Port     string
	Username string
	Auth     string // SSH auth credential (read from zefs, never serialized)
}

// ExecCommand connects to the daemon via SSH and runs a command.
// Returns the command output or an error.
//
// The two streams are read apart rather than merged. stdout is the answer, and
// stderr carries the answer frame the daemon writes for every session, plus any
// plain text it wrote for a person (answer.go, internal/component/ssh). A
// merged read would put frame lines inside the payload a caller unmarshals.
//
// A failure is reported from stderr, and readAnswerFrame is what reads it, so
// this call and ExecCommandStream report one message from one parser.
func ExecCommand(creds Credentials, command string) (string, error) {
	client, err := dialDaemon(creds)
	if err != nil {
		return "", err
	}
	defer client.Close() //nolint:errcheck // best-effort cleanup

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	defer session.Close() //nolint:errcheck // best-effort cleanup

	var frame textbuf.Buffer
	session.Stderr = &frame
	output, err := session.Output(command)
	if err != nil {
		answer, text := readAnswerFrame(strings.NewReader(frame.String()))
		if answer.Message != "" {
			return "", errors.New(answer.Message)
		}
		if text != "" {
			return "", errors.New(trimErrorPrefix(text))
		}
		if len(output) > 0 {
			return "", errors.New(trimErrorPrefix(strings.TrimSpace(string(output))))
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// rawPipe is the pipe operator that asks the daemon to answer with the
// dispatcher's JSON rather than a rendering of it.
//
// The exec channel is two things at once: an operator surface, where
// `environment cli format default` decides the rendering, and ze's own RPC
// transport, where a caller parses the answer. This operator is how the second
// caller says which one it is. It is spelled here and nowhere else.
const rawPipe = " | raw"

// RawCommand appends the raw pipe to a command, so the daemon answers with the
// dispatcher's JSON whatever format the operator configured.
//
// A caller that owns its own transport uses this. A caller that does not uses
// ExecCommandRaw below, which is the same request over this package's channel.
func RawCommand(command string) string {
	var tb textbuf.Buffer
	return tb.Str(command).Str(rawPipe).String()
}

// ExecCommandRaw runs a command over the SSH channel and returns the
// dispatcher's JSON, unchanged by the configured display format.
//
// Every in-tree caller that PARSES the answer MUST use this rather than
// ExecCommand. ExecCommand answers what an operator would see, and an operator
// who commits `environment cli format default table` would otherwise hand every
// such caller a table to unmarshal. Each one degrades quietly, so the failure
// is invisible until somebody asks why completion stopped offering peers.
func ExecCommandRaw(creds Credentials, command string) (string, error) {
	return ExecCommand(creds, RawCommand(command))
}

// errorPrefix is how the daemon's ssh exec handler formats a failure on the
// session's stderr, so that `ssh <host> <command>` reads well on its own.
const errorPrefix = "error: "

// trimErrorPrefix removes the daemon's display prefix from a remote failure.
// The prefix is formatting for a human reading raw ssh output; once the text
// becomes an error value it is data, and every caller that prints it adds its
// own "error: ". Without this the CLI renders "error: error: <msg>".
//
// Unexported: the only caller is ExecCommand above. It was exported without a
// cross-package consumer, which ze-repository-check reports as an unwired export.
func trimErrorPrefix(s string) string {
	return strings.TrimPrefix(s, errorPrefix)
}

// StreamCommand connects to the daemon via SSH and runs a streaming command.
// It reads stdout line-by-line and calls the callback for each line.
// The callback receives the raw JSON event line. If the callback returns an error,
// streaming stops. The function blocks until the session ends (disconnect or callback error).
func StreamCommand(creds Credentials, command string, callback func(line string) error) error {
	client, err := dialDaemon(creds)
	if err != nil {
		return err
	}
	defer client.Close() //nolint:errcheck // best-effort cleanup

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	defer session.Close() //nolint:errcheck // best-effort cleanup

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := session.Start(command); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	// A line ends with exactly one newline and never with `\r\n`. This stream
	// carries the daemon's rendering rather than answer lines, so nothing here
	// states a width; what the split function buys is that a carriage return an
	// operator's data holds reaches the caller (rpc.ScanLinesKeepingReturns).
	//
	// It is NOT rpc.ScanAnswerLines. That one measures a line by the fields it
	// states, and a rendering line that happens to open with a kind word states
	// none: it would be framed at a width nothing wrote, and the byte found
	// there would refuse the whole stream.
	scanner.Split(rpc.ScanLinesKeepingReturns)
	for scanner.Scan() {
		if err := callback(scanner.Text()); err != nil {
			return err
		}
	}

	// Wait for session to complete (server closed connection).
	waitErr := session.Wait()
	if scanErr := scanner.Err(); scanErr != nil {
		return scanErr
	}
	return waitErr
}

// ProtocolSession holds an open SSH session for bidirectional protocol communication.
// The caller reads from Stdout and writes to Stdin to speak the plugin protocol.
// Caller MUST call Close when done to release SSH resources.
type ProtocolSession struct {
	Stdin  io.WriteCloser
	Stdout io.Reader
	sess   *ssh.Session
	client *ssh.Client
}

// Close terminates the SSH session and underlying connection.
func (ps *ProtocolSession) Close() error {
	ps.sess.Close()   //nolint:errcheck,gosec // best-effort cleanup
	ps.client.Close() //nolint:errcheck,gosec // best-effort cleanup
	return nil
}

// Wait blocks until the remote command exits.
func (ps *ProtocolSession) Wait() error {
	return ps.sess.Wait()
}

// OpenProtocolSession connects to the daemon via SSH and starts a persistent
// bidirectional session with the given command. Returns stdin (write) and
// stdout (read) pipes for speaking the plugin protocol over the SSH channel.
func OpenProtocolSession(creds Credentials, command string) (*ProtocolSession, error) {
	client, err := dialDaemon(creds)
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close() //nolint:errcheck,gosec // cleanup on error
		return nil, fmt.Errorf("create session: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close() //nolint:errcheck,gosec // cleanup on error
		client.Close()  //nolint:errcheck,gosec // cleanup on error
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close() //nolint:errcheck,gosec // cleanup on error
		client.Close()  //nolint:errcheck,gosec // cleanup on error
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := session.Start(command); err != nil {
		session.Close() //nolint:errcheck,gosec // cleanup on error
		client.Close()  //nolint:errcheck,gosec // cleanup on error
		return nil, fmt.Errorf("start command: %w", err)
	}

	return &ProtocolSession{
		Stdin:  stdin,
		Stdout: stdout,
		sess:   session,
		client: client,
	}, nil
}

// ReadCredentials reads SSH credentials from a zefs database using the
// default super-admin username from zefs.
//
// Equivalent to ReadCredentialsWithFlags(dbPath, "").
func ReadCredentials(dbPath string) (Credentials, error) {
	return ReadCredentialsWithFlags(dbPath, "")
}

// ReadCredentialsWithFlags reads SSH credentials and lets the caller supply
// a CLI-flag override for the username. Delegates to ReadCredentialsForRemote
// with empty host/port (follows the default pointer).
//
// Username precedence: cliUser > env ze.ssh.username > zefs.
// Password precedence (super-admin): env ze.ssh.password > zefs hash-as-token.
// Password precedence (other user): env ze.ssh.password > TTY prompt > error.
// Host and port: env > zefs default pointer > 127.0.0.1:2222.
func ReadCredentialsWithFlags(dbPath, cliUser string) (Credentials, error) {
	return ReadCredentialsForRemote(dbPath, cliUser, "", "")
}

// ReadCredentialsForRemote reads SSH credentials for a specific host:port.
// When remoteHost/remotePort are empty, the default pointer is followed.
//
// This variant MAY BLOCK on an interactive password prompt (see readCredentials).
// Callers that cannot accept a prompt -- anything running unattended, and shell
// tab completion in particular -- must use a NoPrompt variant instead.
func ReadCredentialsForRemote(dbPath, cliUser, remoteHost, remotePort string) (Credentials, error) {
	return readCredentials(dbPath, cliUser, remoteHost, remotePort, true)
}

// readCredentials resolves SSH credentials from the zefs store, CLI flags, env,
// and built-in defaults.
//
// allowPrompt decides whether resolution may block on an interactive terminal
// prompt for a password. It is a CALLER policy and is deliberately not inferred
// from the tty state alone: tab completion runs with stdin on the operator's
// terminal, so a tty check would say "prompting is fine" and hang the shell.
// When allowPrompt is false and no non-interactive password source exists,
// resolution fails with an error the caller can degrade on.
//
// The store is one source among several, not a precondition. It is a single
// shared 0600 file under a binary-derived config dir (/usr/local/bin/ze ->
// /etc/ze), so every user who did not install ze is unable to read it. Treating
// it as mandatory refused those users before their credentials were even
// considered, even when the flag, env, and defaults supplied everything needed.
func readCredentials(dbPath, cliUser, remoteHost, remotePort string, allowPrompt bool) (Credentials, error) {
	store, err := openStoreIfReadable(dbPath)
	if err != nil && !errors.Is(err, errStoreUnavailable) {
		return Credentials{}, err
	}
	if store != nil {
		defer store.Close() //nolint:errcheck // read-only access
	}

	host, port := remoteHost, remotePort
	if host == "" || port == "" {
		h, p := resolveHostPort(store)
		if host == "" {
			host = h
		}
		if port == "" {
			port = p
		}
	}

	// Empty when there is no store, or the store holds no entry for this
	// host/port. Either way the flag and env may still name the user.
	zefsUser := storedUsername(store, host, port)

	username := resolveUsername(cliUser, zefsUser)
	if username == "" {
		return Credentials{}, fmt.Errorf(
			"no credentials for %s:%s: no stored username and none supplied "+
				"(pass --user <name> with ze.ssh.password set, or run ze init)", host, port)
	}

	// Only a stored username can be the super-admin. Without this guard an empty
	// username would compare equal to an empty zefsUser and send resolvePassword
	// down the hash-as-token path with no store to read.
	isSuperAdmin := zefsUser != "" && username == zefsUser

	password, err := resolvePassword(store, username, host, port, isSuperAdmin, allowPrompt)
	if err != nil {
		return Credentials{}, err
	}

	return Credentials{
		Host:     host,
		Port:     port,
		Username: username,
		Auth:     password,
	}, nil
}

// errStoreUnavailable reports that the credential store is simply not available
// to this user -- absent, or owned by someone else. Resolution continues without
// it; it is not a failure on its own.
var errStoreUnavailable = errors.New("credential store unavailable")

// openStoreIfReadable opens the credential store, returning errStoreUnavailable
// when the file is missing or unreadable by this user.
//
// Any other failure is returned as-is. A corrupt or truncated store is a real
// problem and must surface as one -- silently downgrading it to "no credentials"
// would turn a loud bug into a confusing authentication failure.
func openStoreIfReadable(dbPath string) (*zefs.BlobStore, error) {
	store, err := zefs.Open(dbPath)
	switch {
	case err == nil:
		return store, nil
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, fs.ErrPermission):
		return nil, fmt.Errorf("%w: %s", errStoreUnavailable, dbPath)
	default:
		return nil, fmt.Errorf("open database: %w", err)
	}
}

// storedUsername returns the super-admin username recorded for host:port, or ""
// when there is no store or no entry for that target.
func storedUsername(store *zefs.BlobStore, host, port string) string {
	if store == nil {
		return ""
	}
	user, err := readKey(store, zefs.KeySSHUsername.Key(host, port))
	if err != nil {
		return ""
	}
	return user
}

// resolveUsername picks a username from the CLI flag, env, or zefs in order.
// Comparison is exact (case- and whitespace-sensitive) to match SSH server
// semantics: `--user "admin "` (trailing space) is a different user than the
// zefs `admin` and will exercise the non-super-admin password path.
func resolveUsername(cliUser, zefsUser string) string {
	if cliUser != "" {
		return cliUser
	}
	if v := env.Get("ze.ssh.username"); v != "" {
		return v
	}
	return zefsUser
}

// resolvePassword returns the SSH credential to send. Super-admin can fall
// back to the zefs hash-as-token; other users must supply a real password
// (env or interactive prompt) because only their bcrypt hash lives in YANG.
//
// allowPrompt is the caller's policy on blocking for input; see readCredentials.
// A false value turns the prompt into an error, which is what lets unattended
// callers degrade instead of hanging.
func resolvePassword(store *zefs.BlobStore, username, host, port string, isSuperAdmin, allowPrompt bool) (string, error) {
	if v := env.Get("ze.ssh.password"); v != "" {
		return v, nil
	}
	if isSuperAdmin {
		return readKey(store, zefs.KeySSHPassword.Key(host, port))
	}
	if allowPrompt && isStdinTTY() {
		return passwordPrompter(username)
	}
	return "", fmt.Errorf("no password source for user %q (set ze.ssh.password or run interactively)", username)
}

// isStdinTTY and passwordPrompter are the two seams that let tests drive the
// prompt decision without a real terminal.
//
// They are package-level vars that only tests ever assign, so a test that
// replaces one MUST NOT call t.Parallel: parallel tests in this package would
// race on the assignment, and the race is silent (a stale func value, not a
// crash). Use stubPromptPolicy in client_test.go, which swaps both and restores
// them via t.Cleanup. Nothing in production writes to either.

// isStdinTTY reports whether stdin is a terminal.
var isStdinTTY = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// passwordPrompter indirects promptPassword so tests can assert whether the
// prompt path was taken.
var passwordPrompter = promptPassword

// promptPassword reads a password from the terminal without echo.
func promptPassword(username string) (string, error) {
	fmt.Fprintf(os.Stderr, "password for %s: ", username)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(pw), nil
}

// resolveHostPort picks host and port from env, zefs default pointer, or built-in defaults.
// Env overrides are treated as a pair: setting either env var bypasses the default pointer
// entirely (the unset var gets the built-in default, not the pointer's value).
func resolveHostPort(store *zefs.BlobStore) (string, string) {
	envHost := env.Get("ze.ssh.host")
	envPort := env.Get("ze.ssh.port")
	if envHost != "" || envPort != "" {
		if envHost == "" {
			envHost = defaultHost
		}
		if envPort == "" {
			envPort = defaultPort
		}
		return envHost, envPort
	}

	// The default pointer lives in the store, so a user who cannot read the store
	// cannot learn it and falls back to the built-in target. A non-default daemon
	// address must then come from ze.ssh.host / ze.ssh.port or --remote.
	if store != nil {
		if dflt, err := readKey(store, zefs.KeySSHDefault.Pattern); err == nil {
			parts := strings.SplitN(dflt, "/", 2)
			if len(parts) == 2 {
				return parts[0], parts[1]
			}
		}
	}
	return defaultHost, defaultPort
}

func readKey(store *zefs.BlobStore, key string) (string, error) {
	data, err := store.ReadFile(key)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", key, err)
	}
	return string(data), nil
}

// hostKeyCallback returns an appropriate host key callback for the given host.
// Localhost connections skip host key verification since the daemon runs on the
// same machine. Remote connections reject by default unless ze.ssh.insecure=true.
func hostKeyCallback(host string) (ssh.HostKeyCallback, error) {
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // ze owns the host; localhost resolution trusted
	default:
		if env.IsEnabled("ze.ssh.insecure") {
			return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // explicit opt-in via ze.ssh.insecure
		}
		return nil, fmt.Errorf("remote SSH host %q: host key verification required (set ze.ssh.insecure=true to override)", host)
	}
}

// ResolveDBPath determines the database.zefs path from the resolved config dir.
func ResolveDBPath() string {
	dir := paths.DefaultConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "database.zefs")
}

// LoadCredentials reads SSH credentials from the default zefs database
// using the zefs super-admin username (no CLI flag override).
//
// Preserved for callers that have not yet adopted --user.
//
// MAY BLOCK on an interactive password prompt. Unattended callers want
// LoadCredentialsNoPrompt instead.
func LoadCredentials() (Credentials, error) {
	return LoadCredentialsWithFlags("")
}

// LoadCredentialsNoPrompt is LoadCredentials for callers that must never block
// on input, however interactive the terminal happens to look.
//
// Shell tab completion is the motivating case: it runs with stdin attached to
// the operator's terminal, so the tty check in resolvePassword would happily
// prompt and freeze the shell mid-completion. Resolution here fails with
// "no password source" instead, letting the caller degrade quietly.
func LoadCredentialsNoPrompt() (Credentials, error) {
	dbPath := ResolveDBPath()
	if dbPath == "" {
		return Credentials{}, errCannotDetermineDatabaseLocation
	}
	return readCredentials(dbPath, "", "", "", false)
}

// LoadCredentialsWithFlags reads SSH credentials from the default zefs
// database, applying a CLI-flag username override when non-empty.
// See ReadCredentialsWithFlags for the full precedence rules.
func LoadCredentialsWithFlags(cliUser string) (Credentials, error) {
	dbPath := ResolveDBPath()
	if dbPath == "" {
		return Credentials{}, errCannotDetermineDatabaseLocation
	}
	return ReadCredentialsWithFlags(dbPath, cliUser)
}

// LoadCredentialsForRemote reads SSH credentials for a specific remote
// host:port from the default zefs database. Used by --remote flag handlers.
func LoadCredentialsForRemote(cliUser, host, port string) (Credentials, error) {
	dbPath := ResolveDBPath()
	if dbPath == "" {
		return Credentials{}, errCannotDetermineDatabaseLocation
	}
	return ReadCredentialsForRemote(dbPath, cliUser, host, port)
}
