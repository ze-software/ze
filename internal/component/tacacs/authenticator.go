// Design: (none -- new TACACS+ component)
// Overview: client.go -- TACACS+ TCP client
// Related: accounting.go -- accounting bridge (sibling wrapper around client)
// Related: authorizer.go -- authorization bridge (sibling wrapper around client)

// TacacsAuthenticator bridges the TACACS+ client to the aaa.Authenticator interface.
package tacacs

import (
	"fmt"
	"log/slog"

	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
)

// TacacsAuthenticator implements aaa.Authenticator using a TACACS+ client.
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
func (a *TacacsAuthenticator) Authenticate(request aaa.AuthRequest) (aaa.AuthResult, error) {
	service := request.Service
	if service == "" {
		service = "ssh"
	}

	reply, err := a.client.Authenticate(request.Username, request.Password, service, request.RemoteAddr)
	if err != nil {
		// Connection failure: let chain try next backend.
		return aaa.AuthResult{}, fmt.Errorf("tacacs: %w", err)
	}

	// Handle each status explicitly. RFC 8907 Section 5.2.
	if reply.Status == AuthenStatusPass {
		return a.handlePass(request.Username, reply)
	}
	if reply.Status == AuthenStatusFail {
		// Explicit rejection: chain must NOT try next backend.
		return aaa.AuthResult{Source: "tacacs"}, aaa.ErrAuthRejected
	}
	if reply.Status == AuthenStatusError {
		// Server error: treat as infrastructure failure, chain tries next.
		msg := reply.ServerMsg
		if msg == "" {
			msg = "server error"
		}
		return aaa.AuthResult{}, fmt.Errorf("tacacs: %s", msg)
	}

	// Unknown status: treat as infrastructure failure.
	return aaa.AuthResult{}, fmt.Errorf("tacacs: unexpected authen status 0x%02x", reply.Status)
}

// handlePass processes a PASS reply: extracts priv-lvl and maps to ze profiles.
func (a *TacacsAuthenticator) handlePass(username string, reply *AuthenReply) (aaa.AuthResult, error) {
	// Extract priv-lvl via the stack-safe PrivLvl field rather than reply.Data.
	// reply.Data aliases the client's pool buffer which has already been
	// Put by the time we get here; reading it would race with any concurrent
	// TACACS+ exchange (auth, authz, or accounting) that Gets the same slot.
	// Defaults to 1 when the server sent no priv-lvl byte.
	privLvl := int(reply.PrivLvl)
	if privLvl == 0 && len(reply.Data) == 0 {
		privLvl = 1
	}

	profiles, ok := a.privLvlMap[privLvl]
	if !ok {
		// AC-18: unmapped priv-lvl denies access.
		a.logger.Warn("TACACS+ unmapped privilege level",
			"username", username, "priv-lvl", privLvl)
		return aaa.AuthResult{Source: "tacacs"}, aaa.ErrAuthRejected
	}

	a.logger.Info("TACACS+ auth success",
		"username", username, "priv-lvl", privLvl, "profiles", profiles)
	return aaa.AuthResult{
		Authenticated: true,
		Profiles:      profiles,
		Source:        "tacacs",
	}, nil
}
