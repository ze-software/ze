package textparse

import (
	"testing"
)

// FuzzScanner tests text protocol scanner robustness.
//
// VALIDATES: Scanner handles arbitrary strings without crashing.
// PREVENTS: Panic on malformed commands, embedded NUL bytes, extreme whitespace.
// SECURITY: Scanner input comes from plugin commands over Unix sockets.
func FuzzScanner(f *testing.F) {
	f.Add("update text origin set igp nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24")
	f.Add("update hex attr set 400101 nlri ipv4/unicast add 180a00")
	f.Add("")                         // Empty
	f.Add("   ")                      // Only whitespace
	f.Add("\t\n\r ")                  // Mixed whitespace
	f.Add("a")                        // Single char
	f.Add("a b c d e f g h i j k l")  // Many tokens
	f.Add("\x00\x01\x02")             // Binary junk
	f.Add("token\x00embedded")        // NUL in middle
	f.Add("   leading   trailing   ") // Whitespace padding

	f.Fuzz(func(t *testing.T, text string) {
		s := NewScanner(text)
		// Exhaust all tokens — MUST NOT panic or infinite loop
		for !s.Done() {
			tok, ok := s.Next()
			if !ok {
				break
			}
			_ = tok
		}

		// Also exercise Peek after exhaustion — MUST NOT panic
		s2 := NewScanner(text)
		for {
			_, ok := s2.Peek()
			if !ok {
				break
			}
			s2.Next()
		}
	})
}
