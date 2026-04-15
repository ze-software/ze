// Design: (none -- new TACACS+ component)
// Overview: packet.go -- packet header and encryption

// TacacsAuthenticator bridges the TACACS+ client to the authz.Authenticator interface.
package tacacs

import (
	"fmt"
	"log/slog"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
)

// TacacsAuthenticator implements authz.Authenticator using a TACACS+ client.
type TacacsAuthenticator struct {
	client     *TacacsClient
	privLvlMap map[int][]string // priv-lvl -> ze profile names
	logger     *slog.Logger
}

// NewTacacsAuthenticator creates a TacacsAuthenticator.
// privLvlMap maps TACACS+ privilege levels (0-15) to ze authz profile names.
func NewTacacsAuthenticator(client *TacacsClient, privLvlMap map[int][]string, logger *slog.Logger) *TacacsAuthenticator {
	if logger == nil {
		logger = slog.Default()
	}
	return &TacacsAuthenticator{
		client:     client,
		privLvlMap: privLvlMap,
		logger:     logger,
	}
}

// Authenticate performs PAP authentication against the TACACS+ server(s).
//
// Returns:
//   - (success result, nil) on PASS with mapped priv-lvl
//   - (rejected result, ErrAuthRejected) on FAIL (explicit rejection, chain stops)
//   - (rejected result, ErrAuthRejected) on PASS with unmapped priv-lvl (AC-18)
//   - (zero, error) on ERROR status or connection failure (chain tries next backend)
func (a *TacacsAuthenticator) Authenticate(username, password string) (authz.AuthResult, error) {
	// TODO(tacacs): thread remAddr from SSH session when wiring into hub.
	// Currently empty because the Authenticator interface takes (username, password)
	// and the SSH remote address is not available at this layer.
	reply, err := a.client.Authenticate(username, password, "ssh", "")
	if err != nil {
		// Connection failure: let chain try next backend.
		return authz.AuthResult{}, fmt.Errorf("tacacs: %w", err)
	}

	// Handle each status explicitly. RFC 8907 Section 5.2.
	if reply.Status == AuthenStatusPass {
		return a.handlePass(username, reply)
	}
	if reply.Status == AuthenStatusFail {
		// Explicit rejection: chain must NOT try next backend.
		return authz.AuthResult{Source: "tacacs"}, authz.ErrAuthRejected
	}
	if reply.Status == AuthenStatusError {
		// Server error: treat as infrastructure failure, chain tries next.
		msg := reply.ServerMsg
		if msg == "" {
			msg = "server error"
		}
		return authz.AuthResult{}, fmt.Errorf("tacacs: %s", msg)
	}

	// Unknown status: treat as infrastructure failure.
	return authz.AuthResult{}, fmt.Errorf("tacacs: unexpected authen status 0x%02x", reply.Status)
}

// handlePass processes a PASS reply: extracts priv-lvl and maps to ze profiles.
func (a *TacacsAuthenticator) handlePass(username string, reply *AuthenReply) (authz.AuthResult, error) {
	// Extract priv-lvl from server data (convention: first byte of data, or default 1).
	privLvl := 1
	if len(reply.Data) > 0 {
		privLvl = int(reply.Data[0])
	}

	profiles, ok := a.privLvlMap[privLvl]
	if !ok {
		// AC-18: unmapped priv-lvl denies access.
		a.logger.Warn("TACACS+ unmapped privilege level",
			"username", username, "priv-lvl", privLvl)
		return authz.AuthResult{Source: "tacacs"}, authz.ErrAuthRejected
	}

	a.logger.Info("TACACS+ auth success",
		"username", username, "priv-lvl", privLvl, "profiles", profiles)
	return authz.AuthResult{
		Authenticated: true,
		Profiles:      profiles,
		Source:        "tacacs",
	}, nil
}
