package cli

import (
	"bufio"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: promptWithDefault and promptYesNo report a stdin read failure
// instead of substituting the default answer.
// PREVENTS: `ze bgp plugin cli` registering a plugin under settings the
// operator never chose, because a broken pipe or an over-long line looked
// like Enter at the prompt.

// failingReader yields prefix, then fails. It models a pipe that breaks part
// way through a line.
type failingReader struct {
	prefix []byte
	err    error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.prefix) == 0 {
		return 0, f.err
	}
	n := copy(p, f.prefix)
	f.prefix = f.prefix[n:]
	return n, nil
}

// TestPromptWithDefaultReportsReadFailure drives a reader that fails part way
// through the answer. Scan returns TRUE here: the buffered prefix comes back
// as a final token, so the truncated answer would otherwise be accepted.
func TestPromptWithDefaultReportsReadFailure(t *testing.T) {
	want := errors.New("pipe broke")
	sc := bufio.NewScanner(&failingReader{prefix: []byte("my-plug"), err: want})

	answer, err := promptWithDefault(sc, "Plugin name", "cli-debug")

	require.Error(t, err)
	assert.ErrorIs(t, err, want)
	assert.Empty(t, answer)
}

// TestPromptWithDefaultReportsOverLongLine drives an answer above
// bufio.MaxScanTokenSize. This is the failure mode with no underlying I/O
// error, so it is the one a working pipe still reaches.
func TestPromptWithDefaultReportsOverLongLine(t *testing.T) {
	sc := bufio.NewScanner(strings.NewReader(strings.Repeat("a", bufio.MaxScanTokenSize+1)))

	answer, err := promptWithDefault(sc, "Plugin name", "cli-debug")

	require.Error(t, err)
	assert.ErrorIs(t, err, bufio.ErrTooLong)
	assert.Empty(t, answer)
}

// TestPromptWithDefaultKeepsDefaultOnEOF pins the contract the fix must not
// change: an empty stdin still takes the default.
func TestPromptWithDefaultKeepsDefaultOnEOF(t *testing.T) {
	sc := bufio.NewScanner(strings.NewReader(""))

	answer, err := promptWithDefault(sc, "Plugin name", "cli-debug")

	require.NoError(t, err)
	assert.Equal(t, "cli-debug", answer)
}

// TestPromptWithDefaultReadsTheAnswer pins the normal path.
func TestPromptWithDefaultReadsTheAnswer(t *testing.T) {
	sc := bufio.NewScanner(strings.NewReader("chosen\n"))

	answer, err := promptWithDefault(sc, "Plugin name", "cli-debug")

	require.NoError(t, err)
	assert.Equal(t, "chosen", answer)
}

// TestPromptYesNoReportsReadFailure verifies the yes/no prompt does not answer
// the question for the operator when the read fails.
func TestPromptYesNoReportsReadFailure(t *testing.T) {
	want := errors.New("pipe broke")
	sc := bufio.NewScanner(&failingReader{prefix: []byte("n"), err: want})

	_, err := promptYesNo(sc, "Use default registration?", true)

	require.Error(t, err)
	assert.ErrorIs(t, err, want)
}

// TestPromptYesNoKeepsDefaultOnEOF pins the EOF contract.
func TestPromptYesNoKeepsDefaultOnEOF(t *testing.T) {
	sc := bufio.NewScanner(strings.NewReader(""))

	answer, err := promptYesNo(sc, "Use default registration?", true)

	require.NoError(t, err)
	assert.True(t, answer)
}
