// Design: docs/architecture/isis/isis-10-auth.md -- the logged authentication error event.
//
// VALIDATES: RFC 5310 section 3.5 "an error event SHOULD be logged" at the
// default log level: a mismatched PDU is discarded with a Warn line naming the
// circuit level and the neighbor SNPA, rate-limited to one line per circuit and
// level per authFailWarnInterval, with the suppressed count on the next line.
// PREVENTS: the error event existing only at Debug, where no operator sees it,
// and a forging neighbor flooding the log.

package isis

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// RFC requirement: RFC5310-3.5-1 positive -- with the logger at Info (Debug
// hidden), an L1 LSP whose CRYPTO_AUTH digest does not match is discarded and a
// WARN "auth verification failed" line names level 1 and the neighbor SNPA; a
// second mismatch 1s later is counted but not logged, and one 11s later logs
// again carrying suppressed=1.
// RFC requirement: RFC5310-3.5-1 negative -- the matching LSP is delivered and
// logs no WARN line.
func TestRFC5310MismatchWarnsRateLimited(t *testing.T) {
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(authTestConfig())
	clock := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	e.authWarn = newAuthWarnLimiter(func() time.Time { return clock })
	var logged bytes.Buffer
	e.log = slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelInfo}))

	delivered := 0
	e.dispatch.register(packet.PDUTypeL1LSP, func(transport.RawFrame) { delivered++ })
	neighbor := [transport.MACLen]byte{0x02, 0x00, 0x5e, 0x00, 0x53, 0x07}

	signed := e.signLevelPDU(authTestLSP(levelOne))
	e.dispatch.dispatch(transport.RawFrame{IfIndex: 10, SrcMAC: neighbor, PDU: signed})
	if delivered != 1 || strings.Contains(logged.String(), "level=WARN") {
		t.Fatalf("matching LSP: delivered %d (want 1), log %q (want no WARN)", delivered, logged.String())
	}

	forged := append([]byte(nil), signed...)
	forged[lspAuthDigestOffset] ^= 0x01
	send := func() { e.dispatch.dispatch(transport.RawFrame{IfIndex: 10, SrcMAC: neighbor, PDU: forged}) }

	send()
	lines := warnLines(logged.String())
	if delivered != 1 || len(lines) != 1 {
		t.Fatalf("first mismatch: delivered %d (want 1), WARN lines %q (want 1)", delivered, lines)
	}
	for _, want := range []string{"auth verification failed", "level=l1", "neighbor-snpa=02:00:5e:00:53:07", "suppressed=0"} {
		if !strings.Contains(lines[0], want) {
			t.Fatalf("WARN line %q lacks %q", lines[0], want)
		}
	}

	clock = clock.Add(time.Second)
	send()
	if lines = warnLines(logged.String()); len(lines) != 1 {
		t.Fatalf("mismatch inside the interval logged again: %q", lines)
	}

	clock = clock.Add(authFailWarnInterval)
	send()
	lines = warnLines(logged.String())
	if len(lines) != 2 || !strings.Contains(lines[1], "suppressed=1") {
		t.Fatalf("mismatch after the interval: WARN lines %q, want a second line with suppressed=1", lines)
	}
	if delivered != 1 {
		t.Fatalf("a mismatched LSP reached the handler (deliveries %d, want 1)", delivered)
	}
}

// warnLines returns the WARN lines of a text-handler log.
func warnLines(log string) []string {
	var out []string
	for line := range strings.SplitSeq(log, "\n") {
		if strings.Contains(line, "level=WARN") {
			out = append(out, line)
		}
	}
	return out
}
