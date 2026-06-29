package ppp

import (
	"testing"
)

// FuzzParsePAPRequest exercises ParsePAPRequest against arbitrary
// input to catch panics and out-of-bounds reads. Required by
// .claude/rules/tdd.md "Fuzz (MANDATORY for wire format)".
func FuzzParsePAPRequest(f *testing.F) {
	seeds := [][]byte{
		nil,
		// valid minimal request: empty Peer-ID, empty Password.
		{0x01, 0x00, 0x00, 0x06, 0x00, 0x00},
		// valid alice/secret (Length = 17 = 0x11).
		{
			0x01, 0x42, 0x00, 0x11,
			0x05, 'a', 'l', 'i', 'c', 'e',
			0x06, 's', 'e', 'c', 'r', 'e', 't',
		},
		// length below header.
		{0x01, 0x00, 0x00, 0x03},
		// length far beyond buffer.
		{0x01, 0x00, 0xFF, 0xFF},
		// wrong code (Authenticate-Ack) -- the parser must reject
		// cleanly without touching the Peer-ID field.
		{0x02, 0x00, 0x00, 0x06, 0x00, 0x00},
		// Peer-ID Length 255 with only 3 bytes of payload.
		{0x01, 0x00, 0x00, 0x08, 0xFF, 'a', 'b', 'c'},
		// Passwd-Length 255 with only 2 bytes of payload.
		{
			0x01, 0x00, 0x00, 0x09,
			0x01, 'u',
			0xFF, 'p', 'q',
		},
		// max-frame-sized buffer of zeros (MaxFrameLen - 2).
		make([]byte, MaxFrameLen-2),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		req, err := ParsePAPRequest(data)
		if err != nil {
			return
		}
		// Invariant 1: declared Length is reachable and covers the
		// body the parser consumed.
		if len(data) < papHeaderLen {
			t.Errorf("parse succeeded on buf too short for header: len=%d", len(data))
			return
		}
		length := int(data[2])<<8 | int(data[3])
		minBody := 1 + len(req.Username) + 1 + len(req.Password)
		if papHeaderLen+minBody > length {
			t.Errorf("parsed fields (%d body bytes) exceed Length %d",
				minBody, length)
			return
		}
		// Invariant 2: parsed Username equals the bytes at the Peer-ID
		// offset in the input. Defends against off-by-one bugs that
		// produce self-consistent sizes but read the wrong region.
		userOff := papHeaderLen + 1
		userEnd := userOff + len(req.Username)
		if userEnd > length {
			t.Errorf("Username end %d exceeds Length %d", userEnd, length)
			return
		}
		if string(data[userOff:userEnd]) != req.Username {
			t.Errorf("Username %q does not match input bytes %q",
				req.Username, string(data[userOff:userEnd]))
		}
		// Invariant 3: parsed Password equals the bytes at the
		// Password offset.
		passOff := userEnd + 1
		passEnd := passOff + len(req.Password)
		if passEnd > length {
			t.Errorf("Password end %d exceeds Length %d", passEnd, length)
			return
		}
		if string(data[passOff:passEnd]) != req.Password {
			t.Errorf("Password %q does not match input bytes %q",
				req.Password, string(data[passOff:passEnd]))
		}
	})
}
