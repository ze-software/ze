// Design: docs/architecture/ospf/ospf-10-as-external-asbr.md -- OSPF redistribution consumer.
// Related: internal/plugins/isis/redistribute -- the consumer template this mirrors.
// RFC: rfc/short/rfc2328.md -- sec 12.4.4 AS-External-LSA origination (Type 5)
//
// The OSPF consumer implements configredist.RedistConsumer ("ospf"). The
// redistribute-orchestrator calls InjectRoute / WithdrawRoute for routes a
// configured `redistribute { destination ospf { import <source> } }` rule accepts
// (connected / static / BGP -- OSPF self-import is auto-rejected by the generic
// loop-prevention evaluator because origin "ospf" == importing protocol "ospf").
//
// InjectRoute runs the `ospf filter export` chain over the route (export.go). A
// rejected route is withdrawn; an accepted one asks the engine to originate a
// Type 5 AS-External-LSA (RFC 2328 sec 12.4.4) for the prefix in the AS-wide
// store, at the metric, metric type and tag the chain set, which makes the node an ASBR (Router-
// LSA E-bit) and floods the LSA AS-wide. WithdrawRoute MaxAge-purges it.

package ospfredistribute

import (
	"context"
	"log/slog"
	"net/netip"
	"sync"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
)

// ConsumerName is the single OSPF redistribution consumer name. It MUST equal the
// source name (events.Namespace) so the generic loop-prevention evaluator
// (route.Origin == importingProtocol) auto-rejects OSPF self-import (AC-13).
const ConsumerName = "ospf"

// Consumer is the OSPF RedistConsumer. It owns the redistributed-prefix source
// bookkeeping (so a withdraw, which carries no source, can label its metric) and
// originates Type 5 AS-External-LSAs via the ExternalInjector seam.
type Consumer struct {
	inj ExternalInjector
	// injV6 originates OSPFv3 AS-External-LSAs for redistributed IPv6 routes (the v6 engine).
	// Nil until SetV6Injector wires it; IPv6 redistribution is a no-op while nil.
	injV6 ExternalInjector
	// injV4OverV3 originates OSPFv3 AS-External-LSAs for redistributed IPv4 routes when an
	// RFC 5838 IPv4-unicast-over-OSPFv3 instance is configured (AC-13). When set, IPv4
	// redistribution is diverted to it instead of the OSPFv2 injector; nil otherwise.
	injV4OverV3 ExternalInjector

	// metrics owned by this spec (umbrella canonical Metrics table, owner ospf-10).
	// no-ops until SetMetrics wires a registry. The umbrella assigns exactly these
	// two redistribution counters; failures are logged, not counted (no
	// ze_ospf_redist_inject_failures_total series exists in the contract).
	mInjected  metrics.CounterVec // ze_ospf_redist_injected_total{source}
	mWithdrawn metrics.CounterVec // ze_ospf_redist_withdrawn_total{source}

	// srcMu guards source. The orchestrator dispatches InjectRoute/WithdrawRoute from
	// a single EventBus subscriber today, but the map is mutated on both paths so the
	// lock keeps the bookkeeping correct under any future concurrent caller.
	srcMu sync.Mutex
	// source records the redistribution source label per injected prefix, keyed by the
	// masked prefix. The generic WithdrawRoute carries NO source, so the consumer
	// remembers it at inject time and recovers it on withdraw. Without this the
	// withdrawn metric would always be labeled source="unknown".
	source map[netip.Prefix]string

	// offerMu serializes judging a route and writing the verdict into the LSDB,
	// so a re-judge after a reload and an inject or withdraw from the
	// orchestrator never interleave. It guards offered and is taken before
	// srcMu, never after.
	offerMu sync.Mutex
	// offered holds, per masked prefix, the latest route the orchestrator
	// handed this consumer and has not withdrawn, whether or not the export
	// chain accepted it. Rejudge reads it when a reload changes the chain: a
	// route the old chain rejected is still offered, so a chain that widens
	// originates it.
	offered map[netip.Prefix]offer
}

// offer is one route the orchestrator handed the consumer: its family and
// entry, judged again by Rejudge.
type offer struct {
	fam   family.Family
	entry configredist.RouteEntry
	// originated says an AS-external LSA for the route is in the LSDB, with
	// ext. Rejudge compares a new verdict with it and leaves an unchanged
	// route alone.
	originated bool
	ext        External
}

// NewConsumer constructs an OSPF redistribution consumer writing into inj. inj MUST
// be non-nil (the live engine in production, a fake in tests). Metrics start as
// no-ops until SetMetrics wires a registry.
func NewConsumer(inj ExternalInjector) *Consumer {
	nop := metrics.NopRegistry{}
	return &Consumer{
		inj:        inj,
		mInjected:  nop.CounterVec("", "", nil),
		mWithdrawn: nop.CounterVec("", "", nil),
		source:     map[netip.Prefix]string{},
		offered:    map[netip.Prefix]offer{},
	}
}

// SetV6Injector wires the OSPFv3 (IPv6) engine as the injector for redistributed IPv6
// routes. Until set, IPv6 redistribution is skipped (IPv4-only behavior).
func (c *Consumer) SetV6Injector(inj ExternalInjector) {
	c.injV6 = inj
}

// SetV4OverV3Injector wires the RFC 5838 IPv4-unicast-over-OSPFv3 engine as the injector for
// redistributed IPv4 routes (AC-13). When set, IPv4 redistribution originates an OSPFv3
// AS-External-LSA on that instance instead of an OSPFv2 Type 5; unset restores OSPFv2.
func (c *Consumer) SetV4OverV3Injector(inj ExternalInjector) {
	c.injV4OverV3 = inj
}

// SetMetrics registers the two redistribution counters this spec OWNS (umbrella
// canonical Metrics table). A nil registry is ignored (the no-ops stay).
func (c *Consumer) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	c.mInjected = reg.CounterVec(
		"ze_ospf_redist_injected_total",
		"Total routes injected into OSPF as Type 5 AS-External-LSAs by the redistribution consumer, by source.",
		[]string{"source"},
	)
	c.mWithdrawn = reg.CounterVec(
		"ze_ospf_redist_withdrawn_total",
		"Total routes withdrawn from OSPF Type 5 AS-External-LSAs by the redistribution consumer, by source.",
		[]string{"source"},
	)
}

// Name returns the consumer name "ospf" (registry lookup + self-import rejection).
func (c *Consumer) Name() string { return ConsumerName }

// sourceLabel returns a non-empty source label for metrics (the orchestrator
// populates RouteEntry.Source; default to "unknown" when absent so a label value is
// always present).
func sourceLabel(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func (c *Consumer) rememberSource(prefix netip.Prefix, src string) {
	c.srcMu.Lock()
	c.source[prefix] = src
	c.srcMu.Unlock()
}

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

// InjectRoute originates a Type 5 AS-External-LSA for entry.Prefix in the AS-wide
// store, then makes the node an ASBR and floods. OSPFv2 is IPv4-only: a non-IPv4
// family is a no-op (logged at debug). A malformed prefix is rejected (security
// review: input validation) without mutating state.
// unmap4in6Prefix canonicalizes an IPv4-mapped IPv6 prefix (::ffff:a.b.c.d) to its IPv4 form so
// the address-family guard classifies it correctly: a 4-in-6 address reports Is4()==false and
// would otherwise slip past the IPv6 injector's guard, injecting a v4 route as v6.
func unmap4in6Prefix(p netip.Prefix) netip.Prefix {
	if a := p.Addr(); a.Is4In6() {
		return netip.PrefixFrom(a.Unmap(), max(0, p.Bits()-96))
	}
	return p
}

// injectorFor selects the address-family injector and whether it expects IPv6. RFC 5838:
// an IPv4-over-OSPFv3 instance, when its AF engine is actually running, takes IPv4
// redistribution ahead of the OSPFv2 injector so the route originates an OSPFv3
// AS-External-LSA (AC-13). If the IPv4-over-v3 injector is wired but inactive (no AF engine
// configured), it is skipped so IPv4 redistribution falls back to the OSPFv2 injector and
// still originates a Type 5 rather than silently no-op'ing (ext-15 review fix 1).
func (c *Consumer) injectorFor(fam family.Family) (ExternalInjector, bool) {
	switch fam.AFI {
	case family.AFIIPv4:
		if c.injV4OverV3 != nil {
			if opt, ok := c.injV4OverV3.(OptionalInjector); !ok || opt.Active() {
				return c.injV4OverV3, false
			}
		}
		return c.inj, false
	case family.AFIIPv6:
		return c.injV6, true
	case family.AFIL2VPN, family.AFIBGPLS:
		return nil, false
	default:
		// Redistribution families are an open input set; unsupported AFIs have no injector.
		return nil, false
	}
}

func (c *Consumer) InjectRoute(ctx context.Context, fam family.Family, entry configredist.RouteEntry) {
	c.offerMu.Lock()
	defer c.offerMu.Unlock()

	prefix, ext, originated := c.judgeRoute(ctx, fam, entry)
	if prefix.IsValid() {
		c.offered[prefix] = offer{fam: fam, entry: entry, originated: originated, ext: ext}
	}
}

// Rejudge judges every route the orchestrator offered again, against the
// export chain installed now, and writes each new verdict into the LSDB: a
// route the chain now rejects is withdrawn, one it now accepts is originated,
// and a changed metric, metric type or tag re-originates the LSA with them.
// The config apply MUST call it after SetExportChain, because a reload
// delivers no route again. Safe for concurrent use with InjectRoute and
// WithdrawRoute.
func (c *Consumer) Rejudge(ctx context.Context) {
	c.offerMu.Lock()
	defer c.offerMu.Unlock()

	for prefix, o := range c.offered {
		ext, accepted := exportVerdict(&o.entry)
		if accepted == o.originated && ext == o.ext {
			continue // the verdict did not change: the LSDB already holds it
		}
		_, o.ext, o.originated = c.judgeRoute(ctx, o.fam, o.entry)
		c.offered[prefix] = o
	}
}

// judgeRoute is InjectRoute under offerMu: it judges entry against the export
// chain and originates or withdraws its AS-external LSA. It answers the masked
// prefix (invalid when the route cannot be carried at all), the AS-external
// fields, and true when the LSA is now in the LSDB, false when the chain
// rejected the route or its origination failed.
func (c *Consumer) judgeRoute(ctx context.Context, fam family.Family, entry configredist.RouteEntry) (netip.Prefix, External, bool) {
	src := sourceLabel(entry.Source)
	inj, want6 := c.injectorFor(fam)
	if inj == nil {
		slog.Debug("ospf redist consumer: skipping unsupported/unwired AFI inject",
			"family", fam.String(), "prefix", entry.Prefix, "source", src)
		return netip.Prefix{}, External{}, false
	}
	prefix, err := netip.ParsePrefix(entry.Prefix)
	prefix = unmap4in6Prefix(prefix)
	if err != nil || !prefix.IsValid() || prefix.Addr().Is4() == want6 {
		slog.Warn("ospf redist consumer: rejecting malformed prefix",
			"family", fam.String(), "prefix", entry.Prefix, "source", src, "error", err)
		return netip.Prefix{}, External{}, false
	}
	prefix = prefix.Masked()

	// The export chain judges every route before it reaches the LSDB: a rejected
	// route is not originated, and one this prefix originated before is withdrawn.
	ext, accepted := exportVerdict(&entry)
	if !accepted {
		slog.Debug("ospf redist consumer: export chain rejected route",
			"family", fam.String(), "prefix", entry.Prefix, "source", src)
		c.removeRoute(ctx, fam, entry.Prefix)
		return prefix, External{}, false
	}
	// RFC 2328 Section 12.4.4
	if err := inj.InjectExternal(prefix, src, ext); err != nil {
		// R-3: never swallow a failed AS-External origination -- log it.
		slog.Warn("ospf redist consumer: AS-External origination failed after inject",
			"prefix", prefix.String(), "source", src, "error", err)
		return prefix, ext, false
	}
	c.rememberSource(prefix, src)
	c.mInjected.With(src).Inc()
	return prefix, ext, true
}

// WithdrawRoute MaxAge-purges the Type 5 AS-External-LSA for prefix and re-floods so
// peers withdraw the external (AC-5). A prefix that was never injected is a no-op.
// The generic WithdrawRoute carries no source, so the consumer recovers the source
// label it recorded at inject time (forgetSource).
func (c *Consumer) WithdrawRoute(ctx context.Context, fam family.Family, prefixStr string) {
	c.offerMu.Lock()
	defer c.offerMu.Unlock()

	// The withdrawn route is no longer offered, so no reload re-originates it.
	if prefix, err := netip.ParsePrefix(prefixStr); err == nil {
		delete(c.offered, unmap4in6Prefix(prefix).Masked())
	}
	c.removeRoute(ctx, fam, prefixStr)
}

// removeRoute is WithdrawRoute under offerMu, also used when the export chain
// rejects a route: it MaxAge-purges the prefix's AS-external LSA.
func (c *Consumer) removeRoute(_ context.Context, fam family.Family, prefixStr string) {
	inj, want6 := c.injectorFor(fam)
	if inj == nil {
		slog.Debug("ospf redist consumer: skipping unsupported/unwired AFI withdraw",
			"family", fam.String(), "prefix", prefixStr)
		return
	}
	prefix, err := netip.ParsePrefix(prefixStr)
	prefix = unmap4in6Prefix(prefix)
	if err != nil || !prefix.IsValid() || prefix.Addr().Is4() == want6 {
		slog.Warn("ospf redist consumer: rejecting malformed prefix on withdraw",
			"family", fam.String(), "prefix", prefixStr, "error", err)
		return
	}
	prefix = prefix.Masked()

	removed, err := inj.WithdrawExternal(prefix)
	src := c.forgetSource(prefix)
	if err != nil {
		slog.Warn("ospf redist consumer: Type 5 purge failed after withdraw",
			"prefix", prefix.String(), "source", src, "error", err)
		return
	}
	if !removed {
		return // never injected; nothing to withdraw
	}
	c.mWithdrawn.With(src).Inc()
}

// compile-time assertion: *Consumer satisfies the generic RedistConsumer.
var _ configredist.RedistConsumer = (*Consumer)(nil)
