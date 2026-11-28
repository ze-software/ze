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

	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/ikeprobe"
	"github.com/ze-software/ze/internal/core/ipsecinventory"
	"github.com/ze-software/ze/internal/core/probe"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The keys the run adds to the payload beside the ones mtu.go declares.
const (
	fieldInventory           = "inventory"
	fieldTarget              = "target"
	fieldLabel               = "label"
	fieldOutcome             = "outcome"
	fieldPathMTU             = "path-mtu"
	fieldMethod              = "method"
	fieldProbes              = "probes"
	fieldLossy               = "lossy"
	fieldCachedPathMTU       = "cached-path-mtu"
	fieldProbesSent          = "probes-sent"
	fieldPayload             = "payload"
	fieldWire                = "wire"
	fieldReportedMTU         = "reported-mtu"
	fieldHost                = "host"
	fieldInterface           = "interface"
	fieldKind                = "kind"
	fieldMTU                 = "mtu"
	fieldSource              = "source"
	fieldAdvice              = "advice"
	fieldPeer                = "peer"
	fieldRemote              = "remote"
	fieldMode                = "mode"
	fieldEncapsulation       = "encapsulation"
	fieldTransform           = "transform"
	fieldOverhead            = "overhead"
	fieldAssumed             = "assumed"
	fieldCurrentMTU          = "current-mtu"
	fieldCeiling             = "ceiling"
	fieldRecommended         = "recommended"
	fieldMSS                 = "mss"
	fieldOctets              = "octets"
	fieldSized               = "sized"
	fieldReason              = "reason"
	fieldTCPMTUProbing       = "tcp-mtu-probing"
	fieldProber              = "prober"
	fieldIKEDeclined         = "ike-declined"
	fieldIKEConfirmed        = "ike-confirmed"
	fieldExchanges           = "exchanges"
	labelHost                = "host"
	labelReference           = "reference"
	labelPeer                = "peer"
	xfrmLinkType             = "xfrm"
	tunnelKindIPsec          = "ipsec"
	tunnelKindTunnel         = "tunnel"
	tunnelKindWireguard      = "wireguard"
	interfaceStateUp         = "up"
	underlaySourceRoute      = "route to "
	noPeerPathMeasuredReason = "no path to any peer was measured"
	notAdvisedSuffix         = "; its figures are not advised"
)

// icmpCaveat is what an ICMP-measured figure says about itself: ICMP echo is
// what was sent, so a path that treats ESP, GRE or UDP differently is not seen.
// A figure the IKE prober measured drops it and, without UDP encapsulation,
// carries ikePathCaveat instead: the exchange rode UDP/500 where ESP rides
// protocol 50, and a path can treat the two differently (AC-7). A NAT-T SA
// rides the path ESP-in-UDP rides, marker included, and carries no caveat.
const (
	icmpCaveat    = "the measurement is ICMP echo with Don't Fragment set; a path that treats ESP, GRE or UDP differently is not measured, so every figure is optimistic"
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
	case proberUnspecified:
		panic("BUG: proberKind written to the payload before it was set")
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
	case inventoryUnspecified:
		panic("BUG: inventoryState written to the payload before it was set")
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

// nonIPsecTunnel is one non-IPsec tunnel interface read from the iface
// backend (gre, gretap, ip6gre, ip6gretap, vxlan, ipip, sit, ip6tnl, ipip6).
type nonIPsecTunnel struct {
	name    string
	mtu     int
	up      bool
	addrs   []iface.AddrInfo
	info    iface.TunnelInfo
	readErr error
}

// wireguardPeer holds only the non-secret fields of one WireGuard peer needed
// for endpoint probing and inner family detection. PrivateKey and PresharedKey
// are never copied out of iface.WireguardSpec.
type wireguardPeer struct {
	name       string
	endpoint   netip.Addr
	allowedIPs []netip.Prefix
}

// wireguardInterface is one WireGuard interface read from the iface backend.
type wireguardInterface struct {
	name    string
	mtu     int
	up      bool
	addrs   []iface.AddrInfo
	peers   []wireguardPeer
	readErr error
}

// tunnelSnapshot holds every XFRM, non-IPsec tunnel and WireGuard interface
// collected from a single iface.ListInterfaces pass.
type tunnelSnapshot struct {
	xfrms      []xfrmInterface
	tunnels    []nonIPsecTunnel
	wireguards []wireguardInterface
}

// mtuDeps are every read the run makes outside this package, in one struct
// so a test stands fakes in the place of the live inventory, the wire, the
// interface backend and the kernel state, and drives the whole run through
// handleShowMTU. liveDeps is the production set; each field names its
// producer.
type mtuDeps struct {
	tunnels        func() ([]ipsecinventory.Tunnel, error)
	openProber     func(ctx context.Context, target netip.Addr, df probe.DFMode) (targetProber, error)
	probeIKE       ikeprobe.Prober
	kernelPathMTU  func(ctx context.Context, target netip.Addr) (uint32, error)
	routeInterface func(target netip.Addr) (string, error)
	// getLink reads the underlay's link facts (MTU, kind) without its
	// addresses, so a failed address dump cannot hide the underlay.
	getLink         func(name string) (*iface.InterfaceInfo, error)
	tunnelSnapshot  func() (tunnelSnapshot, error)
	fragmentation   func() (fragmentationCounters, error)
	tcpMTUProbing   func() (tcpMTUProbing, error)
	routeMTUExpires func() (uint32, error)
	// nameServers answers the system name servers of the running config and
	// whether any were published; the reference address defaults to the first.
	nameServers func() ([]string, bool)
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
	getLink:         iface.GetLink,
	tunnelSnapshot:  listTunnelSnapshot,
	fragmentation:   func() (fragmentationCounters, error) { return readFragmentationCounters(procMountPoint) },
	tcpMTUProbing:   readTCPMTUProbing,
	routeMTUExpires: readRouteMTUExpires,
	nameServers:     system.NameServers,
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
// the iface component's lookup (the kernel backend fills "interface" from
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

// listTunnelSnapshot collects every XFRM, non-IPsec tunnel and WireGuard
// interface from one iface.ListInterfaces call. Per-device readback failures
// on non-IPsec tunnel or WireGuard links are recorded on that device so the
// rest of the box is still sized.
func listTunnelSnapshot() (tunnelSnapshot, error) {
	if err := iface.EnsureBackend(); err != nil {
		return tunnelSnapshot{}, err
	}
	all, err := iface.ListInterfaces()
	if err != nil {
		return tunnelSnapshot{}, err
	}
	var snap tunnelSnapshot
	for i := range all {
		link := &all[i]
		switch iface.CanonicalInterfaceType(link) {
		case xfrmLinkType:
			info, infoErr := iface.GetXFRMInfo(link.Name)
			if infoErr != nil {
				return tunnelSnapshot{}, fmt.Errorf("mtu: xfrm interface %s: %w", link.Name, infoErr)
			}
			snap.xfrms = append(snap.xfrms, xfrmInterface{
				name: link.Name,
				ifID: info.IfID,
				mtu:  link.MTU,
				up:   link.State == interfaceStateUp,
			})
		case tunnelKindTunnel:
			tun := nonIPsecTunnel{
				name:  link.Name,
				mtu:   link.MTU,
				up:    link.State == interfaceStateUp,
				addrs: link.Addresses,
			}
			tun.info, tun.readErr = iface.GetTunnelInfo(link.Name)
			if tun.readErr == nil {
				if isDormantKernelFallbackTunnel(link.Name, tun.up, &tun.info) {
					continue
				}
			}
			snap.tunnels = append(snap.tunnels, tun)
		case tunnelKindWireguard:
			wg := wireguardInterface{
				name:  link.Name,
				mtu:   link.MTU,
				up:    link.State == interfaceStateUp,
				addrs: link.Addresses,
			}
			spec, specErr := iface.GetWireguardDevice(link.Name)
			if specErr == nil {
				wg.peers = wireguardPeersFromSpec(&spec)
			} else {
				wg.readErr = specErr
			}
			snap.wireguards = append(snap.wireguards, wg)
		}
	}
	return snap, nil
}

// isDormantKernelFallbackTunnel reports whether a netlink tunnel link is one
// of the Linux kernel's auto-created dormant fallback devices (tunl0, sit0,
// gre0, gretap0, ip6tnl0, ip6gre0) that exist in every network namespace when
// those tunnel modules are loaded: admin-down, no local or remote endpoint,
// no GRE options, and not flow-based. Any configured or admin-up device with
// one of those names is kept.
func isDormantKernelFallbackTunnel(name string, up bool, info *iface.TunnelInfo) bool {
	if up {
		return false
	}
	switch name {
	case "tunl0", "sit0", "gre0", "gretap0", "ip6tnl0", "ip6gre0":
	default:
		return false
	}
	if info.FlowBased {
		return false
	}
	if info.GREKey {
		return false
	}
	if info.GREChecksum {
		return false
	}
	if info.GRESequence {
		return false
	}
	if info.Local.IsValid() {
		if !info.Local.Unmap().IsUnspecified() {
			return false
		}
	}
	if info.Remote.IsValid() {
		if !info.Remote.Unmap().IsUnspecified() {
			return false
		}
	}
	return true
}

// wireguardPeersFromSpec copies only the peer name/public-key label, endpoint
// IP, and AllowedIPs prefixes out of spec. PrivateKey and PresharedKey are
// never retained.
func wireguardPeersFromSpec(spec *iface.WireguardSpec) []wireguardPeer {
	peers := make([]wireguardPeer, 0, len(spec.Peers))
	for i := range spec.Peers {
		src := &spec.Peers[i]
		name := src.Name
		if name == "" {
			name = src.PublicKey.String()
		}
		var ep netip.Addr
		if src.EndpointIP != "" {
			if parsed, err := netip.ParseAddr(src.EndpointIP); err == nil {
				ep = parsed.Unmap()
			}
		}
		allowed := make([]netip.Prefix, 0, len(src.AllowedIPs))
		for _, cidr := range src.AllowedIPs {
			if prefix, err := netip.ParsePrefix(cidr); err == nil {
				allowed = append(allowed, prefix)
			}
		}
		peers = append(peers, wireguardPeer{
			name:       name,
			endpoint:   ep,
			allowedIPs: allowed,
		})
	}
	return peers
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

	inventory     inventoryState
	tunnels       []ipsecinventory.Tunnel
	snapshot      tunnelSnapshot
	snapshotFault bool
	measurements  []measurement
	byTarget      map[netip.Addr]int
	reference     *measurement
	dfGateFailed  bool
	notes         []stateNote
	commands      []string
	expires       uint32
}

// hasNonIPsecTunnels reports whether the snapshot discovered any non-IPsec
// tunnel or WireGuard interface.
func (r *mtuRun) hasNonIPsecTunnels() bool {
	if len(r.snapshot.tunnels) > 0 {
		return true
	}
	return len(r.snapshot.wireguards) > 0
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
	if run.inventory == inventoryRegistered || run.hasNonIPsecTunnels() {
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

// isProbeableRemote reports whether addr names one unicast remote endpoint
// that the ICMP prober may send to: valid, non-unspecified, non-multicast.
func isProbeableRemote(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	unmapped := addr.Unmap()
	if unmapped.IsUnspecified() {
		return false
	}
	return !unmapped.IsMulticast()
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

// resolvePeerTargets snapshots interface tunnels (XFRM, non-IPsec tunnels and
// WireGuard) once, queries the IPsec inventory, and adds every probeable
// unicast remote endpoint to the deduplicated peer target list.
func (r *mtuRun) resolvePeerTargets() {
	snap, snapErr := r.deps.tunnelSnapshot()
	if snapErr == nil {
		r.snapshot = snap
	} else {
		r.snapshotFault = true
		r.note(noteSeverityFault, "the tunnel interfaces could not be listed: "+snapErr.Error())
	}

	tunnels, err := r.deps.tunnels()
	switch {
	case errors.Is(err, ipsecinventory.ErrNotRegistered):
		r.inventory = inventoryNotRegistered
		if r.hasNonIPsecTunnels() {
			r.note(noteSeverityCaution, "no IPsec inventory is registered: this build carries no IKE engine, so IPsec tunnels cannot be checked; non-IPsec tunnels are still sized")
		} else {
			r.note(noteSeverityCaution, "no IPsec inventory is registered: this build carries no IKE engine, so no tunnel exists to size; show mtu host <address> measures one path")
		}
	case err != nil:
		r.inventory = inventoryNotRegistered
		r.note(noteSeverityFault, "the IPsec inventory could not be read: "+err.Error())
	default:
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

	for i := range r.snapshot.tunnels {
		tun := &r.snapshot.tunnels[i]
		if tun.readErr != nil {
			continue
		}
		if tun.info.FlowBased {
			continue
		}
		if !isProbeableRemote(tun.info.Remote) {
			continue
		}
		r.addTarget(tun.info.Remote.Unmap(), labelPeer)
	}
	for i := range r.snapshot.wireguards {
		wg := &r.snapshot.wireguards[i]
		if wg.readErr != nil {
			continue
		}
		for j := range wg.peers {
			ep := wg.peers[j].endpoint
			if !isProbeableRemote(ep) {
				continue
			}
			r.addTarget(ep.Unmap(), labelPeer)
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
		ref, err := referenceAddress(r.deps)
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
	info, err := r.deps.getLink(name)
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
	case searchOutcomeUnspecified:
		panic("BUG: searchPathMTU answered with an unset outcome")
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
	ref, err := referenceAddress(r.deps)
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

// targetPath is the path MTU one target address is sized from: its own
// measurement when it answered; else the tightest measured peer path,
// marked assumed. The third result is false when nothing was measured.
func (r *mtuRun) targetPath(target netip.Addr) (mtu uint16, assumed, ok bool) {
	if target.IsValid() {
		if i, seen := r.byTarget[target.Unmap()]; seen {
			m := &r.measurements[i]
			if m.measured() {
				return m.pathMTU(), false, true
			}
		}
	}
	tightest, found := r.tightestPeerPath()
	if !found {
		return 0, false, false
	}
	return tightest, true, true
}

// tunnelPath is the path MTU an IPsec tunnel is sized from.
func (r *mtuRun) tunnelPath(t *ipsecinventory.Tunnel) (mtu uint16, assumed, ok bool) {
	return r.targetPath(tunnelTarget(t))
}

// tunnelSizingInput carries the inputs shared by IPsec, non-IPsec tunnel, and
// WireGuard sizing once path MTU, overhead, and inner family are known.
type tunnelSizingInput struct {
	cmdKind   string
	ifaceName string
	current   uint16
	path      uint16
	assumed   bool
	overhead  encapOverhead
	inner     ipFamily
	emitMSS   bool
}

// wgCandidate holds the evaluated path and ceiling for one WireGuard peer.
type wgCandidate struct {
	peer     *wireguardPeer
	path     uint16
	assumed  bool
	overhead encapOverhead
	ceiling  uint16
}

// applyTunnelSizing computes ceiling, recommended, optional MSS, verdict,
// octets, and remediation command for one tunnel and writes them into row.
func (r *mtuRun) applyTunnelSizing(row map[string]any, in *tunnelSizingInput) tunnelVerdict {
	row[fieldOverhead] = in.overhead.octets()
	row[fieldPathMTU] = in.path
	row[fieldAssumed] = in.assumed

	ceil := ceiling(in.path, &in.overhead)
	row[fieldCeiling] = ceil
	sizing := tunnelSizing{up: true, current: in.current, ceiling: ceil}
	if rec, recErr := recommended(ceil, &in.overhead, in.inner); recErr == nil {
		sizing.recommended, sizing.hasRecommended = rec, true
		row[fieldRecommended] = rec
		if in.emitMSS {
			if m, mssErr := mss(rec, in.inner); mssErr == nil {
				row[fieldMSS] = m
			}
		}
	}
	verdict, octets := classifyTunnel(&sizing)
	row[fieldVerdict] = verdict.String()
	row[fieldOctets] = octets
	if verdict.needsCommand() {
		r.commands = append(r.commands, tunnelCommand(in.cmdKind, in.ifaceName, sizing.recommended))
	}
	return verdict
}

// sizeTunnels builds one row per IPsec tunnel, non-IPsec tunnel, and WireGuard
// interface, applies the unified arithmetic and classification where the
// tunnel can be sized, and collects the remediation commands. It answers the
// rows, the verdicts the run ladder reads, and whether a fault outside the
// table (a refused overhead or failed device readback) was met.
func (r *mtuRun) sizeTunnels() ([]map[string]any, []tunnelVerdict, bool) {
	rows := make([]map[string]any, 0, len(r.tunnels)+len(r.snapshot.tunnels)+len(r.snapshot.wireguards))
	var verdicts []tunnelVerdict
	fault := r.snapshotFault
	for i := range r.tunnels {
		t := &r.tunnels[i]
		row := tunnelRow(t)
		verdict, sized, rowFault := r.sizeTunnel(t, r.snapshot.xfrms, row)
		row[fieldSized] = sized
		if verdict != tunnelVerdictUnspecified {
			verdicts = append(verdicts, verdict)
		}
		if rowFault {
			fault = true
		}
		rows = append(rows, row)
	}
	for i := range r.snapshot.tunnels {
		tun := &r.snapshot.tunnels[i]
		row := nonIPsecTunnelRow(tun)
		verdict, sized, rowFault := r.sizeNonIPsecTunnel(tun, row)
		row[fieldSized] = sized
		if verdict != tunnelVerdictUnspecified {
			verdicts = append(verdicts, verdict)
		}
		if rowFault {
			fault = true
		}
		rows = append(rows, row)
	}
	for i := range r.snapshot.wireguards {
		wg := &r.snapshot.wireguards[i]
		row := wireguardTunnelRow(wg)
		verdict, sized, rowFault := r.sizeWireguardTunnel(wg, row)
		row[fieldSized] = sized
		if verdict != tunnelVerdictUnspecified {
			verdicts = append(verdicts, verdict)
		}
		if rowFault {
			fault = true
		}
		rows = append(rows, row)
	}
	return rows, verdicts, fault
}

// refusedTransformReason prefixes the IPsec not-sized reason when the
// arithmetic refuses a negotiated transform.
const refusedTransformReason = "transform refused: "

// tunnelRow is the part of an IPsec tunnel's row every IPsec tunnel carries.
func tunnelRow(t *ipsecinventory.Tunnel) map[string]any {
	row := map[string]any{
		fieldKind: tunnelKindIPsec,
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
	// An unresolved transform names nothing; the row's reason says why
	// (deriveESPOverhead refuses it), so no "/" stands in for a pair.
	if t.TransformErr != nil {
		return row
	}
	row[fieldTransform] = t.EncryptionName + "/" + t.IntegrityName
	return row
}

// interfaceDownReason builds the reason string for an admin-down interface.
func interfaceDownReason(name string) string {
	return "interface " + name + " is down"
}

// interfaceMTURangeReason builds the reason string for an out-of-range MTU.
func interfaceMTURangeReason(name string, mtu int) string {
	var b textbuf.Buffer
	b.Str("interface ").Str(name).Str(" reports an MTU of ").Int(int64(mtu)).Str(", outside 1..65535")
	return b.String()
}

// sizeTunnel applies the verdict order to one IPsec tunnel: down before
// anything, then the reasons a tunnel cannot be sized, then the unified
// arithmetic and classification.
func (r *mtuRun) sizeTunnel(t *ipsecinventory.Tunnel, xfrms []xfrmInterface, row map[string]any) (verdict tunnelVerdict, sized, fault bool) {
	if !t.Up {
		row[fieldVerdict] = tunnelVerdictDown.String()
		row[fieldReason] = "no child SA is installed"
		return tunnelVerdictDown, false, false
	}
	if t.IfID == 0 {
		row[fieldReason] = "the child SA is policy-based and bound to no interface"
		return tunnelVerdictUnspecified, false, false
	}
	xfrm, found := xfrmByID(xfrms, t.IfID)
	if !found {
		var b textbuf.Buffer
		b.Str("no xfrm interface carries if_id ").Uint32(t.IfID)
		row[fieldReason] = b.String()
		return tunnelVerdictUnspecified, false, false
	}
	row[fieldInterface] = xfrm.name
	if !xfrm.up {
		row[fieldVerdict] = tunnelVerdictDown.String()
		row[fieldReason] = interfaceDownReason(xfrm.name)
		return tunnelVerdictDown, false, false
	}
	current, ok := interfaceMTUOctets(xfrm.mtu)
	if !ok {
		row[fieldReason] = interfaceMTURangeReason(xfrm.name, xfrm.mtu)
		return tunnelVerdictUnspecified, false, false
	}
	row[fieldCurrentMTU] = current
	inner := innerFamily(t)
	if inner == ipFamilyUnspecified {
		row[fieldReason] = "the child SA carries no traffic selector, so its inner family is unknown"
		return tunnelVerdictUnspecified, false, false
	}
	overhead, err := deriveESPOverhead(t)
	if err != nil {
		row[fieldReason] = refusedTransformReason + err.Error()
		r.note(noteSeverityFault, "peer "+t.Peer+": "+err.Error()+notAdvisedSuffix)
		return tunnelVerdictUnspecified, false, true
	}
	path, assumed, measured := r.tunnelPath(t)
	if !measured {
		row[fieldReason] = noPeerPathMeasuredReason
		return tunnelVerdictUnspecified, false, false
	}
	verdict = r.applyTunnelSizing(row, &tunnelSizingInput{
		cmdKind:   xfrmLinkType,
		ifaceName: xfrm.name,
		current:   current,
		path:      path,
		assumed:   assumed,
		overhead:  overhead,
		inner:     inner,
		emitMSS:   true,
	})
	return verdict, true, false
}

// nonIPsecTunnelRow builds the base fields for one non-IPsec tunnel interface.
func nonIPsecTunnelRow(tun *nonIPsecTunnel) map[string]any {
	row := map[string]any{
		fieldKind:      tunnelKindTunnel,
		fieldInterface: tun.name,
	}
	if tun.readErr != nil {
		return row
	}
	if tun.info.Kind != iface.TunnelKindUnknown {
		row[fieldKind] = tun.info.Kind.String()
	}
	if tun.info.Remote.IsValid() {
		row[fieldRemote] = tun.info.Remote.Unmap().String()
	}
	return row
}

// sizeNonIPsecTunnel applies the verdict order and overhead arithmetic to one
// non-IPsec tunnel interface (gre, gretap, ip6gre, ip6gretap, vxlan, ipip,
// sit, ip6tnl, ipip6).
func (r *mtuRun) sizeNonIPsecTunnel(tun *nonIPsecTunnel, row map[string]any) (verdict tunnelVerdict, sized, fault bool) {
	if !tun.up {
		row[fieldVerdict] = tunnelVerdictDown.String()
		row[fieldReason] = interfaceDownReason(tun.name)
		return tunnelVerdictDown, false, false
	}
	if tun.readErr != nil {
		row[fieldReason] = "tunnel state could not be read: " + tun.readErr.Error()
		r.note(noteSeverityFault, "tunnel "+tun.name+": "+tun.readErr.Error()+notAdvisedSuffix)
		return tunnelVerdictUnspecified, false, true
	}
	current, ok := interfaceMTUOctets(tun.mtu)
	if !ok {
		row[fieldReason] = interfaceMTURangeReason(tun.name, tun.mtu)
		return tunnelVerdictUnspecified, false, false
	}
	row[fieldCurrentMTU] = current
	if tun.info.FlowBased {
		row[fieldReason] = "tunnel is flow-based with no fixed remote endpoint"
		return tunnelVerdictUnspecified, false, false
	}
	if !tun.info.Remote.IsValid() {
		row[fieldReason] = "tunnel has no configured remote endpoint"
		return tunnelVerdictUnspecified, false, false
	}
	remote := tun.info.Remote.Unmap()
	if remote.IsUnspecified() {
		row[fieldReason] = "tunnel has an unspecified point-to-multipoint remote endpoint"
		return tunnelVerdictUnspecified, false, false
	}
	if remote.IsMulticast() {
		row[fieldReason] = "tunnel has a multicast remote endpoint and cannot be probed"
		return tunnelVerdictUnspecified, false, false
	}
	overhead, err := deriveTunnelOverhead(&tun.info)
	if err != nil {
		row[fieldReason] = "overhead refused: " + err.Error()
		r.note(noteSeverityFault, "tunnel "+tun.name+": "+err.Error()+notAdvisedSuffix)
		return tunnelVerdictUnspecified, false, true
	}
	path, assumed, measured := r.targetPath(remote)
	if !measured {
		row[fieldReason] = noPeerPathMeasuredReason
		return tunnelVerdictUnspecified, false, false
	}
	inner, emitMSS := nonIPsecInnerFamily(tun.info.Kind, tun.addrs)
	verdict = r.applyTunnelSizing(row, &tunnelSizingInput{
		cmdKind:   tunnelKindTunnel,
		ifaceName: tun.name,
		current:   current,
		path:      path,
		assumed:   assumed,
		overhead:  overhead,
		inner:     inner,
		emitMSS:   emitMSS,
	})
	return verdict, true, false
}

// wireguardTunnelRow builds the base fields for one WireGuard interface.
func wireguardTunnelRow(wg *wireguardInterface) map[string]any {
	return map[string]any{
		fieldKind:      tunnelKindWireguard,
		fieldInterface: wg.name,
	}
}

// sizeWireguardTunnel evaluates every peer with a probeable unicast endpoint
// and sizes the single WireGuard device to the peer yielding the tightest
// (lowest) ceiling, emitting at most one set interface wireguard command.
func (r *mtuRun) sizeWireguardTunnel(wg *wireguardInterface, row map[string]any) (verdict tunnelVerdict, sized, fault bool) {
	if !wg.up {
		row[fieldVerdict] = tunnelVerdictDown.String()
		row[fieldReason] = interfaceDownReason(wg.name)
		return tunnelVerdictDown, false, false
	}
	if wg.readErr != nil {
		row[fieldReason] = "wireguard state could not be read: " + wg.readErr.Error()
		r.note(noteSeverityFault, "wireguard "+wg.name+": "+wg.readErr.Error()+notAdvisedSuffix)
		return tunnelVerdictUnspecified, false, true
	}
	current, ok := interfaceMTUOctets(wg.mtu)
	if !ok {
		row[fieldReason] = interfaceMTURangeReason(wg.name, wg.mtu)
		return tunnelVerdictUnspecified, false, false
	}
	row[fieldCurrentMTU] = current
	var chosen *wgCandidate
	probeable := 0
	for i := range wg.peers {
		peer := &wg.peers[i]
		if !isProbeableRemote(peer.endpoint) {
			continue
		}
		probeable++
		path, assumed, measured := r.targetPath(peer.endpoint)
		if !measured {
			continue
		}
		overhead, err := deriveWireguardOverhead(peer.endpoint)
		if err != nil {
			row[fieldReason] = "overhead refused: " + err.Error()
			r.note(noteSeverityFault, "wireguard "+wg.name+": "+err.Error()+notAdvisedSuffix)
			return tunnelVerdictUnspecified, false, true
		}
		ceil := ceiling(path, &overhead)
		if chosen == nil {
			chosen = &wgCandidate{peer: peer, path: path, assumed: assumed, overhead: overhead, ceiling: ceil}
			continue
		}
		if ceil < chosen.ceiling {
			chosen = &wgCandidate{peer: peer, path: path, assumed: assumed, overhead: overhead, ceiling: ceil}
			continue
		}
		if ceil == chosen.ceiling && chosen.assumed && !assumed {
			chosen = &wgCandidate{peer: peer, path: path, assumed: assumed, overhead: overhead, ceiling: ceil}
		}
	}
	if probeable == 0 {
		row[fieldReason] = "no WireGuard peer has a known unicast endpoint address"
		return tunnelVerdictUnspecified, false, false
	}
	if chosen == nil {
		row[fieldReason] = noPeerPathMeasuredReason
		return tunnelVerdictUnspecified, false, false
	}
	if chosen.peer.name != "" {
		row[fieldPeer] = chosen.peer.name
	}
	row[fieldRemote] = chosen.peer.endpoint.Unmap().String()
	inner, emitMSS := addressedInnerFamily(wg.addrs, wg.peers)
	verdict = r.applyTunnelSizing(row, &tunnelSizingInput{
		cmdKind:   tunnelKindWireguard,
		ifaceName: wg.name,
		current:   current,
		path:      chosen.path,
		assumed:   chosen.assumed,
		overhead:  chosen.overhead,
		inner:     inner,
		emitMSS:   emitMSS,
	})
	return verdict, true, false
}

// nonIPsecInnerFamily returns the inner address family (controlling the
// minimum MTU floor) and whether TCP MSS is meaningful for one non-IPsec
// tunnel kind.
func nonIPsecInnerFamily(kind iface.TunnelKind, addrs []iface.AddrInfo) (ipFamily, bool) {
	switch kind {
	case iface.TunnelKindIPIP, iface.TunnelKindIPIP6:
		return ipFamilyV4, true
	case iface.TunnelKindSIT, iface.TunnelKindIP6Tnl:
		return ipFamilyV6, true
	case iface.TunnelKindGRETap, iface.TunnelKindIP6GRETap, iface.TunnelKindVxlan:
		return ipFamilyV6, false
	case iface.TunnelKindGRE, iface.TunnelKindIP6GRE:
		return addressedInnerFamily(addrs, nil)
	case iface.TunnelKindUnknown:
		return ipFamilyV6, false
	default:
		// The TunnelKind set is open; enforce the 1280-octet IPv6 floor with no MSS.
		return ipFamilyV6, false
	}
}

// addressedInnerFamily inspects live non-link-local interface addresses and
// WireGuard AllowedIPs prefixes. Any non-link-local IPv6 address or prefix
// selects IPv6 (conservative for dual-stack); else any IPv4 address or prefix
// selects IPv4; else (unaddressed) enforces the 1280-octet floor and omits MSS.
func addressedInnerFamily(addrs []iface.AddrInfo, peers []wireguardPeer) (ipFamily, bool) {
	hasV4 := false
	for i := range addrs {
		if addrs[i].LinkLocal {
			continue
		}
		parsed, err := netip.ParseAddr(addrs[i].Address)
		if err != nil {
			continue
		}
		addr := parsed.Unmap()
		if addr.Is6() {
			if addr.IsLinkLocalUnicast() {
				continue
			}
			return ipFamilyV6, true
		}
		hasV4 = true
	}
	for i := range peers {
		for _, prefix := range peers[i].allowedIPs {
			addr := prefix.Addr().Unmap()
			if addr.Is6() {
				if addr.IsLinkLocalUnicast() {
					continue
				}
				return ipFamilyV6, true
			}
			hasV4 = true
		}
	}
	if hasV4 {
		return ipFamilyV4, true
	}
	return ipFamilyV6, false
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
