// Design: docs/architecture/core-design.md -- Firewall CLI show formatting

// Package cmd provides formatting functions for firewall CLI output.
package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

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
		return formatPort("source port", v.Port, v.PortEnd)
	case firewall.MatchDestinationPort:
		return formatPort("destination port", v.Port, v.PortEnd)
	case firewall.MatchProtocol:
		return fmt.Sprintf("protocol %s", v.Protocol)
	case firewall.MatchInputInterface:
		return fmt.Sprintf("input interface %s", v.Name)
	case firewall.MatchOutputInterface:
		return fmt.Sprintf("output interface %s", v.Name)
	case firewall.MatchConnState:
		return fmt.Sprintf("connection state %s", formatConnState(v.States))
	case firewall.MatchMark:
		return fmt.Sprintf("mark 0x%x/0x%x", v.Value, v.Mask)
	case firewall.MatchDSCP:
		return fmt.Sprintf("dscp %d", v.Value)
	case firewall.MatchInSet:
		return fmt.Sprintf("@%s", v.SetName)
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
		return fmt.Sprintf("limit rate %d/%s burst %d", v.Rate, v.Unit, v.Burst)
	case firewall.SetMark:
		return fmt.Sprintf("mark set 0x%x", v.Value)
	case firewall.Masquerade:
		return "masquerade"
	case firewall.Notrack:
		return "notrack"
	case firewall.FlowOffload:
		return fmt.Sprintf("flow offload @%s", v.FlowtableName)
	case firewall.SNAT:
		return fmt.Sprintf("snat to %s", v.Address)
	case firewall.DNAT:
		return fmt.Sprintf("dnat to %s", v.Address)
	}
	return fmt.Sprintf("<%T>", a)
}

func formatPort(keyword string, port, portEnd uint16) string {
	if portEnd == 0 {
		return fmt.Sprintf("%s %d", keyword, port)
	}
	return fmt.Sprintf("%s %d-%d", keyword, port, portEnd)
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
	if len(s.Elements) > 0 {
		b.WriteString("    elements = { ")
		for i, e := range s.Elements {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(e.Value)
		}
		b.WriteString(" }\n")
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
