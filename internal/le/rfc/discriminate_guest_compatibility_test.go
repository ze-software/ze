// Design: docs/contributing/rfc-conformance-gates.md -- explicit guest compatibility.
package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExplicitGuestPlacementRequiresCompatibleConstraints checks the guest
// decision with host-compatible, guest-only and incompatible files. A kernel
// never overrides build constraints; omitting it preserves host-first behavior.
func TestExplicitGuestPlacementRequiresCompatibleConstraints(t *testing.T) {
	directory := t.TempDir()
	files := map[string]string{
		"plain_test.go":       "package widget\n",
		"darwin_test.go":      "//go:build darwin\n\npackage widget\n",
		"integration_test.go": "//go:build integration && linux\n\npackage widget\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	host, guest := placementContexts(nil)
	host.GOOS = "darwin"
	for _, tc := range []struct {
		file                             string
		explicit, wantGuest, wantRefusal bool
	}{
		{"plain_test.go", false, false, false},
		{"plain_test.go", true, true, false},
		{"darwin_test.go", false, false, false},
		{"darwin_test.go", true, false, true},
		{"integration_test.go", false, true, false},
		{"integration_test.go", true, true, false},
	} {
		got, err := unitNeedsGuest(&host, &guest, filepath.Join(directory, tc.file), tc.explicit)
		if tc.wantRefusal {
			if err == nil {
				t.Fatalf("%s: incompatible explicit guest accepted", tc.file)
			}
			if !strings.Contains(err.Error(), "selected by `kernel`") {
				t.Fatalf("%s: refusal loses explicit selection: %v", tc.file, err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.wantGuest {
			t.Errorf("%s explicit=%v: guest=%v, want %v", tc.file, tc.explicit, got, tc.wantGuest)
		}
	}
}
