// RFC: rfc/short/rfc9568.md -- VRRPv3 (RFCs field, default version)
// RFC: rfc/short/rfc3768.md -- VRRPv2 (RFCs field, opt-in version)
//
// Design: plan/learned/1124-vrrp-first-hop-redundancy.md -- plugin registration and engine entry point
//
// Registration is the plugin's whole coupling to ze: an init() that hands the
// registry a Registration, plus generated blank imports. Delete this directory,
// run `make generate`, and every vrrp surface (config schema, commands, doctor
// codes, metrics, events) disappears with it (ai/rules/plugins.md).
package vrrp

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
	vrrpyang "github.com/ze-software/ze/internal/plugins/vrrp/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// pluginName is the registered plugin name. The log subsystem uses the dotted
// form ("vrrp" has no sub-part, so the two coincide here).
const pluginName = "vrrp"

func init() { registerVRRP() }

func registerVRRP() {
	_ = events.RegisterNamespace(Namespace, EventStateChange)
	registerVRRPDiagnosticCodes()
	registerVRRPDoctor()

	reg := registry.Registration{
		Name:        pluginName,
		Description: "Virtual Router Redundancy Protocol (RFC 9568 / RFC 3768): first-hop gateway redundancy",
		Features:    "yang",
		YANG:        vrrpyang.ZeVrrpConfYANG,
		// The vrrp config lives under interface units, so this plugin consumes
		// the iface component's root and walks only the vrrp-bearing path of it
		// (umbrella A-2). Auto-loading with `interface` means the plugin loads
		// on any interface config; with no groups the engine stays idle (AC-6).
		ConfigRoots:             []string{configRoot},
		Dependencies:            []string{"interface"},
		RFCs:                    []string{"9568", "3768"},
		RunEngine:               runVRRPEngine,
		InProcessConfigVerifier: verifyVRRPConfigSections,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
			transport.SetLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			setMetricsRegistry(reg)
			// The transport owns its own metric series (adverts, packet errors,
			// sockets); wire the same registry into it, mirroring the SetLogger
			// pair above. Without this the ze_vrrp_* transport series stay on the
			// no-op registry and never reach Prometheus (spec-vrrp-4 AC-12).
			sharedTransport.SetMetrics(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
			transport.SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "vrrp: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// verifyVRRPConfigSections is the side-effect-free verifier the engine calls at
// `ze config validate` and commit time.
func verifyVRRPConfigSections(sections []rpc.ConfigSection) error {
	specs, err := extractGroupSpecs(rpcConfigSections(sections))
	if err != nil {
		return err
	}
	return validateGroups(specs, ifaceBackend(rpcConfigSections(sections)))
}

func rpcConfigSections(sections []rpc.ConfigSection) []configSection {
	out := make([]configSection, len(sections))
	for i, s := range sections {
		out[i] = configSection{Root: s.Root, Data: s.Data}
	}
	return out
}

func sdkConfigSections(sections []sdk.ConfigSection) []configSection {
	out := make([]configSection, len(sections))
	for i, s := range sections {
		out[i] = configSection{Root: s.Root, Data: s.Data}
	}
	return out
}

// runVRRPEngine is the in-process entry point.
//
// The plugin MUST run internal: it calls the iface component's registries
// directly (macvlan devices, address ownership), which are in-process Go state.
// A forked copy would mutate its own disconnected copy of that state and no
// virtual address would ever reach the kernel -- the as112 guard precedent
// (internal/plugins/as112/register.go:223).
func runVRRPEngine(conn net.Conn) int {
	log := logger()
	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	if !p.IsInternal() {
		log.Error("vrrp: this plugin must run in-process; configure it as `internal vrrp`",
			"reason", "it drives the interface component's macvlan and address-owner registries, which are same-process state")
		return 1
	}

	eng := newEngine(clock.RealClock{}, livePlatform(), liveDeps())
	defer eng.stopAll()

	// Pending-swap (the ospf model, register.go:403-450): verify parses and
	// validates into `pending`; apply promotes it. Rollback needs no journal --
	// discarding `pending` IS the rollback, because nothing touched the kernel
	// until apply ran.
	var (
		cfgMu   sync.Mutex
		pending []GroupSpec
		havePnd bool
	)

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		specs, err := parseAndVerify(sdkConfigSections(sections))
		if err != nil {
			return err
		}
		cfgMu.Lock()
		pending, havePnd = specs, true
		cfgMu.Unlock()
		return nil
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		specs, err := parseAndVerify(sdkConfigSections(sections))
		if err != nil {
			return err
		}
		eng.apply(specs)
		return nil
	})

	// OnConfigure does not fire on reload; OnConfigApply is the commit step.
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfgMu.Lock()
		specs, ok := pending, havePnd
		havePnd = false
		cfgMu.Unlock()
		if !ok {
			return nil
		}
		eng.apply(specs)
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		cfgMu.Lock()
		pending, havePnd = nil, false
		cfgMu.Unlock()
		return nil
	})

	p.OnStarted(func(context.Context) error {
		go rxLoop(eng)
		return nil
	})

	p.OnExecuteCommand(func(_ string, command string, args []string, _ string) (string, any, error) {
		return handleCommand(eng, command, args)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()

	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands:     commandDecls(),
	}); err != nil {
		log.Error("vrrp: plugin failed", "error", err)
		return 1
	}
	return 0
}

// registerVRRPDiagnosticCodes publishes this plugin's doctor codes so
// `ze explain <code>` resolves them.
func registerVRRPDiagnosticCodes() {
	for _, m := range vrrpDiagnosticCodes {
		_ = diagnostic.Register(m)
	}
}

// registerVRRPDoctor registers the config-sanity check (checkVRRPConfigSanity in
// doctor.go). The os.Exit on a registration failure is confined to this
// registration file (ai/patterns/registration.md).
func registerVRRPDoctor() {
	check := diagnostic.DoctorCheck{
		Name:         "vrrp-config-sanity",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        741, // after the transport's vrrp raw-socket check
		Component:    pluginName,
		Dependencies: []string{"config-tree"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeVRRPConfigInvalid, codeVRRPBackendUnusable},
		Check:        checkVRRPConfigSanity,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "vrrp: doctor config-sanity registration: %v\n", err)
		os.Exit(2)
	}
}

// macString renders a virtual MAC for the iface macvlan spec.
func macString(mac [6]byte) string {
	return textbuf.StringMAC(mac[:])
}

// vipCIDRs renders the virtual addresses for the address-owner registry, each
// masked to the parent subnet that contains it. A non-owner VIP MUST carry the
// subnet's connected route, not a host route: the kernel only answers ARP/ND for
// it from the virtual-MAC device when that device owns the subnet route (proven
// in plan/learned/1122-vrrp-macvlan-vmac-dataplane.md -- a /32 leaves the parent as the sole subnet device and
// it answers with its real MAC).
func (s GroupSpec) vipCIDRs(vips []netip.Addr) []string {
	out := make([]string, 0, len(vips))
	for _, v := range vips {
		out = append(out, netip.PrefixFrom(v, s.vipMaskBits(v)).String())
	}
	return out
}

// vipMaskBits picks the prefix length to install a VIP with. The rule is subnet
// prefix for a normal VIP, host route otherwise:
//
//   - A VIP equal to one of the unit's real addresses is the RFC address owner's
//     address: it already lives on the PARENT with the parent's subnet prefix, so
//     installing it on the macvlan with the subnet prefix too would add a second,
//     competing connected route for the subnet. Install it as a host route.
//     (vMAC ownership is unachievable for the owner regardless -- the address is
//     local to the parent, so the parent answers ARP for it whatever arp_ignore
//     says -- so the host route keeps the owner reachable via the parent without
//     the duplicate-route side effects.)
//   - Otherwise use the LONGEST parent subnet that contains the VIP, so the
//     macvlan owns that subnet's connected route.
//   - A VIP contained by no parent subnet (a misconfiguration -- a VRRP VIP
//     should sit in the parent's subnet) gets a host route, NEVER a non-containing
//     parent prefix's length, which would install a bogus connected route for a
//     subnet the VIP is not in.
func (s GroupSpec) vipMaskBits(v netip.Addr) int {
	if slices.Contains(s.realAddresses, v) {
		return v.BitLen()
	}
	best := -1
	for _, p := range s.realPrefixes {
		if p.Addr().Is4() != v.Is4() {
			continue
		}
		if p.Contains(v) && p.Bits() > best {
			best = p.Bits()
		}
	}
	if best < 0 {
		return v.BitLen()
	}
	return best
}

// parseAndVerify extracts the group specs from a config delivery and applies
// every cross-leaf rule. Pure: safe to call at verify time.
func parseAndVerify(sections []configSection) ([]GroupSpec, error) {
	specs, err := extractGroupSpecs(sections)
	if err != nil {
		return nil, err
	}
	if err := validateGroups(specs, ifaceBackend(sections)); err != nil {
		return nil, err
	}
	return specs, nil
}

// rxLoop feeds transport datagrams to the engine. One long-lived goroutine for
// the plugin's lifetime (ai/rules/goroutine-lifecycle.md), not one per packet.
func rxLoop(eng *engine) {
	for item := range sharedTransport.Receive() {
		eng.dispatchRx(item)
	}
}

// sharedTransport is the process-wide transport. One per plugin: it owns the
// per-instance sockets and the single delivery channel the rx loop drains.
var sharedTransport = transport.New(transport.NewBackend())

// livePlatform wires the engine to the real transport and iface component.
func livePlatform() enginePlatform {
	return enginePlatform{
		openInstance:  sharedTransport.OpenInstance,
		closeInstance: sharedTransport.CloseInstance,
		createMacvlan: func(dev, parent, owner string, mac [6]byte) error {
			if err := iface.RegisterOwnedMacvlan(owner, iface.MacvlanSpec{
				Name:   dev,
				Parent: parent,
				MAC:    macString(mac),
				// Private, not bridge: in bridge mode the parent wins the ARP-flux
				// race for the VIP and answers with its real MAC, so hosts never
				// learn the virtual MAC (proven in plan/learned/1122-vrrp-macvlan-vmac-dataplane.md QEMU probes).
				// Private isolation, together with the dataplane sysctls applied
				// below, makes the virtual-MAC device the sole ARP/ND responder.
				Mode: iface.MacvlanModePrivate,
			}); err != nil {
				return err
			}
			// RegisterOwnedMacvlan only records DESIRED state and pokes a
			// coalescing channel (iface/device_owner.go:59-89 ->
			// registryReconcileCh, iface/register.go:353); the device is created
			// later by reconcileOwnedDevices (iface/config_apply.go:1006). The
			// caller opens a transport that resolves this device BY NAME on the
			// next line, so returning at registration time hands it a name the
			// kernel does not have yet and every instance dies with "resolve
			// macvlan <dev>: no such network interface". Block until the device
			// actually exists, so "createMacvlan" means what it says.
			return waitDevicePresent(dev, macvlanCreateTimeout)
		},
		deleteMacvlan: func(owner, dev string) {
			iface.UnregisterOwnedMacvlan(owner, dev)
		},
		applyDataplane:    applyDataplaneSysctls,
		reassertDataplane: reassertDataplaneSysctls,
		revertDataplane:   revertDataplaneSysctls,
		parentIfindex: func(parent string) (int, error) {
			b, err := iface.Resolve(parent)
			if err != nil {
				return 0, err
			}
			return b.Ifindex, nil
		},
		counterSnapshot: sharedTransport.CounterSnapshot,
		resetCounters:   sharedTransport.ResetCounters,
	}
}

// macvlanCreateTimeout bounds the wait for iface's reconcile pass to create a
// registered macvlan. The pass is a channel wakeup plus a netlink round trip,
// so it lands in milliseconds; this ceiling exists only so a reconcile that
// never runs (a wedged worker, a backend that rejects the device) surfaces as a
// named error instead of hanging the config apply forever.
const macvlanCreateTimeout = 10 * time.Second

// macvlanPollInterval paces waitDevicePresent. There is no completion signal to
// subscribe to: the owned-device registry is desired-state and its reconcile
// pass reports to nobody (iface/device_owner.go:33-37 holds a bare trigger
// func), so a bounded poll is the honest way to observe the outcome. 20ms keeps
// the common case (device already there on the first or second probe) cheap; it
// costs nothing after create returns.
const macvlanPollInterval = 20 * time.Millisecond

// waitDevicePresent blocks until dev exists in the kernel, or timeout elapses.
//
// It probes with net.InterfaceByName deliberately: that is the exact call the
// transport's resolve makes (its failure surfaced as "route ip+net: no such
// network interface"), so a success here guarantees the caller's resolve sees
// the device too. iface.Resolve would be the wrong probe -- it caches by
// logical name (iface/resolve.go:84-97), and a hit there says nothing about
// whether this kernel device is present.
func waitDevicePresent(dev string, timeout time.Duration) error {
	return waitDevicePresentEvery(dev, timeout, macvlanPollInterval)
}

// waitDevicePresentEvery is waitDevicePresent with an explicit poll interval. It
// probes FIRST (before any sleep), so an already-present device returns on the
// first InterfaceByName without waiting a poll cycle. The interval is a
// parameter only so a test can prove the first-probe property deterministically:
// with a huge interval a first-probe hit still returns at once, while a
// sleep-before-probe regression would block for the whole interval. Production
// always passes macvlanPollInterval.
func waitDevicePresentEvery(dev string, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := net.InterfaceByName(dev); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("macvlan %s did not appear within %s: iface reconcile did not create it", dev, timeout)
		}
		time.Sleep(interval)
	}
}

// liveDeps wires the instance workers to the real transport and registries.
func liveDeps() engineDeps {
	return engineDeps{
		sendAdvert: func(key transport.InstanceKey, _ uint8, _ int) error {
			return sharedTransport.SendAdvert(key)
		},
		updateAdvert:   sharedTransport.UpdateAdvert,
		announceMaster: sharedTransport.AnnounceMaster,
		installVIPs:    iface.RegisterOwnedAddresses,
		removeVIPs: func(owner string) {
			iface.UnregisterOwnedAddresses(owner)
		},
		recordRxError:    sharedTransport.RecordRxError,
		emitState:        emitStateChange,
		parentReady:      parentReady,
		watchParent:      watchParent,
		refreshAddresses: sharedTransport.RefreshParentAddresses,
	}
}

// parentReady reports whether a unit's device can host a virtual router.
//
// Two conditions, both from the iface component's view of the kernel: the device
// is operationally up, and it holds an address of the group's family to source
// advertisements from (RFC 9568 Section 7.2 for IPv4; Section 5.1.2 requires the
// IPv6 source to be a link-local). The macvlan is deliberately NOT consulted:
// the kernel leaves its oper-state UP even when the parent dies, so it carries
// no liveness information (spec-vrrp-3 A-4, measured broken).
func parentReady(device, family string) bool {
	b, err := iface.Resolve(device)
	if err != nil {
		return false
	}
	if !strings.EqualFold(b.State, ifaceStateUp) {
		return false
	}
	addrs, err := iface.Addresses(device)
	if err != nil {
		return false
	}
	for _, a := range addrs {
		// Parse rather than trust AddrInfo.Family: the resolver's own
		// classifier only fills LinkLocal (resolve.go:228-240) and leaves
		// Family to whichever backend produced the list, so parsing keeps this
		// check independent of a convention we do not own.
		ip, perr := netip.ParseAddr(a.Address)
		if perr != nil {
			continue
		}
		if family == familyIPv4 && ip.Is4() {
			return true
		}
		// Any IPv6 address makes the parent v6-capable; the advertisement
		// source is the macvlan's link-local, which the transport resolves at
		// send time (RFC 9568 Section 5.1.2).
		if family == familyIPv6 && ip.Is6() {
			return true
		}
	}
	return false
}

// watchParent turns the iface resolver's link events into the coarse
// "something changed" notifications the instance re-evaluates readiness on.
//
// The channel is buffered and lossy on purpose: readiness is recomputed from
// current state on every wake-up, so a coalesced burst of events reaches the
// same answer as replaying each one.
func watchParent(device string) (<-chan struct{}, func()) {
	events, cancel := iface.Subscribe(device)
	out := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(out)
		for {
			select {
			case _, ok := <-events:
				if !ok {
					return
				}
				select {
				case out <- struct{}{}:
				default: // a wake-up is already pending; one is enough
				}
			case <-done:
				return
			}
		}
	}()
	return out, func() {
		close(done)
		cancel()
	}
}

const (
	ifaceStateUp   = "up"
	addrFamilyIPv4 = "ipv4"
	addrFamilyIPv6 = "ipv6"
)

// fsmStateNames keeps the fsm import used by emitStateChange's signature.
var _ = fsm.StateInitialize
