package rtproto

import "testing"

func TestRouteProtocolsAreDistinct(t *testing.T) {
	seen := map[int]string{}
	protocols := []struct {
		protocol int
		name     string
	}{
		{FIBKernel, "fib-kernel"},
		{Static, "static"},
		{PolicyRoute, "policy-route"},
		{int(Iface), "iface"},
		{int(GTSM), "gtsm"},
		{int(IPsecSourceGuard), "ipsec-source-guard"},
	}
	for _, p := range protocols {
		protocol, name := p.protocol, p.name
		if prev, ok := seen[protocol]; ok {
			t.Fatalf("protocol %d used by both %s and %s", protocol, prev, name)
		}
		seen[protocol] = name
	}
}

func TestIsZe(t *testing.T) {
	for _, protocol := range []int{FIBKernel, Static, PolicyRoute, int(GTSM), int(IPsecSourceGuard)} {
		if !IsZe(protocol) {
			t.Fatalf("IsZe(%d) = false, want true", protocol)
		}
	}
	if IsZe(4) {
		t.Fatal("IsZe(RTPROT_STATIC=4) = true, want false")
	}
}

// VALIDATES: spec-fixit-route-removal-protocol-blind AC-6 -- the interface
// layer's protocol is rendered by name but is not a Ze-owned protocol, so
// routewatch keeps delivering DHCP, RA and PPPoE route events.
// PREVENTS: adding Iface to the IsZe set, which silences those events for
// every routewatch subscriber ((*Watcher).deliver in internal/core/routewatch).
func TestIfaceProtocolIsNamedButNotZeOwned(t *testing.T) {
	if IsZe(int(Iface)) {
		t.Fatal("IsZe(Iface) = true, want false: routewatch would drop iface route events")
	}
	name, ok := Name(int(Iface))
	if !ok || name == "" {
		t.Fatalf("Name(Iface) = %q, %v, want a non-empty name", name, ok)
	}
}

// VALIDATES: spec-fixit-route-removal-protocol-blind D-1 -- Any is the named
// constant a caller uses to ask for a protocol-blind match.
// PREVENTS: a caller reaching a wildcard delete by omitting the argument.
func TestAnyIsTheUnsetProtocol(t *testing.T) {
	if Any != 0 {
		t.Fatalf("Any = %d, want 0 (the kernel's RTPROT_UNSPEC wildcard)", Any)
	}
	if _, ok := Name(int(Any)); ok {
		t.Fatal("Name(Any) reported a name: Any is a match rule, not a producer")
	}
}

// VALIDATES: the IPsec source-guard routes carry a Ze-owned protocol with a name, so
// routewatch does not report them as foreign churn and route output names their owner.
func TestIPsecSourceGuardProtocolIsZeOwnedAndNamed(t *testing.T) {
	if !IsZe(int(IPsecSourceGuard)) {
		t.Fatal("IsZe(IPsecSourceGuard) = false, want true")
	}
	if name, ok := Name(int(IPsecSourceGuard)); !ok || name != "ze-ipsec-guard" {
		t.Fatalf("Name(IPsecSourceGuard) = %q, %v; want ze-ipsec-guard, true", name, ok)
	}
}
