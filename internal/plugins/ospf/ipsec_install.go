// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- RFC 4552 IPsec installer (IPv6 family).
// Related: config_ipsec.go -- the validated per-interface IPsec block this consumes.
// Related: v3/transport/transport.go -- InterfaceSource supplies the link-local + ifindex.
// RFC: rfc/short/rfc4552.md -- OSPFv3 IPsec AH/ESP; rfc/short/rfc4303.md -- ESP SPI/keys.
//
// The installer translates the RFC 4552 manual-SA config into kernel transport-mode
// SAs and proto-89 policies via the shared internal/component/ike/dataplane seam, keyed
// to the OSPFv3 IPv6-family interface lifecycle. It owns the install/remove/reconcile
// lifecycle and the ze_ospfv3_ipsec_* metrics; the dataplane owns the netlink mechanics.
//
// Selector model (RFC 4552 §6/§7, transport mode only):
//   - ONE SA PER OSPF DESTINATION, because the kernel resolves a transport-mode state by
//     the destination address of the flow and nothing else widens that. Outbound,
//     xfrm_tmpl_resolve_one sets remote = daddr and xfrm_state_find demands the state's
//     own id.daddr equal it; __xfrm6_state_addr_check wildcards the SOURCE alone.
//     Inbound, xfrm_state_lookup takes (daddr, spi, proto) with daddr read off the
//     received packet. A state selector NARROWS that lookup and can never widen it, so a
//     state installed under :: is reachable by no OSPF flow at all.
//   - The destinations an interface has: ff02::5 (AllSPFRouters) and ff02::6
//     (AllDRouters) for what OSPFv3 multicasts and for what arrives addressed to those
//     groups, this interface's own link-local for the unicast Database Description, Link
//     State Request, Link State Update and acknowledgement a neighbor sends US, and each
//     neighbor's link-local for the same exchange in the other direction. The first three
//     are known when the interface opens; a neighbor's is known from its first Hello, so
//     its SA is installed then and removed when the interface state machine drops it
//     (onNeighborSeen / onNeighborLost / clearNeighbors). RFC 4552 §9 sets that pattern
//     for the virtual link: "the routing module must install the corresponding SPD/SAD
//     entries before starting these exchanges".
//   - Every one of them carries the SAME SPI and key (RFC 4552 §7 Figure 3: "the same SA
//     parameters (SPI, keys, etc.) for both inbound (SAi) and outbound (SAo) SAs"),
//     because IKE cannot key a multicast group and every router on the link has to read
//     what any other router sent. They differ only in destination, which is what makes
//     them distinct kernel states rather than one state installed several times.
//   - Each state keeps the {::/0, ::/0, proto 89} selector, which narrows it to OSPF
//     flows so no other traffic can resolve it.
//   - Policies (out/in/fwd): UpperProto 89 so only OSPF traffic is matched (§5), all
//     over the ::/0 wildcard, and IfIndex = the interface ifindex so the policy applies
//     ONLY on the configured interface (§6 interface-based selector). Non-OSPF traffic
//     (ND/ICMPv6) is untouched. One policy per direction still serves every destination:
//     the template carries the reqid and no address, so the flow's own daddr picks which
//     of the interface's states resolves it.
//
// Because the require-policies are interface-scoped (§6), a plain non-IPsec OSPFv3
// interface on the SAME node is unaffected -- its inbound OSPF does not match this
// interface's inbound require-policy. Multiple IPsec interfaces coexist by using
// distinct per-interface SPIs: a state's identity is (daddr, spi, proto), and two
// interfaces sharing an SPI would collide on the two multicast groups, which have the
// same destination on every link.

package ospf

import (
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

// ipsecReqIDBase namespaces the per-interface XFRM reqid (base + ifindex) so the OSPF
// SAs and policies bind to each other without colliding with IKE child-SA reqids.
const ipsecReqIDBase uint32 = 0x054F0000

// ipsecSharedDir is the direction a multicast-group state carries: none of its own. The
// zero SADir names NEITHER direction (dataplane.SAParams.Dir), and RFC 4552 §7 gives the
// group one manual SPI and key that protects what this router sends and verifies what its
// neighbors send. Two branches read it: setGauges counts such a state in both directions,
// and a backend that flags direction per SA refuses it rather than pick one.
const ipsecSharedDir dataplane.SADir = 0

// kernelDropPollInterval is how often the XFRM error counters are sampled for the
// ze_ospfv3_ipsec_kernel_drops_total metric.
const kernelDropPollInterval = 15 * time.Second

// ipsecDataplane is the subset of dataplane.Dataplane the installer uses. Narrowing it
// keeps the installer testable with a fake and documents that IPsec never lists/closes.
type ipsecDataplane interface {
	InstallSA(dataplane.SAParams) error
	RemoveSA(spi uint32, dst net.IP, proto uint8) error
	InstallPolicy(dataplane.SPParams) error
	RemovePolicyParams(dataplane.SPParams) error
}

// defaultDataplaneSource returns the active XFRM backend, loading it on first use so the
// OSPFv3 IPv6 family works even when IKE has not loaded it (spec-ospf-ext-16 A-8).
func defaultDataplaneSource() (ipsecDataplane, error) {
	if dp := dataplane.Get(); dp != nil {
		return dp, nil
	}
	if err := dataplane.Load("xfrm"); err != nil {
		return nil, err
	}
	dp := dataplane.Get()
	if dp == nil {
		return nil, dataplane.ErrNotRegistered
	}
	return dp, nil
}

// readXfrmDrops is the platform reader for kernel XFRM inbound drop counters (Linux
// reads /proc/net/xfrm_stat; other platforms return nil). Overridable in tests.
var readXfrmDrops = readXfrmDropsPlatform

// ipsecStatus is the operator-visible IPsec state of an interface (never the key).
type ipsecStatus struct {
	Protocol string
	SPI      uint32
}

type installedIPsec struct {
	spec    ipsecInterfaceConfig
	ifindex int
	// local is the link-local the inbound unicast SA is keyed on. A new link-local on the
	// same ifindex makes that SA unreachable, so reconcileInterfaceLocked reinstalls.
	local    netip.Addr
	policies []dataplane.SPParams
	// sas holds the states an interface has for as long as it is protected: the two
	// multicast groups and local. The per-adjacency ones are in neighbors.
	sas []dataplane.SAParams
	// neighbors maps a neighbor's router id to the link-local its outbound SA is keyed
	// on. The router id is the identity the interface state machine reports a drop under,
	// and holding the address beside it is what lets a neighbor that changed its
	// link-local have the stale state removed rather than orphaned.
	//
	// The record is held by value in ipsecInstaller.installed and this map is the one
	// field that changes after the install, so a neighbor event mutates the map in place
	// rather than writing the record back.
	neighbors map[types.RouterID]netip.Addr
}

// ipsecInstaller owns the RFC 4552 kernel-IPsec lifecycle for the OSPFv3 IPv6 family.
type ipsecInstaller struct {
	log     *slog.Logger
	metrics *ipsecMetrics

	// dpSource resolves the kernel dataplane lazily; source resolves the link-local
	// source + ifindex of an open interface. Both are injected for tests.
	dpSource func() (ipsecDataplane, error)
	source   func(name string) (netip.Addr, int, bool)

	mu        sync.Mutex
	desired   map[string]ipsecInterfaceConfig
	installed map[string]installedIPsec

	// kernel-drop poller lifecycle.
	pollOnce sync.Once
	stopPoll chan struct{}
	lastDrop map[string]uint64
}

// newIPsecInstaller builds an installer bound to the production dataplane source. reg
// may be nil (metrics become no-ops). Call setTransportSource before interfaces open.
func newIPsecInstaller(reg metrics.Registry, log *slog.Logger) *ipsecInstaller {
	if log == nil {
		log = slogutil.DiscardLogger()
	}
	return &ipsecInstaller{
		log:       log,
		metrics:   newIPsecMetrics(reg),
		dpSource:  defaultDataplaneSource,
		desired:   make(map[string]ipsecInterfaceConfig),
		installed: make(map[string]installedIPsec),
		stopPoll:  make(chan struct{}),
		lastDrop:  make(map[string]uint64),
	}
}

// installIPsecHooks attaches the RFC 4552 installer to the IPv6-family engine and wires
// the v3 transport link-local/ifindex source. The engine's onInterfaceUp / onInterfaceDown
// (registered on the transport via Transport.OnInterfaceUp / Transport.OnInterfaceDown in
// newEngineWithCodec) drive install/remove, so the kernel policy+SA exist before the first
// Hello (spec-ospf-ext-16 R-1). Called from register.go for the eng6 instance only.
func (e *engine) installIPsecHooks(inst *ipsecInstaller) {
	e.ipsec = inst
}

// ipsecInterfaceView is one row of `show ospf ipv6 interface`. The key is never included
// (spec-ospf-ext-16 R-4/AC-15): only presence, protocol, SPI, and install state.
type ipsecInterfaceView struct {
	Interface string `json:"interface"`
	Enabled   bool   `json:"ipsec"`
	Protocol  string `json:"protocol,omitempty"`
	SPI       uint32 `json:"spi,omitempty"`
	Installed bool   `json:"installed"`
}

// ipsecInterfaceSnapshot renders the IPv6-family interfaces with their RFC 4552 IPsec
// status for `show ospf ipv6 interface`. It reflects the configured intent (protocol/SPI)
// and whether the kernel SA is installed, never the key material.
func (e *engine) ipsecInterfaceSnapshot() []any {
	e.mu.Lock()
	ifaces := make([]interfaceConfig, len(e.cfg.Interfaces))
	copy(ifaces, e.cfg.Interfaces)
	inst := e.ipsec
	e.mu.Unlock()

	out := make([]any, 0, len(ifaces))
	for _, ic := range ifaces {
		row := ipsecInterfaceView{Interface: ic.Name}
		if ic.IPsec != nil {
			row.Enabled = true
			row.Protocol = ic.IPsec.Protocol
			row.SPI = ic.IPsec.SPI
		}
		if inst != nil {
			if _, ok := inst.status(ic.Name); ok {
				row.Installed = true
			}
		}
		out = append(out, row)
	}
	return out
}

// setTransportSource wires the link-local + ifindex accessor (v3 transport InterfaceSource).
func (i *ipsecInstaller) setTransportSource(fn func(name string) (netip.Addr, int, bool)) {
	i.mu.Lock()
	i.source = fn
	i.mu.Unlock()
}

// setConfig replaces the desired per-interface IPsec map from the IPv6-family interfaces.
func (i *ipsecInstaller) setConfig(interfaces []interfaceConfig) {
	next := make(map[string]ipsecInterfaceConfig)
	for _, ic := range interfaces {
		if ic.IPsec != nil {
			next[ic.Name] = *ic.IPsec
		}
	}
	i.mu.Lock()
	i.desired = next
	i.mu.Unlock()
}

// onInterfaceUp installs (or reconciles) the interface's IPsec on link-up. It runs
// synchronously so the kernel SA/policy exist before the engine sends the first Hello
// (spec-ospf-ext-16 R-1/AC-7).
func (i *ipsecInstaller) onInterfaceUp(ifindex int, name string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.reconcileInterfaceLocked(ifindex, name)
}

// onInterfaceDown removes the interface's IPsec on link-down (AC-10).
func (i *ipsecInstaller) onInterfaceDown(_ int, name string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.removeLocked(name)
}

// reconcileAll re-evaluates every desired/installed interface against the current config,
// installing changed SPIs/keys and removing dropped blocks (AC-11).
func (i *ipsecInstaller) reconcileAll() {
	i.mu.Lock()
	defer i.mu.Unlock()
	seen := make(map[string]struct{}, len(i.desired)+len(i.installed))
	for name := range i.desired {
		seen[name] = struct{}{}
	}
	for name := range i.installed {
		seen[name] = struct{}{}
	}
	for name := range seen {
		ifindex := i.ifindexLocked(name)
		i.reconcileInterfaceLocked(ifindex, name)
	}
}

// status reports the IPsec state of an interface for `show ospf ipv6 interface`.
func (i *ipsecInstaller) status(name string) (ipsecStatus, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	cur, ok := i.installed[name]
	if !ok {
		return ipsecStatus{}, false
	}
	return ipsecStatus{Protocol: cur.spec.Protocol, SPI: cur.spec.SPI}, true
}

// Close stops the kernel-drop poller.
func (i *ipsecInstaller) Close() {
	i.mu.Lock()
	defer i.mu.Unlock()
	select {
	case <-i.stopPoll:
	default:
		close(i.stopPoll)
	}
}

func (i *ipsecInstaller) ifindexLocked(name string) int {
	if cur, ok := i.installed[name]; ok && cur.ifindex != 0 {
		return cur.ifindex
	}
	if i.source != nil {
		if _, ifindex, ok := i.source(name); ok {
			return ifindex
		}
	}
	return 0
}

// reconcileInterfaceLocked installs, replaces, or removes the IPsec for one interface.
func (i *ipsecInstaller) reconcileInterfaceLocked(ifindex int, name string) {
	desired, want := i.desired[name]
	cur, have := i.installed[name]
	if !want {
		if have {
			i.removeLocked(name)
		}
		return
	}
	if ifindex == 0 {
		return // interface not open yet; onInterfaceUp installs when it opens.
	}
	local := i.linkLocalLocked(name)
	if have && cur.ifindex == ifindex && cur.local == local && ipsecEqual(&cur.spec, &desired) {
		return
	}
	if have {
		i.removeLocked(name)
	}
	i.installLocked(ifindex, name, local, desired)
}

// linkLocalLocked reports the interface's link-local source address, or the invalid
// address when the transport has not finished opening the interface. It is the
// destination of every unicast OSPF packet a neighbor sends this router, so the inbound
// unicast SA is keyed on it.
func (i *ipsecInstaller) linkLocalLocked(name string) netip.Addr {
	if i.source == nil {
		return netip.Addr{}
	}
	local, _, ok := i.source(name)
	if !ok {
		return netip.Addr{}
	}
	return local
}

// installLocked builds and installs the interface's destination-scoped transport-mode
// SAs then the proto-89 policies. The SAs are installed before the policies so an
// inbound "require" policy never predates its SA (spec-ospf-ext-16 R-1).
func (i *ipsecInstaller) installLocked(ifindex int, name string, local netip.Addr, spec ipsecInterfaceConfig) {
	if i.source == nil {
		i.metrics.failures.With(name, "no-transport-source").Inc()
		return
	}
	// A valid link-local gates protection twice over: a tentative or absent source means
	// the transport has not finished opening the interface, and it is also the
	// destination the inbound unicast SA is keyed on, so there is nothing to install
	// without it.
	if !local.IsValid() {
		i.metrics.failures.With(name, "no-link-local").Inc()
		i.log.Warn("ospf ipsec: no link-local source; interface NOT protected", "interface", name)
		return
	}
	dp, err := i.dpSource()
	if err != nil || dp == nil {
		i.metrics.failures.With(name, "no-dataplane").Inc()
		// R-7/AC-12: fail loud, never silently claim protection.
		i.log.Error("ospf ipsec: kernel dataplane unavailable; interface NOT protected", "interface", name, "err", err)
		return
	}
	// One state per destination the interface already has (the two groups and its own
	// link-local); a neighbor's arrives later through onNeighborSeen.
	sas := buildIPsecInterfaceSAs(ifindex, local, spec)
	policies := buildIPsecPolicies(ifindex, spec)

	rec := installedIPsec{spec: spec, ifindex: ifindex, local: local, neighbors: make(map[types.RouterID]netip.Addr)}
	for index := range sas {
		sa := &sas[index]
		if err := dp.InstallSA(*sa); err != nil {
			i.metrics.failures.With(name, "sa-install").Inc()
			i.log.Error("ospf ipsec: install SA", "interface", name, "spi", sa.SPI, "dst", sa.Dst.String(), "err", err)
			i.rollback(dp, rec)
			return
		}
		rec.sas = append(rec.sas, *sa)
	}
	for _, p := range policies {
		if err := dp.InstallPolicy(p); err != nil {
			i.metrics.failures.With(name, "policy-install").Inc()
			i.log.Error("ospf ipsec: install policy", "interface", name, "err", err)
			i.rollback(dp, rec)
			return
		}
		rec.policies = append(rec.policies, p)
	}
	i.installed[name] = rec
	i.setGauges(name, rec)
	i.startPoller()
	i.log.Info("ospf ipsec: installed", "interface", name, "protocol", spec.Protocol, "spi", spec.SPI, "states", len(rec.sas))
}

// onNeighborSeen installs the outbound SA for one neighbor's unicast address, which is
// where this router sends its Database Description, Link State Request, Link State
// Update and acknowledgement. It is called from the interface state machine's Hello
// handler BEFORE that handler runs the neighbor state machine, because the machine sends
// the first Database Description inline and the kernel drops any packet whose policy
// resolves no state (RFC 4552 §9: install the SAD entry before the exchange starts).
//
// It is called for every Hello and is idempotent: a neighbor already installed at the
// same address costs one map lookup.
func (i *ipsecInstaller) onNeighborSeen(name string, id types.RouterID, addr netip.Addr) {
	i.mu.Lock()
	defer i.mu.Unlock()
	rec, ok := i.installed[name]
	if !ok || !addr.IsValid() {
		return // the interface is not protected, or the Hello carried no source address.
	}
	if known, seen := rec.neighbors[id]; seen {
		if known == addr {
			return
		}
		// The neighbor moved to a different link-local. Its old state would protect
		// nothing and no later event names that address, so it goes now.
		i.removeNeighborSALocked(name, rec, id, known)
	}
	dp, err := i.dpSource()
	if err != nil || dp == nil {
		i.metrics.failures.With(name, "no-dataplane").Inc()
		i.log.Error("ospf ipsec: kernel dataplane unavailable; neighbor NOT protected", "interface", name, "neighbor", id.String(), "err", err)
		return
	}
	sa := buildIPsecSA(rec.ifindex, addr, dataplane.SADirOut, rec.spec)
	if err := dp.InstallSA(sa); err != nil {
		i.metrics.failures.With(name, "neighbor-sa-install").Inc()
		i.log.Error("ospf ipsec: install neighbor SA", "interface", name, "neighbor", id.String(), "dst", addr.String(), "err", err)
		return
	}
	rec.neighbors[id] = addr
	i.setGauges(name, rec)
	i.log.Info("ospf ipsec: neighbor protected", "interface", name, "neighbor", id.String(), "dst", addr.String())
}

// onNeighborLost removes one neighbor's outbound SA. The interface state machine calls it
// for every drop it performs, the dead-interval expiry included, so a state outlives the
// adjacency by nothing.
func (i *ipsecInstaller) onNeighborLost(name string, id types.RouterID) {
	i.mu.Lock()
	defer i.mu.Unlock()
	rec, ok := i.installed[name]
	if !ok {
		return
	}
	addr, seen := rec.neighbors[id]
	if !seen {
		return
	}
	i.removeNeighborSALocked(name, rec, id, addr)
	i.setGauges(name, rec)
}

// clearNeighbors removes every neighbor SA on an interface. The interface state machine
// drops all its neighbors at once when it stops, and reports that as one event rather
// than one per neighbor.
func (i *ipsecInstaller) clearNeighbors(name string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	rec, ok := i.installed[name]
	if !ok {
		return
	}
	for id, addr := range rec.neighbors {
		i.removeNeighborSALocked(name, rec, id, addr)
	}
	i.setGauges(name, rec)
}

// removeNeighborSALocked deletes one neighbor's kernel state and forgets it. The caller
// holds i.mu and owns the gauges, so this stays usable inside a loop over the map.
func (i *ipsecInstaller) removeNeighborSALocked(name string, rec installedIPsec, id types.RouterID, addr netip.Addr) {
	delete(rec.neighbors, id)
	dp, err := i.dpSource()
	if err != nil || dp == nil {
		i.log.Error("ospf ipsec: kernel dataplane unavailable; neighbor SA left installed", "interface", name, "neighbor", id.String(), "err", err)
		return
	}
	if err := dp.RemoveSA(rec.spec.SPI, addr.AsSlice(), ipsecProtoNumber(rec.spec.Protocol)); err != nil {
		i.log.Debug("ospf ipsec: remove neighbor SA", "interface", name, "neighbor", id.String(), "err", err)
	}
}

func (i *ipsecInstaller) removeLocked(name string) {
	cur, ok := i.installed[name]
	if !ok {
		return
	}
	if dp, err := i.dpSource(); err == nil && dp != nil {
		for idx := range cur.policies {
			if err := dp.RemovePolicyParams(cur.policies[idx]); err != nil {
				i.log.Debug("ospf ipsec: remove policy", "interface", name, "err", err)
			}
		}
		for idx := range cur.sas {
			sa := &cur.sas[idx]
			if err := dp.RemoveSA(sa.SPI, sa.Dst, sa.Proto); err != nil {
				i.log.Debug("ospf ipsec: remove SA", "interface", name, "spi", sa.SPI, "err", err)
			}
		}
		proto := ipsecProtoNumber(cur.spec.Protocol)
		for id, addr := range cur.neighbors {
			if err := dp.RemoveSA(cur.spec.SPI, addr.AsSlice(), proto); err != nil {
				i.log.Debug("ospf ipsec: remove neighbor SA", "interface", name, "neighbor", id.String(), "err", err)
			}
		}
	}
	delete(i.installed, name)
	i.clearGauges(name, cur.spec)
	i.log.Info("ospf ipsec: removed", "interface", name)
}

// rollback undoes a partially-installed interface after an install error.
func (i *ipsecInstaller) rollback(dp ipsecDataplane, rec installedIPsec) {
	for idx := range rec.policies {
		_ = dp.RemovePolicyParams(rec.policies[idx])
	}
	for idx := range rec.sas {
		_ = dp.RemoveSA(rec.sas[idx].SPI, rec.sas[idx].Dst, rec.sas[idx].Proto)
	}
}

// setGauges publishes how many kernel states serve each direction on a protected
// interface. The two multicast states count in both: RFC 4552 §7 shares one SPI and key,
// so the state that protects what this router sends to ff02::5 is the state that
// verifies what a neighbor sent to it.
func (i *ipsecInstaller) setGauges(name string, rec installedIPsec) {
	inbound, outbound := 0, len(rec.neighbors)
	for idx := range rec.sas {
		switch rec.sas[idx].Dir {
		case dataplane.SADirIn:
			inbound++
		case dataplane.SADirOut:
			outbound++
		default: // unset: the shared state of RFC 4552 §7, which serves both directions.
			inbound++
			outbound++
		}
	}
	i.metrics.sas.With(name, rec.spec.Protocol, "in").Set(float64(inbound))
	i.metrics.sas.With(name, rec.spec.Protocol, "out").Set(float64(outbound))
	i.metrics.policies.With(name, "out").Set(1)
	i.metrics.policies.With(name, "in").Set(1)
	i.metrics.policies.With(name, "fwd").Set(1)
}

// clearGauges zeroes an interface's series after its IPsec is removed, so a stale value
// never reads as protection that is no longer installed.
func (i *ipsecInstaller) clearGauges(name string, spec ipsecInterfaceConfig) {
	i.metrics.sas.With(name, spec.Protocol, "in").Set(0)
	i.metrics.sas.With(name, spec.Protocol, "out").Set(0)
	i.metrics.policies.With(name, "out").Set(0)
	i.metrics.policies.With(name, "in").Set(0)
	i.metrics.policies.With(name, "fwd").Set(0)
}

// ospfWildcardNet returns a fresh ::/0 IPv6 wildcard prefix. The policies and the state
// selectors are built over ::/0 with the proto-89 upper-layer selector, so one policy per
// direction covers every OSPF destination on the interface (RFC 4552 §5/§6); a fresh
// value per call avoids shared-pointer aliasing between the SA selector and the policies.
//
// The wildcard belongs to a SELECTOR and never to a state's address. A state under ::
// is reachable by no flow at all, which is what the header comment records.
func ospfWildcardNet() *net.IPNet {
	return &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)}
}

// buildIPsecInterfaceSAs builds the transport-mode SAs an interface has from the moment
// it is protected, one per destination (RFC 4552 §7 gives them all the same SPI and key):
//
//   - ff02::5 and ff02::6, which carry the Hello and the flooded Link State Update this
//     router multicasts, and which are also the destination of what its neighbors
//     multicast, so each of those two states serves both directions.
//   - local, this interface's own link-local, which is the destination of every unicast
//     Database Description, Link State Request, Link State Update and acknowledgement a
//     neighbor sends this router. It is inbound only; the outbound half of that exchange
//     is keyed on the neighbor's address and installed by onNeighborSeen.
func buildIPsecInterfaceSAs(ifindex int, local netip.Addr, c ipsecInterfaceConfig) []dataplane.SAParams {
	return []dataplane.SAParams{
		buildIPsecSA(ifindex, ospfv3transport.AllSPFRouters, ipsecSharedDir, c),
		buildIPsecSA(ifindex, ospfv3transport.AllDRouters, ipsecSharedDir, c),
		buildIPsecSA(ifindex, local, dataplane.SADirIn, c),
	}
}

// buildIPsecSA builds one transport-mode SA for one OSPF destination, keyed from the
// interface's manual SPI and key (RFC 4552 §7). Its state selector is {::/0, ::/0, proto
// 89}, which narrows it to OSPF flows.
//
// dst is the state's identity, with the SPI and the protocol: the kernel resolves an
// outbound state by the flow's destination address (xfrm_tmpl_resolve_one sets remote =
// daddr, and xfrm_state_find demands x->id.daddr equal it) and an inbound one by the
// destination on the received packet. Only the SOURCE is wildcarded, by
// __xfrm6_state_addr_check, which is why Src stays :: and Dst never does.
//
// dir names the direction where the destination has one, and is left unset (0) for the
// two multicast groups, which have none: the same manual key protects what this router
// sends to a group and verifies what a neighbor sent to it, and RFC 4552 §7 requires
// exactly that sharing. A backend that flags direction per SA refuses an unset Dir rather
// than pick one (vppUnsupportedSA, ike/dataplane/vpp.go), which is the right answer for a
// state that carries both.
func buildIPsecSA(ifindex int, dst netip.Addr, dir dataplane.SADir, c ipsecInterfaceConfig) dataplane.SAParams {
	reqid := ipsecReqIDBase + uint32(ifindex)
	proto := ipsecProtoNumber(c.Protocol)
	encAlgo := ipsecEncNull
	if c.hasConfidentiality() {
		encAlgo = c.EncAlgo
	}
	return dataplane.SAParams{
		SPI: c.SPI,
		// The source is wildcarded on purpose: __xfrm6_state_addr_check matches a state
		// whose props.saddr is any, so one state per destination serves the flow whatever
		// source address the interface picked.
		Src:   net.IPv6zero,
		Dst:   dst.AsSlice(),
		Proto: proto,
		Mode:  dataplane.ModeTransport, // RFC 4552 §2: transport mode
		ReqID: reqid,
		Dir:   dir,
		Sel: &dataplane.SASelector{
			Src:        ospfWildcardNet(),
			Dst:        ospfWildcardNet(),
			UpperProto: ospfv3transport.Protocol, // 89: only OSPF flows resolve this SA
		},
		AuthAlgo: c.AuthAlgo,
		AuthKey:  c.authKeyBytes(),
		EncAlgo:  encAlgo,
		EncKey:   c.encKeyBytes(),
		// RFC 4302 §3.4.3: "All AH implementations MUST support the anti-replay service,
		// though its use may be enabled or disabled by the receiver on a per-SA basis."
		// The interface's replay-window leaf IS that per-SA switch, and it reaches the
		// kernel through xfrmStateFromParams (ike/dataplane/xfrm_linux.go), which sets a
		// window only for a non-zero ReplayWin. RFC 4303 §3.4.3 states the same
		// obligation for ESP, so the one leaf serves both protocols. Zero is the default
		// because this path is manually keyed and RFC 4302 §5 says a compliant
		// implementation SHOULD NOT provide anti-replay with a manually keyed SA.
		// validateIPsecReplayWindow has already proven the value is 0 or 32..255.
		ReplayWin: uint8(c.ReplayWindow),
	}
}

// buildIPsecPolicies builds the out/in/fwd transport-mode policies with the OSPF proto-89
// upper-layer selector over the ::/0 wildcard (RFC 4552 §5), each scoped to the interface
// ifindex so the require-policies apply ONLY on this interface (§6 interface-based
// selector). Interface scoping is what lets a plain non-IPsec OSPFv3 interface coexist:
// its inbound OSPF never matches this interface's inbound require-policy.
func buildIPsecPolicies(ifindex int, c ipsecInterfaceConfig) []dataplane.SPParams {
	reqid := ipsecReqIDBase + uint32(ifindex)
	proto := ipsecProtoNumber(c.Protocol)
	mk := func(dir dataplane.SADir) dataplane.SPParams {
		return dataplane.SPParams{
			Src:        ospfWildcardNet(),
			Dst:        ospfWildcardNet(),
			Dir:        dir,
			Proto:      proto,
			Mode:       dataplane.ModeTransport,
			ReqID:      reqid,
			UpperProto: ospfv3transport.Protocol, // 89: only OSPF traffic
			IfIndex:    ifindex,                  // §6: scope to this interface only
		}
	}
	return []dataplane.SPParams{
		mk(dataplane.SADirOut),
		mk(dataplane.SADirIn),
		mk(dataplane.SADirFwd),
	}
}

func ipsecProtoNumber(protocol string) uint8 {
	if protocol == ipsecProtoAH {
		return dataplane.ProtoAH
	}
	return dataplane.ProtoESP
}

// startPoller launches the kernel-drop sampler once, on the first successful install.
func (i *ipsecInstaller) startPoller() {
	i.pollOnce.Do(func() {
		go i.pollKernelDrops()
	})
}

func (i *ipsecInstaller) pollKernelDrops() {
	ticker := time.NewTicker(kernelDropPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-i.stopPoll:
			return
		case <-ticker.C:
			i.sampleKernelDrops()
		}
	}
}

// sampleKernelDrops reads the XFRM inbound drop counters and advances the Prometheus
// counter by the delta. XFRM error stats are node-global, so the interface label is
// empty (documented limitation): the reason distinguishes no-policy vs auth-failed.
func (i *ipsecInstaller) sampleKernelDrops() {
	drops, err := readXfrmDrops()
	if err != nil || drops == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for reason, total := range drops {
		prev := i.lastDrop[reason]
		if total > prev {
			i.metrics.kernelDrops.With("", reason).Add(float64(total - prev))
		}
		i.lastDrop[reason] = total
	}
}
