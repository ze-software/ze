// Design: docs/features/interfaces.md -- interface config emission from discovery
// Related: discover.go -- DiscoverInterfaces produces the input
// Related: iface.go -- DiscoveredInterface type

package iface

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config/secret"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
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

	var b strings.Builder
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
			fmt.Fprintf(&b, "    %s %s {\n", di.Type, di.Name)
			if di.MAC != "" {
				fmt.Fprintf(&b, "        mac-address %s;\n", di.MAC)
			}
			fmt.Fprintf(&b, "        os-name %s;\n", di.Name)
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
		fmt.Fprintf(&b, "    %s {\n", zeTypeLoopback)
		b.WriteString("    }\n")
	}

	b.WriteString("}\n")
	return b.String()
}

// emitWireguardBlock writes a wireguard list entry for a discovered netdev.
// If Wireguard is nil (backend could not read kernel state, or wgctrl
// returned an error), a skeleton block is emitted with only the os-name
// leaf so the operator can fill the rest in after the scan. When the spec
// is available, sensitive fields (private-key, peer preshared-key) are
// passed through secret.Encode so the output gets the $9$-encoded form,
// matching the sensitive-leaf pattern used for BGP MD5 passwords and
// other secrets in ze.
func emitWireguardBlock(b *strings.Builder, di *DiscoveredInterface) {
	fmt.Fprintf(b, "    wireguard %s {\n", di.Name)
	fmt.Fprintf(b, "        os-name %s;\n", di.Name)
	spec := di.Wireguard
	if spec == nil {
		b.WriteString("    }\n")
		return
	}
	if spec.ListenPortSet && spec.ListenPort != 0 {
		fmt.Fprintf(b, "        listen-port %d;\n", spec.ListenPort)
	}
	if spec.FirewallMark != 0 {
		fmt.Fprintf(b, "        fwmark %d;\n", spec.FirewallMark)
	}
	if encoded, err := secret.Encode(spec.PrivateKey.String()); err == nil {
		fmt.Fprintf(b, "        private-key \"%s\";\n", encoded)
	}
	for idx := range spec.Peers {
		p := &spec.Peers[idx]
		peerName := textbuf.StrInt("peer", int64(idx))
		fmt.Fprintf(b, "        peer %s {\n", peerName)
		fmt.Fprintf(b, "            public-key \"%s\";\n", p.PublicKey.String())
		if p.HasPresharedKey {
			if encoded, err := secret.Encode(p.PresharedKey.String()); err == nil {
				fmt.Fprintf(b, "            preshared-key \"%s\";\n", encoded)
			}
		}
		if p.EndpointIP != "" && p.EndpointPort != 0 {
			b.WriteString("            endpoint {\n")
			fmt.Fprintf(b, "                ip %s;\n", p.EndpointIP)
			fmt.Fprintf(b, "                port %d;\n", p.EndpointPort)
			b.WriteString("            }\n")
		}
		if len(p.AllowedIPs) > 0 {
			b.WriteString("            allowed-ips [")
			for _, cidr := range p.AllowedIPs {
				fmt.Fprintf(b, " %s", cidr)
			}
			b.WriteString(" ];\n")
		}
		if p.PersistentKeepalive != 0 {
			fmt.Fprintf(b, "            persistent-keepalive %d;\n", p.PersistentKeepalive)
		}
		b.WriteString("        }\n")
	}
	b.WriteString("    }\n")
}

// EmitSetConfig produces set-command format for discovered interfaces.
// Used by the bootstrap path where the template is already in set format.
func EmitSetConfig(discovered []DiscoveredInterface) string {
	if len(discovered) == 0 {
		return ""
	}

	var b strings.Builder
	for i := range discovered {
		di := &discovered[i]
		switch di.Type {
		case zeTypeLoopback:
			// Loopback is a regular container, not a presence container.
			// A bare "set interface loopback" with no child is invalid.
			// Skip it; the OS loopback is always present.
			continue
		case zeTypeEthernet, zeTypeBridge, zeTypeVeth, zeTypeDummy:
			if !safeEmitName(di.Name) {
				continue
			}
			if di.MAC != "" {
				fmt.Fprintf(&b, "set interface %s %s mac-address %s\n", di.Type, di.Name, di.MAC)
			}
			fmt.Fprintf(&b, "set interface %s %s os-name %s\n", di.Type, di.Name, di.Name)
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
func emitWireguardSet(b *strings.Builder, di *DiscoveredInterface) {
	prefix := "set interface wireguard " + di.Name
	fmt.Fprintf(b, "%s os-name %s\n", prefix, di.Name)
	spec := di.Wireguard
	if spec == nil {
		return
	}
	if spec.ListenPortSet && spec.ListenPort != 0 {
		fmt.Fprintf(b, "%s listen-port %d\n", prefix, spec.ListenPort)
	}
	if spec.FirewallMark != 0 {
		fmt.Fprintf(b, "%s fwmark %d\n", prefix, spec.FirewallMark)
	}
	if encoded, err := secret.Encode(spec.PrivateKey.String()); err == nil {
		fmt.Fprintf(b, "%s private-key \"%s\"\n", prefix, encoded)
	}
	for idx := range spec.Peers {
		p := &spec.Peers[idx]
		var bPrefix textbuf.Buffer
		peerPrefix := bPrefix.Reset().Str(prefix).Str(" peer peer").Int(int64(idx)).String()
		fmt.Fprintf(b, "%s public-key \"%s\"\n", peerPrefix, p.PublicKey.String())
		if p.HasPresharedKey {
			if encoded, err := secret.Encode(p.PresharedKey.String()); err == nil {
				fmt.Fprintf(b, "%s preshared-key \"%s\"\n", peerPrefix, encoded)
			}
		}
		if p.EndpointIP != "" && p.EndpointPort != 0 {
			fmt.Fprintf(b, "%s endpoint ip %s\n", peerPrefix, p.EndpointIP)
			fmt.Fprintf(b, "%s endpoint port %d\n", peerPrefix, p.EndpointPort)
		}
		if len(p.AllowedIPs) > 0 {
			for _, cidr := range p.AllowedIPs {
				fmt.Fprintf(b, "%s allowed-ips %s\n", peerPrefix, cidr)
			}
		}
		if p.PersistentKeepalive != 0 {
			fmt.Fprintf(b, "%s persistent-keepalive %d\n", peerPrefix, p.PersistentKeepalive)
		}
	}
}

func emitXFRMBlock(b *strings.Builder, di *DiscoveredInterface) {
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
	b.WriteString(textbuf.Uint32(info.IfID))
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

func emitXFRMSet(b *strings.Builder, di *DiscoveredInterface) {
	prefix := "set interface xfrm " + di.Name
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
	b.WriteString(textbuf.Uint32(info.IfID))
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
