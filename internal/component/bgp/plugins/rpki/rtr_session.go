// Design: docs/architecture/plugin/rib-storage-design.md — RTR session lifecycle
// Overview: rpki.go — plugin entry point managing sessions
// Related: rtr_pdu.go — PDU wire format used by session
package rpki

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/network"
)

var (
	errRtrCacheResetReceivedWillDo = errors.New("rtr: cache reset received, will do full sync")
	errRtrVersionDowngrade         = errors.New("rtr: version downgrade required")
)

// RTR session states.
const (
	sessionIdle      = "idle"
	sessionConnect   = "connect"
	sessionEstablish = "establish"
)

// RTRSession manages a single RTR connection to a cache server.
type RTRSession struct {
	address       string
	port          uint16
	preference    uint8
	sourceAddress string

	conn      net.Conn
	state     string
	sessionID uint16
	serial    uint32
	version   uint8 // negotiated RTR protocol version (starts at rtrVersionMax)

	// Timing parameters from End of Data.
	refreshInterval time.Duration
	retryInterval   time.Duration
	expireInterval  time.Duration

	// pendingVRPs accumulates VRPs between Cache Response and End of Data.
	pendingVRPs []VRP
	pendingDels []VRP

	// pendingASPAs accumulates ASPA records between Cache Response and End of Data.
	pendingASPAs    []ASPARecord
	pendingASPADels []uint32

	mu        sync.Mutex
	stopCh    <-chan struct{}
	cache     *ROACache
	aspaCache *ASPACache

	// onASPAChange is called after ASPA data changes at End of Data.
	// The argument is the set of customer ASNs that were modified.
	onASPAChange func([]uint32)

	// onROAChange is called after the ROA cache (VRP set) changes at End of Data, so tracked
	// routes can be re-validated for RFC 6811 Section 4 origin re-validation.
	onROAChange func()
}

// NewRTRSession creates a new RTR session for the given cache server.
func NewRTRSession(address string, port uint16, pref uint8, sourceAddress string, cache *ROACache, aspaCache *ASPACache, stopCh <-chan struct{}) *RTRSession {
	return &RTRSession{
		address:         address,
		port:            port,
		preference:      pref,
		sourceAddress:   sourceAddress,
		state:           sessionIdle,
		version:         rtrVersionMax,
		refreshInterval: 3600 * time.Second,
		retryInterval:   600 * time.Second,
		expireInterval:  7200 * time.Second,
		cache:           cache,
		aspaCache:       aspaCache,
		stopCh:          stopCh,
	}
}

// Run is the long-lived goroutine for this RTR session.
// It connects, queries, receives VRPs, and reconnects on failure.
// On version mismatch (error code 4), downgrades and retries immediately.
func (s *RTRSession) Run() {
	for !s.stopped() {
		err := s.connectAndSync()
		if err != nil {
			if errors.Is(err, errRtrVersionDowngrade) {
				s.close()
				continue
			}
			logger().Warn("rtr: session error, will retry",
				"address", s.address, "error", err)
		}
		s.close()

		// Wait before retry, or exit on stop signal.
		select {
		case <-s.stopCh:
			return
		case <-time.After(s.retryInterval):
		}
	}
}

// stopped returns true if the stop channel has been closed.
func (s *RTRSession) stopped() bool {
	select {
	case <-s.stopCh:
		return true
	default: //nolint:gosimple // non-blocking channel check
		return false
	}
}

// connectAndSync establishes TCP connection and runs the RTR protocol.
func (s *RTRSession) connectAndSync() error {
	addr := net.JoinHostPort(s.address, strconv.Itoa(int(s.port)))
	dialer := &network.RealDialer{Timeout: 30 * time.Second}
	if err := dialer.SetSourceAddress(s.sourceAddress); err != nil {
		return err
	}

	// Cancel an in-progress dial when the session is stopped, so shutdown is
	// not blocked for up to the 30s connect timeout. The watcher is scoped to
	// the dial (the explicit cancelDial after the dial releases it), and the
	// deferred cancel is a panic-safe backstop -- both paths close dialCtx,
	// which the goroutine waits on.
	dialCtx, cancelDial := context.WithCancel(context.Background())
	defer cancelDial()
	go func() {
		select {
		case <-s.stopCh:
			cancelDial()
		case <-dialCtx.Done():
		}
	}()

	conn, err := dialer.DialContext(dialCtx, "tcp", addr)
	cancelDial()
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

	// Send initial query with current negotiated version.
	buf := make([]byte, pduSerialQueryLen)
	if s.serial == 0 {
		n := writeResetQuery(buf, 0, s.version)
		if _, err := conn.Write(buf[:n]); err != nil {
			return fmt.Errorf("write reset query: %w", err)
		}
	} else {
		n := writeSerialQuery(buf, 0, s.version, s.sessionID, s.serial)
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
		s.pendingASPAs = nil
		s.pendingASPADels = nil
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

	case pduASPA:
		if s.version < 2 {
			return false, fmt.Errorf("rtr: ASPA PDU received in v%d session (protocol violation)", s.version)
		}
		rec, announce, err := parseASPAPDU(buf)
		if err != nil {
			logger().Warn("rtr: malformed ASPA PDU, skipping", "error", err)
			return false, nil
		}
		if rec.CustomerAS == 0 {
			return false, nil
		}
		s.mu.Lock()
		if announce {
			s.pendingASPAs = append(s.pendingASPAs, rec)
		} else {
			s.pendingASPADels = append(s.pendingASPADels, rec.CustomerAS)
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
		// Apply accumulated VRPs to cache atomically.
		announced := len(s.pendingVRPs)
		withdrawn := len(s.pendingDels)
		s.cache.ApplyDelta(s.pendingDels, s.pendingVRPs)
		s.pendingVRPs = nil
		s.pendingDels = nil

		// Apply accumulated ASPA records.
		aspaAnnounced := len(s.pendingASPAs)
		aspaWithdrawn := len(s.pendingASPADels)
		var aspaChanged []uint32
		if s.aspaCache != nil && (aspaAnnounced > 0 || aspaWithdrawn > 0) {
			aspaChanged = s.aspaCache.ChangedCustomers(s.pendingASPADels, s.pendingASPAs)
			s.aspaCache.ApplyDelta(s.pendingASPADels, s.pendingASPAs)
		}
		s.pendingASPAs = nil
		s.pendingASPADels = nil
		s.mu.Unlock()

		// Notify ASPA change callback (re-validation trigger).
		if len(aspaChanged) > 0 && s.onASPAChange != nil {
			s.onASPAChange(aspaChanged)
		}

		// Notify ROA change callback (RFC 6811 Section 4: re-validate installed routes when the
		// VRP set changes). Any announce or withdraw can flip a covering prefix's state.
		if (announced > 0 || withdrawn > 0) && s.onROAChange != nil {
			s.onROAChange()
		}

		if m := rpkiMetricsPtr.Load(); m != nil {
			v4, v6 := s.cache.Count()
			m.vrpsCached.Set(float64(v4 + v6))
		}

		logger().Info("rtr: sync complete",
			"address", s.address,
			"serial", params.SerialNumber,
			"announced", announced,
			"withdrawn", withdrawn,
			"aspa-announced", aspaAnnounced,
			"aspa-withdrawn", aspaWithdrawn,
			"refresh", params.RefreshInterval)
		return true, nil

	case pduCacheReset:
		// Cache cannot serve incremental, need full reset.
		s.mu.Lock()
		s.serial = 0
		s.mu.Unlock()
		return true, errRtrCacheResetReceivedWillDo

	case pduSerialNotify:
		// Ignore during sync per RFC 8210 Section 7.
		return false, nil

	case pduErrorRpt:
		errCode := hdr.SessionID // Error code is in bytes 2-3.
		// RFC 9582 Section 7: Unsupported Protocol Version -> downgrade.
		if errCode == errUnsupportedVersion && s.version > rtrVersionMin {
			s.version--
			logger().Info("rtr: version downgrade", "address", s.address, "new-version", s.version)
			return false, errRtrVersionDowngrade
		}
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

// SessionSnapshot holds a point-in-time copy of session diagnostic fields.
type SessionSnapshot struct {
	Address         string
	Port            uint16
	Preference      uint8
	State           string
	Version         uint8
	SessionID       uint16
	Serial          uint32
	RefreshInterval time.Duration
	RetryInterval   time.Duration
	ExpireInterval  time.Duration
}

// Snapshot returns a point-in-time copy of diagnostic fields.
func (s *RTRSession) Snapshot() SessionSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SessionSnapshot{
		Address:         s.address,
		Port:            s.port,
		Preference:      s.preference,
		State:           s.state,
		Version:         s.version,
		SessionID:       s.sessionID,
		Serial:          s.serial,
		RefreshInterval: s.refreshInterval,
		RetryInterval:   s.retryInterval,
		ExpireInterval:  s.expireInterval,
	}
}
