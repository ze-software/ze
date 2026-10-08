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
	saved := localDataArgCheck
	t.Cleanup(func() {
		ResetForTest()
		ResetLocalDataForTest()
		localDataArgCheck = saved
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

	localDataArgCheck = nil
	if code := plain(args); code != 1 {
		t.Errorf("no check installed: exit %d, want 1", code)
	}
	if ran != nil {
		t.Fatalf("no check installed, yet the handler ran with %v", ran)
	}

	var seenPath string
	RegisterLocalDataArgCheck(func(path string, _ []string) error {
		seenPath = path
		return errors.New("length 129 out of range 1..128")
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

	RegisterLocalDataArgCheck(func(string, []string) error { return nil })
	if code := plain(args); code != 0 {
		t.Errorf("accepted argument: exit %d, want 0", code)
	}
	if len(ran) != 1 || ran[0] != "x" {
		t.Errorf("handler ran with %v, want [x]", ran)
	}
}
