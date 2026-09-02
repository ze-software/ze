// Design: docs/architecture/core-design.md -- Traffic control CLI show formatting

// Package cmd provides formatting functions for traffic control CLI output.
package cmd

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// FormatQoS formats an InterfaceQoS for human-readable CLI output.
func FormatQoS(qos traffic.InterfaceQoS) string {
	var b textbuf.Buffer
	b.Reset()
	b.Str("interface ").Str(qos.Interface).Str(" {\n")
	b.Str("  qdisc ").Str(qos.Qdisc.Type.String())
	if qos.Qdisc.DefaultClass != "" {
		b.Str(" default ").Str(qos.Qdisc.DefaultClass)
	}
	b.Str(" {\n")
	for _, c := range qos.Qdisc.Classes {
		formatClass(&b, &c)
	}
	b.Str("  }\n")
	b.Str("}\n")
	return b.String()
}

// formatQoSMap formats a map of interface QoS configs for CLI output.
func formatQoSMap(m map[string]traffic.InterfaceQoS) string {
	if len(m) == 0 {
		return "No traffic control configured."
	}
	var b textbuf.Buffer
	b.Reset()
	first := true
	for _, qos := range m {
		if !first {
			b.Byte('\n')
		}
		b.Str(FormatQoS(qos))
		first = false
	}
	return b.String()
}

func formatClass(b *textbuf.Buffer, c *traffic.TrafficClass) {
	b.Str("    class ").Str(c.Name).Str(" {\n")
	if c.Rate > 0 {
		b.Str("      rate ").Str(formatRate(c.Rate)).Str(";\n")
	}
	if c.Ceil > 0 {
		b.Str("      ceil ").Str(formatRate(c.Ceil)).Str(";\n")
	}
	b.Str("      priority ").Uint8(c.Priority).Str(";\n")
	for _, f := range c.Filters {
		b.Str("      match ").Str(f.Type.String()).Str(" 0x").Str(strconv.FormatUint(uint64(f.Value), 16)).Str(";\n")
	}
	b.Str("    }\n")
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
