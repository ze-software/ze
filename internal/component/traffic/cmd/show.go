// Design: docs/architecture/core-design.md -- Traffic control CLI show formatting

// Package cmd provides formatting functions for traffic control CLI output.
package cmd

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

// FormatQoS formats an InterfaceQoS for human-readable CLI output.
func FormatQoS(qos traffic.InterfaceQoS) string {
	var b strings.Builder
	fmt.Fprintf(&b, "interface %s {\n", qos.Interface)
	fmt.Fprintf(&b, "  qdisc %s", qos.Qdisc.Type)
	if qos.Qdisc.DefaultClass != "" {
		fmt.Fprintf(&b, " default %s", qos.Qdisc.DefaultClass)
	}
	b.WriteString(" {\n")
	for _, c := range qos.Qdisc.Classes {
		formatClass(&b, &c)
	}
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
}

// FormatQoSMap formats a map of interface QoS configs for CLI output.
func FormatQoSMap(m map[string]traffic.InterfaceQoS) string {
	if len(m) == 0 {
		return "No traffic control configured."
	}
	var b strings.Builder
	first := true
	for _, qos := range m {
		if !first {
			b.WriteByte('\n')
		}
		b.WriteString(FormatQoS(qos))
		first = false
	}
	return b.String()
}

func formatClass(b *strings.Builder, c *traffic.TrafficClass) {
	fmt.Fprintf(b, "    class %s {\n", c.Name)
	if c.Rate > 0 {
		fmt.Fprintf(b, "      rate %s;\n", formatRate(c.Rate))
	}
	if c.Ceil > 0 {
		fmt.Fprintf(b, "      ceil %s;\n", formatRate(c.Ceil))
	}
	fmt.Fprintf(b, "      priority %d;\n", c.Priority)
	for _, f := range c.Filters {
		fmt.Fprintf(b, "      match %s 0x%x;\n", f.Type, f.Value)
	}
	b.WriteString("    }\n")
}

func formatRate(bps uint64) string {
	switch {
	case bps >= 1_000_000_000 && bps%1_000_000_000 == 0:
		return fmt.Sprintf("%dgbit", bps/1_000_000_000)
	case bps >= 1_000_000 && bps%1_000_000 == 0:
		return fmt.Sprintf("%dmbit", bps/1_000_000)
	case bps >= 1_000 && bps%1_000 == 0:
		return fmt.Sprintf("%dkbit", bps/1_000)
	}
	return fmt.Sprintf("%dbit", bps)
}
