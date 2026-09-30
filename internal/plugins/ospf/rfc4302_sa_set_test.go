// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- OSPFv3 IPsec SA installation.
// Related: ipsec_install.go -- buildIPsecInterfaceSAs, the set of SAs one interface installs.
// Related: ipsec_rfc4302_test.go -- the multicast-state identifier and selector tests.
//
// VALIDATES: RFC 4302 on the whole SA set Ze installs for an AH interface: the UNICAST
// inbound state (this interface's own link-local, the destination of every unicast OSPF
// packet a neighbor sends) is keyed on the configured SPI, AH and that unicast address
// (section 2.4), and a 32-packet replay window read from the configuration text reaches
// every SA of the set (section 3.4.3).
// PREVENTS: a unicast state keyed on the wrong address or SPI, and a supported minimum
// window that validates but never reaches an SA.
package ospf

import (
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// rfc4302Local is the interface link-local the unicast inbound state is keyed on.
var rfc4302Local = netip.MustParseAddr("fe80::1")

// rfc4302UnicastState returns the one inbound state of the set whose destination is local.
func rfc4302UnicastState(t *testing.T, set []dataplane.SAParams) dataplane.SAParams {
	t.Helper()
	var found []dataplane.SAParams
	for i := range set {
		if set[i].Dst.Equal(rfc4302Local.AsSlice()) {
			found = append(found, set[i])
		}
	}
	if len(found) != 1 {
		t.Fatalf("SA set holds %d states for the unicast destination %s, want 1: %+v", len(found), rfc4302Local, set)
	}
	return found[0]
}

// RFC requirement: RFC4302-2.4-1 positive -- the AH interface's SA set holds one inbound
// state for the unicast destination (this interface's link-local fe80::1), identified by the
// configured SPI, protocol AH and that destination, with only the source wildcarded: the
// {SPI, destination, protocol} the kernel maps inbound unicast AH traffic by.
// RFC requirement: RFC4302-2.4-1 negative -- the unicast identifier follows the
// configuration: the same interface configured with another SPI keys its unicast state on
// that SPI, so a unicast state carrying a fixed or foreign SPI fails here.
func TestRFC4302UnicastAHStateIdentifiedBySPI(t *testing.T) {
	// Goal: the unicast half of the section 2.4 mapping. Method: build the interface's
	// whole SA set and read the state keyed on the unicast link-local.
	sa := rfc4302UnicastState(t, buildIPsecInterfaceSAs(testIfIndex, rfc4302Local, ahIPsec(0x1000)))
	if sa.SPI != 0x1000 {
		t.Errorf("unicast AH state spi = %#x, want 0x1000", sa.SPI)
	}
	if sa.Proto != dataplane.ProtoAH {
		t.Errorf("unicast AH state proto = %d, want %d (AH)", sa.Proto, dataplane.ProtoAH)
	}
	if sa.Dir != dataplane.SADirIn {
		t.Errorf("unicast AH state direction = %v, want inbound", sa.Dir)
	}
	if !sa.Src.Equal(net.IPv6zero) {
		t.Errorf("unicast AH state src = %v, want :: (the source alone is wildcarded)", sa.Src)
	}
	other := rfc4302UnicastState(t, buildIPsecInterfaceSAs(testIfIndex, rfc4302Local, ahIPsec(0x2000)))
	if other.SPI != 0x2000 {
		t.Errorf("unicast AH state spi = %#x with SPI 0x2000 configured, want 0x2000", other.SPI)
	}
}

// rfc4302InstalledSAs brings one AH interface up through the installer whose link-local is
// local, and returns every state the installer handed the dataplane.
func rfc4302InstalledSAs(t *testing.T, local netip.Addr) []dataplane.SAParams {
	t.Helper()
	inst, fake := testInstaller(t, local)
	inst.setConfig([]interfaceConfig{ahIface(0x1000)})
	inst.onInterfaceUp(testIfIndex, "eth1")
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return append([]dataplane.SAParams(nil), fake.sas...)
}

// rfc4302AssertIndication asserts the address-match indication of every installed state:
// the source is the :: wildcard (no source matching) and the destination is set, and the
// set of destinations is exactly want, one state each.
func rfc4302AssertIndication(t *testing.T, set []dataplane.SAParams, want ...netip.Addr) {
	t.Helper()
	if len(set) != len(want) {
		t.Fatalf("installer handed %d states to the dataplane, want %d: %+v", len(set), len(want), set)
	}
	seen := map[netip.Addr]int{}
	for i := range set {
		if !set[i].Src.Equal(net.IPv6zero) {
			t.Errorf("state to %v src = %v, want :: (the manual SA requires no source match)", set[i].Dst, set[i].Src)
		}
		dst, ok := netip.AddrFromSlice(set[i].Dst)
		if !ok {
			t.Errorf("state %d carries no destination: the destination match is unset", i)
			continue
		}
		seen[dst.Unmap()]++
	}
	for _, addr := range want {
		if seen[addr] != 1 {
			t.Errorf("states keyed on destination %s = %d, want 1 (destinations seen %v)", addr, seen[addr], seen)
		}
	}
}

// RFC requirement: RFC4302-2.4-5 positive -- RFC 4302 Section 2.4: "The indication of
// whether source and destination address matching is required to map inbound IPsec
// traffic to SAs MUST be set either as a side effect of manual SA configuration or via
// negotiation using an SA management protocol, e.g., IKE or Group Domain of
// Interpretation (GDOI) [RFC3547]." Ze negotiates no AH SA, so the manual RFC 4552
// configuration sets it: bringing a configured AH interface up installs three states, and
// every one has a set destination (ff02::5, ff02::6 and the interface link-local fe80::1,
// one each) and the :: source, the SPI-plus-destination identifier section 2.4 gives an
// Any-Source Multicast SA, which is how the kernel's inbound lookup then maps them.
// RFC requirement: RFC4302-2.4-5 negative -- the indication is a side effect of THIS
// interface's manual configuration rather than a constant: the same AH block on an
// interface whose link-local is fe80::2 keys its unicast state on fe80::2, no state is
// keyed on fe80::1, and no state is left without a destination, so an installer that set
// a fixed or an unset destination fails here.
func TestRFC4302AddressMatchIndicationSetByManualConfig(t *testing.T) {
	// Goal: the address-match indication of every AH state follows the manual
	// configuration. Method: bring the interface up through the installer (the path a
	// configured interface takes) and read the states the dataplane received.
	allSPF := netip.MustParseAddr("ff02::5")
	allDR := netip.MustParseAddr("ff02::6")
	rfc4302AssertIndication(t, rfc4302InstalledSAs(t, rfc4302Local), allSPF, allDR, rfc4302Local)

	moved := netip.MustParseAddr("fe80::2")
	rfc4302AssertIndication(t, rfc4302InstalledSAs(t, moved), allSPF, allDR, moved)
}

// RFC requirement: RFC4302-3.4.3-5 positive -- a replay-window of 32, the minimum RFC 4302
// requires be supported, parsed from the configuration text of an AH interface, passes
// validateConfig and reaches every SA of that interface's SA set (both multicast states and
// the unicast state) as a 32-packet window.
func TestRFC4302MinimumReplayWindowReachesEverySA(t *testing.T) {
	// Goal: 32 is supported end to end, not only accepted. Method: config text -> validate
	// -> buildIPsecInterfaceSAs, read ReplayWin on each SA.
	cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
		`"protocol":"ah","spi":256,"algorithm":"sha256","key":"`+hexKey(32)+`","replay-window":32`, "")), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig(replay-window 32) = %v, want accepted", err)
	}
	set := buildIPsecInterfaceSAs(testIfIndex, rfc4302Local, *cfg.V6.Interfaces[0].IPsec)
	if len(set) != 3 {
		t.Fatalf("SA set holds %d states, want 3", len(set))
	}
	for i := range set {
		if set[i].ReplayWin != 32 {
			t.Errorf("SA to %v ReplayWin = %d, want 32", set[i].Dst, set[i].ReplayWin)
		}
	}
}
