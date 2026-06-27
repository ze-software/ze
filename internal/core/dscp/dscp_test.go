package dscp

import (
	"testing"
)

func TestParseNamedDSCP(t *testing.T) {
	tests := []struct {
		input string
		want  uint8
	}{
		{"cs6", 48},
		{"CS6", 48},
		{"ef", 46},
		{"EF", 46},
		{"cs0", 0},
		{"af11", 10},
		{"af43", 38},
		{"cs7", 56},
		{"48", 48},
		{"0", 0},
		{"63", 63},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	tests := []string{"64", "255", "bogus", ""}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(input)
			if err == nil {
				t.Fatalf("Parse(%q): want error, got nil", input)
			}
		})
	}
}
