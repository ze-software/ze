// Design: docs/guide/authentication.md -- the management-listener exposure rule
// Related: exposure.go -- ListenAddrIsLoopback, Exposure.Refusals

package network

import (
	"strings"
	"testing"
)

// TestListenAddrIsLoopback is the predicate's truth table, in the bare-host
// and host:port forms every caller hands it.
//
// VALIDATES: only a literal 127.0.0.0/8 or ::1 address (bracketed with a port,
// bare, or IPv4-mapped) is loopback; wildcards, routable addresses, names,
// a zoned link-local address and garbage are not (fail-closed).
// PREVENTS: an IPv6 loopback listener refused as non-loopback, and remote
// reachability passed off as loopback through a name or a wildcard.
func TestListenAddrIsLoopback(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1", true},
		{"127.0.0.1:9339", true},
		{"127.255.255.254:80", true},
		{"::1", true},
		{"[::1]:9339", true},
		{"::ffff:127.0.0.1", true},
		{"[::ffff:127.0.0.1]:9339", true},
		{"0.0.0.0", false},
		{"0.0.0.0:9339", false},
		{"::", false},
		{"[::]:9339", false},
		{":9339", false},
		{"10.0.0.1:443", false},
		{"2001:db8::1", false},
		{"[2001:db8::1]:9339", false},
		{"[fe80::1%eth0]:9339", false},
		{"localhost:8080", false},
		{"", false},
		{"garbage", false},
		// The unbracketed join this predicate was once handed for [::1]:9339 is
		// a different, non-loopback IPv6 address (::1:9339), so it answers
		// false. Callers MUST bracket (ServerEndpoint.Listen).
		{"::1:9339", false},
	}
	for _, tt := range cases {
		if got := ListenAddrIsLoopback(tt.addr); got != tt.want {
			t.Errorf("ListenAddrIsLoopback(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

// TestExposureRefusals covers the rule: loopback passes, each missing
// protection on a non-loopback address is one refusal naming the address as
// given (an IPv6 one bracketed), and an empty address set is refused.
func TestExposureRefusals(t *testing.T) {
	loopback := Exposure{Service: "gNMI", Addrs: []string{"[::1]:9339", "127.0.0.1:9339"}, RequireTLS: true}
	if refusals := loopback.Refusals(); len(refusals) != 0 {
		t.Fatalf("loopback without auth or TLS: refusals = %d, want 0", len(refusals))
	}

	open := Exposure{Service: "gNMI", Addrs: []string{"[2001:db8::1]:9339"}, RequireTLS: true, AuthFix: "a", TLSFix: "t"}
	refusals := open.Refusals()
	if len(refusals) != 2 {
		t.Fatalf("refusals = %d, want 2 (authentication and TLS)", len(refusals))
	}
	if want := `"[2001:db8::1]:9339" without authentication`; !strings.Contains(refusals[0].Problem(), want) {
		t.Errorf("problem = %q, want it to contain %q", refusals[0].Problem(), want)
	}
	if want := `"[2001:db8::1]:9339" without TLS`; !strings.Contains(refusals[1].Problem(), want) {
		t.Errorf("problem = %q, want it to contain %q", refusals[1].Problem(), want)
	}
	if refusals[0].Fix() != "a" || refusals[1].Fix() != "t" {
		t.Errorf("fixes = %q, %q, want the declared ones", refusals[0].Fix(), refusals[1].Fix())
	}

	secured := Exposure{Service: "gNMI", Addrs: []string{"0.0.0.0:9339"}, Authenticated: true, RequireTLS: true, TLSConfigured: true}
	if refusals := secured.Refusals(); len(refusals) != 0 {
		t.Fatalf("authenticated with TLS: refusals = %d, want 0", len(refusals))
	}

	noAddr := Exposure{Service: "gNMI"}
	if refusals := noAddr.Refusals(); len(refusals) != 1 {
		t.Fatalf("no resolved address: refusals = %d, want 1", len(refusals))
	}
}
