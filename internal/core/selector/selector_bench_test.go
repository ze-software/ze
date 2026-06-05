package selector

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"
)

func buildMultiIPSelector(n int) string {
	var b strings.Builder
	for i := range n {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "10.%d.%d.%d", i>>16, (i>>8)&0xFF, i&0xFF)
	}
	return b.String()
}

func BenchmarkSelectorParse(b *testing.B) {
	for _, n := range []int{4, 16, 64, 256} {
		s := buildMultiIPSelector(n)
		b.Run(fmt.Sprintf("IPs_%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				sel, err := Parse(s)
				if err != nil {
					b.Fatal(err)
				}
				_ = sel
			}
		})
	}
}

func BenchmarkSelectorMatches(b *testing.B) {
	probe := netip.MustParseAddr("10.0.0.5")
	miss := netip.MustParseAddr("192.168.0.1")

	for _, n := range []int{4, 16, 64, 256} {
		s := buildMultiIPSelector(n)
		sel, err := Parse(s)
		if err != nil {
			b.Fatal(err)
		}

		b.Run(fmt.Sprintf("hit_%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = sel.Matches(probe)
			}
		})
		b.Run(fmt.Sprintf("miss_%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = sel.Matches(miss)
			}
		})
	}
}

func BenchmarkSelectorParseKinds(b *testing.B) {
	cases := []struct {
		name  string
		input string
	}{
		{"all", "*"},
		{"addr", "10.0.0.1"},
		{"exclude", "!10.0.0.1"},
		{"name", "peer1"},
		{"asn", "as65000"},
		{"glob", "192.168.*.*"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				sel, err := Parse(c.input)
				if err != nil {
					b.Fatal(err)
				}
				_ = sel
			}
		})
	}
}
