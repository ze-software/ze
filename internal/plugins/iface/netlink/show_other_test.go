//go:build !linux

package ifacenetlink

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

// VALIDATES: the non-Linux backend lists the host's interfaces through the
// standard library, loopback marked, so `ze init` discovers something to write.
// PREVENTS: the stub answering "not supported" again, which left `ze init` on a
// developer machine with no initial config at all.
func TestStubListInterfacesFindsLoopback(t *testing.T) {
	infos, err := (&stubBackend{}).ListInterfaces()
	if err != nil {
		t.Fatalf("ListInterfaces: %v", err)
	}
	for i := range infos {
		if infos[i].Type == "loopback" {
			return
		}
	}
	t.Fatalf("ListInterfaces returned %d interfaces and no loopback", len(infos))
}

// VALIDATES: GetInterface answers the interface ListInterfaces reported.
func TestStubGetInterfaceMatchesList(t *testing.T) {
	infos, err := (&stubBackend{}).ListInterfaces()
	if err != nil {
		t.Fatalf("ListInterfaces: %v", err)
	}
	if len(infos) == 0 {
		t.Fatal("ListInterfaces returned no interface")
	}
	got, err := (&stubBackend{}).GetInterface(infos[0].Name)
	if err != nil {
		t.Fatalf("GetInterface(%q): %v", infos[0].Name, err)
	}
	if got.Index != infos[0].Index {
		t.Fatalf("GetInterface(%q).Index = %d, want %d", infos[0].Name, got.Index, infos[0].Index)
	}
}

// VALIDATES: addresses keep their family, prefix length and link-local mark,
// and a non-prefix net.Addr is skipped.
func TestStdlibAddrInfo(t *testing.T) {
	_, v4, _ := net.ParseCIDR("192.0.2.1/24")
	_, v6, _ := net.ParseCIDR("fe80::1/64")
	v4.IP = net.ParseIP("192.0.2.1")
	v6.IP = net.ParseIP("fe80::1")
	got := stdlibAddrInfo([]net.Addr{v4, &net.IPAddr{IP: net.ParseIP("192.0.2.9")}, v6})
	want := []iface.AddrInfo{
		{Address: "192.0.2.1", PrefixLength: 24, Family: "ipv4"},
		{Address: "fe80::1", PrefixLength: 64, Family: "ipv6", LinkLocal: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d addresses %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("address %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
