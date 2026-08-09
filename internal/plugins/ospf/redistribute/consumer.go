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
// InjectRoute asks the engine to originate a Type 5 AS-External-LSA (RFC 2328 sec
// 12.4.4) for the prefix in the AS-wide store, which makes the node an ASBR (Router-
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
	default:
		return nil, false
	}
}

func (c *Consumer) InjectRoute(_ context.Context, fam family.Family, entry configredist.RouteEntry) {
	src := sourceLabel(entry.Source)
	inj, want6 := c.injectorFor(fam)
	if inj == nil {
		slog.Debug("ospf redist consumer: skipping unsupported/unwired AFI inject",
			"family", fam.String(), "prefix", entry.Prefix, "source", src)
		return
	}
	prefix, err := netip.ParsePrefix(entry.Prefix)
	prefix = unmap4in6Prefix(prefix)
	if err != nil || !prefix.IsValid() || prefix.Addr().Is4() == want6 {
		slog.Warn("ospf redist consumer: rejecting malformed prefix",
			"family", fam.String(), "prefix", entry.Prefix, "source", src, "error", err)
		return
	}
	prefix = prefix.Masked()

	if err := inj.InjectExternal(prefix, src); err != nil {
		// R-3: never swallow a failed AS-External origination -- log it.
		slog.Warn("ospf redist consumer: AS-External origination failed after inject",
			"prefix", prefix.String(), "source", src, "error", err)
		return
	}
	c.rememberSource(prefix, src)
	c.mInjected.With(src).Inc()
}

// WithdrawRoute MaxAge-purges the Type 5 AS-External-LSA for prefix and re-floods so
// peers withdraw the external (AC-5). A prefix that was never injected is a no-op.
// The generic WithdrawRoute carries no source, so the consumer recovers the source
// label it recorded at inject time (forgetSource).
func (c *Consumer) WithdrawRoute(_ context.Context, fam family.Family, prefixStr string) {
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
