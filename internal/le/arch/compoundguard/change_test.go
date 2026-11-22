// Design: docs/contributing/ze-go-style.md -- tests for the commit-time compound-guard gate
//
// VALIDATES: judgeChange is registered in changecheck, refuses newly added
// splittable `if a || b { leave }` guards, and leaves untouched legacy guards
// and compliant split guards alone.
// PREVENTS: compound-guard violations surviving commit until post-push CI.
package archcompoundguard

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

func TestCompoundGuardChangeCheckIsRegistered(t *testing.T) {
	t.Parallel()
	registered := changecheck.Registered()
	if !slices.ContainsFunc(registered, func(c changecheck.Check) bool { return c.Name == area+" check" }) {
		t.Fatalf("%q not registered in changecheck: %+v", area+" check", registered)
	}
}

func TestCompoundGuardJudgeChangeRefusesAddedAndSkipsLegacy(t *testing.T) {
	t.Parallel()
	legacy := "package demo\n\nfunc old(a, b bool) int {\n\tif a || b {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"
	addedClean := legacy + "\nfunc clean(x, y bool) int {\n\tif x {\n\t\treturn 1\n\t}\n\tif y {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"
	addedBad := legacy + "\nfunc bad(x, y bool) int {\n\tif x || y {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"

	makeChange := func(before, after string) changecheck.Change {
		return changecheck.Change{
			Paths: []string{"internal/demo/demo.go"},
			Before: func(rel string) (string, bool, error) {
				return before, before != "", nil
			},
			After: func(rel string) (string, bool, error) {
				return after, true, nil
			},
		}
	}

	if got, err := judgeChange(makeChange(legacy, addedClean)); err != nil || got != "" {
		t.Fatalf("clean addition got (%q, %v), want empty", got, err)
	}
	got, err := judgeChange(makeChange(legacy, addedBad))
	if err != nil {
		t.Fatalf("bad addition err = %v", err)
	}
	if !strings.Contains(got, "bad") {
		t.Fatalf("bad addition refusal = %q, want finding naming bad", got)
	}
	if strings.Contains(got, "(old)") {
		t.Fatalf("bad addition refusal flagged untouched legacy guard:\n%s", got)
	}
}
