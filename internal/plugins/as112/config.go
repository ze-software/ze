// Design: docs/architecture/dns/as112.md -- as112 config parse + validation
// RFC: rfc/short/rfc7534.md -- address-family single-stack option (Section 3.4)

package as112

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/dnsserver"
)

// maxHostnameLen is the DNS label length limit (spec Boundary Tests table).
// maxFacilityLen/maxLocationLen are chosen so the combined "facility, location"
// TXT string (finding M3) stays well under the 255-byte single-TXT-string wire
// limit and the assembled response stays under 512 octets -- proven by
// TestHostnameTXT_TotalResponseUnder512, not by this arithmetic alone.
const (
	maxHostnameLen = 63
	maxFacilityLen = 100
	maxLocationLen = 100
)

const (
	addressFamilyBoth     = "both"
	addressFamilyIPv4Only = "ipv4-only"
	addressFamilyIPv6Only = "ipv6-only"
)

var validAddressFamilies = map[string]bool{
	addressFamilyBoth:     true,
	addressFamilyIPv4Only: true,
	addressFamilyIPv6Only: true,
}

// as112DefaultASN is the origin AS a redistributed AS112 covering prefix carries
// when the operator does not set `asn`: the well-known AS112 number (RFC 7534
// Section 3.2). The redistribute source models an AS112 virtual router, so 112
// is the natural default; an operator or private ASN is an explicit override.
const as112DefaultASN uint32 = 112

// configValueTrue is the canonical boolean-true spelling in config leaf values.
const configValueTrue = "true"

// maxCommunities bounds the community leaf-list so the COMMUNITIES attribute on
// the covering-prefix UPDATE cannot grow past the BGP message-size limit.
// Mirrored by `max-elements 32` on the community leaf-list in ze-as112-conf.yang;
// enforced here because the config validator does not enforce leaf-list
// cardinality for this leaf. TestMaxCommunitiesMatchesYANG guards the two against
// drift, so change both together.
const maxCommunities = 32

// as112Config is the parsed, validated configuration.
type as112Config struct {
	Enabled       bool
	AddressFamily string
	Hostname      string
	Facility      string
	Location      string
	AllowFrom     []netip.Prefix
	// ASN is the origin AS the redistributed covering prefixes carry as a
	// single-ASN AS_PATH (default as112DefaultASN). Used only by the
	// redistribute producer, never by the DNS server.
	ASN uint32
	// Community is the optional standard BGP community list (each packed
	// asn<<16|value) the redistributed covering prefixes carry. Parsed via
	// attribute.ParseCommunity so well-known names (nopeer/no-export, RFC
	// 1997/3765) as well as AA:NN are accepted.
	Community []uint32
	// Watchdog gates announcement on serving state: when true (default) the
	// covering prefixes are announced only while the DNS node is serving (RFC
	// 7534 Section 3.3); false announces as soon as enabled + imported.
	Watchdog bool
	// Secure holds the optional DNS-over-TLS (RFC 7858) and DNS-over-HTTPS (RFC
	// 8484) listener configuration; both bind the anycast addresses on their own
	// ports and share the tls cert material.
	Secure dnsserver.SecureConfig
}

const configRootService = "service"

// parseConfig unmarshals the JSON config section and validates every field.
// It is the single source of truth for both the offline verifier and the
// engine's OnConfigure. An empty/missing service.as112 container yields a
// zero (disabled) config.
func parseConfig(data string) (as112Config, error) {
	cfg := as112Config{AddressFamily: addressFamilyBoth, ASN: as112DefaultASN, Watchdog: true, Secure: dnsserver.DefaultSecureConfig()}

	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return cfg, fmt.Errorf("as112 config: unmarshal: %w", err)
	}
	svc, ok := asMap(root, configRootService)
	if !ok {
		return cfg, nil
	}
	a, ok := asMap(svc, "as112")
	if !ok {
		return cfg, nil
	}

	if v, ok := asString(a, "enabled"); ok {
		cfg.Enabled = v == configValueTrue
	}

	if v, ok := asString(a, "address-family"); ok {
		if !validAddressFamilies[v] {
			return cfg, fmt.Errorf("as112: address-family %q invalid (both|ipv4-only|ipv6-only)", v)
		}
		cfg.AddressFamily = v
	}

	if v, ok := asString(a, "hostname"); ok {
		if len(v) > maxHostnameLen {
			return cfg, fmt.Errorf("as112: hostname %d octets, max %d", len(v), maxHostnameLen)
		}
		cfg.Hostname = v
	}

	if v, ok := asString(a, "facility"); ok {
		if len(v) > maxFacilityLen {
			return cfg, fmt.Errorf("as112: facility %d octets, max %d", len(v), maxFacilityLen)
		}
		cfg.Facility = v
	}

	if v, ok := asString(a, "location"); ok {
		if len(v) > maxLocationLen {
			return cfg, fmt.Errorf("as112: location %d octets, max %d", len(v), maxLocationLen)
		}
		cfg.Location = v
	}

	for _, s := range asStringList(a, "allow-from") {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return cfg, fmt.Errorf("as112: allow-from %q invalid: %w", s, err)
		}
		cfg.AllowFrom = append(cfg.AllowFrom, p.Masked())
	}

	// asn: origin AS for the redistribute path. The default (as112DefaultASN) is
	// applied above; a present value must be a valid 4-byte ASN (0 is reserved).
	if v, ok := asString(a, "asn"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil || n == 0 {
			return cfg, fmt.Errorf("as112: asn %q invalid (expected 1..4294967295)", v)
		}
		cfg.ASN = uint32(n) //nolint:gosec // G115: bounded by ParseUint bitSize=32
	}

	// community: optional BGP communities for the redistribute path. The
	// canonical parser accepts well-known names (nopeer/no-export) and AA:NN
	// alike (RFC 1997/3765); config time is where a malformed value is rejected,
	// not emit time.
	comms := asStringList(a, "community")
	if len(comms) > maxCommunities {
		return cfg, fmt.Errorf("as112: %d communities exceeds max %d", len(comms), maxCommunities)
	}
	for _, s := range comms {
		c, err := attribute.ParseCommunity(s)
		if err != nil {
			return cfg, fmt.Errorf("as112: community %q invalid: %w", s, err)
		}
		cfg.Community = append(cfg.Community, c)
	}

	// watchdog: health gate. The default (true) is applied above; mirror the
	// enabled leaf's boolean parse (YANG `type boolean` rejects non-true/false
	// values upstream).
	if v, ok := asString(a, "watchdog"); ok {
		cfg.Watchdog = v == configValueTrue
	}

	// tls (DoT) + doh (DoH) listener config: shared parse, native-mirror port
	// validation. Defaults were seeded above via DefaultSecureConfig.
	if err := dnsserver.ParseSecureLeaves(a, &cfg.Secure, "as112"); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func asMap(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key].(map[string]any)
	return v, ok
}

func asString(m map[string]any, key string) (string, bool) {
	v, ok := m[key].(string)
	return v, ok
}

// asStringList reads a leaf-list, which arrives as a JSON array of strings or
// a single string.
func asStringList(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}
