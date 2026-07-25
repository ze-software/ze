//go:build !linux

// Design: plan/learned/929-isis-3-l2-transport.md -- non-Linux backend stub
//
// IS-IS raw L2 transport uses AF_PACKET/SOCK_RAW, which is Linux-specific (the
// gokrazy appliance target is Linux). On other platforms NewBackend returns a
// backend whose OpenCircuit fails cleanly, so the component still loads for
// config parsing and unit tests without a privileged socket. The frame codec,
// multicast selection, orchestrator, and MTU logic are platform-neutral and
// fully exercised by unit tests on any OS.

package transport

import (
	"errors"
	"runtime"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// errBackendUnsupported is returned by the non-Linux stub backend.
var errBackendUnsupported = func() error {
	var tb textbuf.Buffer
	return errors.New(tb.Str("isis/transport: raw L2 transport unsupported on ").Str(runtime.GOOS).String())
}()

type stubBackend struct{}

// NewBackend returns the non-Linux stub backend. Its OpenCircuit always fails.
func NewBackend() Backend { return stubBackend{} }

func (stubBackend) OpenCircuit(string) (CircuitHandle, error) {
	return nil, errBackendUnsupported
}
