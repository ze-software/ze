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
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

var errCannotDetermineDatabaseLocation = errors.New("cannot determine database location")

var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.host", Type: "string", Default: "127.0.0.1", Description: "Override SSH host"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.port", Type: "string", Default: "2222", Description: "Override SSH port"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.username", Type: "string", Description: "Override SSH username (default: zefs super-admin)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.password", Type: "string", Description: "SSH password (zefs stores bcrypt hash)", Secret: true})
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
func ExecCommand(creds Credentials, command string) (string, error) {
	hkCb, err := hostKeyCallback(creds.Host)
	if err != nil {
		return "", err
	}
	config := &ssh.ClientConfig{
		User: creds.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(creds.Auth),
		},
		HostKeyCallback: hkCb,
		Timeout:         dialTimeout,
	}

	var tb textbuf.Buffer
	addr := tb.Str(creds.Host).Byte(':').Str(creds.Port).String()
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
			return "", errors.New(TrimErrorPrefix(strings.TrimSpace(string(output))))
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// errorPrefix is how the daemon's ssh exec handler formats a failure on the
// session's stderr, so that `ssh <host> <command>` reads well on its own.
const errorPrefix = "error: "

// TrimErrorPrefix removes the daemon's display prefix from a remote failure.
// The prefix is formatting for a human reading raw ssh output; once the text
// becomes an error value it is data, and every caller that prints it adds its
// own "error: ". Without this the CLI renders "error: error: <msg>".
func TrimErrorPrefix(s string) string {
	return strings.TrimPrefix(s, errorPrefix)
}

// StreamCommand connects to the daemon via SSH and runs a streaming command.
// It reads stdout line-by-line and calls the callback for each line.
// The callback receives the raw JSON event line. If the callback returns an error,
// streaming stops. The function blocks until the session ends (disconnect or callback error).
func StreamCommand(creds Credentials, command string, callback func(line string) error) error {
	hkCb, err := hostKeyCallback(creds.Host)
	if err != nil {
		return err
	}
	config := &ssh.ClientConfig{
		User: creds.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(creds.Auth),
		},
		HostKeyCallback: hkCb,
		Timeout:         dialTimeout,
	}

	var tb textbuf.Buffer
	addr := tb.Str(creds.Host).Byte(':').Str(creds.Port).String()
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
	hkCb, err := hostKeyCallback(creds.Host)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User: creds.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(creds.Auth),
		},
		HostKeyCallback: hkCb,
		Timeout:         dialTimeout,
	}

	var tb textbuf.Buffer
	addr := tb.Str(creds.Host).Byte(':').Str(creds.Port).String()
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
func ReadCredentialsForRemote(dbPath, cliUser, remoteHost, remotePort string) (Credentials, error) {
	store, err := zefs.Open(dbPath)
	if err != nil {
		return Credentials{}, fmt.Errorf("open database: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only access

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

	usernameKey := zefs.KeySSHUsername.Key(host, port)
	zefsUser, err := readKey(store, usernameKey)
	if err != nil {
		return Credentials{}, fmt.Errorf("no credentials for %s:%s", host, port)
	}

	username := resolveUsername(cliUser, zefsUser)
	isSuperAdmin := username == zefsUser

	password, err := resolvePassword(store, username, host, port, isSuperAdmin)
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
func resolvePassword(store *zefs.BlobStore, username, host, port string, isSuperAdmin bool) (string, error) {
	if v := env.Get("ze.ssh.password"); v != "" {
		return v, nil
	}
	if isSuperAdmin {
		return readKey(store, zefs.KeySSHPassword.Key(host, port))
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

	if dflt, err := readKey(store, zefs.KeySSHDefault.Pattern); err == nil {
		parts := strings.SplitN(dflt, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
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
func LoadCredentials() (Credentials, error) {
	return LoadCredentialsWithFlags("")
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
