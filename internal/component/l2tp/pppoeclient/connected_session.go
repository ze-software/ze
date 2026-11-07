// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- connected restart ownership.
// Related: phases.go -- shared negotiation; dialer.go -- stable descriptor ownership.
// RFC 1661 Sections 3.4-3.6 -- see rfc/short/rfc1661.md.

package pppoeclient

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// connectedSession permits restart only after a complete initial negotiation.
// newConnectedSession is its sole nonzero producer. Nil and zero are invalid;
// keepaliveLoop and restart reject them before any operation. Fields stay private.
// It owns no descriptors: Dial's Cleanup retains their reservation. One worker
// owns this value and its receive owner; it is not safe for concurrent use.
type connectedSession struct {
	transport        io.ReadWriteCloser
	reader           *frameReader
	cfg              sessionConfig
	setIPv4Mode      func(ppp.NPMode) error
	current          negotiatedSession
	ipcpIdentifier   uint8
	lcpIdentifier    uint8
	papIdentifier    uint8
	ipcpCodeRejectID uint8
	changes          chan iface.PPPoEResult
}

// newConnectedSession admits the complete initial result without doing I/O.
// The caller MUST give the worker exclusive consumption of the reader and
// MUST supply a mode operation serialized against transport and unit closure.
// Initial CHAP state is copied, so no mutable alias to Dial's result is retained.
// The transport is borrowed until Cleanup, which MUST stop it before releasing
// the unit. Typed-nil transports and missing mode operations are rejected.
func newConnectedSession(transport io.ReadWriteCloser, initial sessionResult,
	cfg sessionConfig, setIPv4Mode func(ppp.NPMode) error) (*connectedSession, error) {
	if transport == nil {
		return nil, errors.New("pppoe-client: missing connected transport")
	}
	// An interface can hold a typed nil. This cold admission boundary checks all
	// nil-capable implementations without imposing reflection on packet handling.
	value := reflect.ValueOf(transport)
	// Only nil-capable kinds need an IsNil check.
	//exhaustive:ignore
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return nil, errors.New("pppoe-client: nil connected transport")
		}
	}
	if err := initial.reader.validate(); err != nil {
		return nil, err
	}
	if setIPv4Mode == nil {
		return nil, errors.New("pppoe-client: missing connected IPv4 mode control")
	}
	// A zero interval would panic the echo ticker, and zero failures would
	// end the session on its first Echo-Request. Dial validates both from
	// config; this refuses a caller that skipped it.
	if cfg.keepaliveInterval <= 0 {
		return nil, errors.New("pppoe-client: missing keepalive interval")
	}
	if cfg.keepaliveFailures < 1 {
		return nil, errors.New("pppoe-client: missing keepalive failure count")
	}
	current, err := newNegotiatedSession(initial, cfg)
	if err != nil {
		return nil, err
	}
	return &connectedSession{
		transport: transport, reader: initial.reader,
		cfg: cfg, setIPv4Mode: setIPv4Mode, ipcpIdentifier: initial.ipcpIdentifier,
		lcpIdentifier: initial.lcpIdentifier, papIdentifier: initial.papIdentifier,
		ipcpCodeRejectID: initial.ipcpCodeRejectID,
		changes:          make(chan iface.PPPoEResult, 1), current: current,
	}, nil
}

// newNegotiatedSession admits a raw result before connection or forwarding.
// It copies CHAP state so the caller cannot mutate the worker's correlation.
// A failed admission returns the invalid zero result, never a publishable value.
func newNegotiatedSession(initial sessionResult, cfg sessionConfig) (negotiatedSession, error) {
	if cfg.mtu < sessionMTUMin {
		return negotiatedSession{}, errSessionMTURange
	}
	if cfg.mtu > sessionMTUMax {
		return negotiatedSession{}, errSessionMTURange
	}
	if !initial.localIP.Is4() {
		return negotiatedSession{}, errors.New("pppoe-client: connected local address is not IPv4")
	}
	if !initial.localIP.IsGlobalUnicast() {
		return negotiatedSession{}, errors.New("pppoe-client: connected local address is not usable")
	}
	if !initial.peerIP.Is4() {
		return negotiatedSession{}, errors.New("pppoe-client: connected peer address is not IPv4")
	}
	if !initial.peerIP.IsGlobalUnicast() {
		return negotiatedSession{}, errors.New("pppoe-client: connected peer address is not usable")
	}
	if initial.negMTU < sessionMTUMin {
		return negotiatedSession{}, errSessionMTURange
	}
	if initial.negMTU > cfg.mtu {
		return negotiatedSession{}, errSessionMTURange
	}
	if initial.peerMRU != 0 {
		if initial.negMTU > initial.peerMRU {
			return negotiatedSession{}, errSessionMTURange
		}
	}
	result := negotiatedSession{
		localIP: initial.localIP, peerIP: initial.peerIP, negMTU: initial.negMTU,
		magic: initial.magic, peerMRU: initial.peerMRU,
	}
	if initial.chap != nil {
		chap := *initial.chap
		result.chap = &chap
	}
	return result, nil
}

// restart consumes an admitted request, retaining the same transport and unit.
// The caller MUST stop echo timing first and MUST end the session on error.
// No partial result is published: current changes only after IPCP and PASS.
// RFC 1661 Section 3.4: "The receipt of the LCP Configure-Request causes a return
// to the Link Establishment phase from the Network-Layer Protocol phase or
// Authentication phase."
func (s *connectedSession) restart(pending *lcpReentry, stop <-chan struct{}, logger *slog.Logger) error {
	if s == nil {
		return errors.New("pppoe-client: missing connected session")
	}
	if s.transport == nil {
		return errors.New("pppoe-client: uninitialized connected session")
	}
	if pending == nil {
		return errors.New("pppoe-client: missing LCP restart request")
	}
	if pending.request.Code != ppp.LCPConfigureRequest {
		return errors.New("pppoe-client: uninitialized LCP restart request")
	}
	countRenegotiation(s.cfg.name)
	// RFC 1661 Section 3.6: "Any supported network-layer protocol packets received
	// when the corresponding NCP is not in the Opened state MUST be silently discarded."
	if err := s.setIPv4Mode(ppp.NPModeDrop); err != nil {
		return fmt.Errorf("pppoe-client: cannot suspend IPv4 for LCP restart: %w", err)
	}
	// RFC 1661 Sections 3.4-3.6: reuse the initial phase owner and ordered reader.
	next, err := negotiatePhases(s.transport, s.reader, s.cfg,
		s.current.magic, pending, &s.ipcpIdentifier, stop, logger, &s.lcpIdentifier, &s.papIdentifier, &s.ipcpCodeRejectID)
	if err != nil {
		return err
	}
	// RFC 1661 Section 3.6: "After a NCP has reached the Opened state, PPP will
	// carry the corresponding network-layer protocol packets."
	if err := s.setIPv4Mode(ppp.NPModePass); err != nil {
		return fmt.Errorf("pppoe-client: cannot resume IPv4 after LCP restart: %w", err)
	}
	s.current = next
	// The worker is the only sender. A consumer can only create more room,
	// so replacing the one pending value never waits for interface operations.
	select {
	case <-s.changes:
	default: // No previous completion is pending.
	}
	s.changes <- iface.PPPoEResult{
		LocalIP: next.localIP, PeerIP: next.peerIP, NegMTU: next.negMTU,
	}
	return nil
}
