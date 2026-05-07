// Design: plan/spec-bng-4-scale-testing.md -- L2TP scale test tooling
//
// Subcommand "ze-test l2tp-scale" provides a Go LAC simulator and
// embedded mock RADIUS server for control-plane scale testing.
// The simulator speaks real L2TP wire protocol over UDP loopback.

package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/radius"
)

var _ = register("l2tp-scale", "L2TP scale test: LAC simulator + mock RADIUS", l2tpScaleCmd)

func l2tpScaleCmd() int {
	fs := flag.NewFlagSet("l2tp-scale", flag.ExitOnError)

	var (
		target      string
		tunnels     int
		sessPerTun  int
		secret      string
		radiusAddr  string
		radiusKey   string
		radiusDelay time.Duration
		dwellTime   time.Duration
		jsonOutput  bool
	)

	fs.StringVar(&target, "target", "127.0.0.1:1701", "Ze LNS address (host:port)")
	fs.IntVar(&tunnels, "tunnels", 10, "number of tunnels")
	fs.IntVar(&sessPerTun, "sessions", 200, "sessions per tunnel")
	fs.StringVar(&secret, "secret", "s3cr3t", "L2TP shared secret")
	fs.StringVar(&radiusAddr, "radius-addr", "127.0.0.1:0", "mock RADIUS bind address")
	fs.StringVar(&radiusKey, "radius-key", "testing123", "RADIUS shared secret")
	fs.DurationVar(&radiusDelay, "radius-delay", 0, "artificial RADIUS response delay")
	fs.DurationVar(&dwellTime, "dwell", 2*time.Second, "steady-state dwell before teardown")
	fs.BoolVar(&jsonOutput, "json", false, "output results as JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test l2tp-scale [options]

L2TP control-plane scale test. Starts a mock RADIUS server, then
simulates multiple LAC tunnels with many sessions each.

Options:
`)
		fs.PrintDefaults()
	}

	if len(os.Args) > 1 && isHelpArg(os.Args[1]) {
		fs.Usage()
		return 0
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		cancel()
	}()

	rad, err := newMockRADIUS(radiusAddr, []byte(radiusKey), radiusDelay)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mock RADIUS: %v\n", err)
		return 1
	}
	go rad.serve(ctx)
	defer rad.shutdown()

	fmt.Fprintf(os.Stderr, "mock RADIUS listening on %s\n", rad.addr())

	udpTarget, err := net.ResolveUDPAddr("udp4", target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve target: %v\n", err)
		return 1
	}

	sim := &lacSimulator{
		target:      udpTarget,
		secret:      []byte(secret),
		tunnelCount: tunnels,
		sessPerTun:  sessPerTun,
		dwell:       dwellTime,
	}

	result := sim.run(ctx)

	result.RADIUSAuth = rad.authCount.Load()
	result.RADIUSAcctStart = rad.acctStarts.Load()
	result.RADIUSAcctStop = rad.acctStops.Load()
	result.RADIUSAcctInterim = rad.acctInterims.Load()

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "json encode: %v\n", err)
		}
	} else {
		printScaleResult(result)
	}

	if len(result.Errors) > 0 {
		return 1
	}
	return 0
}

func printScaleResult(r scaleResult) {
	fmt.Printf("\n=== L2TP Scale Test Results ===\n")
	fmt.Printf("Tunnels:     %d/%d\n", r.TunnelsUp, r.TunnelsRequested)
	fmt.Printf("Sessions:    %d/%d\n", r.SessionsUp, r.SessionsRequested)
	fmt.Printf("Setup time:  %s\n", r.SetupTime)
	fmt.Printf("Teardown:    %s\n", r.TeardownTime)
	fmt.Printf("Rate:        %.1f sessions/s\n", r.SessionsPerSec)
	fmt.Printf("RADIUS auth: %d  acct-start: %d  acct-stop: %d  interim: %d\n",
		r.RADIUSAuth, r.RADIUSAcctStart, r.RADIUSAcctStop, r.RADIUSAcctInterim)
	if len(r.Errors) > 0 {
		fmt.Printf("Errors (%d):\n", len(r.Errors))
		for _, e := range r.Errors {
			fmt.Printf("  - %s\n", e)
		}
	}
	fmt.Println()
}

// --- Results ---

type scaleResult struct {
	TunnelsRequested  int           `json:"tunnels-requested"`
	TunnelsUp         int           `json:"tunnels-up"`
	SessionsRequested int           `json:"sessions-requested"`
	SessionsUp        int           `json:"sessions-up"`
	SetupTime         time.Duration `json:"setup-time-ns"`
	TeardownTime      time.Duration `json:"teardown-time-ns"`
	SessionsPerSec    float64       `json:"sessions-per-sec"`
	RADIUSAuth        int64         `json:"radius-auth"`
	RADIUSAcctStart   int64         `json:"radius-acct-start"`
	RADIUSAcctStop    int64         `json:"radius-acct-stop"`
	RADIUSAcctInterim int64         `json:"radius-acct-interim"`
	Errors            []string      `json:"errors,omitempty"`
}

// --- Mock RADIUS Server ---

type mockRADIUSServer struct {
	sharedKey []byte
	latency   time.Duration
	conn      *net.UDPConn

	authCount    atomic.Int64
	acctStarts   atomic.Int64
	acctStops    atomic.Int64
	acctInterims atomic.Int64
}

func newMockRADIUS(addr string, sharedKey []byte, latency time.Duration) (*mockRADIUSServer, error) {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("resolve: %w", err)
	}
	conn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	return &mockRADIUSServer{
		sharedKey: sharedKey,
		latency:   latency,
		conn:      conn,
	}, nil
}

func (m *mockRADIUSServer) addr() string {
	return m.conn.LocalAddr().String()
}

func (m *mockRADIUSServer) serve(ctx context.Context) {
	buf := make([]byte, radius.MaxPacketLen)
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if err := m.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
			return
		}
		n, remote, err := m.conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		data := make([]byte, n)
		copy(data, buf[:n])
		go m.handlePacket(data, remote)
	}
}

func (m *mockRADIUSServer) handlePacket(data []byte, remote *net.UDPAddr) {
	if m.latency > 0 {
		time.Sleep(m.latency)
	}

	pkt, err := radius.Decode(data)
	if err != nil {
		return
	}

	var respCode uint8

	switch pkt.Code {
	case radius.CodeAccessRequest:
		m.authCount.Add(1)
		respCode = radius.CodeAccessAccept
	case radius.CodeAccountingReq:
		statusVal := pkt.FindAttr(radius.AttrAcctStatusType)
		if len(statusVal) == 4 {
			st := binary.BigEndian.Uint32(statusVal)
			switch st {
			case radius.AcctStatusStart:
				m.acctStarts.Add(1)
			case radius.AcctStatusStop:
				m.acctStops.Add(1)
			case radius.AcctStatusInterimUpdate:
				m.acctInterims.Add(1)
			}
		}
		respCode = radius.CodeAccountingResp
	default:
		return
	}

	resp := &radius.Packet{
		Code:       respCode,
		Identifier: pkt.Identifier,
	}

	var respBuf [radius.MaxPacketLen]byte
	n, encErr := resp.EncodeTo(respBuf[:], 0)
	if encErr != nil {
		return
	}

	if pkt.Code == radius.CodeAccessRequest {
		auth := radius.ResponseAuthenticator(
			respCode, pkt.Identifier,
			binary.BigEndian.Uint16(respBuf[2:4]),
			pkt.Authenticator, respBuf[radius.HeaderLen:n], m.sharedKey,
		)
		copy(respBuf[4:4+radius.AuthenticatorLen], auth[:])
	} else {
		auth := radius.AccountingRequestAuth(respBuf[:n], n, m.sharedKey)
		copy(respBuf[4:4+radius.AuthenticatorLen], auth[:])
	}

	if _, err := m.conn.WriteToUDP(respBuf[:n], remote); err != nil {
		fmt.Fprintf(os.Stderr, "mock RADIUS: write to %s: %v\n", remote, err)
	}
}

func (m *mockRADIUSServer) shutdown() {
	if err := m.conn.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "mock RADIUS close: %v\n", err)
	}
}

// --- LAC Simulator ---

type lacSimulator struct {
	target      *net.UDPAddr
	secret      []byte
	tunnelCount int
	sessPerTun  int
	dwell       time.Duration
}

type tunnelState struct {
	localTID  uint16
	remoteTID uint16
	ns        uint16
	peerNs    uint16 // last Ns received from peer; our Nr = peerNs + 1
	sessions  []sessionState
}

type sessionState struct {
	localSID  uint16
	remoteSID uint16
	up        bool
}

func (s *lacSimulator) run(ctx context.Context) scaleResult {
	result := scaleResult{
		TunnelsRequested:  s.tunnelCount,
		SessionsRequested: s.tunnelCount * s.sessPerTun,
	}

	setupStart := time.Now()

	var mu sync.Mutex
	var wg sync.WaitGroup
	tunnels := make([]*tunnelState, s.tunnelCount)
	conns := make([]*net.UDPConn, s.tunnelCount)

	for i := range s.tunnelCount {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn, err := net.DialUDP("udp4", nil, s.target)
			if err != nil {
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("tunnel %d: dial: %v", idx, err))
				mu.Unlock()
				return
			}
			conns[idx] = conn

			ts, errs := s.setupTunnel(ctx, conn, uint16(idx+1))
			if len(errs) > 0 {
				mu.Lock()
				result.Errors = append(result.Errors, errs...)
				mu.Unlock()
				return
			}
			tunnels[idx] = ts

			sessErrs := s.setupSessions(ctx, conn, ts)
			if len(sessErrs) > 0 {
				mu.Lock()
				result.Errors = append(result.Errors, sessErrs...)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	setupDone := time.Now()
	result.SetupTime = setupDone.Sub(setupStart)

	for _, ts := range tunnels {
		if ts == nil {
			continue
		}
		result.TunnelsUp++
		for _, sess := range ts.sessions {
			if sess.up {
				result.SessionsUp++
			}
		}
	}

	if result.SessionsUp > 0 {
		result.SessionsPerSec = float64(result.SessionsUp) / result.SetupTime.Seconds()
	}

	if s.dwell > 0 && ctx.Err() == nil {
		select {
		case <-time.After(s.dwell):
		case <-ctx.Done():
		}
	}

	teardownStart := time.Now()
	var twg sync.WaitGroup
	for i, ts := range tunnels {
		if ts == nil || conns[i] == nil {
			continue
		}
		twg.Add(1)
		go func(conn *net.UDPConn, t *tunnelState) {
			defer twg.Done()
			s.teardownTunnel(conn, t)
		}(conns[i], ts)
	}
	twg.Wait()
	result.TeardownTime = time.Since(teardownStart)

	for _, c := range conns {
		if c != nil {
			if err := c.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "close conn: %v\n", err)
			}
		}
	}

	return result
}

func (s *lacSimulator) setupTunnel(ctx context.Context, conn *net.UDPConn, localTID uint16) (*tunnelState, []string) {
	ts := &tunnelState{localTID: localTID}
	challenge := make([]byte, 16)
	for i := range challenge {
		challenge[i] = byte(localTID + uint16(i))
	}

	var buf [1500]byte
	off := l2tp.ControlHeaderLen
	off += l2tp.WriteAVPUint16(buf[:], off, true, l2tp.AVPMessageType, uint16(l2tp.MsgSCCRQ))
	off += l2tp.WriteAVPBytes(buf[:], off, true, 0, l2tp.AVPProtocolVersion, []byte{0x01, 0x00})
	off += l2tp.WriteAVPUint32(buf[:], off, true, l2tp.AVPFramingCapabilities, 0x3)
	off += l2tp.WriteAVPUint32(buf[:], off, true, l2tp.AVPBearerCapabilities, 0x0)
	off += l2tp.WriteAVPString(buf[:], off, true, l2tp.AVPHostName, fmt.Sprintf("lac-sim-%d", localTID))
	off += l2tp.WriteAVPUint16(buf[:], off, true, l2tp.AVPAssignedTunnelID, localTID)
	off += l2tp.WriteAVPUint16(buf[:], off, true, l2tp.AVPReceiveWindowSize, 8)
	off += l2tp.WriteAVPBytes(buf[:], off, true, 0, l2tp.AVPChallenge, challenge)
	l2tp.WriteControlHeader(buf[:], 0, uint16(off), 0, 0, ts.ns, 0)
	ts.ns++

	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil, []string{fmt.Sprintf("tunnel %d: set deadline: %v", localTID, err)}
	}
	if _, err := conn.Write(buf[:off]); err != nil {
		return nil, []string{fmt.Sprintf("tunnel %d: send SCCRQ: %v", localTID, err)}
	}

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil, []string{fmt.Sprintf("tunnel %d: set deadline: %v", localTID, err)}
	}
	var recvBuf [1500]byte
	var remoteTID uint16
	var peerChallenge []byte
	for range 20 {
		n, err := conn.Read(recvBuf[:])
		if err != nil {
			if ctx.Err() != nil {
				return nil, []string{fmt.Sprintf("tunnel %d: canceled", localTID)}
			}
			continue
		}
		hdr, err := l2tp.ParseMessageHeader(recvBuf[:n])
		if err != nil || !hdr.IsControl {
			continue
		}
		it := l2tp.NewAVPIterator(recvBuf[hdr.PayloadOff:int(hdr.Length)])
		var msgType l2tp.MessageType
		for {
			_, aType, _, value, ok := it.Next()
			if !ok {
				break
			}
			switch aType { //nolint:exhaustive // only extracting fields we need
			case l2tp.AVPMessageType:
				if len(value) == 2 {
					msgType = l2tp.MessageType(binary.BigEndian.Uint16(value))
				}
			case l2tp.AVPAssignedTunnelID:
				if len(value) == 2 {
					remoteTID = binary.BigEndian.Uint16(value)
				}
			case l2tp.AVPChallenge:
				peerChallenge = make([]byte, len(value))
				copy(peerChallenge, value)
			}
		}
		if msgType == l2tp.MsgSCCRP {
			ts.peerNs = hdr.Ns
			break
		}
	}

	if remoteTID == 0 {
		return nil, []string{fmt.Sprintf("tunnel %d: no SCCRP received", localTID)}
	}
	ts.remoteTID = remoteTID

	off = l2tp.ControlHeaderLen
	off += l2tp.WriteAVPUint16(buf[:], off, true, l2tp.AVPMessageType, uint16(l2tp.MsgSCCCN))
	if peerChallenge != nil && len(s.secret) > 0 {
		resp := l2tp.ChallengeResponse(byte(l2tp.MsgSCCCN), s.secret, peerChallenge)
		off += l2tp.WriteAVPBytes(buf[:], off, true, 0, l2tp.AVPChallengeResponse, resp[:])
	}
	l2tp.WriteControlHeader(buf[:], 0, uint16(off), remoteTID, 0, ts.ns, ts.peerNs+1)
	ts.ns++

	if _, err := conn.Write(buf[:off]); err != nil {
		return nil, []string{fmt.Sprintf("tunnel %d: send SCCCN: %v", localTID, err)}
	}

	if mt := drainZLB(conn, recvBuf[:]); mt == l2tp.MsgStopCCN {
		return nil, []string{fmt.Sprintf("tunnel %d: peer sent StopCCN after SCCCN", localTID)}
	}

	return ts, nil
}

func (s *lacSimulator) setupSessions(ctx context.Context, conn *net.UDPConn, ts *tunnelState) []string {
	var errs []string
	var recvBuf [1500]byte

	for i := range s.sessPerTun {
		if ctx.Err() != nil {
			errs = append(errs, fmt.Sprintf("tunnel %d: canceled at session %d", ts.localTID, i))
			break
		}
		localSID := uint16(i + 1)

		var buf [1500]byte
		off := l2tp.ControlHeaderLen
		off += l2tp.WriteAVPUint16(buf[:], off, true, l2tp.AVPMessageType, uint16(l2tp.MsgICRQ))
		off += l2tp.WriteAVPUint16(buf[:], off, true, l2tp.AVPAssignedSessionID, localSID)
		off += l2tp.WriteAVPUint32(buf[:], off, true, l2tp.AVPCallSerialNumber, uint32(ts.localTID)*10000+uint32(localSID))
		l2tp.WriteControlHeader(buf[:], 0, uint16(off), ts.remoteTID, 0, ts.ns, ts.peerNs+1)
		ts.ns++

		if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			errs = append(errs, fmt.Sprintf("tunnel %d session %d: deadline: %v", ts.localTID, localSID, err))
			continue
		}
		if _, err := conn.Write(buf[:off]); err != nil {
			errs = append(errs, fmt.Sprintf("tunnel %d session %d: send ICRQ: %v", ts.localTID, localSID, err))
			continue
		}

		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			errs = append(errs, fmt.Sprintf("tunnel %d session %d: deadline: %v", ts.localTID, localSID, err))
			continue
		}
		var remoteSID uint16
		for range 10 {
			n, err := conn.Read(recvBuf[:])
			if err != nil {
				if ctx.Err() != nil {
					break
				}
				continue
			}
			hdr, err := l2tp.ParseMessageHeader(recvBuf[:n])
			if err != nil || !hdr.IsControl {
				continue
			}
			if int(hdr.Length) <= hdr.PayloadOff {
				continue
			}
			it := l2tp.NewAVPIterator(recvBuf[hdr.PayloadOff:int(hdr.Length)])
			var msgType l2tp.MessageType
			for {
				_, aType, _, value, ok := it.Next()
				if !ok {
					break
				}
				switch aType { //nolint:exhaustive // only extracting fields we need
				case l2tp.AVPMessageType:
					if len(value) == 2 {
						msgType = l2tp.MessageType(binary.BigEndian.Uint16(value))
					}
				case l2tp.AVPAssignedSessionID:
					if len(value) == 2 {
						remoteSID = binary.BigEndian.Uint16(value)
					}
				}
			}
			if msgType == l2tp.MsgICRP {
				ts.peerNs = hdr.Ns
				break
			}
		}

		if remoteSID == 0 {
			errs = append(errs, fmt.Sprintf("tunnel %d session %d: no ICRP", ts.localTID, localSID))
			continue
		}

		off = l2tp.ControlHeaderLen
		off += l2tp.WriteAVPUint16(buf[:], off, true, l2tp.AVPMessageType, uint16(l2tp.MsgICCN))
		off += l2tp.WriteAVPUint32(buf[:], off, true, l2tp.AVPTxConnectSpeed, 10000000)
		off += l2tp.WriteAVPUint32(buf[:], off, true, l2tp.AVPFramingType, 2)
		l2tp.WriteControlHeader(buf[:], 0, uint16(off), ts.remoteTID, remoteSID, ts.ns, ts.peerNs+1)
		ts.ns++

		if _, err := conn.Write(buf[:off]); err != nil {
			errs = append(errs, fmt.Sprintf("tunnel %d session %d: send ICCN: %v", ts.localTID, localSID, err))
			continue
		}

		if mt := drainZLB(conn, recvBuf[:]); mt == l2tp.MsgCDN {
			errs = append(errs, fmt.Sprintf("tunnel %d session %d: peer sent CDN after ICCN", ts.localTID, localSID))
			continue
		}

		ts.sessions = append(ts.sessions, sessionState{
			localSID:  localSID,
			remoteSID: remoteSID,
			up:        true,
		})
	}

	return errs
}

func (s *lacSimulator) teardownTunnel(conn *net.UDPConn, ts *tunnelState) {
	var buf [1500]byte
	var recvBuf [1500]byte

	for _, sess := range ts.sessions {
		if !sess.up {
			continue
		}
		off := l2tp.ControlHeaderLen
		off += l2tp.WriteAVPUint16(buf[:], off, true, l2tp.AVPMessageType, uint16(l2tp.MsgCDN))
		off += l2tp.WriteAVPUint16(buf[:], off, true, l2tp.AVPAssignedSessionID, sess.localSID)
		resultCode := []byte{0, 1, 0, 0}
		off += l2tp.WriteAVPBytes(buf[:], off, true, 0, l2tp.AVPResultCode, resultCode)
		l2tp.WriteControlHeader(buf[:], 0, uint16(off), ts.remoteTID, sess.remoteSID, ts.ns, ts.ns)
		ts.ns++

		if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
			continue
		}
		if _, err := conn.Write(buf[:off]); err != nil {
			continue
		}
		drainZLB(conn, recvBuf[:])
	}

	off := l2tp.ControlHeaderLen
	off += l2tp.WriteAVPUint16(buf[:], off, true, l2tp.AVPMessageType, uint16(l2tp.MsgStopCCN))
	off += l2tp.WriteAVPUint16(buf[:], off, true, l2tp.AVPAssignedTunnelID, ts.localTID)
	resultCode := []byte{0, 1, 0, 0}
	off += l2tp.WriteAVPBytes(buf[:], off, true, 0, l2tp.AVPResultCode, resultCode)
	l2tp.WriteControlHeader(buf[:], 0, uint16(off), ts.remoteTID, 0, ts.ns, ts.ns)

	if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return
	}
	if _, err := conn.Write(buf[:off]); err != nil {
		fmt.Fprintf(os.Stderr, "tunnel %d: send StopCCN: %v\n", ts.localTID, err)
	}
}

func drainZLB(conn *net.UDPConn, buf []byte) l2tp.MessageType {
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		return 0
	}
	n, err := conn.Read(buf)
	if err != nil || n < l2tp.ControlHeaderLen {
		return 0
	}
	hdr, err := l2tp.ParseMessageHeader(buf[:n])
	if err != nil {
		return 0
	}
	if int(hdr.Length) <= hdr.PayloadOff {
		return 0
	}
	it := l2tp.NewAVPIterator(buf[hdr.PayloadOff:int(hdr.Length)])
	_, aType, _, value, ok := it.Next()
	if ok && aType == l2tp.AVPMessageType && len(value) == 2 {
		return l2tp.MessageType(binary.BigEndian.Uint16(value))
	}
	return 0
}
