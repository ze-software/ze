package dataplane

import (
	"net"
	"testing"
)

type mockDataplane struct {
	installed   []SAParams
	removed     []uint32
	policies    []SPParams
	removedPols int
}

func (m *mockDataplane) InstallSA(p SAParams) error {
	m.installed = append(m.installed, p)
	return nil
}

func (m *mockDataplane) RemoveSA(spi uint32, _ net.IP, _ uint8) error {
	m.removed = append(m.removed, spi)
	return nil
}

func (m *mockDataplane) InstallPolicy(p SPParams) error {
	m.policies = append(m.policies, p)
	return nil
}

func (m *mockDataplane) RemovePolicy(_, _ *net.IPNet, _ SADir) error {
	m.removedPols++
	return nil
}

func (m *mockDataplane) RemovePolicyParams(_ SPParams) error {
	m.removedPols++
	return nil
}

func (m *mockDataplane) ListSAs(_ uint32) ([]SAInfo, error) {
	return nil, nil
}

func (m *mockDataplane) ListPolicies() ([]PolicyInfo, error) {
	return nil, nil
}

func (m *mockDataplane) Close() error { return nil }

func TestDataplaneInterface(t *testing.T) {
	var dp Dataplane = &mockDataplane{}

	err := dp.InstallSA(SAParams{
		SPI:   0x12345678,
		Src:   net.ParseIP("10.0.0.1"),
		Dst:   net.ParseIP("10.0.0.2"),
		IfID:  1,
		Proto: 50,
		Mode:  2,
	})
	if err != nil {
		t.Fatalf("InstallSA: %v", err)
	}

	err = dp.InstallPolicy(SPParams{
		Src:   &net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)},
		Dst:   &net.IPNet{IP: net.ParseIP("10.0.1.0"), Mask: net.CIDRMask(24, 32)},
		Dir:   SADirOut,
		Proto: 50,
		Mode:  2,
		IfID:  1,
	})
	if err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}

	if err = dp.RemoveSA(0x12345678, net.ParseIP("10.0.0.2"), 50); err != nil {
		t.Fatalf("RemoveSA: %v", err)
	}

	m, ok := dp.(*mockDataplane)
	if !ok {
		t.Fatal("unexpected dataplane type")
	}
	if len(m.installed) != 1 {
		t.Errorf("installed count = %d, want 1", len(m.installed))
	}
	if m.installed[0].SPI != 0x12345678 {
		t.Errorf("installed SPI = %d, want 0x12345678", m.installed[0].SPI)
	}
	if len(m.removed) != 1 || m.removed[0] != 0x12345678 {
		t.Errorf("removed = %v, want [0x12345678]", m.removed)
	}
}

func TestRegisterAndLoad(t *testing.T) {
	mu.Lock()
	saved := backends
	backends = make(map[string]func() (Dataplane, error))
	active = nil
	mu.Unlock()

	defer func() {
		mu.Lock()
		backends = saved
		active = nil
		mu.Unlock()
	}()

	err := Register("test", func() (Dataplane, error) {
		return &mockDataplane{}, nil
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	err = Register("test", func() (Dataplane, error) {
		return &mockDataplane{}, nil
	})
	if err == nil {
		t.Fatal("duplicate Register should fail")
	}

	if err := Load("test"); err != nil {
		t.Fatalf("Load: %v", err)
	}

	dp := Get()
	if dp == nil {
		t.Fatal("Get returned nil after Load")
	}

	if err := Load("nonexistent"); err == nil {
		t.Fatal("Load nonexistent should fail")
	}

	if err := CloseBackend(); err != nil {
		t.Fatalf("CloseBackend: %v", err)
	}
	if Get() != nil {
		t.Fatal("Get should return nil after Close")
	}
}

// VALIDATES: spec-ospf-ext-16 AC-2 / A-6 -- an AH SA carries an integrity
// transform only and never an encryption transform (RFC 4302).
// PREVENTS: an AH SA install that sets a Crypt transform and installs a
// malformed state.
func TestSAParamsAHNoEncryption(t *testing.T) {
	plan := planStateAlgos(SAParams{Proto: ProtoAH, AuthAlgo: "sha256", AuthKey: make([]byte, 32)})
	if !plan.Auth {
		t.Error("AH plan must set the Auth transform")
	}
	if plan.Crypt {
		t.Error("AH plan must NOT set a Crypt (encryption) transform (RFC 4302)")
	}
	if plan.AEAD {
		t.Error("AH plan must NOT set an AEAD transform")
	}
}

// VALIDATES: tunnelEndpoints fails closed. Tunnel mode needs both endpoints, and
// an unspecified 0.0.0.0 is not one. Transport mode carries none, per RFC 4301
// Section 4.4.1.2, which leaves a transport-mode template's addresses unused.
// PREVENTS: the silent 0.0.0.0 template that resolves to no state, and the reverse
// mistake where a caller supplies endpoints that transport mode then discards.
func TestTunnelEndpointsFailClosed(t *testing.T) {
	v4a := net.ParseIP("172.28.0.2")
	v4b := net.ParseIP("172.28.0.3")
	v6 := net.ParseIP("2001:db8::1")

	for _, tc := range []struct {
		name    string
		p       SPParams
		wantErr bool
		wantSrc net.IP
	}{
		{"tunnel with both endpoints", SPParams{Mode: ModeTunnel, TunnelSrc: v4a, TunnelDst: v4b}, false, v4a},
		{"tunnel IPv6 endpoints", SPParams{Mode: ModeTunnel, TunnelSrc: v6, TunnelDst: net.ParseIP("2001:db8::2")}, false, v6},
		{"tunnel with no endpoints", SPParams{Mode: ModeTunnel}, true, nil},
		{"tunnel with no source", SPParams{Mode: ModeTunnel, TunnelDst: v4b}, true, nil},
		{"tunnel with no destination", SPParams{Mode: ModeTunnel, TunnelSrc: v4a}, true, nil},
		// The exact value the defect produced. It must never look like an answer.
		{"tunnel with unspecified v4", SPParams{Mode: ModeTunnel, TunnelSrc: net.IPv4zero, TunnelDst: v4b}, true, nil},
		{"tunnel with unspecified v6", SPParams{Mode: ModeTunnel, TunnelSrc: v6, TunnelDst: net.IPv6unspecified}, true, nil},
		{"tunnel with malformed address", SPParams{Mode: ModeTunnel, TunnelSrc: net.IP{1, 2, 3}, TunnelDst: v4b}, true, nil},
		{"tunnel with mixed families", SPParams{Mode: ModeTunnel, TunnelSrc: v4a, TunnelDst: v6}, true, nil},
		{"transport with no endpoints", SPParams{Mode: ModeTransport}, false, nil},
		{"transport with endpoints", SPParams{Mode: ModeTransport, TunnelSrc: v4a, TunnelDst: v4b}, true, nil},
		{"transport with one endpoint", SPParams{Mode: ModeTransport, TunnelDst: v4b}, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src, dst, err := tunnelEndpoints(tc.p)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("accepted %+v, want rejection", tc.p)
				}
				return
			}
			if err != nil {
				t.Fatalf("tunnelEndpoints: %v", err)
			}
			if tc.wantSrc == nil {
				if src != nil || dst != nil {
					t.Errorf("endpoints = %v/%v, want none in transport mode", src, dst)
				}
				return
			}
			if !src.Equal(tc.wantSrc) || !dst.Equal(tc.p.TunnelDst) {
				t.Errorf("endpoints = %v/%v, want %v/%v", src, dst, tc.wantSrc, tc.p.TunnelDst)
			}
		})
	}
}

// VALIDATES: the OSPF caller keeps working unchanged. RFC 4552 installs transport-mode
// policies with no tunnel endpoints, which is what the guard requires of that mode.
// PREVENTS: a tunnel-endpoint guard that breaks the transport-mode IPsec-for-OSPF path.
func TestTunnelEndpointsTransportCallerUnaffected(t *testing.T) {
	_, wildcard, _ := net.ParseCIDR("::/0")
	ospf := SPParams{
		Src: wildcard, Dst: wildcard, Dir: SADirOut,
		Proto: ProtoESP, Mode: ModeTransport, ReqID: 100,
		UpperProto: 89, IfIndex: 3,
	}
	src, dst, err := tunnelEndpoints(ospf)
	if err != nil {
		t.Fatalf("transport-mode OSPF policy rejected: %v", err)
	}
	if src != nil || dst != nil {
		t.Errorf("endpoints = %v/%v, want none", src, dst)
	}
}

// VALIDATES: spec-ospf-ext-16 -- the UpperProto selector defaults to 0 (any,
// today's IKE behavior) and accepts 89 (OSPF, RFC 4552 §5/§6).
func TestSPParamsUpperProtoSelector(t *testing.T) {
	if (SPParams{}).UpperProto != 0 {
		t.Error("zero-value SPParams.UpperProto must be 0 (any)")
	}
	p := SPParams{UpperProto: 89}
	if p.UpperProto != 89 {
		t.Errorf("UpperProto = %d, want 89", p.UpperProto)
	}
}

// VALIDATES: spec-ospf-ext-16 R-3 / AC-14 -- the additive AH/UpperProto changes
// leave the IKE ESP callers byte-identical: zero-value new fields reproduce the
// tunnel-mode ESP algorithm plan (Crypt+Auth) and the AEAD plan.
func TestDataplaneIKEUnaffected(t *testing.T) {
	esp := planStateAlgos(SAParams{Proto: ProtoESP, EncAlgo: "aes256", AuthAlgo: "sha256"})
	if !esp.Crypt || !esp.Auth || esp.AEAD {
		t.Errorf("ESP plan = %+v, want Crypt+Auth", esp)
	}
	aead := planStateAlgos(SAParams{Proto: ProtoESP, IsAEAD: true, EncAlgo: "aes256gcm"})
	if !aead.AEAD || aead.Crypt || aead.Auth {
		t.Errorf("AEAD plan = %+v, want AEAD only", aead)
	}
	// The IKE child SA builds an SPParams with no UpperProto/IfIndex: they must
	// default to 0 (any / node-wide), the historical selector, so installed IKE
	// policies are byte-identical. IfID (the XFRM if_id) is still set by IKE and is
	// a distinct field from the new IfIndex (the policy selector oif).
	ikePolicy := SPParams{Dir: SADirOut, Proto: ProtoESP, Mode: ModeTunnel, IfID: 7, ReqID: 3}
	if ikePolicy.UpperProto != 0 {
		t.Errorf("IKE SPParams.UpperProto = %d, want 0", ikePolicy.UpperProto)
	}
	if ikePolicy.IfIndex != 0 {
		t.Errorf("IKE SPParams.IfIndex = %d, want 0 (node-wide; distinct from IfID)", ikePolicy.IfIndex)
	}
	// The IKE child SA builds an SAParams with no Sel: it must default to nil so the
	// XFRM state selector stays the zero value (no x->sel), byte-identical to before.
	ikeSA := SAParams{Proto: ProtoESP, Mode: ModeTunnel, IfID: 7, ReqID: 3, EncAlgo: "aes256", AuthAlgo: "sha256"}
	if ikeSA.Sel != nil {
		t.Errorf("IKE SAParams.Sel = %v, want nil (no explicit state selector)", ikeSA.Sel)
	}
}

// VALIDATES: kernelXFRMMode maps each Ze mode onto the kernel XFRM mode number
// from uapi/linux/xfrm.h (transport = 0, tunnel = 1), and reports failure for any
// other value rather than returning a usable default.
// PREVENTS: the off-by-one that sent ModeTunnel (2) to the kernel as
// XFRM_MODE_ROUTEOPTIMIZATION, which xfrm4_mode_map does not define, so the ESP
// state add failed with EPROTONOSUPPORT and no Child SA was ever installed.
func TestKernelXFRMMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode uint8
		want uint8
		ok   bool
	}{
		{"transport", ModeTransport, 0, true},
		{"tunnel", ModeTunnel, 1, true},
		{"unset zero value", 0, 0, false},
		{"beyond the known modes", 3, 0, false},
		{"max uint8", 255, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := kernelXFRMMode(tc.mode)
			if ok != tc.ok {
				t.Fatalf("kernelXFRMMode(%d) ok = %v, want %v", tc.mode, ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("kernelXFRMMode(%d) = %d, want %d", tc.mode, got, tc.want)
			}
		})
	}
}

// VALIDATES: the Ze mode constants stay offset from the kernel numbering, so a
// future edit cannot quietly make them coincide and reintroduce a pass-through.
// PREVENTS: an unset Mode field becoming a valid "transport" mode.
// VALIDATES: the Ze mode constants are 1-based and distinct from the kernel numbers,
// so a zero mode is never a valid one.
// PREVENTS: a renumbering that makes ModeTransport equal the kernel value, after which
// an unset Mode field would read as a valid transport-mode SA.
//
// This test compares one constant against another. It therefore says NOTHING about the
// conversion between them: reverting kernelXFRMMode to a pass-through leaves it green.
// TestKernelXFRMMode and TestXfrmPolicyFromParamsMode are the tests that gate the
// conversion, and one of them must go red for a conversion defect to be caught.
func TestModeConstantsAreNotKernelNumbers(t *testing.T) {
	if ModeTransport == kernelModeTransport || ModeTunnel == kernelModeTunnel {
		t.Errorf("Ze mode constants (%d/%d) must not equal the kernel numbers (%d/%d): "+
			"they are 1-based so that zero is never a valid mode",
			ModeTransport, ModeTunnel, kernelModeTransport, kernelModeTunnel)
	}
	if ModeTransport == 0 || ModeTunnel == 0 {
		t.Error("no Ze mode constant may be 0: zero is the unset sentinel")
	}
}

func TestSADirValues(t *testing.T) {
	if SADirIn != 1 {
		t.Errorf("SADirIn = %d, want 1", SADirIn)
	}
	if SADirOut != 2 {
		t.Errorf("SADirOut = %d, want 2", SADirOut)
	}
	if SADirFwd != 3 {
		t.Errorf("SADirFwd = %d, want 3", SADirFwd)
	}
}
