//go:build linux

package ifacenetlink

import (
	"bytes"
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

// VALIDATES: the strict rule restricts the fib lookup to the arrival
// interface and the loose rule does not, and both drop only when no route
// answers. Swapping the flag turns strict into loose silently.
// METHOD: read the fib and cmp expressions of each mode's rule.
func TestRPFDropExprs(t *testing.T) {
	set := &nftables.Set{Name: rpfStrictSet}
	for _, tc := range []struct {
		name   string
		strict bool
	}{{"strict", true}, {"loose", false}} {
		t.Run(tc.name, func(t *testing.T) {
			exprs := rpfDropExprs(set, tc.strict)
			if len(exprs) != 6 {
				t.Fatalf("rule has %d expressions, want 6", len(exprs))
			}
			meta, ok := exprs[0].(*expr.Meta)
			if !ok || meta.Key != expr.MetaKeyIIFNAME {
				t.Fatalf("expression 0 = %#v, want meta iifname", exprs[0])
			}
			lookup, ok := exprs[1].(*expr.Lookup)
			if !ok || lookup.SetName != rpfStrictSet || lookup.Invert {
				t.Fatalf("expression 1 = %#v, want lookup of set %q", exprs[1], rpfStrictSet)
			}
			fib, ok := exprs[2].(*expr.Fib)
			if !ok {
				t.Fatalf("expression 2 = %#v, want fib", exprs[2])
			}
			if !fib.FlagSADDR || !fib.ResultOIF || !fib.FlagPRESENT || fib.FlagDADDR {
				t.Fatalf("fib = %#v, want saddr lookup with oif present", fib)
			}
			if fib.FlagIIF != tc.strict {
				t.Fatalf("fib iif flag = %v, want %v", fib.FlagIIF, tc.strict)
			}
			cmp, ok := exprs[3].(*expr.Cmp)
			if !ok || cmp.Op != expr.CmpOpEq || string(cmp.Data) != "\x00\x00\x00\x00" {
				t.Fatalf("expression 3 = %#v, want cmp eq 0 (oif missing)", exprs[3])
			}
			verdict, ok := exprs[5].(*expr.Verdict)
			if !ok || verdict.Kind != expr.VerdictDrop {
				t.Fatalf("expression 5 = %#v, want drop", exprs[5])
			}
		})
	}
}

// VALIDATES: the exemptions accept exactly fe80::/10 and ::, the sources
// neighbor discovery and DHCPv6 use, and nothing wider.
func TestRPFExemptRules(t *testing.T) {
	rules := rpfExemptRules()
	if len(rules) != 2 {
		t.Fatalf("%d exemption rules, want 2", len(rules))
	}
	linkLocal := rules[0]
	payload, ok := linkLocal[0].(*expr.Payload)
	if !ok || payload.Offset != 8 || payload.Len != 2 || payload.Base != expr.PayloadBaseNetworkHeader {
		t.Fatalf("link-local payload = %#v, want 2 bytes of the source address", linkLocal[0])
	}
	mask, ok := linkLocal[1].(*expr.Bitwise)
	if !ok || string(mask.Mask) != "\xff\xc0" {
		t.Fatalf("link-local mask = %#v, want ffc0 (/10)", linkLocal[1])
	}
	cmp, ok := linkLocal[2].(*expr.Cmp)
	if !ok || string(cmp.Data) != "\xfe\x80" {
		t.Fatalf("link-local cmp = %#v, want fe80", linkLocal[2])
	}
	unspecified := rules[1]
	payload, ok = unspecified[0].(*expr.Payload)
	if !ok || payload.Offset != 8 || payload.Len != 16 {
		t.Fatalf("unspecified payload = %#v, want the 16-byte source address", unspecified[0])
	}
	cmp, ok = unspecified[1].(*expr.Cmp)
	if !ok || !bytes.Equal(cmp.Data, make([]byte, 16)) {
		t.Fatalf("unspecified cmp = %#v, want ::", unspecified[1])
	}
	for i, rule := range rules {
		verdict, ok := rule[len(rule)-1].(*expr.Verdict)
		if !ok || verdict.Kind != expr.VerdictAccept {
			t.Fatalf("exemption %d ends in %#v, want accept", i, rule[len(rule)-1])
		}
	}
}

// VALIDATES: an ifname set key is IFNAMSIZ bytes, NUL padded, which is what
// the kernel compares the arrival device name against.
func TestIfnameKey(t *testing.T) {
	key := ifnameKey("eth0.100")
	if len(key) != 16 {
		t.Fatalf("key length %d, want 16", len(key))
	}
	if string(key[:8]) != "eth0.100" || key[8] != 0 || key[15] != 0 {
		t.Fatalf("key = %q, want eth0.100 NUL padded", key)
	}
}
