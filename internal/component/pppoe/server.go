// Design: plan/learned/669-bng-5-pppoe.md -- per-interface PPPoE server
// Related: discovery.go -- ParseDiscovery, Build* frame constructors
// Related: cookie.go -- GenerateCookie, VerifyCookie
// Related: session.go -- SessionTable, Session
// Related: ratelimit.go -- PADILimiter

package pppoe

import (
	"bytes"
	"log/slog"
	"net"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
)

// InterfaceServer owns the PPPoE state for one access interface.
// The shared discovery reader goroutine dispatches parsed packets
// here by ifindex.
type InterfaceServer struct {
	ifName    string
	ifIndex   int
	hwAddr    [EthALen]byte
	mtu       int
	sessions  *SessionTable
	cookieKey CookieKey
	limiter   *PADILimiter

	cookieTimeout time.Duration
	acName        string
	serviceNames  []string

	discFD    int
	pppDriver *ppp.Driver
	logger    *slog.Logger
}

// HandleDiscovery dispatches a parsed discovery packet to the
// appropriate handler based on its code.
func (s *InterfaceServer) HandleDiscovery(pkt *Packet) {
	switch pkt.Code {
	case CodePADI:
		s.handlePADI(pkt)
	case CodePADR:
		s.handlePADR(pkt)
	case CodePADT:
		s.handlePADT(pkt)
	}
}

func (s *InterfaceServer) handlePADI(pkt *Packet) {
	if pkt.SID != 0 {
		return
	}

	if s.limiter != nil && !s.limiter.Check(pkt.SrcMAC) {
		return
	}

	if !MatchServiceName(pkt, s.serviceNames) {
		s.logger.Debug("pppoe: PADI service-name mismatch", "src", net.HardwareAddr(pkt.SrcMAC[:]))
		return
	}

	cookie := GenerateCookie(s.cookieKey, s.hwAddr[:], pkt.SrcMAC[:], relayIDFromPacket(pkt))

	var buf [EthMaxLen]byte
	frame := BuildPADO(buf[:], s.hwAddr, pkt, s.acName, s.serviceNames, cookie)
	if frame == nil {
		s.logger.Warn("pppoe: PADO frame too large")
		return
	}

	s.sendFrame(frame)
}

func (s *InterfaceServer) handlePADR(pkt *Packet) {
	if pkt.SID != 0 {
		return
	}

	cookieTag := pkt.FindTag(TagACCookie)
	if cookieTag == nil {
		s.logger.Debug("pppoe: PADR without AC-Cookie", "src", net.HardwareAddr(pkt.SrcMAC[:]))
		return
	}

	if !VerifyCookie(s.cookieKey, cookieTag.Value, s.hwAddr[:], pkt.SrcMAC[:], relayIDFromPacket(pkt), s.cookieTimeout) {
		s.logger.Debug("pppoe: PADR invalid cookie", "src", net.HardwareAddr(pkt.SrcMAC[:]))
		return
	}

	if !MatchServiceName(pkt, s.serviceNames) {
		var buf [EthMaxLen]byte
		frame := BuildPADSError(buf[:], s.hwAddr, pkt, s.acName, TagSvcNameError)
		if frame != nil {
			s.sendFrame(frame)
		}
		return
	}

	// PADR dedup: if a session with this MAC already exists and has not
	// yet started PPP (subscriber retransmitted PADR before getting our
	// PADS), re-send PADS with the existing SID instead of allocating a
	// new session. Matches accel-ppp's find_channel check.
	if existing := s.sessions.LookupByMAC(net.HardwareAddr(pkt.SrcMAC[:])); existing != nil && existing.State == StateDiscovery {
		var buf [EthMaxLen]byte
		frame := BuildPADS(buf[:], s.hwAddr, pkt, s.acName, existing.SID)
		if frame != nil {
			s.sendFrame(frame)
		}
		return
	}

	sid, err := s.sessions.AllocSID()
	if err != nil {
		s.logger.Warn("pppoe: session ID exhausted", "error", err)
		var buf [EthMaxLen]byte
		frame := BuildPADSError(buf[:], s.hwAddr, pkt, s.acName, TagACSystemError)
		if frame != nil {
			s.sendFrame(frame)
		}
		return
	}

	sess := &Session{
		SID:         sid,
		MAC:         net.HardwareAddr(append([]byte(nil), pkt.SrcMAC[:]...)),
		IfName:      s.ifName,
		ServiceName: pkt.ServiceNameString(),
		State:       StateDiscovery,
		CreatedAt:   time.Now(),
	}
	if hostUniq := pkt.FindTag(TagHostUniq); hostUniq != nil {
		sess.HostUniq = append([]byte(nil), hostUniq.Value...)
	}
	if err := s.sessions.Add(sess); err != nil {
		s.sessions.FreeSID(sid)
		s.logger.Warn("pppoe: session add failed", "sid", sid, "error", err)
		return
	}

	if s.pppDriver == nil {
		s.logger.Warn("pppoe: no PPP driver, session will not start", "sid", sid)
		closePPPoxFD(s.sessions.Remove(sid))
		return
	}

	pppoxFD, err := pppoeCreate(s.ifName, sid, pkt.SrcMAC)
	if err != nil {
		s.logger.Error("pppoe: kernel socket failed", "sid", sid, "error", err)
		closePPPoxFD(s.sessions.Remove(sid))
		return
	}
	sess.PppoxFD = pppoxFD

	chanFD, unitFD, unitNum, err := ppp.DevPPPSetup(pppoxFD)
	if err != nil {
		s.logger.Error("pppoe: devPPPSetup failed", "sid", sid, "error", err)
		closePPPoxFD(s.sessions.Remove(sid))
		return
	}
	sess.UnitNum = unitNum

	// Send PADS only after kernel setup succeeds. If we sent it
	// earlier and kernel setup failed, the subscriber would wait
	// for LCP that never comes.
	var buf [EthMaxLen]byte
	frame := BuildPADS(buf[:], s.hwAddr, pkt, s.acName, sid)
	if frame == nil {
		closePPPoxFD(chanFD)
		closePPPoxFD(unitFD)
		closePPPoxFD(s.sessions.Remove(sid))
		return
	}
	s.sendFrame(frame)

	sess.State = StateSession

	start := ppp.StartSession{
		TunnelID:        uint16(s.ifIndex),
		SessionID:       sid,
		ChanFD:          chanFD,
		UnitFD:          unitFD,
		UnitNum:         unitNum,
		LNSMode:         true,
		MaxMRU:          PPPoEMaxMTU,
		AccessInterface: s.ifName,
		SubscriberMAC:   net.HardwareAddr(append([]byte(nil), pkt.SrcMAC[:]...)),
		ServiceName:     pkt.ServiceNameString(),
		VendorTags:      vendorTagsFromPacket(pkt),
	}

	s.pppDriver.SessionsIn() <- start
}

func (s *InterfaceServer) handlePADT(pkt *Packet) {
	if pkt.SID == 0 {
		return
	}

	sess := s.sessions.Lookup(pkt.SID)
	if sess == nil {
		return
	}

	if !bytes.Equal(sess.MAC, pkt.SrcMAC[:]) {
		s.logger.Warn("pppoe: PADT MAC mismatch", "sid", pkt.SID,
			"expected", sess.MAC, "got", net.HardwareAddr(pkt.SrcMAC[:]))
		return
	}

	s.logger.Info("pppoe: PADT received", "sid", pkt.SID)

	if s.pppDriver != nil {
		_ = s.pppDriver.StopSession(uint16(s.ifIndex), pkt.SID)
	}

	closePPPoxFD(s.sessions.Remove(pkt.SID))
}

// handleSessionDown is called by the event consumer when PPP reports
// a session has ended. Sends PADT to the subscriber and cleans up.
func (s *InterfaceServer) handleSessionDown(sid uint16) {
	sess := s.sessions.Lookup(sid)
	if sess == nil {
		return
	}

	var dstMAC [EthALen]byte
	copy(dstMAC[:], sess.MAC)

	var buf [EthMaxLen]byte
	frame := BuildPADT(buf[:], s.hwAddr, dstMAC, sid, s.acName)
	if frame != nil {
		s.sendFrame(frame)
	}

	closePPPoxFD(s.sessions.Remove(sid))
}

func (s *InterfaceServer) sendFrame(frame []byte) {
	if err := sendDiscoveryFrame(s.discFD, s.ifIndex, frame); err != nil {
		s.logger.Debug("pppoe: send failed", "error", err)
	}
}

func vendorTagsFromPacket(pkt *Packet) []byte {
	tag := pkt.FindTag(TagVendorSpecific)
	if tag == nil {
		return nil
	}
	return append([]byte(nil), tag.Value...)
}

func relayIDFromPacket(pkt *Packet) []byte {
	tag := pkt.FindTag(TagRelaySessionID)
	if tag == nil {
		return nil
	}
	return tag.Value
}
