package stringsx

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitCountMatchesStringsSplit(t *testing.T) {
	tests := []struct {
		name string
		s    string
		sep  string
	}{
		{name: "empty", s: "", sep: ","},
		{name: "no separator", s: "65001", sep: ","},
		{name: "single separator", s: "65001,65002", sep: ","},
		{name: "leading separator", s: ",65001", sep: ","},
		{name: "trailing separator", s: "65001,", sep: ","},
		{name: "adjacent separators", s: "65001,,65003", sep: ","},
		{name: "multi-byte separator", s: "a::b::c", sep: "::"},
		{name: "empty separator ascii", s: "abc", sep: ""},
		{name: "empty separator utf8", s: "aéb", sep: ""},
		{name: "empty string empty separator", s: "", sep: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, count := SplitCount(tt.s, tt.sep)
			want := strings.Split(tt.s, tt.sep)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("SplitCount(%q, %q) = %#v, want %#v", tt.s, tt.sep, got, want)
			}
			if count != len(want) {
				t.Fatalf("SplitCount(%q, %q) count = %d, want %d", tt.s, tt.sep, count, len(want))
			}
		})
	}
}
