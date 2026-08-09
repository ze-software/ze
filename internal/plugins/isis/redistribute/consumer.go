// Design: docs/architecture/isis/isis-11-redistribution.md -- IS-IS redistribution consumer.
// Related: internal/component/bgp/redistribute -- the consumer template this mirrors.
// RFC: rfc/short/rfc5305.md -- TLV 135 Extended IP Reachability (no external bit; up/down + sub-TLV-present control octet)
// RFC: rfc/short/rfc2966.md -- up/down bit set only on down-level leak (loop prevention)
//
// The IS-IS consumer implements configredist.RedistConsumer ("isis"). The
// redistribute-orchestrator calls InjectRoute / WithdrawRoute for routes a
// configured `redistribute { destination isis { import <source> } }` rule accepts
// (connected / static / BGP -- IS-IS self-import is auto-rejected by the generic
// loop-prevention evaluator because origin "isis" == importing protocol "isis").
//
// InjectRoute turns the RouteEntry into a TLV 135 Extended IP Reachability entry
// (RFC 5305 sec 4) in the node's own LSP set, with the FIXED default metric
// (DefaultRedistMetric) and the up/down bit CLEAR on first injection -- TLV 135
// carries NO external bit (RFC 5305 sec 4), and the up/down bit is set only when a
// prefix is leaked to a lower level (RFC 2966, done by isis-9 SPF leaking, not
// here). Then it re-originates so flooding carries the change to peers, which run
// SPF and install the route.

package isisredistribute

import (
	"context"
	"log/slog"
	"net/netip"
	"sync"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// afiIPv4Label and afiIPv6Label are the metric "afi" label values. They are
// extracted to consts so the inject and withdraw paths (and the IPv6 twin in
// ipv6.go) cannot drift apart.
const (
	afiIPv4Label = "ipv4"
	afiIPv6Label = "ipv6"
)

// ConsumerName is the single IS-IS redistribution consumer name. It MUST equal
// the source name (events.Namespace) so the generic loop-prevention evaluator
// (route.Origin == importingProtocol) auto-rejects IS-IS self-import (AC-10).
const ConsumerName = "isis"

// Consumer is the IS-IS RedistConsumer. It owns the redistributed-prefix
// bookkeeping (a per-prefix record of the levels it was injected into, so a
// withdraw removes exactly the right entries) and writes TLV 135 reachability into
// the engine's own LSPs via the LSPInjector seam.
type Consumer struct {
	inj LSPInjector

	// metrics owned by this spec (umbrella canonical Metrics table, owner isis-11).
	// no-ops until SetMetrics wires a registry.
	mInjected     metrics.CounterVec // ze_isis_redist_injected_total{source,afi}
	mWithdrawn    metrics.CounterVec // ze_isis_redist_withdrawn_total{source,afi}
	mInjectFailed metrics.CounterVec // ze_isis_redist_inject_failures_total{source}

	// srcMu guards source. The orchestrator dispatches InjectRoute/WithdrawRoute
	// from a single EventBus subscriber today, but the map is mutated on both paths
	// so the lock keeps the bookkeeping correct under any future concurrent caller.
	srcMu sync.Mutex
	// source records the redistribution source label per injected prefix, keyed by
	// the masked prefix. The generic WithdrawRoute carries NO source (the
	// orchestrator does not thread it through the withdraw path), so the consumer
	// remembers the source at inject time and recovers it on withdraw, mirroring the
	// inject path's sourceLabel(entry.Source). Without this the withdrawn/failure
	// metrics would always be labeled source="unknown".
	source map[netip.Prefix]string
}

// NewConsumer constructs an IS-IS redistribution consumer writing into inj. inj
// MUST be non-nil (the live engine in production, a fake in tests). Metrics start
// as no-ops until SetMetrics wires a registry.
func NewConsumer(inj LSPInjector) *Consumer {
	nop := metrics.NopRegistry{}
	return &Consumer{
		inj:           inj,
		mInjected:     nop.CounterVec("", "", nil),
		mWithdrawn:    nop.CounterVec("", "", nil),
		mInjectFailed: nop.CounterVec("", "", nil),
		source:        map[netip.Prefix]string{},
	}
}

// rememberSource records the source label for a masked prefix at inject time so a
// later WithdrawRoute (which carries no source) can recover it. IPv4 and IPv6 keys
// never collide (an IPv4 netip.Prefix is never equal to an IPv6 one).
func (c *Consumer) rememberSource(prefix netip.Prefix, src string) {
	c.srcMu.Lock()
	c.source[prefix] = src
	c.srcMu.Unlock()
}

// forgetSource removes and returns the remembered source label for a masked
// prefix. It returns "unknown" (via sourceLabel) when the prefix was never
// injected, so the withdraw metric always carries a label value.
func (c *Consumer) forgetSource(prefix netip.Prefix) string {
	c.srcMu.Lock()
	src, ok := c.source[prefix]
	delete(c.source, prefix)
	c.srcMu.Unlock()
	if !ok {
		return sourceLabel("")
	}
	return src
}

// SetMetrics registers the redistribution counters this spec OWNS (umbrella
// canonical Metrics table). A nil registry is ignored (the no-ops stay).
func (c *Consumer) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	c.mInjected = reg.CounterVec(
		"ze_isis_redist_injected_total",
		"Total routes injected into IS-IS LSPs by the redistribution consumer, by source and address family.",
		[]string{"source", "afi"},
	)
	c.mWithdrawn = reg.CounterVec(
		"ze_isis_redist_withdrawn_total",
		"Total routes withdrawn from IS-IS LSPs by the redistribution consumer, by source and address family.",
		[]string{"source", "afi"},
	)
	c.mInjectFailed = reg.CounterVec(
		"ze_isis_redist_inject_failures_total",
		"Total IS-IS redistribution injections that failed to re-originate the LSP, by source.",
		[]string{"source"},
	)
}

// Name returns the consumer name "isis" (registry lookup + self-import rejection).
func (c *Consumer) Name() string { return ConsumerName }

// sourceLabel returns a non-empty source label for metrics (the orchestrator
// populates RouteEntry.Source; default to "unknown" when absent so a label value
// is always present).
func sourceLabel(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// InjectRoute originates a TLV 135 reachability entry for entry.Prefix in the
// node's own LSP set at every origination level, then re-originates.
//
// Only IPv4-unicast is handled here: IPv6 redistribution (TLV 236) is owned by
// isis-12, so an IPv6 family is a no-op (logged at debug). A malformed prefix is
// rejected (security review: input validation) without mutating state.
//
// RFC 5305 sec 4: TLV 135 entries carry a 32-bit metric, a control octet with the
// up/down bit and sub-TLV-present bit, and the packed prefix. There is NO external
// bit -- redistributed IPv4 routes are ordinary entries with the up/down bit CLEAR
// on first injection (set to 1 only when leaked to a lower level, RFC 2966, done
// by isis-9 SPF leaking, not here).
func (c *Consumer) InjectRoute(ctx context.Context, fam family.Family, entry configredist.RouteEntry) {
	src := sourceLabel(entry.Source)
	if fam.AFI == family.AFIIPv6 {
		// IPv6 (isis-12): originate a TLV 236 entry. Handled in ipv6.go.
		c.injectRouteV6(ctx, fam, entry)
		return
	}
	if fam.AFI != family.AFIIPv4 {
		slog.Debug("isis redist consumer: skipping unsupported AFI inject",
			"family", fam.String(), "prefix", entry.Prefix, "source", src)
		return
	}
	prefix, err := netip.ParsePrefix(entry.Prefix)
	if err != nil || !prefix.IsValid() || !prefix.Addr().Is4() {
		slog.Warn("isis redist consumer: rejecting malformed IPv4 prefix",
			"prefix", entry.Prefix, "source", src, "error", err)
		return
	}
	prefix = prefix.Masked()

	info := lsdb.PrefixInfo{
		Prefix: prefix,
		Metric: types.NewPrefixMetric(DefaultRedistMetric),
		// up/down bit CLEAR on injection: TLV 135 has no external bit (RFC 5305
		// sec 4); the up/down bit is set only on down-level leak (RFC 2966).
		UpDown: false,
	}
	for _, level := range c.inj.OriginationLevels() {
		c.inj.SetRedistPrefix(level, info)
	}
	c.rememberSource(prefix, src)
	c.mInjected.With(src, afiIPv4Label).Inc()

	if err := c.inj.Originate(); err != nil {
		// R-3: never swallow a failed re-origination -- log and count it.
		slog.Warn("isis redist consumer: re-origination failed after inject",
			"prefix", prefix.String(), "source", src, "error", err)
		c.mInjectFailed.With(src).Inc()
	}
}

// WithdrawRoute removes the TLV 135 reachability entry for prefix from every
// origination level and re-originates so peers withdraw the route from their
// kernel (AC-6). A prefix that was never injected is a no-op.
//
// The generic WithdrawRoute carries no source (the orchestrator does not thread
// the source through the withdraw path), so the consumer recovers the source label
// it recorded at inject time (forgetSource), mirroring the inject path's
// sourceLabel(entry.Source). A prefix it never injected resolves to "unknown".
func (c *Consumer) WithdrawRoute(ctx context.Context, fam family.Family, prefixStr string) {
	if fam.AFI == family.AFIIPv6 {
		// IPv6 (isis-12): withdraw the TLV 236 entry. Handled in ipv6.go.
		c.withdrawRouteV6(ctx, fam, prefixStr)
		return
	}
	if fam.AFI != family.AFIIPv4 {
		slog.Debug("isis redist consumer: skipping unsupported AFI withdraw",
			"family", fam.String(), "prefix", prefixStr)
		return
	}
	prefix, err := netip.ParsePrefix(prefixStr)
	if err != nil || !prefix.IsValid() || !prefix.Addr().Is4() {
		slog.Warn("isis redist consumer: rejecting malformed IPv4 prefix on withdraw",
			"prefix", prefixStr, "error", err)
		return
	}
	prefix = prefix.Masked()

	removed := false
	for _, level := range c.inj.OriginationLevels() {
		if c.inj.RemoveRedistPrefix(level, prefix) {
			removed = true
		}
	}
	// Recover the source recorded at inject time so the withdrawn/failure metrics
	// carry the right label instead of always "unknown". forgetSource also drops the
	// bookkeeping entry; do it on the not-injected path too so a stray withdraw does
	// not leak a map entry.
	src := c.forgetSource(prefix)
	if !removed {
		return // never injected; nothing to re-originate
	}
	c.mWithdrawn.With(src, afiIPv4Label).Inc()

	if err := c.inj.Originate(); err != nil {
		slog.Warn("isis redist consumer: re-origination failed after withdraw",
			"prefix", prefix.String(), "source", src, "error", err)
		c.mInjectFailed.With(src).Inc()
	}
}

// compile-time assertion: *Consumer satisfies the generic RedistConsumer.
var _ configredist.RedistConsumer = (*Consumer)(nil)
