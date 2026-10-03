// Design: docs/architecture/isis/isis-10-auth.md -- the error event on an authentication mismatch.
//
// VALIDATES: RFC 5310 section 3.5 as a whole at the production receive path
// (dispatcher -> verifyFrame): a PDU whose calculated data matches is delivered
// and records no error event; a PDU whose authentication data does not match is
// discarded AND the error event is recorded, both as the
// ze_isis_auth_failures_total sample for its level and interface and as a log
// line naming the failure.
// PREVENTS: a silent discard, where the mismatched PDU is dropped but neither the
// counter nor the log records it.

package isis

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// countingVec records every With(...).Inc() by its joined label values.
type countingVec struct {
	counts map[string]float64
}

type countingCounter struct {
	vec *countingVec
	key string
}

func (c countingCounter) Inc()          { c.vec.counts[c.key]++ }
func (c countingCounter) Add(v float64) { c.vec.counts[c.key] += v }

func (v *countingVec) With(labelValues ...string) metrics.Counter {
	return countingCounter{vec: v, key: strings.Join(labelValues, "/")}
}

func (v *countingVec) Delete(...string) bool { return false }

// RFC requirement: RFC5310-3.5-1 positive -- an L1 LSP whose CRYPTO_AUTH data
// matches is delivered to the LSP handler and records no error event: the
// auth-failure counter stays empty and no failure is logged.
// RFC requirement: RFC5310-3.5-1 negative -- the same LSP with one digest octet
// changed is discarded before the handler, and the error event is recorded: one
// ze_isis_auth_failures_total sample for level 1 and a logged "auth verification
// failed" line.
func TestRFC5310MismatchLogsAnErrorEvent(t *testing.T) {
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(authTestConfig())
	failures := &countingVec{counts: map[string]float64{}}
	e.ksMu.Lock()
	e.authFailures = failures
	e.ksMu.Unlock()
	var logged bytes.Buffer
	e.log = slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))

	delivered := 0
	e.dispatch.register(packet.PDUTypeL1LSP, func(transport.RawFrame) { delivered++ })

	signed := e.signLevelPDU(authTestLSP(levelOne))
	e.dispatch.dispatch(transport.RawFrame{IfIndex: 10, PDU: signed})
	if delivered != 1 {
		t.Fatalf("matching LSP delivered %d times, want 1", delivered)
	}
	if len(failures.counts) != 0 || strings.Contains(logged.String(), "auth verification failed") {
		t.Fatalf("matching LSP recorded an error event: counter %v, log %q", failures.counts, logged.String())
	}

	forged := append([]byte(nil), signed...)
	forged[lspAuthDigestOffset] ^= 0x01
	e.dispatch.dispatch(transport.RawFrame{IfIndex: 10, PDU: forged})
	if delivered != 1 {
		t.Fatalf("mismatched LSP reached the handler (deliveries %d, want 1)", delivered)
	}
	var total float64
	for key, n := range failures.counts {
		if !strings.HasPrefix(key, ksLevelToken(levelOne)+"/") {
			t.Fatalf("auth failure recorded under labels %q, want level %q first", key, ksLevelToken(levelOne))
		}
		total += n
	}
	if total != 1 {
		t.Fatalf("mismatched LSP recorded %v auth failures, want 1", total)
	}
	if !strings.Contains(logged.String(), "auth verification failed") {
		t.Fatalf("mismatched LSP discarded with no logged error event; log %q", logged.String())
	}
}
