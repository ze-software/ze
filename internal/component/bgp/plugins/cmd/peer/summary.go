// Design: docs/architecture/api/commands.md — BGP summary and capability handlers
// Overview: peer.go — BGP peer lifecycle and introspection handlers

package peer

import (
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:summary", Handler: handleBgpSummary},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-capabilities", Handler: handleBgpPeerCapabilities},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-statistics", Handler: handleBgpPeerStatistics},
	)
}

// maxFamilyArgLen caps the address-family argument echoed back in
// rejection messages so an unbounded operator string cannot be mirrored
// into the JSON response envelope.
const maxFamilyArgLen = 32

// familyArgRE constrains the address-family argument to the shape of
// an AFI/SAFI or a short form: lowercase letters, digits, slash,
// hyphen. Blocks shell meta, whitespace, and control chars from
// reaching the rejection message.
var familyArgRE = regexp.MustCompile(`^[a-z0-9/_-]+$`)

// handleBgpSummary returns a BGP summary table with per-peer
// statistics. Similar to FRR's "show bgp summary" — aggregate totals
// plus per-peer rows.
//
// With no arguments: every configured peer appears in the table.
//
// With one argument: the argument is an AFI/SAFI string (full form
// like "ipv4/unicast" / "l2vpn/evpn", or one of the shorthands
// `ipv4`, `ipv6`, `l2vpn` which expand to `ipv4/unicast`,
// `ipv6/unicast`, `l2vpn/evpn` respectively). Only peers that have
// completed RFC 4760 multiprotocol negotiation for the requested
// family appear in the table. Unknown or un-negotiated families
// reject with the sorted set of families any peer has actually
// negotiated, so the operator sees exactly what is reachable on the
// current daemon.
//
// Any other shorthand (e.g. `bgp-ls`, IPv4/VPN, labeled-unicast)
// requires the full `afi/safi` form — the shorthand table is
// deliberately small to avoid masking typos.
func handleBgpSummary(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Reactor() == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "reactor not available",
		}, fmt.Errorf("reactor not available")
	}

	var familyFilter string
	if len(args) > 0 {
		if err := validateFamilyArg(args[0]); err != nil {
			return &plugin.Response{Status: plugin.StatusError, Data: err.Error()}, nil //nolint:nilerr // operational error in Response
		}
		familyFilter = expandFamilyShorthand(args[0])
	}

	reactor := ctx.Reactor()
	allPeers := reactor.Peers()
	stats := reactor.Stats()

	// Single pass over allPeers: build the `peers[]` rows, count
	// established-in-filter, and collect the set of families any peer
	// has negotiated. The family set is only consumed when the filter
	// does not match any peer (rejectFamily); building it here avoids
	// a second iteration in the reject path.
	established := 0
	matched := false
	peerRows := make([]map[string]any, 0, len(allPeers))
	var seen map[string]struct{}
	if familyFilter != "" {
		seen = make(map[string]struct{})
	}
	for i := range allPeers {
		p := &allPeers[i]
		if familyFilter != "" {
			for _, f := range p.NegotiatedFamilies {
				seen[f] = struct{}{}
			}
			if !slices.Contains(p.NegotiatedFamilies, familyFilter) {
				continue
			}
			matched = true
		}
		if p.State == "established" {
			established++
		}
		peerRows = append(peerRows, map[string]any{
			"address":             p.Address.String(),
			"name":                p.Name,
			"description":         p.GroupName,
			"remote-as":           p.PeerAS,
			"state":               p.State,
			"uptime":              p.Uptime.String(),
			"updates-received":    p.UpdatesReceived,
			"updates-sent":        p.UpdatesSent,
			"keepalives-received": p.KeepalivesReceived,
			"keepalives-sent":     p.KeepalivesSent,
			"eor-received":        p.EORReceived,
			"eor-sent":            p.EORSent,
		})
	}
	if familyFilter != "" && !matched {
		return rejectFamily(familyFilter, seen)
	}

	// Convert uint32 router-id to dotted-quad IP string. Note: 0 renders
	// as 0.0.0.0 before the reactor has chosen a router-id; inherited
	// behavior, not a regression.
	rid := stats.RouterID
	routerID := netip.AddrFrom4([4]byte{byte(rid >> 24), byte(rid >> 16), byte(rid >> 8), byte(rid)}).String()

	summary := map[string]any{
		"router-id":         routerID,
		"local-as":          stats.LocalAS, // global BGP local AS, kept as "local-as" for summary context
		"uptime":            stats.Uptime.String(),
		"peers-configured":  len(allPeers),
		"peers-established": established,
		"peers":             peerRows,
	}
	if familyFilter != "" {
		summary["family"] = familyFilter
		summary["peers-in-family"] = len(peerRows)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: map[string]any{"summary": summary}}, nil
}

// validateFamilyArg caps length + charset on the operator-supplied
// address-family argument before it reaches any formatter or log.
//
// Ordering matters: the length check runs BEFORE strings.ToLower so a
// malicious caller cannot force a multi-megabyte allocation by passing
// a huge input (ToLower would otherwise allocate a full copy before we
// ever reach the charset check). Do not reorder.
func validateFamilyArg(in string) error {
	if in == "" {
		return fmt.Errorf("family argument is empty")
	}
	if len(in) > maxFamilyArgLen {
		return fmt.Errorf("family argument too long (%d > %d chars)", len(in), maxFamilyArgLen)
	}
	lowered := strings.ToLower(in)
	if !familyArgRE.MatchString(lowered) {
		return fmt.Errorf("invalid family %q: expected afi/safi (e.g. ipv4/unicast)", in)
	}
	return nil
}

// rejectFamily builds the exact-or-reject error naming the families
// actually negotiated on the current daemon so the operator sees the
// concrete valid set. `seen` is the set populated during the single
// handleBgpSummary pass over peers; callers never pass it nil when a
// filter was active.
func rejectFamily(wanted string, seen map[string]struct{}) (*plugin.Response, error) {
	known := make([]string, 0, len(seen))
	for f := range seen {
		known = append(known, f)
	}
	sort.Strings(known)
	msg := fmt.Sprintf("unknown or un-negotiated family %q", wanted)
	if len(known) == 0 {
		msg += "; no peer has completed negotiation"
	} else {
		msg += "; currently negotiated: " + strings.Join(known, ", ")
	}
	return &plugin.Response{Status: plugin.StatusError, Data: msg}, nil
}

// expandFamilyShorthand accepts three operator-friendly short forms:
//
//	"ipv4"  -> "ipv4/unicast"
//	"ipv6"  -> "ipv6/unicast"
//	"l2vpn" -> "l2vpn/evpn"
//
// The shorthand table is intentionally small; any other family
// (bgp-ls, flowspec, labeled-unicast, per-VRF SAFIs, etc.) requires
// the full afi/safi form so a typo like `bgplb` cannot be mis-expanded
// to a valid-looking family. Input is compared case-insensitive; the
// caller validates the returned string against actually-negotiated
// families.
func expandFamilyShorthand(in string) string {
	switch strings.ToLower(in) {
	case "ipv4":
		return "ipv4/unicast"
	case "ipv6":
		return "ipv6/unicast"
	case "l2vpn":
		return "l2vpn/evpn"
	}
	return in
}

// handleBgpPeerCapabilities returns negotiated capabilities for matched peers.
// If no OPEN exchange completed, returns negotiation-complete=false per peer.
// Single peer: flat object. Multiple peers: array of objects.
func handleBgpPeerCapabilities(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersBySelector(ctx)
	if errResp != nil {
		return errResp, err
	}

	if len(peers) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "no matching peers",
		}, fmt.Errorf("no matching peers")
	}

	reactor := ctx.Reactor()
	results := make([]map[string]any, len(peers))
	for i := range peers {
		peer := &peers[i]
		caps := reactor.PeerNegotiatedCapabilities(peer.Address)

		entry := map[string]any{
			"peer":  peer.Address.String(),
			"state": peer.State,
		}

		if caps != nil {
			entry["negotiation-complete"] = true
			neg := map[string]any{
				"families":               caps.Families,
				"extended-message":       caps.ExtendedMessage,
				"enhanced-route-refresh": caps.EnhancedRouteRefresh,
				"asn4":                   caps.ASN4,
			}
			if caps.AddPath != nil {
				neg["add-path"] = caps.AddPath
			}
			entry["negotiated"] = neg
		} else {
			entry["negotiation-complete"] = false
		}
		results[i] = entry
	}

	// Single peer: flat object. Multiple: array.
	var data any = results
	if len(results) == 1 {
		data = results[0]
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   data,
	}, nil
}

// handleBgpPeerStatistics returns per-peer update statistics with rates.
// Rate is computed from cumulative counters and uptime: counter / uptime_seconds.
// Returns 0 for all rates when uptime is zero (peer not established).
// Single peer: flat object. Multiple peers: array.
func handleBgpPeerStatistics(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersBySelector(ctx)
	if errResp != nil {
		return errResp, err
	}

	if len(peers) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "no matching peers",
		}, fmt.Errorf("no matching peers")
	}

	results := make([]map[string]any, len(peers))
	for i := range peers {
		p := &peers[i]
		uptimeSec := p.Uptime.Seconds()

		entry := map[string]any{
			"address":             p.Address.String(),
			"remote-as":           p.PeerAS,
			"state":               p.State,
			"uptime":              p.Uptime.String(),
			"updates-received":    p.UpdatesReceived,
			"updates-sent":        p.UpdatesSent,
			"keepalives-received": p.KeepalivesReceived,
			"keepalives-sent":     p.KeepalivesSent,
			"eor-received":        p.EORReceived,
			"eor-sent":            p.EORSent,
		}

		// Compute rates from cumulative counters / uptime.
		// Zero uptime (not established) → zero rates.
		if uptimeSec > 0 {
			entry["rate-updates-received"] = float64(p.UpdatesReceived) / uptimeSec
			entry["rate-updates-sent"] = float64(p.UpdatesSent) / uptimeSec
			entry["rate-keepalives-received"] = float64(p.KeepalivesReceived) / uptimeSec
			entry["rate-keepalives-sent"] = float64(p.KeepalivesSent) / uptimeSec
		} else {
			entry["rate-updates-received"] = 0.0
			entry["rate-updates-sent"] = 0.0
			entry["rate-keepalives-received"] = 0.0
			entry["rate-keepalives-sent"] = 0.0
		}

		results[i] = entry
	}

	// Single peer: flat object. Multiple: array.
	var data any = results
	if len(results) == 1 {
		data = results[0]
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   data,
	}, nil
}
