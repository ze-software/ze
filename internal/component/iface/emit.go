// Design: docs/features/interfaces.md -- interface config emission from discovery
// Related: discover.go -- DiscoverInterfaces produces the input
// Related: iface.go -- DiscoveredInterface type

package iface

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/config/secret"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// EmitConfig produces Ze config syntax for a slice of DiscoveredInterfaces.
// Used by `ze init` to write the initial config from kernel state and by
// `ze interface scan --config` to emit a snapshot on demand. Sensitive
// wireguard fields (private-key, peer preshared-key) are passed through
// secret.Encode so the output matches the $9$-encoded form that the
// config parser auto-decodes on load.
//
// Returns an empty string if discovered is empty -- callers should treat
// the empty return as "nothing to emit" and skip writing the file.
func EmitConfig(discovered []DiscoveredInterface) string {
	if len(discovered) == 0 {
		return ""
	}

	unique := uniquePermanentMACs(discovered)

	var b textbuf.Buffer
	b.WriteString("interface {\n")

	hasLoopback := false
	for i := range discovered {
		di := &discovered[i]
		switch di.Type {
		case zeTypeLoopback:
			hasLoopback = true
		case zeTypeEthernet, zeTypeBridge, zeTypeVeth, zeTypeDummy:
			if !safeEmitName(di.Name) {
				continue
			}
			fmt.Fprintf(&b, "    %s %s {\n", di.Type, di.Name) //nolint:errcheck // buffer output
			emitSelectorBlock(&b, di, unique, "        ")
			b.WriteString("    }\n")
		case zeTypeWireguard:
			if !safeEmitName(di.Name) {
				continue
			}
			emitWireguardBlock(&b, di)
		case zeTypeXFRM:
			if !safeEmitName(di.Name) {
				continue
			}
			emitXFRMBlock(&b, di)
		}
	}

	if hasLoopback {
		fmt.Fprintf(&b, "    %s {\n", zeTypeLoopback) //nolint:errcheck // buffer output
		b.WriteString("    }\n")
	}

	b.WriteString("}\n")
	return b.String()
}

// zeroMAC is an all-zero hardware address. A driver that reports one for
// IFLA_PERM_ADDRESS has reported nothing, and reading it as an address would
// write a selector that names no device.
const zeroMAC = "00:00:00:00:00:00"

// uniquePermanentMACs returns the permanent addresses carried by exactly one
// discovered ethernet.
//
// A factory address two NICs report is not an identity. validateSelectors
// (config_apply.go) REFUSES a commit whose selector names more than one device,
// so emitting it would cost a first-boot appliance its whole config rather than
// one interface. Those entries fall back to the os-name selector, which is what
// discovery wrote for every kind before the selector existed.
func uniquePermanentMACs(discovered []DiscoveredInterface) map[string]bool {
	seen := make(map[string]int, len(discovered))
	for i := range discovered {
		di := &discovered[i]
		if di.Type != zeTypeEthernet || di.PermanentMAC == zeroMAC || !safeEmitName(di.PermanentMAC) {
			continue
		}
		seen[di.PermanentMAC]++
	}
	unique := make(map[string]bool, len(seen))
	for mac, count := range seen {
		if count == 1 {
			unique[mac] = true
		}
	}
	return unique
}

// matchMACFor returns the hardware address an emitted ethernet entry binds to,
// or "" when the entry must fall back to the os-name selector. unique comes
// from uniquePermanentMACs over the same discovered set.
//
// Only the physical kind is matched, and only by its PERMANENT (factory)
// address. That address identifies the NIC: it survives a kernel rename and an
// operational MAC override, so the entry keeps reaching the same port. The
// kinds Ze creates (bridge, veth, dummy) report no permanent address and are
// identified by the name Ze assigns them, which is what the mac/match leaf
// states (ze-iface-conf.yang).
func matchMACFor(di *DiscoveredInterface, unique map[string]bool) string {
	if !unique[di.PermanentMAC] {
		return ""
	}
	return di.PermanentMAC
}

// emitSelectorBlock writes the hardware selector for one discovered interface
// in block syntax, indented by indent.
//
// A discovered ethernet NEVER gets a mac address override. That leaf IMPOSES an
// address on whichever device the entry resolves to, so an entry that binds by
// NAME and carries one writes this NIC's address onto a different NIC the first
// time the kernel hands the name to another port. It also does nothing in the
// healthy case, because the address it writes back is the one the NIC already
// has. The created kinds keep it: there the address is an instruction, and it
// pins the kernel's random choice across a recreate.
func emitSelectorBlock(b *textbuf.Buffer, di *DiscoveredInterface, unique map[string]bool, indent string) {
	if match := matchMACFor(di, unique); match != "" {
		b.Str(indent).Str("mac {\n").Str(indent).Str("    match ").Str(match).Str(";\n").Str(indent).Str("}\n")
		return
	}
	if di.Type != zeTypeEthernet && safeEmitName(di.MAC) {
		b.Str(indent).Str("mac {\n").Str(indent).Str("    address ").Str(di.MAC).Str(";\n").Str(indent).Str("}\n")
	}
	b.Str(indent).Str("os-name ").Str(di.Name).Str(";\n")
}

// emitSelectorSet writes the same selector as emitSelectorBlock in set-command
// syntax. prefix is "set interface <type> <name>".
func emitSelectorSet(b *textbuf.Buffer, di *DiscoveredInterface, unique map[string]bool, prefix string) {
	if match := matchMACFor(di, unique); match != "" {
		b.Str(prefix).Str(" mac match ").Str(match).Byte('\n')
		return
	}
	if di.Type != zeTypeEthernet && safeEmitName(di.MAC) {
		b.Str(prefix).Str(" mac address ").Str(di.MAC).Byte('\n')
	}
	b.Str(prefix).Str(" os-name ").Str(di.Name).Byte('\n')
}

// emitWireguardBlock writes a wireguard list entry for a discovered netdev.
// If Wireguard is nil (backend could not read kernel state, or wgctrl
// returned an error), a skeleton block is emitted with only the os-name
// leaf so the operator can fill the rest in after the scan. When the spec
// is available, sensitive fields (private-key, peer preshared-key) are
// passed through secret.Encode so the output gets the $9$-encoded form,
// matching the sensitive-leaf pattern used for BGP MD5 passwords and
// other secrets in ze.
func emitWireguardBlock(b *textbuf.Buffer, di *DiscoveredInterface) {
	b.Str("    wireguard ").Str(di.Name).Str(" {\n")
	b.Str("        os-name ").Str(di.Name).Str(";\n")
	spec := di.Wireguard
	if spec == nil {
		b.Str("    }\n")
		return
	}
	if spec.ListenPortSet && spec.ListenPort != 0 {
		b.Str("        listen-port ").Int(int64(spec.ListenPort)).Str(";\n")
	}
	if spec.FirewallMark != 0 {
		b.Str("        fwmark ").Uint(uint64(spec.FirewallMark)).Str(";\n")
	}
	if encoded, err := secret.Encode(spec.PrivateKey.String()); err == nil {
		b.Str("        private-key \"").Str(encoded).Str("\";\n")
	}
	for idx := range spec.Peers {
		p := &spec.Peers[idx]
		b.Str("        peer peer").Int(int64(idx)).Str(" {\n")
		b.Str("            public-key \"").Str(p.PublicKey.String()).Str("\";\n")
		if p.HasPresharedKey {
			if encoded, err := secret.Encode(p.PresharedKey.String()); err == nil {
				b.Str("            preshared-key \"").Str(encoded).Str("\";\n")
			}
		}
		if p.EndpointIP != "" && p.EndpointPort != 0 {
			b.Str("            endpoint {\n")
			b.Str("                ip ").Str(p.EndpointIP).Str(";\n")
			b.Str("                port ").Int(int64(p.EndpointPort)).Str(";\n")
			b.Str("            }\n")
		}
		if len(p.AllowedIPs) > 0 {
			b.Str("            allowed-ips [")
			for _, cidr := range p.AllowedIPs {
				b.Byte(' ').Str(cidr)
			}
			b.Str(" ];\n")
		}
		if p.PersistentKeepalive != 0 {
			b.Str("            persistent-keepalive ").Int(int64(p.PersistentKeepalive)).Str(";\n")
		}
		b.Str("        }\n")
	}
	b.Str("    }\n")
}

// EmitSetConfigWithDHCP produces set-command format with DHCPv4 enabled
// on every discovered ethernet interface. Used by the first-boot
// bootstrap path so the active config has explicit DHCP units that
// do not depend on runtime re-discovery via dhcp-auto.
func EmitSetConfigWithDHCP(discovered []DiscoveredInterface) string {
	return emitSetConfig(discovered, true)
}

func emitSetConfig(discovered []DiscoveredInterface, dhcpEthernet bool) string {
	if len(discovered) == 0 {
		return ""
	}

	unique := uniquePermanentMACs(discovered)

	var b textbuf.Buffer
	for i := range discovered {
		di := &discovered[i]
		switch di.Type {
		case zeTypeLoopback:
			// A bare "set interface loopback" with no child is invalid.
			continue
		case zeTypeEthernet, zeTypeBridge, zeTypeVeth, zeTypeDummy:
			if !safeEmitName(di.Name) {
				continue
			}
			var tb textbuf.Buffer
			emitSelectorSet(&b, di, unique, tb.Str("set interface ").Str(di.Type).Byte(' ').Str(di.Name).String())
			if dhcpEthernet && di.Type == zeTypeEthernet {
				b.Str("set interface ethernet ").Str(di.Name).Str(" unit default ipv4 dhcp enabled true\n")
			}
		case zeTypeWireguard:
			if !safeEmitName(di.Name) {
				continue
			}
			emitWireguardSet(&b, di)
		case zeTypeXFRM:
			if !safeEmitName(di.Name) {
				continue
			}
			emitXFRMSet(&b, di)
		}
	}
	return b.String()
}

// emitWireguardSet writes set-command lines for a discovered wireguard device.
func emitWireguardSet(b *textbuf.Buffer, di *DiscoveredInterface) {
	var tb textbuf.Buffer
	prefix := tb.Str("set interface wireguard ").Str(di.Name).String()
	b.Str(prefix).Str(" os-name ").Str(di.Name).Byte('\n')
	spec := di.Wireguard
	if spec == nil {
		return
	}
	if spec.ListenPortSet && spec.ListenPort != 0 {
		b.Str(prefix).Str(" listen-port ").Int(int64(spec.ListenPort)).Byte('\n')
	}
	if spec.FirewallMark != 0 {
		b.Str(prefix).Str(" fwmark ").Uint(uint64(spec.FirewallMark)).Byte('\n')
	}
	if encoded, err := secret.Encode(spec.PrivateKey.String()); err == nil {
		b.Str(prefix).Str(" private-key \"").Str(encoded).Str("\"\n")
	}
	for idx := range spec.Peers {
		p := &spec.Peers[idx]
		peerPrefix := tb.Reset().Str(prefix).Str(" peer peer").Int(int64(idx)).String()
		b.Str(peerPrefix).Str(" public-key \"").Str(p.PublicKey.String()).Str("\"\n")
		if p.HasPresharedKey {
			if encoded, err := secret.Encode(p.PresharedKey.String()); err == nil {
				b.Str(peerPrefix).Str(" preshared-key \"").Str(encoded).Str("\"\n")
			}
		}
		if p.EndpointIP != "" && p.EndpointPort != 0 {
			b.Str(peerPrefix).Str(" endpoint ip ").Str(p.EndpointIP).Byte('\n')
			b.Str(peerPrefix).Str(" endpoint port ").Int(int64(p.EndpointPort)).Byte('\n')
		}
		if len(p.AllowedIPs) > 0 {
			for _, cidr := range p.AllowedIPs {
				b.Str(peerPrefix).Str(" allowed-ips ").Str(cidr).Byte('\n')
			}
		}
		if p.PersistentKeepalive != 0 {
			b.Str(peerPrefix).Str(" persistent-keepalive ").Int(int64(p.PersistentKeepalive)).Byte('\n')
		}
	}
}

func emitXFRMBlock(b *textbuf.Buffer, di *DiscoveredInterface) {
	b.WriteString("    xfrm ")
	b.WriteString(di.Name)
	b.WriteString(" {\n")
	b.WriteString("        os-name ")
	b.WriteString(di.Name)
	b.WriteString(";\n")
	info := di.XFRM
	if info == nil {
		b.WriteString("    }\n")
		return
	}
	b.WriteString("        if-id ")
	b.WriteString(textbuf.StringUint32(info.IfID))
	b.WriteString(";\n")
	if info.ParentDev != "" {
		b.WriteString("        dev ")
		b.WriteString(info.ParentDev)
		b.WriteString(";\n")
	}
	if len(info.Addresses) > 0 {
		b.WriteString("        unit default {\n")
		for _, addr := range info.Addresses {
			if strings.Contains(addr, ":") {
				b.WriteString("            ipv6 {\n                address ")
				b.WriteString(addr)
				b.WriteString(";\n            }\n")
			} else {
				b.WriteString("            ipv4 {\n                address ")
				b.WriteString(addr)
				b.WriteString(";\n            }\n")
			}
		}
		b.WriteString("        }\n")
	}
	b.WriteString("    }\n")
}

func emitXFRMSet(b *textbuf.Buffer, di *DiscoveredInterface) {
	var tb textbuf.Buffer
	prefix := tb.Str("set interface xfrm ").Str(di.Name).String()
	b.WriteString(prefix)
	b.WriteString(" os-name ")
	b.WriteString(di.Name)
	b.WriteByte('\n')
	info := di.XFRM
	if info == nil {
		return
	}
	b.WriteString(prefix)
	b.WriteString(" if-id ")
	b.WriteString(textbuf.StringUint32(info.IfID))
	b.WriteByte('\n')
	if info.ParentDev != "" {
		b.WriteString(prefix)
		b.WriteString(" dev ")
		b.WriteString(info.ParentDev)
		b.WriteByte('\n')
	}
	for _, addr := range info.Addresses {
		b.WriteString(prefix)
		if strings.Contains(addr, ":") {
			b.WriteString(" unit default ipv6 address ")
		} else {
			b.WriteString(" unit default ipv4 address ")
		}
		b.WriteString(addr)
		b.WriteByte('\n')
	}
}

// EmitBootstrapConfig produces a minimal ze config for first-boot bootstrap
// mode. For every ethernet interface in discovered, it emits an interface
// block with DHCP client enabled. Non-ethernet types are skipped (bridge,
// veth, dummy, loopback, wireguard, xfrm are not useful for bootstrap
// reachability). An SSH block is appended so the operator can connect.
// Returns empty string if no ethernet interfaces are found.
func EmitBootstrapConfig(discovered []DiscoveredInterface) string {
	unique := uniquePermanentMACs(discovered)

	var b textbuf.Buffer
	hasEthernet := false

	for i := range discovered {
		di := &discovered[i]
		if di.Type != zeTypeEthernet {
			continue
		}
		if !safeEmitName(di.Name) {
			continue
		}
		if !hasEthernet {
			b.WriteString("interface {\n")
			hasEthernet = true
		}
		b.WriteString("    ethernet ")
		b.WriteString(di.Name)
		b.WriteString(" {\n")
		emitSelectorBlock(&b, di, unique, "        ")
		b.WriteString("        unit default {\n")
		b.WriteString("            ipv4 {\n")
		b.WriteString("                dhcp {\n")
		b.WriteString("                    enabled true;\n")
		b.WriteString("                }\n")
		b.WriteString("            }\n")
		b.WriteString("        }\n")
		b.WriteString("    }\n")
	}

	if !hasEthernet {
		return ""
	}

	b.WriteString("}\n")
	b.WriteString("environment {\n")
	b.WriteString("    ssh {\n")
	b.WriteString("        enabled true;\n")
	b.WriteString("    }\n")
	b.WriteString("}\n")

	return b.String()
}

// safeEmitName returns true if name is safe to interpolate into config
// syntax. Rejects names containing characters that would break the
// config parser (braces, semicolons, whitespace, NUL).
func safeEmitName(name string) bool {
	if name == "" {
		return false
	}
	for i := range len(name) {
		switch name[i] {
		case '{', '}', ';', '\n', '\r', '\t', ' ', 0:
			return false
		}
	}
	return true
}
