package bridge

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestScriptVersionResetsOnStart: each fork starts from the shared setting,
// so a respawned child is undecided again, and `exabgp.api.version` holds a
// script before it writes anything.
//
// VALIDATES: X-2 "for as long as it runs", X-8, AC-10 (the bridge half).
// PREVENTS: a respawned helper keeping the version its previous run detected.
func TestScriptVersionResetsOnStart(t *testing.T) {
	var setting atomic.Uint32
	setting.Store(uint32(APIVersionAuto))
	v := NewScriptVersion("s", slog.New(slog.DiscardHandler), EncoderText, &setting)
	if got := v.Detect("help"); got != APIV4 {
		t.Fatalf("help decides v4 (V4_SIMPLE_TRANSLATIONS), got %v", got)
	}
	if got := v.Detect("peer * announce route 10.0.0.0/24 next-hop 1.2.3.4"); got != APIV4 {
		t.Errorf("a later v6 line moved a held helper to %v", got)
	}
	if v.Format() != FormatV4Text {
		t.Errorf("format = %v, want v4 text", v.Format())
	}
	v.Reset()
	if got := v.Current(); got != APIVersionAuto {
		t.Errorf("after a restart the helper is %v, want undecided", got)
	}

	setting.Store(uint32(APIV6))
	v.Reset()
	if got := v.Current(); got != APIV6 {
		t.Errorf("with the setting 6 the helper starts %v", got)
	}
	if v.Format() != FormatV6JSON {
		t.Errorf("format with encoder text held to v6 = %v, want v6 json", v.Format())
	}
}

// TestStandaloneRunnerHeldToDetectedVersion drives the standalone runner
// (`ze exabgp plugin` over MuxConn) with a script whose first deciding line is
// v4, and reads back what it answered, what it dispatched and what it fed.
//
// VALIDATES: the standalone runner holds its script to a version exactly as
// the plugin runner does: X-2 (first deciding line), X-3 (a v6 line from a v4
// helper answers error and reaches no peer), X-5/X-6 (its JSON events carry the
// v4 envelope), X-7 (`api version` answered by the runner). AC-5, AC-9.
// PREVENTS: the standalone runner staying 6.0-only beside a versioned plugin
// runner, which is two behaviors for one script.
func TestStandaloneRunnerHeldToDetectedVersion(t *testing.T) {
	b := NewBridge([]string{"echo"})
	b.running = true
	ctx := context.Background()

	// The v4 announce is dispatched (id 1) and flushed (id 2).
	pending := newPendingResponses()
	go func() {
		require.Eventually(t, func() bool { return pending.isWaiting(1) },
			2*time.Second, time.Millisecond, "dispatch waiter (id 1) should register")
		pending.signal(1, pendingResult{ok: true})
		require.Eventually(t, func() bool { return pending.isWaiting(2) },
			2*time.Second, time.Millisecond, "flush waiter (id 2) should register")
		pending.signal(2, pendingResult{ok: true})
	}()
	var zeBuf, answers strings.Builder
	zeOut := &syncWriter{w: &zeBuf}
	b.pluginToZebgp(ctx, strings.NewReader("announce route 1.1.1.1/32 next-hop 11.11.11.11\n"), &answers, zeOut, pending)
	if b.api.Current() != APIV4 {
		t.Fatalf("after a v4 announce the script is held to %v, want 4", b.api.Current())
	}

	b.pluginToZebgp(ctx, strings.NewReader("peer * announce route 2.2.2.2/32 next-hop 22.22.22.22\napi version\n"),
		&answers, zeOut, newPendingResponses())
	got := strings.Split(strings.TrimSpace(answers.String()), "\n")
	if len(got) != 4 {
		t.Fatalf("answers = %q, want four lines", got)
	}
	if got[0] != "done" {
		t.Errorf("answer to the v4 announce = %q, want done", got[0])
	}
	if !strings.HasPrefix(got[1], "error") {
		t.Errorf("answer to the v6 announce from a v4 helper = %q, want error", got[1])
	}
	if got[2] != "API version: auto (this helper: 4)" || got[3] != "done" {
		t.Errorf("answer to api version = %q, %q", got[2], got[3])
	}
	if strings.Contains(zeBuf.String(), "2.2.2.2/32") {
		t.Errorf("the refused v6 line reached the engine: %q", zeBuf.String())
	}

	var events strings.Builder
	b.translateAndForward(`{"type":"state","peer":{"address":"127.0.0.1"},"state":"up"}`, &events)
	if !strings.Contains(events.String(), `"exabgp": "4.0.1"`) {
		t.Errorf("event to a v4 helper = %q, want the 4.0.1 envelope", events.String())
	}
}
