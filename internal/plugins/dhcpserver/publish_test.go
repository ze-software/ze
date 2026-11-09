package dhcpserver

import (
	"encoding/json"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// publishedSelections decodes the one body send captured into its selections.
func publishedSelections(t *testing.T, bodies []string) []dnssource.ConsumerSelection {
	t.Helper()
	if len(bodies) != 1 {
		t.Fatalf("published %d bodies, want 1: %q", len(bodies), bodies)
	}
	var publication dnssource.Publication
	if err := json.Unmarshal([]byte(bodies[0]), &publication); err != nil {
		t.Fatalf("decode publication: %v", err)
	}
	if publication.Consumer != dhcpConsumer {
		t.Fatalf("consumer %q, want %q", publication.Consumer, dhcpConsumer)
	}
	return publication.Selections
}

// TestDHCPPublisherPublishesEachSubnet proves the DHCP server publishes one
// selection per subnet for show system dns and ze doctor: named by the
// subnet prefix, with the self class filled from the listened device's
// address inside the subnet, published again when a snapshot changes the
// answer, and withdrawn as an empty set when the server stops serving.
// Method: a publisher over one handler and a stubbed device, a captured
// sender, and the decoded bodies.
func TestDHCPPublisherPublishesEachSubnet(t *testing.T) {
	previous := dhcpDeviceAddrs
	dhcpDeviceAddrs = func(string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("198.51.100.1"), netip.MustParseAddr("192.0.2.1")}, nil
	}
	t.Cleanup(func() { dhcpDeviceAddrs = previous })

	list, err := dnssource.NewList(subnetDNSSourcePath, dnssource.FamilyIPv4, dnssource.AudienceHandedOn,
		dnssource.ModeMerge, dnssource.ClassSelf, dnssource.ClassReceived)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	h := dnsTestSubnet(t, list)
	var follower dnssource.Follower
	h.learned = &follower

	var publisher dhcpPublisher
	var bodies []string
	publisher.serve([]*dhcpHandler{h}, []dhcpListener{{iface: "lan", device: "lan0"}})
	if len(bodies) != 0 {
		t.Fatalf("published before the sender was set: %q", bodies)
	}
	publisher.setSender(func(body string) { bodies = append(bodies, body) })

	selections := publishedSelections(t, bodies)
	if len(selections) != 1 {
		t.Fatalf("published %d selections, want 1", len(selections))
	}
	self := netip.MustParseAddr("192.0.2.1")
	got := selections[0]
	if got.Instance != "192.0.2.0/24" || got.Path != subnetDNSSourcePath {
		t.Fatalf("selection names %q at %q, want 192.0.2.0/24 at %q", got.Instance, got.Path, subnetDNSSourcePath)
	}
	if !slices.Equal(got.Servers, []netip.Addr{self}) || !slices.Equal(got.Self, []netip.Addr{self}) {
		t.Fatalf("servers %v self %v, want both [%s]", got.Servers, got.Self, self)
	}

	publisher.refresh()
	if len(bodies) != 1 {
		t.Fatalf("an unchanged answer was published again: %q", bodies)
	}

	learned := netip.MustParseAddr("203.0.113.53")
	learnLease(t, &follower, 1, learned)
	publisher.refresh()
	selections = publishedSelections(t, bodies[1:])
	if !slices.Equal(selections[0].Servers, []netip.Addr{self, learned}) {
		t.Fatalf("after the lease servers %v, want [%s %s]", selections[0].Servers, self, learned)
	}

	publisher.serve(nil, nil)
	if selections = publishedSelections(t, bodies[2:]); len(selections) != 0 {
		t.Fatalf("a stopped server still publishes %d selections", len(selections))
	}
}
