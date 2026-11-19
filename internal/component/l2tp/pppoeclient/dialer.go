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
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
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

// resolveSourceInterface, openDiscoverySocket, setDiscoveryTimeout and
// createSessionSocket are the remaining kernel calls Dial makes before PPP
// runs. Tests replace them to drive discovery to the session socket without
// a NIC, and to read which kernel device that socket is bound to.
var (
	resolveSourceInterface = pppoe.ResolveInterface
	openDiscoverySocket    = pppoe.OpenDiscoverySocket
	setDiscoveryTimeout    = pppoe.SetRecvTimeout
	createSessionSocket    = pppoe.PPPoECreate
)

// Dialer implements iface.PPPoEDialer using the pppoe discovery wire
// format and ppp /dev/ppp setup.
type Dialer struct{ backendFactory func() (iface.Backend, error) }

// Dial performs PPPoE discovery (RFC 2516 Section 5) and kernel session
// setup. Returns a PPPoESession with open file descriptors. The caller
// must invoke Cleanup when the session is no longer needed.
func (d *Dialer) Dial(cfg iface.PPPoEClientConfig, stopCh <-chan struct{}, logger *slog.Logger) (iface.PPPoESession, error) { //nolint:gocritic // hugeParam: iface.PPPoEDialer fixes the value signature; one copy per dial, off the data plane
	prepare := prepareClientDial
	if d.backendFactory != nil {
		prepare = func() (*clientDialPreparation, error) { return prepareClientDialFrom(d.backendFactory) }
	}
	return d.dialPrepared(cfg, stopCh, logger, prepare)
}

// dialPrepared keeps the private logical preflight port separate from public
// original-file grants. Production supplies only prepareClientDial.
func (d *Dialer) dialPrepared(cfg iface.PPPoEClientConfig, stopCh <-chan struct{}, logger *slog.Logger, prepare func() (*clientDialPreparation, error)) (session iface.PPPoESession, err error) { //nolint:gocritic // hugeParam: same owned snapshot as Dial.
	// RFC 2516 Section 7: the session MTU bounds the MRU LCP negotiates.
	mtu, err := pppoeSessionMTU(cfg.MTU)
	if err != nil {
		return iface.PPPoESession{}, err
	}
	// The iface parse validates the LCP, IPv6CP, keepalive and reconnect leaves; this is
	// the second check of the pair, before any socket opens.
	if err := cfg.Timing.Validate(); err != nil {
		return iface.PPPoESession{}, err
	}

	// One resolution names the NIC for every kernel call below: discovery
	// sends and filters on its ifindex, PADT leaves from it, and the session
	// socket connects to its kernel device. The configured name is a logical
	// key (os-name, mac match) and never reaches the kernel.
	source, err := resolveSourceInterface(cfg.SourceInterface)
	if err != nil {
		return iface.PPPoESession{}, err
	}
	// Capture the independent backend and its creator before discovery or any
	// PPPoX allocation. The active backend is never looked up during this dial.
	prepared, err := prepare()
	if err != nil {
		if prepared != nil {
			closeErr := prepared.close()
			err = errors.Join(err, closeErr)
			if errors.Is(closeErr, iface.ErrNativePPPPending) {
				session.Cleanup = prepared.close
				if prepared.nativeWriter != nil {
					session.NativeWriter = prepared.nativeWriter()
				}
			}
		}
		return session, err
	}
	preparationTransferred := false
	defer func() {
		if !preparationTransferred {
			closeErr := prepared.close()
			err = errors.Join(err, closeErr)
			if errors.Is(closeErr, iface.ErrNativePPPPending) {
				session.Cleanup = prepared.close
				if prepared.nativeWriter != nil {
					session.NativeWriter = prepared.nativeWriter()
				}
			}
		}
	}()

	discFD, err := openDiscoverySocket()
	if err != nil {
		return iface.PPPoESession{}, err
	}
	if err := setDiscoveryTimeout(discFD, 100*time.Millisecond); err != nil {
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
	frame := pppoe.BuildPADI(buf[:], source.HWAddr, cfg.ServiceName, hostUniq[:])
	if frame == nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, errors.New("pppoeclient: PADI frame too large")
	}
	if err := sendDiscoveryFrame(discFD, source.Ifindex, frame); err != nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	// RFC 2516 Section 5.2: Wait for PADO.
	padoPkt, err := waitForPADO(discFD, source.Ifindex, hostUniq, cfg.ACName, stopCh)
	if err != nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}
	var acMAC [pppoe.EthALen]byte
	copy(acMAC[:], padoPkt.SrcMAC[:])

	// RFC 2516 Section 5.3: Send PADR to the selected AC.
	frame = pppoe.BuildPADR(buf[:], source.HWAddr, &padoPkt, cfg.ServiceName, hostUniq[:])
	if frame == nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, errors.New("pppoeclient: PADR frame too large")
	}
	if err := sendDiscoveryFrame(discFD, source.Ifindex, frame); err != nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	// RFC 2516 Section 5.4: Wait for PADS.
	sessID, err := waitForPADS(discFD, source.Ifindex, acMAC, stopCh)
	if err != nil {
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	logger.Info("pppoeclient: discovery complete",
		"session-id", sessID, "ac-mac", net.HardwareAddr(acMAC[:]).String())

	// Kernel PPPoE session + /dev/ppp setup.
	// RFC 2516 Section 4: after PADS, the kernel handles session framing.
	pppoxFile, err := createSessionSocket(source, sessID, acMAC)
	if err != nil {
		if pppoxFile != nil {
			err = errors.Join(err, closeClientOwner(pppoxFile.Close))
		}
		sendPADT(discFD, source.Ifindex, source.HWAddr, acMAC, sessID, nil)
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	client, err := prepared.setup(pppoxFile, cfg.Name)
	if err != nil {
		err = errors.Join(err, closeClientOwner(pppoxFile.Close))
		sendPADT(discFD, source.Ifindex, source.HWAddr, acMAC, sessID, nil)
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	link := &sessionLink{
		client:         client,
		stopped:        make(chan struct{}),
		closeTransport: pppoxFile.Close,
	}
	unitNum := client.UnitNumber()
	reader, err := newFrameReader(link.readPacket, link.stopped)
	if err != nil {
		err = errors.Join(err, link.Close())
		sendPADT(discFD, source.Ifindex, source.HWAddr, acMAC, sessID, link)
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}
	discoveryDone := make(chan struct{})
	go watchPADT(discFD, source.Ifindex, source.HWAddr, acMAC, sessID, link, stopCh, discoveryDone, logger)
	// ipv6 remains nil when IPv6 is disabled. Cleanup MUST stop its timer even
	// when negotiation fails before a connected worker takes over.
	var ipv6 *clientIPv6CP
	var cleanupOnce sync.Once
	cleanup := func() error {
		cleanupOnce.Do(func() {
			if ipv6 != nil {
				ipv6.restart.Stop()
			}
			_ = link.Close() //nolint:errcheck // shutdown; descriptors are released once
			// The reader MUST exit before its discovery fd can be closed or reused.
			<-discoveryDone
			sendPADT(discFD, source.Ifindex, source.HWAddr, acMAC, sessID, link)
			pppoe.CloseDiscoveryFD(discFD)
			// Close cancels both read requests and result delivery. The raw
			// read observes closure under the descriptor lifecycle lock.
			<-reader.done
		})
		// Every protocol/read owner has joined. Pending native settlement remains
		// retryable outside Once, after physical original-unit retirement.
		return errors.Join(link.Close(), prepared.close())
	}

	// From here one closure owns every partial outcome, including failures.
	defer func() {
		if err == nil {
			return
		}
		closeErr := cleanup()
		err = errors.Join(err, closeErr)
		if errors.Is(closeErr, iface.ErrNativePPPPending) {
			session = iface.PPPoESession{Cleanup: cleanup}
			if prepared.nativeWriter != nil {
				session.NativeWriter = prepared.nativeWriter()
			}
		}
	}()
	preparationTransferred = true
	sessCfg := sessionConfig{
		name:              cfg.Name,
		mtu:               mtu,
		username:          cfg.Username,
		password:          cfg.AuthSecret,
		keepaliveInterval: cfg.Timing.KeepaliveInterval,
		keepaliveFailures: cfg.Timing.KeepaliveFailures,
		lcpRestartTimer:   cfg.Timing.LCPRestartTimer,
		lcpMaxConfigure:   cfg.Timing.LCPMaxConfigure,
		lcpMaxFailure:     cfg.Timing.LCPMaxFailure,
	}
	// ipv6 stays nil, the guard negotiateIPCP and keepaliveLoop read as "no
	// IPv6CP", when the pppoe-client configures no ipv6.
	if cfg.IPv6.Enabled {
		// RFC 1661 Section 3.6: "Any supported network-layer protocol packets
		// received when the corresponding NCP is not in the Opened state MUST
		// be silently discarded." IPv6CP negotiates during IPCP's
		// negotiation, so IPv6 is held from before the first NCP request.
		if err := link.setIPv6Mode(pppkernel.NPModeDrop); err != nil {
			return iface.PPPoESession{}, fmt.Errorf("pppoe-client: cannot hold IPv6 until IPv6CP opens: %w", err)
		}
		// RFC 5072 Section 4.1: the tentative identifier comes from the
		// source interface's EUI-48 (ppp.TentativeIPv6CPInterfaceID).
		ipv6, err = newClientIPv6CP(source.HWAddr[:], 0, cfg.Timing.IPv6CPRestartTimer)
		if err != nil {
			return iface.PPPoESession{}, err
		}
	}
	result, negErr := negotiateSession(link, reader, sessCfg, link.stopped, logger, ipv6)
	if negErr != nil {
		return iface.PPPoESession{}, negErr
	}

	connected, err := newConnectedSession(link, result, sessCfg, func(mode pppkernel.NPMode) error {
		// RFC 1661 Section 3.6: the unit retains ownership while mode changes.
		return link.setIPv4Mode(mode)
	})
	if err != nil {
		return iface.PPPoESession{}, err
	}
	if ipv6 != nil {
		if err := connected.enableIPv6CP(ipv6, func(mode pppkernel.NPMode) error {
			return link.setIPv6Mode(mode)
		}, logger); err != nil {
			return iface.PPPoESession{}, err
		}
	}
	keepaliveDone := make(chan struct{})
	// Private protocol fixtures can omit a native producer. Actual consumers
	// refuse that absence; no fixture acquires a forged kernel capability.
	var nativeWriter iface.NativePPPWriter
	if prepared.nativeWriter != nil {
		nativeWriter = prepared.nativeWriter()
	}
	// Snapshot every initial value before the connected worker can run.
	session = iface.PPPoESession{
		NativeWriter: nativeWriter,
		SessionID:    sessID,
		UnitNum:      unitNum,
		LocalIP:      result.localIP,
		PeerIP:       result.peerIP,
		NegMTU:       result.negMTU,
		// RFC 1877 Section 1: the servers IPCP learned, primary first.
		PrimaryDNS:   result.primaryDNS,
		SecondaryDNS: result.secondaryDNS,
		Changes:      connected.changes,
		Applied:      connected.acknowledge,
		Done:         link.stopped,
		Cleanup: func() error {
			return link.cleanupConnected(keepaliveDone, cleanup)
		},
	}
	go superviseNetworkPhase(link, connected, keepaliveDone, logger)
	preparationTransferred = true
	return session, nil
}

// sessionLink owns a client's PPP channel and kernel transport. Safe for
// concurrent Write and Close. The owner MUST close it before sending PADT,
// then wait for watchPADT before closing the discovery descriptor.
type sessionLink struct {
	client         clientProtocol
	closeTransport func() error
	stopped        chan struct{}
	mu             sync.Mutex
	closed         bool
	closeErr       error
}

func (s *sessionLink) Read(buf []byte) (int, error) {
	return s.readPacket(buf)
}

// cleanupConnected joins the phase worker before receive and unit cleanup.
// The caller MUST supply the phase worker's done signal and the reader/unit cleanup.
// Cleanup MUST NOT release the unit before this join. Every Close and owner
// cleanup cause is returned; pending owner cleanup remains retryable.
func (s *sessionLink) cleanupConnected(done <-chan struct{}, cleanup func() error) error {
	closeErr := s.Close()
	<-done
	return errors.Join(closeErr, cleanup())
}

func (s *sessionLink) Write(buf []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, io.ErrClosedPipe
	}
	return s.client.WritePacket(buf)
}

// setIPv4Mode serializes the unit ioctl with transport closure. Cleanup MUST
// call Close before releasing the original unit, so stopped calls refuse.
// RFC 1661 Section 3.6: "Any supported network-layer protocol packets received
// when the corresponding NCP is not in the Opened state MUST be silently discarded."
func (s *sessionLink) setIPv4Mode(mode pppkernel.NPMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	// RFC 1661 Section 3.6: apply the network-layer forwarding gate.
	return s.client.SetIPv4Mode(mode)
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
	s.closeErr = closeClientOwner(s.client.CloseChannel)
	s.closeErr = errors.Join(s.closeErr, closeClientOwner(s.closeTransport))
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
