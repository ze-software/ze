// Design: docs/architecture/core-design.md -- Traffic control CLI show formatting

// Package cmd provides formatting functions for traffic control CLI output.
package cmd

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// FormatQoS formats an InterfaceQoS for human-readable CLI output.
func FormatQoS(qos traffic.InterfaceQoS) string {
	var b strings.Builder
	fmt.Fprintf(&b, "interface %s {\n", qos.Interface) //nolint:errcheck // buffer output
	fmt.Fprintf(&b, "  qdisc %s", qos.Qdisc.Type)      //nolint:errcheck // buffer output
	if qos.Qdisc.DefaultClass != "" {
		fmt.Fprintf(&b, " default %s", qos.Qdisc.DefaultClass) //nolint:errcheck // buffer output
	}
	b.WriteString(" {\n")
	for _, c := range qos.Qdisc.Classes {
		formatClass(&b, &c)
	}
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
}

// formatQoSMap formats a map of interface QoS configs for CLI output.
func formatQoSMap(m map[string]traffic.InterfaceQoS) string {
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
	fmt.Fprintf(b, "    class %s {\n", c.Name) //nolint:errcheck // output
	if c.Rate > 0 {
		fmt.Fprintf(b, "      rate %s;\n", formatRate(c.Rate)) //nolint:errcheck // output
	}
	if c.Ceil > 0 {
		fmt.Fprintf(b, "      ceil %s;\n", formatRate(c.Ceil)) //nolint:errcheck // output
	}
	fmt.Fprintf(b, "      priority %d;\n", c.Priority) //nolint:errcheck // output
	for _, f := range c.Filters {
		fmt.Fprintf(b, "      match %s 0x%x;\n", f.Type, f.Value) //nolint:errcheck // output
	}
	b.WriteString("    }\n")
}

func formatRate(bps uint64) string {
	switch {
	case bps >= 1_000_000_000 && bps%1_000_000_000 == 0:
		return textbuf.UintStr(bps/1_000_000_000, "gbit")
	case bps >= 1_000_000 && bps%1_000_000 == 0:
		return textbuf.UintStr(bps/1_000_000, "mbit")
	case bps >= 1_000 && bps%1_000 == 0:
		return textbuf.UintStr(bps/1_000, "kbit")
	}
	return textbuf.UintStr(bps, "bit")
}
