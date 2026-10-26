package dataplane

import (
	"net"
	"testing"
)

func overlapTestNet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", cidr, err)
	}
	return n
}

// TestPolicySelectorsOverlapExported proves the exported predicate the IKE engine uses
// for its cross-peer tunnel check (RFC 3948 Section 5.1) answers exactly what the
// unexported selectorsOverlap answers over policyKey, on identical, overlapping,
// disjoint, different-if_id, different-protocol and port inputs, and that the verdict is
// symmetric. Method: a table of SPParams pairs, each judged both ways and against the
// unexported predicate.
func TestPolicySelectorsOverlapExported(t *testing.T) {
	base := func(src, dst string) SPParams {
		return SPParams{
			Src: overlapTestNet(t, src), Dst: overlapTestNet(t, dst),
			Dir: SADirOut, Mode: ModeTunnel, IfID: 1, Owner: "a",
		}
	}
	cases := []struct {
		name string
		a, b SPParams
		want bool
	}{
		{"identical", base("10.0.0.0/24", "10.1.2.3/32"), base("10.0.0.0/24", "10.1.2.3/32"), true},
		{"host inside wildcard", base("0.0.0.0/0", "10.1.2.3/32"), base("192.0.2.10/32", "10.1.2.3/32"), true},
		{"ipv6 host inside wildcard", base("::/0", "2001:db8::1/128"), base("2001:db8:1::/48", "::/0"), true},
		{"adjacent hosts", base("0.0.0.0/0", "10.1.2.3/32"), base("0.0.0.0/0", "10.1.2.4/32"), false},
		{"disjoint local sides", base("192.0.2.10/32", "10.1.2.3/32"), base("198.51.100.0/24", "10.1.2.3/32"), false},
		{"different if_id", base("0.0.0.0/0", "0.0.0.0/0"), func() SPParams {
			p := base("0.0.0.0/0", "0.0.0.0/0")
			p.IfID = 2
			return p
		}(), false},
		{"different protocol", func() SPParams {
			p := base("0.0.0.0/0", "10.1.2.3/32")
			p.UpperProto = 6
			return p
		}(), func() SPParams {
			p := base("0.0.0.0/0", "10.1.2.3/32")
			p.UpperProto = 17
			return p
		}(), false},
		{"port 65535 versus any", func() SPParams {
			p := base("0.0.0.0/0", "10.1.2.3/32")
			p.DstPort = ExactPortMatch(65535)
			return p
		}(), base("0.0.0.0/0", "10.1.2.3/32"), true},
		{"port 179 versus 180", func() SPParams {
			p := base("0.0.0.0/0", "10.1.2.3/32")
			p.DstPort = ExactPortMatch(179)
			return p
		}(), func() SPParams {
			p := base("0.0.0.0/0", "10.1.2.3/32")
			p.DstPort = ExactPortMatch(180)
			return p
		}(), false},
		{"owner and priority do not count", base("0.0.0.0/0", "10.1.2.3/32"), func() SPParams {
			p := base("0.0.0.0/0", "10.1.2.3/32")
			p.Owner = "b"
			p.Priority = 1500
			return p
		}(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PolicySelectorsOverlap(tc.a, tc.b); got != tc.want {
				t.Fatalf("PolicySelectorsOverlap(a, b) = %v, want %v", got, tc.want)
			}
			if got := PolicySelectorsOverlap(tc.b, tc.a); got != tc.want {
				t.Fatalf("PolicySelectorsOverlap(b, a) = %v, want %v (not symmetric)", got, tc.want)
			}
			if got := selectorsOverlap(policyKey(tc.a), policyKey(tc.b)); got != tc.want {
				t.Fatalf("selectorsOverlap = %v, want %v", got, tc.want)
			}
		})
	}
}
