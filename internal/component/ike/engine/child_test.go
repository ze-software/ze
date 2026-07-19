package engine

import (
	"fmt"
	"net"
	"syscall"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/crypto"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/dataplane"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
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

	if len(dp.sas) != 2 {
		t.Fatalf("installed SAs = %d, want 2 (inbound + outbound)", len(dp.sas))
	}
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
func TestGenerateESPSPI(t *testing.T) {
	seen := make(map[uint32]bool)
	for range 100 {
		spi, err := GenerateESPSPI()
		if err != nil {
			t.Fatalf("GenerateESPSPI: %v", err)
		}
		// RFC requirement: RFC3948-2.1-1 negative -- the generator NEVER emits the forbidden
		// zero SPI (zero is reserved for the Non-ESP Marker that distinguishes IKE from ESP on
		// port 4500, RFC 3948 S2.1); a produced spi == 0 fails the test.
		if spi == 0 {
			t.Fatal("SPI must not be zero (RFC 4303)")
		}
		seen[spi] = true
	}
	if len(seen) < 90 {
		t.Errorf("too many SPI collisions: %d unique out of 100", len(seen))
	}
}

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

func TestNarrowTS(t *testing.T) {
	wide := &net.IPNet{IP: net.ParseIP("10.0.0.0").To4(), Mask: net.CIDRMask(8, 32)}
	narrow := &net.IPNet{IP: net.ParseIP("10.1.0.0").To4(), Mask: net.CIDRMask(16, 32)}

	result := narrowTS(narrow, wide)
	if result == nil {
		t.Fatal("narrowTS returned nil for subset")
	}
	ones, _ := result.Mask.Size()
	if ones != 16 {
		t.Errorf("narrowTS prefix = /%d, want /16", ones)
	}

	disjoint := &net.IPNet{IP: net.ParseIP("192.168.0.0").To4(), Mask: net.CIDRMask(16, 32)}
	result = narrowTS(disjoint, wide)
	if result != nil {
		t.Error("narrowTS should return nil for disjoint networks")
	}
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
