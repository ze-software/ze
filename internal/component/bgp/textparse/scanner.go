// Design: docs/architecture/api/text-parser.md — shared text event tokenizer
// Related: keywords.go — shared keyword constants and alias resolution

package textparse

import (
	"fmt"
	"strconv"
)

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

// SourcePolicyEpoch reads the policy epoch from the complete reserved UPDATE
// provenance header. Its caller MUST validate the eight-token message header.
func SourcePolicyEpoch(text string) (uint64, error) {
	epoch, _, _, err := SourceProvenance(text)
	return epoch, err
}

// SourceProvenance reads only the reserved pairs immediately after an UPDATE's
// eight-token header. Absent values are unrecorded zero, never current authority.
// Route attributes or metadata after that header cannot supply any value.
// All four reserved pairs are checked by the same physical provenance parser.
func SourceProvenance(text string) (uint64, uint64, uint64, error) {
	epoch, receipt, owner, _, err := SourcePhysicalProvenance(text)
	return epoch, receipt, owner, err
}

// SourcePhysicalProvenance reads the four reserved numeric pairs immediately
// after an UPDATE's fixed eight-token header. The original owner and connection
// are independent of its replay receipt. Zero means unrecorded, never a current
// authority. A caller needing a physical source MUST refuse either zero value.
// The scan is bounded by four distinct pairs; a repeated fifth pair refuses.
func SourcePhysicalProvenance(text string) (uint64, uint64, uint64, uint64, error) {
	s := NewScanner(text)
	for range 8 {
		if _, ok := s.Next(); !ok {
			return 0, 0, 0, 0, fmt.Errorf("source provenance requires a complete UPDATE header")
		}
	}
	var epoch, receipt, owner, connection uint64
	var epochSeen, receiptSeen, ownerSeen, connectionSeen bool
	for {
		token, _ := s.Next()
		switch token {
		case KWSourcePolicyEpoch:
			if epochSeen {
				return 0, 0, 0, 0, fmt.Errorf("duplicate UPDATE provenance field %s", token)
			}
		case KWSourceReceipt:
			if receiptSeen {
				return 0, 0, 0, 0, fmt.Errorf("duplicate UPDATE provenance field %s", token)
			}
		case KWPeerOwner:
			if ownerSeen {
				return 0, 0, 0, 0, fmt.Errorf("duplicate UPDATE provenance field %s", token)
			}
		case KWConnectionID:
			if connectionSeen {
				return 0, 0, 0, 0, fmt.Errorf("duplicate UPDATE provenance field %s", token)
			}
		default:
			// Header names are open text; the first route token ends provenance.
			return epoch, receipt, owner, connection, nil
		}
		value, ok := s.Next()
		if !ok {
			return 0, 0, 0, 0, fmt.Errorf("%s requires a decimal uint64", token)
		}
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("%s: %w", token, err)
		}
		switch token {
		case KWSourcePolicyEpoch:
			epoch, epochSeen = parsed, true
		case KWSourceReceipt:
			receipt, receiptSeen = parsed, true
		case KWPeerOwner:
			owner, ownerSeen = parsed, true
		case KWConnectionID:
			connection, connectionSeen = parsed, true
		}
	}
}
