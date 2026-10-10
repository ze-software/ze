package testing

import (
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
