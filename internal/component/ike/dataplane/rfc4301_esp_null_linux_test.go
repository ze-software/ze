// VALIDATES: RFC 4301 Section 4.2, "A compliant implementation MUST NOT allow
// instantiation of an ESP SA that employs both NULL encryption and no integrity
// algorithm." On the XFRM backend every ESP SA Ze instantiates, an IKEv2 Child SA
// (engine/child.go) and an RFC 4552 manually keyed OSPFv3 SA (ospf/ipsec_install.go)
// alike, becomes a kernel state through xfrmStateFromParams, so that is where the
// refusal has to hold. The ze_vpp backend is the second instantiation path; its
// refusal is proven in rfc4301_esp_null_vpp_test.go.
// PREVENTS: a NULL-encryption ESP SA with no integrity transform reaching the kernel
// from any path, while NULL encryption with an integrity algorithm stays installable.

//go:build linux

package dataplane

import "testing"

// RFC requirement: RFC4301-4.2-1 positive -- an ESP SA with NULL encryption and an integrity algorithm is instantiated: the SAD entry carries the null cipher and the integrity transform.
func TestRFC4301ESPNullEncryptionWithIntegrityIsInstantiated(t *testing.T) {
	params := boundarySA(0x1000)
	params.EncAlgo = "null"
	params.EncKey = nil
	params.AuthAlgo = "sha256"
	params.AuthKey = []byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	state, err := xfrmStateFromParams(params)
	if err != nil {
		t.Fatalf("NULL encryption with sha256 integrity refused: %v", err)
	}
	if state.Crypt == nil || state.Crypt.Name != xfrmEncNames["null"] {
		t.Errorf("encryption transform %v, want the null cipher %q", state.Crypt, xfrmEncNames["null"])
	}
	if state.Auth == nil || state.Auth.Name != xfrmAuthSHA256 {
		t.Errorf("integrity transform %v, want %s", state.Auth, xfrmAuthSHA256)
	}
}

// RFC requirement: RFC4301-4.2-1 negative -- an ESP SA with NULL encryption and no integrity algorithm is refused and no SAD entry is built, whether the integrity algorithm is absent or named "none".
func TestRFC4301ESPNullEncryptionWithoutIntegrityIsRefused(t *testing.T) {
	for _, integrity := range []string{"", "none"} {
		params := boundarySA(0x1000)
		params.EncAlgo = "null"
		params.EncKey = nil
		params.AuthAlgo = integrity
		params.AuthKey = nil
		if state, err := xfrmStateFromParams(params); err == nil || state != nil {
			t.Errorf("NULL encryption with integrity %q: got state %v err %v, want refusal and no state", integrity, state, err)
		}
	}
}
