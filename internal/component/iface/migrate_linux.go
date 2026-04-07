// Design: docs/features/interfaces.md — Make-before-break interface migration
// Overview: migrate.go — MigrateConfig type and shared docs

package iface

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// validateMigrateConfig checks that all required fields are set and the
// interface type (if specified) is one of the known types.
func validateMigrateConfig(cfg MigrateConfig) error {
	if err := ValidateIfaceName(cfg.OldIface); err != nil {
		return fmt.Errorf("migrate: old interface: %w", err)
	}
	if err := ValidateIfaceName(cfg.NewIface); err != nil {
		return fmt.Errorf("migrate: new interface: %w", err)
	}
	if cfg.Address == "" {
		return errors.New("migrate: address is empty")
	}
	if cfg.NewIfaceType != "" {
		switch cfg.NewIfaceType {
		case "dummy", "veth", "bridge": // valid types
		default: // unknown type
			return fmt.Errorf("migrate: unknown interface type %q (expected dummy, veth, or bridge)", cfg.NewIfaceType)
		}
	}
	return nil
}

// resolveOSName returns the OS interface name for a given interface + unit.
// Unit 0 is the parent interface itself. Unit N (N > 0) is "<name>.<N>".
func resolveOSName(iface string, unit int) string {
	if unit == 0 {
		return iface
	}
	return fmt.Sprintf("%s.%d", iface, unit)
}

// bgpListenerReadyPayload is the JSON payload emitted by the BGP reactor
// on (bgp, listener-ready). We parse it to match on the address.
type bgpListenerReadyPayload struct {
	Address string `json:"address"`
}

// stripPrefix removes the prefix length from a CIDR string, returning just the IP.
func stripPrefix(cidr string) string {
	ip, _, ok := strings.Cut(cidr, "/")
	if ok {
		return ip
	}
	return cidr
}

// MigrateInterface performs a make-before-break IP migration.
//
// Five phases with strict ordering:
//  1. Create new interface (if NewIfaceType is set)
//  2. Add IP to new interface unit
//  3. Wait for BGP readiness ((bgp, listener-ready) event or timeout)
//  4. Remove IP from old interface unit
//  5. Remove old interface (optional, only if it was Ze-managed)
//
// Phase 4 MUST NOT start until Phase 3 confirms BGP has sessions on new address.
// On failure at any phase, earlier phases are rolled back.
func MigrateInterface(cfg MigrateConfig, eb ze.EventBus, timeout time.Duration) error {
	if err := validateMigrateConfig(cfg); err != nil {
		return err
	}
	if eb == nil {
		return errors.New("migrate: event bus is nil")
	}
	if timeout <= 0 {
		return errors.New("migrate: timeout must be positive")
	}

	newOSName := resolveOSName(cfg.NewIface, cfg.NewUnit)
	oldOSName := resolveOSName(cfg.OldIface, cfg.OldUnit)
	createdIface := false

	b := GetBackend()
	if b == nil {
		return errors.New("migrate: no backend loaded")
	}

	// Phase 1: Create new interface (if type specified).
	if cfg.NewIfaceType != "" {
		if err := createByType(b, cfg.NewIface, cfg.NewIfaceType); err != nil {
			return fmt.Errorf("migrate phase 1 (create): %w", err)
		}
		createdIface = true
	}

	// Phase 2: Add address to new interface.
	if err := b.AddAddress(newOSName, cfg.Address); err != nil {
		if createdIface {
			_ = b.DeleteInterface(cfg.NewIface) // rollback phase 1
		}
		return fmt.Errorf("migrate phase 2 (add address): %w", err)
	}

	// Phase 3: Wait for BGP readiness.
	targetIP := stripPrefix(cfg.Address)
	targetParsed, targetErr := netip.ParseAddr(targetIP)
	ready := make(chan struct{}, 1)

	unsub := eb.Subscribe(plugin.NamespaceBGP, plugin.EventListenerReady, func(payload string) {
		var p bgpListenerReadyPayload
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return
		}
		if p.Address == "" {
			return
		}
		// Match on the IP portion of the CIDR (BGP binds to the IP, not the prefix).
		addrIP := stripPrefix(p.Address)
		addrParsed, addrErr := netip.ParseAddr(addrIP)
		if (targetErr == nil && addrErr == nil && targetParsed == addrParsed) || p.Address == cfg.Address {
			select {
			case ready <- struct{}{}: // signal readiness
			default: // already signaled
			}
		}
	})
	defer unsub()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ready: // BGP is ready on the new address.
	case <-timer.C:
		// Timeout: rollback phase 2 and 1.
		_ = b.RemoveAddress(newOSName, cfg.Address)
		if createdIface {
			_ = b.DeleteInterface(cfg.NewIface)
		}
		return fmt.Errorf("migrate phase 3: timed out waiting for BGP readiness on %s", cfg.Address)
	}

	// Phase 4: Remove address from old interface. The new address is live and BGP
	// is running. Failure means the old address remains (dual-homed temporarily).
	if err := b.RemoveAddress(oldOSName, cfg.Address); err != nil {
		loggerPtr.Load().Warn("migrate phase 4: old address not removed (dual-homed)",
			"interface", oldOSName, "address", cfg.Address, "err", err)
		return fmt.Errorf("migrate phase 4: remove old address: %w", err)
	}

	// Phase 5: Old interface cleanup is left to the caller. We cannot determine
	// whether the old interface is Ze-managed or a physical NIC from this context.
	// Callers should use DeleteInterface explicitly if the old interface should be removed.

	return nil
}

// createByType creates an interface of the given type via the backend.
func createByType(b Backend, name, ifaceType string) error {
	switch ifaceType {
	case "dummy":
		return b.CreateDummy(name)
	case "veth":
		// Veth requires a peer name; use "<name>-peer" by convention.
		return b.CreateVeth(name, name+"-peer")
	case "bridge":
		return b.CreateBridge(name)
	default: // unreachable after validateMigrateConfig, but defensive
		return fmt.Errorf("migrate: unknown interface type %q", ifaceType)
	}
}
