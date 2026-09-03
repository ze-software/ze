// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS admin AAA config
// Overview: aaa.go -- backend Build consuming this config
// RFC: rfc/short/rfc2865.md -- Filter-Id (Section 5.11)

// RADIUS admin-authentication configuration extraction from the YANG tree.
// Reads system/authentication/radius; separate from the L2TP subscriber
// RADIUS path (l2tp/plugins/authradius), which owns the l2tp config root.
package radius

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/config"
)

const (
	defaultAdminPort = 1812
	defaultTimeout   = 3 * time.Second
	defaultRetries   = 3
)

// ExtractedConfig holds RADIUS admin-auth configuration read from the tree.
type ExtractedConfig struct {
	Servers         []Server
	Timeout         time.Duration
	Retries         int
	SourceAddress   net.IP
	ProfileAttr     uint8    // Access-Accept reply attribute carrying profile names
	DefaultProfiles []string // profiles applied when the reply carries no ProfileAttr
	AuthMethod      AuthMethod
}

// HasServers reports whether at least one RADIUS server is configured.
func (c *ExtractedConfig) HasServers() bool { return len(c.Servers) > 0 }

// ExtractConfig reads system/authentication/radius from the config tree.
// Safe with a nil tree (returns zero config). Defaults mirror the YANG
// defaults so an operator who omits a leaf gets the schema's advertised value.
//
// It returns an error, and no config, when a server row carries no shared
// secret. RFC 2865 forbids the empty secret outright, so there is no partial
// answer to give: a Server built from such a row would sign nothing, and a
// caller cannot tell that Server from a good one.
func ExtractConfig(tree *config.Tree) (ExtractedConfig, error) {
	cfg := ExtractedConfig{
		Timeout:     defaultTimeout,
		Retries:     defaultRetries,
		ProfileAttr: AttrFilterID,
	}
	if tree == nil {
		return cfg, nil
	}
	sys := tree.GetContainer("system")
	if sys == nil {
		return cfg, nil
	}
	auth := sys.GetContainer("authentication")
	if auth == nil {
		return cfg, nil
	}
	radiusTree := auth.GetContainer("radius")
	if radiusTree == nil {
		return cfg, nil
	}

	// GetListOrdered preserves configured failover order (YANG ordered-by user).
	for _, item := range radiusTree.GetListOrdered("server") {
		port := uint16(defaultAdminPort)
		if v, ok := item.Value.Get("port"); ok {
			if n, err := strconv.ParseUint(v, 10, 16); err == nil {
				port = uint16(n)
			}
		}
		// net.JoinHostPort produces a host:port the client's net.ResolveUDPAddr
		// can split. NOTE: the shared radius.Client binds and resolves with
		// "udp4" (client.go), so an IPv6 server literal will fail to resolve and
		// read as unreachable (fail-safe: login falls through to local, and
		// doctor-radius-admin-unreachable flags it). RADIUS admin auth is IPv4
		// only; see the YANG address leaf and docs/guide/radius.md.
		srv := Server{Address: net.JoinHostPort(item.Key, strconv.Itoa(int(port)))}
		v, ok := item.Value.Get("key")
		// RFC 2865 Section 3: "The secret MUST NOT be empty (length 0) since this
		// would allow packets to be trivially forged."
		//
		// The YANG `key` leaf carries length "1..max", so a config that reaches
		// here with an empty secret already failed schema validation. This is the
		// paired check on the other side of that boundary, and it fails the whole
		// extraction rather than dropping one row: a silently shorter server list
		// reads to the caller exactly like a correct one.
		if !ok || v == "" {
			return ExtractedConfig{}, fmt.Errorf(
				"radius: server %s has no shared secret; RFC 2865 Section 3 forbids an empty secret", item.Key)
		}
		srv.SharedKey = []byte(v)
		cfg.Servers = append(cfg.Servers, srv)
	}

	if v, ok := radiusTree.Get("timeout"); ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil && n > 0 {
			cfg.Timeout = time.Duration(n) * time.Second
		}
	}
	if v, ok := radiusTree.Get("retries"); ok {
		if n, err := strconv.ParseUint(v, 10, 8); err == nil {
			cfg.Retries = int(n)
		}
	}
	if v, ok := radiusTree.Get("source-address"); ok && v != "" {
		cfg.SourceAddress = net.ParseIP(v)
	}
	if v, ok := radiusTree.Get("profile-attribute"); ok {
		cfg.ProfileAttr = profileAttrType(v)
	}
	// An empty value is an absent leaf, exactly as source-address reads it
	// above, and it leaves the PAP default in place.
	if v, ok := radiusTree.Get("auth-method"); ok && v != "" {
		method, err := parseAuthMethod(v)
		if err != nil {
			return ExtractedConfig{}, err
		}
		cfg.AuthMethod = method
	}
	cfg.DefaultProfiles = radiusTree.GetSlice("default-profile")

	return cfg, nil
}

// profileAttrType maps the YANG profile-attribute enum to a RADIUS attribute
// type. Filter-Id is the only enum value and the only conformant carrier, so an
// unknown value lands there too.
//
// RFC 2865 Section 5.25 rules Class out: "The client MUST NOT interpret the
// attribute locally." Section 5.11 gives Filter-Id the opposite job, naming "the
// filter list for this user", which is what a ze authorization profile is.
func profileAttrType(_ string) uint8 {
	return AttrFilterID
}

// AuthMethod names the credential an Access-Request carries for an operator
// login. RFC 2865 Section 4.1: "An Access-Request MUST contain either a
// User-Password or a CHAP-Password or a State.  An Access-Request MUST NOT
// contain both a User-Password and a CHAP-Password." The two values are
// therefore exclusive, and the authenticator builds one of them.
//
// The zero value is AuthMethodPAP, which is the YANG default and the behavior
// ze shipped before the auth-method leaf existed.
type AuthMethod uint8

const (
	// AuthMethodPAP sends User-Password, hidden per RFC 2865 Section 5.2.
	AuthMethodPAP AuthMethod = iota
	// AuthMethodCHAP sends CHAP-Password and CHAP-Challenge, RFC 2865
	// Sections 5.3 and 5.40.
	AuthMethodCHAP
)

// String names the method as the YANG enum spells it.
func (m AuthMethod) String() string {
	if m == AuthMethodCHAP {
		return "chap"
	}
	return "pap"
}

// parseAuthMethod maps the YANG auth-method enum to a typed method.
//
// An unknown word is refused rather than defaulted. The YANG enumeration
// already rejects it at config load, so this is the paired check on the other
// side of that boundary: a config reaching here with a word the schema does not
// define means the two disagree, and picking a credential for the operator
// would send one he did not choose. ExtractConfig returns the error, Build logs
// it and contributes nothing, and login falls through to the local backend.
func parseAuthMethod(s string) (AuthMethod, error) {
	switch s {
	case "pap":
		return AuthMethodPAP, nil
	case "chap":
		return AuthMethodCHAP, nil
	default:
		return AuthMethodPAP, fmt.Errorf(
			"radius: auth-method %q is not pap or chap", s)
	}
}
