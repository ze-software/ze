// VALIDATES: nbma + point-to-multipoint network types parse on both family leaves,
// the nbma-neighbor list and poll-interval resolve per family, and the IPv4/IPv6
// network-type enums stay independent.
// PREVENTS: a new enum value bleeding across family leaves; the NBMA neighbor list or
// poll-interval failing to thread through parseInterface.
package ospf

import (
	"errors"
	"net/netip"
	"os"
	"strings"
	"testing"
)

func TestOSPFParsePtMPInterface(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-multipoint"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.Interfaces[0].NetworkType != networkPointToMultipoint {
		t.Fatalf("network-type = %q, want point-to-multipoint", cfg.Interfaces[0].NetworkType)
	}
}

func TestOSPFv3ParsePtMPInterface(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}},"address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth1":{"area":"0","network-type":"point-to-multipoint"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.V6 == nil || len(cfg.V6.Interfaces) != 1 {
		t.Fatalf("v6 interfaces = %+v", cfg.V6)
	}
	if cfg.V6.Interfaces[0].NetworkType != networkPointToMultipoint {
		t.Fatalf("v6 network-type = %q, want point-to-multipoint", cfg.V6.Interfaces[0].NetworkType)
	}
}

func TestOSPFParseNBMAInterface(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"nbma","poll-interval":"90","nbma-neighbor":{"10.0.0.2":{"address":"10.0.0.2","priority":"5"},"10.0.0.3":{"address":"10.0.0.3","priority":"0"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	ic := cfg.Interfaces[0]
	if ic.NetworkType != networkNBMA {
		t.Fatalf("network-type = %q, want nbma", ic.NetworkType)
	}
	if ic.pollInterval() != 90 {
		t.Fatalf("poll-interval = %d, want 90", ic.pollInterval())
	}
	neighbors := ic.nbmaNeighborList()
	if len(neighbors) != 2 {
		t.Fatalf("nbma-neighbors = %d, want 2", len(neighbors))
	}
	// keyedList sorts by key: 10.0.0.2 then 10.0.0.3.
	if neighbors[0].Address != netip.MustParseAddr("10.0.0.2") || neighbors[0].Priority != 5 {
		t.Fatalf("neighbor[0] = %+v, want 10.0.0.2 priority 5", neighbors[0])
	}
	if neighbors[1].Address != netip.MustParseAddr("10.0.0.3") || neighbors[1].Priority != 0 {
		t.Fatalf("neighbor[1] = %+v, want 10.0.0.3 priority 0", neighbors[1])
	}
}

func TestOSPFParseNBMAInterfaceDefaultPoll(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"nbma"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.Interfaces[0].pollInterval() != DefaultPollInterval {
		t.Fatalf("default poll-interval = %d, want %d", cfg.Interfaces[0].pollInterval(), DefaultPollInterval)
	}
}

func TestOSPFv3ParseNBMAInterface(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0"}}},"address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth1":{"area":"0","network-type":"nbma","poll-interval":"60","nbma-neighbor":{"2.2.2.2":{"router-id":"2.2.2.2","link-local":"fe80::2","priority":"3"}}}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	ic := cfg.V6.Interfaces[0]
	if ic.NetworkType != networkNBMA || ic.pollInterval() != 60 {
		t.Fatalf("v6 nbma iface = %+v", ic)
	}
	neighbors := ic.nbmaNeighborList()
	if len(neighbors) != 1 {
		t.Fatalf("v6 nbma-neighbors = %d, want 1", len(neighbors))
	}
	n := neighbors[0]
	if n.RouterID != ridOf("2.2.2.2") || n.LinkLocal != netip.MustParseAddr("fe80::2") || n.Priority != 3 {
		t.Fatalf("v6 neighbor = %+v, want rid 2.2.2.2 ll fe80::2 priority 3", n)
	}
}

func TestOSPFParseNBMAInterfaceRejectsBadLinkLocal(t *testing.T) {
	_, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","address-family":{"ipv6":{"interfaces":{"interface":{"eth1":{"area":"0","network-type":"nbma","nbma-neighbor":{"2.2.2.2":{"router-id":"2.2.2.2","link-local":"2001:db8::1"}}}}}}}}}`), nil)
	if err == nil || !strings.Contains(err.Error(), "link-local") {
		t.Fatalf("expected a link-local rejection, got %v", err)
	}
}

// TestOSPFNBMARequiresNeighbor: RFC 2328 App C.6 -- an nbma interface with an empty
// nbma-neighbor list has no one to unicast to (no multicast fallback), so validateConfig
// rejects it; one neighbor is enough. A point-to-multipoint interface with an empty list
// stays valid (it discovers neighbors via multicast Hellos).
func TestOSPFNBMARequiresNeighbor(t *testing.T) {
	empty, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"nbma"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(empty nbma): %v", err)
	}
	if err := validateConfig(empty); !errors.Is(err, ErrNBMANoNeighbors) {
		t.Fatalf("validateConfig(nbma, no neighbor) = %v, want ErrNBMANoNeighbors", err)
	}

	withNbr, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"nbma","nbma-neighbor":{"10.0.0.2":{"address":"10.0.0.2","priority":"1"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(nbma w/ neighbor): %v", err)
	}
	if err := validateConfig(withNbr); err != nil {
		t.Fatalf("validateConfig(nbma, one neighbor) = %v, want nil", err)
	}

	ptmp, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-multipoint"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(ptmp): %v", err)
	}
	if err := validateConfig(ptmp); err != nil {
		t.Fatalf("validateConfig(point-to-multipoint, no neighbor) = %v, want nil", err)
	}
}

// TestOSPFNetworkTypeV4V6Isolation: both family leaves gain nbma + point-to-multipoint,
// and the v4-only loopback stays per leaf. The YANG enums are the authoritative
// per-family gate (AC-17, R-9, A-12).
func TestOSPFNetworkTypeV4V6Isolation(t *testing.T) {
	src, err := os.ReadFile("yang/ze-ospf-conf.yang")
	if err != nil {
		t.Fatalf("read yang: %v", err)
	}
	yangText := string(src)
	// The two interface leaf blocks are isolated by their container description marker up
	// to the first `leaf passive` (which follows the network-type enum, poll-interval, and
	// nbma-neighbor list in each block). This is independent of the order the two family
	// blocks appear in the module.
	ipv4 := yangSection(yangText, "OSPF-enabled interfaces.", "leaf passive")
	ipv6 := yangSection(yangText, "OSPFv3-enabled interfaces", "leaf passive")
	if ipv4 == "" || ipv6 == "" {
		t.Fatalf("could not isolate the two interface sections")
	}
	for _, v := range []string{"enum nbma;", "enum point-to-multipoint;"} {
		if !strings.Contains(ipv4, v) {
			t.Errorf("IPv4 interface leaf missing %q", v)
		}
		if !strings.Contains(ipv6, v) {
			t.Errorf("IPv6 interface leaf missing %q", v)
		}
	}
	if !strings.Contains(ipv4, "enum loopback;") {
		t.Errorf("IPv4 interface leaf lost enum loopback;")
	}
	if strings.Contains(ipv6, "enum loopback;") {
		t.Errorf("IPv6 interface leaf gained enum loopback; (must stay IPv4-only)")
	}
	if !strings.Contains(ipv4, "list nbma-neighbor") || !strings.Contains(ipv6, "list nbma-neighbor") {
		t.Errorf("nbma-neighbor list missing from a family leaf")
	}
}

// yangSection returns the substring of text from the first occurrence of start up to
// the first occurrence of end after it (or to the end when end is "").
func yangSection(text, start, end string) string {
	_, after, ok := strings.Cut(text, start)
	if !ok {
		return ""
	}
	if end == "" {
		return after
	}
	section, _, _ := strings.Cut(after, end)
	return section
}
