package report

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// failWriter always returns an error on Write.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

// TestSummaryPassOutput verifies that a passing scenario produces
// the expected output format with PASS verdict.
//
// VALIDATES: PASS verdict and key metrics in output.
// PREVENTS: Missing or malformatted summary fields.
func TestSummaryPassOutput(t *testing.T) {
	s := Summary{
		Seed:       12345,
		Duration:   30 * time.Second,
		PeerCount:  4,
		Announced:  100,
		Received:   300,
		Missing:    0,
		Extra:      0,
		MinLatency: 10 * time.Millisecond,
		AvgLatency: 50 * time.Millisecond,
		MaxLatency: 200 * time.Millisecond,
		P99Latency: 180 * time.Millisecond,
	}

	var buf bytes.Buffer
	exitCode := s.Write(&buf)

	output := buf.String()
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "PASS")
	assert.Contains(t, output, "12345")
	assert.Contains(t, output, "30s")
	assert.Contains(t, output, "4 peers")
	assert.Contains(t, output, "100 announced")
	assert.Contains(t, output, "300 received")
	assert.Contains(t, output, "0 missing")
	assert.Contains(t, output, "0 extra")
}

// TestSummaryFailOutput verifies that a failing scenario produces
// FAIL verdict and exit code 1.
//
// VALIDATES: FAIL verdict when missing or extra routes present.
// PREVENTS: False PASS on validation failure.
func TestSummaryFailOutput(t *testing.T) {
	s := Summary{
		Seed:       99999,
		Duration:   10 * time.Second,
		PeerCount:  3,
		Announced:  50,
		Received:   140,
		Missing:    5,
		Extra:      2,
		MinLatency: 5 * time.Millisecond,
		AvgLatency: 100 * time.Millisecond,
		MaxLatency: 4 * time.Second,
		P99Latency: 3 * time.Second,
		SlowRoutes: 3,
	}

	var buf bytes.Buffer
	exitCode := s.Write(&buf)

	output := buf.String()
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "5 missing")
	assert.Contains(t, output, "2 extra")
	assert.Contains(t, output, "3 slow")
}

// TestSummaryLatencyFormatting verifies that latency values are
// formatted in human-readable form.
//
// VALIDATES: Latency stats appear in output.
// PREVENTS: Missing convergence metrics.
func TestSummaryLatencyFormatting(t *testing.T) {
	s := Summary{
		Seed:       1,
		Duration:   5 * time.Second,
		PeerCount:  2,
		Announced:  10,
		Received:   10,
		MinLatency: 1 * time.Millisecond,
		AvgLatency: 50 * time.Millisecond,
		MaxLatency: 100 * time.Millisecond,
		P99Latency: 95 * time.Millisecond,
	}

	var buf bytes.Buffer
	s.Write(&buf)

	output := buf.String()
	assert.Contains(t, output, "min=1ms")
	assert.Contains(t, output, "avg=50ms")
	assert.Contains(t, output, "max=100ms")
	assert.Contains(t, output, "p99=95ms")
}

// TestSummaryZeroLatency verifies output when no latency data exists.
//
// VALIDATES: No crash when convergence tracking has no data.
// PREVENTS: Division by zero or nil dereference.
func TestSummaryZeroLatency(t *testing.T) {
	s := Summary{
		Seed:      42,
		Duration:  1 * time.Second,
		PeerCount: 2,
		Announced: 0,
		Received:  0,
	}

	var buf bytes.Buffer
	exitCode := s.Write(&buf)

	output := buf.String()
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "PASS")
	assert.Contains(t, output, "0 announced")
}

// TestSummaryChaosStats verifies that chaos injection stats appear
// in the output when ChaosEvents > 0.
//
// VALIDATES: Chaos line printed with event, reconnection, and withdrawal counts.
// PREVENTS: Chaos stats silently missing from summary.
func TestSummaryChaosStats(t *testing.T) {
	s := Summary{
		Seed:          42,
		Duration:      60 * time.Second,
		PeerCount:     4,
		Announced:     100,
		Received:      280,
		ChaosEvents:   8,
		Reconnections: 3,
		Withdrawn:     45,
	}

	var buf bytes.Buffer
	exitCode := s.Write(&buf)

	output := buf.String()
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "chaos: 8 events, 3 reconnections, 45 withdrawn")
}

// TestSummaryChaosStatsHiddenWhenZero verifies that the chaos line
// is omitted when no chaos events occurred.
//
// VALIDATES: No chaos line when chaos is disabled.
// PREVENTS: Noisy output showing "0 events, 0 reconnections, 0 withdrawn".
func TestSummaryChaosStatsHiddenWhenZero(t *testing.T) {
	s := Summary{
		Seed:      42,
		Duration:  10 * time.Second,
		PeerCount: 2,
		Announced: 50,
		Received:  50,
	}

	var buf bytes.Buffer
	s.Write(&buf)

	output := buf.String()
	assert.NotContains(t, output, "chaos:")
}

// TestSummaryWriteError verifies that a write failure returns exit code 1
// even when the validation itself passes.
//
// VALIDATES: Write errors produce non-zero exit code.
// PREVENTS: Silent success when summary output fails.
func TestSummaryWriteError(t *testing.T) {
	s := Summary{
		Seed:      42,
		Duration:  1 * time.Second,
		PeerCount: 2,
	}

	exitCode := s.Write(failWriter{})
	assert.Equal(t, 1, exitCode, "write failure should return exit code 1")
}

// TestSummaryIBGPEBGP verifies iBGP/eBGP counts appear when both types are present.
//
// VALIDATES: Summary shows iBGP/eBGP breakdown when mixed.
// PREVENTS: Missing peer type info in mixed deployments.
func TestSummaryIBGPEBGP(t *testing.T) {
	s := Summary{
		Seed:       42,
		Duration:   30 * time.Second,
		PeerCount:  4,
		IBGPCount:  1,
		EBGPCount:  3,
		Announced:  100,
		Received:   300,
		MinLatency: 10 * time.Millisecond,
		AvgLatency: 50 * time.Millisecond,
		MaxLatency: 200 * time.Millisecond,
		P99Latency: 180 * time.Millisecond,
	}

	var buf bytes.Buffer
	s.Write(&buf)

	output := buf.String()
	assert.Contains(t, output, "1 iBGP")
	assert.Contains(t, output, "3 eBGP")
}

// TestSummaryPropertiesOutput verifies per-property pass/fail lines in output.
//
// VALIDATES: Property lines appear with PASS/FAIL status when properties are set.
// PREVENTS: Missing property results in summary output.
func TestSummaryPropertiesOutput(t *testing.T) {
	s := Summary{
		Seed:      42,
		Duration:  30 * time.Second,
		PeerCount: 4,
		Announced: 100,
		Received:  300,
		Properties: []PropertyLine{
			{Name: "route-consistency", Pass: true},
			{Name: "message-ordering", Pass: false},
		},
	}

	var buf bytes.Buffer
	exitCode := s.Write(&buf)

	output := buf.String()
	assert.Equal(t, 1, exitCode, "failing property should produce exit code 1")
	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "properties:")
	assert.Contains(t, output, "route-consistency")
	assert.Contains(t, output, "PASS")
	assert.Contains(t, output, "message-ordering")
}

// TestSummaryPropertiesHiddenWhenEmpty verifies no properties section when empty.
//
// VALIDATES: No properties line when --properties not used.
// PREVENTS: Noisy output when properties are disabled.
func TestSummaryPropertiesHiddenWhenEmpty(t *testing.T) {
	s := Summary{
		Seed:      42,
		Duration:  10 * time.Second,
		PeerCount: 2,
		Announced: 50,
		Received:  50,
	}

	var buf bytes.Buffer
	s.Write(&buf)

	output := buf.String()
	assert.NotContains(t, output, "properties:")
}

// TestSummaryPropertiesAllPass verifies PASS when all properties pass.
//
// VALIDATES: All-pass properties still produce overall PASS.
// PREVENTS: Property section causing false FAIL.
func TestSummaryPropertiesAllPass(t *testing.T) {
	s := Summary{
		Seed:      42,
		Duration:  10 * time.Second,
		PeerCount: 2,
		Announced: 50,
		Received:  50,
		Properties: []PropertyLine{
			{Name: "route-consistency", Pass: true},
			{Name: "no-duplicate-routes", Pass: true},
		},
	}

	var buf bytes.Buffer
	exitCode := s.Write(&buf)

	assert.Equal(t, 0, exitCode, "all-pass properties should produce exit code 0")
}

// TestSummaryIBGPEBGPHidden verifies iBGP/eBGP line omitted when all same type.
//
// VALIDATES: No iBGP/eBGP line when all peers are the same type.
// PREVENTS: Noisy output for pure iBGP or pure eBGP deployments.
func TestSummaryIBGPEBGPHidden(t *testing.T) {
	s := Summary{
		Seed:      42,
		Duration:  10 * time.Second,
		PeerCount: 4,
		IBGPCount: 0,
		EBGPCount: 4,
		Announced: 50,
		Received:  150,
	}

	var buf bytes.Buffer
	s.Write(&buf)

	output := buf.String()
	assert.NotContains(t, output, "iBGP")
	assert.NotContains(t, output, "eBGP")
}
