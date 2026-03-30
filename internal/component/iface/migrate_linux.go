// Design: plan/spec-iface-0-umbrella.md — Make-before-break interface migration
// Overview: iface.go — shared types and topic constants

package iface

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// BGP listener readiness topic. The BGP subsystem publishes this when a
// listener is active on a new address. MigrateInterface waits for this event
// before removing the old address (phase 4).
const topicBGPListenerReady = "bgp/listener/ready"

// MigrateConfig describes a make-before-break IP migration.
type MigrateConfig struct {
	// Source: the old interface/unit to migrate FROM.
	OldIface string
	OldUnit  int
	Address  string // CIDR to migrate (e.g., "10.0.0.1/24")

	// Destination: the new interface/unit to migrate TO.
	NewIface     string
	NewUnit      int
	NewIfaceType string // "dummy", "veth", "bridge" (empty = already exists)
}

// validateMigrateConfig checks that all required fields are set and the
// interface type (if specified) is one of the known types.
func validateMigrateConfig(cfg MigrateConfig) error {
	if err := validateIfaceName(cfg.OldIface); err != nil {
		return fmt.Errorf("migrate: old interface: %w", err)
	}
	if err := validateIfaceName(cfg.NewIface); err != nil {
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

// bgpReadyConsumer receives events from the Bus and signals when a
// bgp/listener/ready event matches the target address.
type bgpReadyConsumer struct {
	targetAddr string
	ready      chan struct{}
}

// Deliver checks each event for a matching address and signals readiness.
func (c *bgpReadyConsumer) Deliver(events []ze.Event) error {
	for _, ev := range events {
		var payload struct {
			Address string `json:"address"`
		}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			continue
		}
		// Match on the IP portion of the CIDR (BGP binds to the IP, not the prefix).
		addr := payload.Address
		if addr == "" {
			if a, ok := ev.Metadata["address"]; ok {
				addr = a
			}
		}
		targetIP := stripPrefix(c.targetAddr)
		addrIP := stripPrefix(addr)
		if addrIP == targetIP || addr == c.targetAddr {
			select {
			case c.ready <- struct{}{}: // signal readiness
			default: // already signaled
			}
			return nil
		}
	}
	return nil
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
//  3. Wait for BGP readiness (bus event "bgp/listener/ready" or timeout)
//  4. Remove IP from old interface unit
//  5. Remove old interface (optional, only if it was Ze-managed)
//
// Phase 4 MUST NOT start until Phase 3 confirms BGP has sessions on new address.
// On failure at any phase, earlier phases are rolled back.
func MigrateInterface(cfg MigrateConfig, bus ze.Bus, timeout time.Duration) error {
	if err := validateMigrateConfig(cfg); err != nil {
		return err
	}
	if bus == nil {
		return errors.New("migrate: bus is nil")
	}
	if timeout <= 0 {
		return errors.New("migrate: timeout must be positive")
	}

	newOSName := resolveOSName(cfg.NewIface, cfg.NewUnit)
	oldOSName := resolveOSName(cfg.OldIface, cfg.OldUnit)
	createdIface := false

	// Phase 1: Create new interface (if type specified).
	if cfg.NewIfaceType != "" {
		if err := createByType(cfg.NewIface, cfg.NewIfaceType); err != nil {
			return fmt.Errorf("migrate phase 1 (create): %w", err)
		}
		createdIface = true
	}

	// Phase 2: Add address to new interface.
	if err := AddAddress(newOSName, cfg.Address); err != nil {
		if createdIface {
			_ = DeleteInterface(cfg.NewIface) // rollback phase 1
		}
		return fmt.Errorf("migrate phase 2 (add address): %w", err)
	}

	// Phase 3: Wait for BGP readiness.
	consumer := &bgpReadyConsumer{
		targetAddr: cfg.Address,
		ready:      make(chan struct{}, 1),
	}
	sub, err := bus.Subscribe(topicBGPListenerReady, nil, consumer)
	if err != nil {
		// Rollback phase 2 and 1.
		_ = RemoveAddress(newOSName, cfg.Address)
		if createdIface {
			_ = DeleteInterface(cfg.NewIface)
		}
		return fmt.Errorf("migrate phase 3 (subscribe): %w", err)
	}
	defer bus.Unsubscribe(sub)

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-consumer.ready: // BGP is ready on the new address.
	case <-timer.C:
		// Timeout: rollback phase 2 and 1.
		_ = RemoveAddress(newOSName, cfg.Address)
		if createdIface {
			_ = DeleteInterface(cfg.NewIface)
		}
		return fmt.Errorf("migrate phase 3: timed out waiting for BGP readiness on %s", cfg.Address)
	}

	// Phase 4: Remove address from old interface. The new address is live and BGP
	// is running. Failure means the old address remains (dual-homed temporarily).
	if err := RemoveAddress(oldOSName, cfg.Address); err != nil {
		loggerPtr.Load().Warn("migrate phase 4: old address not removed (dual-homed)",
			"interface", oldOSName, "address", cfg.Address, "err", err)
		return fmt.Errorf("migrate phase 4: remove old address: %w", err)
	}

	// Phase 5: Old interface cleanup is left to the caller. We cannot determine
	// whether the old interface is Ze-managed or a physical NIC from this context.
	// Callers should use DeleteInterface explicitly if the old interface should be removed.

	return nil
}

// createByType creates an interface of the given type.
func createByType(name, ifaceType string) error {
	switch ifaceType {
	case "dummy":
		return CreateDummy(name)
	case "veth":
		// Veth requires a peer name; use "<name>-peer" by convention.
		return CreateVeth(name, name+"-peer")
	case "bridge":
		return CreateBridge(name)
	default: // unreachable after validateMigrateConfig, but defensive
		return fmt.Errorf("migrate: unknown interface type %q", ifaceType)
	}
}
