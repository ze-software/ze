// Design: docs/architecture/ospf/ospf-ext-15-multi-af.md -- RFC 5838 per-AF engine lifecycle.
// RFC: rfc/short/rfc5838.md (§2 separate LSDB/neighbors per AF instance)
// Related: register.go -- runOSPFEngine drives this set from the plugin config callbacks.
// Related: multiaf.go -- the AF <-> Instance-ID-range mapping each engine is keyed by.
// Related: debug_wiring.go -- v3Views, the OSPFv3 views that read the engines selectInstances
// answers and lead each row with its instance's identity (afTaggedRow).
//
// v6EngineSet owns the OSPFv3 (IPv6-transport) engine instances, one per configured RFC 5838
// address family. Each engine is fully self-contained (its own LSDB, neighbor table, SPF,
// and Loc-RIB install family), so per-AF separation is structural: multi-AF is a fan-out of
// the single v6 engine, not a new LSDB key. The default IPv6-unicast AF preserves the
// single-instance IPv6-base behavior byte-for-byte when it is the only AF configured.
package ospf

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/netip"
	"strconv"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

// v6EngineSet manages the per-AF OSPFv3 engine instances. Guarded by mu; the plugin config
// callbacks (verify/configure/apply/started) are serialized by the SDK, but the show and
// redistribution paths read the set concurrently.
type v6EngineSet struct {
	mu      sync.Mutex
	engines map[addressFamily]*engine
	started bool
	afGauge metrics.GaugeVec
	state   stateClient
}

// newV6EngineSet builds an empty set and registers the ze_ospf_af_instances gauge.
func newV6EngineSet() *v6EngineSet {
	m := &v6EngineSet{
		engines: make(map[addressFamily]*engine),
		afGauge: metrics.NopRegistry{}.GaugeVec("", "", nil),
	}
	if reg := getMetricsRegistry(); reg != nil {
		m.afGauge = reg.GaugeVec(
			"ze_ospf_af_instances",
			"Current number of running OSPFv3 (RFC 5838) address-family engine instances, by address family.",
			[]string{"af"},
		)
	}
	return m
}

// spawn constructs a v6-codec engine for af over the ospfv3 transport and wires the shared
// metrics registry and event sink so per-AF observability and events work like the OSPFv2
// engine. The engine stays idle (no sockets, no goroutines) until openInterfaces.
func (m *v6EngineSet) spawn(af addressFamily) *engine {
	v6transport := ospfv3transport.New(ospfv3transport.NewBackend())
	e := newEngineWithCodecAF(v6transport, v6Codec{}, af)
	e.state = m.state
	if reg := getMetricsRegistry(); reg != nil {
		e.transport.SetMetrics(reg)
		e.setMetrics(reg)
		// BFD metrics are the shared ze_ospf_bfd_* series (get-or-create by name), so each AF
		// engine registers the SAME series as the IPv4 family; the interface label distinguishes
		// them (learned 970: unified ze_ospf_* namespace, never ze_ospfv3_*).
		e.setBFDMetrics(reg)
	}
	if eb := getEventBus(); eb != nil {
		e.setEventSink(newEventSink(eb))
	}
	// RFC 4552 (spec-ospf-ext-16): attach this AF engine's kernel IPsec installer. It reads
	// the interface link-local + ifindex from THIS engine's v3 transport and is driven by the
	// engine's OnInterfaceUp/OnInterfaceDown hooks, so the kernel policy+SA exist before the
	// first Hello. Per-engine wiring matches the v6set model (each AF has its own transport);
	// only interfaces that actually configure `ipsec` (the IPv6 default family) install
	// anything, and the ze_ospfv3_ipsec_* series are registration-idempotent so sharing the
	// registry across AF engines is safe.
	ipsecInstaller := newIPsecInstaller(getMetricsRegistry(), logger())
	ipsecInstaller.setTransportSource(v6transport.InterfaceSource)
	e.installIPsecHooks(ipsecInstaller)
	return e
}

// getOrCreate returns the engine for af, creating (and registering the gauge) on first use.
// Caller holds mu.
func (m *v6EngineSet) getOrCreateLocked(af addressFamily) *engine {
	if e, ok := m.engines[af]; ok {
		return e
	}
	e := m.spawn(af)
	m.engines[af] = e
	m.afGauge.With(af.String()).Set(1)
	return e
}

// configure stages each configured AF's config into its engine (plugin OnConfigure). It
// creates engines as needed and records multi-AF awareness, but opens no interfaces.
func (m *v6EngineSet) configure(families []v6AFConfig, multiAF bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range families {
		e := m.getOrCreateLocked(families[i].af)
		e.setMultiAF(multiAF)
		e.setConfig(families[i].cfg)
	}
}

// apply reconciles the running engine set against the configured families (plugin
// OnConfigApply): a new AF's engine is created and (once started) brought up; an existing
// engine is reconciled; an AF no longer configured is shut down and forgotten (AC-12).
func (m *v6EngineSet) apply(families []v6AFConfig, multiAF bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := make(map[addressFamily]*v6AFConfig, len(families))
	for i := range families {
		want[families[i].af] = &families[i]
	}
	// Remove AFs that are no longer configured.
	for af, e := range m.engines {
		if _, keep := want[af]; keep {
			continue
		}
		e.shutdown()
		delete(m.engines, af)
		m.afGauge.With(af.String()).Set(0)
	}
	// Add or reconcile the desired AFs.
	for af, fam := range want {
		existing, ok := m.engines[af]
		if ok {
			if m.started {
				if err := existing.initializeState(); err != nil {
					return err
				}
			}
			existing.setMultiAF(multiAF)
			existing.reconcile(fam.cfg)
			continue
		}
		e := m.getOrCreateLocked(af)
		e.setMultiAF(multiAF)
		e.setConfig(fam.cfg)
		if m.started {
			if err := m.bringUpLocked(e); err != nil {
				return err
			}
		}
	}
	return nil
}

// start brings up every configured AF for the initial plugin start (plugin OnStarted).
func (m *v6EngineSet) start(families []v6AFConfig, multiAF bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
	for i := range families {
		if !families[i].cfg.Present() {
			continue
		}
		e := m.getOrCreateLocked(families[i].af)
		e.setMultiAF(multiAF)
		e.setConfig(families[i].cfg)
		if err := m.bringUpLocked(e); err != nil {
			return err
		}
	}
	return nil
}

// bringUpLocked subscribes an engine to interface events and opens its interfaces.
func (m *v6EngineSet) bringUpLocked(e *engine) error {
	if err := e.initializeState(); err != nil {
		return err
	}
	e.subscribeIfaceEvents(getEventBus())
	// RFC 5443/6138 LDP-IGP sync: the OSPFv3 (IPv6) family reuses the AF-neutral LDP-sync
	// machine on the shared interface model; subscribe this AF engine to LDP session events
	// (unsubscribed on shutdown). No-op when LDP is absent.
	e.subscribeLDPSyncEvents(getEventBus())
	return e.openInterfaces()
}

// shutdownAll stops every engine (plugin exit).
func (m *v6EngineSet) shutdownAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for af, e := range m.engines {
		e.shutdown()
		m.afGauge.With(af.String()).Set(0)
	}
	m.engines = make(map[addressFamily]*engine)
}

// engineFor returns the engine handling af, if running.
func (m *v6EngineSet) engineFor(af addressFamily) (*engine, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.engines[af]
	return e, ok
}

// afRangeOrder lists the RFC 5838 address families in Instance-ID range order (Section 2.1).
var afRangeOrder = []addressFamily{afIPv6Unicast, afIPv6Multicast, afIPv4Unicast, afIPv4Multicast}

// runningAFs returns the address families with a running engine, in RFC 5838 range order.
func (m *v6EngineSet) runningAFs() []addressFamily {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]addressFamily, 0, len(m.engines))
	for _, af := range afRangeOrder {
		if _, ok := m.engines[af]; ok {
			out = append(out, af)
		}
	}
	return out
}

// fieldInstanceID is the key that follows keywordAddressFamily (cmd_show.go) at the head of
// every OSPFv3 view row: the keyword that narrows a view is also the key its rows lead with.
const fieldInstanceID = "instance-id"

// errViewRowNotObject is a Ze defect: every OSPFv3 view row is a struct, so it marshals to
// a JSON object that afTaggedRow can lead with the instance identity.
var errViewRowNotObject = errors.New("BUG: an OSPFv3 view row did not marshal to a JSON object")

// afInstance is one running OSPFv3 address-family engine a view reads, with the identity
// each of its rows carries.
type afInstance struct {
	af         addressFamily
	instanceID uint8
	eng        *engine
}

// instanceFor answers the running engine of af with its own Instance ID (the one its
// dispatcher demuxes on), or false when af has no running engine.
func (m *v6EngineSet) instanceFor(af addressFamily) (afInstance, bool) {
	e, ok := m.engineFor(af)
	if !ok {
		return afInstance{}, false
	}
	return afInstance{af: af, instanceID: e.dispatch.currentInstanceID(), eng: e}, true
}

// selectInstances answers the running engines a view of command reads: every running AF in
// RFC 5838 range order when cmdArgs is empty, or the one AF cmdArgs names as
// `address-family <af>`. An AF that is not running, an unknown name, a missing value and
// any other argument are each refused as an operator error, never ignored. The registry
// validates the keyword's value before the plugin, and the check here still holds for a
// caller that reaches the plugin another way. Safe for concurrent use (it reads the set
// under mu).
//
// RFC 5838 Section 2: "Each Instance ID implies a separate OSPFv3 instance with its own
// neighbor adjacencies, link state database, protocol data structures, and shortest path
// first (SPF) computation."
func (m *v6EngineSet) selectInstances(command string, cmdArgs []string) ([]afInstance, error) {
	if len(cmdArgs) == 0 {
		afs := m.runningAFs()
		out := make([]afInstance, 0, len(afs))
		for _, af := range afs {
			if inst, ok := m.instanceFor(af); ok {
				out = append(out, inst)
			}
		}
		return out, nil
	}
	if len(cmdArgs) != 2 {
		return nil, unexpectedViewArgsError(command, cmdArgs)
	}
	if cmdArgs[0] != keywordAddressFamily {
		return nil, unexpectedViewArgsError(command, cmdArgs)
	}
	af, ok := afFromCanonicalName(cmdArgs[1])
	if !ok {
		return nil, unknownAFError(cmdArgs[1])
	}
	inst, ok := m.instanceFor(af)
	if !ok {
		return nil, afNotRunningError(af, m.runningAFs())
	}
	return []afInstance{inst}, nil
}

// unexpectedViewArgsError refuses arguments an OSPFv3 view does not take.
func unexpectedViewArgsError(command string, cmdArgs []string) error {
	problem := textbuf.New()
	defer problem.Release()
	problem.Str(command).Str(" does not take ").Str(strconv.Quote(strings.Join(cmdArgs, " ")))
	return opererr.New(problem.String(),
		"an OSPFv3 view takes no argument, which shows every running address family, "+
			"or address-family followed by one address family",
		command)
}

// unknownAFError refuses an address-family value that names no OSPFv3 address family, for
// a view and for `debug ipv6 ospf inject lsa` alike.
func unknownAFError(name string) error {
	problem := textbuf.New()
	defer problem.Release()
	problem.Str(strconv.Quote(name)).Str(" is not an OSPFv3 address family")
	return opererr.New(problem.String(),
		"the OSPFv3 address families are "+afNameList(afRangeOrder),
		cmdShowIPv6Instance)
}

// afNotRunningError refuses an address family that has no running engine, naming the ones
// that do, so the operator can pick one. A view and the inject command share it.
func afNotRunningError(af addressFamily, running []addressFamily) error {
	problem := textbuf.New()
	defer problem.Release()
	problem.Str("OSPFv3 address family ").Str(af.String()).Str(" is not running")
	why := "no OSPFv3 address family is running, so there is no OSPFv3 instance"
	if len(running) > 0 {
		why = "only a running address family has an OSPFv3 instance, and the running ones are " + afNameList(running)
	}
	return opererr.New(problem.String(), why, cmdShowIPv6Instance)
}

// afNameList spells afs as the config and the address-family keyword spell them.
func afNameList(afs []addressFamily) string {
	names := make([]string, 0, len(afs))
	for _, af := range afs {
		names = append(names, af.String())
	}
	return strings.Join(names, ", ")
}

// tag returns row led by this instance's identity.
func (a afInstance) tag(row any) afTaggedRow {
	return afTaggedRow{addressFamily: a.af, instanceID: a.instanceID, row: row}
}

// afTaggedRow is one OSPFv3 view row led by the identity of the address-family instance it
// came from. The row types are shared with OSPFv2, whose answers must not change, so the
// identity is added when the row is marshaled rather than as fields of those types.
type afTaggedRow struct {
	addressFamily addressFamily
	instanceID    uint8
	row           any
}

// MarshalJSON writes `address-family` and `instance-id` first, then the row's own keys in
// their order, so `| table` shows the identity as the first columns. A key of the row that
// repeats one of the two is dropped, because the engine's identity is the answer: the
// interface detail of a v6 engine carries instance-id 0 whatever its AF's Instance ID
// (instanceIDForV4). Every key and value is encoded by encoding/json; this only joins them.
func (r afTaggedRow) MarshalJSON() ([]byte, error) {
	body, err := json.Marshal(r.row)
	if err != nil {
		return nil, err
	}
	family, err := json.Marshal(r.addressFamily.String())
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	open, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if open != json.Delim('{') {
		return nil, errViewRowNotObject
	}
	out := make([]byte, 0, len(body)+len(family)+len(keywordAddressFamily)+len(fieldInstanceID)+16)
	out = append(out, `{"`+keywordAddressFamily+`":`...)
	out = append(out, family...)
	out = append(out, `,"`+fieldInstanceID+`":`...)
	out = strconv.AppendUint(out, uint64(r.instanceID), 10)
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, errViewRowNotObject
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		if key == keywordAddressFamily {
			continue
		}
		if key == fieldInstanceID {
			continue
		}
		name, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		out = append(out, ',')
		out = append(out, name...)
		out = append(out, ':')
		out = append(out, value...)
	}
	return append(out, '}'), nil
}

// afInstanceView identifies one running OSPFv3 address-family instance for the AF-aware show
// output (RFC 5838 §2 debugging requirement: show commands must identify the AF instance).
type afInstanceView struct {
	AddressFamily string `json:"address-family"`
	InstanceID    uint8  `json:"instance-id"`
	RouterID      string `json:"router-id"`
	Neighbors     int    `json:"neighbors"`
	Interfaces    int    `json:"interfaces"`
}

// afSummary lists every running OSPFv3 address-family instance with its AF, Instance ID,
// Router ID, and neighbor/interface counts (the `show ospf ipv6` payload).
func (m *v6EngineSet) afSummary() []afInstanceView {
	afs := m.runningAFs()
	out := make([]afInstanceView, 0, len(afs))
	for _, af := range afs {
		e, ok := m.engineFor(af)
		if !ok {
			continue
		}
		e.mu.Lock()
		rid := e.cfg.RouterID.String()
		e.mu.Unlock()
		out = append(out, afInstanceView{
			AddressFamily: af.String(),
			InstanceID:    e.dispatch.currentInstanceID(),
			RouterID:      rid,
			Neighbors:     len(e.neighborSnapshot()),
			Interfaces:    len(e.interfaceSnapshot()),
		})
	}
	return out
}

// v6InjectorAF is a redistribution injector bound to one address family. It resolves the
// live engine from the set on each call (rather than capturing a pointer), so a route
// injected before or after that AF's engine (re)spawns always reaches the current instance.
// When the AF is not configured, inject/withdraw are no-ops.
type v6InjectorAF struct {
	set *v6EngineSet
	af  addressFamily
}

var _ ospfredistribute.OptionalInjector = v6InjectorAF{}

// Active reports whether this AF's engine is currently running. When it is not, injectorFor
// skips this injector so IPv4-over-OSPFv3 redistribution falls back to the OSPFv2 engine and
// still originates a Type 5 (RFC 5838 review fix 1: no silent no-op with the AF unconfigured).
func (a v6InjectorAF) Active() bool {
	_, ok := a.set.engineFor(a.af)
	return ok
}

func (a v6InjectorAF) InjectExternal(prefix netip.Prefix, source string, ext ospfredistribute.External) error {
	if e, ok := a.set.engineFor(a.af); ok {
		return e.InjectExternal(prefix, source, ext)
	}
	return nil
}

func (a v6InjectorAF) WithdrawExternal(prefix netip.Prefix) (bool, error) {
	if e, ok := a.set.engineFor(a.af); ok {
		return e.WithdrawExternal(prefix)
	}
	return false, nil
}
