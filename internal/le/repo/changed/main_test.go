package repochanged

import (
	"os"
	"testing"

	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestMain activates the patched GOROOT for the checkout these tests run in,
// as le's root handler does before the selector runs. The selector builds its
// import graph through gotoolchain.New, which activates once per process for
// lepath.Root, and most tests point ZE_REPO_ROOT at a fixture module that holds
// no patch set. Without this, the first test's fixture would answer that one
// activation, its refusal would be remembered for the whole process, and every
// graph would widen to ./... for a reason no test asked about.
func TestMain(m *testing.M) {
	checkout, err := lepath.Root()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n") //nolint:errcheck // the exit code reports it
		os.Exit(2)
	}
	if _, err := gen.Activate(checkout); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n") //nolint:errcheck // the exit code reports it
		os.Exit(2)
	}
	os.Exit(m.Run())
}
