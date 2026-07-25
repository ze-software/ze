// Design: plan/learned/896-filter-irr.md -- IRR filter config parsing from OnConfigure JSON

package filter_irr

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/resolve/irr"
)

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

	configjson.ForEachPeer(bgpCfg, func(peerAddr string, peerMap, groupMap map[string]any) {
		p := parsePeerIRR(peerAddr, peerMap)
		if p.RemoteASN == 0 {
			return
		}
		cfg.Peers = append(cfg.Peers, p)
	})

	return cfg
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
