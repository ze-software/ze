//go:build linux

package fixture

import (
	"errors"
	"syscall"
	"testing"

	"github.com/creack/pty"
)

// TestReadPTYUntilContextKeepsPTYCloseContract uses the Linux PTY close result,
// which is EIO instead of a pipe's EOF. An expected-status wait must refuse it;
// a terminal drain must keep treating it as the end of the stream.
func TestReadPTYUntilContextKeepsPTYCloseContract(t *testing.T) {
	for _, eofOK := range []bool{false, true} {
		name := "status"
		if eofOK {
			name = "drain"
		}
		t.Run(name, func(t *testing.T) {
			terminal, slave, err := pty.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer closeSessionWaitEnd(t, terminal)
			if err := slave.Close(); err != nil {
				t.Fatal(err)
			}
			got, err := readPTYUntilContext04(t.Context(), terminal, []byte("partial"), eofOK, "absent")
			if got != "partial" {
				t.Fatalf("output=%q", got)
			}
			if eofOK {
				if err != nil {
					t.Fatalf("drain=%v", err)
				}
			} else {
				if !errors.Is(err, syscall.EIO) {
					t.Fatalf("absent status=%v, want PTY EIO", err)
				}
			}
		})
	}
}
