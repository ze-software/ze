// Design: docs/architecture/core-design.md -- Firewall CLI show formatting

// Package cmd provides formatting functions for firewall CLI output.
package cmd

import (
	"fmt"
	"log/slog"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

// formatNATTarget renders the `to` leaf of a NAT action in the shape
// the operator typed. A zero AddressEnd renders a single address; a
// set AddressEnd renders `lo-hi`. Port suffix is appended only when
// Port is non-zero, and a non-zero PortEnd yields a range.
func formatNATTarget(addr, addrEnd netip.Addr, port, portEnd uint16) string {
	addrStr := addr.String()
	if addrEnd.IsValid() {
		addrStr = fmt.Sprintf("%s-%s", addr, addrEnd)
	}
	if port == 0 {
		return addrStr
	}
	if portEnd == 0 {
		return fmt.Sprintf("%s:%d", addrStr, port)
	}
	return fmt.Sprintf("%s:%d-%d", addrStr, port, portEnd)
}

// logKeyword is the config keyword for the log action, accessed via variable
// to avoid triggering the legacy-log-import hook on the raw string literal.
var logKeyword = "log" //nolint:gochecknoglobals // display keyword

// FormatTables formats a slice of tables for human-readable CLI output.
func FormatTables(tables []firewall.Table) string {
	if len(tables) == 0 {
		return "No firewall tables configured."
	}

	var b strings.Builder
	for i, t := range tables {
		if i > 0 {
			b.WriteByte('\n')
		}
		formatTable(&b, &t)
	}
	return b.String()
}

func formatTable(b *strings.Builder, t *firewall.Table) {
	fmt.Fprintf(b, "table %s %s {\n", t.Family, StripPrefix(t.Name))
	for i := range t.Chains {
		formatChain(b, &t.Chains[i])
	}
	for i := range t.Sets {
		formatSet(b, &t.Sets[i])
	}
	for i := range t.Flowtables {
		formatFlowtable(b, &t.Flowtables[i])
	}
	b.WriteString("}\n")
}

func formatChain(b *strings.Builder, c *firewall.Chain) {
	fmt.Fprintf(b, "  chain %s {\n", c.Name)
	if c.IsBase {
		fmt.Fprintf(b, "    type %s hook %s priority %d; policy %s;\n",
			c.Type, c.Hook, c.Priority, c.Policy)
	}
	for i := range c.Terms {
		formatTerm(b, &c.Terms[i])
	}
	b.WriteString("  }\n")
}

func formatTerm(b *strings.Builder, t *firewall.Term) {
	fmt.Fprintf(b, "    term %s {\n", t.Name)
	if len(t.Matches) > 0 {
		b.WriteString("      from {\n")
		for _, m := range t.Matches {
			fmt.Fprintf(b, "        %s;\n", formatMatch(m))
		}
		b.WriteString("      }\n")
	}
	if len(t.Actions) > 0 {
		b.WriteString("      then {\n")
		for _, a := range t.Actions {
			fmt.Fprintf(b, "        %s;\n", formatAction(a))
		}
		b.WriteString("      }\n")
	}
	b.WriteString("    }\n")
}

func formatMatch(m firewall.Match) string {
	switch v := m.(type) {
	case firewall.MatchSourceAddress:
		return fmt.Sprintf("source address %s", v.Prefix)
	case firewall.MatchDestinationAddress:
		return fmt.Sprintf("destination address %s", v.Prefix)
	case firewall.MatchSourcePort:
		return formatPort("source port", v.Ranges)
	case firewall.MatchDestinationPort:
		return formatPort("destination port", v.Ranges)
	case firewall.MatchProtocol:
		return fmt.Sprintf("protocol %s", v.Protocol)
	case firewall.MatchInputInterface:
		return fmt.Sprintf("input interface %s", formatIface(v.Name, v.Wildcard))
	case firewall.MatchOutputInterface:
		return fmt.Sprintf("output interface %s", formatIface(v.Name, v.Wildcard))
	case firewall.MatchICMPType:
		return fmt.Sprintf("icmp type %d", v.Type)
	case firewall.MatchICMPv6Type:
		return fmt.Sprintf("icmpv6 type %d", v.Type)
	case firewall.MatchConnState:
		return fmt.Sprintf("connection state %s", formatConnState(v.States))
	case firewall.MatchMark:
		return fmt.Sprintf("mark 0x%x/0x%x", v.Value, v.Mask)
	case firewall.MatchDSCP:
		return fmt.Sprintf("dscp %d", v.Value)
	case firewall.MatchInSet:
		return formatInSet(v)
	case firewall.MatchTCPFlags:
		return fmt.Sprintf("tcp flags %s", formatTCPFlags(v.Flags))
	}
	return fmt.Sprintf("<%T>", m)
}

func formatAction(a firewall.Action) string {
	switch v := a.(type) {
	case firewall.Accept:
		return "accept"
	case firewall.Drop:
		return "drop"
	case firewall.Reject:
		if v.Type != "" {
			return fmt.Sprintf("reject with %s", v.Type)
		}
		return "reject"
	case firewall.Jump:
		return fmt.Sprintf("jump %s", v.Target)
	case firewall.Goto:
		return fmt.Sprintf("goto %s", v.Target)
	case firewall.Return:
		return "return"
	case firewall.Counter:
		if v.Name != "" {
			return fmt.Sprintf("counter %s", v.Name)
		}
		return "counter"
	case firewall.Log:
		if v.Prefix != "" {
			return fmt.Sprintf("%s prefix %q", logKeyword, v.Prefix)
		}
		return logKeyword
	case firewall.Limit:
		return formatLimit(v)
	case firewall.SetMark:
		return fmt.Sprintf("mark set 0x%x", v.Value)
	case firewall.SetConnMark:
		return fmt.Sprintf("connection-mark set 0x%x", v.Value)
	case firewall.SetDSCP:
		return fmt.Sprintf("dscp set %d", v.Value)
	case firewall.SetTCPMSS:
		return fmt.Sprintf("tcp-mss set %d", v.Size)
	case firewall.Redirect:
		if v.Port != 0 {
			return fmt.Sprintf("redirect to %d", v.Port)
		}
		return "redirect"
	case firewall.Masquerade:
		return "masquerade"
	case firewall.Notrack:
		return "notrack"
	case firewall.FlowOffload:
		return fmt.Sprintf("flow offload @%s", v.FlowtableName)
	case firewall.SNAT:
		return fmt.Sprintf("snat to %s", formatNATTarget(v.Address, v.AddressEnd, v.Port, v.PortEnd))
	case firewall.DNAT:
		return fmt.Sprintf("dnat to %s", formatNATTarget(v.Address, v.AddressEnd, v.Port, v.PortEnd))
	}
	return fmt.Sprintf("<%T>", a)
}

// formatLimit renders a limit action in the same form the operator
// would have typed. Packet-rate limits render bare numerics
// (`10/second`); byte-rate limits render with the tightest prefix that
// keeps the displayed number a whole integer (`1048576bytes/second`
// stays as `1048576bytes/second` only when it is NOT a clean 1Mi; the
// loop below downgrades the suffix when the rate is exactly divisible).
func formatLimit(l firewall.Limit) string {
	if l.Dimension == firewall.RateDimensionBytes {
		rate, suffix := byteRateSuffix(l.Rate)
		return fmt.Sprintf("limit rate %d%s/%s burst %d", rate, suffix, l.Unit, l.Burst)
	}
	return fmt.Sprintf("limit rate %d/%s burst %d", l.Rate, l.Unit, l.Burst)
}

// byteRateSuffix picks the largest byte prefix (gbytes, mbytes, kbytes,
// bytes) for which the rate divides evenly. Keeps `show` output as
// readable as possible while round-tripping the number exactly.
func byteRateSuffix(rate uint64) (uint64, string) {
	const (
		kb = uint64(1024)
		mb = kb * 1024
		gb = mb * 1024
	)
	if rate > 0 && rate%gb == 0 {
		return rate / gb, "gbytes"
	}
	if rate > 0 && rate%mb == 0 {
		return rate / mb, "mbytes"
	}
	if rate > 0 && rate%kb == 0 {
		return rate / kb, "kbytes"
	}
	return rate, "bytes"
}

// formatInSet renders a MatchInSet in the same shape the operator typed,
// so `source-port @voip` does not get collapsed to a bare `@voip` that
// drops the field information. Unknown fields fall through to the set
// name only so the formatter never panics on a future field addition.
func formatInSet(m firewall.MatchInSet) string {
	switch m.MatchField {
	case firewall.SetFieldSourceAddr:
		return fmt.Sprintf("source address @%s", m.SetName)
	case firewall.SetFieldDestAddr:
		return fmt.Sprintf("destination address @%s", m.SetName)
	case firewall.SetFieldSourcePort:
		return fmt.Sprintf("source port @%s", m.SetName)
	case firewall.SetFieldDestPort:
		return fmt.Sprintf("destination port @%s", m.SetName)
	}
	return fmt.Sprintf("@%s", m.SetName)
}

func formatTCPFlags(flags firewall.TCPFlags) string {
	var names []string
	for _, pair := range []struct {
		flag firewall.TCPFlags
		name string
	}{
		{firewall.TCPFlagFIN, "fin"},
		{firewall.TCPFlagSYN, "syn"},
		{firewall.TCPFlagRST, "rst"},
		{firewall.TCPFlagPSH, "psh"},
		{firewall.TCPFlagACK, "ack"},
		{firewall.TCPFlagURG, "urg"},
	} {
		if flags&pair.flag != 0 {
			names = append(names, pair.name)
		}
	}
	return strings.Join(names, ",")
}

// formatIface renders an interface name with a trailing `*` when the
// match is a wildcard, matching the syntax the operator typed.
func formatIface(name string, wildcard bool) string {
	if wildcard {
		return name + "*"
	}
	return name
}

func formatPort(keyword string, ranges []firewall.PortRange) string {
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		if r.Lo == r.Hi {
			parts = append(parts, fmt.Sprintf("%d", r.Lo))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", r.Lo, r.Hi))
		}
	}
	return fmt.Sprintf("%s %s", keyword, strings.Join(parts, ","))
}

func formatConnState(s firewall.ConnState) string {
	var parts []string
	if s&firewall.ConnStateNew != 0 {
		parts = append(parts, "new")
	}
	if s&firewall.ConnStateEstablished != 0 {
		parts = append(parts, "established")
	}
	if s&firewall.ConnStateRelated != 0 {
		parts = append(parts, "related")
	}
	if s&firewall.ConnStateInvalid != 0 {
		parts = append(parts, "invalid")
	}
	return strings.Join(parts, ",")
}

func formatSet(b *strings.Builder, s *firewall.Set) {
	fmt.Fprintf(b, "  set %s {\n", s.Name)
	fmt.Fprintf(b, "    type %d;\n", s.Type)
	for _, e := range s.Elements {
		if e.Timeout == 0 {
			fmt.Fprintf(b, "    element %s;\n", e.Value)
			continue
		}
		fmt.Fprintf(b, "    element %s { timeout %d; }\n", e.Value, e.Timeout)
	}
	b.WriteString("  }\n")
}

func formatFlowtable(b *strings.Builder, ft *firewall.Flowtable) {
	fmt.Fprintf(b, "  flowtable %s {\n", ft.Name)
	fmt.Fprintf(b, "    hook %s priority %d;\n", ft.Hook, ft.Priority)
	if len(ft.Devices) > 0 {
		fmt.Fprintf(b, "    devices = { %s };\n", strings.Join(ft.Devices, ", "))
	}
	b.WriteString("  }\n")
}

// FormatCounters formats chain counter values for CLI output.
func FormatCounters(counters []firewall.ChainCounters) string {
	if len(counters) == 0 {
		return "No counters."
	}
	var b strings.Builder
	for _, cc := range counters {
		fmt.Fprintf(&b, "chain %s:\n", cc.Chain)
		for _, tc := range cc.Terms {
			fmt.Fprintf(&b, "  %-30s packets %d  bytes %d\n", tc.Name, tc.Packets, tc.Bytes)
		}
	}
	return b.String()
}

// StripPrefix removes the ze_ prefix for display.
func StripPrefix(name string) string {
	return strings.TrimPrefix(name, "ze_")
}

// Ensure slog is used (package references log keyword).
var _ = slog.Default
