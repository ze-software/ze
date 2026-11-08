// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- shared iterative phase owner.
// Related: session.go -- phase packet handlers; connected_session.go -- restart owner.
// RFC 1661 Sections 3.4-3.6 -- see rfc/short/rfc1661.md.

package pppoeclient

import (
	"errors"
	"io"
	"log/slog"
	"net/netip"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// negotiatedSession is one complete result, with no descriptor ownership.
// Only newNegotiatedSession produces a valid value.
// Its zero value is an unsuccessful result and MUST NOT be published.
// The owning worker alone can mutate its CHAP state.
type negotiatedSession struct {
	localIP netip.Addr
	peerIP  netip.Addr
	// primaryDNS and secondaryDNS are invalid when IPCP learned no server.
	primaryDNS   netip.Addr
	secondaryDNS netip.Addr
	negMTU       uint16
	magic        uint32
	peerMRU      uint16
	chap         *networkPhaseCHAP
}

// negotiatePhases returns only after both directions of IPCP reach Opened.
// ipv6 is nil when no ipv6 is configured; otherwise IPv6CP starts beside IPCP
// on entering the network phase (negotiateIPCP), and the caller applies the
// state it reached once this returns.
// The caller MUST retain all identifiers across calls and be the sole reader
// consumer. No descriptor is created or closed.
// RFC 1661 Section 3.4: "The receipt of the LCP Configure-Request causes a return
// to the Link Establishment phase from the Network-Layer Protocol phase or
// Authentication phase."
func negotiatePhases(transport io.ReadWriteCloser, reader *frameReader,
	cfg sessionConfig, magic uint32, pending *lcpReentry, identifier *uint8,
	stop <-chan struct{}, logger *slog.Logger, lcpIdentifier, papIdentifier *uint8, codeRejectID *uint8,
	ipv6 *clientIPv6CP) (negotiatedSession, error) {
	if err := reader.validate(); err != nil {
		return negotiatedSession{}, err
	}
	var buf [ppp.MaxFrameBufLen]byte
	// Use RFC 1661 Section 4.6's default Max-Configure budget for convergence.
	// This counts epochs, not RFC retransmissions. Each epoch also has phase
	// deadlines, so reentry cannot extend an incomplete attempt indefinitely.
	const epochsMax = 10
	for range epochsMax {
		freshMagic, err := generateDifferentMagic(magic)
		if err != nil {
			return negotiatedSession{}, err
		}
		// RFC 1661 Sections 3.4 and 4.1: scr precedes the pending sca/scn.
		lcp, err := negotiateLCP(transport, reader, buf[:], cfg, freshMagic, pending, stop, logger, lcpIdentifier)
		if err != nil {
			return negotiatedSession{}, err
		}
		pending = nil
		magic = lcp.localMagic
		logger.Info("pppoe-client: LCP opened", "peer-mru", lcp.peerMRU, "auth-proto", lcp.authProto)
		if lcp.authProto != 0 {
			// RFC 1661 Section 3.5: authentication follows every LCP opening.
			if err := runClientAuth(transport, reader, buf[:], lcp, cfg, magic, stop, logger, papIdentifier); err != nil {
				if errors.As(err, &pending) {
					continue
				}
				return negotiatedSession{}, err
			}
		}
		// RFC 1994 Sections 4.1 and 4.2: each epoch owns fresh CHAP correlation.
		chap := networkPhaseCHAPFor(lcp.authProto, cfg)
		// RFC 1332 Section 2: run both IPCP directions on the existing sources,
		// and IPv6CP beside them when it is configured (RFC 1661 Section 3.6).
		ipcp, err := negotiateIPCP(transport, reader, buf[:], magic, lcp.peerMRU, chap, identifier, stop, logger, codeRejectID, ipv6)
		if err != nil {
			if errors.As(err, &pending) {
				if ipv6 != nil {
					// RFC 1661 Section 3.4: the return to Link Establishment
					// takes IPv6CP down too. It was never applied to the
					// unit, so its event changes nothing; the next epoch's
					// negotiateIPCP starts it again.
					ipv6.lowerDown()
				}
				continue
			}
			return negotiatedSession{}, err
		}
		logger.Info("pppoe-client: IPCP opened", "local-ip", ipcp.localIP, "peer-ip", ipcp.peerIP,
			"primary-dns", ipcp.primaryDNS, "secondary-dns", ipcp.secondaryDNS)
		negMTU := lcp.peerMRU
		if negMTU == 0 {
			negMTU = ppp.MaxFrameLen
		}
		return newNegotiatedSession(sessionResult{
			localIP: ipcp.localIP, peerIP: ipcp.peerIP, negMTU: min(negMTU, cfg.mtu),
			primaryDNS: ipcp.primaryDNS, secondaryDNS: ipcp.secondaryDNS,
			magic: magic, peerMRU: lcp.peerMRU, chap: chap,
		}, cfg)
	}
	return negotiatedSession{}, errors.New("pppoe-client: LCP restart limit reached before IPCP opened; check the access concentrator's PPP configuration")
}
