// Design: docs/architecture/testing/ci-format.md -- remote_close requires every frame before transport closure.

package peer

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

// remoteCloseCease is the exact Administrative Shutdown notification used by
// the fatal-owner live-peer draft, including its shutdown communication.
const remoteCloseCease = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D0306021741646D696E6973747261746976652053687574646F776E"

// TestRemoteCloseRunRequiresCeaseBeforeEOF drives the real listening Run path.
// The same established TCP session and EOR precede both outcomes: transport
// closure alone MUST fail, while the expected Cease followed by closure passes.
func TestRemoteCloseRunRequiresCeaseBeforeEOF(t *testing.T) {
	for _, sendCease := range []bool{false, true} {
		t.Run(strconv.FormatBool(sendCease), func(t *testing.T) {
			port := reservePort(t)
			p, err := New(&Config{
				Port: port, RemoteClose: true, Output: io.Discard,
				Expect: []string{
					"expect=bgp:conn=1:seq=1:hex=" + unmetEOR,
					"expect=bgp:conn=1:seq=2:hex=" + remoteCloseCease,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			done := make(chan Result, 1)
			joined := false
			t.Cleanup(func() {
				cancel()
				if !joined {
					<-done
				}
			})
			go runRemoteClosePeer(ctx, p, done)
			select {
			case <-p.Ready():
			case result := <-done:
				joined = true
				t.Fatalf("peer failed before readiness: %v", result.Error)
			}
			var dialer net.Dialer
			conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = conn.Close() })
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := conn.Write(minimalOpenMsg(65001)); err != nil {
				t.Fatal(err)
			}
			reader := newMessageReader(conn)
			for range 2 { // The check peer's OPEN and KEEPALIVE.
				if _, _, err := reader.readMessage(); err != nil {
					t.Fatal(err)
				}
			}
			frames := unmetEOR
			if sendCease {
				frames += remoteCloseCease
			}
			wire, err := hex.DecodeString(frames)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := conn.Write(wire); err != nil {
				t.Fatal(err)
			}
			tcp, ok := conn.(*net.TCPConn)
			if !ok {
				t.Fatalf("dial returned %T, want TCP", conn)
			}
			// End the sender's byte stream without discarding unread peer
			// KEEPALIVEs, which could reset TCP before its frames were read.
			if err := tcp.CloseWrite(); err != nil {
				t.Fatal(err)
			}
			result := <-done
			joined = true
			if sendCease {
				if !result.Success || result.Error != nil {
					t.Fatalf("Cease then close failed: %v", result.Error)
				}
				return
			}
			if result.Success || !errors.Is(result.Error, ErrConnectionClosed) {
				t.Fatalf("EOF before Cease: success=%v error=%v, want ErrConnectionClosed", result.Success, result.Error)
			}
		})
	}
}
