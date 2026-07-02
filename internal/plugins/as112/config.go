// Design: plan/learned/1033-as112-2-dns-server.md -- as112 config parse + validation
// RFC: rfc/short/rfc7534.md -- address-family single-stack option (Section 3.4)

package as112

import (
	"encoding/json"
	"fmt"
	"net/netip"
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

// as112Config is the parsed, validated configuration.
type as112Config struct {
	Enabled       bool
	AddressFamily string
	Hostname      string
	Facility      string
	Location      string
	AllowFrom     []netip.Prefix
}

const configRootService = "service"

// parseConfig unmarshals the JSON config section and validates every field.
// It is the single source of truth for both the offline verifier and the
// engine's OnConfigure. An empty/missing service.as112 container yields a
// zero (disabled) config.
func parseConfig(data string) (as112Config, error) {
	cfg := as112Config{AddressFamily: addressFamilyBoth}

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
		cfg.Enabled = v == "true"
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
