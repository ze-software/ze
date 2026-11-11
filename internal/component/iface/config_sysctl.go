// Design: docs/features/interfaces.md -- Sysctl application for interfaces
// Related: config.go -- parsing, config_apply.go -- reconciliation, config_mirror.go -- mirroring

package iface

import (
	"encoding/json"
	"strconv"

	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
	sysctlreg "github.com/ze-software/ze/internal/core/sysctl"
)

// applySysctl emits per-interface sysctl defaults on the EventBus.
// The sysctl plugin receives these and writes them to the kernel.
// Only settings explicitly configured (non-nil) are emitted.
func applySysctl(osName string, u unitEntry) {
	eb := GetEventBus()
	if eb == nil {
		return
	}
	log := loggerPtr.Load()

	emit := func(key, value string) {
		payload, _ := json.Marshal(struct {
			Key    string `json:"key"`
			Value  string `json:"value"`
			Source string `json:"source"`
		}{Key: key, Value: value, Source: componentNameInterface})
		if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventDefault, string(payload)); err != nil {
			log.Debug("iface: sysctl emit failed", "key", key, "err", err)
		}
	}

	boolVal := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}

	if s := u.IPv4; s != nil {
		if s.Forwarding != nil {
			emit("net.ipv4.conf."+osName+".forwarding", boolVal(*s.Forwarding))
		}
		if s.ArpFilter != nil {
			emit("net.ipv4.conf."+osName+".arp_filter", boolVal(*s.ArpFilter))
		}
		if s.ArpAccept != nil {
			emit("net.ipv4.conf."+osName+".arp_accept", boolVal(*s.ArpAccept))
		}
		if s.ProxyARP != nil {
			emit("net.ipv4.conf."+osName+".proxy_arp", boolVal(*s.ProxyARP))
		}
		if s.ArpAnnounce != nil {
			emit("net.ipv4.conf."+osName+".arp_announce", strconv.Itoa(*s.ArpAnnounce))
		}
		if s.ArpIgnore != nil {
			emit("net.ipv4.conf."+osName+".arp_ignore", strconv.Itoa(*s.ArpIgnore))
		}
		// The rp_filter sysctl judges what the Linux kernel forwards. A
		// backend with its own dataplane enforces the same leaf through
		// Backend.ReplaceIPv4RPF (rpf4.go, desiredIPv4RPF).
		if s.RPFCheck != nil {
			emit("net.ipv4.conf."+osName+".rp_filter", strconv.Itoa(s.RPFCheck.rpfSysctlValue()))
		}
	}
	if u.MPLSEnable != nil {
		// RFC 3031: enabling MPLS label input on the interface. The global
		// net.mpls.platform_labels sysctl (label table size) is managed
		// separately via the sysctl config/known keys. A backend with its own
		// dataplane enforces the same leaf through Backend.ReplaceMPLSInput
		// (mplsinput.go, desiredMPLSInput).
		emit("net.mpls.conf."+osName+".input", boolVal(*u.MPLSEnable))
	}
	if s := u.IPv6; s != nil {
		if s.Autoconf != nil {
			emit("net.ipv6.conf."+osName+".autoconf", boolVal(*s.Autoconf))
		}
		if s.AcceptRA != nil {
			emit("net.ipv6.conf."+osName+".accept_ra", strconv.Itoa(*s.AcceptRA))
		}
		if s.Forwarding != nil {
			emit("net.ipv6.conf."+osName+".forwarding", boolVal(*s.Forwarding))
		}
		// IPv6 rpf-check has no sysctl (Linux limitation LL-IPV6-NO-RP-FILTER). It is
		// enforced by Backend.ReplaceIPv6RPF (rpf6.go, desiredIPv6RPF).
	}
}

// sourceIPv6Forwarding names the sysctl default the interface component holds
// on net.ipv6.conf.all.forwarding. It is apart from componentNameInterface so
// clearing it touches no per-device default.
const sourceIPv6Forwarding = "interface-ipv6-forwarding"

// forwardsIPv6 reports whether the unit sets ipv6 forwarding true.
func (u *unitEntry) forwardsIPv6() bool {
	if u.IPv6 == nil {
		return false
	}
	if u.IPv6.Forwarding == nil {
		return false
	}
	return *u.IPv6.Forwarding
}

// applyIPv6ForwardingSwitch holds net.ipv6.conf.all.forwarding at 1 while a
// unit forwards IPv6, and withdraws that default when none does. Linux
// forwards an IPv6 packet only while all.forwarding is 1, or while the ingress
// device's force_forwarding is 1 (net/ipv6/ip6_output.c ip6_forward): the
// per-device forwarding applySysctl writes decides only the host or router
// role, so on its own it routes nothing. The kernel copies the global value
// into every device; the sysctl store then writes back each unit's own
// forwarding leaf (store.write in internal/component/sysctl/sysctl.go), and
// withdrawing hands the key back to another source that still claims it, or
// to its value before Ze.
func applyIPv6ForwardingSwitch(on bool) {
	eb := GetEventBus()
	if eb == nil {
		return
	}
	log := loggerPtr.Load()

	if !on {
		payload, _ := json.Marshal(struct {
			Source string `json:"source"`
		}{Source: sourceIPv6Forwarding})
		if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventClearSourceDefaults, string(payload)); err != nil {
			log.Warn("iface: cannot withdraw the IPv6 forwarding switch; inspect with ze show sysctl", "err", err)
		}
		return
	}
	payload, _ := json.Marshal(struct {
		Key    string `json:"key"`
		Value  string `json:"value"`
		Source string `json:"source"`
	}{Key: "net.ipv6.conf.all.forwarding", Value: "1", Source: sourceIPv6Forwarding})
	if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventDefault, string(payload)); err != nil {
		log.Warn("iface: cannot turn the IPv6 forwarding switch on; units with ipv6 forwarding true do not route", "err", err)
	}
}

// applySysctlProfiles resolves named profiles and emits their settings as
// sysctl defaults via EventBus. Emits clear-profile-defaults first to remove
// stale keys from a previous config cycle, then emits each profile's settings
// in order (last wins on key overlap).
func applySysctlProfiles(osName string, profiles []string) {
	if len(profiles) == 0 {
		return
	}
	eb := GetEventBus()
	if eb == nil {
		return
	}
	log := loggerPtr.Load()

	// Clear stale profile defaults for this interface before re-emitting.
	clearPayload, _ := json.Marshal(struct {
		Interface string `json:"interface"`
	}{Interface: osName})
	if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventClearProfileDefaults, string(clearPayload)); err != nil {
		log.Debug("iface: clear-profile-defaults emit failed", "iface", osName, "err", err)
	}

	for _, name := range profiles {
		p, ok := sysctlreg.LookupProfile(name)
		if !ok {
			log.Warn("iface: unknown sysctl profile", "profile", name, "iface", osName)
			continue
		}
		resolved := sysctlreg.ResolveProfileSettings(p.Settings, osName)
		for _, s := range resolved {
			payload, _ := json.Marshal(struct {
				Key    string `json:"key"`
				Value  string `json:"value"`
				Source string `json:"source"`
			}{Key: s.Key, Value: s.Value, Source: "profile:" + name})
			if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventDefault, string(payload)); err != nil {
				log.Debug("iface: profile sysctl emit failed", "key", s.Key, "profile", name, "err", err)
			}
		}
	}
}
