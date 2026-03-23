package peer

import (
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsConnReset verifies ECONNRESET detection across error wrapping patterns.
//
// VALIDATES: isConnReset correctly identifies connection reset errors.
// PREVENTS: ECONNRESET errors falling through to generic error handler in test peer.
func TestIsConnReset(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "OpError wrapping SyscallError wrapping ECONNRESET",
			err: &net.OpError{
				Op:  "read",
				Net: "tcp",
				Err: &SyscallErrorHelper{Err: syscall.ECONNRESET},
			},
			want: true,
		},
		{
			name: "bare ECONNRESET",
			err:  syscall.ECONNRESET,
			want: true,
		},
		{
			name: "fmt.Errorf wrapped ECONNRESET",
			err:  fmt.Errorf("read failed: %w", syscall.ECONNRESET),
			want: true,
		},
		{
			name: "io.EOF is not reset",
			err:  io.EOF,
			want: false,
		},
		{
			name: "different syscall error",
			err: &net.OpError{
				Op:  "read",
				Net: "tcp",
				Err: &SyscallErrorHelper{Err: syscall.EPIPE},
			},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isConnReset(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// SyscallErrorHelper wraps a syscall.Errno for testing.
// Mirrors os.SyscallError behavior (Unwrap returns the inner error).
type SyscallErrorHelper struct {
	Err error
}

func (e *SyscallErrorHelper) Error() string { return e.Err.Error() }
func (e *SyscallErrorHelper) Unwrap() error { return e.Err }
