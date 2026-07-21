// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS admin authenticator
// Overview: config.go -- ExtractedConfig this authenticator consumes
// Related: aaa.go -- backend Build that constructs this authenticator
// RFC: rfc/short/rfc2865.md -- Access-Request/Accept/Reject, User-Password (5.2)

// radiusAuthenticator bridges the RADIUS client to aaa.Authenticator for
// operator/admin login (SSH, web, MCP). It is entirely distinct from the L2TP
// subscriber RADIUS path and shares only the transport-level radius.Client.
package radius

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
)

const (
	// serviceTypeLogin is RFC 2865 Section 5.6 Login-User: the Service-Type an
	// operator logging in to a device advertises. Defined here rather than in
	// the shared dict.go (used by the L2TP subscriber path) so the admin
	// backend's changes stay self-contained.
	serviceTypeLogin = 1

	minAuthBudget = 5 * time.Second
	maxAuthBudget = 2 * time.Minute
)

// radiusAuthenticator implements aaa.Authenticator using a RADIUS client.
type radiusAuthenticator struct {
	client          *Client
	nasID           string
	sourceIP        net.IP
	profileAttr     uint8
	defaultProfiles []string
	budget          time.Duration
	logger          *slog.Logger
}

// newRadiusAuthenticator builds an authenticator from the extracted config.
// nasID identifies this device to the RADIUS server (RFC 2865 Section 4.1).
func newRadiusAuthenticator(client *Client, cfg ExtractedConfig, nasID string, logger *slog.Logger) *radiusAuthenticator {
	if logger == nil {
		logger = slog.Default()
	}
	profileAttr := cfg.ProfileAttr
	if profileAttr == 0 {
		profileAttr = AttrFilterID
	}
	return &radiusAuthenticator{
		client:          client,
		nasID:           nasID,
		sourceIP:        cfg.SourceAddress,
		profileAttr:     profileAttr,
		defaultProfiles: cfg.DefaultProfiles,
		budget:          authBudget(cfg),
		logger:          logger,
	}
}

// authBudget bounds the total time one login spends talking to RADIUS before
// the AAA chain falls through to the next backend (local). For a normal config
// it is large enough to let the client exhaust its configured retransmits
// against every server, but always capped so a slow or unreachable server can
// never hang a login (R-5): an exceeded budget surfaces as an infra error,
// never a rejection. Edge case: retries=0 (valid per YANG) yields the 5s floor
// here while NewClient coerces Retries 0->3 (client.go), so the context may
// cancel mid-retransmit -- still fail-safe (faster fallthrough to local).
func authBudget(cfg ExtractedConfig) time.Duration {
	perServer := cfg.Timeout << uint(cfg.Retries) // upper-bounds the backoff sum
	servers := max(len(cfg.Servers), 1)
	budget := perServer*time.Duration(servers) + time.Second
	if budget < minAuthBudget {
		return minAuthBudget
	}
	if budget > maxAuthBudget {
		return maxAuthBudget
	}
	return budget
}

// Authenticate performs PAP (RFC 2865 User-Password) authentication against
// the configured RADIUS servers.
//
// Returns:
//   - (success, nil) on Access-Accept, Profiles mapped from the reply
//   - (zero, ErrAuthRejected) on Access-Reject so the chain stops
//   - (zero, ErrAuthRejected) on an Access-Accept that resolves to no profile
//     names, whether the reply carried none and no default is configured, or the
//     names it carried were all empty
//   - (zero, other error) on timeout/socket/unexpected-code so the chain tries
//     the next backend (local fallback)
func (a *radiusAuthenticator) Authenticate(request aaa.AuthRequest) (aaa.AuthResult, error) {
	auth, err := RandomAuthenticator()
	if err != nil {
		return aaa.AuthResult{}, fmt.Errorf("radius: random authenticator: %w", err)
	}

	// RFC 2865 Section 4.1: an Access-Request MUST carry NAS-IP-Address or
	// NAS-Identifier. The password is placed in cleartext here; the client
	// XOR-hides it per-server (Section 5.2) inside Exchange, so it never
	// reaches the wire in the clear.
	attrs := []Attr{
		{Type: AttrUserName, Value: AttrString(request.Username)},
		{Type: AttrUserPassword, Value: []byte(request.Password)},
		{Type: AttrServiceType, Value: AttrUint32(serviceTypeLogin)},
		{Type: AttrNASIdentifier, Value: AttrString(a.nasID)},
	}
	if v4 := a.sourceIP.To4(); v4 != nil {
		attrs = append(attrs, Attr{Type: AttrNASIPAddress, Value: v4})
	}

	pkt := &Packet{
		Code:          CodeAccessRequest,
		Authenticator: auth,
		Attrs:         attrs,
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.budget)
	defer cancel()

	resp, err := a.client.SendToServers(ctx, pkt)
	if err != nil {
		// Infra failure (timeout / all servers unreachable): let the chain try
		// the next backend rather than lock the operator out (R-4).
		return aaa.AuthResult{}, fmt.Errorf("radius: %w", err)
	}

	switch resp.Code {
	case CodeAccessAccept:
		// An Accept that resolves to no profile names is a denial, not a
		// successful login with nothing attached. The server accepting is not the
		// question: what matters is whether the login names a profile.
		//
		// This shape needs no misconfiguration. RFC 2865 does not require a
		// Filter-Id in an Access-Accept, and default-profile is an optional
		// leaf-list, so ExtractConfig assigns GetSlice("default-profile")
		// unconditionally (config.go:103) and Tree.GetSlice returns nil both when
		// the leaf-list is absent -- the out-of-the-box config -- and when every
		// member is deactivated (tree.go:183-185).
		//
		// Returning success with an empty set would ESCALATE rather than restrict.
		// aaa.RecordLoginProfiles ignores an empty slice (login_profiles.go:46), so
		// nothing is recorded; authz.Store.Authorize then finds no assignment and no
		// login profiles. This is the primary guard: it rejects the login here,
		// before authorization runs. authz.Store.Authorize now also fails closed for
		// a user that resolves no profile (spec-fixit-authz-admin-fallthrough), so
		// the escalation is closed at both layers; historically this branch fell
		// through to the built-in admin profile and a server that omitted Filter-Id
		// handed every user admin.
		//
		// This is NOT the R-4 case above. There, SendToServers produced no answer at
		// all, so asking the next backend is right and locking the operator out on an
		// infra blip would be wrong. Here a reachable server answered: the profile set
		// resolving to zero is the CONTENT of that answer, not the absence of one.
		// Falling through would let a local account shadow the server's verdict, so
		// this rejects like Access-Reject does -- the chain stops.
		profiles := a.mapProfiles(resp)
		if len(profiles) == 0 {
			a.logger.Warn("RADIUS admin auth rejected: no profiles resolved",
				"username", request.Username)
			return aaa.AuthResult{Source: aaaName}, aaa.ErrAuthRejected
		}

		a.logger.Info("RADIUS admin auth accepted",
			"username", request.Username, "profiles", profiles)
		return aaa.AuthResult{
			Authenticated: true,
			Profiles:      profiles,
			Source:        aaaName,
		}, nil
	case CodeAccessReject:
		a.logger.Info("RADIUS admin auth rejected", "username", request.Username)
		return aaa.AuthResult{Source: aaaName}, aaa.ErrAuthRejected
	default:
		return aaa.AuthResult{}, fmt.Errorf("radius: unexpected response code %d", resp.Code)
	}
}

// mapProfiles reads the configured reply attribute (default Filter-Id) as ze
// authorization profile names, one profile per attribute instance. When the
// Access-Accept carries none, the configured default profiles apply (AC-6).
//
// May return an empty set: the reply named no profiles and no default is
// configured. That is a legal config and a legal reply, so this stays a pure
// mapping and leaves the meaning to Authenticate, which rejects the login.
func (a *radiusAuthenticator) mapProfiles(resp *Packet) []string {
	var profiles []string
	for _, v := range resp.FindAllAttr(a.profileAttr) {
		s := string(v)
		if s == "" {
			continue
		}
		// Fail closed: the reply attribute is untrusted server input and can never
		// name a reserved identity. Dropping it stops a hostile or compromised
		// RADIUS server from spoofing the break-glass recovery profile (or any
		// reserved name) over the wire, which authz.Store.Authorize would otherwise
		// grant as allow-all admin. The only legitimate source of a reserved
		// profile is the code-controlled local backend (usersFromZefsDB).
		if aaa.IsReservedName(s) {
			a.logger.Warn("RADIUS reply named a reserved profile; dropping",
				"attr", a.profileAttr)
			continue
		}
		profiles = append(profiles, s)
	}
	if len(profiles) == 0 {
		return a.defaultProfiles
	}
	return profiles
}
