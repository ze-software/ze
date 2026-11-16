// Design: docs/guide/l2tp.md -- the pool requires the daemon's handler registry

package l2tppool

import (
	"bytes"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPoolExternalRefusesBeforeDeclaration proves refusal precedes SDK startup.
// The transport stays open, but records and refuses a declaration write. Old
// code also returns 1 on that transport error, so exit status alone is no proof.
func TestPoolExternalRefusesBeforeDeclaration(t *testing.T) {
	engine, child := net.Pipe()
	defer func() { _ = engine.Close() }() //nolint:errcheck // test transport cleanup
	conn := &poolExternalWriteProbe{Conn: child}
	previous := logger()
	var output bytes.Buffer
	setLogger(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { setLogger(previous) })

	if result := runPlugin(conn); result != 1 {
		t.Fatalf("external pool exit=%d, want refusal 1", result)
	}
	if conn.writes != 0 {
		t.Fatalf("external pool attempted %d SDK writes before refusing, want 0", conn.writes)
	}
	for _, want := range []string{
		"l2tp-pool cannot run as an external plugin",
		"daemon's address and prefix handlers",
		diagnostic.CodeDoctorPluginExternalUnsupported,
		"ze config edit",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("external refusal log %q does not contain %q", output.String(), want)
		}
	}
}

// TestPoolInternalReachesDeclaration proves the mode guard permits the daemon's
// bridged transport. Its first declaration write deliberately fails, so this
// test checks admission only; the daemon tests prove usable allocation/reload.
func TestPoolInternalReachesDeclaration(t *testing.T) {
	engine, child := net.Pipe()
	defer func() { _ = engine.Close() }() //nolint:errcheck // test transport cleanup
	conn := &poolExternalWriteProbe{Conn: child}
	previous := logger()
	var output bytes.Buffer
	setLogger(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { setLogger(previous) })

	if result := runPlugin(rpc.NewBridgedConn(conn, rpc.NewDirectBridge())); result != 1 {
		t.Fatalf("internal pool exit=%d after declaration write failure, want 1", result)
	}
	if conn.writes != 1 {
		t.Fatalf("internal pool attempted %d SDK writes, want its first declaration write", conn.writes)
	}
	if strings.Contains(output.String(), "l2tp-pool cannot run as an external plugin") {
		t.Fatalf("internal pool received the external refusal: %s", output.String())
	}
}

// poolExternalWriteProbe embeds a real open connection, including its read,
// close and deadline behavior. Only writes are intercepted; no early EOF or
// closed net.Pipe can turn missing admission into a false passing exit=1.
type poolExternalWriteProbe struct {
	net.Conn
	writes int
}

func (conn *poolExternalWriteProbe) Write([]byte) (int, error) {
	conn.writes++
	return 0, io.ErrClosedPipe
}
