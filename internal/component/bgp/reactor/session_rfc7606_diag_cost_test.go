package reactor

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
)

// VALIDATES: with the debugging facility switched off, producing the RFC 7606 Section 6
// diagnostics costs nothing at all.
// PREVENTS: the amplification a hostile peer would aim for. slog evaluates its arguments
// EAGERLY, so without the Enabled() guard in rfc7606Diagnostics every malformed UPDATE
// would pay for a full hex encode of its body plus two NLRI walks, discarded immediately
// afterwards by the handler. A peer sending malformed UPDATEs in a loop would be spending
// ze's CPU for free.
//
// Allocation is the right probe because it is what distinguishes the guard from its
// absence. A test that merely asserts "no log line appears" cannot: a level-filtering
// handler drops the line whether or not the arguments were built, so it passes on the
// unguarded implementation too. That was true of the first version of this test, and the
// audit note for RFC7606-6-1 records the correction.
//
// RFC requirement: RFC7606-6-1 negative -- the facility is inert when disabled, so meeting
// Section 6 does not hand a peer a cost-amplification lever.
func TestRFC7606DiagnosticsCostsNothingWhenDisabled(t *testing.T) {
	// A handler that reports Debug disabled, exactly as the default WARN level would.
	// swapSessionLogger overrides the provider through session.go's atomic.Value, so the
	// override is race-free against any live session's cold-path logging (a plain package-var
	// swap raced leaked timer callbacks under stress).
	lg := slog.New(slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelWarn}))
	t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))

	s := rfc7606DiagSession()
	wu := wireu.NewWireUpdate(malformedOriginUpdate(), 0)

	allocs := testing.AllocsPerRun(50, func() {
		s.rfc7606Diagnostics("treat-as-withdraw", wu, 1, "RFC 7606 Section 7.1: ORIGIN length 2")
	})
	assert.Zerof(t, allocs,
		"the disabled facility allocated %.1f times per call; the Enabled() guard must "+
			"return before building the hex dump and the prefix lists", allocs)
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
