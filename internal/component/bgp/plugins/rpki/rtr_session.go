// Design: docs/architecture/plugin/rib-storage-design.md — RTR session lifecycle
// Overview: rpki.go — plugin entry point managing sessions
// Related: rtr_pdu.go — PDU wire format used by session
package rpki

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// RTR session states.
const (
	sessionIdle      = "idle"
	sessionConnect   = "connect"
	sessionEstablish = "establish"
)

// RTRSession manages a single RTR connection to a cache server.
type RTRSession struct {
	address    string
	port       uint16
	preference uint8

	conn      net.Conn
	state     string
	sessionID uint16
	serial    uint32

	// Timing parameters from End of Data.
	refreshInterval time.Duration
	retryInterval   time.Duration
	expireInterval  time.Duration

	// pendingVRPs accumulates VRPs between Cache Response and End of Data.
	pendingVRPs []VRP
	pendingDels []VRP

	mu     sync.Mutex
	stopCh <-chan struct{}
	cache  *ROACache
}

// NewRTRSession creates a new RTR session for the given cache server.
func NewRTRSession(address string, port uint16, pref uint8, cache *ROACache, stopCh <-chan struct{}) *RTRSession {
	return &RTRSession{
		address:         address,
		port:            port,
		preference:      pref,
		state:           sessionIdle,
		refreshInterval: 3600 * time.Second,
		retryInterval:   600 * time.Second,
		expireInterval:  7200 * time.Second,
		cache:           cache,
		stopCh:          stopCh,
	}
}

// Run is the long-lived goroutine for this RTR session.
// It connects, queries, receives VRPs, and reconnects on failure.
func (s *RTRSession) Run() {
	for {
		select {
		case <-s.stopCh:
			s.close()
			return
		default:
		}

		err := s.connectAndSync()
		if err != nil {
			logger().Warn("rtr: session error, will retry",
				"address", s.address, "error", err)
		}
		s.close()

		// Wait before retry.
		select {
		case <-s.stopCh:
			return
		case <-time.After(s.retryInterval):
		}
	}
}

// connectAndSync establishes TCP connection and runs the RTR protocol.
func (s *RTRSession) connectAndSync() error {
	addr := net.JoinHostPort(s.address, fmt.Sprintf("%d", s.port))
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("connect %s: %w", addr, err)
	}

	s.mu.Lock()
	s.conn = conn
	s.state = sessionConnect
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.state = sessionIdle
		s.mu.Unlock()
	}()

	// Send initial query.
	buf := make([]byte, pduSerialQueryLen)
	if s.serial == 0 {
		n := writeResetQuery(buf, 0)
		if _, err := conn.Write(buf[:n]); err != nil {
			return fmt.Errorf("write reset query: %w", err)
		}
	} else {
		n := writeSerialQuery(buf, 0, s.sessionID, s.serial)
		if _, err := conn.Write(buf[:n]); err != nil {
			return fmt.Errorf("write serial query: %w", err)
		}
	}

	// Read and process PDUs until End of Data or error.
	return s.readLoop(conn)
}

// readLoop reads PDUs from the connection until End of Data or error.
func (s *RTRSession) readLoop(conn net.Conn) error {
	headerBuf := make([]byte, pduHeaderLen)

	for {
		// Set read deadline based on expire interval.
		if err := conn.SetReadDeadline(time.Now().Add(s.expireInterval)); err != nil {
			return fmt.Errorf("set deadline: %w", err)
		}

		if _, err := io.ReadFull(conn, headerBuf); err != nil {
			return fmt.Errorf("read header: %w", err)
		}

		hdr, err := parseHeader(headerBuf)
		if err != nil {
			return err
		}

		// Read remaining bytes.
		remaining := int(hdr.Length) - pduHeaderLen
		if remaining < 0 || remaining > 65536 {
			return fmt.Errorf("rtr: invalid PDU length: %d", hdr.Length)
		}

		var pduBuf []byte
		if remaining > 0 {
			pduBuf = make([]byte, int(hdr.Length))
			copy(pduBuf, headerBuf)
			if _, err := io.ReadFull(conn, pduBuf[pduHeaderLen:]); err != nil {
				return fmt.Errorf("read PDU body: %w", err)
			}
		} else {
			pduBuf = headerBuf
		}

		done, err := s.handlePDU(hdr, pduBuf)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

// handlePDU processes a single RTR PDU. Returns true when session sync is complete.
func (s *RTRSession) handlePDU(hdr RTRHeader, buf []byte) (bool, error) {
	switch hdr.Type {
	case pduCacheResp:
		s.mu.Lock()
		s.sessionID = hdr.SessionID
		s.state = sessionEstablish
		s.pendingVRPs = nil
		s.pendingDels = nil
		s.mu.Unlock()
		return false, nil

	case pduIPv4Prefix:
		vrp, announce, err := parseIPv4Prefix(buf)
		if err != nil {
			return false, err
		}
		s.mu.Lock()
		if announce {
			s.pendingVRPs = append(s.pendingVRPs, vrp)
		} else {
			s.pendingDels = append(s.pendingDels, vrp)
		}
		s.mu.Unlock()
		return false, nil

	case pduIPv6Prefix:
		vrp, announce, err := parseIPv6Prefix(buf)
		if err != nil {
			return false, err
		}
		s.mu.Lock()
		if announce {
			s.pendingVRPs = append(s.pendingVRPs, vrp)
		} else {
			s.pendingDels = append(s.pendingDels, vrp)
		}
		s.mu.Unlock()
		return false, nil

	case pduEndOfData:
		params, err := parseEndOfData(buf)
		if err != nil {
			return false, err
		}
		s.mu.Lock()
		s.serial = params.SerialNumber
		if params.RefreshInterval > 0 {
			s.refreshInterval = time.Duration(params.RefreshInterval) * time.Second
		}
		if params.RetryInterval > 0 {
			s.retryInterval = time.Duration(params.RetryInterval) * time.Second
		}
		if params.ExpireInterval > 0 {
			s.expireInterval = time.Duration(params.ExpireInterval) * time.Second
		}
		// Apply accumulated VRPs to cache.
		announced := len(s.pendingVRPs)
		withdrawn := len(s.pendingDels)
		for _, vrp := range s.pendingDels {
			s.cache.Remove(vrp)
		}
		for _, vrp := range s.pendingVRPs {
			s.cache.Add(vrp)
		}
		s.pendingVRPs = nil
		s.pendingDels = nil
		s.mu.Unlock()

		logger().Info("rtr: sync complete",
			"address", s.address,
			"serial", params.SerialNumber,
			"announced", announced,
			"withdrawn", withdrawn,
			"refresh", params.RefreshInterval)
		return true, nil

	case pduCacheReset:
		// Cache cannot serve incremental, need full reset.
		s.mu.Lock()
		s.serial = 0
		s.mu.Unlock()
		return true, fmt.Errorf("rtr: cache reset received, will do full sync")

	case pduSerialNotify:
		// Ignore during sync per RFC 8210 Section 7.
		return false, nil

	case pduErrorRpt:
		errCode := hdr.SessionID // Error code is in bytes 2-3.
		if isFatalError(errCode) {
			return false, fmt.Errorf("rtr: fatal error code %d from cache", errCode)
		}
		logger().Warn("rtr: non-fatal error from cache", "code", errCode)
		return false, nil

	case pduRouterKey:
		// Router Key PDU (Type 9) - for BGPsec, skip for now.
		return false, nil
	}

	return false, fmt.Errorf("rtr: unknown PDU type %d", hdr.Type)
}

// close cleans up the session connection.
func (s *RTRSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
	s.state = sessionIdle
}

// State returns the current session state.
func (s *RTRSession) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}
