package gotoolchain

import (
	"os"
	"testing"

	"github.com/ze-software/ze/internal/le/go/stdpatch"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestMain activates the patched GOROOT with the real go before any test puts
// a fake go on PATH: New activates once per process, so the first New must
// not be one a fake answers. The refusal child of
// TestToolchainOverridesLeaveTheToolchainToTheProcess skips it, because its
// first New is the activation it proves.
func TestMain(m *testing.M) {
	if os.Getenv(refusalChildRoot) == "" {
		checkout, err := lepath.Root()
		if err != nil {
			_, _ = os.Stderr.WriteString(err.Error() + "\n") //nolint:errcheck // the exit code reports it
			os.Exit(2)
		}
		if _, err := stdpatch.Activate(checkout); err != nil {
			_, _ = os.Stderr.WriteString(err.Error() + "\n") //nolint:errcheck // the exit code reports it
			os.Exit(2)
		}
	}
	os.Exit(m.Run())
}
