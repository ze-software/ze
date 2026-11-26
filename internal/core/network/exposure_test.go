// Design: docs/guide/authentication.md -- the management-listener exposure rule
// Related: exposure.go -- ListenAddrIsLoopback, Exposure.Plan, Exposure.Refusals

package network

import (
	"slices"
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

// TestExposurePlan verifies the per-address split between allowed listeners
// and exposure breaches, and that Plan and Refusals report identical errors.
//
// VALIDATES: loopback listeners stay in Allowed in declared order while each
// non-loopback listener missing authentication or required TLS lands in
// Breaches (authentication before TLS) with its exact address and refusal,
// authenticated+TLS listeners allow all addresses as a cloned slice, and an
// empty address list without protection fails closed with Addr == "".
// PREVENTS: dropping safe loopback listeners alongside a breaching remote
// address, losing the breaching address or error order, or Plan diverging from
// Refusals.
func TestExposurePlan(t *testing.T) {
	assertPlanMatchesRefusals := func(t *testing.T, e Exposure, plan ExposurePlan) {
		t.Helper()
		refusals := e.Refusals()
		if len(plan.Breaches) != len(refusals) {
			t.Fatalf("len(plan.Breaches) = %d, len(Refusals()) = %d", len(plan.Breaches), len(refusals))
		}
		for i, breach := range plan.Breaches {
			if breach.Refusal.Error() != refusals[i].Error() {
				t.Errorf("breach[%d].Refusal = %q, want %q", i, breach.Refusal.Error(), refusals[i].Error())
			}
		}
	}

	t.Run("loopback-only without auth or TLS", func(t *testing.T) {
		e := Exposure{
			Service:    "gNMI",
			Addrs:      []string{"[::1]:9339", "127.0.0.1:9339"},
			RequireTLS: true,
		}
		plan := e.Plan()
		wantAllowed := []string{"[::1]:9339", "127.0.0.1:9339"}
		if !slices.Equal(plan.Allowed, wantAllowed) {
			t.Errorf("Allowed = %v, want %v", plan.Allowed, wantAllowed)
		}
		if len(plan.Breaches) != 0 {
			t.Errorf("Breaches = %d, want 0", len(plan.Breaches))
		}
		assertPlanMatchesRefusals(t, e, plan)
	})

	t.Run("mixed loopback and non-loopback without auth", func(t *testing.T) {
		e := Exposure{
			Service: "REST API",
			Addrs:   []string{"127.0.0.1:8080", "10.0.0.1:8080", "[::1]:8080"},
			AuthFix: "set environment api-server rest token <token>",
		}
		plan := e.Plan()
		wantAllowed := []string{"127.0.0.1:8080", "[::1]:8080"}
		if !slices.Equal(plan.Allowed, wantAllowed) {
			t.Errorf("Allowed = %v, want %v", plan.Allowed, wantAllowed)
		}
		if len(plan.Breaches) != 1 {
			t.Fatalf("Breaches = %d, want 1", len(plan.Breaches))
		}
		if plan.Breaches[0].Addr != "10.0.0.1:8080" {
			t.Errorf("Breaches[0].Addr = %q, want %q", plan.Breaches[0].Addr, "10.0.0.1:8080")
		}
		if want := `"10.0.0.1:8080" without authentication`; !strings.Contains(plan.Breaches[0].Refusal.Problem(), want) {
			t.Errorf("problem = %q, want it to contain %q", plan.Breaches[0].Refusal.Problem(), want)
		}
		if plan.Breaches[0].Refusal.Why() != exposureAuthWhy {
			t.Errorf("why = %q, want %q", plan.Breaches[0].Refusal.Why(), exposureAuthWhy)
		}
		if plan.Breaches[0].Refusal.Fix() != e.AuthFix {
			t.Errorf("fix = %q, want %q", plan.Breaches[0].Refusal.Fix(), e.AuthFix)
		}
		assertPlanMatchesRefusals(t, e, plan)
	})

	t.Run("mixed loopback and non-loopback with RequireTLS and neither auth nor TLS", func(t *testing.T) {
		e := Exposure{
			Service:    "gNMI",
			Addrs:      []string{"[::1]:9339", "[2001:db8::1]:9339", "127.0.0.1:9339"},
			RequireTLS: true,
			AuthFix:    "a",
			TLSFix:     "t",
		}
		plan := e.Plan()
		wantAllowed := []string{"[::1]:9339", "127.0.0.1:9339"}
		if !slices.Equal(plan.Allowed, wantAllowed) {
			t.Errorf("Allowed = %v, want %v", plan.Allowed, wantAllowed)
		}
		if len(plan.Breaches) != 2 {
			t.Fatalf("Breaches = %d, want 2 (authentication then TLS)", len(plan.Breaches))
		}
		if plan.Breaches[0].Addr != "[2001:db8::1]:9339" {
			t.Errorf("Breaches[0].Addr = %q, want %q", plan.Breaches[0].Addr, "[2001:db8::1]:9339")
		}
		if want := `"[2001:db8::1]:9339" without authentication`; !strings.Contains(plan.Breaches[0].Refusal.Problem(), want) {
			t.Errorf("Breaches[0].Problem = %q, want it to contain %q", plan.Breaches[0].Refusal.Problem(), want)
		}
		if plan.Breaches[0].Refusal.Fix() != "a" {
			t.Errorf("Breaches[0].Fix = %q, want %q", plan.Breaches[0].Refusal.Fix(), "a")
		}
		if plan.Breaches[1].Addr != "[2001:db8::1]:9339" {
			t.Errorf("Breaches[1].Addr = %q, want %q", plan.Breaches[1].Addr, "[2001:db8::1]:9339")
		}
		if want := `"[2001:db8::1]:9339" without TLS`; !strings.Contains(plan.Breaches[1].Refusal.Problem(), want) {
			t.Errorf("Breaches[1].Problem = %q, want it to contain %q", plan.Breaches[1].Refusal.Problem(), want)
		}
		if plan.Breaches[1].Refusal.Fix() != "t" {
			t.Errorf("Breaches[1].Fix = %q, want %q", plan.Breaches[1].Refusal.Fix(), "t")
		}
		assertPlanMatchesRefusals(t, e, plan)
	})

	t.Run("non-loopback authenticated with TLS", func(t *testing.T) {
		e := Exposure{
			Service:       "gNMI",
			Addrs:         []string{"0.0.0.0:9339", "[2001:db8::1]:9339"},
			Authenticated: true,
			RequireTLS:    true,
			TLSConfigured: true,
		}
		plan := e.Plan()
		wantAllowed := []string{"0.0.0.0:9339", "[2001:db8::1]:9339"}
		if !slices.Equal(plan.Allowed, wantAllowed) {
			t.Errorf("Allowed = %v, want %v", plan.Allowed, wantAllowed)
		}
		if len(plan.Breaches) != 0 {
			t.Errorf("Breaches = %d, want 0", len(plan.Breaches))
		}
		plan.Allowed[0] = "mutated"
		if e.Addrs[0] != "0.0.0.0:9339" {
			t.Errorf("mutating plan.Allowed mutated e.Addrs[0] to %q", e.Addrs[0])
		}
		assertPlanMatchesRefusals(t, e, plan)
	})

	t.Run("empty Addrs without auth", func(t *testing.T) {
		e := Exposure{Service: "gNMI"}
		plan := e.Plan()
		if plan.Allowed != nil {
			t.Errorf("Allowed = %v, want nil", plan.Allowed)
		}
		if len(plan.Breaches) != 1 {
			t.Fatalf("Breaches = %d, want 1", len(plan.Breaches))
		}
		if plan.Breaches[0].Addr != "" {
			t.Errorf("Breaches[0].Addr = %q, want empty", plan.Breaches[0].Addr)
		}
		if want := "without authentication and with no resolved listen address"; !strings.Contains(plan.Breaches[0].Refusal.Problem(), want) {
			t.Errorf("problem = %q, want it to contain %q", plan.Breaches[0].Refusal.Problem(), want)
		}
		if plan.Breaches[0].Refusal.Why() != exposureNoAddrWhy {
			t.Errorf("why = %q, want %q", plan.Breaches[0].Refusal.Why(), exposureNoAddrWhy)
		}
		if plan.Breaches[0].Refusal.Fix() != exposureNoAddrFix {
			t.Errorf("fix = %q, want %q", plan.Breaches[0].Refusal.Fix(), exposureNoAddrFix)
		}
		assertPlanMatchesRefusals(t, e, plan)
	})
}
