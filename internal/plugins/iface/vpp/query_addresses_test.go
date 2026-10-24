// Design: docs/guide/vpp.md -- VPP interface backend
// Related: query.go -- GetInterface, ListInterfaces and GetLink judged here
// Related: apply_test.go -- dumpChannel, the fake these tests drive

package ifacevpp

import (
	"net/netip"
	"slices"
	"strconv"
	"testing"

	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_types"

	"github.com/ze-software/ze/internal/component/iface"
)

// addressTestChannel answers one interface, xe0 at index 10, holding one
// IPv4 and one IPv6 prefix.
func addressTestChannel() *dumpChannel {
	return &dumpChannel{
		details: []interfaces.SwInterfaceDetails{
			{SwIfIndex: 10, InterfaceName: asciiName("xe0")},
		},
		addresses: map[interface_types.InterfaceIndex][]ip_types.AddressWithPrefix{
			10: {
				toAddressWithPrefix(netip.MustParsePrefix("192.0.2.1/24")),
				toAddressWithPrefix(netip.MustParsePrefix("2001:db8::1/64")),
			},
		},
	}
}

// addressesOf renders an answer's addresses as "address/length family".
func addressesOf(info *iface.InterfaceInfo) []string {
	out := make([]string, 0, len(info.Addresses))
	for _, a := range info.Addresses {
		out = append(out, a.Address+"/"+strconv.Itoa(a.PrefixLength)+" "+a.Family)
	}
	slices.Sort(out)
	return out
}

// TestVPPGetInterfaceReportsAddresses reads one interface whose VPP state
// holds an IPv4 and an IPv6 prefix.
//
// VALIDATES: GetInterface answers both prefixes VPP holds, marks the
// addresses read, and says the DAD outcome is not reported.
// PREVENTS: an interface holding addresses answered as holding none, which
// IS-IS, OSPF, BFD and `show interface` read as "no address".
func TestVPPGetInterfaceReportsAddresses(t *testing.T) {
	ch := addressTestChannel()
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	info, err := b.GetInterface("xe0")
	if err != nil {
		t.Fatalf("GetInterface: %v", err)
	}
	want := []string{"192.0.2.1/24 ipv4", "2001:db8::1/64 ipv6"}
	if got := addressesOf(info); !slices.Equal(got, want) {
		t.Errorf("addresses: got %v, want %v", got, want)
	}
	if !info.AddressesRead {
		t.Error("AddressesRead false on an answer that read the addresses")
	}
	if info.DADReported {
		t.Error("DADReported true: VPP's address dump carries no DAD outcome")
	}
}

// TestVPPListInterfacesReportsAddresses lists the same interface.
//
// VALIDATES: every listed interface carries the addresses VPP holds.
// PREVENTS: the list answering every interface address-less.
func TestVPPListInterfacesReportsAddresses(t *testing.T) {
	ch := addressTestChannel()
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	got, err := b.ListInterfaces()
	if err != nil {
		t.Fatalf("ListInterfaces: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1", len(got))
	}
	want := []string{"192.0.2.1/24 ipv4", "2001:db8::1/64 ipv6"}
	if addrs := addressesOf(&got[0]); !slices.Equal(addrs, want) {
		t.Errorf("addresses: got %v, want %v", addrs, want)
	}
	if !got[0].AddressesRead {
		t.Error("AddressesRead false on a listed interface whose addresses were read")
	}
}

// TestVPPGetLinkReadsNoAddress reads the link facts of the same interface.
//
// VALIDATES: GetLink issues no IPAddressDump and answers the addresses
// unread, as Backend.GetLink specifies.
// PREVENTS: a failed address dump failing an existence or MTU check.
func TestVPPGetLinkReadsNoAddress(t *testing.T) {
	ch := addressTestChannel()
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	info, err := b.GetLink("xe0")
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}
	if ch.addressDumps != 0 {
		t.Errorf("GetLink issued %d IPAddressDump, want 0", ch.addressDumps)
	}
	if info.AddressesRead {
		t.Error("AddressesRead true on a GetLink answer")
	}
	if len(info.Addresses) != 0 {
		t.Errorf("GetLink answered addresses %v", info.Addresses)
	}
}
