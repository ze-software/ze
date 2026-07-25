// Design: plan/learned/924-isis-12-ipv6.md -- IPv6 redistribution (both directions).
// Related: consumer.go -- the IPv4 consumer this extends to AFI=2.
// Related: source.go -- the IPv4 producer this extends to AFI=2.
//
// RFC: rfc/short/rfc5308.md sec 2 -- TLV 236 IPv6 Reachability; the external (X)
//   bit SHALL be set when the prefix was distributed into IS-IS from another
//   protocol (so a redistributed IPv6 route originates with X set); link-local
//   prefixes MUST NOT be advertised.
// RFC: rfc/short/rfc2966.md -- up/down bit CLEAR on first injection (set only on
//   a down-level leak, done by SPF, not here).
//
// This file is the IPv6 half of IS-IS redistribution. The consumer (InjectRoute /
// WithdrawRoute in consumer.go) dispatches AFI=2 here, turning connected/static/
// BGP IPv6 imports into TLV 236 entries in the own LSP set. The source emits an
// IPv6 redistevents batch (AFI=2) from the IPv6 SPF delta so IS-IS IPv6 routes
// reach the BGP IPv6-unicast consumer. Both are the redistribution path only,
// separate from the FIB install (the SPF IPv6 Installer's Loc-RIB path).

package isisredistribute

import (
	"context"
	"log/slog"
	"net/netip"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// injectRouteV6 originates a TLV 236 reachability entry for entry.Prefix in the
// node's own LSP set at every origination level, then re-originates. A
// link-local or malformed prefix is rejected without mutating state (security
// review: input validation; RFC 5308 sec 2: no link-local in TLV 236). The
// external (X) bit is set: the prefix is distributed into IS-IS from another
// protocol (RFC 5308 sec 2). The up/down bit is CLEAR on first injection (set
// only on a down-level leak, RFC 2966).
func (c *Consumer) injectRouteV6(_ context.Context, _ family.Family, entry configredist.RouteEntry) {
	src := sourceLabel(entry.Source)
	prefix, err := netip.ParsePrefix(entry.Prefix)
	if err != nil || !prefix.IsValid() || !prefix.Addr().Is6() || prefix.Addr().Is4In6() {
		slog.Warn("isis redist consumer: rejecting malformed IPv6 prefix",
			"prefix", entry.Prefix, "source", src, "error", err)
		return
	}
	prefix = prefix.Masked()
	if prefix.Addr().IsLinkLocalUnicast() {
		// RFC 5308 sec 2: link-local prefixes MUST NOT be advertised in TLV 236.
		slog.Debug("isis redist consumer: skipping link-local IPv6 prefix (RFC 5308 sec 2)",
			"prefix", prefix.String(), "source", src)
		return
	}

	info := lsdb.PrefixInfoV6{
		Prefix:   prefix,
		Metric:   types.NewPrefixMetric(DefaultRedistMetric),
		UpDown:   false, // RFC 2966: clear on injection
		External: true,  // RFC 5308 sec 2: distributed from another protocol
	}
	for _, level := range c.inj.OriginationLevels() {
		c.inj.SetRedistPrefixV6(level, info)
	}
	c.rememberSource(prefix, src)
	c.mInjected.With(src, afiIPv6Label).Inc()

	if err := c.inj.Originate(); err != nil {
		slog.Warn("isis redist consumer: re-origination failed after IPv6 inject",
			"prefix", prefix.String(), "source", src, "error", err)
		c.mInjectFailed.With(src).Inc()
	}
}

// withdrawRouteV6 removes the TLV 236 entry for prefix from every origination
// level and re-originates so peers withdraw the IPv6 route (AC-6). A prefix that
// was never injected is a no-op.
func (c *Consumer) withdrawRouteV6(_ context.Context, _ family.Family, prefixStr string) {
	prefix, err := netip.ParsePrefix(prefixStr)
	if err != nil || !prefix.IsValid() || !prefix.Addr().Is6() || prefix.Addr().Is4In6() {
		slog.Warn("isis redist consumer: rejecting malformed IPv6 prefix on withdraw",
			"prefix", prefixStr, "error", err)
		return
	}
	prefix = prefix.Masked()

	removed := false
	for _, level := range c.inj.OriginationLevels() {
		if c.inj.RemoveRedistPrefixV6(level, prefix) {
			removed = true
		}
	}
	// Recover the source recorded at inject time (see WithdrawRoute) so the
	// withdrawn/failure metrics carry the right label instead of always "unknown".
	src := c.forgetSource(prefix)
	if !removed {
		return // never injected; nothing to re-originate
	}
	c.mWithdrawn.With(src, afiIPv6Label).Inc()

	if err := c.inj.Originate(); err != nil {
		slog.Warn("isis redist consumer: re-origination failed after IPv6 withdraw",
			"prefix", prefix.String(), "source", src, "error", err)
		c.mInjectFailed.With(src).Inc()
	}
}

// ConnectedPrefixInfosV6 turns a list of NON-LINK-LOCAL IPv6 interface prefixes
// into TLV 236 PrefixInfoV6 internal-reachability entries advertised at the
// node's own metric (AC-6). The prefix is masked; the up/down and external bits
// are 0 (internal reachability, not leaked, not redistributed). The IPv6 twin of
// ConnectedPrefixInfos. Link-local prefixes are dropped defensively (RFC 5308
// sec 2), though the caller already excludes them.
func ConnectedPrefixInfosV6(prefixes []netip.Prefix, metric uint32) []lsdb.PrefixInfoV6 {
	out := make([]lsdb.PrefixInfoV6, 0, len(prefixes))
	for _, p := range prefixes {
		if !p.IsValid() || !p.Addr().Is6() || p.Addr().Is4In6() {
			continue
		}
		if p.Addr().IsLinkLocalUnicast() {
			continue // RFC 5308 sec 2
		}
		out = append(out, lsdb.PrefixInfoV6{
			Prefix: p.Masked(),
			Metric: types.NewPrefixMetric(metric),
		})
	}
	return out
}

// OnSPFChangeV6 is the SPF Computer OnChangeV6 callback (isis-12): it turns the
// IPv6 route delta into a single redistevents batch at AFI=2 and emits it. It is
// the REDISTRIBUTION path only -- the FIB install is the Computer's own IPv6
// Installer (Loc-RIB), a separate path. Mirrors OnSPFChange for IPv4.
func (s *Source) OnSPFChangeV6(delta spf.RouteDelta) {
	emitDeltaFamily(delta, isisProtocolIDV6(), family.AFIIPv6, s.send)
}

// isisProtocolIDV6 returns the IS-IS redistevents identity. IS-IS uses a SINGLE
// "isis" protocol/source for both address families (umbrella "Redistribution
// source"): the batch's AFI field, not the ProtocolID, distinguishes the family.
// This accessor exists so the IPv6 emit reads the same identity explicitly.
func isisProtocolIDV6() redistevents.ProtocolID { return spf.ProtocolID() }

// emitDeltaFamily is emitDelta generalized over the address family (isis-12). It
// converts an SPF RouteDelta into one redistevents.RouteChangeBatch with the
// given AFI (unicast SAFI) and hands it to sink. Both IPv4 and IPv6 use it.
func emitDeltaFamily(delta spf.RouteDelta, protocol redistevents.ProtocolID, afi family.AFI, sink func(*redistevents.RouteChangeBatch)) {
	if delta.Empty() {
		return
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = protocol
	b.AFI = uint16(afi)
	b.SAFI = uint8(family.SAFIUnicast)

	for i := range delta.Added {
		b.Entries = append(b.Entries, addEntry(&delta.Added[i]))
	}
	for i := range delta.Changed {
		b.Entries = append(b.Entries, addEntry(&delta.Changed[i]))
	}
	for _, pfx := range delta.Removed {
		b.Entries = append(b.Entries, redistevents.RouteChangeEntry{
			Action: redistevents.ActionRemove,
			Prefix: pfx,
		})
	}
	if len(b.Entries) == 0 {
		return
	}
	sink(b)
}
