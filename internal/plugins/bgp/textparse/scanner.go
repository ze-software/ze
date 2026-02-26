// Design: docs/architecture/api/text-parser.md — shared text event tokenizer

package textparse

// Scanner tokenizes a whitespace-delimited text string without allocating.
// It holds a reference to the original string and tracks the current position.
type Scanner struct {
	text string
	pos  int
}

// NewScanner creates a Scanner positioned at the start of text.
func NewScanner(text string) Scanner {
	return Scanner{text: text}
}

// Next returns the next whitespace-delimited token and advances past it.
// Returns ("", false) when no more tokens remain.
func (s *Scanner) Next() (string, bool) {
	// skip whitespace
	for s.pos < len(s.text) && isSpace(s.text[s.pos]) {
		s.pos++
	}
	if s.pos >= len(s.text) {
		return "", false
	}
	start := s.pos
	for s.pos < len(s.text) && !isSpace(s.text[s.pos]) {
		s.pos++
	}
	return s.text[start:s.pos], true
}

// Peek returns the next token without advancing the position.
// Returns ("", false) when no more tokens remain.
func (s *Scanner) Peek() (string, bool) {
	saved := s.pos
	tok, ok := s.Next()
	s.pos = saved
	return tok, ok
}

// Done returns true when no more tokens remain.
func (s *Scanner) Done() bool {
	for i := s.pos; i < len(s.text); i++ {
		if !isSpace(s.text[i]) {
			return false
		}
	}
	return true
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
