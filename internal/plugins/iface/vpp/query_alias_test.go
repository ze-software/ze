// Design: docs/guide/vpp.md -- configured interface names and VPP interfaces
// Related: query.go -- detailsByName resolves configured aliases
// Related: apply_test.go -- dumpChannel supplies VPP replies

package ifacevpp

import (
	"errors"
	"slices"
	"testing"

	interfaces "go.fd.io/govpp/binapi/interface"

	"github.com/ze-software/ze/internal/component/iface"
)

// TestVPPQueriesResolveConfiguredAlias validates that link and address reads
// resolve the same registered identity as writes. It prevents VLAN reload
// mistaking qos0.100 for absent because VPP calls it loop1.100.
func TestVPPQueriesResolveConfiguredAlias(t *testing.T) {
	for _, addresses := range []bool{false, true} {
		name := "link"
		if addresses {
			name = "addresses"
		}
		t.Run(name, func(t *testing.T) {
			ch := addressTestChannel()
			ch.details[0].InterfaceName = asciiName("loop1.100")
			b := &vppBackendImpl{ch: ch, names: newNameMap()}
			if err := b.ensureChannel(); err != nil {
				t.Fatal(err)
			}
			b.names.Add("qos0.100", 10, "qos0.100")
			read := b.GetLink
			if addresses {
				read = b.GetInterface
			}
			info, err := read("qos0.100")
			if err != nil {
				t.Fatal(err)
			}
			if info.Name != "loop1.100" {
				t.Fatalf("name = %q, want resolved VPP name", info.Name)
			}
			if addresses {
				want := []string{"192.0.2.1/24 ipv4", "2001:db8::1/64 ipv6"}
				if !info.AddressesRead || !slices.Equal(addressesOf(info), want) {
					t.Fatalf("resolved addresses = %+v", info)
				}
			} else if ch.addressDumps != 0 || info.AddressesRead {
				t.Fatal("link read unexpectedly read addresses")
			}
		})
	}
}

// TestVPPQueriesKeepAliasReadFailures validates refusal when a registered
// identity cannot be read. It prevents treating a failed read as permission
// to create an interface again.
func TestVPPQueriesKeepAliasReadFailures(t *testing.T) {
	transport := errors.New("VPP dump unavailable")
	for _, failure := range []struct {
		name string
		err  error
	}{
		{name: "missing-index"},
		{name: "transport", err: transport},
	} {
		t.Run(failure.name, func(t *testing.T) {
			ch := addressTestChannel()
			b := &vppBackendImpl{ch: ch, names: newNameMap()}
			if err := b.ensureChannel(); err != nil {
				t.Fatal(err)
			}
			b.names.Add("qos0.100", 11, "qos0.100")
			ch.receiveErr = failure.err
			_, err := b.GetLink("qos0.100")
			if err == nil || errors.Is(err, iface.ErrInterfaceNotFound) {
				t.Fatalf("registered alias read must refuse, got %v", err)
			}
			if failure.err != nil && !errors.Is(err, failure.err) {
				t.Fatalf("lost transport failure: %v", err)
			}
		})
	}
}

// TestVPPQueriesFindUnmappedRawName validates discovery after initial map
// population, while preventing VPP substring matches from selecting a neighbor.
func TestVPPQueriesFindUnmappedRawName(t *testing.T) {
	ch := &dumpChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	if err := b.ensureChannel(); err != nil {
		t.Fatal(err)
	}
	ch.details = []interfaces.SwInterfaceDetails{{SwIfIndex: 10, InterfaceName: asciiName("loop1.100")}}
	if _, err := b.GetLink("loop1.100"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.GetLink("loop1"); !errors.Is(err, iface.ErrInterfaceNotFound) {
		t.Fatalf("substring neighbor must be absent, got %v", err)
	}
}
