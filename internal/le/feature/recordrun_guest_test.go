package feature

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
)

// VALIDATES: a .ci the runner says this host skips (runner.NeedsGuest) is run
// in the QEMU guest through `le test qemu run ... all-tests test <path>`, and a
// .ci it does not skip still runs through its host runner.
// PREVENTS: record-run observing a host SKIP of a capability-gated test and
// recording nothing, with no route to the place the test really runs.
//
// Method: the set's le is a script that prints its own argv and a PASS line,
// so the test reads which command record-run chose; the guest daemons count as
// built, so nothing compiles.
func TestAGuestOnlyTestRunsInTheQEMUGuest(t *testing.T) {
	toolchain, err := gotoolchain.New(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("toolchain: %v", err)
	}
	set := t.TempDir()
	script := "#!/bin/sh\necho \"argv: $*\"\necho \"1.0s  1/1  PASS  a  gated\"\n"
	if err := os.WriteFile(filepath.Join(set, testfunctional.LE), []byte(script), 0o700); err != nil { //nolint:gosec // the fake le must be executable
		t.Fatal(err)
	}
	for _, guest := range []bool{true, false} {
		runner := &repoRunner{tree: t.TempDir(), toolchain: toolchain, prepared: true, guestBuilt: true,
			set:        testfunctional.BinarySet{Dir: set},
			needsGuest: func(string) (bool, error) { return guest, nil }}
		seen, err := runner.run("test/policy/gated.ci")
		if err != nil {
			t.Fatalf("guest=%v: %v", guest, err)
		}
		inGuest := strings.Contains(seen.output, "argv: test qemu run ") &&
			strings.Contains(seen.output, " test qemu all-tests test test/policy/gated.ci")
		if inGuest != guest {
			t.Fatalf("guest=%v: the command record-run chose was:\n%s", guest, seen.output)
		}
		if problem := observedPass("test/policy/gated.ci", seen); problem != "" {
			t.Fatalf("guest=%v: the test's own PASS line was not read: %s", guest, problem)
		}
	}
}
