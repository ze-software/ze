// Design: docs/architecture/testing/ci-format.md -- remote_close peer option
// Overview: reject.go -- completed calls this before announcing success

package peer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

func parsePeerOptionRemoteClose(config *Config, kv map[string]string) error {
	value, err := strconv.ParseBool(kv[keyValue])
	if err != nil {
		return fmt.Errorf("remote_close requires a boolean value: %w", err)
	}
	config.RemoteClose = value
	return nil
}

// waitRemoteClose proves the remote transport ended after the final frame.
// The peer's context bounds the loop; teardown cancellation is a failure, never
// evidence of EOF. No writes, local close, or early success can supply the proof.
func (p *Peer) waitRemoteClose(ctx context.Context, conn net.Conn) Result {
	var next [1]byte
	for {
		if err := ctx.Err(); err != nil {
			return Result{Error: fmt.Errorf("waiting for remote close: %w", err)}
		}
		if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			return Result{Error: fmt.Errorf("remote close deadline: %w", err)}
		}
		n, err := conn.Read(next[:])
		if n > 0 {
			return Result{Error: errors.New("received data after the final expectation while waiting for remote close")}
		}
		if errors.Is(err, io.EOF) {
			p.remoteClosed.Store(true)
			p.printf("the remote closed the transport after the final expectation\n")
			return Result{Success: true}
		}
		if isConnReset(err) {
			p.remoteClosed.Store(true)
			p.printf("the remote reset the transport after the final expectation\n")
			return Result{Success: true}
		}
		if isTimeout(err) {
			continue
		}
		if err != nil {
			return Result{Error: fmt.Errorf("waiting for remote close: %w", err)}
		}
	}
}
