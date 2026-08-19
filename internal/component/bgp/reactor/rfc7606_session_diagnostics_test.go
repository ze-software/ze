package reactor

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// captureSessionLog swaps the package's session logger for one writing into a buffer at
// the given level, restoring it afterwards.
//
// sessionLogger is a package function backed by an atomic.Value (session.go), built by
// slogutil.LazyLogger over its own handler. slog.SetDefault does NOT intercept it, so the
// usual default-logger recorder used elsewhere in this package cannot see these lines.
// swapSessionLogger overrides it through that atomic.Value so the override is race-free
// against any live session's cold-path logging.
//
// The sink is a syncBuffer, not a bytes.Buffer: the override is process-global, so every
// live session's background goroutines (keepalive and hold timers, the cancel goroutine)
// write into it while the test reads it. A bare bytes.Buffer here is a data race, seen
// under `make ze-unit-reactor-test-race` between a keepalive timer's Debug line and this test's
// String().
func captureSessionLog(t *testing.T, level slog.Level) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	lg := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: level}))
	t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))
	return buf
}

func rfc7606DiagSession() *Session {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.ReceiveHoldTime = 90 * time.Second
	return NewSession(settings)
}

// malformedOriginUpdate announces 10.0.0.0/8 with an ORIGIN of length 2, which RFC 7606
// Section 7.1 makes malformed and Section 2 turns into treat-as-withdraw.
func malformedOriginUpdate() []byte {
	attrs := []byte{
		0x40, 0x01, 0x02, 0x00, 0x00, // ORIGIN, length 2 (invalid)
		0x40, 0x02, 0x00, // AS_PATH (empty)
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP
	}
	return makeUpdateBody(nil, attrs, []byte{0x08, 0x0a}) // NLRI 10.0.0.0/8
}

// VALIDATES: RFC 7606 Section 6 — when a malformed attribute is detected, the debugging
// facility logs the NLRI involved AND the entire malformed UPDATE.
// PREVENTS: the pre-existing behavior, where every RFC 7606 log line carried only an
// attribute code and a description, leaving an operator unable to identify which routes
// were affected or to reconstruct what the peer actually sent.
//
// The dump is deliberately the UPDATE BODY: the 19-octet header is a fixed marker plus
// length and type and carries no diagnostic information. The log key says so.
//
// RFC requirement: RFC7606-6-1 positive -- the debugging facility lists the NLRI involved
// and contains the malformed UPDATE when a malformed attribute is detected.
func TestRFC7606DiagnosticsListNLRIAndUpdate(t *testing.T) {
	buf := captureSessionLog(t, slog.LevelDebug)
	s := rfc7606DiagSession()

	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(malformedOriginUpdate(), 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionTreatAsWithdraw, action)

	out := buf.String()
	require.Contains(t, out, "RFC 7606 diagnostics")
	assert.Contains(t, out, "nlri-prefixes", "the NLRI involved must be listed")
	assert.Contains(t, out, "10.0.0.0/8", "the affected prefix must be identifiable")
	assert.Contains(t, out, "update-body-hex", "the malformed UPDATE must be present")
	// The ORIGIN attribute's malformed bytes, as the peer sent them, must be recoverable
	// from the dump: 0x40 0x01 0x02 is the attribute header with the invalid length.
	assert.Contains(t, out, "400102", "the dump must be the UPDATE the peer actually sent")
}

// VALIDATES: the facility costs nothing when it is switched off.
// PREVENTS: the amplification a hostile peer would aim for. slog evaluates arguments
// eagerly, so an unguarded hex encode would run for every malformed UPDATE even with debug
// logging disabled; a peer sending malformed UPDATEs in a loop would pay ze to hex-encode
// them. The Enabled() guard is what makes the untruncated dump affordable.
//
// RFC requirement: RFC7606-6-1 negative -- with the debugging facility disabled no
// diagnostic line is emitted, so the requirement is met by a facility rather than by
// unconditional logging.
func TestRFC7606DiagnosticsSilentWhenDebugDisabled(t *testing.T) {
	buf := captureSessionLog(t, slog.LevelWarn)
	s := rfc7606DiagSession()

	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(malformedOriginUpdate(), 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionTreatAsWithdraw, action)

	assert.NotContains(t, buf.String(), "RFC 7606 diagnostics")
}

// VALIDATES: a session-reset carries the same detail as the other two actions.
// PREVENTS: the most damaging outcome being the least diagnosable. A reset tears the
// session down, so the operator has only the log to work from.
//
// RFC requirement: RFC7606-6-1 positive -- the facility covers the session-reset action,
// not only the treat-as-withdraw one.
func TestRFC7606DiagnosticsCoverSessionReset(t *testing.T) {
	buf := captureSessionLog(t, slog.LevelDebug)
	s := rfc7606DiagSession()

	// Withdrawn Routes Length larger than the message: Section 3(b), session reset.
	body := []byte{0x00, 0x20, 0x00, 0x00}
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.Error(t, err)
	require.Equal(t, message.RFC7606ActionSessionReset, action)

	out := buf.String()
	assert.Contains(t, out, "RFC 7606 diagnostics")
	assert.Contains(t, out, "update-body-hex")
}

// VALIDATES: the prefix decoder reports what it can and stops, on input already known to
// be malformed.
// PREVENTS: a decoder that gives up entirely on the first bad byte, which would leave the
// "listing the NLRI involved" half of Section 6 empty exactly when it is needed.
func TestIPv4PrefixListTolerance(t *testing.T) {
	for _, tc := range []struct {
		name    string
		field   []byte
		addPath bool
		want    []string
	}{
		{"two prefixes", []byte{0x08, 0x0a, 0x18, 0xc0, 0x00, 0x02}, false,
			[]string{"10.0.0.0/8", "192.0.2.0/24"}},
		{"host route", []byte{0x20, 0x0a, 0x00, 0x00, 0x01}, false, []string{"10.0.0.1/32"}},
		{"default route", []byte{0x00}, false, []string{"0.0.0.0/0"}},
		{"prefix length over 32", []byte{0x21, 0x0a}, false, []string{"invalid-prefix-length/33"}},
		{"truncated prefix", []byte{0x18, 0x0a}, false, []string{"truncated-prefix"}},
		{"good then truncated", []byte{0x08, 0x0a, 0x18, 0x0a}, false,
			[]string{"10.0.0.0/8", "truncated-prefix"}},
		{"add-path path id skipped", []byte{0x00, 0x00, 0x00, 0x2a, 0x08, 0x0a}, true,
			[]string{"10.0.0.0/8"}},
		{"empty", nil, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ipv4PrefixList(tc.field, tc.addPath))
		})
	}
}
