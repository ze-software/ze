// Design: docs/guide/pppoe.md -- IPv6 on the PPPoE client
// Related: pppoe_client.go -- the session worker that applies these settings
// Related: config.go -- parsePPPoEClientEntry reads the ipv6 container

package iface

import (
	"encoding/json"
	"errors"
	"net/netip"
	"strconv"

	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
)

// pppoeClientSysctlMax is the number of per-device IPv6 sysctls a
// pppoe-client can set: autoconf, accept_ra and forwarding.
const pppoeClientSysctlMax = 3

// pppoeClientSysctl is one net.ipv6.conf.<device>.<Suffix> write. An empty
// Suffix is an unused slot.
type pppoeClientSysctl struct {
	Suffix string
	Value  string
}

// PPPoEClientIPv6 is the ipv6 container of a pppoe-client. It is a value with
// no pointer, slice or map, so PPPoEClientConfig stays comparable with ==
// (pppoeClientConfigChanged). The zero value is "no ipv6": IPv6CP is not
// negotiated, and no IPv6 sysctl and no DHCPv6 client run on the device.
type PPPoEClientIPv6 struct {
	// Enabled is the ipv6 enabled leaf: the client negotiates IPv6CP
	// (RFC 5072) and installs the link-local address it yields.
	Enabled bool
	// Sysctls are the configured autoconf, accept-ra and forwarding leaves,
	// written to the session device each time a session brings it up.
	Sysctls [pppoeClientSysctlMax]pppoeClientSysctl
	// DHCPv6 is the dhcpv6 container; DHCPv6.Enabled false runs no client.
	DHCPv6 dhcpv6UnitConfig
	// AcceptRA is the accept-ra leaf as a number (0, 1 or 2). Its zero is
	// both "absent" and "0", and acceptsRA reads either as "not processing
	// RAs", which is what both mean for RA-learned resolvers.
	AcceptRA int
}

// acceptsRA reports whether the configured accept-ra leaf lets the kernel
// process Router Advertisements on the session device: written, and above 0.
// An absent leaf leaves the kernel's own default, which Ze does not read, so
// the session is then no target for RA-learned resolvers (ra_rdnss.go).
func (v *PPPoEClientIPv6) acceptsRA() bool { return v.AcceptRA > 0 }

// errPPPoEClientIPv6Disabled refuses IPv6 leaves on a client that does not
// negotiate IPv6CP, which would otherwise configure a device that never
// carries IPv6.
var errPPPoEClientIPv6Disabled = errors.New("ipv6: the container sets IPv6 leaves but ipv6 enabled is not true, so no IPv6CP is negotiated and the leaves would never apply; set ipv6 enabled true or remove the ipv6 container")

// parsePPPoEClientIPv6 reads the ipv6 container of one pppoe-client entry.
func parsePPPoEClientIPv6(m map[string]any) (PPPoEClientIPv6, error) {
	v6, ok := m["ipv6"].(map[string]any)
	if !ok {
		return PPPoEClientIPv6{}, nil
	}
	var out PPPoEClientIPv6
	if v, ok := v6["enabled"].(string); ok {
		out.Enabled = v == yangTrue
	}
	slot := 0
	if v, ok := v6["autoconf"].(string); ok {
		out.Sysctls[slot] = pppoeClientSysctl{Suffix: "autoconf", Value: sysctlBool(v == yangTrue)}
		slot++
	}
	if v, ok := v6["accept-ra"].(string); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return PPPoEClientIPv6{}, errors.New("ipv6 accept-ra " + strconv.Quote(v) + ": not a number; write 0, 1 or 2")
		}
		if n < 0 {
			return PPPoEClientIPv6{}, errors.New("ipv6 accept-ra " + v + ": out of range; write 0, 1 or 2")
		}
		if n > 2 {
			return PPPoEClientIPv6{}, errors.New("ipv6 accept-ra " + v + ": out of range; write 0, 1 or 2")
		}
		out.Sysctls[slot] = pppoeClientSysctl{Suffix: "accept_ra", Value: v}
		out.AcceptRA = n
		slot++
	}
	if v, ok := v6["forwarding"].(string); ok {
		out.Sysctls[slot] = pppoeClientSysctl{Suffix: "forwarding", Value: sysctlBool(v == yangTrue)}
	}
	if dhcpv6 := parseDHCPv6Config(v6); dhcpv6 != nil {
		out.DHCPv6 = *dhcpv6
	}
	if out.Enabled {
		return out, nil
	}
	if out != (PPPoEClientIPv6{}) {
		return PPPoEClientIPv6{}, errPPPoEClientIPv6Disabled
	}
	return out, nil
}

// sysctlBool is the sysctl spelling of a boolean leaf.
func sysctlBool(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// pppoeClientSysctlSource names the sysctl defaults of one pppoe-client, so
// clear-source-defaults withdraws exactly its own.
func pppoeClientSysctlSource(name string) string {
	return "pppoe-client:" + name
}

// applyPPPoEClientSysctls writes the client's IPv6 sysctls to the session
// device the dialer has just created under name.
//
// The device is new on every session, so the kernel holds its own defaults
// there again, while the sysctl store remembers the value it wrote to the
// previous device and skips a repeated default (sysctl store setDefault).
// Clearing this client's source first makes the store write every value to
// the new device.
func applyPPPoEClientSysctls(name string, v6 PPPoEClientIPv6) {
	eb := GetEventBus()
	if eb == nil {
		return
	}
	log := loggerPtr.Load()
	source := pppoeClientSysctlSource(name)
	clearPayload, err := json.Marshal(struct {
		Source string `json:"source"`
	}{Source: source})
	if err != nil {
		log.Warn("pppoe-client: cannot encode the sysctl reset; IPv6 sysctls may keep the previous session's state", "name", name, "error", err)
		return
	}
	if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventClearSourceDefaults, string(clearPayload)); err != nil {
		log.Warn("pppoe-client: cannot reset the IPv6 sysctls of the session device; inspect with ze show sysctl", "name", name, "error", err)
	}
	for _, s := range v6.Sysctls {
		if s.Suffix == "" {
			continue
		}
		payload, err := json.Marshal(struct {
			Key    string `json:"key"`
			Value  string `json:"value"`
			Source string `json:"source"`
		}{Key: "net.ipv6.conf." + name + "." + s.Suffix, Value: s.Value, Source: source})
		if err != nil {
			log.Warn("pppoe-client: cannot encode an IPv6 sysctl", "name", name, "sysctl", s.Suffix, "error", err)
			continue
		}
		if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventDefault, string(payload)); err != nil {
			log.Warn("pppoe-client: cannot apply an IPv6 sysctl to the session device; inspect with ze show sysctl", "name", name, "sysctl", s.Suffix, "error", err)
		}
	}
}

// pppoeLinkLocalPrefix is the address applyPPPoELinkLocal installs: the
// link-local address in fe80::/64 (RFC 4291 Section 2.5.6).
func pppoeLinkLocalPrefix(addr netip.Addr) string {
	return netip.PrefixFrom(addr, 64).String()
}

// applyPPPoELinkLocal moves the device's IPv6CP link-local address from
// applied to next, either of which MAY be invalid, and returns the address
// now installed. RFC 5072 Section 5: "The negotiated interface identifier is
// used by the local end of the PPP link to autoconfigure an IPv6 link-local
// unicast address for the PPP interface." Linux does not derive one on a PPP
// device itself, as pppd's sif6addr shows by installing it.
func applyPPPoELinkLocal(b Backend, ifname string, applied, next netip.Addr) (netip.Addr, error) {
	if next == applied {
		return applied, nil
	}
	if applied.IsValid() {
		if err := b.RemoveAddress(ifname, pppoeLinkLocalPrefix(applied)); err != nil {
			return applied, err
		}
	}
	if next.IsValid() {
		if err := b.AddAddress(ifname, pppoeLinkLocalPrefix(next)); err != nil {
			return netip.Addr{}, err
		}
	}
	return next, nil
}
