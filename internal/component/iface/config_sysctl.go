// Design: docs/features/interfaces.md -- Sysctl and mirror application for interfaces
// Related: config.go -- parsing, config_apply.go -- reconciliation

package iface

import (
	"encoding/json"
	"fmt"
	"strconv"

	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
	sysctlreg "github.com/ze-software/ze/internal/core/sysctl"
	"github.com/ze-software/ze/pkg/plugin/sdk"
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
		}{Key: key, Value: value, Source: "interface"})
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
		if s.RPFCheck != nil {
			emit("net.ipv4.conf."+osName+".rp_filter", strconv.Itoa(s.RPFCheck.rpfSysctlValue()))
		}
	}
	if u.MPLSEnable != nil {
		// RFC 3031: enabling MPLS label input on the interface. The global
		// net.mpls.platform_labels sysctl (label table size) is managed
		// separately via the sysctl config/known keys.
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
		if s.RPFCheck != nil && log != nil {
			log.Warn("iface: IPv6 rpf-check requires VPP data plane, ignored on Linux", "iface", osName)
		}
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

// applyMirror configures traffic mirroring on an interface from unit config.
// Only applied when at least one of ingress/egress destination is configured.
// Returns errors for mirror operations that failed.
func applyMirror(b Backend, osName string, u unitEntry, journal *sdk.Journal) []error {
	if u.MirrorIngress == "" && u.MirrorEgress == "" {
		return nil
	}

	var errs []error
	fail := func(what string, err error) {
		loggerPtr.Load().Warn("iface config: "+what, "iface", osName, "err", err)
		errs = append(errs, fmt.Errorf("%s %s: %w", osName, what, err))
	}

	ingress := u.MirrorIngress != ""
	egress := u.MirrorEgress != ""

	if ingress && egress && u.MirrorIngress == u.MirrorEgress {
		if err := applyBackendStep(journal, func() error {
			return b.SetupMirror(osName, u.MirrorIngress, true, true)
		}, func() error {
			return b.RemoveMirror(osName)
		}); err != nil {
			fail("mirror", err)
		}
		return errs
	}
	if ingress {
		if err := applyBackendStep(journal, func() error {
			return b.SetupMirror(osName, u.MirrorIngress, true, false)
		}, func() error {
			return b.RemoveMirror(osName)
		}); err != nil {
			fail("mirror ingress", err)
			return errs
		}
	}
	if egress {
		hadIngress := ingress
		ingressDst := u.MirrorIngress
		if err := applyBackendStep(journal, func() error {
			return b.SetupMirror(osName, u.MirrorEgress, false, true)
		}, func() error {
			if hadIngress {
				return b.SetupMirror(osName, ingressDst, true, false)
			}
			return b.RemoveMirror(osName)
		}); err != nil {
			fail("mirror egress", err)
			return errs
		}
	}
	return errs
}
