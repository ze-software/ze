// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- PPPoE client discovery dialer
// Related: session.go -- LCP/auth/NCP negotiation invoked after discovery
// RFC: rfc/short/rfc2516.md

package pppoeclient

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

const discoveryTimeout = 10 * time.Second

var errDiscoveryTimeout = errors.New("pppoeclient: discovery timeout")

// sessionMTUMax is the largest MRU RFC 2516 lets LCP negotiate, and the MTU a
// session runs with when its config names none. The iface config refuses a
// PPPoE client MTU outside sessionMTUMin..sessionMTUMax too: the two checks
// are a pair, so a caller that skips the config still cannot pass 1500.
const (
	sessionMTUMin = 68
	sessionMTUMax = 1492
)

var errSessionMTURange = errors.New("pppoeclient: session mtu out of range 68..1492")

// RFC 2516 Section 7: "The Maximum-Receive-Unit (MRU) option MUST NOT be
// negotiated to a larger size than 1492."
//
// pppoeSessionMTU answers the MTU a client session negotiates LCP with: 1492
// when configured is zero (no MTU configured), configured itself inside
// 68..1492, and errSessionMTURange otherwise. negotiateLCP proposes this
// value as the client's MRU and caps the Access Concentrator's MRU by it.
func pppoeSessionMTU(configured int) (uint16, error) {
	if configured == 0 {
		return sessionMTUMax, nil
	}
	if configured < sessionMTUMin {
		return 0, fmt.Errorf("%w: %d", errSessionMTURange, configured)
	}
	if configured > sessionMTUMax {
		return 0, fmt.Errorf("%w: %d", errSessionMTURange, configured)
	}
	return uint16(configured), nil
}

// readDiscoveryFrame reads one discovery frame. A package variable so a
// test can substitute a fake without opening a real AF_PACKET socket.
// SetRecvTimeout (called by Dial before either wait loop below ever runs)
// puts SO_RCVTIMEO on the real socket, so the production implementation
// blocks for that timeout when no frame is waiting: that is what paces
// waitForPADO and waitForPADS. Neither loop adds a wait of its own.
var readDiscoveryFrame = pppoe.ReadDiscoveryFrame

// sendDiscoveryFrame shares the discovery socket's send path with lifecycle
// tests that observe whether PPP has stopped at the PADT publication boundary.
var sendDiscoveryFrame = pppoe.SendDiscoveryFrame

// Dialer implements iface.PPPoEDialer using the pppoe discovery wire
// format and ppp /dev/ppp setup.
type Dialer struct{}

// Dial performs PPPoE discovery (RFC 2516 Section 5) and kernel session
// setup. Returns a PPPoESession with open file descriptors. The caller
// must invoke Cleanup when the session is no longer needed.
func (d *Dialer) Dial(cfg iface.PPPoEClientConfig, stopCh <-chan struct{}, logger *slog.Logger) (iface.PPPoESession, error) {
	// RFC 2516 Section 7: the session MTU bounds the MRU LCP negotiates.
	mtu, err := pppoeSessionMTU(cfg.MTU)
	if err != nil {
		return iface.PPPoESession{}, err
	}
	// The iface parse validates the keepalive and reconnect leaves; this is
	// the second check of the pair, before any socket opens.
	if err := cfg.Timing.Validate(); err != nil {
		return iface.PPPoESession{}, err
	}

	ifindex, hwaddr, _, err := pppoe.ResolveInterface(cfg.SourceInterface)
	if err != nil {
		return iface.PPPoESession{}, err
	}

	discFD, err := pppoe.OpenDiscoverySocket()
	if err != nil {
		return iface.PPPoESession{}, err
	}
	if err := pppoe.SetRecvTimeout(discFD, 100*time.Millisecond); err != nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	// RFC 2516 Section 5.1: Generate Host-Uniq for correlation.
	var hostUniq [4]byte
	if _, err := rand.Read(hostUniq[:]); err != nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	// RFC 2516 Section 5.1: Send PADI to broadcast.
	var buf [pppoe.EthMaxLen]byte
	frame := pppoe.BuildPADI(buf[:], hwaddr, cfg.ServiceName, hostUniq[:])
	if frame == nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, errors.New("pppoeclient: PADI frame too large")
	}
	if err := pppoe.SendDiscoveryFrame(discFD, ifindex, frame); err != nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	// RFC 2516 Section 5.2: Wait for PADO.
	padoPkt, err := waitForPADO(discFD, ifindex, hostUniq, cfg.ACName, stopCh)
	if err != nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}
	var acMAC [pppoe.EthALen]byte
	copy(acMAC[:], padoPkt.SrcMAC[:])

	// RFC 2516 Section 5.3: Send PADR to the selected AC.
	frame = pppoe.BuildPADR(buf[:], hwaddr, &padoPkt, cfg.ServiceName, hostUniq[:])
	if frame == nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, errors.New("pppoeclient: PADR frame too large")
	}
	if err := pppoe.SendDiscoveryFrame(discFD, ifindex, frame); err != nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	// RFC 2516 Section 5.4: Wait for PADS.
	sessID, err := waitForPADS(discFD, ifindex, acMAC, stopCh)
	if err != nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	logger.Info("pppoeclient: discovery complete",
		"session-id", sessID, "ac-mac", net.HardwareAddr(acMAC[:]).String())

	// Kernel PPPoE session + /dev/ppp setup.
	// RFC 2516 Section 4: after PADS, the kernel handles session framing.
	pppoxFD, err := pppoe.PPPoECreate(cfg.SourceInterface, sessID, acMAC)
	if err != nil {
		sendPADT(discFD, ifindex, hwaddr, acMAC, sessID, nil)
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	chanFD, unitFD, unitNum, err := ppp.DevPPPSetup(pppoxFD, cfg.Name)
	if err != nil {
		pppoe.ClosePPPoxFD(pppoxFD)
		sendPADT(discFD, ifindex, hwaddr, acMAC, sessID, nil)
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	chanFile := ppp.NewFDFile(chanFD, "pppoe-client.chan")
	link := &sessionLink{
		channel:   chanFile,
		channelFD: chanFD,
		unitFD:    unitFD,
		stopped:   make(chan struct{}),
		closeTransport: func() {
			pppoe.ClosePPPoxFD(pppoxFD)
		},
	}
	// NewFDFile sets blocking mode. Prepare raw nonblocking reads only after
	// creating that wrapper, and never wrap either descriptor again afterward.
	if err := prepareClientDescriptors(chanFD, unitFD); err != nil {
		_ = link.Close() //nolint:errcheck // preparation failed; release the transport once
		pppoe.ClosePPPoxFD(unitFD)
		sendPADT(discFD, ifindex, hwaddr, acMAC, sessID, link)
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}
	reader, err := newFrameReader(link.readPacket, link.stopped)
	if err != nil {
		_ = link.Close() //nolint:errcheck // receive startup failed; release the transport once
		pppoe.ClosePPPoxFD(unitFD)
		sendPADT(discFD, ifindex, hwaddr, acMAC, sessID, link)
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}
	discoveryDone := make(chan struct{})
	go watchPADT(discFD, ifindex, hwaddr, acMAC, sessID, link, stopCh, discoveryDone, logger)
	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			_ = link.Close() //nolint:errcheck // shutdown; descriptors are released once
			// The reader MUST exit before its discovery fd can be closed or reused.
			<-discoveryDone
			sendPADT(discFD, ifindex, hwaddr, acMAC, sessID, link)
			pppoe.CloseDiscoveryFD(discFD)
			// Close cancels both read requests and result delivery. The raw
			// read observes closure under the descriptor lifecycle lock.
			<-reader.done
			// The caller may still be configuring ppp<unitNum> after Done
			// closes. Reserve its identity until that owner invokes Cleanup.
			pppoe.ClosePPPoxFD(unitFD)
		})
	}

	sessCfg := sessionConfig{
		name:              cfg.Name,
		mtu:               mtu,
		username:          cfg.Username,
		password:          cfg.AuthSecret,
		chanFD:            chanFD,
		keepaliveInterval: cfg.Timing.KeepaliveInterval,
		keepaliveFailures: cfg.Timing.KeepaliveFailures,
	}
	result, negErr := negotiateSession(link, reader, unitFD, unitNum, sessCfg, link.stopped, logger)
	if negErr != nil {
		cleanup()
		return iface.PPPoESession{}, negErr
	}

	connected, err := newConnectedSession(link, result, sessCfg, func(mode ppp.NPMode) error {
		// RFC 1661 Section 3.6: the unit retains ownership while mode changes.
		return link.setIPv4Mode(unitFD, mode)
	})
	if err != nil {
		cleanup()
		return iface.PPPoESession{}, err
	}
	if cfg.IPv6.Enabled {
		// RFC 5072 Section 4.1: the tentative identifier comes from the
		// source interface's EUI-48 (ppp.TentativeIPv6CPInterfaceID).
		if err := connected.enableIPv6CP(hwaddr[:], func(mode ppp.NPMode) error {
			return link.setIPv6Mode(unitFD, mode)
		}); err != nil {
			cleanup()
			return iface.PPPoESession{}, err
		}
	}
	keepaliveDone := make(chan struct{})
	// Snapshot every initial value before the connected worker can run.
	session := iface.PPPoESession{
		SessionID: sessID,
		UnitNum:   unitNum,
		LocalIP:   result.localIP,
		PeerIP:    result.peerIP,
		NegMTU:    result.negMTU,
		// RFC 1877 Section 1: the servers IPCP learned, primary first.
		PrimaryDNS:   result.primaryDNS,
		SecondaryDNS: result.secondaryDNS,
		Changes:      connected.changes,
		Done:         link.stopped,
		Cleanup: func() {
			link.cleanupConnected(keepaliveDone, cleanup)
		},
	}
	go superviseNetworkPhase(link, connected, keepaliveDone, logger)
	return session, nil
}

// sessionLink owns a client's PPP channel and kernel transport. Safe for
// concurrent Write and Close. The owner MUST close it before sending PADT,
// then wait for watchPADT before closing the discovery descriptor.
type sessionLink struct {
	channel        io.ReadWriteCloser
	channelFD      int
	unitFD         int
	unitConnected  bool // Protected by mu; an unconnected unit reads EOF.
	closeTransport func()
	stopped        chan struct{}
	mu             sync.Mutex
	closed         bool
	closeErr       error
}

func (s *sessionLink) Read(buf []byte) (int, error) {
	return s.channel.Read(buf)
}

// cleanupConnected joins the phase worker before receive and unit cleanup.
// The caller MUST supply the phase worker's done signal and the reader/unit cleanup.
// Cleanup MUST NOT release the unit before this join.
func (s *sessionLink) cleanupConnected(done <-chan struct{}, cleanup func()) {
	_ = s.Close() //nolint:errcheck // shutdown; cleanup retains descriptor release ownership
	<-done
	cleanup()
}

func (s *sessionLink) Write(buf []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, io.ErrClosedPipe
	}
	return s.channel.Write(buf)
}

// setIPv4Mode serializes the unit ioctl with transport closure. Cleanup MUST
// call Close before releasing unitFD, so a late call refuses a reused fd.
// RFC 1661 Section 3.6: "Any supported network-layer protocol packets received
// when the corresponding NCP is not in the Opened state MUST be silently discarded."
func (s *sessionLink) setIPv4Mode(unitFD int, mode ppp.NPMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	// RFC 1661 Section 3.6: apply the network-layer forwarding gate.
	return ppp.SetIPv4Mode(unitFD, mode)
}

// Close MUST precede outbound PADT and discovery-reader shutdown. It releases
// the transport and blocks subsequent PPP writes before publishing Done.
// Dial retains the PPP unit reservation until the owner invokes Cleanup.
// Cleanup MUST call Close before releasing the unit used by setIPv4Mode.
func (s *sessionLink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.closeErr
	}
	// RFC 2516 Section 5.5: "Even normal PPP termination packets MUST NOT
	// be sent after sending or receiving a PADT."
	s.closed = true
	s.closeErr = s.channel.Close()
	s.closeTransport()
	close(s.stopped)
	return s.closeErr
}

// superviseNetworkPhase is the session's one network-phase goroutine. It runs
// keepaliveLoop, which closes done when it returns, and then closes link. The
// close publishes Done and refuses every later PPP write, whatever ended the
// loop: an LCP Terminate-Request, an echo timeout, a read failure or the
// owner's stop.
//
// RFC 2516 Section 7: "When LCP terminates, the Host and Access concentrator
// MUST stop using that PPPoE session." The close is how the Host stops.
func superviseNetworkPhase(link *sessionLink, session *connectedSession, done chan<- struct{}, logger *slog.Logger) {
	keepaliveLoop(session, done, link.stopped, logger)
	_ = link.Close() //nolint:errcheck // shutdown after PPP termination or read failure
}

// watchPADT owns discovery reads from PADS until the session ends. Its caller
// MUST wait for done before closing the discovery fd. SO_RCVTIMEO bounds the
// stop latency; this loop lasts at most the lifetime of link.
func watchPADT(discFD, ifindex int, localMAC, peerMAC [pppoe.EthALen]byte, sid uint16, link *sessionLink, stopCh <-chan struct{}, done chan<- struct{}, logger *slog.Logger) {
	defer close(done)
	var buf [pppoe.EthMaxLen]byte
	for {
		select {
		case <-link.stopped:
			return
		case <-stopCh:
			_ = link.Close() //nolint:errcheck // shutdown requested by owner
			return
		default:
			n, rxIfindex, err := readDiscoveryFrame(discFD, buf[:])
			if err != nil {
				if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EINTR) {
					continue
				}
				logger.Warn("pppoe-client: discovery read failed", "error", err)
				_ = link.Close() //nolint:errcheck // failed discovery socket ends session monitoring
				return
			}
			if rxIfindex != ifindex {
				continue
			}
			pkt, err := pppoe.ParseDiscovery(buf[:n])
			if err != nil {
				continue
			}
			if pkt.Code != pppoe.CodePADT {
				continue
			}
			// RFC 2516 Section 4: "It's value is fixed for a given PPP
			// session and, in fact, defines a PPP session along with the
			// Ethernet SOURCE_ADDR and DESTINATION_ADDR."
			if pkt.SID != sid || pkt.SrcMAC != peerMAC || pkt.DstMAC != localMAC {
				continue
			}
			_ = link.Close() //nolint:errcheck // PADT ends PPP even when descriptor close reports an error
			return
		}
	}
}

func waitForPADO(discFD, ifindex int, hostUniq [4]byte, wantACName string, stopCh <-chan struct{}) (pppoe.Packet, error) {
	deadline := time.NewTimer(discoveryTimeout)
	defer deadline.Stop()

	for {
		select {
		case <-stopCh:
			return pppoe.Packet{}, errors.New("pppoeclient: stopped during discovery")
		case <-deadline.C:
			return pppoe.Packet{}, errDiscoveryTimeout
		default:
			// SO_RCVTIMEO paces this call at ~100ms per attempt when no
			// frame arrives, so this arm blocks on the socket rather than
			// spinning: no separate yield is needed, and none is added.
			pkt, ok := tryReadPADO(discFD, ifindex, hostUniq, wantACName)
			if ok {
				return pkt, nil
			}
		}
	}
}

func tryReadPADO(discFD, ifindex int, hostUniq [4]byte, wantACName string) (pppoe.Packet, bool) {
	var rxBuf [pppoe.EthMaxLen]byte
	n, rxIfindex, rxErr := readDiscoveryFrame(discFD, rxBuf[:])
	if rxErr != nil || rxIfindex != ifindex {
		return pppoe.Packet{}, false
	}

	pkt, pErr := pppoe.ParseDiscovery(rxBuf[:n])
	if pErr != nil || pkt.Code != pppoe.CodePADO || pkt.SID != 0 {
		return pppoe.Packet{}, false
	}

	// RFC 2516 Section 5.1: verify Host-Uniq echoed unchanged.
	hu := pkt.FindTag(pppoe.TagHostUniq)
	if hu == nil || len(hu.Value) != len(hostUniq) {
		return pppoe.Packet{}, false
	}
	for i := range hostUniq {
		if hu.Value[i] != hostUniq[i] {
			return pppoe.Packet{}, false
		}
	}

	if wantACName != "" {
		acTag := pkt.FindTag(pppoe.TagACName)
		if acTag == nil || string(acTag.Value) != wantACName {
			return pppoe.Packet{}, false
		}
	}

	return pkt, true
}

func waitForPADS(discFD, ifindex int, acMAC [pppoe.EthALen]byte, stopCh <-chan struct{}) (uint16, error) {
	deadline := time.NewTimer(discoveryTimeout)
	defer deadline.Stop()

	for {
		select {
		case <-stopCh:
			return 0, errors.New("pppoeclient: stopped during discovery")
		case <-deadline.C:
			return 0, errDiscoveryTimeout
		default:
			// SO_RCVTIMEO paces this call at ~100ms per attempt when no
			// frame arrives, so this arm blocks on the socket rather than
			// spinning: no separate yield is needed, and none is added.
			sid, err := tryReadPADS(discFD, ifindex, acMAC)
			if err != nil {
				return 0, err
			}
			if sid != 0 {
				return sid, nil
			}
		}
	}
}

func tryReadPADS(discFD, ifindex int, acMAC [pppoe.EthALen]byte) (uint16, error) {
	var rxBuf [pppoe.EthMaxLen]byte
	n, rxIfindex, rxErr := readDiscoveryFrame(discFD, rxBuf[:])
	if rxErr != nil {
		return 0, nil //nolint:nilerr // read timeout or transient error; caller retries
	}
	if rxIfindex != ifindex {
		return 0, nil
	}

	pkt, pErr := pppoe.ParseDiscovery(rxBuf[:n])
	if pErr != nil {
		return 0, nil //nolint:nilerr // malformed frame; caller retries
	}
	if pkt.Code != pppoe.CodePADS || pkt.SrcMAC != acMAC {
		return 0, nil
	}

	// RFC 2516 Section 5.4: PADS with SID=0 is an error response.
	if pkt.SID == 0 {
		errTag := pkt.FindTag(pppoe.TagSvcNameError)
		if errTag == nil {
			errTag = pkt.FindTag(pppoe.TagACSystemError)
		}
		if errTag == nil {
			errTag = pkt.FindTag(pppoe.TagGenericError)
		}
		msg := "unknown error"
		if errTag != nil && len(errTag.Value) > 0 {
			msg = string(errTag.Value)
		}
		return 0, errors.New("pppoeclient: PADS error: " + msg)
	}

	return pkt.SID, nil
}

// sendPADT stops PPP before sending the session termination frame. link is nil
// only when kernel setup failed before a PPP channel could be created.
func sendPADT(discFD, ifindex int, srcMAC, dstMAC [pppoe.EthALen]byte, sid uint16, link *sessionLink) {
	// RFC 2516 Section 5.5: "Even normal PPP termination packets MUST NOT
	// be sent after sending or receiving a PADT."
	if link != nil {
		_ = link.Close() //nolint:errcheck // teardown still requires PADT after a close error
	}
	var buf [pppoe.EthMaxLen]byte
	frame := pppoe.BuildPADT(buf[:], srcMAC, dstMAC, sid, "ze")
	if frame != nil {
		_ = sendDiscoveryFrame(discFD, ifindex, frame)
	}
}
