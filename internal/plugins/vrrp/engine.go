// RFC: rfc/short/rfc9568.md -- VRRPv3 virtual router identity (Sections 1.2, 7.3)
//
// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- instance manager: config diff + lifecycle
//
// The engine owns the set of running instances, keyed (interface, unit, family,
// vrid). Applying config is a diff: create what is new, reconfigure what changed
// in place, delete what is gone. Reconfiguring in place matters operationally --
// tearing an instance down to rebuild it would drop mastership and blackhole
// traffic for a full master-down interval over, say, a priority edit.
//
// Ordering within a create is load-bearing: the macvlan must exist before the
// transport opens, because the tx socket binds to that device to egress with the
// virtual MAC.
package vrrp

import (
	"fmt"
	"net/netip"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
)

// Device-name prefixes handed to iface.ComposeOwnedDeviceName. One per family,
// because a family's VRID namespace is its own (RFC 9568 Section 1.2) and both
// families may host the same VRID on one parent.
const (
	devPrefixV4 = "zv4"
	devPrefixV6 = "zv6"
)

// enginePlatform is the kernel-facing seam. Real wiring calls the transport and
// the iface component; tests substitute fakes so the whole manager is testable
// off-Linux.
type enginePlatform struct {
	openInstance      func(spec transport.InstanceSpec) (transport.InstanceKey, error)
	closeInstance     func(key transport.InstanceKey) error
	createMacvlan     func(dev, parent, owner string, mac [6]byte) error
	deleteMacvlan     func(owner, dev string)
	applyDataplane    func(parent, vmac, family string) error
	reassertDataplane func(parent, vmac, family string)
	revertDataplane   func(parent, vmac, family string)
	parentIfindex     func(parent string) (int, error)
	counterSnapshot   func(key transport.InstanceKey) (transport.CounterSnapshot, bool)
	resetCounters     func(key transport.InstanceKey)
}

// engine is the instance manager.
type engine struct {
	mu        sync.Mutex
	instances map[string]*instance

	clk      clock.Clock
	platform enginePlatform
	deps     engineDeps
}

func newEngine(clk clock.Clock, platform enginePlatform, deps engineDeps) *engine {
	return &engine{
		instances: make(map[string]*instance),
		clk:       clk,
		platform:  platform,
		deps:      deps,
	}
}

// apply diffs the desired group set against the running instances.
func (e *engine) apply(specs []GroupSpec) {
	e.mu.Lock()
	defer e.mu.Unlock()

	desired := make(map[string]GroupSpec, len(specs))
	for i := range specs {
		desired[specs[i].Key()] = specs[i]
	}

	// Delete first: a removed group must release its VRID, device, and sockets
	// before a new group could be created reusing them.
	for key, in := range e.instances {
		if _, keep := desired[key]; !keep {
			e.teardown(key, in)
		}
	}

	for key := range desired {
		spec := desired[key]
		if in, running := e.instances[key]; running {
			in.reconfigure(spec)
			continue
		}
		if err := e.create(key, spec); err != nil {
			logger().Error("vrrp: instance create failed",
				"interface", spec.Interface, "unit", spec.Unit, "family", spec.Family,
				"group", spec.Name, "vrid", spec.VRID, "error", err)
		}
	}

	// Re-assert the dataplane recipe for every running instance. This apply cycle
	// may have carried an iface config that re-emitted the parent's ARP sysctls
	// (config_sysctl.go), silently overwriting the values VRRP needs; re-writing
	// them here (idempotent, no refcount change) keeps VRRP the effective owner of
	// those knobs while a group is active. Runs for unchanged instances too, since
	// the clobber can come from an iface-only config change that leaves the VRRP
	// group set untouched.
	for _, in := range e.instances {
		e.platform.reassertDataplane(in.spec.ParentDevice, in.dev, in.spec.Family)
	}
}

// create brings up one instance: macvlan, then transport, then the worker.
func (e *engine) create(key string, spec GroupSpec) error {
	dev, err := deviceName(spec, e.platform.parentIfindex)
	if err != nil {
		return err
	}
	mac := packet.VirtualMAC(familyCode(spec.Family), spec.VRID)
	owner := ownerString(dev)

	// Everything kernel-facing hangs off the unit's DEVICE, never the logical
	// interface name: a group on a VLAN unit lives on eth0.100, and binding
	// eth0 instead would advertise into the wrong broadcast domain. Using the
	// device also keeps the transport key unique, since two units of one
	// interface are two devices (and could otherwise share an InstanceKey).
	if err := e.platform.createMacvlan(dev, spec.ParentDevice, owner, mac); err != nil {
		return fmt.Errorf("create macvlan %s on %s: %w", dev, spec.ParentDevice, err)
	}

	// Make the virtual MAC the sole ARP/ND responder for the VIP. Ordered here,
	// after the device exists and before the worker starts, so a host resolving
	// the VIP once the instance advertises already gets the virtual MAC (the
	// parent sysctls settle well before promotion). See dataplane_linux.go.
	if err := e.platform.applyDataplane(spec.ParentDevice, dev, spec.Family); err != nil {
		e.platform.deleteMacvlan(owner, dev)
		return fmt.Errorf("apply dataplane for %s: %w", dev, err)
	}

	tkey, err := e.platform.openInstance(transport.InstanceSpec{
		Family:        familyCode(spec.Family),
		VRID:          spec.VRID,
		Parent:        spec.ParentDevice,
		MacvlanDevice: dev,
		VirtualMAC:    mac,
	})
	if err != nil {
		e.platform.revertDataplane(spec.ParentDevice, dev, spec.Family)
		e.platform.deleteMacvlan(owner, dev)
		return fmt.Errorf("open transport for %s: %w", dev, err)
	}

	in := newInstance(spec, tkey, dev, e.clk, e.deps)
	e.instances[key] = in
	go in.run()
	return nil
}

// teardown stops a worker and releases everything it owns. The worker's Shutdown
// runs first so a Master sends its Priority-0 advertisement (RFC 9568 Section
// 6.4.3) while its sockets are still open.
func (e *engine) teardown(key string, in *instance) {
	in.shutdown()
	if err := e.platform.closeInstance(in.key); err != nil {
		logger().Warn("vrrp: close transport instance failed",
			"interface", in.spec.Interface, "group", in.spec.Name, "vrid", in.spec.VRID, "error", err)
	}
	e.platform.revertDataplane(in.spec.ParentDevice, in.dev, in.spec.Family)
	e.platform.deleteMacvlan(in.own, in.dev)
	// Drop the state gauge with the instance: a deleted group that kept
	// reporting "master" would be worse than reporting nothing.
	clearMetrics(in.spec)
	delete(e.instances, key)
}

// stopAll tears every instance down (plugin shutdown).
func (e *engine) stopAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for key, in := range e.instances {
		e.teardown(key, in)
	}
}

// dispatchRx routes one received datagram to the instance whose socket received
// it, which the transport stamps on the item.
//
// Routing on the key (not on the packet's VRID) is what keeps the copies honest:
// each instance has its own socket joined to the group, so one advertisement on
// the wire arrives once per instance, and each copy belongs to exactly the
// instance that received it. That instance's own Decode then rejects any VRID
// that is not its own (spec-vrrp-5 D-B: the engine owns Decode because it holds
// the group table).
func (e *engine) dispatchRx(item transport.RxItem) {
	e.mu.Lock()
	target := e.byTransportKey(item.Key)
	e.mu.Unlock()

	if target == nil {
		// The instance was torn down between the readLoop's delivery and here.
		return
	}
	target.onPacket(item)
}

// byTransportKey finds the instance owning a transport key. Callers hold e.mu.
func (e *engine) byTransportKey(key transport.InstanceKey) *instance {
	for _, in := range e.instances {
		if in.key == key {
			return in
		}
	}
	return nil
}

// snapshots returns every instance's show view, sorted for stable output.
func (e *engine) snapshots() []instanceView {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]instanceView, 0, len(e.instances))
	for _, in := range e.instances {
		out = append(out, in.snapshot())
	}
	sortViews(out)
	return out
}

// snapshotsForInterface returns the views for one parent interface.
func (e *engine) snapshotsForInterface(name string) []instanceView {
	all := e.snapshots()
	out := make([]instanceView, 0, len(all))
	for i := range all {
		if all[i].Interface == name {
			out = append(out, all[i])
		}
	}
	return out
}

// statistics returns every instance's counters, merging the transport's wire
// counters with the engine-owned Priority-0 counts (D-F).
func (e *engine) statistics() []statisticsView {
	e.mu.Lock()
	instances := make([]*instance, 0, len(e.instances))
	for _, in := range e.instances {
		instances = append(instances, in)
	}
	e.mu.Unlock()

	out := make([]statisticsView, 0, len(instances))
	for _, in := range instances {
		out = append(out, in.statistics(e.platform.counterSnapshot))
	}
	sortStats(out)
	return out
}

// clearStatistics resets every instance's counters and reports how many were
// cleared. State is untouched: clearing counters must never perturb the
// protocol.
func (e *engine) clearStatistics() int {
	e.mu.Lock()
	instances := make([]*instance, 0, len(e.instances))
	for _, in := range e.instances {
		instances = append(instances, in)
	}
	e.mu.Unlock()

	for _, in := range instances {
		in.clearCounters(e.platform.resetCounters)
	}
	return len(instances)
}

// sortStats orders statistics output like the show views.
func sortStats(stats []statisticsView) {
	sort.Slice(stats, func(i, j int) bool {
		return statsKey(stats[i]) < statsKey(stats[j])
	})
}

func statsKey(s statisticsView) string {
	var tb textbuf.Buffer
	return tb.Str(s.Interface).Byte('/').Str(s.Unit).Byte('/').Str(s.Family).Byte('/').Uint8(s.VRID).String()
}

// deviceName composes this instance's macvlan name.
//
// iface.ComposeOwnedDeviceName enforces the 15-char IFNAMSIZ budget and rejects
// rather than truncates, so a name collision can never silently point two
// instances at one device.
func deviceName(spec GroupSpec, parentIfindex func(string) (int, error)) (string, error) {
	idx, err := parentIfindex(spec.ParentDevice)
	if err != nil {
		return "", fmt.Errorf("resolve parent %s: %w", spec.ParentDevice, err)
	}
	prefix := devPrefixV4
	if spec.Family == familyIPv6 {
		prefix = devPrefixV6
	}
	dev, err := iface.ComposeOwnedDeviceName(prefix, idx, int(spec.VRID))
	if err != nil {
		return "", fmt.Errorf("compose device name for %s vrid %d: %w", spec.Interface, spec.VRID, err)
	}
	return dev, nil
}

// familyCode maps the config family string to the codec's family constant.
func familyCode(family string) uint8 {
	if family == familyIPv6 {
		return packet.V6
	}
	return packet.V4
}

// sortViews orders show output by interface, unit, family, vrid.
func sortViews(views []instanceView) {
	sort.Slice(views, func(i, j int) bool {
		return viewKey(views[i]) < viewKey(views[j])
	})
}

func viewKey(v instanceView) string {
	var tb textbuf.Buffer
	return tb.Str(v.Interface).Byte('/').Str(v.Unit).Byte('/').Str(v.Family).Byte('/').Uint8(v.VRID).String()
}

// vipStrings renders an address list for logs.
func vipStrings(vips []netip.Addr) string {
	var tb textbuf.Buffer
	for i, v := range vips {
		if i > 0 {
			tb.Byte(',')
		}
		tb.Addr(v)
	}
	return tb.String()
}

// emitStateChange is the default EmitStateChange executor: log + metrics +
// eventbus. Wired by register.go.
func emitStateChange(spec GroupSpec, from, to fsm.State, reason string) {
	logger().Info("vrrp: state change",
		"interface", spec.Interface, "unit", spec.Unit, "family", spec.Family, "group", spec.Name, "vrid", spec.VRID,
		"from", viewState(from), "to", viewState(to), "reason", reason,
		"virtual-addresses", vipStrings(spec.VIPs))
	recordTransition(spec, to)
	publishStateChange(spec, from, to, reason)
}
