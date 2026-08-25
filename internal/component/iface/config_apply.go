// Design: docs/features/interfaces.md -- Interface reconciliation and application
// Related: config.go -- parsing, config_sysctl.go -- sysctl/mirror

package iface

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// desiredState builds a map of OS interface name -> desired addresses from config.
// Also returns the set of Ze-managed interface names (dummy, veth, bridge, VLAN)
// that should exist. Physical interfaces (ethernet) are never in the managed set.
//
// devices is bindDevices' answer for this config: it turns each logical
// entry name into the kernel device the entry's hardware selector names, so
// every key here is a device the operator selected rather than a name they
// chose. An entry whose selector is unbound contributes nothing, which is how a
// deferred binding stays deferred instead of claiming a device that merely
// shares its name.
//
// staleNames is address_owner.go's ownedAddresses() staleNames, passed
// through unchanged -- see clearStaleIfaces for why callers that reconcile
// against this exact snapshot must use it, not a fresh read of the live
// registry, to clear staleIfaces afterward.
func (cfg *ifaceConfig) desiredState(devices map[string]string) (addrs map[string]map[string]bool, managed map[string]bool, staleNames []string) {
	addrs = make(map[string]map[string]bool)
	managed = make(map[string]bool)

	addIfaceAddrs := func(name string, units []unitEntry) {
		device, bound := deviceFor(devices, name)
		if !bound {
			return
		}
		for i := range units {
			u := &units[i]
			if u.Disable {
				continue
			}
			osName := unitOSName(device, u)
			if u.VLANID > 0 {
				managed[osName] = true
			}
			if addrs[osName] == nil {
				addrs[osName] = make(map[string]bool)
			}
			for _, a := range u.Addresses {
				addrs[osName][a] = true
			}
		}
	}

	for _, e := range cfg.Dummy {
		if e.Disable {
			continue
		}
		managed[e.Name] = true
		addIfaceAddrs(e.Name, e.Units)
	}
	for _, e := range cfg.Veth {
		if e.Disable {
			continue
		}
		managed[e.Name] = true
		addIfaceAddrs(e.Name, e.Units)
	}
	for _, e := range cfg.Bridge {
		if e.Disable {
			continue
		}
		managed[e.Name] = true
		addIfaceAddrs(e.Name, e.Units)
	}
	for i := range cfg.Tunnel {
		e := &cfg.Tunnel[i]
		if e.Disable {
			continue
		}
		managed[e.Name] = true
		addIfaceAddrs(e.Name, e.Units)
	}
	for i := range cfg.Wireguard {
		e := &cfg.Wireguard[i]
		if e.Disable {
			continue
		}
		managed[e.Name] = true
		addIfaceAddrs(e.Name, e.Units)
	}
	for i := range cfg.XFRM {
		e := &cfg.XFRM[i]
		if e.Disable {
			continue
		}
		managed[e.Name] = true
		addIfaceAddrs(e.Name, e.Units)
	}
	for _, e := range cfg.Ethernet {
		if e.Disable {
			continue
		}
		addIfaceAddrs(e.Name, e.Units)
	}
	if cfg.Loopback != nil {
		for i := range cfg.Loopback.Units {
			u := &cfg.Loopback.Units[i]
			if u.Disable {
				continue
			}
			if addrs["lo"] == nil {
				addrs["lo"] = make(map[string]bool)
			}
			for _, a := range u.Addresses {
				addrs["lo"][a] = true
			}
		}
	}

	// Merge plugin-registered addresses (address_owner.go) on top of the
	// YANG-derived set. A registered owner's addresses are desired for as
	// long as the registration exists, without a duplicated YANG
	// declaration; an address present in both sources counts once.
	owned, staleNames := ownedAddresses()
	for ifaceName, ownedAddrs := range owned {
		if addrs[ifaceName] == nil {
			addrs[ifaceName] = make(map[string]bool, len(ownedAddrs))
		}
		for a := range ownedAddrs {
			addrs[ifaceName][a] = true
		}
	}

	return addrs, managed, staleNames
}

// currentAddrSet builds a map of OS interface name -> set of current CIDR addresses.
func currentAddrSet(infos []InterfaceInfo) map[string]map[string]bool {
	result := make(map[string]map[string]bool)
	for i := range infos {
		if len(infos[i].Addresses) == 0 {
			continue
		}
		m := make(map[string]bool, len(infos[i].Addresses))
		for _, a := range infos[i].Addresses {
			var bCidr textbuf.Buffer
			cidr := bCidr.Reset().Str(a.Address).Byte('/').Int(int64(a.PrefixLength)).String()
			m[cidr] = true
		}
		result[infos[i].Name] = m
	}
	return result
}

// currentIfaceSet builds a set of OS interface names by type.
func currentIfaceSet(infos []InterfaceInfo) map[string]string {
	result := make(map[string]string, len(infos))
	for i := range infos {
		result[infos[i].Name] = infos[i].Type
	}
	return result
}

// zeManageable returns true if the interface type is one Ze creates/deletes
// (not physical ethernet or loopback).
func zeManageable(linkType string) bool {
	switch linkType {
	case zeTypeDummy, zeTypeVeth, zeTypeBridge, zeTypeWireguard, "vlan":
		return true
	}
	return kernelTunnelKinds[linkType]
}

// rememberPreviousManaged records the names Ze owned in the last successfully
// applied config. Reconcile uses this runtime-only set as its deletion scope:
// a manageable kernel link is eligible for deletion only if Ze managed it
// before and it disappeared from the new config.
//
// previousDevices is bindDevices' answer for the PREVIOUS config, so a VLAN
// made on a selected parent is remembered under the kernel name it carries. An
// entry whose selector is unbound is remembered as owning nothing, which is the
// safe direction: the prune deletes nothing it cannot name, and a guess here
// deletes an operator's device.
func (cfg *ifaceConfig) rememberPreviousManaged(previous *ifaceConfig, previousDevices map[string]string) {
	cfg.previousManaged = nil
	if previous == nil {
		return
	}
	_, managed, _ := previous.desiredState(previousDevices)
	cfg.previousManaged = managed
}

// removedManagedNames returns the subset of previousManaged that is no longer
// present in currentManaged.
func removedManagedNames(previousManaged, currentManaged map[string]bool) map[string]bool {
	if len(previousManaged) == 0 {
		return nil
	}
	removed := make(map[string]bool)
	for name := range previousManaged {
		if currentManaged[name] {
			continue
		}
		removed[name] = true
	}
	return removed
}

// unitOSName returns the OS device name a unit configures on kernel device
// osName: the device itself for an untagged unit, "<device>.<vlan>" for a VLAN
// unit. The caller passes the kernel device the entry resolved to, never the
// logical entry name, because both backends compose the VLAN netdev name from
// the parent they are handed -- netlinkBackend.CreateVLAN and
// vppBackendImpl.CreateVLAN each build "<spec.Parent>.<VLANID>", and VLANSpec
// carries no name of its own. A VLAN on a selected parent therefore IS named
// after the hardware, and naming it anything else names a device that does not
// exist.
func unitOSName(osName string, u *unitEntry) string {
	if u.VLANID <= 0 {
		return osName
	}
	var b textbuf.Buffer
	return b.Reset().Str(osName).Byte('.').Int(int64(u.VLANID)).String()
}

// deviceFor returns the kernel device a logical interface name configures, and
// whether the name is bound to one at all. Only ethernet entries carry a
// hardware selector, so a name absent from devices is its own kernel device.
// A name mapped to the empty string selected hardware that is not present (or
// that more than one device answers to): it is UNBOUND, and every apply phase
// skips it rather than falling back to the logical name. That fallback is how
// an address reaches a device which merely shares the entry's name.
func deviceFor(devices map[string]string, name string) (device string, bound bool) {
	device, selected := devices[name]
	if !selected {
		return name, true
	}
	return device, device != ""
}

// bindDevices maps the logical name of every ethernet entry to the kernel
// device the apply path must act on, derived from one interface listing.
// mac/match binds to the device whose match MAC the selector names; os-name
// aliases a kernel device by name; an entry with neither binds to its own name.
// Ethernet is the only kind that carries a selector and the only kind Ze does
// not create, so it is the only kind in this map: every other name is its own
// kernel device and is absent here.
//
// A DISABLED entry is bound like any other. Disabling an interface says what ze
// must do with it. It never says which device it is. The reconcile phases need
// the device precisely because the entry is disabled. mirrorScope retires the
// mirror on a disabled interface. It can name that mirror only by the device
// the selector answers with.
//
// Leaving a disabled entry out of this map made
// deviceFor fall back to the logical name. A disabled
// `ethernet uplink { os-name eth3; disable; }` then left its real mirror on
// eth3 standing. It also claimed authority over any kernel device called
// `uplink`. Every apply phase filters on Disable itself, so none of them sees
// a difference.
//
// An entry maps to the empty string -- UNBOUND -- when no device answers its
// selector, when several do, or when the device it names is not present. Every
// apply phase skips an unbound entry. None falls back to the logical name,
// which is what the YANG promises for an absent binding ("the binding defers
// until it appears") and what keeps a device that merely shares the entry's
// name from being configured in its place.
//
// infos is one interface listing, taken once per apply. A nil listing (the
// backend could not answer) leaves every entry unbound, so an apply that cannot
// see the hardware defers rather than inventing a binding.
func (cfg *ifaceConfig) bindDevices(infos []InterfaceInfo) map[string]string {
	if cfg == nil || len(cfg.Ethernet) == 0 {
		return nil
	}
	present := make(map[string]struct{}, len(infos))
	for i := range infos {
		present[infos[i].Name] = struct{}{}
	}
	devices := make(map[string]string, len(cfg.Ethernet))
	for i := range cfg.Ethernet {
		e := &cfg.Ethernet[i]
		device := e.Name
		switch {
		case e.MatchMAC != "":
			matched := devicesWithMAC(infos, e.MatchMAC)
			if len(matched) != 1 {
				devices[e.Name] = ""
				continue
			}
			device = infos[matched[0]].Name
		case e.OSName != "" && e.OSName != e.Name:
			device = e.OSName
		}
		if _, ok := present[device]; !ok {
			devices[e.Name] = ""
			continue
		}
		devices[e.Name] = device
	}
	return devices
}

// validateSelectors refuses a mac/match selector that more than one present
// device answers to. Nothing distinguishes the candidates, so binding to one is
// a guess about which physical port the entry's addresses, MTU and admin state
// land on. Returns nil when every selector names at most one device, including
// when it names none -- an absent device is a deferred binding, which the YANG
// promises, not an error.
func (cfg *ifaceConfig) validateSelectors(infos []InterfaceInfo) error {
	if cfg == nil {
		return nil
	}
	for i := range cfg.Ethernet {
		e := &cfg.Ethernet[i]
		if e.Disable || e.MatchMAC == "" {
			continue
		}
		matched := devicesWithMAC(infos, e.MatchMAC)
		if len(matched) <= 1 {
			continue
		}
		names := make([]string, 0, len(matched))
		for _, idx := range matched {
			names = append(names, infos[idx].Name)
		}
		sort.Strings(names)
		return fmt.Errorf("ethernet %q: MAC %s is carried by %d devices (%s); a hardware MAC selects at most one device",
			e.Name, normalizeMAC(e.MatchMAC), len(names), strings.Join(names, ", "))
	}
	return nil
}

// devicesWithMAC returns the index in infos of every device whose match MAC
// equals want and that a hardware selector can name. deviceMatchMAC decides
// which address a device is matched on: its permanent (factory) address when it
// reports one, else its current address, so a binding survives an operational
// MAC override.
//
// A device is a candidate only when the address it is matched on is its OWN,
// and that exclusion is load-bearing rather than tidy: the selector names one
// physical port, so a second device wearing that port's address makes the
// binding ambiguous and fails the whole apply. Linux hands a device an address
// it did not bring in two ways, and both are excluded here.
//
//   - It hangs off a parent (IFLA_LINK). A VLAN sub-interface inherits its
//     parent's hardware address, so the moment ze creates one on a mac/match
//     parent, TWO devices carry the selector's MAC. Measured against a live
//     kernel: creating zesel0.100 on zesel0 made the next resolution of
//     zesel0's selector refuse, and its addresses were never applied.
//   - It aggregates members (IFLA_MASTER). A bridge with no permanent address
//     of its own wears its lowest-MAC port's address, and a bond master wears a
//     slave's while the slave keeps its own IFLA_PERM_ADDRESS. Measured against
//     a live kernel: enslaving a device with MAC 02:00:00:00:be:99 to a bridge
//     made the bridge report that same address, so an ethernet entry selecting
//     that MAC and listing itself as a bridge member was refused from the apply
//     after the one that created the bridge.
func devicesWithMAC(infos []InterfaceInfo, want string) []int {
	target := normalizeMAC(want)
	if target == "" {
		return nil
	}
	aggregators := aggregatingDevices(infos)
	var matched []int
	for i := range infos {
		if isStackedDevice(&infos[i]) {
			continue
		}
		if _, aggregates := aggregators[infos[i].Index]; aggregates {
			continue
		}
		if normalizeMAC(deviceMatchMAC(&infos[i])) == target {
			matched = append(matched, i)
		}
	}
	return matched
}

// isStackedDevice reports whether info describes a device built on top of
// another one, whose hardware address is therefore its parent's rather than its
// own. The netlink backend sets ParentIndex for the two kinds that inherit an
// address, vlan and macvlan (show_linux.go linkToInfo); the VPP backend reports
// SupSwIfIndex for every interface, and a top-level one names itself, which is
// why self-parenthood does not count.
func isStackedDevice(info *InterfaceInfo) bool {
	return info.ParentIndex != 0 && info.ParentIndex != info.Index
}

// aggregatingDevices returns the index of every device in infos that another
// device names as its master (IFLA_MASTER): a bridge that has a port, a bond
// that has a slave. Such a device wears a member's hardware address rather than
// one of its own, which is why a hardware selector must not name it.
//
// Membership is read from the members rather than from the link type, because
// the type says what a device is and only the membership says whose address it
// wears. A bridge with no port keeps the address the kernel gave it at
// creation, no member claims that address, and this returns nothing for it.
//
// The map stays nil until a member is seen, so the common listing -- no bridge,
// no bond -- allocates nothing on a path walked once per ethernet entry.
func aggregatingDevices(infos []InterfaceInfo) map[int]struct{} {
	var masters map[int]struct{}
	for i := range infos {
		master := infos[i].MasterIndex
		if master == 0 || master == infos[i].Index {
			continue
		}
		if masters == nil {
			masters = make(map[int]struct{})
		}
		masters[master] = struct{}{}
	}
	return masters
}

// allIfaceEntries returns every interface entry in the config, across the
// families that carry per-unit settings. Order follows the apply phases:
// ethernet and dummy first, then the created kinds.
func allIfaceEntries(cfg *ifaceConfig) []ifaceEntry {
	if cfg == nil {
		return nil
	}
	entries := make([]ifaceEntry, 0, len(cfg.Ethernet)+len(cfg.Dummy)+len(cfg.Veth)+len(cfg.Bridge)+len(cfg.Tunnel)+len(cfg.Wireguard)+len(cfg.XFRM))
	entries = append(entries, cfg.Ethernet...)
	entries = append(entries, cfg.Dummy...)
	for _, e := range cfg.Veth {
		entries = append(entries, e.ifaceEntry)
	}
	for _, e := range cfg.Bridge {
		entries = append(entries, e.ifaceEntry)
	}
	for i := range cfg.Tunnel {
		entries = append(entries, cfg.Tunnel[i].ifaceEntry)
	}
	for i := range cfg.Wireguard {
		entries = append(entries, cfg.Wireguard[i].ifaceEntry)
	}
	for i := range cfg.XFRM {
		entries = append(entries, cfg.XFRM[i].ifaceEntry)
	}
	return entries
}

// indexTunnelSpecs returns a name -> Spec map for the previous config's
// tunnel entries. Used by applyConfig to detect Spec changes across reloads
// so that only changed tunnels are recreated. Returns an empty map if
// previous is nil (first apply).
func indexTunnelSpecs(previous *ifaceConfig) map[string]TunnelSpec {
	if previous == nil {
		return nil
	}
	specs := make(map[string]TunnelSpec, len(previous.Tunnel))
	for i := range previous.Tunnel {
		e := &previous.Tunnel[i]
		specs[e.Name] = e.Spec
	}
	return specs
}

// indexWireguardSpecs returns a name -> Spec map for the previous config's
// wireguard entries. Used by applyConfig to decide whether a reload needs
// to touch the kernel at all: if the Spec is unchanged, the netdev and
// peer set are already correct and ConfigureWireguardDevice can be skipped.
func indexWireguardSpecs(previous *ifaceConfig) map[string]WireguardSpec {
	if previous == nil {
		return nil
	}
	specs := make(map[string]WireguardSpec, len(previous.Wireguard))
	for i := range previous.Wireguard {
		e := &previous.Wireguard[i]
		specs[e.Name] = e.Spec
	}
	return specs
}

// wireguardSpecEqual reports whether two WireguardSpec values describe
// the same desired kernel state. Slice fields (Peers, AllowedIPs) prevent
// the direct == comparison that tunnelEntry uses, so the helper does a
// field-by-field walk that treats the Peer list as an unordered set
// keyed by public-key.
func wireguardSpecEqual(a, b WireguardSpec) bool {
	if a.Name != b.Name {
		return false
	}
	if a.PrivateKey != b.PrivateKey {
		return false
	}
	if a.ListenPortSet != b.ListenPortSet || a.ListenPort != b.ListenPort {
		return false
	}
	if a.FirewallMark != b.FirewallMark {
		return false
	}
	if len(a.Peers) != len(b.Peers) {
		return false
	}
	byKeyA := make(map[WireguardKey]*WireguardPeerSpec, len(a.Peers))
	for i := range a.Peers {
		byKeyA[a.Peers[i].PublicKey] = &a.Peers[i]
	}
	for i := range b.Peers {
		pa, ok := byKeyA[b.Peers[i].PublicKey]
		if !ok {
			return false
		}
		if !wireguardPeerEqual(*pa, b.Peers[i]) {
			return false
		}
	}
	return true
}

// wireguardPeerEqual reports whether two peer specs describe the same
// kernel-visible peer configuration. Name is excluded because it is a
// config-file label, not part of the kernel state.
func wireguardPeerEqual(a, b WireguardPeerSpec) bool {
	if a.PublicKey != b.PublicKey {
		return false
	}
	if a.HasPresharedKey != b.HasPresharedKey {
		return false
	}
	if a.HasPresharedKey && a.PresharedKey != b.PresharedKey {
		return false
	}
	if a.EndpointIP != b.EndpointIP || a.EndpointPort != b.EndpointPort {
		return false
	}
	if a.PersistentKeepalive != b.PersistentKeepalive {
		return false
	}
	if a.Disable != b.Disable {
		return false
	}
	if len(a.AllowedIPs) != len(b.AllowedIPs) {
		return false
	}
	setA := make(map[string]struct{}, len(a.AllowedIPs))
	for _, cidr := range a.AllowedIPs {
		setA[cidr] = struct{}{}
	}
	for _, cidr := range b.AllowedIPs {
		if _, ok := setA[cidr]; !ok {
			return false
		}
	}
	return true
}

func indexXFRMSpecs(previous *ifaceConfig) map[string]XFRMSpec {
	if previous == nil {
		return nil
	}
	specs := make(map[string]XFRMSpec, len(previous.XFRM))
	for i := range previous.XFRM {
		e := &previous.XFRM[i]
		specs[e.Name] = e.Spec
	}
	return specs
}

func xfrmSpecEqual(a, b XFRMSpec) bool {
	return a.Name == b.Name && a.IfID == b.IfID && a.PhysicalDev == b.PhysicalDev
}

// applyAndPublish publishes cfg's hardware-selector mapping to the shared
// resolver and then applies cfg. The order is the contract, and it is why the
// two calls have one name: the apply is itself a resolver consumer -- every
// by-name dispatch op it reaches translates through the resolver, and so does
// every consumer that reacts to what it does -- so publishing afterwards runs
// the apply against the mapping of the commit before it.
//
// Every path that applies a config goes through here, the rollback re-apply
// included: leaving a refused config's mapping published would serve bindings
// no committed config asked for.
func applyAndPublish(cfg, previous *ifaceConfig, b Backend) []error {
	setResolverConfig(cfg)
	return applyConfig(cfg, previous, b)
}

// applyConfig applies the parsed interface config declaratively via the backend.
// 1. Creates missing Ze-managed interfaces (dummy, veth, tunnel, xfrm, bridge, VLAN)
// 2. Sets properties (MTU, MAC, sysctl, mirror) on all configured interfaces
// 3. Adds missing addresses, removes extra addresses on configured interfaces
// 4. Deletes Ze-managed interfaces not in config.
//
// previous is the last successfully applied config, or nil on first apply.
// It is used to detect tunnels whose Spec changed across the reload, so that
// only those tunnels are deleted-and-recreated; tunnels with unchanged Spec
// stay up across SIGHUP, preserving any traffic flowing through them.
//
// previous describes this process, and a netdev outlives the process that made
// it. An apply with no previous spec for a name -- daemon start, plugin start,
// the second apply of a reload that starts the plugin -- therefore meets links
// an earlier run created, or links an operator made. Every create step below
// treats a name already held as success and keeps the link. The tunnel step
// keeps it only when the device is a tunnel of the configured kind and fails
// the apply otherwise, since keeping a device of another kind would hand the
// later phases something that is not the configured interface.
//
// Returns collected errors. The first mutating failure aborts the apply and
// rolls back successful steps that have an exact inverse.
func applyConfig(cfg, previous *ifaceConfig, b Backend) []error {
	log := loggerPtr.Load()
	var errs []error
	journal := sdk.NewJournal()

	rollbackPartial := func() []error {
		for _, err := range journal.Rollback() {
			log.Warn("iface config: rollback partial apply", "err", err)
			errs = append(errs, fmt.Errorf("rollback partial apply: %w", err))
		}
		return errs
	}
	record := func(msg string, err error) []error {
		log.Warn(msg, "err", err)
		errs = append(errs, fmt.Errorf("%s: %w", msg, err))
		return rollbackPartial()
	}

	// Phase 1: Create missing interfaces.
	//
	// Order matters: tunnels are created BEFORE bridges so a bridge can
	// list a gretap tunnel in its `member` block on a fresh start. Bridges
	// are still created AFTER veths and dummies for the same reason.
	for _, e := range cfg.Dummy {
		if e.Disable {
			continue
		}
		created := false
		if err := applyBackendStep(journal, func() error {
			if err := b.CreateDummy(e.Name); err != nil {
				// ErrInterfaceExists is the backend saying it kept the
				// interface an earlier apply made. GetInterface cannot
				// settle that case on every backend: the vpp one names a
				// loopback loopN in the dataplane while the operator names
				// it here, so the dump filter finds nothing. Either way
				// this step created nothing, so created stays false and
				// the undo leaves the device alone.
				if errors.Is(err, ErrInterfaceExists) {
					return nil
				}
				if _, getErr := b.GetInterface(e.Name); getErr != nil {
					return err
				}
				return nil
			}
			created = true
			return nil
		}, func() error {
			if !created {
				return nil
			}
			return b.DeleteInterface(e.Name)
		}); err != nil {
			return record("dummy "+e.Name+" create", err)
		}
		// On the vpp backend, shadow the loopback into Linux with an LCP pair
		// so kernel networking (the ze BGP listener) can bind on it.
		// SetupLCPPair no-ops when LCP is disabled; the netlink backend never
		// reaches here because this is gated on the vpp backend.
		if cfg.Backend == vppBackendName {
			name := e.Name
			if err := applyBackendStep(journal, func() error {
				return b.SetupLCPPair(name, name)
			}, func() error {
				return b.RemoveLCPPair(name)
			}); err != nil {
				var tb textbuf.Buffer
				return record(tb.Str("dummy ").Str(name).Str(" lcp pair").String(), err)
			}
		}
	}
	for _, e := range cfg.Veth {
		if e.Disable {
			continue
		}
		peer := e.Peer
		if peer == "" {
			peer = e.Name + "-peer"
		}
		created := false
		if err := applyBackendStep(journal, func() error {
			if err := b.CreateVeth(e.Name, peer); err != nil {
				if _, getErr := b.GetInterface(e.Name); getErr != nil {
					return err
				}
				return nil
			}
			created = true
			return nil
		}, func() error {
			if !created {
				return nil
			}
			return b.DeleteInterface(e.Name)
		}); err != nil {
			return record("veth "+e.Name+" create", err)
		}
	}
	previousTunnelSpecs := indexTunnelSpecs(previous)
	for i := range cfg.Tunnel {
		e := &cfg.Tunnel[i]
		if e.Disable {
			continue
		}
		prev, hadPrev := previousTunnelSpecs[e.Name]
		if hadPrev && prev == e.Spec {
			// Spec unchanged: keep the existing netdev. Phase 2 still
			// applies MTU/MAC and Phase 3 reconciles addresses, so any
			// non-Spec change still takes effect.
			continue
		}
		if hadPrev {
			// Spec changed: delete-then-create. Linux does not support
			// modifying most tunnel kinds in place; this matches VyOS's
			// behavior for gretap/ip6gretap.
			deleted := false
			if err := applyBackendStep(journal, func() error {
				if err := b.DeleteInterface(e.Name); err != nil {
					if !interfaceExists(b, e.Name) {
						log.Debug("iface config: tunnel already absent before recreate",
							"name", e.Name, "err", err)
						return nil
					}
					return err
				}
				deleted = true
				return nil
			}, func() error {
				if !deleted {
					return nil
				}
				return b.CreateTunnel(prev)
			}); err != nil {
				return record("tunnel "+e.Name+" delete before recreate", err)
			}
		}
		created := false
		if err := applyBackendStep(journal, func() error {
			if err := b.CreateTunnel(e.Spec); err != nil {
				// What holds the name decides. Nothing holds it: the
				// create failed on its own merits, so report that error. A
				// tunnel of this kind holds it: the kernel answered EEXIST
				// for a netdev that outlived the process which made it, so
				// keep it as the five sibling steps keep theirs, since
				// rebuilding an identical link would break the traffic
				// crossing it. created stays false, so the rollback below
				// cannot delete a netdev this pass did not make. Any other
				// device holds it, a tunnel of another kind included: fail
				// the apply, as reconcileOwnedDevices does on that state,
				// because Phases 2, 2c and 3 would otherwise push this
				// tunnel's MTU, admin state and addresses onto a device
				// that is not it. An operator who edits the encapsulation
				// while ze is down reaches that failure, as they did
				// before this branch existed: previous is nil at plugin
				// start, so the edit never reaches the delete-then-create
				// branch above.
				//
				// Only the KIND is checked. InterfaceInfo carries no
				// encapsulation field, so the WARN says the parameters
				// were not verified rather than claiming the kept link
				// agrees with the config.
				info, getErr := b.GetInterface(e.Name)
				if getErr != nil {
					return err
				}
				wantType, known := e.Spec.Kind.kernelLinkType()
				if info == nil || !known || info.Type != wantType {
					return fmt.Errorf("%w: %q is held by a device of type %s, not by a %s tunnel",
						err, e.Name, existingLinkType(info), e.Spec.Kind)
				}
				log.Warn("iface config: kept existing tunnel netdev, parameters not verified",
					"name", e.Name, "kind", e.Spec.Kind.String())
				return nil
			}
			created = true
			return nil
		}, func() error {
			if !created {
				return nil
			}
			return b.DeleteInterface(e.Name)
		}); err != nil {
			return record("tunnel "+e.Name+" create", err)
		}
	}
	previousWireguardSpecs := indexWireguardSpecs(previous)
	for i := range cfg.Wireguard {
		e := &cfg.Wireguard[i]
		if e.Disable {
			continue
		}
		prev, hadPrev := previousWireguardSpecs[e.Name]
		if hadPrev && wireguardSpecEqual(prev, e.Spec) {
			// Spec unchanged: keep the existing netdev and peer set.
			// Phase 2 still applies MTU and Phase 3 reconciles addresses,
			// so any non-Spec change still takes effect.
			continue
		}
		if !hadPrev {
			// New wireguard interface: create the netdev first.
			created := false
			if err := applyBackendStep(journal, func() error {
				if err := b.CreateWireguardDevice(e.Name); err != nil {
					// CreateWireguardDevice fails on "already exists" when
					// the previous-state tracker is stale. That is harmless.
					// A genuine failure (e.g. missing kernel module) means
					// we must skip ConfigureWireguardDevice -- there is
					// nothing to configure.
					if _, getErr := b.GetInterface(e.Name); getErr != nil {
						return err
					}
					log.Debug("iface config: create wireguard (already exists)",
						"name", e.Name, "err", err)
					return nil
				}
				created = true
				return nil
			}, func() error {
				if !created {
					return nil
				}
				return b.DeleteInterface(e.Name)
			}); err != nil {
				return record("wireguard "+e.Name+" create", err)
			}
		}
		// Whether newly created or spec-changed, push the full desired
		// state. wgctrl handles key rotation, endpoint changes, peer
		// add/remove, keepalive updates in a single genetlink message
		// with ReplacePeers: true -- peers that are still in the spec
		// preserve their handshake state because the kernel matches
		// them by public key.
		if err := applyBackendStep(journal, func() error {
			return b.ConfigureWireguardDevice(e.Spec)
		}, func() error {
			if !hadPrev {
				return nil
			}
			return b.ConfigureWireguardDevice(prev)
		}); err != nil {
			return record("wireguard "+e.Name+" configure", err)
		}
	}
	previousXFRMSpecs := indexXFRMSpecs(previous)
	for i := range cfg.XFRM {
		e := &cfg.XFRM[i]
		if e.Disable {
			continue
		}
		prev, hadPrev := previousXFRMSpecs[e.Name]
		if hadPrev && xfrmSpecEqual(prev, e.Spec) {
			continue
		}
		if hadPrev {
			deleted := false
			if err := applyBackendStep(journal, func() error {
				if err := b.DeleteInterface(e.Name); err != nil {
					if !interfaceExists(b, e.Name) {
						return nil
					}
					return err
				}
				deleted = true
				return nil
			}, func() error {
				if !deleted {
					return nil
				}
				return b.CreateXFRM(prev)
			}); err != nil {
				return record("xfrm "+e.Name+" delete before recreate", err)
			}
		}
		created := false
		if err := applyBackendStep(journal, func() error {
			if err := b.CreateXFRM(e.Spec); err != nil {
				if _, getErr := b.GetInterface(e.Name); getErr != nil {
					return err
				}
				return nil
			}
			created = true
			return nil
		}, func() error {
			if !created {
				return nil
			}
			return b.DeleteInterface(e.Name)
		}); err != nil {
			return record("xfrm "+e.Name+" create", err)
		}
	}
	for _, e := range cfg.Bridge {
		if e.Disable {
			continue
		}
		created := false
		if err := applyBackendStep(journal, func() error {
			if err := b.CreateBridge(e.Name); err != nil {
				if _, getErr := b.GetInterface(e.Name); getErr != nil {
					return err
				}
				return nil
			}
			created = true
			return nil
		}, func() error {
			if !created {
				return nil
			}
			return b.DeleteInterface(e.Name)
		}); err != nil {
			return record("bridge "+e.Name+" create", err)
		}
		oldSTP := !e.STP
		if err := applyBackendStep(journal, func() error {
			return b.BridgeSetSTP(e.Name, e.STP)
		}, func() error {
			return b.BridgeSetSTP(e.Name, oldSTP)
		}); err != nil {
			return record("bridge "+e.Name+" stp", err)
		}
		// The members are enslaved in Phase 2a, not here: a member names
		// another interface, and the map that turns a logical name into a
		// kernel device cannot exist until Phase 1 has created the devices a
		// selector can bind to.
	}

	// Phase 2: Set properties and create VLANs.
	//
	// One interface listing answers every hardware-selector question the rest of
	// this apply asks, so each logical name resolves once and every phase below
	// keys its work by the same kernel device. It is taken HERE rather than at
	// the top because Phase 1 has just created devices, and the reconcile takes
	// its own listing at the same point in its own pass: resolving both against a
	// post-create view is what keeps the two agreeing.
	//
	// A backend that cannot list leaves every ethernet entry unbound: those
	// entries are skipped here, and a later reconcile picks them up. That drops
	// the MTU, MAC, offloads and admin state of every ethernet entry from this
	// apply, so the reason is stated once, HERE, at the level of what it costs.
	// It is the only line that names the real cause, which is why the per-entry
	// loop below stays silent after it.
	//
	// Which level that is depends on the answer, exactly as the reconcile's own
	// listing does (reconcileOnReadyWithJournal). The vpp handshake still being
	// in flight is the designed path and says nothing an operator must act on.
	// Any other failure is one: the ethernet half of the commit did not happen.
	infos, listErr := b.ListInterfaces()
	if listErr != nil {
		if errors.Is(listErr, ErrBackendNotReady) {
			log.Debug("iface config: backend not ready, every ethernet binding deferred", "err", listErr)
		} else {
			log.Warn("iface config: interface listing unavailable, every ethernet binding deferred", "err", listErr)
		}
		infos = nil
	}
	if selectorErr := cfg.validateSelectors(infos); selectorErr != nil {
		return record("hardware selector", selectorErr)
	}
	devices := cfg.bindDevices(infos)
	previousDevices := previous.bindDevices(infos)
	cfg.rememberPreviousManaged(previous, previousDevices)

	// Phase 2a: enslave each bridge's members.
	//
	// A member names another interface, so this is the one create-time step
	// that has to translate a logical name into a kernel device, and the map
	// that does it cannot exist until Phase 1 has created the devices a
	// selector can bind to. Enslaving by the logical name is what put the
	// WRONG physical port in a bridge: a member naming an entry bound by
	// mac/match reached whatever device happened to carry that name.
	//
	// Running here rather than beside the bridge create is safe in both
	// directions. It still precedes every property the loop below sets, which
	// is where it ran before. And it cannot invalidate the bindings just
	// taken, because enslaving a port changes neither its name nor the address
	// it was matched on: the bridge takes the port's address, never the
	// reverse.
	for _, e := range cfg.Bridge {
		if e.Disable {
			continue
		}
		for _, member := range e.Members {
			port, bound := deviceFor(devices, member)
			if !bound {
				log.Warn("iface config: no present device answers this bridge member's hardware selector, enslaving deferred",
					"bridge", e.Name, "member", member)
				continue
			}
			if err := applyBackendStep(journal, func() error {
				return b.BridgeAddPort(e.Name, port)
			}, func() error {
				return b.BridgeDelPort(port)
			}); err != nil {
				var bPort textbuf.Buffer
				return record(bPort.Str("bridge ").Str(e.Name).Str(" add port ").Str(port).String(), err)
			}
		}
	}

	allEntries := allIfaceEntries(cfg)

	// Retire the mirrors the new config no longer asks for, before the loop
	// below installs the ones it does ask for. tc filters are additive, so a
	// changed destination has to be removed rather than overwritten.
	if mirrorErrs := removeStaleMirrors(cfg, previous, devices, previousDevices, b, journal); len(mirrorErrs) > 0 {
		errs = append(errs, mirrorErrs...)
		return rollbackPartial()
	}

	// Physical (Ethernet) interfaces whose kernel device is absent are skipped,
	// not treated as a fatal error. An appliance must not brick because a
	// configured NIC is missing -- an unplugged cable, a hardware change, or an
	// image built on a host whose interface names differ from the deployment
	// target. bindDevices has already made that judgement for every ethernet
	// entry, over the device its selector names rather than over its logical
	// name, so an entry bound by mac/match to a NIC that has not appeared yet is
	// skipped here exactly as an absent one is. Created interface types
	// (dummy/veth/bridge/tunnel/wireguard/xfrm) are made in Phase 1 and exist by
	// now, so they are NOT skipped; a genuine error on any interface still
	// aborts + rolls back.
	//
	// The skip covers the address reconcile too (Phase 3+4 reads the same
	// bindings), because an entry whose device is absent has nowhere to put its
	// addresses: adding them by the logical name is what failed the whole commit
	// before, and what put them on a same-named stranger when one existed.
	//
	// A listing failure is reported once above instead. Every line below would
	// name a cause that is not the one that happened: a device the backend
	// could not list can be present, and its selector can have an answer.
	if listErr == nil {
		for i := range cfg.Ethernet {
			e := &cfg.Ethernet[i]
			if e.Disable {
				continue
			}
			if _, bound := deviceFor(devices, e.Name); bound {
				continue
			}
			if e.MatchMAC != "" || e.OSName != "" {
				log.Warn("iface config: no present device answers this interface's hardware selector, binding deferred",
					"iface", e.Name, "mac-match", e.MatchMAC, "os-name", e.OSName)
				continue
			}
			log.Warn("iface config: configured interface not present, skipping", "iface", e.Name)
		}
	}
	// Drop unbound ethernet entries from every per-interface phase (property set
	// in Phase 2, admin-up in Phase 2c) so a missing NIC never aborts the apply.
	// Created types (dummy/veth/bridge/tunnel/wireguard/xfrm) are made in Phase 1
	// and are never in devices, so deviceFor answers with their own name.
	kept := allEntries[:0]
	for _, e := range allEntries {
		if _, bound := deviceFor(devices, e.Name); bound {
			kept = append(kept, e)
		}
	}
	allEntries = kept

	for _, e := range allEntries {
		if e.Disable {
			continue
		}
		// The kernel device this entry configures. Every backend call below
		// takes it, never e.Name: the logical name steers the config tree, the
		// device steers the hardware.
		device, _ := deviceFor(devices, e.Name)
		if e.MTU > 0 {
			oldMTU := 0
			if info, err := b.GetInterface(device); err == nil {
				oldMTU = info.MTU
			}
			if err := applyBackendStep(journal, func() error {
				return b.SetMTU(device, e.MTU)
			}, func() error {
				if oldMTU <= 0 {
					return nil
				}
				return b.SetMTU(device, oldMTU)
			}); err != nil {
				var bMtu textbuf.Buffer
				return record(bMtu.Reset().Str(device).Str(" set mtu ").Int(int64(e.MTU)).String(), err)
			}
		}
		if e.MACAddress != "" {
			oldMAC, _ := b.GetMACAddress(device)
			if err := applyBackendStep(journal, func() error {
				return b.SetMACAddress(device, e.MACAddress)
			}, func() error {
				if oldMAC == "" {
					return nil
				}
				return b.SetMACAddress(device, oldMAC)
			}); err != nil {
				return record(device+" set mac", err)
			}
		}
		applyOffloads(device, e.Offload)
		for i := range e.Units {
			u := &e.Units[i]
			if u.Disable {
				continue
			}
			osName := device
			if u.VLANID > 0 {
				vlanName := unitOSName(device, u)
				created := false
				if err := applyBackendStep(journal, func() error {
					if err := b.CreateVLAN(VLANSpec{
						Parent:        device,
						VLANID:        u.VLANID,
						IngressQoSMap: u.IngressQoSMap,
						EgressQoSMap:  u.EgressQoSMap,
					}); err != nil {
						if _, getErr := b.GetInterface(vlanName); getErr != nil {
							return err
						}
						return nil
					}
					created = true
					return nil
				}, func() error {
					if !created {
						return nil
					}
					return b.DeleteInterface(vlanName)
				}); err != nil {
					return record("vlan "+vlanName+" create", err)
				}
				osName = vlanName
			}
			applySysctl(osName, *u)
			applySysctlProfiles(osName, u.SysctlProfiles)
			if mirrorErrs := applyMirror(b, osName, *u, devices, journal); len(mirrorErrs) > 0 {
				errs = append(errs, mirrorErrs...)
				return rollbackPartial()
			}
		}
	}

	applyRFSGlobal(cfg)

	// Phase 2b: Apply sysctl for loopback units.
	if cfg.Loopback != nil {
		for i := range cfg.Loopback.Units {
			u := &cfg.Loopback.Units[i]
			if u.Disable {
				continue
			}
			applySysctl("lo", *u)
			applySysctlProfiles("lo", u.SysctlProfiles)
		}
	}

	// Phase 2c: Bring all non-disabled interfaces administratively UP.
	// Without this, DHCP and other protocols cannot send packets on the
	// interface even if the physical link is connected.
	for _, e := range allEntries {
		if e.Disable {
			continue
		}
		device, _ := deviceFor(devices, e.Name)
		wasDown := false
		if info, err := b.GetInterface(device); err == nil {
			wasDown = info.State != "" && info.State != "up" && info.State != "UP"
		}
		if err := applyBackendStep(journal, func() error {
			return b.SetAdminUp(device)
		}, func() error {
			if !wasDown {
				return nil
			}
			return b.SetAdminDown(device)
		}); err != nil {
			return record(device+" admin up", err)
		}
	}

	// Phase 3+4: Reconcile addresses (add missing, remove extra) and prune
	// non-config interfaces. If the backend is not yet ready (vpp handshake
	// still in flight), log + defer: reconcile will re-run once
	// vppevents.EventConnected fires. The additive-only fallback still
	// applies desired addresses so the daemon is usable before vpp comes up.
	reconcileErrs, deferred := reconcileOnReadyWithJournal(cfg, b, journal, previous)
	if deferred {
		log.Debug("iface reconcile deferred, backend not ready")
		addDesiredAddresses(cfg, b, devices)
		return errs
	}
	if len(reconcileErrs) > 0 {
		errs = append(errs, reconcileErrs...)
		return rollbackPartial()
	}

	return errs
}

// reconcileOnReady runs Phase 3 (address reconcile), the mirror reconcile, and
// Phase 4 (prune non-config interfaces) for cfg against backend b. It is called from
// applyConfig on every config apply; it is also called from the
// vppevents.EventConnected / EventReconnected handler in register.go so that
// reconciliation deferred at startup (because the vpp backend was not yet
// ready) fires once GoVPP is connected.
//
// Returns (nil, true) when the backend reports iface.ErrBackendNotReady --
// callers can retry later. Returns (errs, false) with any operational
// failures encountered during a normal reconcile.
func reconcileOnReady(cfg *ifaceConfig, b Backend) (errs []error, deferred bool) {
	return reconcileOnReadyWithJournal(cfg, b, nil, nil)
}

// reconcileMu serializes reconcileOnReadyWithJournal across its three
// independent trigger paths -- applyConfig (config commits), vppReconcileCh's
// worker (reconcileOnVPPReady, vpp connect/reconnect events), and
// registryReconcileCh's worker (reconcileOnRegistryChange, address_owner.go
// registry changes) -- which otherwise run on different goroutines with no
// other synchronization against the same Backend and the same
// desiredState() (which unconditionally merges the FULL live registry on
// every call, so ANY commit's reconcile independently re-decides every
// registry-owned address too, not just its own YANG diff).
//
// This spec's review weighed two options and picked this one deliberately:
//   - Serialize all three (this option): a config commit can BLOCK for the
//     duration of a concurrent background pass's real kernel I/O. Bounded
//     downside: only a problem if that pass exceeds the plugin's
//     ApplyBudget (10s, register.go), realistically only under a VPP
//     reconnect RPC storm -- rare, and the system is already in a degraded
//     state from the underlying VPP crash when it happens.
//   - Serialize only the two background paths, leave applyConfig
//     unsynchronized (tried first, reverted): an adversarial re-review
//     showed this lets a registry-triggered reconcile (routine plugin
//     enable/disable, NOT rare) race an unrelated commit's own reconcile
//     over the SAME address, both issuing AddAddress/RemoveAddress
//     concurrently; the kernel's second, colliding call returns EEXIST or
//     ENOENT, which reconcileOnReadyWithJournal (below) treats as fatal --
//     rolling back the ENTIRE unrelated commit (Phase 1/2 changes and all)
//     for a reason having nothing to do with what the operator actually
//     changed. Unbounded, deterministic-on-collision downside, and far
//     higher frequency than a VPP reconnect.
//
// Between "a commit occasionally waits a bit longer" and "a commit
// occasionally fails for a reason unrelated to what it changed," the first
// is the better failure mode. The pre-existing race this closes (config
// commits vs. vpp-event reconciles) predates this spec; this spec chooses
// to fix it now because the new registry-change path would otherwise make
// it meaningfully more frequent.
var reconcileMu sync.Mutex

func reconcileOnReadyWithJournal(cfg *ifaceConfig, b Backend, journal *sdk.Journal, previous *ifaceConfig) (errs []error, deferred bool) {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()

	log := loggerPtr.Load()

	currentInfos, err := b.ListInterfaces()
	if err != nil {
		if errors.Is(err, ErrBackendNotReady) {
			return nil, true
		}
		errs = append(errs, fmt.Errorf("list interfaces for reconciliation: %w", err))
		return errs, false
	}

	// Resolve the hardware selectors against THIS listing, so a reconcile
	// triggered long after the commit (vpp connect, an owned-address
	// registration) sees the devices present now rather than the ones present
	// when the config was applied -- a mac/match binding that was deferred then
	// can have appeared since. The previous config resolves against the same
	// listing, so the prune scope below names the devices it actually made.
	devices := cfg.bindDevices(currentInfos)
	previousDevices := previous.bindDevices(currentInfos)
	desiredAddrs, managedNames, staleNames := cfg.desiredState(devices)

	record := func(msg string, err error) {
		log.Warn(msg, "err", err)
		errs = append(errs, fmt.Errorf("%s: %w", msg, err))
	}

	currentAddrs := currentAddrSet(currentInfos)

	// Owned-device pass (macvlan): create/re-assert/delete plugin-owned
	// devices from the owned-device registry (device_owner.go) BEFORE the
	// address loops, so a VIP registered on an owned device via the address
	// registry lands on an EXISTING device this same pass (an AddAddress on a
	// missing device fails "not found" and, in the applyConfig path, aborts +
	// rolls back the WHOLE commit). Orphan detection reads the kernel-side
	// IFLA_IFALIAS ownership marker, so it also cleans up crash leftovers with
	// no in-memory history. Fails fast like the address loops.
	if !reconcileOwnedDevices(b, journal, currentInfos, record) {
		return errs, false
	}

	// Add missing addresses on configured interfaces.
	for osName, desired := range desiredAddrs {
		current := currentAddrs[osName]
		for addr := range desired {
			if current != nil && current[addr] {
				continue
			}
			if err := applyBackendStep(journal, func() error {
				return b.AddAddress(osName, addr)
			}, func() error {
				return b.RemoveAddress(osName, addr)
			}); err != nil {
				record(osName+" add address "+addr, err)
				return errs, false
			}
		}
	}

	// Remove extra addresses on configured interfaces.
	for osName, desired := range desiredAddrs {
		current := currentAddrs[osName]
		for addr := range current {
			if desired[addr] {
				continue
			}
			if err := applyBackendStep(journal, func() error {
				return b.RemoveAddress(osName, addr)
			}, func() error {
				return b.AddAddress(osName, addr)
			}); err != nil {
				record(osName+" remove stale address "+addr, err)
				return errs, false
			} else {
				log.Info("iface config: removed stale address", "iface", osName, "addr", addr)
			}
		}
	}

	// Mirror pass: make the dataplane's mirrors equal the ones cfg asks for,
	// read from live state rather than from a delta. It runs HERE, after the
	// devices this config creates exist and before Phase 4 deletes the ones it
	// dropped. A mirror on an interface about to be deleted is therefore still
	// readable and removable. A mirror on a VLAN sub-interface created in this
	// same apply has a device to be installed on.
	//
	// This is also the pass that reaches a reconcile no commit triggered. A vpp
	// connect or a registry change re-decides the mirrors exactly as it
	// re-decides every address.
	if mirrorErrs := reconcileMirrors(cfg, previous, b, devices, previousDevices, journal); len(mirrorErrs) > 0 {
		errs = append(errs, mirrorErrs...)
		return errs, false
	}

	// Phase 4: Delete interfaces Ze owned in the previous config but which are
	// no longer in the current config. Do not adopt/delete arbitrary existing
	// dummy/veth/bridge/tunnel devices just because their link type is manageable.
	currentIfaces := currentIfaceSet(currentInfos)
	removedManaged := removedManagedNames(cfg.previousManaged, managedNames)
	for name, linkType := range currentIfaces {
		if !zeManageable(linkType) {
			continue
		}
		if !removedManaged[name] {
			continue
		}
		if err := applyBackendStep(journal, func() error {
			return b.DeleteInterface(name)
		}, func() error {
			return recreateManagedInterface(previous, previousDevices, name, b)
		}); err != nil {
			record("delete "+name+" ("+linkType+")", err)
			return errs, false
		} else {
			log.Info("iface config: deleted interface not in config", "name", name, "type", linkType)
		}
	}

	// Reached only when every add/remove/delete step above succeeded (each
	// failure path returns early) -- a fully clean pass over desiredAddrs,
	// which included every name in staleNames (this call's own
	// ownedAddresses() snapshot, not a fresh read) as an empty key. Any
	// stale address the registry left behind on exactly these interfaces
	// was therefore just pruned (or there was none). Passing staleNames
	// (not a blanket clear) means a concurrent UnregisterOwnedAddresses
	// call that added a DIFFERENT interface to staleIfaces after this
	// pass's snapshot was taken is never silently discarded here.
	clearStaleIfaces(staleNames)

	return errs, false
}

func applyBackendStep(journal *sdk.Journal, apply, undo func() error) error {
	if journal == nil {
		return apply()
	}
	if undo == nil {
		undo = func() error { return nil }
	}
	return journal.Record(apply, undo)
}

// reconcileOwnedDevices creates, re-asserts, and deletes plugin-owned macvlan
// devices to match the owned-device registry (device_owner.go). It runs inside
// reconcileOnReadyWithJournal BEFORE the address loops so a VIP registered on
// an owned device is applied to an existing device in the same pass.
//
// Desired state comes from the registry; actual state AND ownership come from
// the kernel via the pass's existing ListInterfaces snapshot (InterfaceInfo
// carries the "ze:owned:" IFLA_IFALIAS marker), so reconcile is stateless
// across restarts and crashes -- no staleDevices bookkeeping.
//
// For each registered device: create if absent; if a foreign (non-owned)
// device occupies the name, fail closed (never delete an operator device); if
// an owned macvlan exists but drifted from spec, delete + recreate in this
// same pass (VIPs are re-added by the address loop that follows). Then the
// orphan scan deletes any aliased macvlan with no registration (owner release,
// crash leftovers, drift remnants) -- requiring BOTH kind macvlan AND the
// ownership alias so operator devices are never touched.
//
// Returns true when every device step succeeded. On the first failure it
// records via record (which logs + appends to the pass's errs) and returns
// false, matching the address loops' fail-fast + whole-commit-rollback
// contract.
func reconcileOwnedDevices(b Backend, journal *sdk.Journal, currentInfos []InterfaceInfo, record func(string, error)) bool {
	specs, owners := ownedMacvlans()
	log := loggerPtr.Load()

	currentByName := make(map[string]InterfaceInfo, len(currentInfos))
	for i := range currentInfos {
		currentByName[currentInfos[i].Name] = currentInfos[i]
	}

	// Create or re-assert every registered device.
	for name, spec := range specs {
		desired := spec
		desired.Alias = ownedDeviceAliasPrefix + owners[name]

		cur, exists := currentByName[name]
		if !exists {
			if err := applyBackendStep(journal, func() error {
				return b.CreateMacvlanDevice(desired)
			}, func() error {
				return b.DeleteInterface(name)
			}); err != nil {
				record("create owned macvlan", fmt.Errorf("%s: %w", name, err))
				return false
			}
			continue
		}

		// A device already holds the desired name. A non-macvlan kind is a
		// foreign device (operator's) and is never deleted -- fail closed. A
		// macvlan holding a REGISTERED name is ours by construction even if
		// its ownership alias is missing (the kernel ignores IFLA_IFALIAS at
		// LinkAdd, so a crash between create and LinkSetAlias leaves exactly
		// this state -- A-2 fallback): the drift path below adopts and
		// re-marks it via delete + recreate to spec.
		if cur.Type != zeTypeMacvlan {
			record("owned macvlan name conflict", fmt.Errorf("%s: name occupied by a non-owned %s device", name, cur.Type))
			return false
		}
		if ownedMacvlanMatchesSpec(cur, desired, currentByName) {
			continue
		}
		// Drift (wrong MAC/parent/MTU, or a missing/foreign ownership alias --
		// the adopt + re-mark case): delete + recreate to spec in the same pass.
		log.Warn("iface owned-device: re-asserting drifted macvlan", "name", name, "owner", owners[name])
		if err := applyBackendStep(journal, func() error {
			return b.DeleteInterface(name)
		}, nil); err != nil {
			record("delete drifted owned macvlan", fmt.Errorf("%s: %w", name, err))
			return false
		}
		if err := applyBackendStep(journal, func() error {
			return b.CreateMacvlanDevice(desired)
		}, func() error {
			return b.DeleteInterface(name)
		}); err != nil {
			record("recreate drifted owned macvlan", fmt.Errorf("%s: %w", name, err))
			return false
		}
	}

	// Orphan scan: delete aliased macvlans with no registration.
	for i := range currentInfos {
		info := &currentInfos[i]
		if info.Type != zeTypeMacvlan || !strings.HasPrefix(info.Alias, ownedDeviceAliasPrefix) {
			continue
		}
		if _, registered := specs[info.Name]; registered {
			continue
		}
		name := info.Name
		if err := applyBackendStep(journal, func() error {
			return b.DeleteInterface(name)
		}, nil); err != nil {
			record("delete orphan owned macvlan", fmt.Errorf("%s: %w", name, err))
			return false
		}
		log.Info("iface owned-device: deleted orphan macvlan", "name", name, "alias", info.Alias)
	}

	updateOwnedDeviceGauge()
	return true
}

// ownedMacvlanMatchesSpec reports whether an existing owned macvlan (cur, known
// to be kind macvlan with the ownership alias) already matches the desired
// spec, so the drift path is skipped. Compares the ownership alias, the MAC,
// and -- when the parent is currently resolvable -- the parent index and MTU
// (owned macvlans inherit the parent MTU, so a parent MTU change is drift,
// eventually-consistently). Mode drift IS detected when the backend reports the
// live mode (MacvlanMode); an older binary that predates the mode field reports
// empty, which is treated as "unknown" and does not force a re-create.
func ownedMacvlanMatchesSpec(cur InterfaceInfo, desired MacvlanSpec, currentByName map[string]InterfaceInfo) bool {
	if cur.Alias != desired.Alias {
		return false
	}
	if !macEqual(cur.MAC, desired.MAC) {
		return false
	}
	// A device created in the wrong delivery mode (e.g. by an older binary that
	// only made bridge macvlans) is drift: a consumer that picked private mode
	// (so its own MAC answers ARP) must not silently keep a stale bridge device.
	// Compare only when the backend reported the mode (empty means "unknown", so
	// do not force a needless re-create).
	if cur.MacvlanMode != "" && cur.MacvlanMode != desired.Mode.String() {
		return false
	}
	if parent, ok := currentByName[desired.Parent]; ok {
		if cur.ParentIndex != parent.Index {
			return false
		}
		if cur.MTU != parent.MTU {
			return false
		}
	}
	return true
}

// existingLinkType names the kind of device a read-back found, for an error
// that must say what occupies a name. A backend that reports no error and no
// info is a contract break rather than an empty answer, so it is named as
// unreadable instead of being spelled as an empty string in the message.
func existingLinkType(info *InterfaceInfo) string {
	if info == nil || info.Type == "" {
		return "unreadable"
	}
	return info.Type
}

func interfaceExists(b Backend, name string) bool {
	_, err := b.GetInterface(name)
	return err == nil
}

// recreateManagedInterface rebuilds the device the prune step deleted, as that
// step's undo. cfg is the PREVIOUS config, which is the one that made the
// device, and devices is that config's binding map, so a VLAN is recreated on
// the kernel parent it was made on rather than on a logical name.
func recreateManagedInterface(cfg *ifaceConfig, devices map[string]string, name string, b Backend) error {
	if cfg == nil {
		return nil
	}
	for _, e := range cfg.Dummy {
		if !e.Disable && e.Name == name {
			// The prune step deleted this device, so the create is expected
			// to make it. A backend that answers ErrInterfaceExists still
			// holds it, which is the state this undo wanted: keep going and
			// re-establish the LCP shadow rather than failing the undo.
			if err := b.CreateDummy(e.Name); err != nil && !errors.Is(err, ErrInterfaceExists) {
				return err
			}
			// Re-establish the LCP shadow on the vpp backend, matching
			// applyConfig's Phase 1 so a loopback recreated on the deferred
			// vpp-ready / post-crash path is not left without its Linux TAP.
			if cfg.Backend == vppBackendName {
				return b.SetupLCPPair(e.Name, e.Name)
			}
			return nil
		}
	}
	for _, e := range cfg.Veth {
		if e.Disable || e.Name != name {
			continue
		}
		peer := e.Peer
		if peer == "" {
			peer = e.Name + "-peer"
		}
		return b.CreateVeth(e.Name, peer)
	}
	for _, e := range cfg.Bridge {
		if !e.Disable && e.Name == name {
			return b.CreateBridge(e.Name)
		}
	}
	for i := range cfg.Tunnel {
		e := &cfg.Tunnel[i]
		if !e.Disable && e.Name == name {
			return b.CreateTunnel(e.Spec)
		}
	}
	for i := range cfg.Wireguard {
		e := &cfg.Wireguard[i]
		if !e.Disable && e.Name == name {
			if err := b.CreateWireguardDevice(e.Name); err != nil {
				return err
			}
			return b.ConfigureWireguardDevice(e.Spec)
		}
	}
	for _, e := range cfg.Ethernet {
		if e.Disable {
			continue
		}
		device, bound := deviceFor(devices, e.Name)
		if !bound {
			continue
		}
		if err := recreateManagedVLAN(device, e.Units, name, b); err != nil {
			return err
		}
	}
	for _, e := range cfg.Dummy {
		if e.Disable {
			continue
		}
		if err := recreateManagedVLAN(e.Name, e.Units, name, b); err != nil {
			return err
		}
	}
	for _, e := range cfg.Veth {
		if e.Disable {
			continue
		}
		if err := recreateManagedVLAN(e.Name, e.Units, name, b); err != nil {
			return err
		}
	}
	for _, e := range cfg.Bridge {
		if e.Disable {
			continue
		}
		if err := recreateManagedVLAN(e.Name, e.Units, name, b); err != nil {
			return err
		}
	}
	for i := range cfg.Tunnel {
		e := &cfg.Tunnel[i]
		if e.Disable {
			continue
		}
		if err := recreateManagedVLAN(e.Name, e.Units, name, b); err != nil {
			return err
		}
	}
	for i := range cfg.Wireguard {
		e := &cfg.Wireguard[i]
		if e.Disable {
			continue
		}
		if err := recreateManagedVLAN(e.Name, e.Units, name, b); err != nil {
			return err
		}
	}
	return nil
}

// recreateManagedVLAN recreates the VLAN unit of parent whose device is called
// name. parent is the KERNEL device, resolved by the caller, because that is
// what the netdev name was composed from when the VLAN was made.
func recreateManagedVLAN(parent string, units []unitEntry, name string, b Backend) error {
	for i := range units {
		u := &units[i]
		if u.Disable || u.VLANID <= 0 {
			continue
		}
		if unitOSName(parent, u) != name {
			continue
		}
		return b.CreateVLAN(VLANSpec{
			Parent:        parent,
			VLANID:        u.VLANID,
			IngressQoSMap: u.IngressQoSMap,
			EgressQoSMap:  u.EgressQoSMap,
		})
	}
	return nil
}

// addDesiredAddresses adds every configured address without consulting the
// backend for current state. Used as the additive-only fallback when
// reconcileOnReady defers; the full reconcile fires later via
// vppevents.EventConnected.
func addDesiredAddresses(cfg *ifaceConfig, b Backend, devices map[string]string) {
	log := loggerPtr.Load()
	desiredAddrs, _, _ := cfg.desiredState(devices)
	for osName, addrs := range desiredAddrs {
		for addr := range addrs {
			if err := b.AddAddress(osName, addr); err != nil {
				log.Debug("iface config: add address", "iface", osName, "addr", addr, "err", err)
			}
		}
	}
}

// reconcileOnVPPReady is invoked from the register.go EventBus handler when
// vppevents.EventConnected or EventReconnected fires. It reloads the VPP
// backend via LoadBackend to clear stale state (dead GoVPP channel, stale
// name map, stale bridge domains) from the pre-crash instance, then runs
// reconcileOnReady against the fresh backend. It also retries
// b.StartMonitor so a monitor deferred at startup (backend not ready at
// OnConfigure time) installs as soon as the backend becomes live. No-op
// when no config has been applied yet, when the backend is still
// unregistered, or when the active backend is not vpp.
//
// Exposed at package level so the register-time handler is easy to test
// without standing up the SDK event loop.
func reconcileOnVPPReady(activeCfg *atomic.Pointer[ifaceConfig]) {
	log := loggerPtr.Load()
	cfg := activeCfg.Load()
	if cfg == nil {
		return
	}
	// Guard against firing against a non-vpp backend. A reload that flipped
	// the backend from vpp to netlink would otherwise call netlink's
	// StartMonitor on every EventConnected / EventReconnected, leaking a
	// fresh monitor goroutine per event (netlink's StartMonitor is not
	// idempotent -- see internal/plugins/iface/netlink/monitor_linux.go).
	if cfg.Backend != vppBackendName {
		return
	}

	// Reload the backend to clear stale state (dead GoVPP channel, stale
	// name map, stale bridge domains) from the pre-crash VPP instance.
	// LoadBackend creates a fresh vppBackendImpl; the old one is closed.
	if err := LoadBackend(vppBackendName); err != nil {
		log.Warn("iface: reload vpp backend on reconnect", "err", err)
	}

	b := GetBackend()
	if b == nil {
		return
	}

	// Retry the monitor install if it was deferred at OnConfigure time.
	// StartMonitor is idempotent (a second call after a first success is
	// a no-op), so a call on every vpp event is safe.
	if eb := GetEventBus(); eb != nil {
		if err := b.StartMonitor(eb); err != nil {
			if errors.Is(err, ErrBackendNotReady) {
				log.Debug("iface monitor still deferred after vpp event")
				return
			}
			log.Warn("iface monitor start on vpp ready", "err", err)
		}
	}

	errs, deferred := reconcileOnReady(cfg, b)
	if deferred {
		log.Debug("iface reconcile still deferred after vpp event")
		return
	}
	for _, e := range errs {
		log.Warn("iface reconcile on vpp ready", "err", e)
	}
}

// reconcileOnRegistryChange re-runs address reconciliation after
// address_owner.go's registry changes (RegisterOwnedAddresses /
// UnregisterOwnedAddresses), so a plugin enabling or disabling its service
// sees its addresses applied (or removed) within the same operation instead
// of waiting for an unrelated config commit (design finding B1).
// Unlike reconcileOnVPPReady, this is not gated to any particular backend --
// a registry mutation must reach the kernel regardless of which backend is
// active. No-op when no config has been applied yet or no backend is loaded.
//
// Exposed at package level so the trigger wiring in runEngine is easy to
// test without standing up the SDK event loop.
func reconcileOnRegistryChange(activeCfg *atomic.Pointer[ifaceConfig]) {
	log := loggerPtr.Load()
	cfg := activeCfg.Load()
	if cfg == nil {
		return
	}
	b := GetBackend()
	if b == nil {
		return
	}

	errs, deferred := reconcileOnReady(cfg, b)
	if deferred {
		log.Debug("iface reconcile on registry change deferred, backend not ready")
		return
	}
	recordRegistryReconcileOutcome(errs)
	for _, e := range errs {
		log.Warn("iface reconcile on registry change", "err", e)
	}
}
