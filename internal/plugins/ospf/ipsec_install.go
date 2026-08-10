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
//   - One shared wildcard SA per interface: Src = Dst = :: (wildcard) with a state
//     selector of {::/0, ::/0, proto 89}, reqid-bound to the policies. OSPFv3 sends
//     protocol-89 to ff02::5 (AllSPFRouters), ff02::6 (AllDRouters), and neighbor
//     link-local unicast (DBD / LSU retransmit); a destination-scoped SA cannot cover
//     all three (the neighbor unicast daddr is unknown at install), so one wildcard SA
//     matched by the proto-89 selector protects every OSPF flow in BOTH directions.
//     RFC 4552 §7: IKE cannot key the multicast group, so a single manual key/SPI is
//     shared for inbound (verify) and outbound (protect); the kernel identifies a state
//     by (daddr, spi, proto), so the two directions are the SAME wildcard state -- one
//     install, not two.
//   - Policies (out/in/fwd): UpperProto 89 so only OSPF traffic is matched (§5), all
//     over the ::/0 wildcard, and IfIndex = the interface ifindex so the policy applies
//     ONLY on the configured interface (§6 interface-based selector). Non-OSPF traffic
//     (ND/ICMPv6) is untouched.
//
// Because the require-policies are interface-scoped (§6), a plain non-IPsec OSPFv3
// interface on the SAME node is unaffected -- its inbound OSPF does not match this
// interface's inbound require-policy. Multiple IPsec interfaces coexist by using
// distinct per-interface SPIs (the shared wildcard state's identity is (::, spi, proto),
// so two interfaces sharing an SPI would collide -- configure a unique SPI per interface).

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
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

// ipsecReqIDBase namespaces the per-interface XFRM reqid (base + ifindex) so the OSPF
// SAs and policies bind to each other without colliding with IKE child-SA reqids.
const ipsecReqIDBase uint32 = 0x054F0000

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
	spec     ipsecInterfaceConfig
	ifindex  int
	policies []dataplane.SPParams
	sas      []dataplane.SAParams
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
	if have && cur.ifindex == ifindex && ipsecEqual(&cur.spec, &desired) {
		return
	}
	if have {
		i.removeLocked(name)
	}
	i.installLocked(ifindex, name, desired)
}

// installLocked builds and installs the shared transport-mode SA then the proto-89
// policies. The SA is installed before the policies so an inbound "require" policy
// never predates its SA (spec-ospf-ext-16 R-1).
func (i *ipsecInstaller) installLocked(ifindex int, name string, spec ipsecInterfaceConfig) {
	if i.source == nil {
		i.metrics.failures.With(name, "no-transport-source").Inc()
		return
	}
	// The SA/policy selectors are ::/0 + proto 89 scoped by ifindex, so the link-local
	// is no longer part of them; but a valid link-local still gates protection -- a
	// tentative/absent source means the transport has not finished opening the interface.
	ll, _, ok := i.source(name)
	if !ok || !ll.IsValid() {
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
	// One shared wildcard SA per interface (RFC 4552 §7): the same (::, spi, proto)
	// state protects egress and verifies ingress, so it is installed once.
	sa := buildIPsecSA(ifindex, spec)
	policies := buildIPsecPolicies(ifindex, spec)

	rec := installedIPsec{spec: spec, ifindex: ifindex}
	if err := dp.InstallSA(sa); err != nil {
		i.metrics.failures.With(name, "sa-install").Inc()
		i.log.Error("ospf ipsec: install SA", "interface", name, "spi", sa.SPI, "err", err)
		i.rollback(dp, rec)
		return
	}
	rec.sas = append(rec.sas, sa)
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
	i.setGauges(name, spec, 1)
	i.startPoller()
	i.log.Info("ospf ipsec: installed", "interface", name, "protocol", spec.Protocol, "spi", spec.SPI)
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
	}
	delete(i.installed, name)
	i.setGauges(name, cur.spec, 0)
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

func (i *ipsecInstaller) setGauges(name string, spec ipsecInterfaceConfig, v float64) {
	// One shared wildcard SA protects both directions (RFC 4552 §7); the in/out labels
	// report that both directions are covered, not two distinct kernel states.
	i.metrics.sas.With(name, spec.Protocol, "in").Set(v)
	i.metrics.sas.With(name, spec.Protocol, "out").Set(v)
	i.metrics.policies.With(name, "out").Set(v)
	i.metrics.policies.With(name, "in").Set(v)
	i.metrics.policies.With(name, "fwd").Set(v)
}

// ospfWildcardNet returns a fresh ::/0 IPv6 wildcard prefix. Every OSPFv3 IPsec SA
// and policy is built over ::/0 so one manually-keyed SA per interface covers ff02::5,
// ff02::6, and neighbor link-local unicast (RFC 4552 §6/§7); a fresh value per call
// avoids shared-pointer aliasing between the SA selector and the policies.
func ospfWildcardNet() *net.IPNet {
	return &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)}
}

// buildIPsecSA builds the single shared wildcard-address transport-mode SA for the
// interface (RFC 4552 §7). Src = Dst = :: with a {::/0, ::/0, proto 89} state selector
// so the kernel resolves it for every OSPF destination in both directions, and one
// manual SPI/key protects egress and verifies ingress. The kernel identifies a state
// by (daddr, spi, proto): with a wildcard daddr the inbound and outbound SA are the
// SAME state, so the installer installs it once.
//
// SAParams.Dir is left unset on purpose. This one state has no single direction.
// Either SADirIn or SADirOut would claim something RFC 4552 Section 7 does not. A
// backend that flags direction per SA refuses an unset Dir rather than pick one
// (vppUnsupportedSA, ike/dataplane/vpp.go). That is the right answer here, because VPP
// cannot express one bidirectional SA.
func buildIPsecSA(ifindex int, c ipsecInterfaceConfig) dataplane.SAParams {
	reqid := ipsecReqIDBase + uint32(ifindex)
	proto := ipsecProtoNumber(c.Protocol)
	encAlgo := ipsecEncNull
	if c.hasConfidentiality() {
		encAlgo = c.EncAlgo
	}
	return dataplane.SAParams{
		SPI:   c.SPI,
		Src:   net.IPv6zero, // wildcard: one SA covers all OSPFv3 daddrs (RFC 4552 §7)
		Dst:   net.IPv6zero,
		Proto: proto,
		Mode:  dataplane.ModeTransport, // RFC 4552 §2: transport mode
		ReqID: reqid,
		Sel: &dataplane.SASelector{
			Src:        ospfWildcardNet(),
			Dst:        ospfWildcardNet(),
			UpperProto: ospfv3transport.Protocol, // 89: only OSPF flows resolve this SA
		},
		AuthAlgo: c.AuthAlgo,
		AuthKey:  c.authKeyBytes(),
		EncAlgo:  encAlgo,
		EncKey:   c.encKeyBytes(),
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
