// Design: plan/spec-ipsec-esp-dual-form-receive.md -- one Child SA receives both ESP forms
// RFC: rfc/short/rfc7296.md -- receive both ESP forms at any time (Section 2.23)
// Overview: espform.go -- the datagram builder, the watched-SPI registry and the rate bound

//go:build linux

package dataplane

import (
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// errNoESPFormReceiver reports that this backend was built without the receiver that
// serves the second ESP form, so it cannot honor SAParams.AcceptBothESPForms.
var errNoESPFormReceiver = errors.New("dataplane: no ESP form receiver, so this SA would receive one ESP wire form only")

// espFormReadBufLen bounds one inbound ESP read. An ESP datagram cannot exceed the IPv4
// total length, and each buffer is allocated once per reader rather than per datagram
// (ai/rules/memory-architecture.md).
const espFormReadBufLen = 0xFFFF

// espFormReceiver re-presents inbound ESP that arrived in the form its XFRM state does
// not accept, so ONE Child SA receives both wire forms.
//
// RFC 7296 Section 2.23 requires a device that supports NAT-T to receive both
// UDP-encapsulated and bare ESP "at any time". Linux XFRM binds one inbound state to one
// form: net/xfrm/xfrm_input.c compares the datagram's encapsulation type against the
// state's template and raises XfrmInStateMismatch when they disagree. A second state on
// the same SPI is refused outright ("file exists"), and a live state's template cannot be
// changed (net/xfrm/xfrm_state.c returns -EINVAL), so the kernel alone cannot serve both
// forms for one SA.
//
// This receiver covers the gap for the states that carry a template. The kernel keeps
// serving the encapsulated form on its fast path. A bare ESP datagram for the same SPI is
// refused by XFRM, and Linux delivers a copy to a raw IPPROTO_ESP socket first
// (ip_protocol_deliver_rcu calls raw_local_deliver before the protocol handler), so this
// receiver reads it and sends it back through the port-4500 socket in the encapsulated
// form the template demands. That socket carries UDP_ENCAP_ESPINUDP, so the kernel strips
// the header again and hands XFRM the encapsulation type the template wants.
//
// MEASURED against a real kernel by TestEncapOneStateAcceptsBothForms and
// TestEncapBareESPVisibleToUserspaceWhenStateIsTemplated
// (encap_hybrid_integration_linux_test.go).
//
// Only SPIs whose inbound state carries a template are watched, which is what keeps the
// bare fast path of a template-free SA off this path entirely. A bare datagram for a
// watched SPI is one XFRM has already refused, so re-presenting it can never duplicate a
// packet the kernel accepted.
//
// The cryptography stays in XFRM. This receiver changes only how a datagram is PRESENTED,
// and never decrypts, so the kernel keeps owning the replay window and the integrity
// check.
//
// Watch, Forget and Close are safe for concurrent use.
type espFormReceiver struct {
	log *slog.Logger
	reg espFormRegistry

	mu       sync.Mutex
	conn     net.PacketConn
	injectFD int
	stop     chan struct{}
	wg       sync.WaitGroup
}

func newESPFormReceiver(log *slog.Logger) *espFormReceiver {
	return &espFormReceiver{log: log, injectFD: -1}
}

// Watch re-presents bare ESP for this SPI until Forget. peer is the inbound state's
// source address and local its destination, which is the pair the state was installed
// with.
//
// It reports an error when the sockets cannot be opened. The caller MUST surface that,
// because a receiver that is not running leaves one of the two ESP forms unserved and the
// tunnel then carries traffic in one form only (ai/rules/fail-closed-guards.md).
// A nil receiver FAILS CLOSED here, and that is the whole point of returning an error.
// xfrmBackend can be built as a bare literal, which leaves espForms nil. An SA that asked
// to receive both ESP forms and got a silent nil would install a state serving ONE form
// and drop the other, which is the quietest failure this subsystem has
// (ai/rules/fail-closed-guards.md). Refusing the install surfaces it instead.
func (r *espFormReceiver) Watch(spi uint32, peer, local netip.Addr) error {
	if r == nil {
		return errNoESPFormReceiver
	}
	first := r.reg.watch(spi, peer, local)
	if !first {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn != nil {
		return nil
	}
	if err := r.startLocked(); err != nil {
		r.reg.forget(spi)
		return err
	}
	return nil
}

// Forget stops re-presenting bare ESP for this SPI, and releases the sockets once no SPI
// is watched. A deployment whose SAs all run without a template therefore holds no raw
// socket at all, and the kernel clones no bare ESP packet for it.
// A nil receiver holds nothing, so Forget is a no-op rather than a panic. Watch is where
// the absence is reported; a teardown path must never be the thing that crashes.
func (r *espFormReceiver) Forget(spi uint32) {
	if r == nil {
		return
	}
	if last := r.reg.forget(spi); !last {
		return
	}
	r.mu.Lock()
	r.stopLocked()
	r.mu.Unlock()
	r.wg.Wait()
}

// Close releases the sockets and waits for the reader to finish. Callers MUST call it
// when the backend shuts down.
// A nil receiver holds no socket and no goroutine, so Close succeeds trivially.
func (r *espFormReceiver) Close() error {
	if r == nil {
		return nil
	}
	r.reg.forgetAll()
	r.mu.Lock()
	r.stopLocked()
	r.mu.Unlock()
	r.wg.Wait()
	return nil
}

// logger reports the receiver's logger, and a usable one when the receiver is nil so a
// caller on an error path never has to guard the call.
func (r *espFormReceiver) logger() *slog.Logger {
	if r == nil || r.log == nil {
		return slogutil.Logger("ike.dataplane")
	}
	return r.log
}

// running reports whether the sockets are open. A nil receiver never has any.
func (r *espFormReceiver) running() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.conn != nil
}

// startLocked opens both sockets and starts the reader. r.mu must be held.
func (r *espFormReceiver) startLocked() error {
	conn, err := net.ListenPacket("ip4:esp", "0.0.0.0")
	if err != nil {
		return err
	}
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_RAW)
	if err != nil {
		if cerr := conn.Close(); cerr != nil {
			r.log.Debug("esp-form: close reader after inject socket failure", "error", cerr)
		}
		return err
	}

	r.conn = conn
	r.injectFD = fd
	r.stop = make(chan struct{})

	r.wg.Add(1)
	go r.run(conn, fd, r.stop)
	return nil
}

// stopLocked closes the sockets and signals the reader. r.mu must be held.
func (r *espFormReceiver) stopLocked() {
	if r.conn == nil {
		return
	}
	close(r.stop)
	if err := r.conn.Close(); err != nil {
		r.log.Debug("esp-form: close reader socket", "error", err)
	}
	if err := unix.Close(r.injectFD); err != nil {
		r.log.Debug("esp-form: close inject socket", "error", err)
	}
	r.conn = nil
	r.injectFD = -1
}

// espFormStopped reports whether the reader has been asked to finish.
func espFormStopped(stop <-chan struct{}) bool {
	select {
	case <-stop:
		return true
	default:
		return false
	}
}

// run reads bare ESP and re-presents every datagram whose SPI is watched. It is one
// long-lived worker for the life of the sockets (ai/rules/goroutine-lifecycle.md), and
// both buffers are allocated once here rather than per datagram.
func (r *espFormReceiver) run(conn net.PacketConn, fd int, stop <-chan struct{}) {
	defer r.wg.Done()

	read := make([]byte, espFormReadBufLen)
	out := make([]byte, espFormReadBufLen)
	limiter := newESPFormLimiter(time.Now())

	for {
		n, _, err := conn.ReadFrom(read)
		if err != nil {
			if !espFormStopped(stop) {
				r.log.Warn("esp-form: stopped reading refused ESP, one ESP form is no longer received",
					"error", err)
			}
			return
		}

		spi, ok := espFormSPI(read[:n])
		if !ok {
			continue
		}
		target, watched := r.reg.target(spi)
		if !watched {
			// A template-free SA's own traffic, which XFRM has already accepted.
			continue
		}
		if !limiter.allow(time.Now()) {
			continue
		}

		wrote := writeESPForm(out, target.peer, target.local, read[:n])
		if wrote == 0 {
			continue
		}

		var addr unix.SockaddrInet4
		addr.Addr = target.local.As4()
		if err := unix.Sendto(fd, out[:wrote], 0, &addr); err != nil {
			r.log.Debug("esp-form: re-present refused ESP", "spi", spi, "error", err)
		}
	}
}
