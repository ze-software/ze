package config

import (
	"testing"
)

func TestJoinPath(t *testing.T) {
	tests := []struct {
		name   string
		parts  []string
		expect string
	}{
		{"single", []string{"bgp"}, "bgp"},
		{"two", []string{"bgp", "peer"}, "bgp/peer"},
		{"three", []string{"bgp", "peer", "timer"}, "bgp/peer/timer"},
		{"empty slice", []string{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinPath(tt.parts...)
			if got != tt.expect {
				t.Errorf("JoinPath(%v) = %q, want %q", tt.parts, got, tt.expect)
			}
		})
	}
}

func TestAppendPath(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		seg    string
		expect string
	}{
		{"empty prefix", "", "bgp", "bgp"},
		{"one level", "bgp", "peer", "bgp/peer"},
		{"two levels", "bgp/peer", "timer", "bgp/peer/timer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AppendPath(tt.prefix, tt.seg)
			if got != tt.expect {
				t.Errorf("AppendPath(%q, %q) = %q, want %q", tt.prefix, tt.seg, got, tt.expect)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		expect []string
	}{
		{"single", "bgp", []string{"bgp"}},
		{"two", "bgp/peer", []string{"bgp", "peer"}},
		{"three", "bgp/peer/timer", []string{"bgp", "peer", "timer"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitPath(tt.path)
			if len(got) != len(tt.expect) {
				t.Errorf("SplitPath(%q) = %v, want %v", tt.path, got, tt.expect)
				return
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Errorf("SplitPath(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.expect[i])
				}
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	parts := []string{"bgp", "peer", "session", "asn"}
	joined := JoinPath(parts...)
	split := SplitPath(joined)
	if len(split) != len(parts) {
		t.Fatalf("round-trip failed: %v -> %q -> %v", parts, joined, split)
	}
	for i := range parts {
		if split[i] != parts[i] {
			t.Errorf("round-trip[%d]: %q != %q", i, split[i], parts[i])
		}
	}
}
