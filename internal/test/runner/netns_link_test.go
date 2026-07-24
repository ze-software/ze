// VALIDATES: option=netns-link parsing -- name= is required, address= is an
// optional CIDR parsed into a netip.Prefix, a malformed address is rejected at
// parse time on every platform, and a name-only spec leaves a zero (invalid)
// address meaning "link, no address".
// PREVENTS: the test/policy 005-next-hop regression where a policy next-hop had
// no connected route to resolve its gateway. The fix provisions an interface in
// the per-test netns from this option; a parse that silently dropped the option
// or accepted a bad address would leave the interface unprovisioned and the
// route add failing "network is unreachable" again, on Linux only.

package runner

import (
	"net/netip"
	"testing"
)

func parseNetnsLinkLine(t *testing.T, line string) (*Record, error) {
	t.Helper()
	et := &EncodingTests{}
	r := newRecord("netns-link-test")
	return r, et.parseLine(r, "test/policy/fake.ci", line)
}

func TestNetnsLinkParsesNameAndAddress(t *testing.T) {
	r, err := parseNetnsLinkLine(t, "option=netns-link:name=eth1:address=10.0.0.2/24")
	if err != nil {
		t.Fatalf("parseLine: %v", err)
	}
	if len(r.NetnsLinks) != 1 {
		t.Fatalf("NetnsLinks = %d, want 1", len(r.NetnsLinks))
	}
	got := r.NetnsLinks[0]
	if got.Name != "eth1" {
		t.Errorf("Name = %q, want eth1", got.Name)
	}
	want := netip.MustParsePrefix("10.0.0.2/24")
	if got.Address != want {
		t.Errorf("Address = %v, want %v", got.Address, want)
	}
}

func TestNetnsLinkNameOnlyLeavesAddressInvalid(t *testing.T) {
	r, err := parseNetnsLinkLine(t, "option=netns-link:name=eth1")
	if err != nil {
		t.Fatalf("parseLine: %v", err)
	}
	if len(r.NetnsLinks) != 1 {
		t.Fatalf("NetnsLinks = %d, want 1", len(r.NetnsLinks))
	}
	if r.NetnsLinks[0].Address.IsValid() {
		t.Errorf("Address = %v, want invalid (name-only means no address)", r.NetnsLinks[0].Address)
	}
}

func TestNetnsLinkMissingNameRejected(t *testing.T) {
	if _, err := parseNetnsLinkLine(t, "option=netns-link:address=10.0.0.2/24"); err == nil {
		t.Fatal("a netns-link without name= was accepted; the runner would have nothing to create")
	}
}

func TestNetnsLinkBadAddressRejected(t *testing.T) {
	if _, err := parseNetnsLinkLine(t, "option=netns-link:name=eth1:address=not-a-cidr"); err == nil {
		t.Fatal("a malformed address was accepted; it would fail silently at run time on Linux only")
	}
}
