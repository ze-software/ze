// Design: docs/architecture/ike/ipsec-3-data-model.md -- the SA parameters the dataplane installs
// Related: xfrm_linux.go -- xfrmStateFromParams
// Related: ../crypto/rfc4106_gcm_keymat_test.go -- the KEYMAT and salt the key carries
//
// VALIDATES: the AES-GCM-ESP state Ze hands the Linux XFRM layer. The kernel's
// rfc4106(gcm(aes)) transform generates every IV and seals every packet, so what Ze owns
// is the state: the transform name, the ICV length, and the sequence number handling that
// bounds how much one key can encrypt.
// PREVENTS: an AES-GCM SA installed with an ICV length RFC 4106 forbids, and an SA whose
// sequence counter may wrap or run 64 bits, which would let one key encrypt without bound.
//
//go:build linux

package dataplane

import (
	"bytes"
	"net"
	"testing"
)

func rfc4106GCMParams() SAParams {
	return SAParams{
		SPI:     0x3000,
		Src:     net.ParseIP("192.0.2.1"),
		Dst:     net.ParseIP("198.51.100.1"),
		Proto:   ProtoESP,
		Mode:    ModeTunnel,
		EncAlgo: "aes256gcm",
		EncKey:  bytes.Repeat([]byte{0x66}, 36),
		IsAEAD:  true,
	}
}

// TestRFC4106GCMStateCarriesTheFullICV checks the transform name and the ICV length of the
// AES-GCM state.
//
// RFC 4106 Section 6: "Implementations MUST support a full-length 16-octet ICV, and MAY
// support 8 or 12 octet ICVs, and MUST NOT support other ICV lengths."
//
// RFC 4106 Section 3.1: "The AES-GCM-ESP IV field MUST be eight octets." The kernel writes
// the IV field of every packet, so what Ze owns is the transform it names: the
// rfc4106(gcm(aes)) template carries an 8-octet IV beside the 4-octet salt at the tail of
// the key, where the generic gcm(aes) template takes a 12-octet IV from its caller.
func TestRFC4106GCMStateCarriesTheFullICV(t *testing.T) {
	// RFC requirement: RFC4106-6-1 positive -- an AES-GCM SA (aes128gcm and aes256gcm) is
	// installed as the AEAD transform rfc4106(gcm(aes)) with a 128-bit ICV.
	// RFC requirement: RFC4106-3.1-1 positive -- an AES-GCM SA (aes128gcm and aes256gcm) is
	// installed as the AEAD transform rfc4106(gcm(aes)).
	for word, octets := range map[string]int{"aes128gcm": 20, "aes256gcm": 36} {
		p := rfc4106GCMParams()
		p.EncAlgo = word
		p.EncKey = bytes.Repeat([]byte{0x66}, octets)
		state, err := xfrmStateFromParams(p)
		if err != nil {
			t.Fatalf("xfrmStateFromParams(%s): %v", word, err)
		}
		if state.Aead == nil || state.Aead.Name != "rfc4106(gcm(aes))" || state.Aead.ICVLen != 128 {
			t.Errorf("%s: installed %+v, want rfc4106(gcm(aes)) with a 128-bit ICV", word, state.Aead)
		}
	}
	// RFC requirement: RFC4106-6-1 negative -- no AEAD word Ze installs carries an ICV other
	// than 128 bits: every entry of xfrmAEADNames builds a state whose ICV is 128 bits.
	for word := range xfrmAEADNames {
		p := rfc4106GCMParams()
		p.EncAlgo = word
		state, err := xfrmStateFromParams(p)
		if err != nil {
			t.Fatalf("xfrmStateFromParams(%s): %v", word, err)
		}
		if state.Aead == nil || state.Aead.ICVLen != 128 {
			t.Errorf("%s: installed an AEAD state with ICV %+v, want 128 bits", word, state.Aead)
		}
	}
}

// TestRFC4106GCMKeyCannotOutliveTheSequenceSpace checks the state property that bounds how
// much data one AES-GCM key encrypts.
//
// RFC 4106 Section 10: "To protect against this problem, implementations MUST generate a
// fresh key before encrypting 2^64 blocks of data with a given key."
//
// Ze installs every state with 32-bit sequence numbers (no ESN, because the engine offers
// and accepts only ESN 0) and with rollover forbidden, so the kernel refuses to send once
// the counter reaches 2^32 packets. An ESP packet carries at most 65535 octets, which is
// 4096 AES blocks, so one key encrypts at most 2^44 blocks before a fresh SA, and with it
// a fresh key, is required.
//
// RFC 4106 Section 2: "The same nonce and key combination MUST NOT be used more than once."
// RFC 4106 Section 3.1: "For a given key, the IV MUST NOT repeat." The kernel's IV
// generator for rfc4106(gcm(aes)) derives each IV from the SA's sequence number, so the
// value Ze installs that keeps an IV, and with the fixed salt the nonce, from repeating
// under one key is the same rollover flag.
func TestRFC4106GCMKeyCannotOutliveTheSequenceSpace(t *testing.T) {
	state, err := xfrmStateFromParams(rfc4106GCMParams())
	if err != nil {
		t.Fatalf("xfrmStateFromParams: %v", err)
	}
	// RFC requirement: RFC4106-10-2 positive -- the AES-GCM state is installed with
	// sequence number rollover forbidden and without Extended Sequence Numbers.
	// RFC requirement: RFC4106-2-1 positive -- the AES-GCM state is installed with
	// sequence number rollover forbidden (OSeqMayWrap false).
	// RFC requirement: RFC4106-3.1-2 positive -- the AES-GCM state is installed with
	// sequence number rollover forbidden (OSeqMayWrap false).
	if state.OSeqMayWrap {
		t.Error("the AES-GCM state permits sequence number rollover, so one key could encrypt without bound")
	}
	if state.ESN {
		t.Error("the AES-GCM state runs 64-bit Extended Sequence Numbers")
	}
}
