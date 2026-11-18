// Design: docs/architecture/l2tp/bng-5-pppoe.md -- independent PPPoX lifetime
// Related: factory.go -- owner transfer into the admitted session

package kernel

import (
	"errors"
	"io"
	"os"
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// pppoxTransport owns one connected PPPoX descriptor. Its zero value is invalid;
// only openTransport creates a production owner. Close and RelayDescriptor are
// safe for concurrent use. A borrowed relay descriptor MUST NOT outlive Close.
type pppoxTransport struct {
	mu   sync.Mutex
	file *os.File
	// fd and closeFD are private logical-fixture ports. Production always
	// retains file, and never operates on or closes through this label.
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
	if t.file != nil {
		t.closeErr = t.file.Close()
		return t.closeErr
	}
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
	if t.file != nil {
		return int(t.file.Fd()), nil
	}
	return t.fd, nil
}

var _ dataplane.RelayTransport = (*pppoxTransport)(nil)

// WithOriginalFile excludes this actual transport's physical close while a
// bounded native capture uses the original file. It MUST NOT wait on a backend.
func (t *pppoxTransport) WithOriginalFile(use func(*os.File) error) error {
	if use == nil {
		return errors.New("pppoe: original transport borrow has no operation")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return io.ErrClosedPipe
	}
	if t.file == nil {
		return errors.New("pppoe: transport has no original file for native relay")
	}
	return use(t.file)
}

var _ dataplane.OriginalRelayTransport = (*pppoxTransport)(nil)
