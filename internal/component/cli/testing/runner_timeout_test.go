package testing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// timeoutCase is a session-free editor test whose only variable is its
// declared option=timeout line.
func timeoutCase(timeoutLine string) string {
	return `# Timeout test
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  router-id 1.2.3.4;
}
EOF_CONF

option=file:path=test.conf
` + timeoutLine + `

input=type:text=set bgp router-id 5.6.7.8
input=enter
expect=dirty:true
`
}

// TestRunnerEnforcesDeclaredTimeout proves the runner stops waiting for a test
// that outlasts its declared option=timeout and fails it.
//
// VALIDATES: a run longer than option=timeout fails with "timed out", and the
// runner returns instead of waiting for the run to end.
// PREVENTS: option=timeout parsed and dropped, so a hung command (a deadlock
// on the write-through lock) blocks the editor suite until a human kills it.
// Method: a 1ns budget, which every real run exceeds.
func TestRunnerEnforcesDeclaredTimeout(t *testing.T) {
	result := runETTest(timeoutCase("option=timeout:value=1ns"))
	require.NotNil(t, result)
	assert.False(t, result.Passed, "a run past its timeout must fail")
	assert.Contains(t, result.Error, "timed out after 1ns")
	removeHangStacks(t, result.Error)
}

// removeHangStacks answers the stacks file a timed-out run's error names and
// removes it when t ends. The runner leaves a real timeout's file for
// post-mortem, so a test that times out on purpose MUST clean up its own.
func removeHangStacks(t *testing.T, runError string) string {
	t.Helper()
	_, stacks, found := strings.Cut(runError, "goroutine stacks: ")
	require.True(t, found, "a timed-out run must name its stacks file: %s", runError)
	t.Cleanup(func() { os.Remove(stacks) }) //nolint:errcheck // test cleanup
	return stacks
}

// TestRunnerHangStacksLieOutsideTempDir proves the goroutine stacks a
// timed-out run names are written outside the run's temp dir.
//
// VALIDATES: the stacks file the error names exists, holds the worker's
// stack, and lies directly in the system temp dir, outside the run's temp
// dir, which the worker removes when the late run ends.
// PREVENTS: the stacks of a run that was late rather than hung being deleted
// with its temp dir, so the path a red test prints points at nothing.
// Method: a 1ns budget, which every real run exceeds.
func TestRunnerHangStacksLieOutsideTempDir(t *testing.T) {
	result := runETTest(timeoutCase("option=timeout:value=1ns"))
	require.NotNil(t, result)
	stacks := removeHangStacks(t, result.Error)

	// The run's temp dir is a child of the system temp dir, so a stacks file
	// directly in the system temp dir lies outside it.
	assert.Equal(t, filepath.Clean(os.TempDir()), filepath.Dir(stacks), "stacks %s must lie outside the run's temp dir", stacks)
	content, err := os.ReadFile(stacks) //nolint:gosec // path the runner just named
	require.NoError(t, err)
	assert.Contains(t, string(content), "runTestCaseWorker")
}

// TestRunnerRefusesInvalidTimeout proves a timeout the runner cannot parse
// fails the test rather than falling back to the default in silence.
//
// VALIDATES: option=timeout:value=banana fails, naming the option.
// PREVENTS: a typo in the budget giving the test a budget nobody wrote.
func TestRunnerRefusesInvalidTimeout(t *testing.T) {
	result := runETTest(timeoutCase("option=timeout:value=banana"))
	require.NotNil(t, result)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Error, "option=timeout")
}

// TestRunnerTimeoutPassesWithinBudget proves a declared budget the run fits
// in leaves the result untouched.
//
// VALIDATES: a 30s budget passes a sub-second run.
// PREVENTS: the deadline machinery failing or altering a healthy run.
func TestRunnerTimeoutPassesWithinBudget(t *testing.T) {
	result := runETTest(timeoutCase("option=timeout:value=30s"))
	require.NotNil(t, result)
	assert.True(t, result.Passed, "a run inside its budget must pass: %s", result.Error)
}

// TestTestBudget proves the budget a run gets is its declared timeout, or the
// default when it declares none, multiplied by the caller's headroom.
//
// VALIDATES: declared and default budgets, the headroom multiplier, and the
// refusal of an unparsable timeout and of a headroom below one.
// PREVENTS: the parallel headroom being lost, or a zero budget failing every
// test at once.
func TestTestBudget(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		headroom int
		want     time.Duration
		wantErr  string
	}{
		{name: "default", line: "", headroom: 1, want: defaultTestTimeout},
		{name: "declared", line: "option=timeout:value=10s", headroom: 1, want: 10 * time.Second},
		{name: "headroom", line: "option=timeout:value=10s", headroom: 3, want: 30 * time.Second},
		{name: "invalid", line: "option=timeout:value=banana", headroom: 1, wantErr: "option=timeout"},
		{name: "no headroom", line: "", headroom: 0, wantErr: "headroom"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tc, err := parseETFile(timeoutCase(c.line))
			require.NoError(t, err)
			got, err := testBudget(tc, c.headroom)
			if c.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}
