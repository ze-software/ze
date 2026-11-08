// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- connected restart ownership.
// Related: phases.go -- shared negotiation; dialer.go -- stable descriptor ownership.
// RFC 1661 Sections 3.4-3.6 -- see rfc/short/rfc1661.md.

package pppoeclient

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
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
	// seq is the Seq of the last result publish sent; applied carries the
	// Seq the interface consumer last reported applied (acknowledge), at most
	// one pending value, newest wins.
	seq     uint64
	applied chan uint64
	// ipv6 is nil when the pppoe-client configures no ipv6: that nil is a
	// guard, keepaliveLoop then drops IPv6CP like any unconfigured protocol.
	// enableIPv6CP sets it with setIPv6Mode.
	ipv6        *clientIPv6CP
	setIPv6Mode func(ppp.NPMode) error
	// linkLocal is the local link-local address while IPv6CP is Opened, the
	// invalid address otherwise; publish carries it in every result.
	linkLocal netip.Addr
	// ipcpStopped is true while IPCP is out of Opened on the connected link
	// and LCP stays up (handleIPCP): IPv4 is discarded and publish withdraws
	// it. False, the zero value, is the Opened state newConnectedSession
	// admits and restart reaches.
	ipcpStopped bool
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
		applied: make(chan uint64, 1),
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
		primaryDNS: initial.primaryDNS, secondaryDNS: initial.secondaryDNS,
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
// No partial result is published: current changes only after IPCP opens, and
// the result is published then. IPv4, and IPv6 when IPv6CP reached Opened,
// pass only once the interface consumer has applied that result
// (awaitApplied), as pppd's lcp_up sets the MTU before link_established.
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
		s.current.magic, pending, &s.ipcpIdentifier, stop, logger, &s.lcpIdentifier, &s.papIdentifier, &s.ipcpCodeRejectID, s.ipv6)
	if err != nil {
		return err
	}
	s.current = next
	s.ipcpStopped = false
	// IPv6CP ran beside IPCP inside negotiatePhases with IPv6 discarded
	// (keepaliveLoop applied lowerDown before this restart). It passes only
	// after the result below is applied, if it reached Opened; otherwise
	// keepaliveLoop goes on negotiating.
	ipv6Opened := false
	if s.ipv6 != nil {
		ipv6Opened = s.ipv6.opened()
	}
	if ipv6Opened {
		s.linkLocal = s.ipv6.linkLocal()
	}
	if err := s.awaitApplied(s.publish(), stop); err != nil {
		return err
	}
	// RFC 1661 Section 3.6: "After a NCP has reached the Opened state, PPP will
	// carry the corresponding network-layer protocol packets."
	if err := s.setIPv4Mode(ppp.NPModePass); err != nil {
		return fmt.Errorf("pppoe-client: cannot resume IPv4 after LCP restart: %w", err)
	}
	if ipv6Opened {
		// RFC 5072 Section 2: "Before any IPv6 packets may be communicated,
		// PPP MUST reach the network-layer protocol phase, and the IPv6
		// Control Protocol MUST reach the Opened state."
		if err := s.setIPv6Mode(ppp.NPModePass); err != nil {
			return fmt.Errorf("pppoe-client: cannot pass IPv6 after IPv6CP opened: %w", err)
		}
		recordIPv6CPOpened(s.cfg.name, true)
		logger.Info("pppoe-client: IPv6CP opened", "link-local", s.linkLocal)
	}
	return nil
}

// awaitApplied waits until the interface consumer reports the result numbered
// seq, or a later one, applied (iface.PPPoESession.Applied, acknowledge). It
// ends with an error when stop closes, which is how a consumer that stopped
// or failed an apply releases the worker: its Cleanup closes the session.
func (s *connectedSession) awaitApplied(seq uint64, stop <-chan struct{}) error {
	for {
		select {
		case <-stop:
			return errors.New("pppoe-client: session stopped before the interface applied the renegotiated result")
		case got := <-s.applied:
			if got >= seq {
				return nil
			}
		}
	}
}

// acknowledge records that the interface consumer applied the result numbered
// seq. It is iface.PPPoESession.Applied, called from the consumer's goroutine,
// the only sender: the session worker only drains applied, so after one drain
// the send succeeds, and the loop runs at most twice. A newer value replaces
// an unread older one, since the worker needs only the highest.
func (s *connectedSession) acknowledge(seq uint64) {
	for {
		select {
		case s.applied <- seq:
			return
		default:
		}
		select {
		case <-s.applied:
		default:
		}
	}
}

// publish replaces the one pending result with the session's complete current
// state: the IPCP values, invalid while IPCP is Stopped, and the IPv6CP
// link-local address. It answers the Seq it gave the result.
func (s *connectedSession) publish() uint64 {
	// The worker is the only sender. A consumer can only create more room,
	// so replacing the one pending value never waits for interface operations.
	select {
	case <-s.changes:
	default: // No previous completion is pending.
	}
	s.seq++
	result := iface.PPPoEResult{NegMTU: s.current.negMTU, LinkLocal: s.linkLocal, Seq: s.seq}
	if !s.ipcpStopped {
		result.LocalIP, result.PeerIP = s.current.localIP, s.current.peerIP
		result.PrimaryDNS, result.SecondaryDNS = s.current.primaryDNS, s.current.secondaryDNS
	}
	s.changes <- result
	return s.seq
}
