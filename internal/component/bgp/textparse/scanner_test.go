package textparse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: AC-11 — shared TextScanner with next()/peek()/done()
// PREVENTS: positional field indexing regressions in parser

func TestTextScannerNext(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		tokens []string
	}{
		{"single token", "hello", []string{"hello"}},
		{"two tokens", "hello world", []string{"hello", "world"}},
		{"multiple spaces", "hello   world", []string{"hello", "world"}},
		{"leading spaces", "  hello", []string{"hello"}},
		{"trailing spaces", "hello  ", []string{"hello"}},
		{"empty string", "", nil},
		{"only spaces", "   ", nil},
		{"three tokens", "peer 10.0.0.1 asn", []string{"peer", "10.0.0.1", "asn"}},
		{"tab separated", "hello\tworld", []string{"hello", "world"}},
		{"mixed whitespace", " \thello \t world\t", []string{"hello", "world"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewScanner(tt.input)
			var got []string
			for {
				tok, ok := s.Next()
				if !ok {
					break
				}
				got = append(got, tok)
			}
			assert.Equal(t, tt.tokens, got)
		})
	}
}

func TestTextScannerPeek(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		peekTok string
		peekOK  bool
		nextTok string
		nextOK  bool
		sameTok bool
	}{
		{"peek then next same token", "hello world", "hello", true, "hello", true, true},
		{"peek empty", "", "", false, "", false, true},
		{"peek only spaces", "   ", "", false, "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewScanner(tt.input)
			peekTok, peekOK := s.Peek()
			assert.Equal(t, tt.peekTok, peekTok)
			assert.Equal(t, tt.peekOK, peekOK)

			nextTok, nextOK := s.Next()
			assert.Equal(t, tt.nextTok, nextTok)
			assert.Equal(t, tt.nextOK, nextOK)

			if tt.sameTok {
				assert.Equal(t, peekTok, nextTok)
			}
		})
	}
}

func TestTextScannerPeekDoesNotAdvance(t *testing.T) {
	s := NewScanner("a b c")

	tok1, ok1 := s.Peek()
	assert.True(t, ok1)
	assert.Equal(t, "a", tok1)

	tok2, ok2 := s.Peek()
	assert.True(t, ok2)
	assert.Equal(t, "a", tok2)

	tok3, ok3 := s.Next()
	assert.True(t, ok3)
	assert.Equal(t, "a", tok3)

	tok4, ok4 := s.Peek()
	assert.True(t, ok4)
	assert.Equal(t, "b", tok4)
}

func TestTextScannerDone(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		doneInit bool
	}{
		{"empty is done", "", true},
		{"spaces only is done", "   ", true},
		{"has content not done", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewScanner(tt.input)
			assert.Equal(t, tt.doneInit, s.Done())
		})
	}

	t.Run("done after consuming all", func(t *testing.T) {
		s := NewScanner("a b")
		assert.False(t, s.Done())
		s.Next()
		assert.False(t, s.Done())
		s.Next()
		assert.True(t, s.Done())
	})
}

func TestTextScannerRealEvent(t *testing.T) {
	// Simulate parsing a real text event header
	s := NewScanner("peer 10.0.0.1 remote as 65001 received update 42 origin igp")
	expected := []string{"peer", "10.0.0.1", "remote", "as", "65001", "received", "update", "42", "origin", "igp"}

	var got []string
	for !s.Done() {
		tok, ok := s.Next()
		assert.True(t, ok)
		got = append(got, tok)
	}
	assert.Equal(t, expected, got)
}
