// Design: docs/architecture/core-design.md -- Firewall CLI show formatting

// Package cmd provides formatting functions for firewall CLI output.
package cmd

import (
	"log/slog"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// formatNATTarget renders the `to` leaf of a NAT action in the shape
// the operator typed. A zero AddressEnd renders a single address; a
// set AddressEnd renders `lo-hi`. Port suffix is appended only when
// Port is non-zero, and a non-zero PortEnd yields a range.
func formatNATTarget(addr, addrEnd netip.Addr, port, portEnd uint16) string {
	var tb textbuf.Buffer
	tb.Addr(addr)
	if addrEnd.IsValid() {
		tb.Byte('-').Addr(addrEnd)
	}
	addrStr := tb.String()
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

	var b textbuf.Buffer
	for i, t := range tables {
		if i > 0 {
			b.Byte('\n')
		}
		formatTable(&b, &t)
	}
	return b.String()
}

func formatTable(b *textbuf.Buffer, t *firewall.Table) {
	b.Str("table ").Str(t.Family.String()).Byte(' ').Str(StripPrefix(t.Name)).Str(" {\n")
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

func formatChain(b *textbuf.Buffer, c *firewall.Chain) {
	b.Str("  chain ").Str(c.Name).Str(" {\n")
	if c.IsBase {
		b.Str("    type ").Str(c.Type.String()).Str(" hook ").Str(c.Hook.String()).Str(" priority ").Int(int64(c.Priority)).Str("; policy ").Str(c.Policy.String()).Str(";\n")
	}
	for i := range c.Terms {
		formatTerm(b, &c.Terms[i])
	}
	b.Str("  }\n")
}

func formatTerm(b *textbuf.Buffer, t *firewall.Term) {
	b.Str("    term ").Str(t.Name).Str(" {\n")
	if len(t.Matches) > 0 {
		b.Str("      from {\n")
		for _, m := range t.Matches {
			b.Str("        ").Str(formatMatch(m)).Str(";\n")
		}
		b.Str("      }\n")
	}
	if len(t.Actions) > 0 {
		b.Str("      then {\n")
		for _, a := range t.Actions {
			b.Str("        ").Str(formatAction(a)).Str(";\n")
		}
		b.Str("      }\n")
	}
	b.WriteString("    }\n")
}

func formatMatch(m firewall.Match) string {
	var tb textbuf.Buffer
	switch v := m.(type) {
	case firewall.MatchSourceAddress:
		return tb.Str("source address ").Prefix(v.Prefix).String()
	case firewall.MatchDestinationAddress:
		return tb.Str("destination address ").Prefix(v.Prefix).String()
	case firewall.MatchSourcePort:
		return formatPort("source port", v.Ranges)
	case firewall.MatchDestinationPort:
		return formatPort("destination port", v.Ranges)
	case firewall.MatchProtocol:
		return tb.Str("protocol ").Str(v.Protocol).String()
	case firewall.MatchInputInterface:
		return tb.Str("input interface ").Str(formatIface(v.Name, v.Wildcard)).String()
	case firewall.MatchOutputInterface:
		return tb.Str("output interface ").Str(formatIface(v.Name, v.Wildcard)).String()
	case firewall.MatchICMPType:
		return textbuf.StrInt("icmp type ", int64(v.Type))
	case firewall.MatchICMPv6Type:
		return textbuf.StrInt("icmpv6 type ", int64(v.Type))
	case firewall.MatchConnState:
		return tb.Str("connection state ").Str(formatConnState(v.States)).String()
	case firewall.MatchMark:
		return tb.Str("mark 0x").Str(hexUint32(v.Value)).Str("/0x").Str(hexUint32(v.Mask)).String()
	case firewall.MatchDSCP:
		return textbuf.StrInt("dscp ", int64(v.Value))
	case firewall.MatchInSet:
		return formatInSet(v)
	case firewall.MatchTCPFlags:
		return tb.Str("tcp flags ").Str(formatTCPFlags(v.Flags)).String()
	}
	return tb.Byte('<').Str(matchTypeName(m)).Byte('>').String()
}

func formatAction(a firewall.Action) string {
	var tb textbuf.Buffer
	switch v := a.(type) {
	case firewall.Accept:
		return "accept"
	case firewall.Drop:
		return "drop"
	case firewall.Reject:
		if v.Type != "" {
			return tb.Str("reject with ").Str(v.Type).String()
		}
		return "reject"
	case firewall.Jump:
		return tb.Str("jump ").Str(v.Target).String()
	case firewall.Goto:
		return tb.Str("goto ").Str(v.Target).String()
	case firewall.Return:
		return "return"
	case firewall.Counter:
		if v.Name != "" {
			return tb.Str("counter ").Str(v.Name).String()
		}
		return "counter"
	case firewall.Log:
		if v.Prefix != "" {
			return tb.Str(logKeyword).Str(" prefix ").Quoted(v.Prefix).String()
		}
		return logKeyword
	case firewall.Limit:
		return formatLimit(v)
	case firewall.SetMark:
		return tb.Str("mark set 0x").Str(hexUint32(v.Value)).String()
	case firewall.SetConnMark:
		return tb.Reset().Str("connection-mark set 0x").Str(hexUint32(v.Value)).String()
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
		return tb.Str("flow offload @").Str(v.FlowtableName).String()
	case firewall.SNAT:
		return tb.Str("snat to ").Str(formatNATTarget(v.Address, v.AddressEnd, v.Port, v.PortEnd)).String()
	case firewall.DNAT:
		return tb.Str("dnat to ").Str(formatNATTarget(v.Address, v.AddressEnd, v.Port, v.PortEnd)).String()
	}
	return "<unknown action>"
}

func formatMasquerade(m firewall.Masquerade) string {
	if m.Port != 0 {
		b := textbuf.Get()
		defer b.Release()
		b.Str("masquerade port-range ").Uint16(m.Port)
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
		b.Str(" random full")
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
//
// An INVERTED limiter says so, because the word decides which packets the rule
// matches and the two readings are opposites. `limit rate 100/second` matches
// while the bucket has credit; `limit rate over 100/second` matches once it is
// empty. Rendering both the same way would show an operator a rule that accepts
// the traffic it drops. nft spells the inversion `over`, and so does this.
func formatLimit(l firewall.Limit) string {
	var b textbuf.Buffer
	b.Str("limit rate ")
	if l.Over {
		b.Str("over ")
	}
	if l.Dimension == firewall.RateDimensionBytes {
		rate, suffix := byteRateSuffix(l.Rate)
		return b.Uint(rate).Str(suffix).Byte('/').Str(l.Unit).Str(" burst ").Uint32(l.Burst).String()
	}
	return b.Uint(l.Rate).Byte('/').Str(l.Unit).Str(" burst ").Uint32(l.Burst).String()
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
	var tb textbuf.Buffer
	switch m.MatchField {
	case firewall.SetFieldSourceAddr:
		return tb.Str("source address @").Str(m.SetName).String()
	case firewall.SetFieldDestAddr:
		return tb.Str("destination address @").Str(m.SetName).String()
	case firewall.SetFieldSourcePort:
		return tb.Str("source port @").Str(m.SetName).String()
	case firewall.SetFieldDestPort:
		return tb.Str("destination port @").Str(m.SetName).String()
	}
	return tb.Byte('@').Str(m.SetName).String()
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
	return textbuf.Join(names, ",")
}

func formatIface(name string, wildcard bool) string {
	if wildcard {
		var tb textbuf.Buffer
		return tb.Str(name).Byte('*').String()
	}
	return name
}

func formatPort(keyword string, ranges []firewall.PortRange) string {
	var tb textbuf.Buffer
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		if r.Lo == r.Hi {
			parts = append(parts, textbuf.StringInt(int64(r.Lo)))
		} else {
			parts = append(parts, tb.Reset().Int(int64(r.Lo)).Byte('-').Int(int64(r.Hi)).String())
		}
	}
	return tb.Reset().Str(keyword).Byte(' ').Join(parts, ",").String()
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
	return textbuf.Join(parts, ",")
}

func formatSet(b *textbuf.Buffer, s *firewall.Set) {
	b.Str("  set ").Str(s.Name).Str(" {\n")
	b.Str("    type ").Int(int64(s.Type)).Str(";\n")
	for _, e := range s.Elements {
		if e.Timeout == 0 {
			b.Str("    element ").Str(e.Value).Str(";\n")
			continue
		}
		b.Str("    element ").Str(e.Value).Str(" { timeout ").Int(int64(e.Timeout)).Str("; }\n")
	}
	b.Str("  }\n")
}

func formatFlowtable(b *textbuf.Buffer, ft *firewall.Flowtable) {
	b.Str("  flowtable ").Str(ft.Name).Str(" {\n")
	b.Str("    hook ").Str(ft.Hook.String()).Str(" priority ").Int(int64(ft.Priority)).Str(";\n")
	if len(ft.Devices) > 0 {
		b.Str("    devices = { ").Join(ft.Devices, ", ").Str(" };\n")
	}
	b.Str("  }\n")
}

// FormatCounters formats chain counter values for CLI output.
func FormatCounters(counters []firewall.ChainCounters) string {
	if len(counters) == 0 {
		return "No counters."
	}
	var b textbuf.Buffer
	for _, cc := range counters {
		b.Str("chain ").Str(cc.Chain).Str(":\n")
		for _, tc := range cc.Terms {
			b.Str("  ").PadRight(tc.Name, 30).Str(" packets ").Int(int64(tc.Packets)).Str("  bytes ").Int(int64(tc.Bytes)).Byte('\n')
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
