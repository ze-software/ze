package engine

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/transport"
)

// refusingTransport refuses every Send with errSendRefused, the shape of the
// transport's answer for a single-hop peer off its interface's subnets.
type refusingTransport struct{}

var errSendRefused = errors.New("peer on no subnet of the session interface")

func (refusingTransport) Start() error                  { return nil }
func (refusingTransport) Stop() error                   { return nil }
func (refusingTransport) Send(transport.Outbound) error { return errSendRefused }
func (refusingTransport) RX() <-chan transport.Inbound  { return nil }

// VALIDATES: a Control packet the transport refuses is reported at Warn,
// naming the session (peer, interface) and the transport's reason, once per
// sendWarnInterval per session; the repeats inside the window go to Debug.
// PREVENTS: a single-hop session falling Down on Control Detection Time
// Expired with its cause (an unresolvable interface, an off-subnet peer) only
// visible at Debug.
// Method: a Loop over a transport that refuses every Send, a stepped clock,
// and the package logger swapped for one writing to a buffer; three sends at
// 0 s, 10 s and 61 s. Not parallel: it swaps the package logger.
func TestSendFailureWarnsOncePerIntervalNamingTheSession(t *testing.T) {
	var log bytes.Buffer
	saved := engineLog
	engineLog = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	t.Cleanup(func() { engineLog = saved })

	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	l := NewLoop(refusingTransport{}, clk)
	req := reqFor(addrB, addrA)
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	send := func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		entry := l.sessions[req.Key()]
		l.sendLocked(entry, entry.machine.Build())
	}
	warns := func() int { return strings.Count(log.String(), "level=WARN") }

	send()
	if got := warns(); got != 1 {
		t.Fatalf("first refused send: %d Warn lines, want 1:\n%s", got, log.String())
	}
	for _, want := range []string{"peer=" + addrB, "interface=" + req.Interface, errSendRefused.Error()} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("Warn line does not name %q:\n%s", want, log.String())
		}
	}

	clk.now = clk.now.Add(10 * time.Second)
	send()
	if got := warns(); got != 1 {
		t.Errorf("refused send 10s later: %d Warn lines, want still 1:\n%s", got, log.String())
	}
	if !strings.Contains(log.String(), "level=DEBUG") {
		t.Errorf("refused send inside the window left no Debug line:\n%s", log.String())
	}

	clk.now = clk.now.Add(51 * time.Second)
	send()
	if got := warns(); got != 2 {
		t.Errorf("refused send 61s after the first: %d Warn lines, want 2:\n%s", got, log.String())
	}
}
