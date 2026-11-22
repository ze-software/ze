// Design: docs/contributing/ze-go-style.md -- tests for the commit-time refuse-by-default gate
//
// VALIDATES: judgeChange is registered in changecheck, refuses newly added
// refuse-by-default shapes (e.g. error dropped at Debug), and skips untouched
// legacy shapes and properly annotated guards.
// PREVENTS: refuse-by-default defects surviving commit until post-push CI.
package archrefusebydefault

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

func TestRefuseByDefaultChangeCheckIsRegistered(t *testing.T) {
	t.Parallel()
	registered := changecheck.Registered()
	if !slices.ContainsFunc(registered, func(c changecheck.Check) bool { return c.Name == area+" check" }) {
		t.Fatalf("%q not registered in changecheck: %+v", area+" check", registered)
	}
}

func TestRefuseByDefaultJudgeChangeRefusesAddedAndSkipsLegacy(t *testing.T) {
	t.Parallel()
	legacy := "package demo\n\nimport \"log/slog\"\n\nfunc old(err error) {\n\tif err != nil {\n\t\tslog.Debug(\"ignored\", \"err\", err)\n\t\treturn\n\t}\n}\n"
	addedBad := legacy + "\nfunc bad(err error) {\n\tif err != nil {\n\t\tslog.Debug(\"dropped\", \"err\", err)\n\t\treturn\n\t}\n}\n"
	addedClean := legacy + "\nfunc clean(err error) {\n\tif err != nil {\n\t\tslog.Warn(\"failed\", \"err\", err)\n\t\treturn\n\t}\n}\n"

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
		t.Fatalf("bad addition refusal flagged untouched legacy shape:\n%s", got)
	}
}
