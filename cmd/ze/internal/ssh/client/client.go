// Design: docs/architecture/system-architecture.md — SSH client helper for CLI tools
// Related: ../../../../../pkg/zefs/store.go — BlobStore reads credentials (meta/ssh/*)

// Package client provides SSH client connectivity for ze CLI tools.
// CLI tools connect to the daemon via SSH instead of Unix sockets.
package client

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// Env var registration for config directory and SSH overrides.
var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.config.dir", Type: "string", Description: "Override default config directory"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.host", Type: "string", Default: "127.0.0.1", Description: "Override SSH host"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.port", Type: "string", Default: "2222", Description: "Override SSH port"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.username", Type: "string", Description: "Override SSH username (default: zefs super-admin)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.password", Type: "string", Description: "SSH password (zefs stores bcrypt hash)", Secret: true})
)

// dialTimeout is the maximum time to establish an SSH connection.
const dialTimeout = 10 * time.Second

// Credentials holds SSH connection parameters.
type Credentials struct {
	Host     string
	Port     string
	Username string
	Auth     string // SSH auth credential (read from zefs, never serialized)
}

// ExecCommand connects to the daemon via SSH and runs a command.
// Returns the command output or an error.
func ExecCommand(creds Credentials, command string) (string, error) {
	config := &ssh.ClientConfig{
		User: creds.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(creds.Auth),
		},
		HostKeyCallback: hostKeyCallback(creds.Host),
		Timeout:         dialTimeout,
	}

	addr := creds.Host + ":" + creds.Port
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("connect to %s: %w", addr, err)
	}
	defer client.Close() //nolint:errcheck // best-effort cleanup

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	defer session.Close() //nolint:errcheck // best-effort cleanup

	output, err := session.CombinedOutput(command)
	if err != nil {
		if len(output) > 0 {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(output)))
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// StreamCommand connects to the daemon via SSH and runs a streaming command.
// It reads stdout line-by-line and calls the callback for each line.
// The callback receives the raw JSON event line. If the callback returns an error,
// streaming stops. The function blocks until the session ends (disconnect or callback error).
func StreamCommand(creds Credentials, command string, callback func(line string) error) error {
	config := &ssh.ClientConfig{
		User: creds.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(creds.Auth),
		},
		HostKeyCallback: hostKeyCallback(creds.Host),
		Timeout:         dialTimeout,
	}

	addr := creds.Host + ":" + creds.Port
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", addr, err)
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
	config := &ssh.ClientConfig{
		User: creds.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(creds.Auth),
		},
		HostKeyCallback: hostKeyCallback(creds.Host),
		Timeout:         dialTimeout,
	}

	addr := creds.Host + ":" + creds.Port
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
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
// a CLI-flag override for the username.
//
// Username precedence: cliUser > env ze.ssh.username > zefs meta/ssh/username.
// Password precedence (super-admin path -- cliUser empty or matches zefs user):
//
//	env ze.ssh.password > zefs meta/ssh/password (sent as bcrypt hash-as-token).
//
// Password precedence (different user -- cliUser names a non-super-admin):
//
//	env ze.ssh.password > interactive TTY prompt > error.
//
// Host and port: env > zefs > defaults (127.0.0.1:2222) regardless of user.
func ReadCredentialsWithFlags(dbPath, cliUser string) (Credentials, error) {
	store, err := zefs.Open(dbPath)
	if err != nil {
		return Credentials{}, fmt.Errorf("open database: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only access

	zefsUser, err := readKey(store, zefs.KeySSHUsername.Pattern)
	if err != nil {
		return Credentials{}, err
	}

	username := resolveUsername(cliUser, zefsUser)
	isSuperAdmin := username == zefsUser

	password, err := resolvePassword(store, username, isSuperAdmin)
	if err != nil {
		return Credentials{}, err
	}

	host, port := resolveHostPort(store)

	return Credentials{
		Host:     host,
		Port:     port,
		Username: username,
		Auth:     password,
	}, nil
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
func resolvePassword(store *zefs.BlobStore, username string, isSuperAdmin bool) (string, error) {
	if v := env.Get("ze.ssh.password"); v != "" {
		return v, nil
	}
	if isSuperAdmin {
		return readKey(store, zefs.KeySSHPassword.Pattern)
	}
	if isStdinTTY() {
		return promptPassword(username)
	}
	return "", fmt.Errorf("no password source for user %q (set ze.ssh.password or run interactively)", username)
}

// isStdinTTY reports whether stdin is a terminal.
func isStdinTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

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

// resolveHostPort picks host and port from env, zefs, or built-in defaults.
func resolveHostPort(store *zefs.BlobStore) (string, string) {
	host := env.Get("ze.ssh.host")
	port := env.Get("ze.ssh.port")
	if host == "" {
		if h, hErr := readKey(store, zefs.KeySSHHost.Pattern); hErr == nil {
			host = h
		}
	}
	if port == "" {
		if p, pErr := readKey(store, zefs.KeySSHPort.Pattern); pErr == nil {
			port = p
		}
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "2222"
	}
	return host, port
}

func readKey(store *zefs.BlobStore, key string) (string, error) {
	data, err := store.ReadFile(key)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", key, err)
	}
	return string(data), nil
}

// hostKeyCallback returns an appropriate host key callback for the given host.
// Localhost connections (127.0.0.1, ::1, localhost) skip host key verification
// since the daemon runs on the same machine. Remote connections also skip
// verification but log a warning -- the user explicitly opts in via ze.ssh.host.
func hostKeyCallback(host string) ssh.HostKeyCallback {
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return ssh.InsecureIgnoreHostKey() //nolint:gosec // localhost daemon connection
	default:
		// Remote host: user explicitly configured ze_ssh_host.
		// TODO: support known_hosts or host key pinning for remote targets.
		return ssh.InsecureIgnoreHostKey() //nolint:gosec // user-configured remote host
	}
}

// ResolveDBPath determines the database.zefs path from env or default config dir.
func ResolveDBPath() string {
	if dir := env.Get("ze.config.dir"); dir != "" {
		return filepath.Join(dir, "database.zefs")
	}
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
func LoadCredentials() (Credentials, error) {
	return LoadCredentialsWithFlags("")
}

// LoadCredentialsWithFlags reads SSH credentials from the default zefs
// database, applying a CLI-flag username override when non-empty.
// See ReadCredentialsWithFlags for the full precedence rules.
func LoadCredentialsWithFlags(cliUser string) (Credentials, error) {
	dbPath := ResolveDBPath()
	if dbPath == "" {
		return Credentials{}, fmt.Errorf("cannot determine database location")
	}
	return ReadCredentialsWithFlags(dbPath, cliUser)
}
