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
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/aaa"
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
	method          AuthMethod
	budget          time.Duration
	logger          *slog.Logger
	// random supplies the CHAP challenge and identifier. It is always
	// crypto/rand.Reader outside tests, which set it to force the
	// generation failure the credential branch must fail closed on.
	random io.Reader
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
		method:          cfg.AuthMethod,
		budget:          authBudget(cfg),
		logger:          logger,
		random:          rand.Reader,
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

// Authenticate authenticates against the configured RADIUS servers with the
// credential the auth-method leaf selects: PAP (RFC 2865 User-Password,
// Section 5.2), CHAP (CHAP-Password and CHAP-Challenge, Sections 5.3 and 5.40),
// or EAP inside EAP-Message attributes (RFC 3579 Section 3.1), where ze answers
// as the EAP peer with the password the operator typed.
//
// Returns:
//   - (success, nil) on Access-Accept, Profiles mapped from the reply
//   - (zero, ErrAuthRejected) on Access-Reject so the chain stops
//   - (zero, ErrAuthRejected) on an Access-Accept that resolves to no profile
//     names, whether the reply carried none and no default is configured, or the
//     names it carried were all empty
//   - (zero, other error) on timeout/socket/unexpected-code, on a CHAP
//     challenge that could not be generated, or on an EAP conversation that hit
//     its round cap or could not be continued, so the chain tries the next
//     backend (local fallback)
func (a *radiusAuthenticator) Authenticate(request aaa.AuthRequest) (aaa.AuthResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), a.budget)
	defer cancel()

	if _, isEAP := a.method.EAPType(); isEAP {
		return a.authenticateEAP(ctx, request)
	}

	credential, err := a.credential(request.Password)
	if err != nil {
		return aaa.AuthResult{}, err
	}

	resp, err := a.exchange(ctx, request.Username, credential)
	if err != nil {
		return aaa.AuthResult{}, err
	}
	return a.result(resp, request.Username)
}

// exchange sends one Access-Request carrying credential beside the attributes
// every admin login owes, and returns the server's reply.
//
// The Request Authenticator is drawn here and overwritten by SendToServers,
// which draws its own per server. RFC 2865 Section 4.1 requires a new one with
// each new Identifier, and failover assigns both.
func (a *radiusAuthenticator) exchange(ctx context.Context, username string, credential []Attr) (*Packet, error) {
	auth, err := RandomAuthenticator()
	if err != nil {
		return nil, fmt.Errorf("radius: random authenticator: %w", err)
	}

	// RFC 2865 Section 4.1: an Access-Request MUST carry NAS-IP-Address or
	// NAS-Identifier.
	attrs := make([]Attr, 0, len(credential)+4)
	attrs = append(attrs, credential...)
	attrs = append(attrs,
		Attr{Type: AttrServiceType, Value: AttrUint32(serviceTypeLogin)},
		Attr{Type: AttrNASIdentifier, Value: AttrString(a.nasID)},
	)
	// RFC 2865 Section 5: "Text of length zero (0) MUST NOT be sent; omit the
	// entire attribute instead." A login carrying no name would otherwise put a
	// zero-length User-Name on the wire. Section 4.1 makes User-Name a SHOULD,
	// and NAS-Identifier above already meets the MUST that Section 4.1 states,
	// so omitting it leaves the request conformant.
	attrs = AppendTextAttr(attrs, AttrUserName, username)
	if v4 := a.sourceIP.To4(); v4 != nil {
		attrs = append(attrs, Attr{Type: AttrNASIPAddress, Value: v4})
	}

	pkt := &Packet{
		Code:          CodeAccessRequest,
		Authenticator: auth,
		Attrs:         attrs,
	}

	resp, err := a.client.SendToServers(ctx, pkt)
	if err != nil {
		// Infra failure (timeout / all servers unreachable): let the chain try
		// the next backend rather than lock the operator out (R-4).
		return nil, fmt.Errorf("radius: %w", err)
	}
	return resp, nil
}

// result turns a server's reply into the login's verdict. It is the whole
// answer for PAP and CHAP, and the concluding answer for EAP.
//
// RFC 3579 Section 2.6.3: "The NAS MUST make its access control decision based
// solely on the RADIUS Packet Type (Access-Accept/Access-Reject)", and "The
// access control decision MUST NOT be based on the contents of the EAP packet
// encapsulated in one or more EAP-Message attributes, if present." This
// function therefore reads resp.Code and never an EAP-Message: the EAP peer has
// already seen the encapsulated packet by the time the caller gets here, and
// what the peer made of it changes nothing decided below.
func (a *radiusAuthenticator) result(resp *Packet, username string) (aaa.AuthResult, error) {
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
		// Returning success with an empty set would escalate rather than restrict.
		// The result-scoped authorizer fails closed when no profile resolves. This
		// primary guard rejects the login before authorization runs. Historically,
		// this branch fell through to the built-in admin profile, so a server that
		// omitted Filter-Id handed every user admin.
		//
		// This is NOT the R-4 case above. There, SendToServers produced no answer at
		// all, so asking the next backend is right and locking the operator out on an
		// infra blip would be wrong. Here a reachable server answered: the profile set
		// resolving to zero is the CONTENT of that answer, not the absence of one.
		// Falling through would let a local account shadow the server's verdict, so
		// this rejects like Access-Reject does -- the chain stops.
		// RFC 2865 Section 5.6: "A NAS is not required to implement all of these
		// service types, and MUST treat unknown or unsupported Service-Types as
		// though an Access-Reject had been received instead."
		// RFC 2865 Section 1.1: "A NAS MUST treat a RADIUS access-accept
		// authorizing an unavailable service as an access-reject instead."
		//
		// Admin login is the one service this path provides, and its
		// Access-Request asks for Login-User. An Accept naming anything else
		// authorizes a service ze cannot give the operator, so it is a rejection
		// and the chain stops, exactly as for the empty profile set below.
		if !AcceptedServiceType(resp, serviceTypeLogin) {
			a.logger.Warn("RADIUS admin auth rejected: Access-Accept names an unsupported Service-Type",
				"username", username)
			return aaa.AuthResult{Source: aaaName}, aaa.ErrAuthRejected
		}

		profiles := a.mapProfiles(resp)
		if len(profiles) == 0 {
			a.logger.Warn("RADIUS admin auth rejected: no profiles resolved",
				"username", username)
			return aaa.AuthResult{Source: aaaName}, aaa.ErrAuthRejected
		}

		a.logger.Info("RADIUS admin auth accepted",
			"username", username, "profiles", profiles)
		return aaa.AuthResult{
			Authenticated: true,
			Profiles:      profiles,
			Source:        aaaName,
		}, nil
	case CodeAccessReject:
		a.logger.Info("RADIUS admin auth rejected", "username", username)
		return aaa.AuthResult{Source: aaaName}, aaa.ErrAuthRejected
	case CodeAccessChallenge:
		// RFC 2865 Section 4.4: "If the NAS does not support challenge/response,
		// it MUST treat an Access-Challenge as though it had received an
		// Access-Reject instead."
		//
		// PAP and CHAP send one Access-Request and have no path back to the
		// operator for a second one, so ze does not support challenge/response
		// for them. Returning a plain error would leave the challenge looking like
		// an infrastructure failure, and the chain would try TACACS+ and local
		// next.
		//
		// The two EAP methods DO support it, and authenticateEAP consumes every
		// challenge before it calls this function, so a challenge reaching here is
		// always a login that cannot answer one.
		a.logger.Info("RADIUS admin auth rejected: Access-Challenge and no challenge/response support",
			"username", username, "method", a.method.String())
		return aaa.AuthResult{Source: aaaName}, aaa.ErrAuthRejected
	default:
		return aaa.AuthResult{}, fmt.Errorf("radius: unexpected response code %d", resp.Code)
	}
}

// credential builds the one credential attribute set the Access-Request
// carries. RFC 2865 Section 4.1: "An Access-Request MUST contain either a
// User-Password or a CHAP-Password or a State.  An Access-Request MUST NOT
// contain both a User-Password and a CHAP-Password." This selects; it never
// appends, so no path can emit both.
//
// PAP puts the password here in cleartext, and the client XOR-hides it
// per-server (Section 5.2) inside Exchange, so it never reaches the wire in the
// clear. CHAP hashes it here and the password never leaves this process.
//
// A method the two constants do not name is refused rather than defaulted: an
// Access-Request carrying no credential violates Section 4.1, and one carrying
// a credential the operator did not choose is worse than a failed login, which
// the chain answers by trying the next backend.
func (a *radiusAuthenticator) credential(password string) ([]Attr, error) {
	switch a.method {
	case AuthMethodPAP:
		return []Attr{{Type: AttrUserPassword, Value: []byte(password)}}, nil
	case AuthMethodCHAP:
		return chapCredential(a.random, password)
	default:
		return nil, fmt.Errorf("radius: unknown auth method %d", uint8(a.method))
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
