// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS admin AAA config
// Overview: aaa.go -- backend Build consuming this config
// RFC: rfc/short/rfc2865.md -- Filter-Id (Section 5.11), Class (Section 5.25)

// RADIUS admin-authentication configuration extraction from the YANG tree.
// Reads system/authentication/radius; separate from the L2TP subscriber
// RADIUS path (l2tp/plugins/authradius), which owns the l2tp config root.
package radius

import (
	"net"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/config"
)

const (
	defaultAdminPort = 1812
	defaultTimeout   = 3 * time.Second
	defaultRetries   = 3
	// attrClass is the RADIUS Class attribute (RFC 2865 Section 5.25). It is
	// defined here rather than in dict.go so the admin backend never has to
	// touch the shared dictionary used by the L2TP subscriber path.
	attrClass = 25
)

// ExtractedConfig holds RADIUS admin-auth configuration read from the tree.
type ExtractedConfig struct {
	Servers         []Server
	Timeout         time.Duration
	Retries         int
	SourceAddress   net.IP
	ProfileAttr     uint8    // Access-Accept reply attribute carrying profile names
	DefaultProfiles []string // profiles applied when the reply carries no ProfileAttr
}

// HasServers reports whether at least one RADIUS server is configured.
func (c *ExtractedConfig) HasServers() bool { return len(c.Servers) > 0 }

// ExtractConfig reads system/authentication/radius from the config tree.
// Safe with a nil tree (returns zero config). Defaults mirror the YANG
// defaults so an operator who omits a leaf gets the schema's advertised value.
func ExtractConfig(tree *config.Tree) ExtractedConfig {
	cfg := ExtractedConfig{
		Timeout:     defaultTimeout,
		Retries:     defaultRetries,
		ProfileAttr: AttrFilterID,
	}
	if tree == nil {
		return cfg
	}
	sys := tree.GetContainer("system")
	if sys == nil {
		return cfg
	}
	auth := sys.GetContainer("authentication")
	if auth == nil {
		return cfg
	}
	radiusTree := auth.GetContainer("radius")
	if radiusTree == nil {
		return cfg
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
		if v, ok := item.Value.Get("key"); ok {
			srv.SharedKey = []byte(v)
		}
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
	cfg.DefaultProfiles = radiusTree.GetSlice("default-profile")

	return cfg
}

// profileAttrType maps the YANG profile-attribute enum to a RADIUS attribute
// type. Unknown values fall back to Filter-Id (the schema default and the
// RFC 2865 Section 5.11 standard authorization carrier).
func profileAttrType(name string) uint8 {
	switch name {
	case "class":
		return attrClass
	default:
		return AttrFilterID
	}
}
