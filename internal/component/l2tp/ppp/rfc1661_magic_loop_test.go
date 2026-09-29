// VALIDATES: RFC 1661 Section 6.4 loop detection -- when a Configure-Nak names
// the Magic-Number ze itself sent in its last Configure-Nak, ze chooses a new
// Magic-Number and carries it in its next Configure-Request.
// PREVENTS: keeping, or redrawing to, the Magic-Number already on a link that
// is probably looped back.
//
// RFC: rfc/short/rfc1661.md
package ppp

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"testing"
)

// loopMagicLocal is the Magic-Number ze carries before the loop is seen.
const loopMagicLocal uint32 = 0x13572468

// playMagicLoop drives a Req-Sent session through the loop RFC 1661 Section
// 6.4 describes: ze sends a Configure-Request with loopMagicLocal, receives it
// back, answers it with a Configure-Nak naming a different value, and then
// receives that Configure-Nak back. It returns the Magic-Number ze named in
// its own Configure-Nak.
func playMagicLoop(t *testing.T, s *pppSession, rec *frameRecorder) uint32 {
	t.Helper()
	s.magic = loopMagicLocal
	if !s.sendConfigureRequest() {
		t.Fatal("initial Configure-Request failed")
	}
	own := lastLCPConfigureRequest(t, rec)

	looped := lcpOptionsPacket(t, LCPConfigureRequest, own.Identifier, []LCPOption{magicOption(loopMagicLocal)})
	if term := s.handleLCPPacket(looped); term {
		t.Fatal("session terminated on its own looped Configure-Request")
	}
	nak, ok := findCode(t, rec, LCPConfigureNak)
	if !ok {
		t.Fatal("no Configure-Nak answered a Configure-Request carrying ze's own Magic-Number")
	}
	opts, err := ParseLCPOptions(nak.Data)
	if err != nil {
		t.Fatalf("ParseLCPOptions(Configure-Nak): %v", err)
	}
	sent, ok := lookupMagicNumber(opts)
	if !ok {
		t.Fatal("ze's Configure-Nak carries no Magic-Number")
	}
	if sent == loopMagicLocal {
		t.Fatalf("ze's Configure-Nak names its own Magic-Number %#x", sent)
	}

	loopedNak := lcpOptionsPacket(t, LCPConfigureNak, own.Identifier, []LCPOption{magicOption(sent)})
	if term := s.handleLCPPacket(loopedNak); term {
		t.Fatal("session terminated on its own looped Configure-Nak")
	}
	return sent
}

// requestMagic reads the Magic-Number of the last LCP Configure-Request ze wrote.
func requestMagic(t *testing.T, rec *frameRecorder) uint32 {
	t.Helper()
	opts, err := ParseLCPOptions(lastLCPConfigureRequest(t, rec).Data)
	if err != nil {
		t.Fatalf("ParseLCPOptions(Configure-Request): %v", err)
	}
	magic, ok := lookupMagicNumber(opts)
	if !ok {
		t.Fatal("Configure-Request carries no Magic-Number")
	}
	return magic
}

// VALIDATES: a Configure-Nak naming the Magic-Number of ze's last Configure-Nak
// makes ze choose a new, non-zero Magic-Number and send it in its next
// Configure-Request.
// METHOD: playMagicLoop, then read the session's value and the next request.
//
// RFC requirement: RFC1661-6.4-7 positive -- after a Configure-Nak whose Magic-Number equals the one ze sent in its last Configure-Nak, the session's Magic-Number differs from the one it held, is non-zero, and is the Magic-Number of the Configure-Request ze writes next.
func TestRFC1661LoopedNakMagicChoosesNewMagic(t *testing.T) {
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	playMagicLoop(t, s, rec)

	if s.magic == loopMagicLocal {
		t.Fatalf("Magic-Number %#x kept after the looped Configure-Nak", s.magic)
	}
	if s.magic == 0 {
		t.Fatal("Magic-Number chosen as zero")
	}
	if got := requestMagic(t, rec); got != s.magic {
		t.Fatalf("next Configure-Request carries Magic-Number %#x, want the new %#x", got, s.magic)
	}
}

// VALIDATES: the new Magic-Number is new even when the random source repeats
// the old one.
// METHOD: crypto/rand.Reader is fed the old Magic-Number as the first draw and
// a different value as the second, so a redraw that accepted the first draw
// would keep the Magic-Number on the looped link.
//
// RFC requirement: RFC1661-6.4-7 negative -- with the first random draw equal to the Magic-Number ze held, the looped Configure-Nak does not leave that value in place: the session takes the second draw, and the next Configure-Request carries it.
func TestRFC1661LoopedNakMagicRefusesRepeatedDraw(t *testing.T) {
	const second uint32 = 0x2468ACE1
	s, rec, _ := newRFC1661Session(LCPStateReqSent)

	var draws [8]byte
	binary.BigEndian.PutUint32(draws[0:4], loopMagicLocal)
	binary.BigEndian.PutUint32(draws[4:8], second)
	saved := rand.Reader
	t.Cleanup(func() { rand.Reader = saved })
	rand.Reader = bytes.NewReader(draws[:])

	playMagicLoop(t, s, rec)

	if s.magic != second {
		t.Fatalf("Magic-Number = %#x after a first draw repeating %#x, want the second draw %#x",
			s.magic, loopMagicLocal, second)
	}
	if got := requestMagic(t, rec); got != second {
		t.Fatalf("next Configure-Request carries Magic-Number %#x, want %#x", got, second)
	}
}
