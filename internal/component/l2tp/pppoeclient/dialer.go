// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- PPPoE client discovery dialer
// Related: session.go -- LCP/auth/NCP negotiation invoked after discovery

package pppoeclient

import (
	"crypto/rand"
	"errors"
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

	chanFD, unitFD, unitNum, err := ppp.DevPPPSetup(pppoxFD)
	if err != nil {
		pppoe.ClosePPPoxFD(pppoxFD)
		sendPADT(discFD, ifindex, hwaddr, acMAC, sessID, nil)
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	chanFile := ppp.NewFDFile(chanFD, "pppoe-client.chan")
	link := &sessionLink{
		channel: chanFile,
		stopped: make(chan struct{}),
		closeTransport: func() {
			pppoe.ClosePPPoxFD(pppoxFD)
		},
	}
	frames := startReader(link, link.stopped)
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
			// Closing the PPP transport MUST precede joining its reader:
			// /dev/ppp can retain a blocking read until the channel detaches.
			for range frames {
			}
			// The caller may still be configuring ppp<unitNum> after Done
			// closes. Reserve its identity until that owner invokes Cleanup.
			pppoe.ClosePPPoxFD(unitFD)
		})
	}

	mtu := uint16(cfg.MTU)
	if mtu == 0 {
		mtu = 1492
	}
	sessCfg := sessionConfig{
		mtu:      mtu,
		username: cfg.Username,
		password: cfg.AuthSecret,
		chanFD:   chanFD,
	}
	result, negErr := negotiateSession(link, frames, unitFD, unitNum, sessCfg, link.stopped, logger)
	if negErr != nil {
		cleanup()
		return iface.PPPoESession{}, negErr
	}

	keepaliveDone := make(chan struct{})
	go func() {
		keepaliveLoop(link, result.frames, result.magic, keepaliveDone, link.stopped, logger)
		_ = link.Close() //nolint:errcheck // shutdown after PPP termination or read failure
	}()

	return iface.PPPoESession{
		SessionID: sessID,
		UnitNum:   unitNum,
		LocalIP:   result.localIP,
		PeerIP:    result.peerIP,
		NegMTU:    result.negMTU,
		Done:      link.stopped,
		Cleanup: func() {
			cleanup()
			<-keepaliveDone
		},
	}, nil
}

// sessionLink owns a client's PPP channel and kernel transport. Safe for
// concurrent Write and Close. The owner MUST close it before sending PADT,
// then wait for watchPADT before closing the discovery descriptor.
type sessionLink struct {
	channel        io.ReadWriteCloser
	closeTransport func()
	stopped        chan struct{}
	mu             sync.Mutex
	closed         bool
	closeErr       error
}

func (s *sessionLink) Read(buf []byte) (int, error) {
	return s.channel.Read(buf)
}

func (s *sessionLink) Write(buf []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, io.ErrClosedPipe
	}
	return s.channel.Write(buf)
}

// Close MUST precede outbound PADT and discovery-reader shutdown. It releases
// the transport and blocks subsequent PPP writes before publishing Done.
// Dial retains the PPP unit reservation until the owner invokes Cleanup.
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
