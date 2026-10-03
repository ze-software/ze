// Design: docs/architecture/mrt.md — observation independent of semantic delivery.
package reactor

import (
	"encoding/binary"
	"io"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// observeReceivedWire borrows a complete original message synchronously. Observers MUST
// copy bytes they retain and MUST NOT reenter Session writes. Semantic rejection
// and synthetic withdrawals cannot affect this boundary.
func (s *Session) observeReceivedWire(wire []byte) {
	if s.onWireMessage == nil {
		return
	}
	s.mu.RLock()
	ctxID := s.recvCtxID
	endpoints := s.transport.Load()
	s.mu.RUnlock()
	s.onWireMessage(wire, ctxID, false, endpoints)
}

// observedBGPWriter belongs to one connection epoch and is called only under
// Session.writeMu. The usual whole-frame path borrows Write's bytes; a frame
// crossing writes uses at most the BGP maximum (65535 octets) of lazy storage.
// No staged-but-unflushed UPDATE is reported as sent.
type observedBGPWriter struct {
	writer io.Writer
	session *Session
	partial []byte
	context atomic.Uint32
	transport *sessionTransport
	invalid bool
}

func (w *observedBGPWriter) Write(data []byte) (int, error) {
	n, err := w.writer.Write(data)
	if n > 0 && w.session.onWireMessage != nil && !w.invalid {
		w.observe(data[:n])
	}
	return n, err
}

// observe frames accepted transport bytes, including n > 0 with an error.
// RFC 6396 Section 4.4.2: "Only one BGP message SHALL be encoded in the
// BGP4MP_MESSAGE Subtype."
func (w *observedBGPWriter) observe(data []byte) {
	for len(data) > 0 {
		if len(w.partial) != 0 {
			if len(w.partial) < 19 {
				take := min(19-len(w.partial), len(data))
				w.partial = append(w.partial, data[:take]...)
				data = data[take:]
				if len(w.partial) < 19 {
					return
				}
			}
			length := int(binary.BigEndian.Uint16(w.partial[16:18]))
			if length < 19 {
				w.invalid = true
				sessionLogger().Error("cannot observe outbound BGP: invalid frame length", "length", length)
				return
			}
			take := min(length-len(w.partial), len(data))
			w.partial = append(w.partial, data[:take]...)
			data = data[take:]
			if len(w.partial) == length {
				w.session.onWireMessage(w.partial, bgpctx.ContextID(w.context.Load()), true, w.transport)
				w.partial = w.partial[:0]
			}
			continue
		}
		if len(data) >= 19 {
			length := int(binary.BigEndian.Uint16(data[16:18]))
			if length < 19 {
				w.invalid = true
				sessionLogger().Error("cannot observe outbound BGP: invalid frame length", "length", length)
				return
			}
			if length <= len(data) {
				w.session.onWireMessage(data[:length], bgpctx.ContextID(w.context.Load()), true, w.transport)
				data = data[length:]
				continue
			}
		}
		if w.partial == nil {
			w.partial = make([]byte, 0, 65535)
		}
		w.partial = append(w.partial, data...)
		return
	}
}

// dispatchObservedWire uses immutable epoch metadata, not the Peer occupying a
// current lookup slot. It must not acquire Peer or Session locks: teardown can
// flush a transport while holding Session.mu.
func (r *Reactor) dispatchObservedWire(peer plugin.PeerInfo, wire []byte, ctxID bgpctx.ContextID, sent bool, endpoints *sessionTransport) {
	peer.MessageContextID = ctxID
	if ctx := bgpctx.Registry.Get(ctxID); ctx != nil {
		peer.PeerAS = ctx.PeerASN()
	}
	if endpoints != nil {
		peer.LocalAS = endpoints.localAS.Load()
		peer.LocalAddress = endpoints.local
		peer.LocalAddressStr = endpoints.localString
		peer.LocalPort, peer.RemotePort = endpoints.localPort, endpoints.remotePort
	}
	r.notifyWireMessage(&peer, wire, sent)
}

// notifyWireMessage is the only observer dispatch, independent of route delivery.
// Callers MUST provide one original complete message; observers MUST NOT retain
// its bytes or reenter session writes during this synchronous callback.
func (r *Reactor) notifyWireMessage(peer *plugin.PeerInfo, wire []byte, sent bool) {
	if len(wire) < 19 {
		return
	}
	msgType := msgtype.MessageType(wire[18])
	body := wire[19:]
	if r.capture != nil {
		var code, subcode uint8
		if msgType == msgtype.TypeNOTIFICATION && len(body) >= 2 {
			code, subcode = body[0], body[1]
		}
		r.capture.Append(sent, peer.Address, msgType, len(body), code, subcode)
	}
	if capture := r.rawCapture.Load(); capture != nil {
		var direction uint8
		if sent {
			direction = 1
		}
		capture.Append(direction, peer.Address, peer.LocalAddress, msgType, body)
	}
	r.observersMu.RLock()
	observers := r.msgObservers
	r.observersMu.RUnlock()
	for _, observer := range observers {
		observer.OnBGPMessage(peer, msgType, sent, wire)
	}
}
