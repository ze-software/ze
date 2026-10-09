package registry

import (
	"errors"
	"testing"
)

// TestLocalDataPlainHandlerJudgesArguments proves the plain local handler that
// RegisterLocalData builds runs the installed argument check before the data
// handler, and refuses when no check is installed.
//
// VALIDATES: a refused argument never reaches the data handler and exits 1; an
// accepted one reaches it, and the check sees the registered path; a process
// with no check installed refuses rather than running unvalidated.
// PREVENTS: `ze show env get` with a 129-character key reaching the handler,
// because this route never passes through command.ServeLocal.
// Method: register one data handler, then call the plain handler LookupLocal
// returns under each check.
func TestLocalDataPlainHandlerJudgesArguments(t *testing.T) {
	ResetForTest()
	ResetLocalDataForTest()
	saved := localArgCheck
	t.Cleanup(func() {
		ResetForTest()
		ResetLocalDataForTest()
		localArgCheck = saved
	})

	var ran []string
	data := func(args []string) (any, int) {
		ran = args
		return nil, 0
	}
	render := func(string, any) int { return 0 }
	if err := RegisterLocalData("show thing get", data, Meta{}, render); err != nil {
		t.Fatal(err)
	}
	notDeclared := func(string) (bool, error) { return false, nil }
	plain, args, err := LookupLocal([]string{"show", "thing", "get", "x"}, notDeclared)
	if err != nil {
		t.Fatal(err)
	}
	if plain == nil {
		t.Fatal("no plain handler registered for show thing get")
	}

	localArgCheck = nil
	if code := plain(args); code != 1 {
		t.Errorf("no check installed: exit %d, want 1", code)
	}
	if ran != nil {
		t.Fatalf("no check installed, yet the handler ran with %v", ran)
	}

	var seenPath string
	RegisterLocalArgCheck(func(path string, _ []string) ([]string, error) {
		seenPath = path
		return nil, errors.New("length 129 out of range 1..128")
	})
	if code := plain(args); code != 1 {
		t.Errorf("refused argument: exit %d, want 1", code)
	}
	if ran != nil {
		t.Fatalf("a refused argument reached the handler: %v", ran)
	}
	if seenPath != "show thing get" {
		t.Errorf("check saw path %q, want %q", seenPath, "show thing get")
	}

	// The check answers tokens that differ from the raw ones, so the handler
	// running with them proves the route invokes it with what was judged.
	RegisterLocalArgCheck(func(string, []string) ([]string, error) { return []string{"judged"}, nil })
	if code := plain(args); code != 0 {
		t.Errorf("accepted argument: exit %d, want 0", code)
	}
	if len(ran) != 1 || ran[0] != "judged" {
		t.Errorf("handler ran with %v, want the judged [judged]", ran)
	}
}

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
