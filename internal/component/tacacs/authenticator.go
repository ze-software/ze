// Design: (none -- new TACACS+ component)
// Overview: client.go -- TACACS+ TCP client
// Related: accounting.go -- accounting bridge (sibling wrapper around client)
// Related: authorizer.go -- authorization bridge (sibling wrapper around client)

// tacacsAuthenticator bridges the TACACS+ client to aaa.Authenticator.
package tacacs

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/ze-software/ze/internal/component/aaa"
)

// tacacsAuthenticator implements aaa.Authenticator using a TACACS+ client.
type tacacsAuthenticator struct {
	client     *TacacsClient
	privLvlMap map[int][]string // priv-lvl -> ze profile names
	logger     *slog.Logger
}

// newTacacsAuthenticator creates a tacacsAuthenticator.
// privLvlMap maps TACACS+ privilege levels (0-15) to ze authz profile names.
func newTacacsAuthenticator(client *TacacsClient, privLvlMap map[int][]string, logger *slog.Logger) *tacacsAuthenticator {
	if logger == nil {
		logger = slog.Default()
	}
	return &tacacsAuthenticator{
		client:     client,
		privLvlMap: privLvlMap,
		logger:     logger,
	}
}

// Authenticate performs PAP authentication against the TACACS+ server(s).
//
// Returns:
//   - (success result, nil) after PASS and session authorization with mapped priv-lvl
//   - (rejected result, ErrAuthRejected) on either exchange's explicit rejection
//   - (rejected result, ErrAuthRejected) on unsupported mandatory session policy
//     or a privilege level that resolves to no profiles
//   - (zero, error) on ERROR status or connection failure (chain tries next backend)
func (a *tacacsAuthenticator) Authenticate(request aaa.AuthRequest) (aaa.AuthResult, error) {
	username, err := prepareUsername(request.Username)
	if err != nil {
		return aaa.AuthResult{Source: backendName}, aaa.ErrAuthRejected
	}
	if aaa.IsReservedName(username) {
		return aaa.AuthResult{Source: backendName}, aaa.ErrAuthRejected
	}
	request.Username = username
	service := request.Service
	if service == "" {
		service = defaultService
	}

	reply, err := a.client.Authenticate(request.Username, request.Password, service, request.RemoteAddr)
	if err != nil {
		if errors.Is(err, errRequestInvalid) {
			return aaa.AuthResult{Source: backendName}, aaa.ErrAuthRejected
		}
		// Connection failure: let chain try next backend.
		return aaa.AuthResult{}, fmt.Errorf("tacacs: %w", err)
	}

	// Handle each status explicitly. RFC 8907 Section 5.2.
	if reply.Status == AuthenStatusPass {
		return a.handlePass(request)
	}
	if reply.Status == AuthenStatusFail {
		// Explicit rejection: chain must NOT try next backend.
		return aaa.AuthResult{Source: backendName}, aaa.ErrAuthRejected
	}
	// RFC 8907 Section 5.4.3: "If a client does not implement the
	// TAC_PLUS_AUTHEN_STATUS_RESTART option, then it MUST process the
	// response as if the status was TAC_PLUS_AUTHEN_STATUS_FAIL." Ze runs a
	// single PAP exchange and never restarts with another authen_type, so a
	// RESTART is a rejection, not an infrastructure failure: the chain stops
	// here exactly as it does on FAIL.
	if reply.Status == AuthenStatusRestart {
		return aaa.AuthResult{Source: backendName}, aaa.ErrAuthRejected
	}
	// RFC 8907 Section 10.5.5: "TACACS+ clients SHOULD deprecate this
	// feature by treating TAC_PLUS_AUTHEN_STATUS_FOLLOW as
	// TAC_PLUS_AUTHEN_STATUS_FAIL."
	if reply.Status == AuthenStatusFollow {
		return aaa.AuthResult{Source: backendName}, aaa.ErrAuthRejected
	}

	// Unknown status: treat as infrastructure failure.
	return aaa.AuthResult{}, fmt.Errorf("tacacs: unexpected authen status 0x%02x", reply.Status)
}

// handlePass obtains the session policy before granting profiles. PAP reply
// data is not a privilege assignment: RFC 8907 Section 9 states, "This privilege
// level is returned by the server in a session-based shell authorization (when
// \"service\" equals \"shell\" and \"cmd\" is empty).".
func (a *tacacsAuthenticator) handlePass(request aaa.AuthRequest) (aaa.AuthResult, error) {
	args := []string{"service=shell", "cmd="}
	reply, err := a.client.SendAuthorization(&AuthorRequest{
		AuthenMethod:  AuthenMethodTACACS,
		PrivLvl:       1,
		AuthenType:    authenTypePAP,
		AuthenService: authenServiceLogin,
		User:          request.Username,
		Port:          portSSH,
		RemAddr:       request.RemoteAddr,
		Args:          args,
	})
	if err != nil {
		if errors.Is(err, errRequestInvalid) {
			return aaa.AuthResult{Source: backendName}, aaa.ErrAuthRejected
		}
		return aaa.AuthResult{}, fmt.Errorf("tacacs session authorization: %w", err)
	}
	privLvl, allowed := applyAuthorizationArgs(args, reply, true)
	if !allowed {
		return aaa.AuthResult{Source: backendName}, aaa.ErrAuthRejected
	}

	// A level that resolves to no profile names is treated as unmapped, not as a
	// successful login with nothing attached. Presence in the map is not the
	// question: what matters is whether the mapping names a profile.
	//
	// An empty entry is reachable from config. ExtractConfig assigns
	// GetSlice("profile") unconditionally (config.go:103), and Tree.GetSlice
	// returns nil both when the leaf-list is absent (`tacacs-profile { level 15; }`)
	// and when every member is deactivated -- so the key is present with an empty
	// value and a plain `, ok :=` lookup reports it as mapped.
	//
	// Returning success with an empty set would escalate rather than restrict.
	// The result-scoped authorizer fails closed when no profile resolves. This
	// primary guard rejects the login before authorization runs. Historically,
	// this branch fell through to the built-in admin profile and handed the level
	// admin.
	profiles, ok := a.privLvlMap[privLvl]
	// Defense in depth: ValidateAuthzConfig already rejects a reserved-name
	// reference in a tacacs-profile mapping, but strip any that reach here so a
	// priv-level can never resolve to a reserved identity (the break-glass recovery
	// profile or the trusted internal identity) even if the map were built without
	// validation. Only the code-controlled local backend may deliver a reserved
	// profile. This also returns a fresh slice, never aliasing the config map.
	profiles = aaa.FilterReservedNames(profiles)
	if !ok || len(profiles) == 0 {
		// AC-18: an unmapped, empty, or all-reserved priv-lvl denies access.
		a.logger.Warn("TACACS+ unmapped privilege level",
			"username", request.Username, "priv-lvl", privLvl)
		return aaa.AuthResult{Source: backendName}, aaa.ErrAuthRejected
	}

	a.logger.Info("TACACS+ auth success",
		"username", request.Username, "priv-lvl", privLvl, "profiles", profiles)
	return aaa.AuthResult{
		Authenticated: true,
		Profiles:      profiles,
		Source:        backendName,
	}, nil
}

// parsePrivLvl accepts only the decimal privilege range defined in RFC 8907
// Section 9. Leading zeroes fit only within the two-digit representation.
func parsePrivLvl(value string) (int, bool) {
	// RFC 8907 Section 8.1: "All arguments include a length field, and
	// TACACS+ implementations MUST verify that they can accommodate the
	// lengths of numeric arguments before attempting to process them."
	if value == "" || len(value) > 2 {
		return 0, false
	}
	level := 0
	for i := range len(value) {
		if value[i] < '0' || value[i] > '9' {
			return 0, false
		}
		level = level*10 + int(value[i]-'0')
	}
	return level, level <= 15
}
