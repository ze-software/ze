// Design: docs/architecture/mrt.md — MRT recording consumer for chaos

package report

import (
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/mrt"
)

// MRTLogConfig holds parameters needed to construct MRT record headers.
type MRTLogConfig struct {
	// LocalAS is the route server's AS number.
	LocalAS uint32

	// LocalAddr is the route server's IP address.
	LocalAddr netip.Addr

	// Peers maps peer index to its AS and IP for MRT headers.
	Peers []MRTPeer
}

// MRTPeer holds the per-peer info needed for MRT BGP4MP headers.
type MRTPeer struct {
	ASN  uint32
	Addr netip.Addr
}

// MRTLog writes BGP4MP_MESSAGE_AS4 and BGP4MP_STATE_CHANGE_AS4 records
// from chaos peer events. It implements the Consumer interface.
type MRTLog struct {
	mu          sync.Mutex
	writer      *mrt.Writer
	cfg         MRTLogConfig
	localIP     []byte
	established []bool // per-peer: true after EventEstablished, false after EventDisconnected
	err         error
}

// NewMRTLog creates an MRTLog consumer that writes to the given file pattern.
func NewMRTLog(pattern string, cfg MRTLogConfig) *MRTLog {
	return &MRTLog{
		writer:      mrt.NewWriter(pattern),
		cfg:         cfg,
		localIP:     addrBytes(cfg.LocalAddr),
		established: make([]bool, len(cfg.Peers)),
	}
}

// ProcessEvent encodes and writes an MRT record for message and state events.
func (m *MRTLog) ProcessEvent(ev peer.Event) {
	switch ev.Type {
	case peer.EventEstablished:
		if ev.PeerIndex >= 0 && ev.PeerIndex < len(m.established) {
			m.established[ev.PeerIndex] = true
		}
		m.writeStateChange(ev, mrt.FSMIdle, mrt.FSMEstablished)
	case peer.EventDisconnected:
		if ev.PeerIndex < 0 || ev.PeerIndex >= len(m.established) || !m.established[ev.PeerIndex] {
			return
		}
		m.established[ev.PeerIndex] = false
		m.writeStateChange(ev, mrt.FSMEstablished, mrt.FSMIdle)
	case peer.EventRouteSent, peer.EventRouteReceived, peer.EventWithdrawalSent:
		if ev.BGPMessage == nil {
			return
		}
		m.writeMessage(ev)
	case peer.EventRouteWithdrawn, peer.EventEORSent, peer.EventError,
		peer.EventChaosExecuted, peer.EventReconnecting, peer.EventRouteAction,
		peer.EventDroppedEvents:
	}
}

// Close flushes and closes the underlying MRT writer, returning
// the first error encountered during the session.
func (m *MRTLog) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if closeErr := m.writer.Close(); closeErr != nil && m.err == nil {
		m.err = closeErr
	}
	return m.err
}

func (m *MRTLog) writeMessage(ev peer.Event) {
	hdr := m.bgp4mpHeader(ev.PeerIndex)

	// BGP4MP common header size: PeerAS(4) + LocalAS(4) + IfIndex(2) + AFI(2) + PeerIP(4) + LocalIP(4) = 20 for IPv4
	bodyLen := bgp4mpHeaderLen(hdr.AFI) + len(ev.BGPMessage)
	recordLen := mrt.CommonHeaderLen + bodyLen

	buf := make([]byte, recordLen)
	off := mrt.WriteCommonHeader(buf, 0, uint32(ev.Time.Unix()), mrt.TypeBGP4MP, mrt.BGP4MPMessageAS4, uint32(bodyLen))
	mrt.WriteBGP4MPMessage(buf, off, &hdr, true, ev.BGPMessage)

	m.mu.Lock()
	if m.err == nil {
		m.err = m.writer.Write(buf)
	}
	m.mu.Unlock()
}

func (m *MRTLog) writeStateChange(ev peer.Event, oldState, newState uint16) {
	hdr := m.bgp4mpHeader(ev.PeerIndex)

	// State change body: BGP4MP common header + OldState(2) + NewState(2)
	bodyLen := bgp4mpHeaderLen(hdr.AFI) + 4
	recordLen := mrt.CommonHeaderLen + bodyLen

	buf := make([]byte, recordLen)
	off := mrt.WriteCommonHeader(buf, 0, uint32(ev.Time.Unix()), mrt.TypeBGP4MP, mrt.BGP4MPStateChangeAS4, uint32(bodyLen))
	mrt.WriteBGP4MPStateChange(buf, off, &hdr, true, oldState, newState)

	m.mu.Lock()
	if m.err == nil {
		m.err = m.writer.Write(buf)
	}
	m.mu.Unlock()
}

func (m *MRTLog) bgp4mpHeader(peerIndex int) mrt.BGP4MPHeader {
	var peerIP []byte
	var peerAS uint32
	if peerIndex >= 0 && peerIndex < len(m.cfg.Peers) {
		p := &m.cfg.Peers[peerIndex]
		peerAS = p.ASN
		peerIP = addrBytes(p.Addr)
	} else {
		peerIP = make([]byte, 4)
	}

	return mrt.BGP4MPHeader{
		PeerAS:  peerAS,
		LocalAS: m.cfg.LocalAS,
		IfIndex: 0,
		AFI:     mrt.AFIIPv4,
		PeerIP:  peerIP,
		LocalIP: m.localIP,
	}
}

// bgp4mpHeaderLen returns the byte size of the BGP4MP AS4 common header for the given AFI.
func bgp4mpHeaderLen(afi uint16) int {
	ipLen := 4
	if afi == mrt.AFIIPv6 {
		ipLen = 16
	}
	// PeerAS(4) + LocalAS(4) + IfIndex(2) + AFI(2) + PeerIP + LocalIP
	return 4 + 4 + 2 + 2 + ipLen*2
}

func addrBytes(addr netip.Addr) []byte {
	if addr.Is4() {
		a4 := addr.As4()
		return a4[:]
	}
	a16 := addr.As16()
	return a16[:]
}
