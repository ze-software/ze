package replay

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDiffIdenticalLogs verifies that identical logs produce no divergence.
//
// VALIDATES: Two identical event logs → exit code 0, no divergence reported.
// PREVENTS: False positives from diff tool on identical input.
func TestDiffIdenticalLogs(t *testing.T) {
	log := buildNDJSON(
		makeHeader(42, 2),
		makeEvent(1, "established", 0),
		makeEvent(2, "route-sent", 0, withPrefix("10.0.0.0/24")),
		makeEvent(3, "route-received", 1, withPrefix("10.0.0.0/24")),
	)

	var out bytes.Buffer
	exitCode := Diff(strings.NewReader(log), strings.NewReader(log), &out)

	assert.Equal(t, 0, exitCode, "identical logs should return 0")
	assert.Contains(t, out.String(), "identical")
}

// TestDiffDivergentLogs verifies that divergent logs report the first difference.
//
// VALIDATES: First divergence point is identified with seq number and context.
// PREVENTS: Diff tool missing divergence or reporting wrong location.
func TestDiffDivergentLogs(t *testing.T) {
	log1 := buildNDJSON(
		makeHeader(42, 2),
		makeEvent(1, "established", 0),
		makeEvent(2, "route-sent", 0, withPrefix("10.0.0.0/24")),
		makeEvent(3, "route-received", 1, withPrefix("10.0.0.0/24")),
	)
	log2 := buildNDJSON(
		makeHeader(42, 2),
		makeEvent(1, "established", 0),
		makeEvent(2, "route-sent", 0, withPrefix("10.0.1.0/24")), // different prefix
		makeEvent(3, "route-received", 1, withPrefix("10.0.1.0/24")),
	)

	var out bytes.Buffer
	exitCode := Diff(strings.NewReader(log1), strings.NewReader(log2), &out)

	assert.Equal(t, 1, exitCode, "divergent logs should return 1")
	output := out.String()
	assert.Contains(t, output, "divergence")
	assert.Contains(t, output, "seq 2") // first difference at seq 2
}

// TestDiffDifferentLengths verifies that logs of different lengths are detected.
//
// VALIDATES: Shorter log ending early is reported as divergence.
// PREVENTS: Diff silently ignoring trailing events in the longer log.
func TestDiffDifferentLengths(t *testing.T) {
	log1 := buildNDJSON(
		makeHeader(42, 2),
		makeEvent(1, "established", 0),
		makeEvent(2, "route-sent", 0, withPrefix("10.0.0.0/24")),
	)
	log2 := buildNDJSON(
		makeHeader(42, 2),
		makeEvent(1, "established", 0),
	)

	var out bytes.Buffer
	exitCode := Diff(strings.NewReader(log1), strings.NewReader(log2), &out)

	assert.Equal(t, 1, exitCode, "different lengths should return 1")
	assert.Contains(t, out.String(), "length")
}

// TestDiffEventTypeDivergence verifies divergence on event type.
//
// VALIDATES: Different event types at same seq produce divergence.
// PREVENTS: Diff comparing only prefixes and missing event type changes.
func TestDiffEventTypeDivergence(t *testing.T) {
	log1 := buildNDJSON(
		makeHeader(42, 2),
		makeEvent(1, "established", 0),
		makeEvent(2, "route-sent", 0, withPrefix("10.0.0.0/24")),
	)
	log2 := buildNDJSON(
		makeHeader(42, 2),
		makeEvent(1, "established", 0),
		makeEvent(2, "disconnected", 0), // different event type
	)

	var out bytes.Buffer
	exitCode := Diff(strings.NewReader(log1), strings.NewReader(log2), &out)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "divergence")
}

// TestDiffHeaderOnly verifies that header-only logs are identical.
//
// VALIDATES: Two logs with only headers and no events → identical.
// PREVENTS: Edge case where no events causes incorrect divergence.
func TestDiffHeaderOnly(t *testing.T) {
	log := buildNDJSON(makeHeader(42, 2))

	var out bytes.Buffer
	exitCode := Diff(strings.NewReader(log), strings.NewReader(log), &out)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "identical")
}

// VALIDATES: A changed withdrawal list locates the difference in a v2 capture.
// PREVENTS: Reporting identical withdrawals that affect different prefixes.
func TestDiffObservationWithdrawals(t *testing.T) {
	header := makeHeader(42, 2)
	header["version"] = 2
	end := map[string]any{"record-type": "observation-end", "seq": 2}
	log1 := buildNDJSON(header, makeEvent(1, "withdrawal-sent", 0, func(event map[string]any) {
		event["withdrawn-prefixes"] = []string{"2001:db8::/48"}
	}), end)
	log2 := buildNDJSON(header, makeEvent(1, "withdrawal-sent", 0, func(event map[string]any) {
		event["withdrawn-prefixes"] = []string{"2001:db8:1::/48"}
	}), end)

	var out bytes.Buffer
	assert.Equal(t, 1, Diff(strings.NewReader(log1), strings.NewReader(log2), &out))
	assert.Contains(t, out.String(), "seq 1")
}

// VALIDATES: The observation checkpoint ends the comparison of a v2 capture.
// PREVENTS: Different cleanup events appearing as differences in the run.
func TestDiffObservationEndsBeforeTeardown(t *testing.T) {
	header := makeHeader(42, 2)
	header["version"] = 2
	end := map[string]any{"record-type": "observation-end", "seq": 2}
	first := makeEvent(1, "established", 0)
	log1 := buildNDJSON(header, first, end)
	log2 := buildNDJSON(header, first, end, makeEvent(3, "disconnected", 0))

	var out bytes.Buffer
	assert.Equal(t, 0, Diff(strings.NewReader(log1), strings.NewReader(log2), &out))
	assert.Contains(t, out.String(), "identical")
}
