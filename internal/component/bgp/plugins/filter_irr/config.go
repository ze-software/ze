// Design: docs/architecture/bgp/filter-irr.md -- IRR filter config parsing from OnConfigure JSON

package filter_irr

import (
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/resolve/irr"
)

// pluginName is the registered plugin name (register.go). It is also the
// left-hand side of a filter chain reference: the engine routes a chain entry to
// a plugin by cutting the ref at the first ':' and dispatching on that prefix
// (internal/component/bgp/reactor/filter_chain.go PolicyFilterChain), so
// "bgp-filter-irr:65001" is the only spelling that can reach this plugin.
const pluginName = "bgp-filter-irr"

const (
	defaultServer          = "whois.radb.net"
	defaultPeeringDBURL    = "https://www.peeringdb.com"
	defaultRefreshInterval = uint32(3600)
	minRefreshInterval     = uint32(60)
	maxRefreshInterval     = uint32(86400)
)

type irrConfig struct {
	Server          string
	PeeringDBURL    string
	SourceAddress   string
	RefreshInterval uint32
	Peers           []peerIRRConfig
}

type peerIRRConfig struct {
	PeerAddr  string
	RemoteASN uint32
	ASSet     string
	Disabled  bool

	// UsesIRR reports whether this peer actually opted into IRR filtering,
	// either by naming a bgp-filter-irr entry in a filter chain that applies to
	// it (global, group, or peer level) or by setting an explicit
	// session.irr.as-set. Only such a peer is enrolled for resolution; see
	// handleConfigure. A peer that merely has a remote ASN has NOT asked for
	// IRR: its filter chain never reaches this plugin, so resolving it would be
	// an unsolicited whois/PeeringDB request on the operator's behalf.
	UsesIRR bool
}

func parseIRRConfig(bgpCfg map[string]any) *irrConfig {
	cfg := &irrConfig{
		Server:          defaultServer,
		PeeringDBURL:    defaultPeeringDBURL,
		RefreshInterval: defaultRefreshInterval,
	}

	if policyBlock, ok := bgpCfg["policy"].(map[string]any); ok {
		if irrBlock, ok := policyBlock["irr"].(map[string]any); ok {
			if srv, ok := irrBlock["server"].(string); ok && srv != "" {
				cfg.Server = srv
			}
			if pdbURL, ok := irrBlock["peeringdb-url"].(string); ok && pdbURL != "" {
				cfg.PeeringDBURL = pdbURL
			}
			if sa, ok := irrBlock["source-address"].(string); ok && sa != "" {
				cfg.SourceAddress = sa
			}
			if v, ok := readUint(irrBlock["refresh-interval"]); ok {
				cfg.RefreshInterval = clampRefreshInterval(v)
			}
		}
	}

	globalChained := chainReferencesIRR(bgpCfg)

	configjson.ForEachPeer(bgpCfg, func(peerAddr string, peerMap, groupMap map[string]any) {
		p := parsePeerIRR(peerAddr, peerMap)
		if p.RemoteASN == 0 {
			return
		}
		// A chain at any level that applies to this peer opts it in; so does an
		// explicit as-set, which is a direct request to resolve this peer's
		// prefixes (and keeps `show bgp irr check` usable as a dry run before
		// the chain is wired).
		p.UsesIRR = globalChained ||
			chainReferencesIRR(groupMap) ||
			chainReferencesIRR(peerMap) ||
			p.ASSet != ""
		cfg.Peers = append(cfg.Peers, p)
	})

	return cfg
}

// chainReferencesIRR reports whether the import or export filter chain at one
// config level (bgp / group / peer) names this plugin. A chain entry is
// "<plugin>:<filter>"; the engine cuts at the first ':' and dispatches on the
// prefix (reactor/filter_chain.go PolicyFilterChain), so only that spelling can
// invoke us -- a bare name would dispatch to a plugin of that name instead.
// Refs arrive clean via ToMap; per-member deactivation is out-of-band, so there
// is no "inactive:" prefix to strip (same as filter_family's chain reader).
func chainReferencesIRR(m map[string]any) bool {
	if m == nil {
		return false
	}
	filterBlock, ok := m["filter"].(map[string]any)
	if !ok {
		return false
	}
	for _, key := range [...]string{"import", "export"} {
		for _, ref := range toStringList(filterBlock[key]) {
			if plugin, _, found := strings.Cut(ref, ":"); found && plugin == pluginName {
				return true
			}
		}
	}
	return false
}

// toStringList normalises a leaf-list value to []string. The config loader may
// pass []any (JSON round-trip), []string (multi-value), or a bare string.
func toStringList(v any) []string {
	switch s := v.(type) {
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	case []string:
		return s
	case string:
		if s == "" {
			return nil
		}
		return []string{s}
	}
	return nil
}

func parsePeerIRR(peerAddr string, peerMap map[string]any) peerIRRConfig {
	p := peerIRRConfig{PeerAddr: peerAddr}
	if peerMap == nil {
		return p
	}

	session, ok := peerMap["session"].(map[string]any)
	if !ok {
		return p
	}

	if asnBlock, ok := session["asn"].(map[string]any); ok {
		if v, ok := readUint(asnBlock["remote"]); ok {
			p.RemoteASN = uint32(v) //nolint:gosec // clamped by readUint uint64
		}
	}

	if irrBlock, ok := session["irr"].(map[string]any); ok {
		if asSet, ok := irrBlock["as-set"].(string); ok && asSet != "" {
			if irr.ValidateASSetName(asSet) == nil {
				p.ASSet = asSet
			} else {
				logger().Warn("irr: ignoring invalid as-set in config", "peer", peerAddr, "as-set", asSet)
			}
		}
		if enable, ok := irrBlock["enable"].(string); ok && enable == "disable" {
			p.Disabled = true
		}
	}

	return p
}

func clampRefreshInterval(v uint64) uint32 {
	if v < uint64(minRefreshInterval) {
		return minRefreshInterval
	}
	if v > uint64(maxRefreshInterval) {
		return maxRefreshInterval
	}
	return uint32(v) //nolint:gosec // range checked above
}

func readUint(v any) (uint64, bool) {
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true //nolint:gosec // range checked above
	case uint64:
		return n, true
	case string:
		x, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			return 0, false
		}
		return x, true
	default:
		return 0, false
	}
}
