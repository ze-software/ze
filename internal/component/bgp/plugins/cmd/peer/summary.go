// Design: docs/architecture/api/commands.md — BGP summary and capability handlers
// RFC: rfc/short/rfc4271.md — NOTIFICATION code/subcode rendered as last-error (Section 4.5)
// Overview: peer.go — BGP peer lifecycle and introspection handlers

package peer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errFamilyArgumentIsEmpty = errors.New("family argument is empty")
	errNoMatchingPeers       = errors.New("no matching peers")
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:summary", Handler: handleBgpSummary},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-capabilities", Handler: handleBgpPeerCapabilities},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:peer-statistics", Handler: handleBgpPeerStatistics},
	)
}

// lastErrorString renders a peer's last NOTIFICATION for the "last-error"
// field. Returns "" when the peer has never sent or received one, so a healthy
// peer reports no error rather than a fabricated "none".
//
// RFC 4271 Section 4.5: "Error code / Error subcode". The rendering goes through
// message.Notification, whose NotifyErrorCode.String() maps unknown codes to
// "Unknown(N)" and unknown subcodes to "Subcode(N)" instead of echoing them, and
// which emits no Data bytes here because PeerInfo does not carry them. That
// bounding is load-bearing: last-error reaches the PUBLIC looking glass
// (lg/server.go query path), and code/subcode originate from a remote peer.
func lastErrorString(p *plugin.PeerInfo) string {
	if p.LastNotifTime.IsZero() {
		return ""
	}
	n := message.Notification{
		ErrorCode:    message.NotifyErrorCode(p.LastNotifCode),
		ErrorSubcode: p.LastNotifSubcode,
	}
	return n.String()
}

// stateChangedString renders a peer's most recent FSM transition time as
// RFC3339. Returns "" for a peer that has never transitioned, so consumers show
// a blank rather than the zero epoch ("0001-01-01T00:00:00Z").
func stateChangedString(p *plugin.PeerInfo) string {
	if p.LastStateChange.IsZero() {
		return ""
	}
	return p.LastStateChange.Format(time.RFC3339)
}

// cmdRibStatus is the bgp-rib plugin's status command. Kept as a string
// constant, not an import: cmd/peer reaches the RIB plugin's per-peer route
// counts by runtime dispatch (ForwardToPlugin), preserving plugin
// self-containment, exactly as cmd/rib does. See ai/rules/plugin-self-containment.md.
const cmdRibStatus = "show bgp rib status"

// ribRouteCount is a peer's Adj-RIB-In (in) and Adj-RIB-Out (out) size.
type ribRouteCount struct {
	in  int
	out int
}

// fetchRibRouteCounts asks the bgp-rib plugin for its per-peer route counts.
// famFilter (the summary's expanded afi/safi filter, or "") scopes the counts to
// one family so a family-filtered summary reports family-scoped, not all-family,
// counts. Best-effort by design: returns nil when the plugin is not loaded
// (ForwardToPlugin -> ErrUnknownCommand) or on any error, so the summary still
// renders — the route-count keys are omitted, never faked to 0.
func fetchRibRouteCounts(ctx *pluginserver.CommandContext, famFilter string) map[string]ribRouteCount {
	d := ctx.Dispatcher()
	if d == nil {
		return nil
	}
	var args []string
	if famFilter != "" {
		args = []string{famFilter}
	}
	resp, err := d.ForwardToPlugin(ctx, cmdRibStatus, args, "")
	if err != nil || resp == nil || resp.Status != plugin.StatusDone {
		return nil
	}
	raw, ok := resp.Data.(plugin.RawJSON)
	if !ok {
		return nil
	}
	return parseRibRouteCounts([]byte(raw))
}

// parseRibRouteCounts extracts the per-peer `route-counts` map from a
// `show bgp rib status` JSON response (produced by RIBManager.status). Returns
// nil on absence or malformed input; a nil map merges as "no counts".
func parseRibRouteCounts(raw []byte) map[string]ribRouteCount {
	if len(raw) == 0 {
		return nil
	}
	var payload struct {
		RouteCounts map[string]struct {
			In  int `json:"in"`
			Out int `json:"out"`
		} `json:"route-counts"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	if len(payload.RouteCounts) == 0 {
		return nil
	}
	counts := make(map[string]ribRouteCount, len(payload.RouteCounts))
	for addr, c := range payload.RouteCounts {
		counts[addr] = ribRouteCount{in: c.In, out: c.Out}
	}
	return counts
}

// mergeRibRouteCounts adds the birdwatcher route-count keys to a peer row when
// the RIB reported counts for that address. routes-received and routes-accepted
// are both the Adj-RIB-In size: Ze retains only accepted routes (rejects are
// dropped at the reactor gate and never stored), so there is no separate
// pre-policy received count here. routes-filtered is deliberately never emitted
// — Ze does not retain filtered routes (ai/rules/project-knowledge.md).
// When counts is nil (RIB absent) the keys are omitted rather than faked to 0.
func mergeRibRouteCounts(row map[string]any, addr string, counts map[string]ribRouteCount) {
	c, ok := counts[addr]
	if !ok {
		return
	}
	row["routes-received"] = c.in
	row["routes-accepted"] = c.in
	row["routes-sent"] = c.out
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
			Error:  "reactor not available",
		}, errReactorNotAvailable
	}

	var familyFilter string
	if len(args) > 0 {
		if err := validateFamilyArg(args[0]); err != nil {
			return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response
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
	var famFilter family.Family
	if familyFilter != "" {
		seen = make(map[string]struct{})
		famFilter, _ = family.LookupFamily(familyFilter)
	}
	// Per-peer route counts owned by the bgp-rib plugin (Adj-RIB-In/Out sizes).
	// Best-effort: nil when the plugin is absent, in which case the route-count
	// keys are simply omitted (never faked to 0).
	ribCounts := fetchRibRouteCounts(ctx, familyFilter)
	for i := range allPeers {
		p := &allPeers[i]
		if familyFilter != "" {
			for _, f := range p.NegotiatedFamilies {
				seen[f.String()] = struct{}{}
			}
			if !slices.Contains(p.NegotiatedFamilies, famFilter) {
				continue
			}
			matched = true
		}
		if p.State == plugin.PeerStateEstablished {
			established++
		}
		row := map[string]any{
			"address":             p.Address.String(),
			"name":                p.Name,
			"description":         p.GroupName,
			"remote-as":           p.PeerAS,
			"peer-type":           p.PeerType,
			"state":               p.State.String(),
			"state-changed":       stateChangedString(p),
			"last-error":          lastErrorString(p),
			"uptime":              p.Uptime.Truncate(time.Second).String(),
			"updates-received":    p.UpdatesReceived,
			"updates-sent":        p.UpdatesSent,
			"keepalives-received": p.KeepalivesReceived,
			"keepalives-sent":     p.KeepalivesSent,
			"eor-received":        p.EORReceived,
			"eor-sent":            p.EORSent,
			"connections-dropped": p.ConnectionsDropped,
		}
		mergeRibRouteCounts(row, p.Address.String(), ribCounts)
		peerRows = append(peerRows, row)
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
		"uptime":            stats.Uptime.Truncate(time.Second).String(),
		"peers-configured":  len(allPeers),
		"peers-established": established,
		"peers":             peerRows,
	}
	if familyFilter != "" {
		summary["family"] = familyFilter
		summary["peers-in-family"] = len(peerRows)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"summary": summary}}, nil
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
		return errFamilyArgumentIsEmpty
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
		msg += "; currently negotiated: " + textbuf.Join(known, ", ")
	}
	return &plugin.Response{Status: plugin.StatusError, Error: msg}, nil
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
func handleBgpPeerCapabilities(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersByArgs(ctx, args)
	if errResp != nil {
		return errResp, err
	}

	if len(peers) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "no matching peers",
		}, errNoMatchingPeers
	}

	reactor := ctx.Reactor()
	results := make([]map[string]any, len(peers))
	for i := range peers {
		peer := &peers[i]
		caps := reactor.PeerNegotiatedCapabilities(peer.Address)

		entry := map[string]any{
			"peer":  peer.Address.String(),
			"state": peer.State.String(),
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

	if len(results) == 1 {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(results[0])}, nil
	}
	typed := make(plugin.Slice[plugin.Map], len(results))
	for i, r := range results {
		typed[i] = plugin.Map(r)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: typed}, nil
}

// handleBgpPeerStatistics returns per-peer update statistics with rates.
// Rate is computed from cumulative counters and uptime: counter / uptime_seconds.
// Returns 0 for all rates when uptime is zero (peer not established).
// Single peer: flat object. Multiple peers: array.
func handleBgpPeerStatistics(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	peers, errResp, err := filterPeersByArgs(ctx, args)
	if errResp != nil {
		return errResp, err
	}

	if len(peers) == 0 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "no matching peers",
		}, errNoMatchingPeers
	}

	results := make([]map[string]any, len(peers))
	for i := range peers {
		p := &peers[i]
		uptimeSec := p.Uptime.Seconds()

		entry := map[string]any{
			"address":             p.Address.String(),
			"remote-as":           p.PeerAS,
			"state":               p.State.String(),
			"uptime":              p.Uptime.Truncate(time.Second).String(),
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

	if len(results) == 1 {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(results[0])}, nil
	}
	typed := make(plugin.Slice[plugin.Map], len(results))
	for i, r := range results {
		typed[i] = plugin.Map(r)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: typed}, nil
}
