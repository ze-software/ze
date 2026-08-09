// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- PPPoE client discovery dialer
// Related: session.go -- LCP/auth/NCP negotiation invoked after discovery

package pppoeclient

import (
	"crypto/rand"
	"errors"
	"log/slog"
	"net"
	"runtime"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

const discoveryTimeout = 10 * time.Second

var errDiscoveryTimeout = errors.New("pppoeclient: discovery timeout")

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
		sendPADT(discFD, ifindex, hwaddr, acMAC, sessID)
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	chanFD, unitFD, unitNum, err := ppp.DevPPPSetup(pppoxFD)
	if err != nil {
		pppoe.ClosePPPoxFD(pppoxFD)
		sendPADT(discFD, ifindex, hwaddr, acMAC, sessID)
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, err
	}

	chanFile := ppp.NewFDFile(chanFD, "pppoe-client.chan")

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
	result, negErr := negotiateSession(chanFile, unitFD, unitNum, sessCfg, stopCh, logger)
	if negErr != nil {
		chanFile.Close() //nolint:errcheck // rollback; primary error is negErr
		pppoe.ClosePPPoxFD(unitFD)
		pppoe.ClosePPPoxFD(pppoxFD)
		sendPADT(discFD, ifindex, hwaddr, acMAC, sessID)
		pppoe.CloseDiscoveryFD(discFD)
		return iface.PPPoESession{}, negErr
	}

	doneCh := make(chan struct{})
	keepaliveStop := make(chan struct{})
	go keepaliveLoop(chanFile, result.frames, result.magic, doneCh, keepaliveStop, logger)

	return iface.PPPoESession{
		SessionID: sessID,
		UnitNum:   unitNum,
		LocalIP:   result.localIP,
		PeerIP:    result.peerIP,
		NegMTU:    result.negMTU,
		Done:      doneCh,
		Cleanup: func() {
			close(keepaliveStop)
			<-doneCh
			chanFile.Close() //nolint:errcheck // shutdown cleanup
			pppoe.ClosePPPoxFD(unitFD)
			sendPADT(discFD, ifindex, hwaddr, acMAC, sessID)
			pppoe.ClosePPPoxFD(pppoxFD)
			pppoe.CloseDiscoveryFD(discFD)
		},
	}, nil
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
			// SO_RCVTIMEO makes this return after ~100ms if no frame arrives.
			pkt, ok := tryReadPADO(discFD, ifindex, hostUniq, wantACName)
			if ok {
				return pkt, nil
			}
			runtime.Gosched()
		}
	}
}

func tryReadPADO(discFD, ifindex int, hostUniq [4]byte, wantACName string) (pppoe.Packet, bool) {
	var rxBuf [pppoe.EthMaxLen]byte
	n, rxIfindex, rxErr := pppoe.ReadDiscoveryFrame(discFD, rxBuf[:])
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
			// SO_RCVTIMEO makes this return after ~100ms if no frame arrives.
			sid, err := tryReadPADS(discFD, ifindex, acMAC)
			if err != nil {
				return 0, err
			}
			if sid != 0 {
				return sid, nil
			}
			runtime.Gosched()
		}
	}
}

func tryReadPADS(discFD, ifindex int, acMAC [pppoe.EthALen]byte) (uint16, error) {
	var rxBuf [pppoe.EthMaxLen]byte
	n, rxIfindex, rxErr := pppoe.ReadDiscoveryFrame(discFD, rxBuf[:])
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

// sendPADT sends a session termination frame (RFC 2516 Section 5.5).
func sendPADT(discFD, ifindex int, srcMAC, dstMAC [pppoe.EthALen]byte, sid uint16) {
	var buf [pppoe.EthMaxLen]byte
	frame := pppoe.BuildPADT(buf[:], srcMAC, dstMAC, sid, "ze")
	if frame != nil {
		_ = pppoe.SendDiscoveryFrame(discFD, ifindex, frame)
	}
}
