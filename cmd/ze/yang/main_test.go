package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: AC-9 -- help subcommand.
// PREVENTS: Missing help text.
func TestRunHelp(t *testing.T) {
	code := Run([]string{"help"})
	assert.Equal(t, 0, code)
}

// VALIDATES: AC-1 -- completion subcommand runs.
// PREVENTS: Completion subcommand crash.
func TestRunCompletion(t *testing.T) {
	code := Run([]string{"completion"})
	assert.Equal(t, 0, code)
}

// VALIDATES: AC-4 -- tree subcommand runs.
// PREVENTS: Tree subcommand crash.
func TestRunTree(t *testing.T) {
	code := Run([]string{"tree"})
	assert.Equal(t, 0, code)
}

// VALIDATES: AC-8 -- doc --list subcommand runs.
// PREVENTS: Doc subcommand crash.
func TestRunDoc(t *testing.T) {
	code := Run([]string{"doc", "--list"})
	assert.Equal(t, 0, code)
}

// VALIDATES: AC-9 -- unknown subcommand returns error.
// PREVENTS: Silent failure on unknown subcommand.
func TestRunUnknown(t *testing.T) {
	code := Run([]string{"nonexistent"})
	assert.Equal(t, 1, code)
}

// VALIDATES: AC-9 -- no args returns error.
// PREVENTS: Panic on empty args.
func TestRunNoArgs(t *testing.T) {
	code := Run(nil)
	assert.Equal(t, 1, code)
}

// VALIDATES: AC-3 -- min-prefix boundary: 0 is invalid.
// PREVENTS: Off-by-one in flag validation.
func TestRunCompletionMinPrefixInvalid(t *testing.T) {
	code := Run([]string{"completion", "--min-prefix", "0"})
	assert.Equal(t, 1, code)
}

// VALIDATES: AC-3 -- min-prefix boundary: 11 is invalid.
// PREVENTS: Values above range accepted.
func TestRunCompletionMinPrefixTooHigh(t *testing.T) {
	code := Run([]string{"completion", "--min-prefix", "11"})
	assert.Equal(t, 1, code)
}

// VALIDATES: AC-3 -- min-prefix boundary: 10 is valid.
// PREVENTS: Last valid value rejected.
func TestRunCompletionMinPrefixMax(t *testing.T) {
	code := Run([]string{"completion", "--min-prefix", "10"})
	assert.Equal(t, 0, code)
}
