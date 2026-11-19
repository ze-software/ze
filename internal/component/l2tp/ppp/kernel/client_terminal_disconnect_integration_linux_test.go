//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- terminal exact disconnect
// Related: client_interleave_integration_linux_test.go -- actual original inputs

package kernel

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestClientTerminalNativeDisconnectRunsBeforeFileClose wraps only the actual
// CPE terminal callback around the actual exact-pair ioctl. Its useful native
// input is attached first. The legacy observation then proves that attachment
// ended while BOTH original files are still usable, before physical channel
// close. It fabricates neither a native receipt nor a successful disconnect.
func TestClientTerminalNativeDisconnectRunsBeforeFileClose(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	client, unit, channel, _, _ := clientNativeInterleaveInputs(t, "zfinal-origin", 88)
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := clientNativeAttachmentWitness(channel); err != nil {
		t.Fatalf("useful attached input missing: %v", err)
	}
	if err := connectOriginalSessionPair(client.pair); err != nil {
		t.Fatalf("reconnect exact original for cleanup: %v", err)
	}
	called := 0
	actual := client.ops.disconnect
	client.ops.disconnect = func(pair *claimedSessionPair) error {
		called++
		if pair != client.pair {
			t.Fatal("terminal callback changed original pair")
		}
		if err := actual(pair); err != nil {
			return err
		}
		if pair.channelClosed {
			t.Fatal("disconnect followed physical channel close")
		}
		if pair.unitClosed {
			t.Fatal("disconnect followed unit close")
		}
		if err := clientNativeAttachmentWitness(channel); !errors.Is(err, unix.EINVAL) {
			t.Fatalf("exact native disconnect left attachment: %v", err)
		}
		if err := unit.WithDescriptor(func(int) error { return nil }); err != nil {
			t.Fatalf("disconnect closed original unit before final sample: %v", err)
		}
		return nil
	}
	for range 2 {
		if err := client.CloseChannel(); err != nil {
			t.Fatal(err)
		}
	}
	if called != 1 {
		t.Fatalf("actual terminal disconnect calls = %d", called)
	}
	if err := channel.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("terminal channel stayed open: %v", err)
	}
	if _, err := unit.Counters(); err != nil {
		t.Fatalf("post-join original snapshot unavailable: %v", err)
	}
	if err := client.CloseUnit(); err != nil {
		t.Fatal(err)
	}
	if _, err := unit.Counters(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("terminal unit retained: %v", err)
	}
}
