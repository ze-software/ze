package ldp

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestLDPLabelDoctorReportsRefusedEntry proves the ldp-label-entries check
// reports a label entry the writer refused, names the entry and the writer's
// cause under doctor-fib-mpls-no-writer with a fix, and clears it once a
// later apply of the same in-label succeeds.
//
// VALIDATES: AC-22 -- an entry no writer installs reaches `show doctor` with
// its cause, not only the log.
// METHOD: a writer on the production bus refuses the first pop; the finding
// is read through labelDoctorFindings; the writer then accepts and LDP
// programs the same in-label again.
func TestLDPLabelDoctorReportsRefusedEntry(t *testing.T) {
	writer := &labelWriter{refusal: errors.New("vpp: mpls_route_add_del retval -6")}
	bus := newLabelBus(t, writer)
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.1.0.1/32")

	require.Empty(t, fib.labelDoctorFindings(), "nothing refused yet")
	fib.programPop(fec, 18000)

	findings := fib.labelDoctorFindings()
	require.Len(t, findings, 1)
	assert.Equal(t, mplsfibevents.CodeDoctorNoWriter, findings[0].Code)
	assert.Equal(t, "error", findings[0].Severity)
	assert.Contains(t, findings[0].Message, "in-label 18000")
	assert.Contains(t, findings[0].Message, "10.1.0.1/32")
	assert.Contains(t, findings[0].Message, "retval -6", "the writer's cause is named")
	assert.NotEmpty(t, findings[0].Fix)

	writer.refusal = nil
	fib.programPop(fec, 18000)
	assert.Empty(t, fib.labelDoctorFindings(), "an accepted apply clears the refusal")
}

// TestLDPLabelDoctorBoundsFindings proves one answer lists at most 32
// entries and counts the rest in one more finding.
//
// VALIDATES: the check stays bounded when every FEC of a large IGP is refused.
// METHOD: a writer on the production bus refuses 37 pops LDP programs; the
// answer is read through labelDoctorFindings.
func TestLDPLabelDoctorBoundsFindings(t *testing.T) {
	writer := &labelWriter{refusal: errors.New("no writer acknowledged")}
	fib := newLDPFIB(newLabelBus(t, writer), slogutil.DiscardLogger())
	for i := range 37 {
		fib.programPop(netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 1, 0, byte(i)}), 32), uint32(16+i))
	}
	findings := fib.labelDoctorFindings()
	require.Len(t, findings, 33)
	assert.True(t, strings.Contains(findings[32].Message, "5 more"), findings[32].Message)
	assert.Equal(t, mplsfibevents.CodeDoctorNoWriter, findings[32].Code)
}
