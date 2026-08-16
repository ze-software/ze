package engine

import (
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestIsXFRMUnsupported(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"ENOPROTOOPT", syscall.ENOPROTOOPT, true},
		{"EPROTONOSUPPORT", syscall.EPROTONOSUPPORT, true},
		{"EAFNOSUPPORT", syscall.EAFNOSUPPORT, true},
		{"ENOSYS", syscall.ENOSYS, true},
		{"ErrNotSupported", dataplane.ErrNotSupported, true},
		{"wrapped EPROTONOSUPPORT", fmt.Errorf("xfrm: state add spi=1: %w", syscall.EPROTONOSUPPORT), true},
		{"wrapped ErrNotSupported", fmt.Errorf("child-sa: install inbound: %w", dataplane.ErrNotSupported), true},
		{"EPERM", syscall.EPERM, false},
		{"EINVAL", syscall.EINVAL, false},
		{"generic error", fmt.Errorf("something else"), false},
		{"string match", fmt.Errorf("protocol not supported"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isXFRMUnsupported(tt.err)
			if got != tt.want {
				t.Errorf("isXFRMUnsupported(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

type mockDP struct {
	sas      []dataplane.SAParams
	policies []dataplane.SPParams
	removed  []uint32
}

func (m *mockDP) InstallSA(p dataplane.SAParams) error {
	m.sas = append(m.sas, p)
	return nil
}

func (m *mockDP) RemoveSA(spi uint32, _ net.IP, _ uint8) error {
	m.removed = append(m.removed, spi)
	return nil
}

func (m *mockDP) InstallPolicy(p dataplane.SPParams) error {
	m.policies = append(m.policies, p)
	return nil
}

func (m *mockDP) RemovePolicy(_, _ *net.IPNet, _ dataplane.SADir) error { return nil }
func (m *mockDP) RemovePolicyParams(_ dataplane.SPParams) error         { return nil }
func (m *mockDP) ListSAs(_ uint32) ([]dataplane.SAInfo, error)          { return nil, nil }
func (m *mockDP) ListPolicies() ([]dataplane.PolicyInfo, error)         { return nil, nil }
func (m *mockDP) Close() error                                          { return nil }

func testSA() *SA {
	return &SA{
		PeerName:    "test-peer",
		LocalNonce:  make([]byte, 32),
		RemoteNonce: make([]byte, 32),
		Proposal: crypto.IKEProposal{
			PRF: crypto.PRFTransform{ID: crypto.PRF_HMAC_SHA2_256, KeyLength: 32, OutputLength: 32},
		},
		SKKeys: &crypto.SKKeys{
			SK_d: make([]byte, 32),
		},
		PeerCfg: ipsec.SiteToSitePeer{
			RemoteAddress: "10.0.0.2",
		},
	}
}

func testESPGroup() ipsec.ESPGroup {
	return ipsec.ESPGroup{
		Name:     "esp-default",
		Lifetime: 3600,
		Proposals: []ipsec.ESPProposal{{
			Number:     1,
			Encryption: ipsec.EncryptionAES256,
			Hash:       ipsec.HashSHA256,
		}},
	}
}

func TestChildSAKeyDerivation(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if child.Keys == nil {
		t.Fatal("child SA keys are nil")
	}
	if len(child.Keys.EncryptKeyI) == 0 {
		t.Error("initiator encrypt key is empty")
	}
	if len(child.Keys.EncryptKeyR) == 0 {
		t.Error("responder encrypt key is empty")
	}
	if len(child.Keys.IntegKeyI) == 0 {
		t.Error("initiator integrity key is empty")
	}
	if len(child.Keys.IntegKeyR) == 0 {
		t.Error("responder integrity key is empty")
	}
	child.Clear()
}

func TestChildSAInstallsInDataplane(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 42, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	// RFC requirement: RFC4301-4.5-1 positive -- automated (IKE) keying: createFirstChildSA
	// derives ESP KEYMAT from the negotiated IKE SA and installs both Child SAs with no
	// manual key (the manual-keying half is TestIPsecInstallOnInterfaceUp, RFC 4552).
	if len(dp.sas) != 2 {
		t.Fatalf("installed SAs = %d, want 2 (inbound + outbound)", len(dp.sas))
	}
	// RFC requirement: RFC4301-4.4.1-1 positive -- the install populates the SPD: two PROTECT
	// policies (inbound + outbound) are captured for the SA pair (child.go:276,:290).
	if len(dp.policies) != 2 {
		t.Fatalf("installed policies = %d, want 2 (in + out)", len(dp.policies))
	}

	inbound := dp.sas[0]
	if inbound.SPI != child.InboundSPI {
		t.Errorf("inbound SPI = %d, want %d", inbound.SPI, child.InboundSPI)
	}
	if inbound.IfID != 42 {
		t.Errorf("inbound IfID = %d, want 42", inbound.IfID)
	}

	outbound := dp.sas[1]
	if outbound.SPI != child.OutboundSPI {
		t.Errorf("outbound SPI = %d, want %d", outbound.SPI, child.OutboundSPI)
	}

	// RFC requirement: RFC4301-4.1-1 positive -- Ze supports both IPsec modes; this asserts the
	// tunnel-mode half (the transport-mode half is TestIPsecSAIsWildcardWithOSPFSelector).
	// RFC requirement: RFC4301-4.1-3 positive -- an SA to a peer is instantiated in tunnel mode:
	// each installed Child SA carries Mode == modeTunnel (child.go:224,:253).
	// RFC requirement: RFC4301-4.1-4 positive -- the inbound and outbound SA of one IKE-keyed
	// pair use the SAME mode (both modeTunnel), never a transport/tunnel mix.
	for _, s := range []struct {
		label string
		p     dataplane.SAParams
	}{{"inbound", inbound}, {"outbound", outbound}} {
		if s.p.Mode != modeTunnel {
			t.Errorf("%s SA mode = %d, want modeTunnel (%d)", s.label, s.p.Mode, modeTunnel)
		}
	}

	// RFC requirement: RFC4301-4.1-2 positive -- a security gateway supports tunnel mode: the
	// Child SA install emits tunnel-mode PROTECT policies alongside the tunnel-mode SAs
	// (child.go:281,:295).
	for i, p := range dp.policies {
		if p.Mode != modeTunnel {
			t.Errorf("policy[%d] mode = %d, want modeTunnel (%d)", i, p.Mode, modeTunnel)
		}
	}
}

func TestChildSARemoval(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	removeChildSA(child, dp, log)
	if len(dp.removed) != 2 {
		t.Errorf("removed SAs = %d, want 2", len(dp.removed))
	}
}

func TestChildSANilDataplane(t *testing.T) {
	sa := testSA()
	log := slogutil.DiscardLogger()

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, nil, log)
	if err != nil {
		t.Fatalf("createFirstChildSA with nil dp: %v", err)
	}
	if child == nil {
		t.Fatal("child SA is nil")
	}
}

func TestChildSANoESPProposals(t *testing.T) {
	sa := testSA()
	log := slogutil.DiscardLogger()

	_, err := createFirstChildSA(sa, ipsec.ESPGroup{}, "10.0.0.1", "10.0.0.2", 1, nil, log)
	if err == nil {
		t.Fatal("expected error with empty ESP proposals")
	}
}

func TestChildSANoIKEKeys(t *testing.T) {
	sa := testSA()
	sa.SKKeys = nil
	log := slogutil.DiscardLogger()

	_, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, nil, log)
	if err == nil {
		t.Fatal("expected error with nil SK keys")
	}
}

// RFC requirement: RFC3948-2.1-1 positive -- GenerateESPSPI (child.go:77 loops until spi != 0)
// yields a valid, non-zero ESP SPI on every call; 100 generated SPIs are all non-zero and
// near-unique, so a compliant SPI is what the generator produces.
// RFC requirement: RFC4303-2.1-1 positive -- same producer, same evidence: GenerateESPSPI
// (child.go:77-88 loops until spi != 0) always returns a non-zero ESP SPI, so the SPI Ze puts on
// the wire is drawn from the 1..2^32-1 range and the reserved value 0 (RFC 4303 S2.1) is never
// what the generator hands back.
func TestGenerateESPSPI(t *testing.T) {
	seen := make(map[uint32]bool)
	for range 100 {
		spi, err := generateESPSPI()
		if err != nil {
			t.Fatalf("GenerateESPSPI: %v", err)
		}
		// RFC requirement: RFC3948-2.1-1 negative -- the generator NEVER emits the forbidden
		// zero SPI (zero is reserved for the Non-ESP Marker that distinguishes IKE from ESP on
		// port 4500, RFC 3948 S2.1); a produced spi == 0 fails the test.
		// RFC requirement: RFC4303-2.1-1 negative -- the same assertion pins RFC 4303 S2.1: SPI 0
		// MUST NOT appear on the wire, and a generated spi == 0 (the reserved value) fails the
		// test, so the forbidden zero SPI is never produced for an ESP SA.
		if spi == 0 {
			t.Fatal("SPI must not be zero (RFC 4303)")
		}
		seen[spi] = true
	}
	if len(seen) < 90 {
		t.Errorf("too many SPI collisions: %d unique out of 100", len(seen))
	}
}

// RFC requirement: RFC7296-2.23-2 positive -- when NAT is detected, ESP traffic floats to UDP
// 4500: installChildSA (child.go:235-238,263-267) sets UDP encapsulation on both Child SAs to
// transport.NATTPort (4500), the same port IKE floats to (established.go:79, register.go:285).
// RFC requirement: RFC3948-2.1-2 positive -- when NAT is detected, the Child SA install sets
// the UDP-encapsulation source and destination ports to the IKE NAT-T port 4500
// (transport.NATTPort) on both the inbound and outbound SA (child.go:235-238,263-267), so
// ESP is encapsulated on the same port pair IKE floated to (register.go:285 listens on :4500).
func TestChildSANATTEncapPorts(t *testing.T) {
	sa := testSA()
	sa.NATDetected = true
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	if _, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log); err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if len(dp.sas) != 2 {
		t.Fatalf("installed SAs = %d, want 2 (inbound + outbound)", len(dp.sas))
	}
	if transport.NATTPort != 4500 {
		t.Fatalf("transport.NATTPort = %d, want 4500 (IKE NAT-T port)", transport.NATTPort)
	}
	for _, dir := range []struct {
		label string
		p     dataplane.SAParams
	}{{"inbound", dp.sas[0]}, {"outbound", dp.sas[1]}} {
		if !dir.p.UDPEncap {
			t.Errorf("%s: UDPEncap = false, want true (RFC 3948 UDP encapsulation on NAT)", dir.label)
		}
		if dir.p.UDPEncapSPort != transport.NATTPort {
			t.Errorf("%s: UDPEncapSPort = %d, want %d (must match IKE NAT-T port)", dir.label, dir.p.UDPEncapSPort, transport.NATTPort)
		}
		if dir.p.UDPEncapDPort != transport.NATTPort {
			t.Errorf("%s: UDPEncapDPort = %d, want %d (must match IKE NAT-T port)", dir.label, dir.p.UDPEncapDPort, transport.NATTPort)
		}
	}
}

// RFC requirement: RFC4303-3.4.3-1 positive -- the anti-replay window an IKE-keyed Child SA
// projects into the dataplane is 32 packets: createFirstChildSA installs both the inbound and the
// outbound ESP SA with SAParams.ReplayWin == replayWindow (child.go:42 const replayWindow = 32,
// applied at :226 and :255), meeting the RFC 4303 S3.4.3 minimum supported window of 32.
func TestChildSAReplayWindowMinimum(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	if _, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log); err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if len(dp.sas) != 2 {
		t.Fatalf("installed SAs = %d, want 2 (inbound + outbound)", len(dp.sas))
	}
	for _, s := range []struct {
		label string
		p     dataplane.SAParams
	}{{"inbound", dp.sas[0]}, {"outbound", dp.sas[1]}} {
		if s.p.ReplayWin != 32 {
			t.Errorf("%s SA ReplayWin = %d, want 32 (RFC 4303 S3.4.3 minimum anti-replay window)", s.label, s.p.ReplayWin)
		}
	}
}

// RFC requirement: RFC4303-3.4.3-2 positive -- anti-replay is never enabled on an integrity-less
// ESP SA in the IKE keying path: every Child SA createFirstChildSA installs with a non-zero
// ReplayWin (child.go:226,:255) also carries an ESP integrity transform on the SAME SAParams --
// AuthAlgo + AuthKey for a separate-algorithm SA, or an AEAD transform -- so the replay window is
// only ever set on an SA that also has ESP integrity (RFC 4303 S3.4.3).
func TestChildSAReplayRequiresIntegrity(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	if _, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log); err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if len(dp.sas) != 2 {
		t.Fatalf("installed SAs = %d, want 2 (inbound + outbound)", len(dp.sas))
	}
	sawWindow := false
	for _, s := range []struct {
		label string
		p     dataplane.SAParams
	}{{"inbound", dp.sas[0]}, {"outbound", dp.sas[1]}} {
		if s.p.ReplayWin == 0 {
			continue // anti-replay disabled: the MUST NOT does not apply to this SA
		}
		sawWindow = true
		hasIntegrity := s.p.IsAEAD || (s.p.AuthAlgo != "" && len(s.p.AuthKey) > 0)
		if !hasIntegrity {
			t.Errorf("%s SA has ReplayWin=%d but no ESP integrity (AuthAlgo=%q AuthKey=%dB AEAD=%v); "+
				"anti-replay MUST NOT be enabled without ESP integrity (RFC 4303 S3.4.3)",
				s.label, s.p.ReplayWin, s.p.AuthAlgo, len(s.p.AuthKey), s.p.IsAEAD)
		}
	}
	if !sawWindow {
		t.Fatal("no installed ESP SA carried an anti-replay window; the requirement was not exercised")
	}
}

// TestNarrowTS tested narrowTS, a function with NO non-test caller, which
// this change deletes (ai/rules/no-layering.md: delete X before implementing Y). Its
// RFC4301-4.4.2-1 tag claimed the narrowed result reached a Child SA install; no
// production path ever called it, so the tag proved nothing. The obligation is re-bound
// in the SAME change to TestNarrowedSelectorsReachTheInstalledPolicy (ts_narrow_test.go),
// which drives the narrowing engine the responder calls and then asserts the installed
// inbound policy carries its result. Coverage moves onto a live path rather than dropping.
// rfc-test-change-approved: 2026-07-31 owner standing approval for
// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, strengthening only.

// TestChildSAInboundPolicyUsesNegotiatedTS asserts the inbound SPD/SAD entry is populated
// with the Child SA's negotiated traffic selectors, not the raw tunnel endpoints.
func TestChildSAInboundPolicyUsesNegotiatedTS(t *testing.T) {
	sa := testSA()
	sa.IsInitiator = true
	sa.NegotiatedTSi = &net.IPNet{IP: net.ParseIP("10.1.0.0").To4(), Mask: net.CIDRMask(16, 32)}
	sa.NegotiatedTSr = &net.IPNet{IP: net.ParseIP("10.2.0.0").To4(), Mask: net.CIDRMask(16, 32)}
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 7, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	// As initiator, TSi is local and TSr is remote (child.go:155-164), so the negotiated
	// selectors override the raw /32 tunnel endpoints on the Child SA.
	if child.TSLocal.String() != sa.NegotiatedTSi.String() {
		t.Errorf("child TSLocal = %v, want negotiated TSi %v", child.TSLocal, sa.NegotiatedTSi)
	}
	if child.TSRemote.String() != sa.NegotiatedTSr.String() {
		t.Errorf("child TSRemote = %v, want negotiated TSr %v", child.TSRemote, sa.NegotiatedTSr)
	}
	if len(dp.policies) != 2 {
		t.Fatalf("installed policies = %d, want 2 (in + out)", len(dp.policies))
	}

	// RFC requirement: RFC4301-4.4.2-1 positive -- the inbound SAD/SPD entry is populated with
	// the negotiated traffic selectors: the inbound policy (Dir=In) carries Src == negotiated
	// TSr and Dst == negotiated TSi (child.go:276-288 over the narrowed TS from :155-164).
	inPol := dp.policies[0]
	if inPol.Dir != dataplane.SADirIn {
		t.Fatalf("policies[0] Dir = %d, want inbound (%d)", inPol.Dir, dataplane.SADirIn)
	}
	if inPol.Src.String() != sa.NegotiatedTSr.String() {
		t.Errorf("inbound policy Src = %v, want negotiated TSr %v", inPol.Src, sa.NegotiatedTSr)
	}
	if inPol.Dst.String() != sa.NegotiatedTSi.String() {
		t.Errorf("inbound policy Dst = %v, want negotiated TSi %v", inPol.Dst, sa.NegotiatedTSi)
	}
	child.Clear()
}

func TestDeleteNotification(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	inSPI := child.InboundSPI
	outSPI := child.OutboundSPI

	removeChildSA(child, dp, log)

	found := map[uint32]bool{inSPI: false, outSPI: false}
	for _, spi := range dp.removed {
		found[spi] = true
	}
	if !found[inSPI] {
		t.Error("inbound SPI not removed")
	}
	if !found[outSPI] {
		t.Error("outbound SPI not removed")
	}
}
