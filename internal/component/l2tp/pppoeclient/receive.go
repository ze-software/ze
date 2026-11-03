// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- ordered receive and dispatch.
// Related: receive_linux.go -- descriptor ordering; session.go -- phase consumers.

package pppoeclient

import (
	"errors"
	"io"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// frameReader owns one worker until stop or a terminal read. The phase consumer
// MUST call received after each delivery and MUST be its sole caller. next does
// not authorize another read while a previous request is outstanding. Cleanup
// MUST stop the transport, join the phase consumer, then join done. The zero
// value is invalid. Only the phase consumer accesses pending.
type frameReader struct {
	requests chan struct{}
	frames   chan readFrame
	done     chan struct{}
	pending  bool
}

// newFrameReader starts one worker. The caller MUST supply a read operation that
// transport shutdown interrupts, close stop on shutdown, and join done afterward.
func newFrameReader(read func([]byte) (int, error), stop <-chan struct{}) (*frameReader, error) {
	if read == nil {
		return nil, errors.New("pppoe-client: missing packet read operation")
	}
	if stop == nil {
		return nil, errors.New("pppoe-client: missing receive stop signal")
	}
	r := &frameReader{
		requests: make(chan struct{}, 1), frames: make(chan readFrame), done: make(chan struct{}),
	}
	go r.run(read, stop)
	return r, nil
}

// validate rejects nil and the uninitialized zero value at consumer admission.
func (r *frameReader) validate() error {
	if r == nil {
		return errors.New("pppoe-client: missing receive owner")
	}
	if r.requests == nil {
		return errors.New("pppoe-client: uninitialized receive owner")
	}
	return nil
}

// next authorizes one read, after the preceding packet has been dispatched.
// The consumer MUST call received for the result before calling next again.
func (r *frameReader) next() <-chan readFrame {
	if !r.pending {
		select {
		case r.requests <- struct{}{}:
		case <-r.done:
		}
		r.pending = true
	}
	return r.frames
}

func (r *frameReader) received() { r.pending = false }

// run is the session's only receive worker. The unbuffered result and explicit
// request prevent read-ahead across phase transitions. Cancellation releases
// both request and delivery waits; the transport MUST cancel a blocked read.
func (r *frameReader) run(read func([]byte) (int, error), stop <-chan struct{}) {
	defer close(r.done)
	defer close(r.frames)
	for {
		select {
		case <-stop:
			return
		case <-r.requests:
		}
		buf := make([]byte, ppp.MaxFrameBufLen)
		n, err := read(buf)
		if n < 0 {
			n = 0
		}
		if n > len(buf) {
			n = 0
			err = errors.New("pppoe-client: packet reader exceeded its buffer")
		}
		if n == 0 {
			if err == nil {
				err = io.EOF
			}
		}
		select {
		case r.frames <- readFrame{data: buf[:n], err: err}:
		case <-stop:
			return
		}
		if err != nil {
			return
		}
	}
}
