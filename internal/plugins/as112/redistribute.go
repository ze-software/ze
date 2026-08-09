// Design: docs/architecture/bgp/as112-coordination.md -- AS112 layering rule:
// as112 never reads bgp config; it emits a generic named redistribute source and
// BGP imports "as112" the same way it imports "static".
// Related: internal/plugins/as112/events/events.go -- typed EventBus handle + ProtocolID
// RFC: rfc/short/rfc7534.md Section 3.3 -- do not advertise while the name server
// is not running (the watchdog serving-state gate); Section 3.4 -- restrict
// advertisement (the community + AS-PATH-origin match handles).

package as112

import (
	"net/netip"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	as112events "github.com/ze-software/ze/internal/plugins/as112/events"
)

// The four fixed AS112 COVERING prefixes BGP announces (RFC 7534 Section 3.4
// Direct Delegation, RFC 7535 Section 3.1 DNAME Redirection) -- NOT the /32,/128
// host addresses bound on lo (register.go's hostAddresses) -- live in the shared
// as112events leaf as CoveringPrefixesV4/V6, the single source shared with the
// fakeas112 test producer so the announced set cannot drift. Conflating the
// covering prefixes with the host addresses is the documented AS112 mistake
// (spec-as112-0 umbrella finding H3).

var sourcesOnce sync.Once

// registerAS112Sources registers "as112" as a redistribute source so
// `redistribute { destination bgp { import as112 } }` resolves. Called from
// init() (register.go) -- not the engine run -- so the source is visible to
// `ze config validate`, which imports plugins but does not start their engines.
// sync.Once keeps it idempotent. The Name/Protocol match as112events.Namespace
// so the orchestrator resolves the ProtocolID by the same string.
func registerAS112Sources() {
	sourcesOnce.Do(func() {
		_ = redistribute.RegisterSource(redistribute.RouteSource{
			Name:        as112events.Namespace,
			Protocol:    as112events.Namespace,
			Description: "AS112 covering prefixes originated as a virtual router",
		})
	})
}

// as112Producer originates the fixed AS112 covering prefixes into BGP as a
// redistribute producer. It holds the currently-announced state so it can
// reconcile on each config apply, withdraw on shutdown, and re-emit its current
// set on a late-join replay request. It NEVER reads bgp config: `import as112`
// gates whether the emitted routes reach the RIB (via the orchestrator's
// evaluator), which preserves the AS112 layering rule.
type as112Producer struct {
	mu        sync.Mutex
	announced bool
	families  []family.Family
	originASN uint32
	community []uint32
	// lastCfg is the last applied config. servingFn reports the LIVE serving state
	// of a SINGLE family (that family's anycast listeners are up) from the DNS
	// server on each reconcile, so a config apply and a runtime serving-state
	// transition both re-derive the announce decision from the current combined
	// state. Reading live (rather than storing a serving snapshot) means concurrent
	// listener edges cannot leave a stale serving value: whichever reconcile runs
	// last reads the true state. It is queried PER FAMILY so a partial anycast
	// outage (e.g. IPv6 binds fail while IPv4 is up) withdraws only the affected
	// family rather than blackholing it (RFC 7534 Section 3.3). Nil until wired
	// (treated as not serving); tests pin it via setServingFn.
	lastCfg   as112Config
	servingFn func(family.Family) bool
}

func newAS112Producer() *as112Producer { return &as112Producer{} }

// familiesFor maps the address-family config to the emitted BGP families.
func familiesFor(addressFamily string) []family.Family {
	var fams []family.Family
	if addressFamily != addressFamilyIPv6Only {
		fams = append(fams, family.IPv4Unicast)
	}
	if addressFamily != addressFamilyIPv4Only {
		fams = append(fams, family.IPv6Unicast)
	}
	return fams
}

// coveringPrefixesFor returns the covering prefixes for a family (from the shared
// as112events source).
func coveringPrefixesFor(fam family.Family) []netip.Prefix {
	if fam.AFI == family.IPv4Unicast.AFI {
		return as112events.CoveringPrefixesV4
	}
	return as112events.CoveringPrefixesV6
}

// reconcile recomputes the announcement from the current combined (config,
// serving) state and emits the delta. newCfg, when non-nil, updates the stored
// config first, so a config apply and a runtime serving-state transition funnel
// through the same convergent logic (order does not matter). A covering-prefix
// family is announced when the service is enabled AND (the watchdog is disabled
// OR that family's DNS anycast listeners are up) -- RFC 7534 Section 3.3,
// evaluated PER FAMILY so a partial outage withdraws only the affected family.
//
// The delta is computed AND emitted under p.mu. This is deliberate: the serving
// gate races (a config apply can run concurrently with a runtime listener edge),
// and if two reconciles released the lock before emitting, the EventBus could
// deliver a stale ADD after a newer WITHDRAW -- announcing a prefix into a dead
// server (blackhole) or leaking a route with no self-heal. Holding p.mu across
// Emit forces the emit order to match the state-commit order. It is safe: a
// covering-prefix announce never re-enters this producer (it does not trigger a
// ReplayRequest), so there is no lock re-entry, and these emits are tiny (<=4
// prefixes) and rare (config/serving edges), not a hot path.
func (p *as112Producer) reconcile(newCfg *as112Config) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if newCfg != nil {
		p.lastCfg = *newCfg
	}
	cfg := p.lastCfg

	var newFamilies []family.Family
	if cfg.Enabled {
		for _, fam := range familiesFor(cfg.AddressFamily) {
			// A family is announced only when its own anycast listeners serve.
			if !cfg.Watchdog || (p.servingFn != nil && p.servingFn(fam)) {
				newFamilies = append(newFamilies, fam)
			}
		}
	}
	newASN, newCommunity := cfg.ASN, cfg.Community
	if len(newFamilies) == 0 {
		newASN, newCommunity = 0, nil
	}

	oldFamilies := p.families
	oldASN, oldCommunity := p.originASN, p.community
	attrsChanged := oldASN != newASN || !slices.Equal(oldCommunity, newCommunity)

	var toWithdraw, toAdd []family.Family
	for _, f := range oldFamilies {
		if !slices.Contains(newFamilies, f) {
			toWithdraw = append(toWithdraw, f) // a family that dropped out
		}
	}
	for _, f := range newFamilies {
		if !slices.Contains(oldFamilies, f) || attrsChanged {
			toAdd = append(toAdd, f) // newly announced, or an attribute update
		}
	}

	p.announced = len(newFamilies) > 0
	p.families = newFamilies
	p.originASN = newASN
	p.community = newCommunity

	// Withdraw dropped families with their OLD attributes (the withdraw path
	// ignores attributes, but keep them consistent); add with the new ones.
	for _, f := range toWithdraw {
		p.emit(redistevents.ActionRemove, f, oldASN, oldCommunity, 0)
	}
	for _, f := range toAdd {
		p.emit(redistevents.ActionAdd, f, newASN, newCommunity, 0)
	}
}

// applyConfig reconciles a new config against the live serving state. Called on
// each config apply.
func (p *as112Producer) applyConfig(cfg as112Config) { p.reconcile(&cfg) }

// setServingFn wires the live per-family serving source. Assigned under p.mu to
// honor the field's lock invariant (reconcile reads servingFn under p.mu); in
// practice it runs once during setup before any reconcile can race it.
func (p *as112Producer) setServingFn(fn func(family.Family) bool) {
	p.mu.Lock()
	p.servingFn = fn
	p.mu.Unlock()
}

// onServingChanged reconciles on a runtime serving-state edge signaled by the
// DNS server (production entry point). reconcile re-reads the live serving state
// via servingFn, so this withdraws the covering prefixes when the node stops
// serving without a config change, and re-announces when it recovers (AC-7,
// RFC 7534 Section 3.3).
func (p *as112Producer) onServingChanged() { p.reconcile(nil) }

// withdraw retracts every currently-announced covering prefix. Used on plugin
// shutdown so the routes do not linger in the reactor RIB after the DNS node
// stops (AC-9). Idempotent: a no-op when nothing is announced.
func (p *as112Producer) withdraw() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.announced {
		return
	}
	fams := p.families
	asn, community := p.originASN, p.community
	p.announced = false
	p.families = nil
	p.originASN = 0
	p.community = nil

	// Emit under p.mu so the withdraws cannot be reordered past a concurrent
	// reconcile's emits (see reconcile for the ordering rationale).
	for _, f := range fams {
		p.emit(redistevents.ActionRemove, f, asn, community, 0)
	}
}

// reemitAll re-emits the current announced set tagged with replayID so the
// redistribute orchestrator can replay it to a peer that established after the
// original emit (AC-14, spec-redistribute-late-join-replay). Reflects the
// CURRENT set (all adds); a replayID of 0 is a no-op (the orchestrator only
// allocates nonzero tokens). Does NOT mutate the announced state.
func (p *as112Producer) reemitAll(replayID uint64) {
	if replayID == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.announced {
		return
	}
	// Emit under p.mu: a concurrent config change must not replace the origin
	// ASN / community between the snapshot and the replay emit (see reconcile).
	for _, f := range p.families {
		p.emit(redistevents.ActionAdd, f, p.originASN, p.community, replayID)
	}
}

// emit publishes one per-family batch of the covering prefixes on the EventBus.
// One batch carries a single (protocol, family) tuple per the redistevents
// contract; the batch is released immediately after Emit (subscribers dispatch
// synchronously and must not retain it).
func (p *as112Producer) emit(action redistevents.RouteAction, fam family.Family, asn uint32, community []uint32, replayID uint64) {
	bus := getEventBus()
	if bus == nil {
		return
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = as112events.ProtocolID
	b.AFI = uint16(fam.AFI)
	b.SAFI = uint8(fam.SAFI)
	b.OriginASN = asn
	b.Community = community
	b.ReplayID = replayID
	for _, pfx := range coveringPrefixesFor(fam) {
		b.Entries = append(b.Entries, redistevents.RouteChangeEntry{Action: action, Prefix: pfx})
	}
	if _, err := as112events.RouteChange.Emit(bus, b); err != nil {
		loggerPtr.Load().Warn("as112: route-change emit failed", "family", fam.String(), "action", action.String(), "error", err)
	}
}

// subscribeReplay wires the producer to the shared late-join replay request.
// Returns an unsubscribe func (a no-op when no EventBus is configured).
func (p *as112Producer) subscribeReplay() func() {
	bus := getEventBus()
	if bus == nil {
		return func() {}
	}
	return redistevents.ReplayRequestEvent.Subscribe(bus, func(r *redistevents.ReplayRequest) {
		p.reemitAll(r.ReplayID)
	})
}
