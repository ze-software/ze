// RFC: rfc/short/rfc8210.md — Section 10, the router polls its caches in preference order
// RFC: rfc/short/rfc8210.md — Section 6, the Refresh, Retry and Expire Intervals time the poll
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

	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/network"
)

var (
	errRtrCacheResetReceivedWillDo = errors.New("rtr: cache reset received, will do full sync")
	errRtrVersionDowngrade         = errors.New("rtr: version downgrade required")
	errRtrUnsupportedProtocol      = errors.New("rtr: unrecognized protocol version")
	errRtrDataExpired              = errors.New("rtr: serial response cannot update expired data")
	errRtrNoDataAvailable          = errors.New("rtr: cache has no data available")
)

// RTR session states.
const (
	sessionIdle      = "idle"
	sessionConnect   = "connect"
	sessionEstablish = "establish"
)

// Timer defaults, used until an End of Data PDU carries the cache's own values.
//
// RFC 8210 Section 6 gives each interval a default and a range, and states the default the
// router uses before a cache has spoken: "Refresh Interval ... Default: 3600 seconds",
// "Retry Interval ... Default: 600 seconds", "Expire Interval ... Default: 7200 seconds".
const (
	rtrIntervalRefreshDefault = 3600 * time.Second
	rtrIntervalRetryDefault   = 600 * time.Second
	rtrIntervalExpireDefault  = 7200 * time.Second
)

// RTRSession manages a single RTR connection to a cache server.
type RTRSession struct {
	address       string
	port          uint16
	preference    uint8
	sourceAddress string
	tlsSettings   *rtrTLSSettings
	pkiConfig     *pki.PKIConfig

	conn      net.Conn
	state     string
	sessionID uint16
	serial    uint32
	version   uint8 // negotiated RTR protocol version (starts at rtrVersionMax)

	// synced is true once an End of Data PDU has completed a sync on this session.
	// state cannot answer that question: it returns to "idle" between polls, and
	// "establish" only says a Cache Response arrived.
	synced bool

	// fullSync is true while the query in flight is a Reset Query whose answer is the
	// cache's whole set, so its End of Data REPLACES the cached set instead of merging
	// into it. Set by startFullSync when the group switches cache, and by a Cache Reset
	// PDU, which says the cache cannot serve the delta this session asked for. Cleared
	// by the End of Data that consumed it.
	fullSync bool

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

	mu             sync.Mutex
	stopCh         <-chan struct{}
	cache          *ROACache
	aspaCache      *aSPACache
	dataLease      *rtrDataLease
	dataGeneration uint64

	// onASPAChange is called after ASPA data changes at End of Data.
	// The argument is the set of customer ASNs that were modified.
	onASPAChange func([]uint32)

	// onROAChange is called after the ROA cache (VRP set) changes at End of Data, so tracked
	// routes can be re-validated for RFC 6811 Section 4 origin re-validation.
	onROAChange func()
}

// newRTRSession creates a new RTR session for the given cache server.
func newRTRSession(address string, port uint16, pref uint8, sourceAddress string, cache *ROACache, aspaCache *aSPACache, stopCh <-chan struct{}) *RTRSession {
	return &RTRSession{
		address:         address,
		port:            port,
		preference:      pref,
		sourceAddress:   sourceAddress,
		state:           sessionIdle,
		version:         rtrVersionMax,
		refreshInterval: rtrIntervalRefreshDefault,
		retryInterval:   rtrIntervalRetryDefault,
		expireInterval:  rtrIntervalExpireDefault,
		cache:           cache,
		aspaCache:       aspaCache,
		stopCh:          stopCh,
	}
}

// cacheGroup polls the configured RTR cache servers in preference order and loads data from
// one of them at a time.
//
// RFC 8210 Section 10: "The client router attempts to establish a session with each potential
// serving cache in preference order and then starts to load data from the most preferred cache
// to which it can connect and authenticate." The same section defines the leaf an operator
// sets: "Preference: An unsigned integer denoting the router's preference to connect to that
// cache; the lower the value, the more preferred."
//
// It owns one goroutine. The caller creates the group, starts Run in that goroutine, and stops
// it by closing the stop channel the sessions carry; the caller MUST wait for Run to return
// before it reads the sessions for anything but a Snapshot.
type cacheGroup struct {
	// sessions holds the configured cache servers, most preferred first. newCacheGroup's
	// one caller establishes that order.
	sessions []*RTRSession

	// holder is the session whose data the ROA and ASPA caches carry, nil until the first
	// sync completes. A round won by any other session replaces that data rather than
	// merging into it: RFC 8210 Section 10 says caches "simply cannot be rigorously
	// synchronous", so one cache's serial says nothing about another cache's set.
	holder *RTRSession

	stopCh <-chan struct{}
}

// newCacheGroup creates the group over sessions, which MUST be ordered most preferred first.
func newCacheGroup(sessions []*RTRSession, stopCh <-chan struct{}) *cacheGroup {
	if len(sessions) != 0 {
		sessions[0].mu.Lock()
		lease := sessions[0].dataLeaseLocked()
		sessions[0].mu.Unlock()
		for _, session := range sessions[1:] {
			session.mu.Lock()
			session.dataLease = lease
			session.mu.Unlock()
		}
	}
	return &cacheGroup{sessions: sessions, stopCh: stopCh}
}

// Run polls the caches until the stop channel closes.
//
// The loop has no other bound, and that is the decision: a router polls its caches for as long
// as it runs. Each round restarts at the head of the preference order, which is how a recovered
// cache takes the load back, which is the same section's "When a more-preferred cache becomes
// available, if resources allow, it would be prudent for the client to start fetching from
// that cache".
func (g *cacheGroup) Run() {
	if len(g.sessions) == 0 {
		return
	}

	for !stopped(g.stopCh) {
		delay := g.poll()

		select {
		case <-g.stopCh:
			return
		case <-time.After(delay):
		}
	}
}

// poll runs one round over the caches and returns how long to wait before the next one.
//
// The first cache that completes a sync ends the round and its Refresh Interval times the next
// one. When no cache answers, the Retry Interval of the last one tried does, which is RFC 8210
// Section 6 read over a list rather than over a single server.
func (g *cacheGroup) poll() time.Duration {
	delay := rtrIntervalRetryDefault
	for _, s := range g.sessions {
		if s.stopped() {
			return delay
		}

		if s != g.holder {
			s.startFullSync()
		}

		err := s.syncOnce()
		if err == nil {
			g.holder = s
			if s.Snapshot().Synced {
				setSessionsActive(1)
			} else {
				setSessionsActive(0)
			}
			return s.pollDelay(true)
		}

		logger().Warn("rtr: cache did not answer, trying the next one in preference order",
			"address", s.address, "preference", s.preference, "error", err)
		delay = s.pollDelay(false)
	}

	setSessionsActive(0)
	return delay
}

// syncOnce runs one connect-query-sync cycle against this cache and closes the connection
// behind it.
//
// A version downgrade is not a failure of the cache, so it costs the cache no turn in the
// preference order: RFC 8210 Section 7 has the router retry at the lower version. The retry
// count is bounded by the version range, because connectAndSync asks for a downgrade only
// while the session version is above rtrVersionMin and lowers it each time.
func (s *RTRSession) syncOnce() error {
	err := errRtrVersionDowngrade
	for range rtrVersionMax - rtrVersionMin + 1 {
		err = s.connectAndSync()
		s.close()
		if !errors.Is(err, errRtrVersionDowngrade) {
			return err
		}
	}
	return err
}

// startFullSync makes this session's next query a Reset Query and has its End of Data replace
// the cached set instead of merging into it.
//
// RFC 8210 Section 10: "If the client decides to switch to a new cache, it SHOULD retain the
// data from the previous cache until it has a full set of data from one or more other caches."
// The replacement happens inside the End of Data handler, under one lock, so the previous
// cache's data stays live for every validation until the new full set is there to take over.
func (s *RTRSession) startFullSync() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serial = 0
	s.fullSync = true
}

// stopped reports whether the stop channel has been closed.
func stopped(stopCh <-chan struct{}) bool {
	select {
	case <-stopCh:
		return true
	default:
		return false
	}
}

// pollDelay returns how long to wait before the next Serial Query or Reset Query.
//
// RFC 8210 Section 6: the Refresh Interval "tells the router how long to wait before
// next attempting to poll the cache", and its countdown "starts upon receipt of the
// containing End Of Data PDU". The Retry Interval "tells the router how long to wait
// before retrying a failed Serial Query or Reset Query", and its countdown "starts
// upon failure of the query". Waiting the retry interval after a sync completed polls
// the cache more often than the cache asked for: against a cache sending refresh 3600
// and retry 600, six times more often.
//
// A query that failed takes the retry interval whatever a previous End of Data said,
// which also covers the same section's "if the router has never issued a successful
// query against a particular cache, it SHOULD retry periodically using the default
// Retry Interval".
func (s *RTRSession) pollDelay(synced bool) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if synced {
		return s.refreshInterval
	}
	return s.retryInterval
}

// stopped reports whether this session's stop channel has been closed.
func (s *RTRSession) stopped() bool {
	return stopped(s.stopCh)
}

// connectAndSync authenticates the selected transport and runs the RTR protocol.
func (s *RTRSession) connectAndSync() error {
	addr := net.JoinHostPort(s.address, strconv.Itoa(int(s.port)))
	dialer := &network.RealDialer{Timeout: 30 * time.Second}
	if err := dialer.SetSourceAddress(s.sourceAddress); err != nil {
		return err
	}

	// The dial and TLS handshake share one bounded, cancellable attempt. A
	// stopped session must not wait for an unauthenticated peer to finish TLS.
	dialCtx, cancelDial := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelDial()
	go func() {
		select {
		case <-s.stopCh:
			cancelDial()
		case <-dialCtx.Done():
		}
	}()

	conn, err := dialer.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("connect %s: %w", addr, err)
	}

	s.mu.Lock()
	if s.stopped() {
		s.mu.Unlock()
		_ = conn.Close()
		return context.Canceled
	}
	s.conn = conn
	s.state = sessionConnect
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.state = sessionIdle
		s.mu.Unlock()
	}()
	if s.tlsSettings != nil {
		secure, err := s.startTLS(dialCtx, conn)
		if err != nil {
			_ = conn.Close()
			return err
		}
		conn = secure
		s.mu.Lock()
		s.conn = conn
		s.mu.Unlock()
	}
	cancelDial()

	// Send initial query with current negotiated version.
	s.prepareQuery()
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
		// Bound an idle read separately from the published data's lease. A
		// received PDU may extend this deadline, never the payload lifetime.
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
			if hdr.Type != pduErrorRpt && (hdr.Type == pduASPA || errors.Is(err, errRtrUnsupportedProtocol)) {
				// 8210bis Section 5.12: an invalid provider list requires Error
				// Report 9. Other malformed ASPA PDUs carry Corrupt Data (0).
				code := uint16(0)
				if errors.Is(err, errRtrUnsupportedProtocol) {
					code = errUnsupportedVersion
				} else if errors.Is(err, errASPAProviderList) {
					code = 9
				}
				report := make([]byte, 16+len(pduBuf))
				n := writeErrorReport(report, s.version, code, pduBuf)
				if deadlineErr := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); deadlineErr != nil {
					return errors.Join(err, deadlineErr)
				}
				if _, writeErr := conn.Write(report[:n]); writeErr != nil {
					return errors.Join(err, writeErr)
				}
			}
			return err
		}
		if done {
			return nil
		}
	}
}

// handlePDU processes a single RTR PDU. Returns true when session sync is complete.
func (s *RTRSession) handlePDU(hdr rTRHeader, buf []byte) (bool, error) {
	// Section 7: unrecognized versions require downgrade or Error Report 4
	// and termination. Serial Notify is ignored during initial synchronization
	// regardless of its version; Error Reports must not elicit Error Reports.
	if hdr.Type != pduSerialNotify && hdr.Type != pduErrorRpt {
		if hdr.Version < rtrVersionMin || hdr.Version > rtrVersionMax {
			return false, errRtrUnsupportedProtocol
		}
	}
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
			return false, err
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
		received := time.Now()
		s.mu.Lock()
		lease := s.dataLeaseLocked()
		lease.mu.Lock()
		// An overdue timer may not have run yet. A serial delta cannot renew
		// a base whose absolute lifetime ended before this EOD arrived.
		expired := !lease.deadline.IsZero() && !received.Before(lease.deadline)
		if !s.fullSync && (s.dataGeneration != lease.generation || expired) {
			s.serial = 0
			s.fullSync = true
			lease.mu.Unlock()
			s.mu.Unlock()
			return false, errRtrDataExpired
		}
		s.serial = params.SerialNumber
		s.synced = true
		if params.RefreshInterval > 0 {
			s.refreshInterval = time.Duration(params.RefreshInterval) * time.Second
		}
		if params.RetryInterval > 0 {
			s.retryInterval = time.Duration(params.RetryInterval) * time.Second
		}
		if params.ExpireInterval > 0 {
			s.expireInterval = time.Duration(params.ExpireInterval) * time.Second
		}
		// Apply the accumulated VRPs. A Reset Query is answered with the cache's whole
		// set, so its End of Data REPLACES what ze holds; a Serial Query is answered with
		// a delta against the set ze already has. Both run under the lock this branch
		// holds, so no validation reads a half-applied set.
		fullSync := s.fullSync
		s.fullSync = false
		announced := len(s.pendingVRPs)
		withdrawn := len(s.pendingDels)
		if fullSync {
			lease.generation++
			s.cache.Replace(s.pendingVRPs)
		} else {
			s.cache.ApplyDelta(s.pendingDels, s.pendingVRPs)
		}
		s.pendingVRPs = nil
		s.pendingDels = nil

		// Apply the accumulated ASPA records, the same two ways.
		aspaAnnounced := len(s.pendingASPAs)
		aspaWithdrawn := len(s.pendingASPADels)
		var aspaChanged []uint32
		if s.aspaCache != nil {
			switch {
			case fullSync:
				aspaChanged = s.aspaCache.Replace(s.pendingASPAs)
			case aspaAnnounced > 0 || aspaWithdrawn > 0:
				aspaChanged = s.aspaCache.changedCustomers(s.pendingASPADels, s.pendingASPAs)
				s.aspaCache.ApplyDelta(s.pendingASPADels, s.pendingASPAs)
			}
		}
		s.pendingASPAs = nil
		s.pendingASPADels = nil
		s.dataGeneration = lease.generation
		lease.renewLocked(received, s.expireInterval)
		lease.mu.Unlock()
		s.mu.Unlock()

		// Notify ASPA change callback (re-validation trigger).
		if len(aspaChanged) > 0 && s.onASPAChange != nil {
			s.onASPAChange(aspaChanged)
		}

		// Notify ROA change callback (RFC 6811 Section 4: re-validate installed routes when the
		// VRP set changes). Any announce or withdraw can flip a covering prefix's state, and a
		// full sync swapped the whole set, so it can flip one even when this cache sent no
		// prefix at all.
		if (fullSync || announced > 0 || withdrawn > 0) && s.onROAChange != nil {
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
		// The cache cannot serve the delta this session asked for, so the next query is a
		// Reset Query and its answer is the whole set. It replaces what ze holds rather
		// than merging into it: the records this session withdrew while ze was away would
		// otherwise stay valid forever, with nothing left to withdraw them.
		s.mu.Lock()
		s.serial = 0
		s.fullSync = true
		s.mu.Unlock()
		return true, errRtrCacheResetReceivedWillDo

	case pduSerialNotify:
		// Ignore during sync per RFC 8210 Section 7.
		return false, nil

	case pduErrorRpt:
		errCode := hdr.SessionID // Error code is in bytes 2-3.
		// 8210bis Section 7: retry at the cache's advertised version C,
		// never at a guessed version or one this cache has already rejected.
		if errCode == errUnsupportedVersion {
			if hdr.Version < rtrVersionMin || hdr.Version >= s.version {
				return false, fmt.Errorf("rtr: no supported common version with cache version %d", hdr.Version)
			}
			s.mu.Lock()
			s.version = hdr.Version
			// Serial numbers and session IDs belong to the negotiated version.
			// Fetch a complete lower-version set; no ASPAs survive a v1 sync.
			s.serial = 0
			s.fullSync = true
			s.mu.Unlock()
			logger().Info("rtr: version downgrade", "address", s.address, "new-version", s.version)
			return false, errRtrVersionDowngrade
		}
		if errCode == errNoDataAvail {
			// Section 8.4: leave this response and try the next cache. If none
			// answers, the group retries with a Reset Query, not an old serial.
			s.mu.Lock()
			s.serial = 0
			s.fullSync = true
			s.mu.Unlock()
			return false, errRtrNoDataAvailable
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

// SessionSnapshot holds a point-in-time copy of session diagnostic fields.
type SessionSnapshot struct {
	Address    string
	Port       uint16
	Preference uint8
	State      string
	// Synced reports a completed sync whose published data has not expired.
	Synced          bool
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
		Synced:          s.synced && (s.dataLease == nil || s.dataLease.current(s.dataGeneration)),
		Version:         s.version,
		SessionID:       s.sessionID,
		Serial:          s.serial,
		RefreshInterval: s.refreshInterval,
		RetryInterval:   s.retryInterval,
		ExpireInterval:  s.expireInterval,
	}
}
