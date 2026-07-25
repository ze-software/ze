// Design: plan/spec-cos-dynamic.md -- CoS Filter-Id parser tests
// VALIDATES: AC-1, AC-6, AC-7 -- Filter-Id parsing via core/cos

package cos

import (
	"testing"

	coreCos "github.com/ze-software/ze/internal/core/cos"
)

func TestParseCoSFilterID(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantOK   bool
	}{
		{"cos:residential", "residential", true},
		{"cos:business", "business", true},
		{"cos:gold", "gold", true},
		{"10mbit", "", false},
		{"rate:10M/5M", "", false},
		{"", "", false},
		{"cos:", "", false},
		{"COS:residential", "", false},
		{"residential", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, ok := coreCos.ParseFilterID(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("ParseFilterID(%q): ok=%v, want %v", tt.input, ok, tt.wantOK)
			}
			if name != tt.wantName {
				t.Fatalf("ParseFilterID(%q): name=%q, want %q", tt.input, name, tt.wantName)
			}
		})
	}
}
