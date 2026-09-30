// VALIDATES: the RFC 4301 Section 4.4.2.1 SAD data items Ze installs into Linux XFRM,
// one item at a time: the SPI, the sequence number counter width, the anti-replay
// window, the AH and ESP transforms with their keys, and the protocol mode.
// PREVENTS: an SAD entry installed without an item the kernel needs to process the SA
// as negotiated, or with an item the kernel cannot hold silently dropped.

//go:build linux

package dataplane

import (
	"bytes"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

// TestRFC4301SADItemSPIIsTheNegotiatedValue proves the SAD entry carries the SPI the
// receiving end selected, for an inbound and an outbound SA. Method: build both
// directions and read the SPI back.
func TestRFC4301SADItemSPIIsTheNegotiatedValue(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-1 positive -- the SAD entry of an inbound and of an outbound SA carries the SPI Ze was given for it, unchanged.
	for _, dir := range []SADir{SADirIn, SADirOut} {
		params := boundarySA(0xc0de0101)
		params.Dir = dir
		state, err := xfrmStateFromParams(params)
		if err != nil {
			t.Fatalf("dir %d: xfrmStateFromParams: %v", dir, err)
		}
		if uint32(state.Spi) != 0xc0de0101 {
			t.Errorf("dir %d: spi %#x, want 0xc0de0101", dir, state.Spi)
		}
	}
}

// TestRFC4301SADItemZeroSPIIsRefused proves an SA whose SPI is zero, a value that
// identifies no SA on the wire, builds no SAD entry. Method: build an SA with SPI 0.
func TestRFC4301SADItemZeroSPIIsRefused(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-1 negative -- an SA whose SPI is zero, which identifies no SA, is refused and no SAD entry is built.
	if state, err := xfrmStateFromParams(boundarySA(0)); err == nil || state != nil {
		t.Fatalf("spi 0: got state %v err %v, want refusal and no state", state, err)
	}
}

// TestRFC4301SADItemSequenceCounterIsThe32BitCounterNegotiated proves the SAD entry
// asks the kernel for the 32-bit sequence number counter Ze negotiates (ESN transform
// "not extended") and starts that counter fresh. Method: build an SA and read the ESN
// flag and the replay state back.
func TestRFC4301SADItemSequenceCounterIsThe32BitCounterNegotiated(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-4 positive -- the SAD entry carries no ESN flag, so the kernel keeps the 32-bit counter Ze negotiated, and carries no preset replay state, so that counter starts at zero.
	state, err := xfrmStateFromParams(boundarySA(0x1000))
	if err != nil {
		t.Fatalf("xfrmStateFromParams: %v", err)
	}
	if state.ESN {
		t.Error("ESN flag set, want the 32-bit counter Ze negotiates")
	}
	if state.Replay != nil {
		t.Errorf("replay state %+v, want none so the counter starts at zero", state.Replay)
	}
}

// TestRFC4301SADItemAntiReplayWindowIsInstalled proves the SAD entry carries the
// anti-replay window Ze was given. Method: build SAs with two window sizes.
func TestRFC4301SADItemAntiReplayWindowIsInstalled(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-6 positive -- the SAD entry carries the anti-replay window size Ze was given, 32 or 64.
	for _, window := range []uint8{32, 64} {
		params := boundarySA(0x1000)
		params.ReplayWin = window
		state, err := xfrmStateFromParams(params)
		if err != nil {
			t.Fatalf("window %d: xfrmStateFromParams: %v", window, err)
		}
		if state.ReplayWindow != int(window) {
			t.Errorf("replay window %d, want %d", state.ReplayWindow, window)
		}
	}
}

// TestRFC4301SADItemAHCarriesItsIntegrityAlgorithmAndKey proves an AH SAD entry
// carries the integrity algorithm and its key, and no encryption transform. Method:
// build an AH SA and read the transforms back.
func TestRFC4301SADItemAHCarriesItsIntegrityAlgorithmAndKey(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-7 positive -- an AH SAD entry carries the integrity algorithm, its truncation and its key, and no encryption transform.
	params := boundarySA(0x1000)
	params.Proto = ProtoAH
	params.Mode = ModeTransport
	params.AuthKey = []byte("0123456789abcdef0123456789abcdef")
	state, err := xfrmStateFromParams(params)
	if err != nil {
		t.Fatalf("xfrmStateFromParams: %v", err)
	}
	if state.Proto != netlink.XFRM_PROTO_AH {
		t.Errorf("protocol %d, want AH", state.Proto)
	}
	if state.Auth == nil {
		t.Fatal("no integrity transform on an AH SA")
	}
	if state.Auth.Name != xfrmAuthSHA256 || state.Auth.TruncateLen != 128 {
		t.Errorf("integrity %s/%d, want %s/128", state.Auth.Name, state.Auth.TruncateLen, xfrmAuthSHA256)
	}
	if !bytes.Equal(state.Auth.Key, params.AuthKey) {
		t.Error("integrity key is not the key Ze was given")
	}
	if state.Crypt != nil || state.Aead != nil {
		t.Errorf("AH SA carries crypt=%v aead=%v, want neither", state.Crypt, state.Aead)
	}
}

// TestRFC4301SADItemAHUnknownIntegrityIsRefused proves an AH SA naming an integrity
// algorithm the kernel has no transform for builds no SAD entry. Method: build one.
func TestRFC4301SADItemAHUnknownIntegrityIsRefused(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-7 negative -- an AH SA whose integrity algorithm the kernel cannot hold is refused rather than installed without one.
	params := boundarySA(0x1000)
	params.Proto = ProtoAH
	params.AuthAlgo = "unknown"
	if state, err := xfrmStateFromParams(params); err == nil || state != nil {
		t.Fatalf("got state %v err %v, want refusal and no state", state, err)
	}
}

// TestRFC4301SADItemESPTransformsCarryAlgorithmAndKey proves an ESP SAD entry carries
// the encryption algorithm with its key and the integrity algorithm with its key.
// Method: build an ESP SA with distinct keys and read both transforms back.
func TestRFC4301SADItemESPTransformsCarryAlgorithmAndKey(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-8 positive -- an ESP SAD entry carries the encryption algorithm and the encryption key Ze was given.
	// RFC requirement: RFC4301-4.4.2.1-9 positive -- an ESP SAD entry carries the integrity algorithm, its truncation and the integrity key Ze was given.
	params := boundarySA(0x1000)
	params.EncKey = []byte("EEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE")
	params.AuthKey = []byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	state, err := xfrmStateFromParams(params)
	if err != nil {
		t.Fatalf("xfrmStateFromParams: %v", err)
	}
	if state.Crypt == nil || state.Auth == nil {
		t.Fatalf("transforms crypt=%v auth=%v, want both", state.Crypt, state.Auth)
	}
	if state.Crypt.Name != xfrmEncAESCBC || !bytes.Equal(state.Crypt.Key, params.EncKey) {
		t.Errorf("encryption %s key %q, want %s with the given key", state.Crypt.Name, state.Crypt.Key, xfrmEncAESCBC)
	}
	if state.Auth.Name != xfrmAuthSHA256 || state.Auth.TruncateLen != 128 {
		t.Errorf("integrity %s/%d, want %s/128", state.Auth.Name, state.Auth.TruncateLen, xfrmAuthSHA256)
	}
	if !bytes.Equal(state.Auth.Key, params.AuthKey) {
		t.Error("integrity key is not the key Ze was given")
	}
}

// TestRFC4301SADItemUnknownESPTransformIsRefused proves an ESP SA naming an encryption
// or integrity algorithm the kernel has no transform for builds no SAD entry. Method:
// build one of each.
func TestRFC4301SADItemUnknownESPTransformIsRefused(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-8 negative -- an ESP SA whose encryption algorithm the kernel cannot hold is refused rather than installed without one.
	// RFC requirement: RFC4301-4.4.2.1-9 negative -- an ESP SA whose integrity algorithm the kernel cannot hold is refused rather than installed without one.
	unknownEnc := boundarySA(0x1000)
	unknownEnc.EncAlgo = "unknown"
	if state, err := xfrmStateFromParams(unknownEnc); err == nil || state != nil {
		t.Errorf("unknown encryption: got state %v err %v, want refusal and no state", state, err)
	}
	unknownAuth := boundarySA(0x1000)
	unknownAuth.AuthAlgo = "unknown"
	if state, err := xfrmStateFromParams(unknownAuth); err == nil || state != nil {
		t.Errorf("unknown integrity: got state %v err %v, want refusal and no state", state, err)
	}
}

// TestRFC4301SADItemCombinedModeCarriesOneAEADTransform proves an ESP SA with a
// combined mode algorithm carries that algorithm with its key and its ICV length, and
// neither separate transform. Method: build an AES-GCM SA and read it back.
func TestRFC4301SADItemCombinedModeCarriesOneAEADTransform(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-10 positive -- a combined mode ESP SAD entry carries the AEAD algorithm, its key and its ICV length, and no separate encryption or integrity transform.
	params := boundarySA(0x1000)
	params.IsAEAD = true
	params.EncAlgo = "aes256gcm"
	params.EncKey = []byte("GGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGssss")
	state, err := xfrmStateFromParams(params)
	if err != nil {
		t.Fatalf("xfrmStateFromParams: %v", err)
	}
	if state.Aead == nil {
		t.Fatal("no AEAD transform on a combined mode SA")
	}
	if state.Aead.Name != xfrmAEADAESGCM || state.Aead.ICVLen != 128 {
		t.Errorf("aead %s/%d, want %s/128", state.Aead.Name, state.Aead.ICVLen, xfrmAEADAESGCM)
	}
	if !bytes.Equal(state.Aead.Key, params.EncKey) {
		t.Error("AEAD key is not the key Ze was given")
	}
	if state.Crypt != nil || state.Auth != nil {
		t.Errorf("combined mode SA carries crypt=%v auth=%v, want neither", state.Crypt, state.Auth)
	}
}

// TestRFC4301SADItemUnknownCombinedModeIsRefused proves a combined mode SA naming an
// algorithm the kernel has no AEAD transform for builds no SAD entry. Method: build one.
func TestRFC4301SADItemUnknownCombinedModeIsRefused(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-10 negative -- a combined mode ESP SA whose algorithm the kernel cannot hold is refused rather than installed without one.
	params := boundarySA(0x1000)
	params.IsAEAD = true
	params.EncAlgo = "unknown"
	if state, err := xfrmStateFromParams(params); err == nil || state != nil {
		t.Fatalf("got state %v err %v, want refusal and no state", state, err)
	}
}

// TestRFC4301SADItemModeIsInstalled proves the SAD entry carries the IPsec protocol
// mode, tunnel or transport. Method: build one SA of each mode.
func TestRFC4301SADItemModeIsInstalled(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-12 positive -- a tunnel mode SA and a transport mode SA each carry their own mode in the SAD entry.
	cases := []struct {
		mode uint8
		want netlink.Mode
	}{
		{ModeTunnel, netlink.XFRM_MODE_TUNNEL},
		{ModeTransport, netlink.XFRM_MODE_TRANSPORT},
	}
	for _, tc := range cases {
		params := boundarySA(0x1000)
		params.Mode = tc.mode
		state, err := xfrmStateFromParams(params)
		if err != nil {
			t.Fatalf("mode %d: xfrmStateFromParams: %v", tc.mode, err)
		}
		if state.Mode != tc.want {
			t.Errorf("mode %d installed as %d, want %d", tc.mode, state.Mode, tc.want)
		}
	}
}

// TestRFC4301SADItemUnknownModeIsRefused proves an SA whose mode is neither tunnel nor
// transport builds no SAD entry. Method: build one.
func TestRFC4301SADItemUnknownModeIsRefused(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-12 negative -- an SA whose mode is neither tunnel nor transport is refused and no SAD entry is built.
	params := boundarySA(0x1000)
	params.Mode = 9
	if state, err := xfrmStateFromParams(params); err == nil || state != nil {
		t.Fatalf("got state %v err %v, want refusal and no state", state, err)
	}
}

// TestRFC4301SADItemTunnelHeaderAddressesAreInstalled proves a tunnel mode SAD entry
// carries the tunnel header source and destination address, for IPv4 and for IPv6.
// Method: build one SA of each family and read the pair back.
func TestRFC4301SADItemTunnelHeaderAddressesAreInstalled(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-18 positive -- a tunnel mode SAD entry carries the tunnel header source and destination address Ze was given, both IPv4 or both IPv6.
	pairs := [][2]string{{"192.0.2.1", "198.51.100.1"}, {"2001:db8::1", "2001:db8:1::1"}}
	for _, pair := range pairs {
		params := boundarySA(0x1000)
		params.Src = net.ParseIP(pair[0])
		params.Dst = net.ParseIP(pair[1])
		state, err := xfrmStateFromParams(params)
		if err != nil {
			t.Fatalf("%s -> %s: xfrmStateFromParams: %v", pair[0], pair[1], err)
		}
		if !state.Src.Equal(params.Src) || !state.Dst.Equal(params.Dst) {
			t.Errorf("tunnel header %s -> %s, want %s -> %s", state.Src, state.Dst, pair[0], pair[1])
		}
	}
}

// TestRFC4301SADItemMixedFamilyTunnelHeaderIsRefused proves a tunnel mode SA whose
// tunnel header addresses are of two families builds no SAD entry. Method: build one.
func TestRFC4301SADItemMixedFamilyTunnelHeaderIsRefused(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2.1-18 negative -- a tunnel mode SA with an IPv4 source and an IPv6 destination in its tunnel header is refused and no SAD entry is built.
	params := boundarySA(0x1000)
	params.Dst = net.ParseIP("2001:db8:1::1")
	if state, err := xfrmStateFromParams(params); err == nil || state != nil {
		t.Fatalf("got state %v err %v, want refusal and no state", state, err)
	}
}
