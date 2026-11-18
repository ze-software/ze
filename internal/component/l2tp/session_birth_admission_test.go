// Design: docs/architecture/l2tp/subscriber-session-model.md -- refuse an invalid birth before publication

package l2tp

import (
	"crypto/rand"
	"io"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/report"
)

// TestL2TPSessionBirthRefusesZeroEntropyBeforePublication drives actual ICRQ
// admission with a valid wire SID draw but an invalid all-zero birth draw.
// Admission must answer the peer without publishing the invalid logical row.
// The useful real-admission controls are the separate session-capture tests.
func TestL2TPSessionBirthRefusesZeroEntropyBeforePublication(t *testing.T) {
	original := rand.Reader
	rand.Reader = zeroSessionBirthReader{original: original}
	t.Cleanup(func() { rand.Reader = original })
	t.Cleanup(func() { report.ClearWarning("l2tp", report.CodeInternalDefect, sessionBirthReportSubject) })
	reactor := newReactorForSnapshot(t)
	insertEstablishedTunnel(t, reactor, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"))
	tunnel := reactor.tunnelsByLocalID[100]
	var packet [512]byte
	// RFC 2661 Section 6.6: drive actual incoming-call allocation.
	length := writeICRQBody(packet[:], 70, 70, 0, "", "")
	// RFC 2661 Section 6.6.
	answer := tunnel.handleICRQ(packet[:length], reactor.params.Clock(), reactor.logger)
	require.NotEmpty(t, answer, "the refused call must receive an answer")
	require.Empty(t, tunnel.sessions, "an invalid logical birth must never reach the published row map")
	_, reported, _ := report.SourceWarnings("l2tp", report.CodeInternalDefect, sessionBirthReportSubject)
	require.True(t, reported, "doctor must retain the refused identity allocation")
}

// zeroSessionBirthReader overrides only the two draw lengths belonging to the
// real SID and Birth producers. All unrelated entropy remains untouched.
type zeroSessionBirthReader struct{ original io.Reader }

func (reader zeroSessionBirthReader) Read(buffer []byte) (int, error) {
	if len(buffer) == 2 {
		buffer[0], buffer[1] = 0, 7
		return len(buffer), nil
	}
	if len(buffer) == 16 {
		clear(buffer)
		return len(buffer), nil
	}
	return io.ReadFull(reader.original, buffer)
}
