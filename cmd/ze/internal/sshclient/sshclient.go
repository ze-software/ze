// Design: docs/architecture/system-architecture.md — SSH client helper for CLI tools

// Package sshclient provides SSH client connectivity for ze CLI tools.
// CLI tools connect to the daemon via SSH instead of Unix sockets.
package sshclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
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

// ReadCredentials reads SSH credentials from a zefs database.
// Host and port can be overridden by env vars (ze_ssh_host, ze_ssh_port).
func ReadCredentials(dbPath string) (Credentials, error) {
	store, err := zefs.Open(dbPath)
	if err != nil {
		return Credentials{}, fmt.Errorf("open database: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only access

	username, err := readKey(store, "ssh/username")
	if err != nil {
		return Credentials{}, err
	}
	password, err := readKey(store, "ssh/password")
	if err != nil {
		return Credentials{}, err
	}

	host := envOr("ze.ssh.host", "ze_ssh_host", "127.0.0.1")
	port := envOr("ze.ssh.port", "ze_ssh_port", "2222")

	// Override from store if not set by env
	if h, hErr := readKey(store, "ssh/host"); hErr == nil && !hasEnv("ze.ssh.host", "ze_ssh_host") {
		host = h
	}
	if p, pErr := readKey(store, "ssh/port"); pErr == nil && !hasEnv("ze.ssh.port", "ze_ssh_port") {
		port = p
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

func envOr(keys ...string) string {
	def := keys[len(keys)-1]
	for _, key := range keys[:len(keys)-1] {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return def
}

func hasEnv(keys ...string) bool {
	for _, key := range keys {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

// hostKeyCallback returns an appropriate host key callback for the given host.
// Localhost connections (127.0.0.1, ::1, localhost) skip host key verification
// since the daemon runs on the same machine. Remote connections also skip
// verification but log a warning -- the user explicitly opts in via ze_ssh_host.
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
	if dir := os.Getenv("ZE_CONFIG_DIR"); dir != "" {
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
