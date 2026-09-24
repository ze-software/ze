// Design: docs/architecture/testing/ci-format.md -- cmd=background ready= barrier
// Related: internal/test/runner/background_ready.go -- awaitBackgroundReady, the barrier these drive

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

func init() {
	Register("runner/background-ready-server", backgroundReadyServer)
	Register("runner/background-ready-dial", backgroundReadyDial)
}

// backgroundReadyText is the line the server prints once it listens, and the
// ready= value test/runner/background-ready.ci waits for.
const backgroundReadyText = "BACKGROUND-READY: listening"

// backgroundReadyBindDelay is how long the server waits before it binds. It is
// the condition under test, not a pause for something else to settle: it makes
// the server slower to listen than the 100ms the runner otherwise grants a
// background helper, which is the race a slow fixture start produces on a loaded
// host. Without the ready= barrier the dial step starts first and is refused.
const backgroundReadyBindDelay = time.Second

// backgroundReadyServer binds 127.0.0.1:<port> after backgroundReadyBindDelay,
// announces it on stderr, and serves until the test ends.
func backgroundReadyServer(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: background-ready-server <port>")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(backgroundReadyBindDelay):
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", net.JoinHostPort("127.0.0.1", args[0]))
	if err != nil {
		return err
	}
	defer listener.Close() //nolint:errcheck // fixture teardown
	fmt.Fprintln(os.Stderr, backgroundReadyText)
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // fixture teardown
	<-ctx.Done()
	return nil
}

// backgroundReadyDial dials 127.0.0.1:<port> ONCE. A refusal means the runner
// started this step before the background server said it was ready, so it is
// an error, never a retry: a retry would hide exactly the race under test.
func backgroundReadyDial(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: background-ready-dial <port>")
	}
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", args[0]))
	if err != nil {
		return fmt.Errorf("the background server was not listening when this step started: %w", err)
	}
	conn.Close() //nolint:errcheck,gosec // the connection proved the listener; its close changes no assertion
	fmt.Println("ready-barrier-held")
	return nil
}
