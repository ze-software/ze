// VALIDATES: the ICV lengths an AES CCM implementation may support (RFC 5282 §3.2), over
// the table that turns a wire Transform ID into an ICV length and over the seal and open
// pair the engine drives.
// PREVENTS: an ICV length reaching the wire from an encryption transform ze does not
// specify, and a zero ICV length standing in for a refusal.
package crypto

import (
	"bytes"
	"errors"
	"testing"
)

// ccmKeyWithSalt is one AES CCM 256 key material block: a 32 octet cipher key followed by
// the three octet salt of RFC 4309 Section 7.1, which RFC 5282 Section 7.1 carries into
// the IKE SA.
func ccmKeyWithSalt() []byte {
	material := make([]byte, 35)
	for i := range material {
		material[i] = byte(i + 1)
	}
	return material
}

// RFC requirement: RFC5282-3.2-4 positive -- no AES CCM ICV length other than 8, 12 and 16
// octets can be produced or accepted. Over the whole encryption Transform ID range of the
// IANA registry, AEADICVOctets answers a length for four identifiers only, and each answer
// is one of those three. Every other identifier is refused with ErrUnsupportedAlgorithm
// and a zero length, and SealIKEAEAD and OpenIKEAEAD refuse it too.
//
// RFC 5282 Section 3.2: "AES CCM provides an encrypted ICV. Implementations MUST support
// ICV sizes of 8 octets and 16 octets. Implementations MAY also support 12 octet ICVs and
// MUST NOT support other ICV lengths."
//
// An ICV length exists in ze only where a Transform ID names one, because Section 7.2
// assigns one identifier per AES CCM ICV size. Sweeping the identifier space is therefore
// the way to show that no other length is reachable, and it is what a table gaining a
// fourth AES CCM entry would fail.
func TestRFC5282CCMRefusesEveryOtherICVLength(t *testing.T) {
	// RFC 7296 Section 3.3.2 gives Transform Type 1 a two octet identifier, and the IANA
	// registry has assigned through 35. The sweep covers the assigned space and the
	// reserved value zero.
	const highestAssigned = 35
	permitted := map[int]bool{8: true, 12: true, 16: true}
	answered := make(map[EncryptionID]int)

	for id := range EncryptionID(highestAssigned + 1) {
		icvOctets, err := AEADICVOctets(id)
		if err != nil {
			if !errors.Is(err, ErrUnsupportedAlgorithm) {
				t.Errorf("AEADICVOctets(%d) = %v, want ErrUnsupportedAlgorithm", id, err)
			}
			if icvOctets != 0 {
				t.Errorf("AEADICVOctets(%d) answered %d octets beside the refusal", id, icvOctets)
			}
			if _, err := SealIKEAEAD(id, ccmKeyWithSalt(), []byte("payloads"), nil); err == nil {
				t.Errorf("SealIKEAEAD(%d) sealed a message under a transform with no ICV length", id)
			}
			if _, err := OpenIKEAEAD(id, ccmKeyWithSalt(), bytes.Repeat([]byte{0}, 40), nil); err == nil {
				t.Errorf("OpenIKEAEAD(%d) opened a message under a transform with no ICV length", id)
			}
			continue
		}
		if !permitted[icvOctets] {
			t.Errorf("AEADICVOctets(%d) = %d octets, want 8, 12 or 16", id, icvOctets)
		}
		answered[id] = icvOctets
	}

	// RFC 5282 Section 7.2: "14 for AES CCM with an 8-octet ICV; 15 for AES CCM with a
	// 12-octet ICV; 16 for AES CCM with a 16-octet ICV; ... and 20 for AES GCM with a
	// 16-octet ICV." Those four are the whole answered set, so no fifth transform can
	// carry an ICV of any length.
	want := map[EncryptionID]int{
		ENCR_AES_CCM_8:  8,
		ENCR_AES_CCM_12: 12,
		ENCR_AES_CCM_16: 16,
		ENCR_AES_GCM_16: 16,
	}
	if len(answered) != len(want) {
		t.Fatalf("%d transforms answered an ICV length, want %d: %v", len(answered), len(want), answered)
	}
	for id, icvOctets := range want {
		if answered[id] != icvOctets {
			t.Errorf("AEADICVOctets(%s) = %d octets, want %d", id, answered[id], icvOctets)
		}
	}
}

// RFC requirement: RFC5282-3.2-4 negative -- the refusal above is specific to the lengths
// RFC 5282 Section 3.2 forbids rather than a blanket one. Each of the three AES CCM
// identifiers seals a message whose ciphertext is longer than its plaintext by exactly the
// ICV that identifier names, and opens it back to the same plaintext.
//
// Without this polarity a table that answered an error for every transform would satisfy
// the positive case, and ze would support no ICV length at all.
func TestRFC5282CCMSupportsTheThreePermittedICVLengths(t *testing.T) {
	key := ccmKeyWithSalt()
	plaintext := []byte("inner payloads and the pad length")
	aad := bytes.Repeat([]byte{0x11}, 32)

	for id, icvOctets := range map[EncryptionID]int{
		ENCR_AES_CCM_8:  8,
		ENCR_AES_CCM_12: 12,
		ENCR_AES_CCM_16: 16,
	} {
		data, err := SealIKEAEAD(id, key, plaintext, aad)
		if err != nil {
			t.Fatalf("SealIKEAEAD(%s): %v", id, err)
		}
		// The payload data is the eight octet IV of Section 3.1, then the ciphertext
		// with its ICV, so the expansion beyond the plaintext is IV plus ICV.
		if got := len(data) - ikeIVOctets - len(plaintext); got != icvOctets {
			t.Errorf("%s sealed a %d octet ICV, want %d", id, got, icvOctets)
		}
		opened, err := OpenIKEAEAD(id, key, data, aad)
		if err != nil {
			t.Fatalf("OpenIKEAEAD(%s): %v", id, err)
		}
		if !bytes.Equal(opened, plaintext) {
			t.Errorf("%s opened %q, want %q", id, opened, plaintext)
		}
	}
}
