// Design: docs/architecture/l2tp/bng-5-pppoe.md -- independent PPPoX lifetime
// Related: factory.go -- owner transfer into the admitted session

package kernel

import (
	"io"
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// pppoxTransport owns one connected PPPoX descriptor. Its zero value is invalid;
// only openTransport creates a production owner. Close and RelayDescriptor are
// safe for concurrent use. A borrowed relay descriptor MUST NOT outlive Close.
type pppoxTransport struct {
	mu       sync.Mutex
	fd       int
	closeFD  func(int) error
	closed   bool
	closeErr error
}

func (t *pppoxTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return t.closeErr
	}
	t.closed = true
	t.closeErr = t.closeFD(t.fd)
	return t.closeErr
}

// RelayDescriptor lends the still-owned transport to the native realm relay.
// Its caller MUST end the borrow before CloseTransport ends transport ownership.
func (t *pppoxTransport) RelayDescriptor() (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return -1, io.ErrClosedPipe
	}
	return t.fd, nil
}

var _ dataplane.RelayTransport = (*pppoxTransport)(nil)
