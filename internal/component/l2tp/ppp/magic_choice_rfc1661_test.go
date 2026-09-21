// Design: docs/architecture/l2tp/bng-5-pppoe.md -- RFC 1661 conformance coverage
//
// Proves RFC 1661 Section 6.4: ze chooses its Magic-Number before it requests
// the Magic-Number Configuration Option. generateMagic draws the value the
// session runs on before LCP starts (session_run.go), and
// BuildLocalConfigRequest emits option 5 only when a chosen, non-zero value is
// present, so a Configure-Request never requests the option ahead of the choice.

package ppp

import (
	"encoding/binary"
	"testing"
)

// TestMagicNumberChosenBeforeRequested draws a Magic-Number the way the session
// does and reads the option list ze puts in its Configure-Request with, and
// without, a chosen value.
//
// RFC requirement: RFC1661-6.4-5 positive — generateMagic returns a non-zero Magic-Number, and BuildLocalConfigRequest carrying it emits option 5 whose four data octets hold exactly that value.
// RFC requirement: RFC1661-6.4-5 negative — BuildLocalConfigRequest with no chosen Magic-Number (zero) emits no option 5 at all, so the option is never requested before a value is chosen.
func TestMagicNumberChosenBeforeRequested(t *testing.T) {
	t.Parallel()

	magic, err := generateMagic()
	if err != nil {
		t.Fatalf("generateMagic: %v", err)
	}
	if magic == 0 {
		t.Fatal("generateMagic returned zero, which RFC 1661 Section 6.4 makes illegal")
	}

	chosen := BuildLocalConfigRequest(LCPOptions{MRU: 1500, Magic: magic})
	found := 0
	for _, opt := range chosen {
		if opt.Type != LCPOptMagic {
			continue
		}
		found++
		if len(opt.Data) != 4 {
			t.Fatalf("Magic-Number option data = %d octets, want 4", len(opt.Data))
		}
		if got := binary.BigEndian.Uint32(opt.Data); got != magic {
			t.Errorf("Magic-Number option carries %#x, want the chosen %#x", got, magic)
		}
	}
	if found != 1 {
		t.Errorf("Configure-Request carries %d Magic-Number options, want exactly 1", found)
	}

	unchosen := BuildLocalConfigRequest(LCPOptions{MRU: 1500})
	if len(unchosen) == 0 {
		t.Fatal("Configure-Request with no Magic-Number chosen carries no options at all; the MRU option is missing")
	}
	for _, opt := range unchosen {
		if opt.Type == LCPOptMagic {
			t.Errorf("Configure-Request requests the Magic-Number option (data %x) before any Magic-Number was chosen", opt.Data)
		}
	}
}

// TestEqualMagicNumberIsNaked feeds NegotiatePeerOptions a peer Configure-Request
// whose Magic-Number equals the one ze last sent (policy.LocalMagic), and one
// whose Magic-Number differs, and reads the reply each earns.
//
// RFC requirement: RFC1661-6.4-6 positive — a peer Magic-Number equal to the one ze last sent draws a Configure-Nak carrying option 5 with a non-zero value different from ze's own.
// RFC requirement: RFC1661-6.4-6 negative — a peer Magic-Number different from the one ze last sent is acknowledged unchanged, with no Nak and no Reject.
func TestEqualMagicNumberIsNaked(t *testing.T) {
	t.Parallel()

	const localMagic = 0x5A5A1234
	policy := LCPNegPolicy{MaxMRU: MaxFrameLen, LocalMagic: localMagic}

	acks, naks, rejects := NegotiatePeerOptions([]LCPOption{magicOption(localMagic)}, policy)
	if len(rejects) != 0 {
		t.Fatalf("rejects = %+v, want none: Section 6.4 forbids Configure-Reject of a Magic-Number ze transmits itself", rejects)
	}
	if len(acks) != 0 {
		t.Fatalf("acks = %+v, want none for a Magic-Number equal to the one ze last sent", acks)
	}
	if len(naks) != 1 || naks[0].Type != LCPOptMagic || len(naks[0].Data) != 4 {
		t.Fatalf("naks = %+v, want exactly one 4-octet Magic-Number option", naks)
	}
	offered := binary.BigEndian.Uint32(naks[0].Data)
	if offered == localMagic {
		t.Errorf("Configure-Nak offers %#x, the same Magic-Number the request carried; Section 6.4 requires a different value", offered)
	}
	if offered == 0 {
		t.Error("Configure-Nak offers a zero Magic-Number, which Section 6.4 makes illegal")
	}

	acks, naks, rejects = NegotiatePeerOptions([]LCPOption{magicOption(localMagic + 1)}, policy)
	if len(naks) != 0 || len(rejects) != 0 {
		t.Fatalf("naks = %+v, rejects = %+v, want none for a Magic-Number that differs from ze's own", naks, rejects)
	}
	if len(acks) != 1 || acks[0].Type != LCPOptMagic {
		t.Fatalf("acks = %+v, want the peer's Magic-Number option", acks)
	}
	if got := binary.BigEndian.Uint32(acks[0].Data); got != localMagic+1 {
		t.Errorf("Configure-Ack carries %#x, want the peer's %#x unchanged", got, uint32(localMagic+1))
	}
}
