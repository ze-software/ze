// Design: docs/guide/radius.md -- panic containment in the client's read task
// Related: panic_guard.go -- readTask, the boundary under test
// Related: rfc3579_client_test.go -- unsignedReply, the answer that reaches the seam

package radius

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
)

// TestRADIUSClientReadPanicContained proves the client's read task contains a
// Ze defect met while it dispatches one server's answer: the reader carries on,
// the next exchange on the same client completes, and the panic is counted and
// crash-recorded on the real guard.
// METHOD: the metric set holds no discard counter, a state only a defect
// produces since bindRadiusMetrics always fills it. Server A answers the first
// request with an EAP-bearing answer and no Message-Authenticator, which
// dispatchResponse counts through that empty counter on the read goroutine, and
// ignores the rest. Server B answers correctly, signed with a
// Message-Authenticator as the default server row requires. One client asks A,
// then B.
// PREVENTS: a server-driven panic in the answer reader killing the daemon, or
// ending the reader and with it every later RADIUS login and accounting answer.
func TestRADIUSClientReadPanicContained(t *testing.T) {
	before, declared := readGuard.Panics(taskRead)
	if !declared {
		t.Fatalf("task %q is not declared on the radius boundary", taskRead)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	previous := radiusMetricsPtr.Swap(&radiusMetrics{})
	t.Cleanup(func() { radiusMetricsPtr.Store(previous) })

	secret := []byte("testing123")
	var answered atomic.Bool
	defective := newRFC2869Server(t, func(req []byte) []byte {
		if !answered.CompareAndSwap(false, true) {
			return nil
		}
		return unsignedReply(CodeAccessAccept, req, secret, true)
	})
	healthy := newRFC2869Server(t, func(req []byte) []byte {
		return buildReplyResponse(CodeAccessAccept, req, secret, nil)
	})

	client, err := NewClient(ClientConfig{Timeout: 300 * time.Millisecond, Retries: 1})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { closeSilent(client) })
	exchange := func(server string) (*Packet, error) {
		auth, authErr := RandomAuthenticator()
		if authErr != nil {
			t.Fatalf("RandomAuthenticator: %v", authErr)
		}
		req := &Packet{
			Code:          CodeAccessRequest,
			Identifier:    client.NextID(),
			Authenticator: auth,
			Attrs:         []Attr{{Type: AttrUserName, Value: AttrString("dave")}},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return client.Exchange(ctx, req, Server{Address: server, SharedKey: secret})
	}

	if resp, exchangeErr := exchange(defective.addr); exchangeErr == nil {
		t.Fatalf("the answer that panicked was delivered as code %d", resp.Code)
	}
	if !answered.Load() {
		t.Fatal("server A never answered: the seam was not reached")
	}
	resp, err := exchange(healthy.addr)
	if err != nil {
		t.Fatalf("the exchange after the contained panic failed: %v: the reader did not carry on", err)
	}
	if resp.Code != CodeAccessAccept {
		t.Fatalf("healthy server answered code %d, want Access-Accept", resp.Code)
	}

	if count, _ := readGuard.Panics(taskRead); count != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d: the panic was not counted", taskRead, count, before+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
	want := "radius " + taskRead + ", unit answer from server " + defective.addr
	if record := crashlog.ReadCrash(crashes[0].Name); !strings.Contains(record, want) {
		t.Fatalf("crash record lacks %q:\n%s", want, record)
	}
}
