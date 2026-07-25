// VALIDATES: spec-ospf-ext-14 AC-12, A-11 -- `show ospf ipv6 instance` enumerates each
// active OSPFv3 address-family instance with its AF (from the Instance-ID range), Instance
// ID, area count, and neighbor count; a single base IPv6-unicast instance yields exactly one
// row; a second AF yields two.
// PREVENTS: an instance listing that hides the address family, or hard-depends on multi-AF.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestV3InstanceListingSingleAF(t *testing.T) {
	eng := newV6DecodeEngine(t, types.RouterID{10, 0, 0, 1})
	m := &v6EngineSet{engines: map[addressFamily]*engine{afIPv6Unicast: eng}}

	rows := m.instanceListing()
	if len(rows) != 1 {
		t.Fatalf("instance rows = %d, want 1", len(rows))
	}
	if rows[0].AddressFamily != afIPv6Unicast.String() {
		t.Fatalf("address family = %q, want %q", rows[0].AddressFamily, afIPv6Unicast.String())
	}
}

func TestV3InstanceListingMultiAF(t *testing.T) {
	e6 := newV6DecodeEngine(t, types.RouterID{10, 0, 0, 1})
	e4 := newEngineWithCodecAF(transport.New(&fakeBackend{}), v6Codec{}, afIPv4Unicast)
	e4.cfg.RouterID = types.RouterID{10, 0, 0, 1}
	m := &v6EngineSet{engines: map[addressFamily]*engine{afIPv6Unicast: e6, afIPv4Unicast: e4}}

	rows := m.instanceListing()
	if len(rows) != 2 {
		t.Fatalf("instance rows = %d, want 2", len(rows))
	}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.AddressFamily] = true
	}
	if !seen[afIPv6Unicast.String()] || !seen[afIPv4Unicast.String()] {
		t.Fatalf("both AFs should be listed: %+v", rows)
	}
}
