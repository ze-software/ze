package replay

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: Run and Diff report a truncated event log as an error (2) instead
// of judging the run by the events that happened to arrive.
// PREVENTS: `logs identical` over two logs cut at the same point, a length
// divergence reported for a log that was never short, and a replay verdict
// drawn from half an event log.

// failingReader yields prefix, then fails. It models a log file on a device
// that stops answering part way through.
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

const replayHeader = `{"record-type":"header","peers":2,"seed":7,"start-time":"2026-01-01T00:00:00Z"}` + "\n"

// replayEvent builds one route-sent event line with the given sequence.
func replayEvent(seq int) string {
	return `{"record-type":"event","seq":` + string(rune('0'+seq)) + `,"event-type":"route-sent","peer-index":0}` + "\n"
}

// TestRunReportsTruncatedLog drives a log that stops part way through an event
// line. Without the check the surviving events produce a normal pass/fail
// summary, which is a verdict over an input nobody read in full.
func TestRunReportsTruncatedLog(t *testing.T) {
	body := replayHeader + replayEvent(1) + `{"record-type":"event","seq":2,"eve`
	r := &failingReader{prefix: []byte(body), err: errors.New("device stopped")}

	var out bytes.Buffer
	code := Run(r, &out)

	assert.Equal(t, 2, code, "a truncated log must not produce a replay verdict")
	assert.Contains(t, out.String(), "reading event log")
}

// TestRunOverLongLine covers the failure mode with no underlying I/O error: an
// event line above bufio.MaxScanTokenSize.
func TestRunOverLongLine(t *testing.T) {
	body := replayHeader + `{"record-type":"event","prefix":"` + strings.Repeat("a", bufio.MaxScanTokenSize) + `"}` + "\n"

	var out bytes.Buffer
	code := Run(strings.NewReader(body), &out)

	assert.Equal(t, 2, code)
	assert.Contains(t, out.String(), "reading event log")
}

// TestRunCompleteLogStillReports pins the other side: a whole log still reaches
// the summary, so the check above is not refusing every run.
func TestRunCompleteLogStillReports(t *testing.T) {
	body := replayHeader + replayEvent(1)

	var out bytes.Buffer
	code := Run(strings.NewReader(body), &out)

	assert.NotEqual(t, 2, code, "a complete log must still be validated")
	assert.NotContains(t, out.String(), "reading event log")
}

// TestDiffReportsTruncatedLog drives two logs cut at the same point. Both
// scanners stop together, which reads as "both exhausted" and prints the
// identical verdict.
func TestDiffReportsTruncatedLog(t *testing.T) {
	body := replayHeader + replayEvent(1)
	stop := errors.New("device stopped")

	var out bytes.Buffer
	code := Diff(
		&failingReader{prefix: []byte(body), err: stop},
		&failingReader{prefix: []byte(body), err: stop},
		&out,
	)

	assert.Equal(t, 2, code, "two truncated logs must not read as identical")
	assert.NotContains(t, out.String(), "identical")
	assert.Contains(t, out.String(), "reading log")
}

// TestDiffReportsTruncatedFirstLog drives one truncated log against a whole
// one. The truncation otherwise reads as a length divergence, which blames the
// log rather than the read.
func TestDiffReportsTruncatedFirstLog(t *testing.T) {
	whole := replayHeader + replayEvent(1) + replayEvent(2)
	cut := replayHeader + replayEvent(1)

	var out bytes.Buffer
	code := Diff(
		&failingReader{prefix: []byte(cut), err: errors.New("device stopped")},
		strings.NewReader(whole),
		&out,
	)

	assert.Equal(t, 2, code, "a failed read must not be reported as a length mismatch")
	assert.Contains(t, out.String(), "reading log1")
}

// TestDiffIdenticalLogsStillPass pins the other side.
func TestDiffIdenticalLogsStillPass(t *testing.T) {
	body := replayHeader + replayEvent(1)

	var out bytes.Buffer
	code := Diff(strings.NewReader(body), strings.NewReader(body), &out)

	require.Equal(t, 0, code)
	assert.Contains(t, out.String(), "identical")
}
