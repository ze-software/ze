package system_test

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/dnssource"
)

// TestRegistryKeepsEachConsumersLastPublication proves the registry replaces
// a consumer's set with each publication (a removed list leaves by being
// absent), keeps other consumers untouched, answers them sorted by name, and
// refuses a publication with no consumer name. Method: publish twice for one
// consumer and once for another on a fresh Resolvers, then read Consumers.
func TestRegistryKeepsEachConsumersLastPublication(t *testing.T) {
	var r system.Resolvers
	subnet := dnssource.ConsumerSelection{
		Path:     "dhcp-server subnet dns-source",
		Instance: "192.0.2.0/24",
		Sources:  []dnssource.Class{dnssource.ClassReceived},
		Servers:  []netip.Addr{netip.MustParseAddr("192.0.2.53")},
	}
	second := subnet
	second.Instance = "198.51.100.0/24"
	if err := r.SetPublication(&dnssource.Publication{Consumer: "dhcp-server", Selections: []dnssource.ConsumerSelection{subnet, second}}); err != nil {
		t.Fatalf("first publication refused: %v", err)
	}
	if err := r.SetPublication(&dnssource.Publication{Consumer: "dns-resolver", Selections: []dnssource.ConsumerSelection{{Path: "service dns-resolver forward dns-source"}}}); err != nil {
		t.Fatalf("resolver publication refused: %v", err)
	}
	if err := r.SetPublication(&dnssource.Publication{Consumer: "dhcp-server", Selections: []dnssource.ConsumerSelection{second}}); err != nil {
		t.Fatalf("replacing publication refused: %v", err)
	}

	got := r.Consumers()
	if len(got) != 2 {
		t.Fatalf("Consumers() = %d publications, want 2: %+v", len(got), got)
	}
	if got[0].Consumer != "dhcp-server" || got[1].Consumer != "dns-resolver" {
		t.Fatalf("Consumers() order = %q, %q; want dhcp-server, dns-resolver", got[0].Consumer, got[1].Consumer)
	}
	if len(got[0].Selections) != 1 || got[0].Selections[0].Instance != "198.51.100.0/24" {
		t.Fatalf("dhcp-server selections = %+v, want the second publication's one subnet", got[0].Selections)
	}

	if err := r.SetPublication(&dnssource.Publication{}); err == nil {
		t.Fatal("a publication with no consumer name was accepted")
	}
}
