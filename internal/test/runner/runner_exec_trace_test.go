package runner

import (
	"bytes"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/trace"
)

// TestExecClockStampsStartAndFirstStderr checks that a started command's exec
// step gains the clock marks a timeout investigation needs.
//
// VALIDATES: the step names the clock's wall-clock start in the daemon's log
// layout, the offset at which Start returned and the PID, and the offset of
// the first stderr write, which is never earlier than the start mark.
// PREVENTS: a TIME failure whose budget went before the daemon logged anything
// staying unattributable between the runner's exec, the exec itself and the
// daemon's own startup.
func TestExecClockStampsStartAndFirstStderr(t *testing.T) {
	start := time.Date(2026, 10, 27, 3, 14, 50, 712_000_000, time.UTC)
	rec := &Record{StepTrace: []trace.StepResult{{Step: 1, Kind: stepKindExec, Assert: "ze start x.conf  [no stdin block]", Passed: true}}}
	clock := newExecClock(start)

	var sink bytes.Buffer
	stderr := clock.watch(&sink)
	_, seen := stderr.at()
	require.False(t, seen, "nothing was written yet, so no first-output mark exists")

	clock.mark(0, 4242, stderr)
	_, err := stderr.Write([]byte("created memory store\n"))
	require.NoError(t, err)
	_, err = stderr.Write([]byte("second line\n"))
	require.NoError(t, err)
	assert.Equal(t, "created memory store\nsecond line\n", sink.String(), "the watch passes every write through unchanged")

	clock.stamp(rec)
	got := rec.StepTrace[0].Assert
	match := regexp.MustCompile(`^ze start x\.conf  \[no stdin block\]  \[clock start 2026-10-27T03:14:50\.712Z, started \+([0-9.]+)s pid 4242, first stderr \+([0-9.]+)s\]$`).FindStringSubmatch(got)
	require.NotNil(t, match, "unexpected stamp: %q", got)
	started, err := strconv.ParseFloat(match[1], 64)
	require.NoError(t, err)
	first, err := strconv.ParseFloat(match[2], 64)
	require.NoError(t, err)
	assert.LessOrEqual(t, started, first, "the first write happened after Start returned, so its mark cannot be earlier")
}

// TestExecClockNamesSilentCommand checks the stamp of a command that never
// wrote to stderr.
//
// VALIDATES: the step says no stderr arrived rather than printing an offset.
// PREVENTS: a zero offset reading as output at the very start of the run.
func TestExecClockNamesSilentCommand(t *testing.T) {
	rec := &Record{StepTrace: []trace.StepResult{{Step: 1, Kind: stepKindExec, Assert: "a"}}}
	clock := newExecClock(time.Now())
	clock.mark(0, 7, clock.watch(&bytes.Buffer{}))
	clock.stamp(rec)
	assert.Regexp(t, `^a  \[clock start \S+Z, started \+[0-9.]+s pid 7, no stderr before the run ended\]$`, rec.StepTrace[0].Assert)
}
