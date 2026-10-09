package registry

import (
	"errors"
	"testing"
)

// VALIDATES: ValidateLocalArgs, the judgment the R6 (`ze <verb>`) and R7
// (offline fallback) callers run, refuses when no check is installed and
// otherwise answers exactly what the installed check judged, for the path the
// caller names.
// PREVENTS: a process that loaded no model running a local handler on
// unjudged tokens.
func TestValidateLocalArgsAnswersTheJudgedTokens(t *testing.T) {
	saved := localArgCheck
	t.Cleanup(func() { localArgCheck = saved })

	localArgCheck = nil
	if _, err := ValidateLocalArgs("show host", []string{"cpu"}); !errors.Is(err, errLocalArgCheckMissing) {
		t.Fatalf("no check installed: err = %v, want errLocalArgCheckMissing", err)
	}
	var seenPath string
	RegisterLocalArgCheck(func(path string, args []string) ([]string, error) {
		seenPath = path
		return append([]string{"judged"}, args...), nil
	})
	got, err := ValidateLocalArgs("show host", []string{"cpu"})
	if err != nil {
		t.Fatal(err)
	}
	if seenPath != "show host" || len(got) != 2 || got[0] != "judged" {
		t.Errorf("path %q tokens %v, want the check's answer for show host", seenPath, got)
	}
}
