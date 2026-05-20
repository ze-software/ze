// Design: docs/features/interfaces.md -- Interface reconciliation and application
// Related: config.go -- parsing, config_sysctl.go -- sysctl/mirror

package iface

import (
	"errors"
	"fmt"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// desiredState builds a map of OS interface name -> desired addresses from config.
// Also returns the set of Ze-managed interface names (dummy, veth, bridge, VLAN)
// that should exist. Physical interfaces (ethernet) are never in the managed set.
func (cfg *ifaceConfig) desiredState() (addrs map[string]map[string]bool, managed map[string]bool) {
	addrs = make(map[string]map[string]bool)
	managed = make(map[string]bool)

	addIfaceAddrs := func(name string, units []unitEntry) {
		for i := range units {
			u := &units[i]
			if u.Disable {
				continue
			}
			osName := name
			if u.VLANID > 0 {
				var bName textbuf.Buffer
				osName = bName.Reset().Str(name).Byte('.').Int(int64(u.VLANID)).String()
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

	return addrs, managed
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
func (cfg *ifaceConfig) rememberPreviousManaged(previous *ifaceConfig) {
	cfg.previousManaged = nil
	if previous == nil {
		return
	}
	_, managed := previous.desiredState()
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
// Returns collected errors. The first mutating failure aborts the apply and
// rolls back successful steps that have an exact inverse.
func applyConfig(cfg, previous *ifaceConfig, b Backend) []error {
	log := loggerPtr.Load()
	var errs []error
	journal := sdk.NewJournal()
	cfg.rememberPreviousManaged(previous)

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
				return err
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
		for _, member := range e.Members {
			if err := applyBackendStep(journal, func() error {
				return b.BridgeAddPort(e.Name, member)
			}, func() error {
				return b.BridgeDelPort(member)
			}); err != nil {
				return record("bridge "+e.Name+" add port "+member, err)
			}
		}
	}

	// Phase 2: Set properties and create VLANs.
	allEntries := make([]ifaceEntry, 0, len(cfg.Ethernet)+len(cfg.Dummy)+len(cfg.Veth)+len(cfg.Bridge)+len(cfg.Tunnel)+len(cfg.Wireguard)+len(cfg.XFRM))
	allEntries = append(allEntries, cfg.Ethernet...)
	allEntries = append(allEntries, cfg.Dummy...)
	for _, e := range cfg.Veth {
		allEntries = append(allEntries, e.ifaceEntry)
	}
	for _, e := range cfg.Bridge {
		allEntries = append(allEntries, e.ifaceEntry)
	}
	for i := range cfg.Tunnel {
		allEntries = append(allEntries, cfg.Tunnel[i].ifaceEntry)
	}
	for i := range cfg.Wireguard {
		allEntries = append(allEntries, cfg.Wireguard[i].ifaceEntry)
	}
	for i := range cfg.XFRM {
		allEntries = append(allEntries, cfg.XFRM[i].ifaceEntry)
	}

	for _, e := range allEntries {
		if e.Disable {
			continue
		}
		if e.MTU > 0 {
			oldMTU := 0
			if info, err := b.GetInterface(e.Name); err == nil {
				oldMTU = info.MTU
			}
			if err := applyBackendStep(journal, func() error {
				return b.SetMTU(e.Name, e.MTU)
			}, func() error {
				if oldMTU <= 0 {
					return nil
				}
				return b.SetMTU(e.Name, oldMTU)
			}); err != nil {
				var bMtu textbuf.Buffer
				return record(bMtu.Reset().Str(e.Name).Str(" set mtu ").Int(int64(e.MTU)).String(), err)
			}
		}
		if e.MACAddress != "" {
			oldMAC, _ := b.GetMACAddress(e.Name)
			if err := applyBackendStep(journal, func() error {
				return b.SetMACAddress(e.Name, e.MACAddress)
			}, func() error {
				if oldMAC == "" {
					return nil
				}
				return b.SetMACAddress(e.Name, oldMAC)
			}); err != nil {
				return record(e.Name+" set mac", err)
			}
		}
		applyOffloads(e.Name, e.Offload)
		for i := range e.Units {
			u := &e.Units[i]
			if u.Disable {
				continue
			}
			osName := e.Name
			if u.VLANID > 0 {
				var bVlan textbuf.Buffer
				vlanName := bVlan.Reset().Str(e.Name).Byte('.').Int(int64(u.VLANID)).String()
				created := false
				if err := applyBackendStep(journal, func() error {
					if err := b.CreateVLAN(e.Name, u.VLANID); err != nil {
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
			if mirrorErrs := applyMirror(b, osName, *u, journal); len(mirrorErrs) > 0 {
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
		wasDown := false
		if info, err := b.GetInterface(e.Name); err == nil {
			wasDown = info.State != "" && info.State != "up" && info.State != "UP"
		}
		if err := applyBackendStep(journal, func() error {
			return b.SetAdminUp(e.Name)
		}, func() error {
			if !wasDown {
				return nil
			}
			return b.SetAdminDown(e.Name)
		}); err != nil {
			return record(e.Name+" admin up", err)
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
		addDesiredAddresses(cfg, b)
		return errs
	}
	if len(reconcileErrs) > 0 {
		errs = append(errs, reconcileErrs...)
		return rollbackPartial()
	}

	return errs
}

// reconcileOnReady runs Phase 3 (address reconcile) and Phase 4 (prune
// non-config interfaces) for cfg against backend b. It is called from
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

func reconcileOnReadyWithJournal(cfg *ifaceConfig, b Backend, journal *sdk.Journal, previous *ifaceConfig) (errs []error, deferred bool) {
	log := loggerPtr.Load()
	desiredAddrs, managedNames := cfg.desiredState()

	currentInfos, err := b.ListInterfaces()
	if err != nil {
		if errors.Is(err, ErrBackendNotReady) {
			return nil, true
		}
		errs = append(errs, fmt.Errorf("list interfaces for reconciliation: %w", err))
		return errs, false
	}

	record := func(msg string, err error) {
		log.Warn(msg, "err", err)
		errs = append(errs, fmt.Errorf("%s: %w", msg, err))
	}

	currentAddrs := currentAddrSet(currentInfos)

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
			return recreateManagedInterface(previous, name, b)
		}); err != nil {
			record("delete "+name+" ("+linkType+")", err)
			return errs, false
		} else {
			log.Info("iface config: deleted interface not in config", "name", name, "type", linkType)
		}
	}

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

func interfaceExists(b Backend, name string) bool {
	_, err := b.GetInterface(name)
	return err == nil
}

func recreateManagedInterface(cfg *ifaceConfig, name string, b Backend) error {
	if cfg == nil {
		return nil
	}
	for _, e := range cfg.Dummy {
		if !e.Disable && e.Name == name {
			return b.CreateDummy(e.Name)
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
		if err := recreateManagedVLAN(e.Name, e.Units, name, b); err != nil {
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

func recreateManagedVLAN(parent string, units []unitEntry, name string, b Backend) error {
	for i := range units {
		u := &units[i]
		if u.Disable || u.VLANID <= 0 {
			continue
		}
		var bCheck textbuf.Buffer
		if bCheck.Reset().Str(parent).Byte('.').Int(int64(u.VLANID)).Slice() != name {
			continue
		}
		return b.CreateVLAN(parent, u.VLANID)
	}
	return nil
}

// addDesiredAddresses adds every configured address without consulting the
// backend for current state. Used as the additive-only fallback when
// reconcileOnReady defers; the full reconcile fires later via
// vppevents.EventConnected.
func addDesiredAddresses(cfg *ifaceConfig, b Backend) {
	log := loggerPtr.Load()
	desiredAddrs, _ := cfg.desiredState()
	for osName, addrs := range desiredAddrs {
		for addr := range addrs {
			if err := b.AddAddress(osName, addr); err != nil {
				log.Debug("iface config: add address", "iface", osName, "addr", addr, "err", err)
			}
		}
	}
}

// reconcileOnVPPReady is invoked from the register.go EventBus handler when
// vppevents.EventConnected or EventReconnected fires. It looks up the
// currently-active config and, if one exists and the currently active
// backend is vpp, runs reconcileOnReady. It also retries b.StartMonitor so
// a monitor deferred at startup (backend not ready at OnConfigure time)
// installs as soon as the backend becomes live. No-op when no config has
// been applied yet, when the backend is still unregistered, or when the
// active backend is not vpp (the subscription is installed unconditionally
// for simplicity but should not mutate netlink state on a vpp lifecycle
// event, since vpp-ready has no meaning for the netlink backend).
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
	// idempotent -- see ifacenetlink/monitor_linux.go:364).
	if cfg.Backend != vppBackendName {
		return
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
