// Design: docs/guide/operations.md -- verified remote SSH connections.

package sshclient

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
)

// The trust file is caller-side bootstrap input: remote configuration cannot
// select the trust used to authenticate that same remote server.
var _ = env.MustRegister(env.EnvEntry{Key: "ze.ssh.known.hosts", Type: envTypeString, Description: "OpenSSH known_hosts file for remote SSH trust"})

func knownHostsPath() (string, error) {
	if path := env.Get("ze.ssh.known.hosts"); path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", opererr.Wrap(err, "The SSH host-key trust file could not be located", "Set the caller's home directory or select an OpenSSH known_hosts file containing verified device keys.", "export ZE_SSH_KNOWN_HOSTS=/absolute/path/to/verified/known_hosts")
	}
	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

func knownHostKeyCallback(target string) (ssh.HostKeyCallback, error) {
	path, err := knownHostsPath()
	if err != nil {
		return nil, err
	}
	verify, err := knownhosts.New(path)
	if err != nil {
		return nil, opererr.Wrap(err, "The SSH host-key trust file could not be loaded: "+path, "Provision this OpenSSH file with verified device keys. Before accepting a key in OpenSSH, compare its fingerprint with a trusted device inventory or operator.", knownHostsConnectCommand(target, path))
	}
	return func(address string, remote net.Addr, key ssh.PublicKey) error {
		if err := verify(address, remote, key); err != nil {
			return knownHostKeyError(err, address, path, key)
		}
		return nil
	}, nil
}

func knownHostKeyError(cause error, address, path string, key ssh.PublicKey) error {
	problem := "The SSH host key could not be verified for " + address
	why := fmt.Sprintf("Ze refused the connection using %s. The server offered %s. Inspect the trusted entry and verify the device identity before changing it.", path, ssh.FingerprintSHA256(key))
	fix := "ssh-keygen -F " + hostKeyShellArg(knownhosts.Normalize(address)) + " -f " + hostKeyShellArg(path)
	var revoked *knownhosts.RevokedError
	var keyError *knownhosts.KeyError
	switch {
	case errors.As(cause, &revoked):
		problem = "The SSH host key is revoked for " + address
	case errors.As(cause, &keyError):
		if len(keyError.Want) == 0 {
			problem = "The SSH host key is unknown for " + address
			why = fmt.Sprintf("No matching trusted key exists in %s. The server offered %s. Compare this fingerprint with a trusted device inventory or operator before accepting it in OpenSSH.", path, ssh.FingerprintSHA256(key))
			fix = knownHostsConnectCommand(address, path)
		} else {
			problem = "The SSH host key changed for " + address
		}
	}
	return opererr.Wrap(cause, problem, why, fix)
}

// OpenSSH prompts before enrolling an unknown key. Ze never enrolls a key;
// the operator MUST verify its fingerprint through a trusted channel first.
func knownHostsConnectCommand(target, path string) string {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		host, port = target, defaultPort
	}
	// OpenSSH parses -o values again after the shell has formed argv. Quote
	// its filename first, then quote the complete option for the shell.
	option := "UserKnownHostsFile=" + hostKeyOpenSSHArg(path)
	return "ssh -o " + hostKeyShellArg(option) + " -o StrictHostKeyChecking=ask -p " + hostKeyShellArg(port) + " -- " + hostKeyShellArg(host)
}

// OpenSSH's argv_split consumes backslash and quote escapes independently of
// the shell. Double quotes preserve spaces and apostrophes in one filename.
func hostKeyOpenSSHArg(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return "\"" + value + "\""
}

// Arguments in an operator repair are shell text, not Go-quoted strings.
func hostKeyShellArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
