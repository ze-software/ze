// Design: docs/architecture/system-architecture.md — SSH client helper for CLI tools
//
// Package client is a thin compatibility shim over internal/component/cli/sshclient.
//
// The SSH command-channel client lives under internal/component/cli so that
// command owners under internal/component and internal/plugins can reach the
// daemon without importing anything beneath cmd/ze. This shim re-exports the API at the old
// import path so existing cmd/ze callers keep compiling; new code should import
// github.com/ze-software/ze/internal/component/cli/sshclient directly.
package client

import "github.com/ze-software/ze/internal/component/cli/sshclient"

// Re-exported types. Aliases preserve ProtocolSession's methods and keep
// Credentials assignable across the shim boundary.
type (
	// Credentials holds the connection details for the daemon SSH channel.
	Credentials = sshclient.Credentials
	// ProtocolSession is an open interactive session over the SSH channel.
	ProtocolSession = sshclient.ProtocolSession
)

// ExecCommand runs a single command over the SSH channel and returns its output.
func ExecCommand(creds Credentials, command string) (string, error) {
	return sshclient.ExecCommand(creds, command)
}

// StreamCommand runs a command and streams each output line to callback.
func StreamCommand(creds Credentials, command string, callback func(line string) error) error {
	return sshclient.StreamCommand(creds, command, callback)
}

// OpenProtocolSession opens an interactive protocol session over the SSH channel.
func OpenProtocolSession(creds Credentials, command string) (*ProtocolSession, error) {
	return sshclient.OpenProtocolSession(creds, command)
}

// ReadCredentials reads SSH credentials from the configured store directory.
func ReadCredentials(dbPath string) (Credentials, error) {
	return sshclient.ReadCredentials(dbPath)
}

// ReadCredentialsWithFlags reads credentials, overriding the user with cliUser.
func ReadCredentialsWithFlags(dbPath, cliUser string) (Credentials, error) {
	return sshclient.ReadCredentialsWithFlags(dbPath, cliUser)
}

// ReadCredentialsForRemote reads credentials for a remote host:port target.
func ReadCredentialsForRemote(dbPath, cliUser, remoteHost, remotePort string) (Credentials, error) {
	return sshclient.ReadCredentialsForRemote(dbPath, cliUser, remoteHost, remotePort)
}

// LoadCredentials loads credentials from the default store directory.
// May block on an interactive password prompt. Callers that must not block
// (shell tab completion) want sshclient.LoadCredentialsNoPrompt; no re-export exists
// here because this facade has no importers to serve.
func LoadCredentials() (Credentials, error) { return sshclient.LoadCredentials() }

// LoadCredentialsWithFlags loads credentials with a user override.
func LoadCredentialsWithFlags(cliUser string) (Credentials, error) {
	return sshclient.LoadCredentialsWithFlags(cliUser)
}

// LoadCredentialsForRemote loads credentials for a remote host:port target.
func LoadCredentialsForRemote(cliUser, host, port string) (Credentials, error) {
	return sshclient.LoadCredentialsForRemote(cliUser, host, port)
}
