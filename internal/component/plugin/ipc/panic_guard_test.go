package ipc

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// guardLines counts the connect-back guard's ERROR lines in the log ring.
func guardLines(t *testing.T) int {
	t.Helper()
	entries, err := slogutil.GlobalLogRing().Snapshot(0, "ERROR", "plugin.ipc")
	require.NoError(t, err)
	lines := 0
	for _, e := range entries {
		if strings.Contains(e.Message, "an internal Ze defect ended this unit") {
			lines++
		}
	}
	return lines
}

// TestPluginAcceptorPanicContained: a panic while the acceptor authenticates
// one connect-back (PluginAcceptor.handleConn, through a per-client secret
// lookup that panics for one name) closes that connection only. The plugin
// that sent it reads EOF instead of its auth answer, and a second plugin that
// connects afterwards is authenticated and handed to its waiter. The panic is
// counted on the real guard, logged and crash-recorded. Without the boundary
// the panic ends the test binary.
func TestPluginAcceptorPanicContained(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	beforeCount, declared := acceptorPanics().guard.Panics(taskConnectBack)
	require.True(t, declared)
	beforeLines := guardLines(t)

	serverTLS, clientTLS := selfSignedTLSConfig(t)
	ln := startTestListener(t, serverTLS)
	const secret = "acceptor-secret-at-least-32-chars"
	acceptor := NewPluginAcceptor(ln, secret, nil)
	acceptor.SetSecretLookup(func(name string) (string, bool) {
		if name == "broken-plugin" {
			panic("boom: secret lookup defect")
		}
		return "", false
	})
	acceptor.Start()
	defer acceptor.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialer := &tls.Dialer{Config: clientTLS}

	broken, err := dialer.DialContext(ctx, "tcp", acceptor.Addr().String())
	require.NoError(t, err)
	defer broken.Close() //nolint:errcheck // test cleanup
	require.NoError(t, SendAuth(ctx, broken, secret, "broken-plugin"))
	require.NoError(t, broken.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, readErr := broken.Read(make([]byte, 64))
	require.Error(t, readErr, "the panicking connect-back must be closed, not answered")
	require.NotContains(t, readErr.Error(), "timeout", "the connection was left open")

	waiter := acceptor.ExpectPlugin("healthy-plugin")
	go func() {
		conn, dialErr := dialer.DialContext(ctx, "tcp", acceptor.Addr().String())
		if dialErr != nil {
			return
		}
		if authErr := SendAuth(ctx, conn, secret, "healthy-plugin"); authErr != nil {
			conn.Close() //nolint:errcheck // test cleanup
			return
		}
		if _, answerErr := conn.Read(make([]byte, 64)); answerErr != nil {
			conn.Close() //nolint:errcheck // test cleanup
		}
	}()
	conn, err := waiter.Wait(ctx)
	require.NoError(t, err, "the acceptor stopped serving after a contained panic")
	require.NoError(t, conn.Close())

	afterCount, _ := acceptorPanics().guard.Panics(taskConnectBack)
	require.Equal(t, beforeCount+1, afterCount, "the panic was not counted")
	require.Equal(t, beforeLines+1, guardLines(t), "the panic was not logged")
	crashes := crashlog.ListCrashes()
	require.Len(t, crashes, 1)
	require.Equal(t, crashlog.KindRecovered, crashes[0].Kind)
}
