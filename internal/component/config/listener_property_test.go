// Property tests for listener conflict detection (spec followup-test-infra L92).
//
// Engine: stdlib testing/quick (no third-party dependency; see
// ai/rules/go-standards.md Dependencies). A fixed RNG seed keeps CI runs
// deterministic (R-1: property tests must not flake under CI time limits).
//
// The property set below asserts ONLY invariants that provably hold for
// conflicts()/ipsConflict()/FindListenerConflict (listener.go:289-328):
// symmetry, port independence, wildcard dominance, cross-family independence,
// protocol independence, and the FindListenerConflict<->pairwise-predicate
// equivalence. Transitivity is deliberately absent and is proven FALSE by a
// concrete counterexample below (R-2): a wildcard 0.0.0.0:80 conflicts with
// both 1.1.1.1:80 and 2.2.2.2:80, which do not conflict with each other.
package config

import (
	"math/rand"
	"net"
	"reflect"
	"testing"
	"testing/quick"
)

// propertyQuickConfig returns a deterministic quick.Config: a fixed seed and a
// bounded iteration count so the property tests are reproducible and cannot
// time out in CI.
func propertyQuickConfig(seed int64) *quick.Config {
	return &quick.Config{
		MaxCount: 2000,
		Rand:     rand.New(rand.NewSource(seed)), //nolint:gosec // deterministic test seed, not crypto
	}
}

// genEndpoint is a testing/quick generator for ListenerEndpoint that produces
// realistic, collision-prone endpoints: a small port set and a small IP pool
// per family force conflicts to actually occur (random 16-byte IPs would almost
// never collide, making the properties vacuous).
type genEndpoint struct {
	ListenerEndpoint
}

// v4Pool and v6Pool are small so generated endpoints collide often. The first
// entry of each is the family wildcard so wildcard dominance is exercised.
var (
	v4Pool = []net.IP{net.IPv4zero, net.ParseIP("1.1.1.1"), net.ParseIP("2.2.2.2"), net.ParseIP("3.3.3.3")}
	v6Pool = []net.IP{net.IPv6zero, net.ParseIP("2001:db8::1"), net.ParseIP("2001:db8::2"), net.ParseIP("fe80::1")}
	portP  = []uint16{80, 179, 443}
	protoP = []string{ProtocolTCP, ProtocolUDP, ""} // "" is treated as TCP by conflicts()
)

// Generate implements quick.Generator.
func (genEndpoint) Generate(r *rand.Rand, _ int) reflect.Value {
	return reflect.ValueOf(genEndpoint{randEndpoint(r)})
}

func randEndpoint(r *rand.Rand) ListenerEndpoint {
	pool := v4Pool
	if r.Intn(2) == 0 {
		pool = v6Pool
	}
	return ListenerEndpoint{
		Service:  "svc",
		Protocol: protoP[r.Intn(len(protoP))],
		IP:       pool[r.Intn(len(pool))],
		Port:     portP[r.Intn(len(portP))],
	}
}

// TestListenerConflictProperties is the A-2 pilot: it proves stdlib
// testing/quick is expressive enough for ze's property targets. Each sub-test
// is an independent quick.Check over generated endpoints.
//
// VALIDATES: AC-1 / L92 -- symmetry, port/family/protocol independence,
// wildcard dominance, and FindListenerConflict<->pairwise-predicate equivalence.
// PREVENTS: a conflict-detection change that breaks an invariant (e.g. making
// conflict asymmetric or letting a wildcard miss a same-family endpoint).
func TestListenerConflictProperties(t *testing.T) {
	t.Parallel()

	// Property 1 -- Symmetry: conflicts(a,b) == conflicts(b,a).
	t.Run("symmetry", func(t *testing.T) {
		t.Parallel()
		f := func(a, b genEndpoint) bool {
			return conflicts(a.ListenerEndpoint, b.ListenerEndpoint) ==
				conflicts(b.ListenerEndpoint, a.ListenerEndpoint)
		}
		if err := quick.Check(f, propertyQuickConfig(1)); err != nil {
			t.Fatalf("symmetry violated: %v", err)
		}
	})

	// Property 2 -- Port independence: endpoints on different ports never conflict.
	t.Run("port_independence", func(t *testing.T) {
		t.Parallel()
		f := func(a, b genEndpoint) bool {
			if a.Port == b.Port {
				return true // vacuously satisfied; different-port case is the claim
			}
			return !conflicts(a.ListenerEndpoint, b.ListenerEndpoint)
		}
		if err := quick.Check(f, propertyQuickConfig(2)); err != nil {
			t.Fatalf("port independence violated: %v", err)
		}
	})

	// Property 3 -- Wildcard dominance: the family wildcard conflicts with every
	// same-family endpoint sharing proto+port.
	t.Run("wildcard_dominance", func(t *testing.T) {
		t.Parallel()
		f := func(a genEndpoint) bool {
			wildIP := net.IPv4zero
			if a.IP.To4() == nil {
				wildIP = net.IPv6zero
			}
			wild := ListenerEndpoint{Service: "wild", Protocol: a.Protocol, IP: wildIP, Port: a.Port}
			return conflicts(wild, a.ListenerEndpoint)
		}
		if err := quick.Check(f, propertyQuickConfig(3)); err != nil {
			t.Fatalf("wildcard dominance violated: %v", err)
		}
	})

	// Property 4 -- Cross-family independence: an IPv4 and an IPv6 endpoint on
	// the same proto+port never conflict.
	t.Run("cross_family_independence", func(t *testing.T) {
		t.Parallel()
		f := func(a genEndpoint) bool {
			v4 := ListenerEndpoint{Service: "v4", Protocol: a.Protocol, IP: net.ParseIP("1.1.1.1"), Port: a.Port}
			v6 := ListenerEndpoint{Service: "v6", Protocol: a.Protocol, IP: net.ParseIP("2001:db8::1"), Port: a.Port}
			return !conflicts(v4, v6)
		}
		if err := quick.Check(f, propertyQuickConfig(4)); err != nil {
			t.Fatalf("cross-family independence violated: %v", err)
		}
	})

	// Property 5 -- Protocol independence: same ip+port on TCP vs UDP never conflict.
	t.Run("protocol_independence", func(t *testing.T) {
		t.Parallel()
		f := func(a genEndpoint) bool {
			tcp := ListenerEndpoint{Service: "t", Protocol: ProtocolTCP, IP: a.IP, Port: a.Port}
			udp := ListenerEndpoint{Service: "u", Protocol: ProtocolUDP, IP: a.IP, Port: a.Port}
			return !conflicts(tcp, udp)
		}
		if err := quick.Check(f, propertyQuickConfig(5)); err != nil {
			t.Fatalf("protocol independence violated: %v", err)
		}
	})

	// Property 6 -- FindListenerConflict equivalence: it returns a non-nil
	// conflict iff some pair in the set pairwise-conflicts.
	t.Run("find_matches_pairwise", func(t *testing.T) {
		t.Parallel()
		f := func(gs []genEndpoint) bool {
			eps := make([]ListenerEndpoint, len(gs))
			for i, g := range gs {
				eps[i] = g.ListenerEndpoint
			}
			want := anyPairwiseConflict(eps)
			got := FindListenerConflict(eps) != nil
			return want == got
		}
		if err := quick.Check(f, propertyQuickConfig(6)); err != nil {
			t.Fatalf("FindListenerConflict/pairwise equivalence violated: %v", err)
		}
	})
}

// anyPairwiseConflict is the brute-force oracle FindListenerConflict is checked
// against: true iff any distinct pair conflicts.
func anyPairwiseConflict(eps []ListenerEndpoint) bool {
	for i := range eps {
		for j := i + 1; j < len(eps); j++ {
			if conflicts(eps[i], eps[j]) {
				return true
			}
		}
	}
	return false
}

// TestListenerConflictNotTransitive pins the design correction: conflict is NOT
// transitive. A wildcard conflicts with two distinct unicast addresses that do
// not conflict with each other. This test exists so a future "fix" that asserts
// transitivity fails loudly (R-2).
//
// VALIDATES: AC-1 / L92 -- the redefined property set excludes transitivity.
// PREVENTS: a future change re-introducing a (false) transitivity assertion and
// forcing a wrong production change to conflict detection.
func TestListenerConflictNotTransitive(t *testing.T) {
	t.Parallel()

	wild := ListenerEndpoint{Service: "wild", Protocol: ProtocolTCP, IP: net.IPv4zero, Port: 80}
	a := ListenerEndpoint{Service: "a", Protocol: ProtocolTCP, IP: net.ParseIP("1.1.1.1"), Port: 80}
	b := ListenerEndpoint{Service: "b", Protocol: ProtocolTCP, IP: net.ParseIP("2.2.2.2"), Port: 80}

	if !conflicts(wild, a) {
		t.Fatal("expected wildcard to conflict with 1.1.1.1:80")
	}
	if !conflicts(wild, b) {
		t.Fatal("expected wildcard to conflict with 2.2.2.2:80")
	}
	if conflicts(a, b) {
		t.Fatal("transitivity must NOT hold: 1.1.1.1:80 and 2.2.2.2:80 must not conflict")
	}
}
