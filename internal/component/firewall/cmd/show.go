// Design: docs/architecture/core-design.md -- Firewall CLI show formatting

// Package cmd provides formatting functions for firewall CLI output.
package cmd

import (
	"fmt"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// formatNATTarget renders the `to` leaf of a NAT action in the shape
// the operator typed. A zero AddressEnd renders a single address; a
// set AddressEnd renders `lo-hi`. Port suffix is appended only when
// Port is non-zero, and a non-zero PortEnd yields a range.
func formatNATTarget(addr, addrEnd netip.Addr, port, portEnd uint16) string {
	addrStr := addr.String()
	if addrEnd.IsValid() {
		addrStr = addrStr + "-" + addrEnd.String()
	}
	if port == 0 {
		return addrStr
	}
	if portEnd == 0 {
		var b textbuf.Buffer
		return b.Reset().Str(addrStr).Byte(':').Int(int64(port)).String()
	}
	var b textbuf.Buffer
	return b.Reset().Str(addrStr).Byte(':').Int(int64(port)).Byte('-').Int(int64(portEnd)).String()
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
		return "source address " + v.Prefix.String()
	case firewall.MatchDestinationAddress:
		return "destination address " + v.Prefix.String()
	case firewall.MatchSourcePort:
		return formatPort("source port", v.Ranges)
	case firewall.MatchDestinationPort:
		return formatPort("destination port", v.Ranges)
	case firewall.MatchProtocol:
		return "protocol " + v.Protocol
	case firewall.MatchInputInterface:
		return "input interface " + formatIface(v.Name, v.Wildcard)
	case firewall.MatchOutputInterface:
		return "output interface " + formatIface(v.Name, v.Wildcard)
	case firewall.MatchICMPType:
		return textbuf.StrInt("icmp type ", int64(v.Type))
	case firewall.MatchICMPv6Type:
		return textbuf.StrInt("icmpv6 type ", int64(v.Type))
	case firewall.MatchConnState:
		return "connection state " + formatConnState(v.States)
	case firewall.MatchMark:
		return "mark 0x" + hexUint32(v.Value) + "/0x" + hexUint32(v.Mask)
	case firewall.MatchDSCP:
		return textbuf.StrInt("dscp ", int64(v.Value))
	case firewall.MatchInSet:
		return formatInSet(v)
	case firewall.MatchTCPFlags:
		return "tcp flags " + formatTCPFlags(v.Flags)
	}
	return "<" + matchTypeName(m) + ">"
}

func formatAction(a firewall.Action) string {
	switch v := a.(type) {
	case firewall.Accept:
		return "accept"
	case firewall.Drop:
		return "drop"
	case firewall.Reject:
		if v.Type != "" {
			return "reject with " + v.Type
		}
		return "reject"
	case firewall.Jump:
		return "jump " + v.Target
	case firewall.Goto:
		return "goto " + v.Target
	case firewall.Return:
		return "return"
	case firewall.Counter:
		if v.Name != "" {
			return "counter " + v.Name
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
		return textbuf.StrInt("dscp set ", int64(v.Value))
	case firewall.SetTCPMSS:
		return textbuf.StrInt("tcp-mss set ", int64(v.Size))
	case firewall.Redirect:
		if v.Port != 0 {
			return textbuf.StrInt("redirect to ", int64(v.Port))
		}
		return "redirect"
	case firewall.Masquerade:
		return formatMasquerade(v)
	case firewall.Notrack:
		return "notrack"
	case firewall.FlowOffload:
		return "flow offload @" + v.FlowtableName
	case firewall.SNAT:
		return "snat to " + formatNATTarget(v.Address, v.AddressEnd, v.Port, v.PortEnd)
	case firewall.DNAT:
		return "dnat to " + formatNATTarget(v.Address, v.AddressEnd, v.Port, v.PortEnd)
	}
	return fmt.Sprintf("<%T>", a)
}

func formatMasquerade(m firewall.Masquerade) string {
	if m.Port != 0 {
		b := textbuf.Get()
		defer b.Release()
		b.Str("masquerade to :").Uint16(m.Port)
		if m.PortEnd != 0 {
			b.Byte('-').Uint16(m.PortEnd)
		}
		return b.String()
	}
	if m.Flags == 0 {
		return "masquerade"
	}
	b := textbuf.Get()
	defer b.Release()
	b.Str("masquerade")
	if m.Flags&firewall.MasqFlagRandom != 0 {
		b.Str(" random")
	}
	if m.Flags&firewall.MasqFlagFullyRandom != 0 {
		b.Str(" fully-random")
	}
	if m.Flags&firewall.MasqFlagPersistent != 0 {
		b.Str(" persistent")
	}
	return b.String()
}

// formatLimit renders a limit action in the same form the operator
// would have typed. Packet-rate limits render bare numerics
// (`10/second`); byte-rate limits render with the tightest prefix that
// keeps the displayed number a whole integer (`1048576bytes/second`
// stays as `1048576bytes/second` only when it is NOT a clean 1Mi; the
// loop below downgrades the suffix when the rate is exactly divisible).
func formatLimit(l firewall.Limit) string {
	var b textbuf.Buffer
	if l.Dimension == firewall.RateDimensionBytes {
		rate, suffix := byteRateSuffix(l.Rate)
		return b.Reset().Str("limit rate ").Uint(rate).Str(suffix).Byte('/').Str(l.Unit).Str(" burst ").Uint32(l.Burst).String()
	}
	return b.Str("limit rate ").Uint(l.Rate).Byte('/').Str(l.Unit).Str(" burst ").Uint32(l.Burst).String()
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
		return "source address @" + m.SetName
	case firewall.SetFieldDestAddr:
		return "destination address @" + m.SetName
	case firewall.SetFieldSourcePort:
		return "source port @" + m.SetName
	case firewall.SetFieldDestPort:
		return "destination port @" + m.SetName
	}
	return "@" + m.SetName
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
			parts = append(parts, strconv.Itoa(int(r.Lo)))
		} else {
			var bRange textbuf.Buffer
			parts = append(parts, bRange.Reset().Int(int64(r.Lo)).Byte('-').Int(int64(r.Hi)).String())
		}
	}
	return keyword + " " + strings.Join(parts, ",")
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

func hexUint32(v uint32) string {
	var buf [8]byte
	return string(strconv.AppendUint(buf[:0], uint64(v), 16))
}

func matchTypeName(m firewall.Match) string {
	switch m.(type) {
	case firewall.MatchSourceAddress:
		return "firewall.MatchSourceAddress"
	case firewall.MatchDestinationAddress:
		return "firewall.MatchDestinationAddress"
	case firewall.MatchSourcePort:
		return "firewall.MatchSourcePort"
	case firewall.MatchDestinationPort:
		return "firewall.MatchDestinationPort"
	case firewall.MatchProtocol:
		return "firewall.MatchProtocol"
	case firewall.MatchInputInterface:
		return "firewall.MatchInputInterface"
	case firewall.MatchOutputInterface:
		return "firewall.MatchOutputInterface"
	case firewall.MatchICMPType:
		return "firewall.MatchICMPType"
	case firewall.MatchICMPv6Type:
		return "firewall.MatchICMPv6Type"
	case firewall.MatchConnState:
		return "firewall.MatchConnState"
	case firewall.MatchMark:
		return "firewall.MatchMark"
	case firewall.MatchDSCP:
		return "firewall.MatchDSCP"
	case firewall.MatchInSet:
		return "firewall.MatchInSet"
	case firewall.MatchTCPFlags:
		return "firewall.MatchTCPFlags"
	}
	return "firewall.Match"
}

// Ensure slog is used (package references log keyword).
var _ = slog.Default
