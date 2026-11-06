// Design: docs/architecture/testing/ci-workflows.md -- CGO-disabled package selection.
package codeql

import (
	"context"
	"slices"
	"testing"
	"time"
)

// TestSelectExcludesBothCgoImportSpellings uses real tracked packages and the Go
// loader to prove that interpreted and raw string imports both mean import C.
func TestSelectExcludesBothCgoImportSpellings(t *testing.T) {
	for _, test := range []struct {
		name    string
		literal string
	}{
		{"interpreted", `"C"`},
		{"raw", "`C`"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := fixtureRepository(t, map[string]string{
				"cgoonly/cgo.go": "package cgoonly\nimport " + test.literal + "\n",
			})
			t.Setenv("CGO_ENABLED", "0")
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			target, err := BuildContext(ctx, root, "ze_setup")
			if err != nil {
				t.Fatal(err)
			}
			packages, err := Select(ctx, root, target)
			if err != nil {
				t.Fatalf("valid CGO-disabled population must exclude %s import C: %v", test.name, err)
			}
			if want := []string{"./authored"}; !slices.Equal(packages, want) {
				t.Fatalf("selected %v, want %v", packages, want)
			}
		})
	}
}
