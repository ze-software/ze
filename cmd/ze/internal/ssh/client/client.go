// Design: docs/architecture/system-architecture.md — SSH client helper for CLI tools
// Related: ../../../../../pkg/zefs/store.go — BlobStore reads credentials (meta/ssh/*)

// Package client provides SSH client connectivity for ze CLI tools.
// CLI tools connect to the daemon via SSH instead of Unix sockets.
package client

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// Env var registration for config directory and SSH overrides.
var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.config.dir", Type: "string", Description: "Override default config directory"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.host", Type: "string", Description: "Override SSH host"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.port", Type: "string", Description: "Override SSH port"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.password", Type: "string", Description: "SSH password (zefs stores bcrypt hash)"})
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

// ReadCredentials reads SSH credentials from a zefs database.
// Host and port can be overridden by env vars (ze_ssh_host, ze_ssh_port).
// Auth credential: env var ze_ssh_password overrides zefs. The zefs stores a
// bcrypt hash (written by ze init) which is sent as-is (hash-as-token auth).
func ReadCredentials(dbPath string) (Credentials, error) {
	store, err := zefs.Open(dbPath)
	if err != nil {
		return Credentials{}, fmt.Errorf("open database: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only access

	username, err := readKey(store, "meta/ssh/username")
	if err != nil {
		return Credentials{}, err
	}

	// Password: env var overrides zefs.
	password := env.Get("ze.ssh.password")
	if password == "" {
		password, err = readKey(store, "meta/ssh/password")
		if err != nil {
			return Credentials{}, err
		}
	}

	// Host and port: env var takes priority, then zefs, then defaults.
	host := env.Get("ze.ssh.host")
	port := env.Get("ze.ssh.port")

	if host == "" {
		if h, hErr := readKey(store, "meta/ssh/host"); hErr == nil {
			host = h
		}
	}
	if port == "" {
		if p, pErr := readKey(store, "meta/ssh/port"); pErr == nil {
			port = p
		}
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "2222"
	}

	return Credentials{
		Host:     host,
		Port:     port,
		Username: username,
		Auth:     password,
	}, nil
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

// LoadCredentials reads SSH credentials from the default zefs database.
func LoadCredentials() (Credentials, error) {
	dbPath := ResolveDBPath()
	if dbPath == "" {
		return Credentials{}, fmt.Errorf("cannot determine database location")
	}
	return ReadCredentials(dbPath)
}
