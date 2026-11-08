package system

import (
	"slices"
	"testing"
)

// TestResolverOriginRegistryOrdersByRank: sources order by the rank their
// origin registered, whatever order the origins registered in, and a second
// registration of a name or a rank panics.
//
// VALIDATES: AC-14, the registered rank replaces the closed origin list.
// PREVENTS: a new lease protocol reordering existing servers, and two
// producers silently sharing one place in resolv.conf.
func TestResolverOriginRegistryOrdersByRank(t *testing.T) {
	var registry resolverOriginRegistry
	ppp := registry.register("ppp", 300, false)
	v4 := registry.register("dhcpv4", 100, false)
	v6 := registry.register("dhcpv6", 200, false)

	var r Resolvers
	for _, lease := range []struct {
		source ResolverSource
		server string
	}{
		{ppp.Source("wan0", ""), "192.0.2.3"},
		{v6.Source("eth0", "0"), "2001:db8::2"},
		{v4.Source("eth0", "0"), "192.0.2.1"},
	} {
		if err := r.SetLease(lease.source, []string{lease.server}, UntilWithdrawn); err != nil {
			t.Fatalf("set lease %v: %v", lease.source, err)
		}
	}
	if got, want := r.Servers(), []string{"192.0.2.1", "2001:db8::2", "192.0.2.3"}; !slices.Equal(got, want) {
		t.Fatalf("servers = %v, want %v (rank order)", got, want)
	}

	for _, tc := range []struct {
		label string
		name  string
		rank  uint16
	}{
		{"duplicate name", "ppp", 400},
		{"duplicate rank", "slaac", 200},
		{"rank zero", "slaac", 0},
		{"empty name", "", 500},
	} {
		t.Run(tc.label, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("register(%q, %d) did not panic", tc.name, tc.rank)
				}
			}()
			registry.register(tc.name, tc.rank, false)
		})
	}

	if err := r.SetLease(ResolverOrigin{}.Source("eth0", "0"), []string{"192.0.2.9"}, UntilWithdrawn); err == nil {
		t.Fatal("a source of the zero origin was recorded")
	}
}
