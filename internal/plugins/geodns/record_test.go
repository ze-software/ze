package geodns

import (
	"net/netip"
	"testing"
)

// VALIDATES: addrRecord tags an address as A (IPv4) or AAAA (IPv6) by family.
// PREVENTS: a v6 address being served as an A record (or a v4 as AAAA), which
// would make the daemon emit malformed answers.
func TestAddrRecordClassifiesByFamily(t *testing.T) {
	cases := []struct {
		addr string
		want recordKind
	}{
		{"1.2.3.4", kindA},
		{"82.219.4.10", kindA},
		{"2a02:b80::1", kindAAAA},
		{"::1", kindAAAA},
	}
	for _, tc := range cases {
		a := netip.MustParseAddr(tc.addr)
		r := addrRecord(300, a)
		if r.Kind != tc.want {
			t.Errorf("addrRecord(%s).Kind = %d, want %d", tc.addr, r.Kind, tc.want)
		}
		if r.TTL != 300 {
			t.Errorf("addrRecord(%s).TTL = %d, want 300", tc.addr, r.TTL)
		}
		if r.Addr != a {
			t.Errorf("addrRecord(%s).Addr = %v, want %v", tc.addr, r.Addr, a)
		}
	}
}

// VALIDATES: srvRecord carries priority/weight/port/target as SRV (RFC 2782).
// PREVENTS: SRV fields being dropped or reordered when building the answer.
func TestSrvRecord(t *testing.T) {
	r := srvRecord(300, 10, 20, 88, "kerberos.example.")
	if r.Kind != kindSRV {
		t.Fatalf("srvRecord Kind = %d, want kindSRV", r.Kind)
	}
	if r.Priority != 10 || r.Weight != 20 || r.Port != 88 {
		t.Errorf("srvRecord fields = %d/%d/%d, want 10/20/88", r.Priority, r.Weight, r.Port)
	}
	if r.Target != "kerberos.example." {
		t.Errorf("srvRecord Target = %q, want kerberos.example.", r.Target)
	}
}
