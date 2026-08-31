// VALIDATES: spec-ospf-ext-16 AC-7/AC-10/AC-11 + R-1/R-2/R-5/A-3/A-8 -- the IPsec
// installer builds the RFC 4552 transport-mode SAs/policies from the interface
// link-local, installs on interface-up before the first Hello, removes on down,
// and reconciles a changed SPI/key.
// PREVENTS: an interface that appears protected but has no kernel SA, and a stale
// SA surviving a rekey.

package ospf

import (
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

type fakeDP struct {
	sas         []dataplane.SAParams
	pols        []dataplane.SPParams
	removedSAs  []uint32
	removedPols []dataplane.SPParams
}

func (f *fakeDP) InstallSA(p dataplane.SAParams) error { f.sas = append(f.sas, p); return nil }
func (f *fakeDP) RemoveSA(spi uint32, _ net.IP, _ uint8) error {
	f.removedSAs = append(f.removedSAs, spi)
	return nil
}
func (f *fakeDP) InstallPolicy(p dataplane.SPParams) error { f.pols = append(f.pols, p); return nil }
func (f *fakeDP) RemovePolicyParams(p dataplane.SPParams) error {
	f.removedPols = append(f.removedPols, p)
	return nil
}

// testIfIndex is the fixed kernel ifindex the test installer reports for every interface.
const testIfIndex = 3

// testInstaller builds an installer wired to a fake dataplane and a fixed link-local.
func testInstaller(t *testing.T, ll netip.Addr) (*ipsecInstaller, *fakeDP) {
	t.Helper()
	fake := &fakeDP{}
	inst := newIPsecInstaller(nil, nil)
	inst.dpSource = func() (ipsecDataplane, error) { return fake, nil }
	inst.setTransportSource(func(string) (netip.Addr, int, bool) { return ll, testIfIndex, true })
	return inst, fake
}

// espIface is the ESP fixture for the unit tests, on the fixed interface "eth1".
func espIface(spi uint32) interfaceConfig {
	return interfaceConfig{
		Name:  "eth1",
		IPsec: &ipsecInterfaceConfig{SPI: spi, Protocol: "esp", AuthAlgo: "sha256", AuthKey: hexKey(32)},
	}
}

func TestIPsecInstallOnInterfaceUp(t *testing.T) {
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{espIface(256)})

	// RFC requirement: RFC4301-4.5-1 positive -- manual keying: the OSPFv3 (RFC 4552) installer
	// installs the SA and policies from a statically configured SPI+key with no IKE exchange
	// (the automated-keying half is TestChildSAInstallsInDataplane).
	inst.onInterfaceUp(testIfIndex, "eth1")

	// RFC 4552 §7: one shared wildcard SA (the same (::, spi, proto) state protects
	// egress and verifies ingress), plus out/in/fwd require-policies.
	// RFC requirement: RFC4552-3-2 positive -- an esp interface installs a kernel SA (Proto
	// resolves to ProtoESP), so ESP is supported (ipsecProtoNumber, ipsec_install.go:454-459).
	if len(fake.sas) != 1 {
		t.Fatalf("InstallSA count = %d, want 1 (shared wildcard SA)", len(fake.sas))
	}
	// RFC requirement: RFC4552-11-3 positive -- an IPsec-enabled interface installs the
	// out/in/fwd protect (require) policies into the SPD (installLocked, ipsec_install.go:329-337).
	if len(fake.pols) != 3 {
		t.Fatalf("InstallPolicy count = %d, want 3 (out/in/fwd)", len(fake.pols))
	}
}

func TestIPsecInstallBeforeFirstHello(t *testing.T) {
	// R-1/AC-7: install completes synchronously within the up callback, so the kernel
	// policy+SA exist before the engine starts the interface FSM (which sends Hellos).
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{espIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")
	if len(fake.sas) == 0 || len(fake.pols) == 0 {
		t.Fatal("install did not complete synchronously in onInterfaceUp")
	}
}

func TestIPsecRemoveOnInterfaceDown(t *testing.T) {
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{espIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")
	inst.onInterfaceDown(testIfIndex, "eth1")

	if len(fake.removedPols) != 3 {
		t.Errorf("RemovePolicyParams count = %d, want 3", len(fake.removedPols))
	}
	if len(fake.removedSAs) != 1 {
		t.Errorf("RemoveSA count = %d, want 1 (shared wildcard SA)", len(fake.removedSAs))
	}
	if _, ok := inst.status("eth1"); ok {
		t.Error("status still reports IPsec after down")
	}
}

func TestIPsecReconcileReplacesSA(t *testing.T) {
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{espIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")

	// Rekey: change the SPI, push new config, reconcile.
	inst.setConfig([]interfaceConfig{espIface(512)})
	inst.reconcileAll()

	if len(fake.removedSAs) != 1 {
		t.Errorf("rekey RemoveSA count = %d, want 1 (old shared SA removed)", len(fake.removedSAs))
	}
	if len(fake.sas) != 2 {
		t.Errorf("total InstallSA = %d, want 2 (1 old + 1 new)", len(fake.sas))
	}
	st, ok := inst.status("eth1")
	if !ok || st.SPI != 512 {
		t.Errorf("status after rekey = %+v, want spi 512", st)
	}
}

func TestIPsecSAIsWildcardWithOSPFSelector(t *testing.T) {
	// FIX-1 (corrected model): the SA is a single wildcard-address (Src=Dst=::)
	// transport-mode state bound to a {::/0, ::/0, proto 89} selector, so the kernel
	// resolves it for every OSPFv3 destination in both directions -- not the broken
	// Dst=ff02::5 / Dst=link-local pair that could not carry unicast DBD or ff02::6.
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{espIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")

	// RFC requirement: RFC4552-7-1 positive -- one manually configured SPI/key drives a single shared
	// SA that both protects egress and verifies ingress with the same parameters (installed once,
	// ipsec_install.go:316-328), so exactly one SA is installed for both directions.
	if len(fake.sas) != 1 {
		t.Fatalf("want 1 shared SA, got %d: %+v", len(fake.sas), fake.sas)
	}
	sa := fake.sas[0]
	if !sa.Src.Equal(net.IPv6zero) || !sa.Dst.Equal(net.IPv6zero) {
		t.Errorf("SA must be wildcard-address (::,::); got src=%v dst=%v", sa.Src, sa.Dst)
	}
	// RFC requirement: RFC4301-4.1-1 positive -- Ze supports both IPsec modes; this asserts the
	// transport-mode half: the OSPFv3 (RFC 4552) SA is installed with Mode == ModeTransport
	// (the tunnel-mode half is TestChildSAInstallsInDataplane, the IKE Child SA).
	// RFC requirement: RFC4552-2-2 positive -- transport-mode SA MUST be supported: the installed
	// OSPFv3 SA carries Mode == ModeTransport (buildIPsecSA Mode=ModeTransport, ipsec_install.go:413),
	// and there is no tunnel-mode SA path for OSPFv3 to reject.
	if sa.Mode != dataplane.ModeTransport {
		t.Errorf("SA mode = %d, want transport (RFC 4552 §2)", sa.Mode)
	}
	if sa.ReqID != ipsecReqIDBase+uint32(testIfIndex) {
		t.Errorf("SA reqid = %d, want per-interface base+ifindex %d", sa.ReqID, ipsecReqIDBase+uint32(testIfIndex))
	}
	if sa.Sel == nil {
		t.Fatal("SA must carry a state selector so the kernel resolves it for any OSPF daddr")
	}
	if sa.Sel.UpperProto != ospfv3transport.Protocol {
		t.Errorf("SA selector UpperProto = %d, want %d (OSPF)", sa.Sel.UpperProto, ospfv3transport.Protocol)
	}
	// The selector Dst must be the ::/0 wildcard so it covers ff02::5, ff02::6, and
	// neighbor link-local unicast alike (not just AllSPFRouters).
	assertWildcardV6(t, "SA selector src", sa.Sel.Src)
	assertWildcardV6(t, "SA selector dst", sa.Sel.Dst)
	for _, dst := range []netip.Addr{
		ospfv3transport.AllSPFRouters,  // ff02::5
		ospfv3transport.AllDRouters,    // ff02::6
		netip.MustParseAddr("fe80::2"), // neighbor link-local unicast (DBD/LSU-retransmit)
	} {
		if !sa.Sel.Dst.Contains(dst.AsSlice()) {
			t.Errorf("SA selector dst %v does not cover OSPF destination %v", sa.Sel.Dst, dst)
		}
	}
}

func TestIPsecPoliciesInterfaceScopedWildcard(t *testing.T) {
	// FIX-1/FIX-2: every policy is the ::/0 wildcard with the OSPF proto-89 selector
	// and IfIndex scoped to the interface, so IPsec applies ONLY on this interface
	// (a plain non-IPsec OSPFv3 interface on the same node is unaffected).
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{espIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")

	// RFC requirement: RFC4552-6-1 positive -- IPsec in transport mode is supported: every installed
	// policy is Mode == ModeTransport (asserted in the loop; buildIPsecPolicies, ipsec_install.go:441).
	// RFC requirement: RFC4552-6-2 positive -- multiple SPDs with interface-based selection: each policy
	// is scoped by IfIndex (asserted in the loop; buildIPsecPolicies IfIndex, ipsec_install.go:444), so
	// the per-interface policy set is the interface-selected SPD.
	// RFC requirement: RFC4552-6-3 positive -- source, destination, protocol and direction are all used
	// as selectors: each policy carries wildcard Src/Dst, UpperProto 89, and a Dir (asserted in the loop
	// and the direction-completeness check; buildIPsecPolicies, ipsec_install.go:436-445).
	// RFC requirement: RFC4552-6-4 positive -- inbound packets are tagged with the arrival interface: the
	// in/out/fwd policies (including the inbound one) carry IfIndex == testIfIndex (asserted in the loop;
	// ipsec_install.go:444-449), the interface selector the kernel uses to tag arrivals.
	// RFC requirement: RFC4552-11-1 positive -- the IPsec barrier is around all OSPF traffic: the out/in/
	// fwd proto-89 policies cover every inbound and outbound OSPF flow on the interface (all three
	// directions asserted below; buildIPsecPolicies, ipsec_install.go:447-451).
	dirs := map[dataplane.SADir]bool{}
	for _, p := range fake.pols {
		dirs[p.Dir] = true
		if p.UpperProto != ospfv3transport.Protocol {
			t.Errorf("policy dir=%d UpperProto = %d, want %d (OSPF)", p.Dir, p.UpperProto, ospfv3transport.Protocol)
		}
		if p.IfIndex != testIfIndex {
			t.Errorf("policy dir=%d IfIndex = %d, want %d (RFC 4552 §6 interface scope)", p.Dir, p.IfIndex, testIfIndex)
		}
		if p.Mode != dataplane.ModeTransport {
			t.Errorf("policy dir=%d mode = %d, want transport", p.Dir, p.Mode)
		}
		assertWildcardV6(t, "policy src", p.Src)
		assertWildcardV6(t, "policy dst", p.Dst)
	}
	for _, d := range []dataplane.SADir{dataplane.SADirOut, dataplane.SADirIn, dataplane.SADirFwd} {
		if !dirs[d] {
			t.Errorf("missing policy direction %d (want out/in/fwd)", d)
		}
	}
}

// assertWildcardV6 checks that n is the IPv6 ::/0 prefix (any address).
func assertWildcardV6(t *testing.T, what string, n *net.IPNet) {
	t.Helper()
	if n == nil {
		t.Fatalf("%s is nil, want ::/0 wildcard", what)
	}
	ones, bits := n.Mask.Size()
	if ones != 0 || bits != 128 || !n.IP.Equal(net.IPv6zero) {
		t.Errorf("%s = %v, want ::/0 wildcard", what, n)
	}
}

func TestSAParamsSharedKey(t *testing.T) {
	// RFC 4552 §7: one configured key/SPI drives the single shared SA that both
	// protects egress and verifies ingress for the multicast group.
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{espIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")

	if len(fake.sas) != 1 {
		t.Fatalf("want 1 shared SA, got %d", len(fake.sas))
	}
	// RFC requirement: RFC4552-6-5 positive -- manually configured keys secure the specified traffic:
	// the SA is keyed from the statically configured SPI+key with no IKE (buildIPsecSA,
	// ipsec_install.go:401-424), asserted by the configured auth key and SPI flowing into the SA.
	if len(fake.sas[0].AuthKey) == 0 {
		t.Error("shared SA must carry the configured auth key (RFC 4552 §7)")
	}
	if fake.sas[0].SPI != 256 {
		t.Errorf("shared SA must use the configured SPI 256; got %d", fake.sas[0].SPI)
	}
}

func TestIPsecAHNoEncryptionParams(t *testing.T) {
	// AC-2: an AH interface installs SAs with the AH proto and no encryption key.
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{{
		Name:  "eth1",
		IPsec: &ipsecInterfaceConfig{SPI: 256, Protocol: "ah", AuthAlgo: "sha256", AuthKey: hexKey(32)},
	}})
	inst.onInterfaceUp(testIfIndex, "eth1")
	for _, sa := range fake.sas {
		if sa.Proto != dataplane.ProtoAH {
			t.Errorf("AH SA proto = %d, want %d", sa.Proto, dataplane.ProtoAH)
		}
		if len(sa.EncKey) != 0 {
			t.Errorf("AH SA must carry no encryption key, got %d bytes", len(sa.EncKey))
		}
	}
}

func TestIPsecDisabledInterfaceBypass(t *testing.T) {
	// RFC requirement: RFC4552-11-2 positive -- an interface with OSPFv3 authentication/confidentiality
	// disabled (no ipsec block) installs no require-policy, so its OSPF is bypassed by the kernel default
	// SPD: setConfig registers only interfaces that carry an ipsec block (ipsec_install.go:190-200), so
	// onInterfaceUp on a plain interface installs neither an SA nor a policy.
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{{Name: "eth1"}}) // no IPsec block => disabled
	inst.onInterfaceUp(testIfIndex, "eth1")
	if len(fake.sas) != 0 || len(fake.pols) != 0 {
		t.Fatalf("disabled interface must install nothing (SPD bypass); got %d SAs, %d policies", len(fake.sas), len(fake.pols))
	}
}

// RFC requirement: RFC4303-2-1 positive -- RFC 4303 Section 2: "The (outer) protocol header
// (IPv4, IPv6, or Extension) that immediately precedes the ESP header SHALL contain the value
// 50 in its Protocol (IPv4) or Next Header (IPv6, Extension) field". An esp interface builds
// an SA whose protocol number is 50, which is what makes the kernel write 50 in the header
// preceding ESP (buildIPsecSA -> ipsecProtoNumber, ipsec_install.go).
// RFC requirement: RFC4303-2-1 negative -- the number tracks the configured protocol rather
// than being 50 for everything: an ah interface builds an SA carrying 51, so a blanket-50
// installer fails this test.
func TestIPsecSAProtocolNumber(t *testing.T) {
	esp := buildIPsecSA(testIfIndex, ipsecInterfaceConfig{
		SPI: 256, Protocol: "esp", AuthAlgo: "sha256", AuthKey: hexKey(32),
	})
	if esp.Proto != 50 {
		t.Errorf("esp SA proto = %d, want 50 (RFC 4303 Section 2)", esp.Proto)
	}
	ah := buildIPsecSA(testIfIndex, ipsecInterfaceConfig{
		SPI: 256, Protocol: "ah", AuthAlgo: "sha256", AuthKey: hexKey(32),
	})
	if ah.Proto != 51 {
		t.Errorf("ah SA proto = %d, want 51; the ESP value 50 is chosen for ESP only", ah.Proto)
	}
}

// RFC requirement: RFC4303-2.1-2 positive -- RFC 4303 Section 2.1: "The indication of whether
// source and destination address matching is required to map inbound IPsec traffic to SAs
// MUST be set either as a side effect of manual SA configuration or via negotiation using an
// SA management protocol". The RFC 4552 manual configuration sets it: buildIPsecSA gives the
// state an explicit selector, so the indication is a configured value and never a default the
// kernel supplies.
// RFC requirement: RFC4303-2.1-2 negative -- the indication that is set is a narrowing one. The
// selector names OSPF (upper protocol 89), so traffic other than OSPF cannot map to this SA;
// an installer that left the selector at the wildcard 0 fails this test.
func TestIPsecSAAddressMatchIndication(t *testing.T) {
	sa := buildIPsecSA(testIfIndex, ipsecInterfaceConfig{
		SPI: 256, Protocol: "esp", AuthAlgo: "sha256", AuthKey: hexKey(32),
	})
	if sa.Sel == nil {
		t.Fatal("manual SA carries no state selector; the address-match indication is unset")
	}
	if sa.Sel.Src == nil || sa.Sel.Dst == nil {
		t.Fatalf("state selector src/dst = (%v, %v), want the configured ::/0 prefixes", sa.Sel.Src, sa.Sel.Dst)
	}
	if ones, _ := sa.Sel.Src.Mask.Size(); ones != 0 {
		t.Errorf("selector src prefix = /%d, want /0: RFC 4552 Section 7 keys one SA for every OSPFv3 address", ones)
	}
	if sa.Sel.UpperProto != ospfv3transport.Protocol {
		t.Errorf("selector upper protocol = %d, want %d (OSPF); a wildcard would map non-OSPF traffic to this SA",
			sa.Sel.UpperProto, ospfv3transport.Protocol)
	}
}

func TestIPsecLoadsXFRMBackend(t *testing.T) {
	// A-8: the default dataplane source loads/gets the xfrm backend even when IKE
	// has not loaded it (the backend is registered in the dataplane package init).
	dp, err := defaultDataplaneSource()
	if err != nil || dp == nil {
		t.Fatalf("defaultDataplaneSource() = (%v, %v), want a backend", dp, err)
	}
}
