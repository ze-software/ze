// VALIDATES: spec-ospf-ext-16 AC-7/AC-10/AC-11 + R-1/R-2/R-5/A-3/A-8 -- the IPsec
// installer builds the RFC 4552 transport-mode SAs/policies from the interface
// link-local, installs on interface-up before the first Hello, removes on down,
// and reconciles a changed SPI/key.
// PREVENTS: an interface that appears protected but has no kernel SA, and a stale
// SA surviving a rekey.

package ospf

import (
	"bytes"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// fakeDP records what the installer asks of the kernel. Safe for concurrent use: the
// interface state machine's dead-interval expiry removes a neighbor SA from its own
// goroutine while a test reads the record (ipsec_neighbor_test.go).
type fakeDP struct {
	mu          sync.Mutex
	sas         []dataplane.SAParams
	pols        []dataplane.SPParams
	removedSAs  []uint32
	removedDsts []netip.Addr
	removedPols []dataplane.SPParams
	// removeSAErr and removePolErr are what every RemoveSA and RemovePolicyParams call
	// answers after recording it; nil is a delete the kernel accepted.
	removeSAErr  error
	removePolErr error
}

// failRemovals makes every later delete answer the given errors.
func (f *fakeDP) failRemovals(saErr, policyErr error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removeSAErr = saErr
	f.removePolErr = policyErr
}

func (f *fakeDP) RequiresForwardPolicy() bool { return false }

func (f *fakeDP) InstallSA(p dataplane.SAParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sas = append(f.sas, p)
	return nil
}

func (f *fakeDP) RemoveSA(spi uint32, dst net.IP, _ uint8) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removedSAs = append(f.removedSAs, spi)
	addr, ok := netip.AddrFromSlice(dst)
	if !ok {
		panic("BUG: fakeDP.RemoveSA: dst is not an IP address")
	}
	f.removedDsts = append(f.removedDsts, addr)
	return f.removeSAErr
}

func (f *fakeDP) InstallPolicy(p dataplane.SPParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pols = append(f.pols, p)
	return nil
}

func (f *fakeDP) RemovePolicyParams(p dataplane.SPParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removedPols = append(f.removedPols, p)
	return f.removePolErr
}

// installedTo reports the direction of the installed SA keyed on dst, and whether one is.
func (f *fakeDP) installedTo(dst netip.Addr) (dataplane.SADir, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for index := range f.sas {
		if addr, ok := netip.AddrFromSlice(f.sas[index].Dst); ok && addr == dst {
			return f.sas[index].Dir, true
		}
	}
	return 0, false
}

// removedTo reports whether an SA keyed on dst was removed.
func (f *fakeDP) removedTo(dst netip.Addr) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Contains(f.removedDsts, dst)
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

	// RFC 4552 §7: one state per destination the interface has when it opens (ff02::5,
	// ff02::6 and its own link-local), all on the same SPI and key, plus out/in/fwd
	// require-policies. A neighbor's state arrives with its Hello (ipsec_neighbor_test.go).
	// RFC requirement: RFC4552-3-2 positive -- an esp interface installs kernel SAs (Proto
	// resolves to ProtoESP), so ESP is supported (ipsecProtoNumber, ipsec_install.go).
	if len(fake.sas) != 3 {
		t.Fatalf("InstallSA count = %d, want 3 (ff02::5, ff02::6, link-local)", len(fake.sas))
	}
	for index := range fake.sas {
		if fake.sas[index].Proto != dataplane.ProtoESP {
			t.Fatalf("SA %d Proto = %d, want ESP (%d)", index, fake.sas[index].Proto, dataplane.ProtoESP)
		}
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
	if len(fake.removedSAs) != 3 {
		t.Errorf("RemoveSA count = %d, want 3 (ff02::5, ff02::6, link-local)", len(fake.removedSAs))
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

	if len(fake.removedSAs) != 3 {
		t.Errorf("rekey RemoveSA count = %d, want 3 (the old SPI's three states removed)", len(fake.removedSAs))
	}
	if len(fake.sas) != 6 {
		t.Errorf("total InstallSA = %d, want 6 (3 old + 3 new)", len(fake.sas))
	}
	st, ok := inst.status("eth1")
	if !ok || st.SPI != 512 {
		t.Errorf("status after rekey = %+v, want spi 512", st)
	}
}

func TestIPsecSAPerDestinationWithOSPFSelector(t *testing.T) {
	// The kernel resolves a transport-mode state by the flow's destination and nothing
	// widens that, so the interface installs one state per OSPF destination it has when
	// it opens: ff02::5 and ff02::6, which carry both directions, and its own link-local,
	// which is inbound. Each keeps the {::/0, ::/0, proto 89} selector and wildcards only
	// the SOURCE, which __xfrm6_state_addr_check honors.
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{espIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")

	// RFC requirement: RFC4552-7-1 positive -- one manually configured SPI/key drives every
	// state of the interface, and the two multicast states carry NO direction: the same
	// state protects what this router sends to the group and verifies what a neighbor sent
	// to it (buildIPsecInterfaceSAs, ipsecSharedDir, ipsec_install.go).
	want := map[netip.Addr]dataplane.SADir{
		ospfv3transport.AllSPFRouters:  ipsecSharedDir,
		ospfv3transport.AllDRouters:    ipsecSharedDir,
		netip.MustParseAddr("fe80::1"): dataplane.SADirIn,
	}
	if len(fake.sas) != len(want) {
		t.Fatalf("want %d states, got %d: %+v", len(want), len(fake.sas), fake.sas)
	}
	for index := range fake.sas {
		sa := &fake.sas[index]
		dst, ok := netip.AddrFromSlice(sa.Dst)
		if !ok {
			t.Fatalf("SA %d Dst %v is not an address", index, sa.Dst)
		}
		dir, expected := want[dst]
		if !expected {
			t.Fatalf("SA %d Dst %s is not an OSPF destination of the interface", index, dst)
		}
		delete(want, dst)
		if sa.Dir != dir {
			t.Errorf("SA to %s Dir = %v, want %v", dst, sa.Dir, dir)
		}
		if sa.SPI != 256 {
			t.Errorf("SA to %s SPI = %d, want 256 (one SPI for every state)", dst, sa.SPI)
		}
		if !sa.Src.Equal(net.IPv6zero) {
			t.Errorf("SA to %s Src = %v, want :: (the source alone is wildcarded)", dst, sa.Src)
		}
		// RFC requirement: RFC4301-4.1-1 positive -- Ze supports both IPsec modes; this asserts the
		// transport-mode half: every OSPFv3 (RFC 4552) SA is installed with Mode == ModeTransport
		// (the tunnel-mode half is TestChildSAInstallsInDataplane, the IKE Child SA).
		// RFC requirement: RFC4552-2-2 positive -- transport-mode SA MUST be supported: every
		// installed OSPFv3 SA carries Mode == ModeTransport (buildIPsecSA Mode=ModeTransport,
		// ipsec_install.go), and there is no tunnel-mode SA path for OSPFv3 to reject.
		if sa.Mode != dataplane.ModeTransport {
			t.Errorf("SA to %s mode = %d, want transport (RFC 4552 §2)", dst, sa.Mode)
		}
		if sa.ReqID != ipsecReqIDBase+uint32(testIfIndex) {
			t.Errorf("SA to %s reqid = %d, want per-interface base+ifindex %d", dst, sa.ReqID, ipsecReqIDBase+uint32(testIfIndex))
		}
		if sa.Sel == nil {
			t.Fatalf("SA to %s carries no state selector, so a non-OSPF flow could resolve it", dst)
		}
		if sa.Sel.UpperProto != ospfv3transport.Protocol {
			t.Errorf("SA to %s selector UpperProto = %d, want %d (OSPF)", dst, sa.Sel.UpperProto, ospfv3transport.Protocol)
		}
		assertWildcardV6(t, "SA selector src", sa.Sel.Src)
		assertWildcardV6(t, "SA selector dst", sa.Sel.Dst)
	}
	if len(want) != 0 {
		t.Fatalf("destinations with no state: %v", want)
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
	// RFC 4552 §7: one configured key/SPI drives every state of the interface, the
	// multicast ones protecting egress and verifying ingress alike.
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{espIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")

	if len(fake.sas) != 3 {
		t.Fatalf("want 3 states (ff02::5, ff02::6, link-local), got %d", len(fake.sas))
	}
	// RFC requirement: RFC4552-6-5 positive -- manually configured keys secure the specified traffic:
	// every state is keyed from the statically configured SPI+key with no IKE (buildIPsecSA,
	// ipsec_install.go), asserted by the configured auth key and SPI flowing into each SA.
	for index := range fake.sas {
		if len(fake.sas[index].AuthKey) == 0 {
			t.Errorf("SA to %v must carry the configured auth key (RFC 4552 §7)", fake.sas[index].Dst)
		}
		if fake.sas[index].SPI != 256 {
			t.Errorf("SA to %v must use the configured SPI 256; got %d", fake.sas[index].Dst, fake.sas[index].SPI)
		}
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
// RFC requirement: RFC4302-2-1 positive -- RFC 4302 Section 2: "The protocol header (IPv4,
// IPv6, or IPv6 Extension) immediately preceding the AH header SHALL contain the value 51 in
// its Protocol (IPv4) or Next Header (IPv6, Extension) fields [DH98]." An ah interface builds
// an SA whose protocol number is 51, which is the value that makes the kernel write 51 in the
// header preceding AH (buildIPsecSA -> ipsecProtoNumber, ipsec_install.go).
// RFC requirement: RFC4302-2-1 negative -- 51 is written for AH alone: an esp interface builds
// an SA carrying 50, so an installer writing a blanket 51 fails this test.
func TestIPsecSAProtocolNumber(t *testing.T) {
	esp := buildIPsecSA(testIfIndex, ospfv3transport.AllSPFRouters, ipsecSharedDir, ipsecInterfaceConfig{
		SPI: 256, Protocol: "esp", AuthAlgo: "sha256", AuthKey: hexKey(32),
	})
	if esp.Proto != 50 {
		t.Errorf("esp SA proto = %d, want 50 (RFC 4303 Section 2)", esp.Proto)
	}
	ah := buildIPsecSA(testIfIndex, ospfv3transport.AllSPFRouters, ipsecSharedDir, ipsecInterfaceConfig{
		SPI: 256, Protocol: "ah", AuthAlgo: "sha256", AuthKey: hexKey(32),
	})
	if ah.Proto != 51 {
		t.Errorf("ah SA proto = %d, want 51; the ESP value 50 is chosen for ESP only", ah.Proto)
	}
}

// TestIPsecSAAddressMatchIndication checks the state selector buildIPsecSA gives a manual SA,
// which narrows the traffic that may use the SA once the inbound lookup has found it: the
// ::/0 prefixes RFC 4552 Section 7 keys one SA for every OSPFv3 address, and upper protocol
// 89, so traffic other than OSPF cannot use this SA. The lookup key itself (SPI and
// destination) is RFC 4303 Section 2.1's address-match indication, proven by
// TestRFC4303InboundESPSAKeyedOnConfiguredDestination and the rfc4303 XFRM probes.
func TestIPsecSAAddressMatchIndication(t *testing.T) {
	sa := buildIPsecSA(testIfIndex, ospfv3transport.AllSPFRouters, ipsecSharedDir, ipsecInterfaceConfig{
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

// TestIPsecSAReplayWindow checks that the interface's replay-window leaf is what decides
// the anti-replay state of the SA Ze installs. The method is buildIPsecSA, because
// SAParams.ReplayWin is the field xfrmStateFromParams (ike/dataplane/xfrm_linux.go) turns
// into the kernel window, and it sets one only when ReplayWin is non-zero.
//
// RFC requirement: RFC4302-3.4.3-1 positive -- RFC 4302 Section 3.4.3: "All AH
// implementations MUST support the anti-replay service, though its use may be enabled or
// disabled by the receiver on a per-SA basis." An interface configuring replay-window 64
// builds an SA carrying 64, so the receiver CAN enable the service on that SA. ESP carries
// it on the same terms (RFC 4303 Section 3.4.3), so both protocols are checked.
// RFC requirement: RFC4302-3.4.3-1 negative -- the choice is per SA and it works in the
// other direction too: an interface that configures no window builds an SA carrying 0, so
// anti-replay stays DISABLED for it. An installer that enabled anti-replay for every SA,
// or that ignored the leaf, fails this test.
func TestIPsecSAReplayWindow(t *testing.T) {
	for _, proto := range []string{ipsecProtoAH, ipsecProtoESP} {
		enabled := buildIPsecSA(testIfIndex, ospfv3transport.AllSPFRouters, ipsecSharedDir, ipsecInterfaceConfig{
			SPI: 256, Protocol: proto, AuthAlgo: "sha256", AuthKey: hexKey(32), ReplayWindow: 64,
		})
		if enabled.ReplayWin != 64 {
			t.Errorf("%s SA ReplayWin = %d, want 64: the configured window must reach the kernel SA",
				proto, enabled.ReplayWin)
		}
		disabled := buildIPsecSA(testIfIndex, ospfv3transport.AllSPFRouters, ipsecSharedDir, ipsecInterfaceConfig{
			SPI: 256, Protocol: proto, AuthAlgo: "sha256", AuthKey: hexKey(32),
		})
		if disabled.ReplayWin != 0 {
			t.Errorf("%s SA ReplayWin = %d with no replay-window configured, want 0: RFC 4302 Section 5 says "+
				"a manually keyed SA SHOULD NOT carry anti-replay unless the operator asks for it",
				proto, disabled.ReplayWin)
		}
	}
}

// warnLines answers the lines of a text log written at WARN.
func warnLines(log string) []string {
	var found []string
	for line := range strings.SplitSeq(log, "\n") {
		if strings.Contains(line, "level=WARN") {
			found = append(found, line)
		}
	}
	return found
}

// warnSinkFor points the installer's log at a buffer that keeps WARN and above.
func warnSinkFor(inst *ipsecInstaller) *bytes.Buffer {
	sink := &bytes.Buffer{}
	inst.log = slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return sink
}

// assertRemovalWarnings checks that every WARN line names why the entry matters, the
// entry-left doctor code and the interface, that each line has its error event raised
// under the same code with the entry as subject and the delete of that entry as fix,
// that the raises name every subject in subjects, and that no line or raise carries the
// interface's key.
func assertRemovalWarnings(t *testing.T, log string, lines []string, raised *recordedIssues, subjects []string) {
	t.Helper()
	for _, line := range lines {
		for _, part := range []string{
			`why="` + ipsecRemovalWhy + `"`, "code=" + codeOSPFv3IPsecEntryLeft, "interface=eth1", "report=queued",
		} {
			if !strings.Contains(line, part) {
				t.Errorf("WARN line lacks %s:\n%s", part, line)
			}
		}
	}
	if strings.Contains(log, hexKey(32)) {
		t.Fatalf("a removal WARN prints the interface's authentication key:\n%s", log)
	}
	issues := raised.withCode(codeOSPFv3IPsecEntryLeft)
	if len(issues) != len(lines) {
		t.Fatalf("entry-left raises = %d, want %d (one per WARN line): %+v", len(issues), len(lines), issues)
	}
	var seen []string
	for _, issue := range issues {
		seen = append(seen, issue.Subject)
		if issue.Severity != rpc.ReportError {
			t.Errorf("raise %q severity = %q, want %q", issue.Subject, issue.Severity, rpc.ReportError)
		}
		stateFix := "ip -6 xfrm state delete " + issue.Subject
		policyFix := "ip -6 xfrm policy delete " + issue.Subject
		if issue.Fix != stateFix && issue.Fix != policyFix {
			t.Errorf("raise %q fix = %q, want the delete of that entry", issue.Subject, issue.Fix)
		}
		if !strings.Contains(log, `msg="`+issue.Message+`"`) {
			t.Errorf("raise %q message %q is on no WARN line", issue.Subject, issue.Message)
		}
		if strings.Contains(issue.Subject+issue.Message+issue.Fix, hexKey(32)) {
			t.Fatalf("a removal raise carries the interface's authentication key: %+v", issue)
		}
	}
	for _, subject := range subjects {
		if !slices.Contains(seen, subject) {
			t.Errorf("no raise names entry %q; raised %q", subject, seen)
		}
	}
}

// VALIDATES: spec-dropped-error-6 AC-3 -- a kernel delete the installer cannot complete
// reaches the operator at WARN, one line per entry left in the kernel, naming the entry,
// why it matters and the command that deletes it, under the entry-left doctor code, and
// raises one error event per entry through the SDK publisher, subject the entry.
// Method: protect an interface and one neighbor, make every delete fail with EPERM, take
// the interface down, and read the WARN lines and the raises a recording publisher kept.
// PREVENTS: Ze-installed XFRM state staying in the kernel with only a debug line, which
// the default level hides; a leftover entry ze doctor never lists, because the line
// named the XFRM-availability code and nothing raised it; a policy fix naming the
// device, which stops resolving once the interface is deleted; and a removal WARN that
// prints the key.
func TestOSPFIPsecRemovalFailureIsReported(t *testing.T) {
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	sink := warnSinkFor(inst)
	raised := recordIssues(t)
	inst.setConfig([]interfaceConfig{espIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")
	inst.onNeighborSeen("eth1", ridMust(t, "10.0.0.2"), netip.MustParseAddr("fe80::2"))
	fake.failRemovals(fmt.Errorf("xfrm: state del spi=256: %w", syscall.EPERM),
		fmt.Errorf("xfrm: policy del: %w", syscall.EPERM))

	inst.onInterfaceDown(testIfIndex, "eth1")

	log := sink.String()
	lines := warnLines(log)
	// Three policies, the three interface states and the neighbor's state.
	if len(lines) != 7 {
		t.Fatalf("WARN lines = %d, want 7 (3 policies, 3 interface states, 1 neighbor state):\n%s", len(lines), log)
	}
	for _, want := range []string{
		`msg="OSPF IPsec could not remove the out security policy of interface eth1"`,
		`fix="ip -6 xfrm policy delete dir out src ::/0 dst ::/0 proto 89 dev if3"`,
		`fix="ip -6 xfrm policy delete dir fwd src ::/0 dst ::/0 proto 89 dev if3"`,
		`msg="OSPF IPsec could not remove the kernel security association of interface eth1"`,
		`fix="ip -6 xfrm state delete dst ff02::5 proto esp spi 256"`,
		`msg="OSPF IPsec could not remove the kernel security association of neighbor 10.0.0.2 on interface eth1"`,
		`fix="ip -6 xfrm state delete dst fe80::2 proto esp spi 256"`,
	} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %s:\n%s", want, log)
		}
	}
	assertRemovalWarnings(t, log, lines, raised, []string{
		"dir out src ::/0 dst ::/0 proto 89 dev if3",
		"dir in src ::/0 dst ::/0 proto 89 dev if3",
		"dir fwd src ::/0 dst ::/0 proto 89 dev if3",
		"dst ff02::5 proto esp spi 256",
		"dst ff02::6 proto esp spi 256",
		"dst fe80::1 proto esp spi 256",
		"dst fe80::2 proto esp spi 256",
	})
}

// VALIDATES: spec-dropped-error-6 AC-3 -- a neighbor's state that the kernel refuses to
// delete when the adjacency drops is reported at WARN with the neighbor and the command.
// Method: protect a neighbor, make its delete fail, report the neighbor lost.
// PREVENTS: removeNeighborSALocked hiding the failure that the interface path reports.
func TestOSPFIPsecNeighborRemovalFailureIsReported(t *testing.T) {
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	sink := warnSinkFor(inst)
	raised := recordIssues(t)
	inst.setConfig([]interfaceConfig{espIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")
	neighbor := ridMust(t, "10.0.0.2")
	inst.onNeighborSeen("eth1", neighbor, netip.MustParseAddr("fe80::2"))
	fake.failRemovals(fmt.Errorf("xfrm: state del spi=256: %w", syscall.EPERM), nil)

	inst.onNeighborLost("eth1", neighbor)

	log := sink.String()
	lines := warnLines(log)
	if len(lines) != 1 {
		t.Fatalf("WARN lines = %d, want 1 for the neighbor's state:\n%s", len(lines), log)
	}
	for _, want := range []string{
		`msg="OSPF IPsec could not remove the kernel security association of neighbor 10.0.0.2 on interface eth1"`,
		`fix="ip -6 xfrm state delete dst fe80::2 proto esp spi 256"`,
	} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("WARN line lacks %s:\n%s", want, lines[0])
		}
	}
	assertRemovalWarnings(t, log, lines, raised, []string{"dst fe80::2 proto esp spi 256"})
}

// VALIDATES: spec-dropped-error-6 A-2 -- a delete the kernel answers with "no such
// entry" (ESRCH for a state, ENOENT for a policy) is the outcome the delete asked for,
// so it writes nothing at WARN and raises nothing, while the record is still forgotten.
// Method: make every delete answer the absent-entry errno, take the interface down.
// PREVENTS: a WARN or a doctor event on every second teardown whose fix names state
// that is already gone.
func TestOSPFIPsecRemovalOfAbsentEntryIsQuiet(t *testing.T) {
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	sink := warnSinkFor(inst)
	raised := recordIssues(t)
	inst.setConfig([]interfaceConfig{espIface(256)})
	inst.onInterfaceUp(testIfIndex, "eth1")
	inst.onNeighborSeen("eth1", ridMust(t, "10.0.0.2"), netip.MustParseAddr("fe80::2"))
	fake.failRemovals(fmt.Errorf("xfrm: state del spi=256: %w", syscall.ESRCH),
		fmt.Errorf("xfrm: policy del: %w", syscall.ENOENT))

	inst.onInterfaceDown(testIfIndex, "eth1")

	if lines := warnLines(sink.String()); len(lines) != 0 {
		t.Fatalf("WARN lines = %d for entries the kernel no longer held, want 0:\n%s", len(lines), sink.String())
	}
	if issues := raised.withCode(codeOSPFv3IPsecEntryLeft); len(issues) != 0 {
		t.Fatalf("entry-left raises = %d for entries the kernel no longer held, want 0: %+v", len(issues), issues)
	}
	if _, ok := inst.status("eth1"); ok {
		t.Error("status still reports IPsec after a teardown the kernel answered with absent entries")
	}
}
