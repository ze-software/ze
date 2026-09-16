// Design: docs/architecture/diagnostics/path-mtu.md -- the run: targets, measurements, sizing, payload
// Related: mtu.go -- the handler, the grammar and the payload enums
// Related: search.go -- the per-target measurement this file drives
// Related: verdict.go -- the classification and the underlay advice this file applies
//
// The command is read-only. Nothing here, and nothing this file calls, writes
// configuration or kernel state: the remediation commands are strings in the
// payload for the operator to apply. The run answers ONE payload when every
// target has been measured. The RPC contract is one Response per command, and
// the streaming path the ping component uses (NewPingSession, reached through
// the `monitor` verb's session factories) is a different verb, so a per-peer
// progress row would need `monitor mtu`, which this spec does not add; the
// probe budget is bounded instead (runProbeBudget per target), so the worst
// case is computable.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/netip"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/ikeprobe"
	"github.com/ze-software/ze/internal/core/ipsecinventory"
	"github.com/ze-software/ze/internal/core/probe"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The keys the run adds to the payload beside the ones mtu.go declares.
const (
	fieldInventory      = "inventory"
	fieldTarget         = "target"
	fieldLabel          = "label"
	fieldOutcome        = "outcome"
	fieldPathMTU        = "path-mtu"
	fieldMethod         = "method"
	fieldProbes         = "probes"
	fieldLossy          = "lossy"
	fieldCachedPathMTU  = "cached-path-mtu"
	fieldProbesSent     = "probes-sent"
	fieldPayload        = "payload"
	fieldWire           = "wire"
	fieldReportedMTU    = "reported-mtu"
	fieldHost           = "host"
	fieldInterface      = "interface"
	fieldKind           = "kind"
	fieldMTU            = "mtu"
	fieldSource         = "source"
	fieldAdvice         = "advice"
	fieldPeer           = "peer"
	fieldRemote         = "remote"
	fieldMode           = "mode"
	fieldEncapsulation  = "encapsulation"
	fieldTransform      = "transform"
	fieldAssumed        = "assumed"
	fieldCurrentMTU     = "current-mtu"
	fieldCeiling        = "ceiling"
	fieldRecommended    = "recommended"
	fieldMSS            = "mss"
	fieldOctets         = "octets"
	fieldSized          = "sized"
	fieldReason         = "reason"
	fieldTCPMTUProbing  = "tcp-mtu-probing"
	fieldProber         = "prober"
	fieldIKEDeclined    = "ike-declined"
	fieldIKEConfirmed   = "ike-confirmed"
	fieldExchanges      = "exchanges"
	labelHost           = "host"
	labelReference      = "reference"
	labelPeer           = "peer"
	xfrmLinkType        = "xfrm"
	interfaceStateUp    = "up"
	underlaySourceRoute = "route to "
)

// icmpCaveat is what an ICMP-measured figure says about itself: ICMP echo is
// what was sent, so a path that treats ESP or UDP differently is not seen. A
// figure the IKE prober measured drops it and, without UDP encapsulation,
// carries ikePathCaveat instead: the exchange rode UDP/500 where ESP rides
// protocol 50, and a path can treat the two differently (AC-7). A NAT-T SA
// rides the path ESP-in-UDP rides, marker included, and carries no caveat.
const (
	icmpCaveat    = "the measurement is ICMP echo with Don't Fragment set; a path that treats ESP or UDP differently is not measured, so every figure is optimistic"
	ikePathCaveat = "the measurement is a padded IKE exchange on UDP/500 while ESP rides protocol 50, a different path when a middlebox treats the two differently; only a NAT-T tunnel measures the path ESP-in-UDP rides"
)

// proberKind names which prober produced a measurement's figure. The zero value
// is Unspecified so an unset field can never pass for an answer.
type proberKind uint8

const (
	proberUnspecified proberKind = iota
	// proberICMP: the ICMP echo prober (wireProber, search.go), the prober of a
	// host, of the reference address, and of a tunnel that is down.
	proberICMP
	// proberIKE: the padded INFORMATIONAL exchange over the peer's live IKE SA
	// (ikeProber, search.go), which confirmed the figure on the path ESP rides.
	proberIKE
)

// String answers the payload spelling. It is written and never compared.
func (k proberKind) String() string {
	switch k {
	case proberICMP:
		return "icmp"
	case proberIKE:
		return "ike"
	default:
		panic("BUG: proberKind written to the payload before it was set")
	}
}

// inventoryState says whether the IPsec inventory answered. The zero value is
// Unspecified so an unset field can never pass for an answer; a host run
// never consults the inventory and carries no inventory key at all.
type inventoryState uint8

const (
	inventoryUnspecified inventoryState = iota
	// inventoryRegistered: the IKE engine is linked and answered, with zero
	// or more tunnels.
	inventoryRegistered
	// inventoryNotRegistered: no IKE engine is linked into this build, which
	// is a different fact from a registered inventory holding no tunnel.
	inventoryNotRegistered
)

// String answers the wire spelling of the state. It is written into the
// payload and never compared.
func (s inventoryState) String() string {
	switch s {
	case inventoryRegistered:
		return "registered"
	case inventoryNotRegistered:
		return "not-registered"
	default:
		panic("BUG: inventoryState written to the payload before it was set")
	}
}

// targetProber is one open prober for one target: the search's prober plus
// the close its owner owes. *wireProber is the production one.
type targetProber interface {
	prober
	close() error
}

// xfrmInterface is one XFRM interface of the box: the name a tunnel's IfID
// resolves to, and the MTU and state the verdict reads.
type xfrmInterface struct {
	name string
	ifID uint32
	mtu  int
	up   bool
}

// mtuDeps are every read the run makes outside this package, in one struct
// so a test stands fakes in the place of the live inventory, the wire, the
// interface backend and the kernel state, and drives the whole run through
// handleShowMTU. liveDeps is the production set; each field names its
// producer.
type mtuDeps struct {
	tunnels         func() ([]ipsecinventory.Tunnel, error)
	openProber      func(ctx context.Context, target netip.Addr, df probe.DFMode) (targetProber, error)
	probeIKE        ikeprobe.Prober
	kernelPathMTU   func(ctx context.Context, target netip.Addr) (uint32, error)
	routeInterface  func(target netip.Addr) (string, error)
	getInterface    func(name string) (*iface.InterfaceInfo, error)
	xfrmInterfaces  func() ([]xfrmInterface, error)
	fragmentation   func() (fragmentationCounters, error)
	tcpMTUProbing   func() (tcpMTUProbing, error)
	routeMTUExpires func() (uint32, error)
}

// liveDeps is the production dependency set. It is a variable for the same
// reason the ping package's openProbeConn is one: a test replaces a field
// and reaches the run through the real command path without CAP_NET_RAW.
//
//nolint:gochecknoglobals // Injection seam, replaced only by tests.
var liveDeps = mtuDeps{
	tunnels:         ipsecinventory.Tunnels,
	openProber:      openLiveProber,
	probeIKE:        ikeprobe.Probe,
	kernelPathMTU:   probe.KernelPathMTU,
	routeInterface:  routeInterfaceOf,
	getInterface:    iface.GetInterface,
	xfrmInterfaces:  listXFRMInterfaces,
	fragmentation:   func() (fragmentationCounters, error) { return readFragmentationCounters(procMountPoint) },
	tcpMTUProbing:   readTCPMTUProbing,
	routeMTUExpires: readRouteMTUExpires,
}

// openLiveProber adapts openWireProber to the seam. The nil check keeps a
// failed open from becoming a non-nil interface holding a nil pointer.
func openLiveProber(ctx context.Context, target netip.Addr, df probe.DFMode) (targetProber, error) {
	w, err := openWireProber(ctx, target, df)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// routeInterfaceOf answers the interface the route to target leaves by, from
// the iface component's lookup (the netlink backend fills "interface" from
// the route's link, internal/plugins/iface/netlink/route_linux.go RouteLookup).
func routeInterfaceOf(target netip.Addr) (string, error) {
	// A daemon whose config carries no interface block has loaded no backend
	// yet; the kernel still has a route table, so the default backend is
	// loaded here rather than the read refused.
	if err := iface.EnsureBackend(); err != nil {
		return "", err
	}
	route, err := iface.RouteLookup(target)
	if err != nil {
		return "", err
	}
	name, ok := route[fieldInterface].(string)
	if !ok {
		return "", fmt.Errorf("mtu: the route to %s names no interface", target)
	}
	if name == "" {
		return "", fmt.Errorf("mtu: the route to %s names no interface", target)
	}
	return name, nil
}

// listXFRMInterfaces answers every XFRM interface with its if_id, read from
// the iface component: ListInterfaces for the type (the netlink backend
// reports link.Type(), "xfrm" for an XFRM interface) and GetXFRMInfo for the
// id. The list is bounded by the interfaces the box holds.
func listXFRMInterfaces() ([]xfrmInterface, error) {
	if err := iface.EnsureBackend(); err != nil {
		return nil, err
	}
	all, err := iface.ListInterfaces()
	if err != nil {
		return nil, err
	}
	var out []xfrmInterface
	for i := range all {
		if all[i].Type != xfrmLinkType {
			continue
		}
		info, err := iface.GetXFRMInfo(all[i].Name)
		if err != nil {
			return nil, fmt.Errorf("mtu: xfrm interface %s: %w", all[i].Name, err)
		}
		out = append(out, xfrmInterface{
			name: all[i].Name,
			ifID: info.IfID,
			mtu:  all[i].MTU,
			up:   all[i].State == interfaceStateUp,
		})
	}
	return out, nil
}

// measurement is one target's result: the label the payload shows, the
// search result, the error when the prober could not be opened or the search
// aborted, and the kernel's cached PMTU read BEFORE the probe. peer is the
// configured name of the tunnel whose IKE SA is up to this target, and empty
// when no live SA reaches it; udpEncap says whether that SA rides UDP
// encapsulation; prober names which prober the figure is from; result is the
// ICMP search, ikeResult the IKE search (its pathMTU is the figure the IKE
// prober reports: the ICMP figure it confirmed, or the smaller size it found
// after refuting it, measureByIKE), ikeConfirmed the largest wire size the
// padded exchange proved the path carries whole (set with prober ike) and
// exchanges what the IKE prober spent, the ended exchange included; and
// ikeDeclined says why the IKE prober was asked and produced no figure.
type measurement struct {
	target       netip.Addr
	label        string
	peer         string
	udpEncap     bool
	prober       proberKind
	ikeDeclined  string
	result       searchResult
	ikeResult    searchResult
	ikeConfirmed int
	exchanges    int
	err          error
	cache        pathMTUCache
}

// measured reports whether the target answered with a path MTU.
func (m *measurement) measured() bool {
	if m.err != nil {
		return false
	}
	return m.result.outcome == searchMeasured
}

// pathMTU is the measured path MTU in octets: the IKE figure once the IKE
// prober measured one, the ICMP figure otherwise. MUST be called only when
// measured() is true; the search bounds the figure by payloadMax plus the
// ICMP overhead, so it fits.
func (m *measurement) pathMTU() uint16 {
	figure := m.result.pathMTU
	if m.prober == proberIKE {
		figure = m.ikeResult.pathMTU
	}
	if figure > math.MaxUint16 {
		panic("BUG: a path MTU above 65535 was measured; the search bounds it by payloadMax")
	}
	return uint16(figure)
}

// caveats is what the figure cannot see, by the prober that produced it.
// MUST be called only when measured() is true.
func (m *measurement) caveats() []string {
	switch m.prober {
	case proberICMP:
		return []string{icmpCaveat}
	case proberIKE:
		if m.udpEncap {
			return []string{}
		}
		return []string{ikePathCaveat}
	case proberUnspecified:
		panic("BUG: caveats asked of a measurement with no prober")
	default:
		panic("BUG: caveats asked of an unknown prober")
	}
}

// mtuRun is the state of one run while runMTU builds it.
type mtuRun struct {
	ctx  context.Context
	req  *mtuRequest
	deps *mtuDeps

	inventory    inventoryState
	tunnels      []ipsecinventory.Tunnel
	measurements []measurement
	byTarget     map[netip.Addr]int
	reference    *measurement
	dfGateFailed bool
	notes        []stateNote
	commands     []string
	expires      uint32
}

// runMTU is the whole run: resolve the targets, read the underlay, measure
// each target then the reference, size each tunnel, advise on the underlay,
// read the local state, and answer the payload. It owns the control flow;
// the helpers compute and never decide.
func runMTU(ctx context.Context, req *mtuRequest, deps *mtuDeps) map[string]any {
	run := mtuRun{ctx: ctx, req: req, deps: deps, byTarget: map[netip.Addr]int{}, commands: []string{}}

	if req.host.IsValid() {
		run.addTarget(req.host, labelHost)
	} else {
		run.resolvePeerTargets()
	}

	underlay, underlayLink, underlayMTU := run.readUnderlay()

	run.measureTargets()
	if !req.host.IsValid() {
		run.measureReference()
	}

	payload := map[string]any{
		fieldStatus:       run.status().String(),
		fieldMeasurements: run.measurementRows(),
		fieldCaveats:      run.caveats(),
	}
	if underlay != nil {
		payload[fieldUnderlay] = underlay
	}
	if run.reference != nil {
		payload[fieldReference] = run.referenceRow()
	}

	tunnelRows := []map[string]any{}
	if run.inventory != inventoryUnspecified {
		payload[fieldInventory] = run.inventory.String()
	}
	if run.inventory == inventoryRegistered {
		var verdicts []tunnelVerdict
		var fault bool
		tunnelRows, verdicts, fault = run.sizeTunnels()
		payload[fieldVerdict] = runVerdictOf(verdicts, fault).String()
		if underlay != nil {
			run.adviseUnderlay(underlay, &underlayLink, underlayMTU)
		}
	}
	payload[fieldTunnels] = tunnelRows

	run.readLocalState(payload)

	payload[fieldCommands] = run.commands
	payload[fieldNotes] = noteRows(run.notes)
	return payload
}

// addTarget records one target once and answers its index; the same address
// reached through two tunnels is measured once.
func (r *mtuRun) addTarget(target netip.Addr, label string) int {
	if i, seen := r.byTarget[target]; seen {
		return i
	}
	r.byTarget[target] = len(r.measurements)
	r.measurements = append(r.measurements, measurement{target: target, label: label})
	return len(r.measurements) - 1
}

// resolvePeerTargets asks the inventory and derives one target per tunnel:
// the installed remote while the child is up (AC-17), else the configured
// remote when it is an address. A configured remote that is a hostname or
// `any` on a down tunnel is not probed; the tunnel is still listed.
func (r *mtuRun) resolvePeerTargets() {
	tunnels, err := r.deps.tunnels()
	if errors.Is(err, ipsecinventory.ErrNotRegistered) {
		r.inventory = inventoryNotRegistered
		r.note(noteSeverityCaution, "no IPsec inventory is registered: this build carries no IKE engine, so no tunnel exists to size; show mtu host <address> measures one path")
		return
	}
	if err != nil {
		r.inventory = inventoryNotRegistered
		r.note(noteSeverityFault, "the IPsec inventory could not be read: "+err.Error())
		return
	}
	r.inventory = inventoryRegistered
	r.tunnels = tunnels
	for i := range tunnels {
		target := tunnelTarget(&tunnels[i])
		if !target.IsValid() {
			continue
		}
		m := &r.measurements[r.addTarget(target, labelPeer)]
		// A live SA is what the IKE prober measures over. The first tunnel up to
		// an address names the SA; a second one to the same address shares the
		// figure, and one SA is enough to confirm it.
		if tunnels[i].Up && m.peer == "" {
			m.peer = tunnels[i].Peer
			m.udpEncap = tunnels[i].UDPEncap
		}
	}
}

// tunnelTarget is the address a tunnel's path is measured to: the installed
// endpoint while the child SA is up, the configured one otherwise, and the
// invalid Addr when neither is an address.
func tunnelTarget(t *ipsecinventory.Tunnel) netip.Addr {
	if t.Up {
		return t.InstalledRemote
	}
	return t.ConfiguredRemote
}

// readUnderlay names the interface the first target's route leaves by, and
// its MTU, the way the ported tool read the default route. With no target the
// route to the reference address stands in. The row names the source so an
// operator can tell which route was consulted; when the route or the
// interface cannot be read the row is nil and a note says why.
func (r *mtuRun) readUnderlay() (row map[string]any, link underlayLink, mtu uint16) {
	var routeTo netip.Addr
	if len(r.measurements) > 0 {
		routeTo = r.measurements[0].target
	} else {
		ref, err := referenceAddress()
		if err != nil {
			r.note(noteSeverityFault, "the underlay interface could not be found: "+err.Error())
			return nil, underlayLink{}, 0
		}
		routeTo = ref
	}
	name, err := r.deps.routeInterface(routeTo)
	if err != nil {
		r.note(noteSeverityFault, "the underlay interface could not be found: "+err.Error())
		return nil, underlayLink{}, 0
	}
	info, err := r.deps.getInterface(name)
	if err != nil {
		r.note(noteSeverityFault, "the underlay interface "+name+" could not be read: "+err.Error())
		return nil, underlayLink{}, 0
	}
	if info == nil {
		r.note(noteSeverityFault, "the underlay interface "+name+" could not be read: the interface backend answered nothing")
		return nil, underlayLink{}, 0
	}
	mtu, ok := interfaceMTUOctets(info.MTU)
	if !ok {
		var b textbuf.Buffer
		b.Str("the underlay interface ").Str(name).Str(" reports an MTU of ").Int(int64(info.MTU)).Str(", outside 1..65535")
		r.note(noteSeverityFault, b.String())
		return nil, underlayLink{}, 0
	}
	// The kind is the iface YANG list the link belongs to, which is what the
	// remediation command is spelled with; it is empty for a link type the
	// schema does not configure, and the advice then carries no command.
	link = underlayLink{name: name, kind: iface.CanonicalInterfaceType(info)}
	row = map[string]any{
		fieldInterface: name,
		fieldMTU:       mtu,
		fieldSource:    underlaySourceRoute + routeTo.String(),
	}
	if link.kind != "" {
		row[fieldKind] = link.kind
	}
	return row, link, mtu
}

// underlayLink is the underlay interface as the advice needs it: its name and
// the iface YANG list it belongs to.
type underlayLink struct {
	name string
	kind string
}

// interfaceMTUOctets narrows an interface MTU the backend reports as int to
// the octet count the arithmetic uses. The loopback reports 65536, which no
// tunnel rides, so it is refused rather than wrapped.
func interfaceMTUOctets(mtu int) (uint16, bool) {
	if mtu <= 0 {
		return 0, false
	}
	if mtu > math.MaxUint16 {
		return 0, false
	}
	return uint16(mtu), true
}

// measureTargets measures every target in order, one prober each, and stops
// at the first DF gate failure: a box whose Don't Fragment is not honored
// yields fragment sizes for every later probe, so nothing later is believed.
func (r *mtuRun) measureTargets() {
	for i := range r.measurements {
		if r.dfGateFailed {
			r.measurements[i].err = errors.New("not probed: an earlier target answered the Don't Fragment gate")
			continue
		}
		r.measure(&r.measurements[i])
	}
}

// measure runs one target: the cache read first, then the search. An error
// leaves the target unmeasurable and the run continues with the next (AC-7).
func (r *mtuRun) measure(m *measurement) {
	// ErrPathMTUUnknown is the kernel holding no entry, which earns no note;
	// any other error is a read that failed, and the cache note it would
	// have produced is replaced by one saying so rather than silently absent.
	cached, err := r.deps.kernelPathMTU(r.ctx, m.target)
	switch {
	case err == nil:
		m.cache = pathMTUCache{cached: cached, hasCached: true}
	case errors.Is(err, probe.ErrPathMTUUnknown):
	default:
		r.note(noteSeverityCaution, "the cached path MTU for "+m.target.String()+" could not be read, so the cache note is missing: "+err.Error())
	}
	df := r.dfMode()
	p, err := r.deps.openProber(r.ctx, m.target, df)
	if err != nil {
		m.err = err
		r.note(noteSeverityFault, m.target.String()+" could not be probed: "+err.Error())
		return
	}
	defer p.close() //nolint:errcheck // The socket is discarded; a close error changes nothing the run reports.

	m.prober = proberICMP
	m.result, m.err = searchPathMTU(r.ctx, p, m.target, r.req.exhaustive)
	if m.err != nil {
		r.note(noteSeverityFault, m.target.String()+" could not be measured: "+m.err.Error())
		return
	}
	switch m.result.outcome {
	case searchMeasured:
		r.cacheNote(m)
		if m.peer != "" {
			r.measureByIKE(m, df)
		}
	case searchUnmeasurable:
		r.note(noteSeverityFault, m.target.String()+" answered no probe at any size after "+probesText(m.result.probes)+"; it is unmeasurable")
	case searchDFGateFailed:
		r.dfGateFailed = true
		r.note(noteSeverityFault, m.target.String()+" answered a "+probesText(sanityPayload)+" payload with Don't Fragment set, so this box does not honor DF and no figure can be believed")
	default:
		panic("BUG: searchPathMTU answered with an unset outcome")
	}
}

// dfMode is the Don't Fragment mode of every probe in this run: the kernel's
// cached path MTU is honored on a default run and bypassed under `exhaustive`.
func (r *mtuRun) dfMode() probe.DFMode {
	if r.req.exhaustive {
		return probe.DFBypassCache
	}
	return probe.DFHonorCache
}

// measureByIKE offers a measured ICMP figure to the peer's live IKE SA.
// Confirm-first: the figure itself is asked, as a padded INFORMATIONAL
// exchange on the SA's own socket, and a fit makes it the IKE figure. A figure
// that is too big starts the same ladder-then-bisect search the ICMP path runs
// (pathSearch.refine), from a bracket whose top is the ICMP figure, so no
// exchange is ever asked above it: the ICMP figure is what the path carried
// whole for ICMP, and a size above it is the DF-clear copy a fragment-dropping
// path loses (A-6). A figure above ikeWireMax is not offered at all (AC-6).
// The attempt ends without a figure, and the ICMP figure stands unconfirmed
// with the reason in the row and a caution note, when the engine refuses by
// name, when the SA fails under an exchange (the first silent size is the last
// one asked, ikeProbeStopAtFirstSilence), when the exchange budget is spent
// before any size fitted, and when this build carries no engine. A budget
// spent after a size fitted is a coarser figure, not a missing one: the
// largest size the peer answered whole is a floor the path is proven to
// carry, which is what a tunnel is sized to, so the row reports it as the IKE
// figure and a note carries the bracket the search stopped inside.
//
// The IKE figure REPLACES the ICMP figure only when IKE refuted it: the ask
// answered too big, and the descent found a smaller size the peer answered
// whole. A fit at the ask, or at the largest size on a CBC suite's 16-octet
// grid below the ask, tested nothing above what it sent, so it refutes
// nothing: the ICMP figure stands as the path MTU, the row says the IKE
// prober confirmed it, and ike-confirmed carries the size the engine SENT and
// the peer answered whole (ikeProber.fitOctets). A caution note says when the
// grid made that size smaller than the ask.
func (r *mtuRun) measureByIKE(m *measurement, df probe.DFMode) {
	figure := m.result.pathMTU
	if figure > ikeWireMax {
		var b textbuf.Buffer
		b.Str("the ICMP figure ").Int(int64(figure)).Str(" is above ").Int(ikeWireMax).Str(" octets, the largest IKE message (RFC 7296 Section 2)")
		r.declineIKE(m, b.String())
		return
	}
	p := &ikeProber{peer: m.peer, overhead: icmpOverhead(m.target), df: df, ceiling: int(figure), ask: r.deps.probeIKE}
	s := pathSearch{ctx: r.ctx, prober: p, overhead: p.overhead}
	fitsAtAsk, err := s.attempt(int(figure) - s.overhead)
	if err == nil {
		if fitsAtAsk {
			s.result.outcome = searchMeasured
		} else {
			err = s.refine()
		}
	}
	m.ikeResult = s.result
	m.exchanges = p.exchanges
	settled := true
	if errors.Is(err, errIKEExchangeBudget) {
		if s.low > 0 {
			// A floor the peer answered whole is a figure, coarser than a
			// converged one: the bracket goes in a note.
			m.ikeResult.outcome = searchMeasured
			settled = false
			err = nil
		}
	}
	if m.ikeResult.outcome == searchMeasured {
		m.ikeResult.pathMTU = figure
		if !fitsAtAsk {
			m.ikeResult.pathMTU = uint32(p.fitOctets)
		}
	}
	if err != nil {
		// The text is the reason as its producer spelled it: a refusal names
		// its condition and a failed SA its outcome and the size (ikeProbeEnded,
		// search.go), the budget names itself (errIKEExchangeBudget), and an
		// unregistered leaf names itself (ikeprobe.ErrNotRegistered).
		r.declineIKE(m, err.Error())
		return
	}
	if m.ikeResult.outcome != searchMeasured {
		r.declineIKE(m, "no padded exchange of any size was answered whole")
		return
	}
	m.prober = proberIKE
	m.ikeConfirmed = p.fitOctets
	if !fitsAtAsk {
		var b textbuf.Buffer
		b.Str(m.target.String()).Str(": the padded IKE exchange measured ").Int(int64(m.ikeResult.pathMTU)).Str(" octets where ICMP measured ").Int(int64(figure))
		r.note(noteSeverityInfo, b.String())
	}
	if p.fitOctets != p.fitAsked {
		var b textbuf.Buffer
		b.Str(m.target.String()).Str(": the SA's cipher suite sends on a block grid, so the exchange asked at ").Int(int64(p.fitAsked)).Str(" octets was sent at ").Int(int64(p.fitOctets)).Str("; IKE confirmed the path down to ").Int(int64(p.fitOctets)).Str(" octets")
		r.note(noteSeverityCaution, b.String())
	}
	if !settled {
		var b textbuf.Buffer
		b.Str(m.target.String()).Str(": the budget of ").Int(ikeExchangesPerRunMax).Str(" IKE exchanges was spent before the figure settled between ").Int(int64(m.ikeResult.pathMTU)).Str(" and ").Int(int64(s.high + s.overhead)).Str(" octets; the size the peer answered whole is reported")
		r.note(noteSeverityCaution, b.String())
	}
}

// declineIKE records that the IKE prober was asked and produced no figure:
// the ICMP figure and its prober stay, the row carries why, and a caution
// note says the figure is unconfirmed.
func (r *mtuRun) declineIKE(m *measurement, reason string) {
	m.ikeDeclined = reason
	r.note(noteSeverityCaution, m.target.String()+" keeps its ICMP figure, unconfirmed: "+reason)
}

// caveats is what the run's figures cannot see: the ICMP caveat while any
// figure in the payload, a measurement or the reference, is ICMP-measured.
// Each measurement row carries its own; this is the run's summary.
func (r *mtuRun) caveats() []string {
	for i := range r.measurements {
		m := &r.measurements[i]
		if !m.measured() {
			continue
		}
		if m.prober == proberICMP {
			return []string{icmpCaveat}
		}
	}
	if r.reference == nil {
		return []string{}
	}
	if !r.reference.measured() {
		return []string{}
	}
	if r.reference.prober == proberICMP {
		return []string{icmpCaveat}
	}
	return []string{}
}

// probesText spells a count for a note.
func probesText(n int) string {
	var b textbuf.Buffer
	b.Int(int64(n))
	if n == 1 {
		return b.String() + " probe"
	}
	return b.String() + " probes"
}

// cacheNote adds the AC-11 note for a measured target: the cached PMTU read
// before the probe, against what the wire answered.
func (r *mtuRun) cacheNote(m *measurement) {
	if !m.cache.hasCached {
		return
	}
	if r.expires == 0 {
		expires, err := r.deps.routeMTUExpires()
		if err == nil {
			r.expires = expires
		}
	}
	note, ok := pathMTUCacheFinding(m.target, &m.cache, m.pathMTU(), r.req.exhaustive, r.expires)
	if ok {
		r.notes = append(r.notes, note)
	}
}

// measureReference measures the reference address on a default run, unless
// it is already a target, in which case that measurement is the reference.
func (r *mtuRun) measureReference() {
	ref, err := referenceAddress()
	if err != nil {
		panic("BUG: handleShowMTU validated the reference address before the run")
	}
	if i, seen := r.byTarget[ref]; seen {
		r.reference = &r.measurements[i]
		return
	}
	if r.dfGateFailed {
		return
	}
	m := measurement{target: ref, label: labelReference}
	r.measure(&m)
	r.reference = &m
	if !m.measured() {
		r.note(noteSeverityCaution, "the reference address "+ref.String()+" did not answer, so the underlay advice is undecidable")
	}
}

// tightestPeerPath is the lowest path MTU measured to any peer target, and
// false when no peer target was measured. The reference is not a peer.
func (r *mtuRun) tightestPeerPath() (uint16, bool) {
	var tightest uint16
	found := false
	for i := range r.measurements {
		m := &r.measurements[i]
		if m.label != labelPeer {
			continue
		}
		if !m.measured() {
			continue
		}
		if !found {
			tightest = m.pathMTU()
			found = true
			continue
		}
		tightest = min(tightest, m.pathMTU())
	}
	return tightest, found
}

// tunnelPath is the path MTU a tunnel is sized from: its own target's
// measurement when it answered; else the tightest measured peer path,
// marked assumed. The third result is false when nothing was measured.
func (r *mtuRun) tunnelPath(t *ipsecinventory.Tunnel) (mtu uint16, assumed, ok bool) {
	target := tunnelTarget(t)
	if i, seen := r.byTarget[target]; seen {
		m := &r.measurements[i]
		if m.measured() {
			return m.pathMTU(), false, true
		}
	}
	tightest, found := r.tightestPeerPath()
	if !found {
		return 0, false, false
	}
	return tightest, true, true
}

// sizeTunnels builds one row per tunnel, applies the arithmetic and the
// verdict where the tunnel can be sized, and collects the remediation
// commands. It answers the rows, the verdicts the run ladder reads, and
// whether a fault outside the table (a refused transform) was met.
func (r *mtuRun) sizeTunnels() ([]map[string]any, []tunnelVerdict, bool) {
	rows := make([]map[string]any, 0, len(r.tunnels))
	var verdicts []tunnelVerdict
	fault := false
	xfrms, err := r.deps.xfrmInterfaces()
	if err != nil {
		r.note(noteSeverityFault, "the XFRM interfaces could not be listed: "+err.Error())
		fault = true
	}
	for i := range r.tunnels {
		t := &r.tunnels[i]
		row := tunnelRow(t)
		verdict, sized := r.sizeTunnel(t, xfrms, row)
		row[fieldSized] = sized
		if verdict != tunnelVerdictUnspecified {
			verdicts = append(verdicts, verdict)
		}
		if reason, refused := row[fieldReason].(string); refused {
			if sizedReasonIsFault(reason) {
				fault = true
			}
		}
		rows = append(rows, row)
	}
	return rows, verdicts, fault
}

// refusedTransformReason prefixes the one not-sized reason that is a fault
// outside the table: a transform the arithmetic refuses to advise on.
const refusedTransformReason = "transform refused: "

func sizedReasonIsFault(reason string) bool {
	return len(reason) >= len(refusedTransformReason) && reason[:len(refusedTransformReason)] == refusedTransformReason
}

// tunnelRow is the part of a tunnel's row every tunnel carries.
func tunnelRow(t *ipsecinventory.Tunnel) map[string]any {
	row := map[string]any{
		fieldPeer: t.Peer,
	}
	if target := tunnelTarget(t); target.IsValid() {
		row[fieldRemote] = target.String()
	}
	if !t.Up {
		return row
	}
	row[fieldMode] = t.Mode.String()
	row[fieldEncapsulation] = t.UDPEncap
	row[fieldTransform] = t.EncryptionName + "/" + t.IntegrityName
	return row
}

// sizeTunnel applies the verdict order to one tunnel: down before anything,
// then the reasons a tunnel cannot be sized, then the arithmetic and the
// classification. It answers the verdict (Unspecified when the tunnel is not
// classified) and whether the row carries figures.
func (r *mtuRun) sizeTunnel(t *ipsecinventory.Tunnel, xfrms []xfrmInterface, row map[string]any) (tunnelVerdict, bool) {
	if !t.Up {
		row[fieldVerdict] = tunnelVerdictDown.String()
		row[fieldReason] = "no child SA is installed"
		return tunnelVerdictDown, false
	}
	if t.IfID == 0 {
		row[fieldReason] = "the child SA is policy-based and bound to no interface"
		return tunnelVerdictUnspecified, false
	}
	xfrm, found := xfrmByID(xfrms, t.IfID)
	if !found {
		var b textbuf.Buffer
		b.Str("no xfrm interface carries if_id ").Uint32(t.IfID)
		row[fieldReason] = b.String()
		return tunnelVerdictUnspecified, false
	}
	row[fieldInterface] = xfrm.name
	if !xfrm.up {
		row[fieldVerdict] = tunnelVerdictDown.String()
		row[fieldReason] = "interface " + xfrm.name + " is down"
		return tunnelVerdictDown, false
	}
	current, ok := interfaceMTUOctets(xfrm.mtu)
	if !ok {
		var b textbuf.Buffer
		b.Str("interface ").Str(xfrm.name).Str(" reports an MTU of ").Int(int64(xfrm.mtu)).Str(", outside 1..65535")
		row[fieldReason] = b.String()
		return tunnelVerdictUnspecified, false
	}
	row[fieldCurrentMTU] = current
	inner := innerFamily(t)
	if inner == ipFamilyUnspecified {
		row[fieldReason] = "the child SA carries no traffic selector, so its inner family is unknown"
		return tunnelVerdictUnspecified, false
	}
	overhead, err := deriveESPOverhead(t)
	if err != nil {
		row[fieldReason] = refusedTransformReason + err.Error()
		r.note(noteSeverityFault, "peer "+t.Peer+": "+err.Error()+"; its figures are not advised")
		return tunnelVerdictUnspecified, false
	}
	path, assumed, measured := r.tunnelPath(t)
	if !measured {
		row[fieldReason] = "no path to any peer was measured"
		return tunnelVerdictUnspecified, false
	}
	row[fieldPathMTU] = path
	row[fieldAssumed] = assumed

	ceil := ceiling(path, &overhead)
	row[fieldCeiling] = ceil
	sizing := tunnelSizing{up: true, current: current, ceiling: ceil}
	if rec, recErr := recommended(ceil, &overhead, inner); recErr == nil {
		sizing.recommended, sizing.hasRecommended = rec, true
		row[fieldRecommended] = rec
		if m, mssErr := mss(rec, inner); mssErr == nil {
			row[fieldMSS] = m
		}
	}
	verdict, octets := classifyTunnel(&sizing)
	row[fieldVerdict] = verdict.String()
	row[fieldOctets] = octets
	if verdict.needsCommand() {
		r.commands = append(r.commands, tunnelCommand(xfrm.name, sizing.recommended))
	}
	return verdict, true
}

// xfrmByID finds the interface a tunnel's if_id names.
func xfrmByID(xfrms []xfrmInterface, ifID uint32) (xfrmInterface, bool) {
	for i := range xfrms {
		if xfrms[i].ifID == ifID {
			return xfrms[i], true
		}
	}
	return xfrmInterface{}, false
}

// innerFamily is the family of the traffic the tunnel carries, read from the
// remote selector and then the local one; Unspecified when the child holds
// neither, which no sizing can proceed from.
func innerFamily(t *ipsecinventory.Tunnel) ipFamily {
	if t.TSRemote.IsValid() {
		return familyOf(t.TSRemote.Addr())
	}
	if t.TSLocal.IsValid() {
		return familyOf(t.TSLocal.Addr())
	}
	return ipFamilyUnspecified
}

// adviseUnderlay applies the advice matrix once a peer path was measured,
// writes the outcome into the underlay row and its sentence into the notes,
// and appends the command when the matrix produced one.
func (r *mtuRun) adviseUnderlay(row map[string]any, link *underlayLink, current uint16) {
	tightest, found := r.tightestPeerPath()
	if !found {
		return
	}
	in := underlayInput{iface: link.name, kind: link.kind, current: current, tightest: tightest}
	if r.reference != nil {
		if r.reference.measured() {
			in.hasReference = true
			in.reference = r.reference.pathMTU()
			in.referenceHost = r.reference.target
		}
	}
	advice := adviseUnderlay(&in)
	row[fieldAdvice] = advice.outcome.String()
	r.notes = append(r.notes, stateNote{severity: advice.outcome.severity(), text: advice.outcome.text(&in, advice.mtu)})
	if advice.command != "" {
		r.commands = append(r.commands, advice.command)
	}
}

// readLocalState adds the fragmentation findings and the tcp_mtu_probing
// state. A counter set that cannot be read is a fault note, never a zero.
func (r *mtuRun) readLocalState(payload map[string]any) {
	counters, err := r.deps.fragmentation()
	if err != nil {
		r.note(noteSeverityFault, "the fragmentation counters could not be read: "+err.Error())
	} else {
		r.notes = append(r.notes, counters.findings()...)
	}
	probing, err := r.deps.tcpMTUProbing()
	if err != nil {
		r.note(noteSeverityFault, "tcp_mtu_probing could not be read: "+err.Error())
		return
	}
	payload[fieldTCPMTUProbing] = probing.String()
	if note, ok := probing.finding(); ok {
		r.notes = append(r.notes, note)
	}
}

// status is the run outcome: the DF gate beats everything, then whether any
// target answered.
func (r *mtuRun) status() runStatus {
	if r.dfGateFailed {
		return runStatusDFGateFailed
	}
	for i := range r.measurements {
		if r.measurements[i].measured() {
			return runStatusOK
		}
	}
	if r.reference != nil {
		if r.reference.measured() {
			return runStatusOK
		}
	}
	return runStatusNothingMeasured
}

// measurementRows renders every target. path-mtu and method are present only
// under a measured outcome; an error names itself under outcome.
func (r *mtuRun) measurementRows() []map[string]any {
	rows := make([]map[string]any, 0, len(r.measurements))
	for i := range r.measurements {
		rows = append(rows, r.measurementRow(&r.measurements[i]))
	}
	return rows
}

func (r *mtuRun) measurementRow(m *measurement) map[string]any {
	row := map[string]any{
		fieldTarget: m.target.String(),
		fieldLabel:  m.label,
	}
	if m.cache.hasCached {
		row[fieldCachedPathMTU] = m.cache.cached
	}
	if m.err != nil {
		row[fieldOutcome] = "error"
		row[fieldReason] = m.err.Error()
		// The prober is unset when it could not be opened, and no prober is
		// what the row then says.
		if m.prober != proberUnspecified {
			row[fieldProber] = m.prober.String()
		}
		return row
	}
	row[fieldOutcome] = m.result.outcome.String()
	// probes counts the ICMP probes, as it always did; the IKE exchanges are
	// their own count, present once the IKE step ran for a live SA.
	row[fieldProbes] = m.result.probes
	row[fieldLossy] = m.result.lossy
	row[fieldProber] = m.prober.String()
	if m.ikeDeclined != "" {
		row[fieldIKEDeclined] = m.ikeDeclined
	}
	if m.result.outcome == searchMeasured {
		row[fieldPathMTU] = m.pathMTU()
		row[fieldMethod] = m.result.method.String()
		row[fieldCaveats] = m.caveats()
		if m.peer != "" {
			row[fieldExchanges] = m.exchanges
		}
		if m.prober == proberIKE {
			row[fieldIKEConfirmed] = m.ikeConfirmed
		}
	}
	if r.req.detail {
		row[fieldProbesSent] = probeRows(m)
	}
	return row
}

// probeRows is the detail view of one measurement: every probe sent, its
// payload and wire size, and what came back.
func probeRows(m *measurement) []map[string]any {
	overhead := icmpOverhead(m.target)
	rows := make([]map[string]any, 0, len(m.result.records))
	for i := range m.result.records {
		rec := &m.result.records[i]
		row := map[string]any{
			fieldPayload: rec.payload,
			fieldWire:    rec.payload + overhead,
			fieldOutcome: rec.answer.outcome.String(),
		}
		if rec.answer.mtu != 0 {
			row[fieldReportedMTU] = rec.answer.mtu
		}
		rows = append(rows, row)
	}
	return rows
}

// referenceRow renders the reference measurement.
func (r *mtuRun) referenceRow() map[string]any {
	row := map[string]any{fieldHost: r.reference.target.String()}
	if r.reference.measured() {
		row[fieldPathMTU] = r.reference.pathMTU()
		row[fieldMethod] = r.reference.result.method.String()
		row[fieldProber] = r.reference.prober.String()
		return row
	}
	if r.reference.err != nil {
		row[fieldOutcome] = "error"
		return row
	}
	row[fieldOutcome] = r.reference.result.outcome.String()
	return row
}

// note appends one note.
func (r *mtuRun) note(severity noteSeverity, text string) {
	r.notes = append(r.notes, stateNote{severity: severity, text: text})
}

// noteRows renders the notes.
func noteRows(notes []stateNote) []map[string]any {
	rows := make([]map[string]any, 0, len(notes))
	for i := range notes {
		rows = append(rows, map[string]any{
			fieldSeverity: notes[i].severity.String(),
			fieldText:     notes[i].text,
		})
	}
	return rows
}
