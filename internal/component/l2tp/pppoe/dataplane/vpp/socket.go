// Design: docs/architecture/l2tp/bng-5-pppoe.md -- one socket per VPP PPP session
// Related: socket_linux.go -- native polled datagram endpoint; session.go -- owner
package vpp

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

type controlEndpoint interface {
	io.ReadWriteCloser
	SetReadDeadline(time.Time) error
}

// controlSocket owns its endpoint; borrowed reads and writes may run together.
// Close's endpoint implementation MUST deregister polling before closing its
// native descriptor, and MUST unblock all I/O. A session uses the same Close
// through both ownership roles, never a second wrapper of the descriptor.
type controlSocket struct {
	file         controlEndpoint
	receiveMRU   atomic.Int32
	once         sync.Once
	closeErr     error
	carrierMu    sync.RWMutex
	carrier      *ipv6Carrier
	closed       bool
	ipv6Discards atomic.Uint64
}

func newControlSocket(file controlEndpoint) *controlSocket {
	socket := &controlSocket{file: file}
	socket.receiveMRU.Store(ppp.MaxFrameLen)
	return socket
}

func (s *controlSocket) Read(buf []byte) (int, error) {
	limit := int(s.receiveMRU.Load()) + ppp.ProtoFieldLen
	if len(buf) > limit {
		buf = buf[:limit]
	}
	// This is PPP's one lifetime reader. Network controls are consumed here;
	// only negotiation packets return to the common PPP state machine.
	for {
		n, err := s.file.Read(buf)
		if n > len(buf) {
			return 0, errors.New("pppoe-vpp: control datagram exceeds the receive window")
		}
		if n >= ppp.ProtoFieldLen {
			if binary.BigEndian.Uint16(buf[:2]) == ppp.ProtoIPv6 {
				// RFC 8200 Section 3; RFC 4861 Section 6.1.1.
				s.dispatchIPv6(buf[2:n])
				if err != nil {
					return 0, err
				}
				continue
			}
		}
		return n, err
	}
}

func (s *controlSocket) Write(buf []byte) (int, error) {
	if len(buf) < ppp.ProtoFieldLen {
		return 0, errors.New("pppoe-vpp: control frame lacks a PPP protocol")
	}
	if len(buf) > ppp.MaxFrameBufLen {
		return 0, errors.New("pppoe-vpp: control frame exceeds the PPP payload bound")
	}
	n, err := s.file.Write(buf)
	if err == nil {
		if n != len(buf) {
			err = io.ErrShortWrite
		}
	}
	return n, err
}

func (s *controlSocket) InterruptRead() error { return s.file.SetReadDeadline(time.Now()) }
func (s *controlSocket) Close() error {
	s.once.Do(func() {
		s.carrierMu.Lock()
		s.closed = true
		carrier := s.carrier
		s.carrierMu.Unlock()
		// Interrupt an in-flight write before joining carrier serialization;
		// otherwise a full datagram send buffer could hold Close forever.
		s.closeErr = s.file.Close()
		if carrier != nil {
			s.closeErr = errors.Join(s.closeErr, carrier.Close())
		}
	})
	return s.closeErr
}
