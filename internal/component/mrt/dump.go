// Design: docs/architecture/mrt.md — MRT record building from BGP events

package mrt

import (
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	mrtfmt "codeberg.org/thomas-mangin/ze/internal/mrt"
)

type poolBuf struct {
	b []byte
}

var bufPool = sync.Pool{
	New: func() any { return &poolBuf{b: make([]byte, 64*1024)} },
}

func getBuf() *poolBuf {
	pb, ok := bufPool.Get().(*poolBuf)
	if !ok {
		return &poolBuf{b: make([]byte, 64*1024)}
	}
	return pb
}

func writeHeader(buf []byte, extTimestamp bool, now time.Time, typ, subtype uint16, msgLen int) {
	ts := uint32(now.Unix())
	if extTimestamp {
		usec := uint32(now.Nanosecond() / 1000)
		mrtfmt.WriteExtendedHeader(buf, 0, ts, usec, typ, subtype, uint32(msgLen+mrtfmt.ExtTimestampLen))
	} else {
		mrtfmt.WriteCommonHeader(buf, 0, ts, typ, subtype, uint32(msgLen))
	}
}

// peerInfoToHeader writes peer/local IP into ipBuf (caller-owned, avoids heap escape)
// and returns a BGP4MPHeader whose IP slices point into ipBuf.
// ipBuf must be at least 32 bytes (2 x 16 for IPv6).
func peerInfoToHeader(peer *plugin.PeerInfo, ipBuf []byte) *mrtfmt.BGP4MPHeader {
	hdr := &mrtfmt.BGP4MPHeader{
		PeerAS:  peer.PeerAS,
		LocalAS: peer.LocalAS,
	}
	addr := peer.Address
	localAddr := peer.LocalAddress
	if addr.Is6() {
		hdr.AFI = mrtfmt.AFIIPv6
		p := addr.As16()
		copy(ipBuf[0:16], p[:])
		l := localAddr.As16()
		copy(ipBuf[16:32], l[:])
		hdr.PeerIP = ipBuf[0:16]
		hdr.LocalIP = ipBuf[16:32]
	} else {
		hdr.AFI = mrtfmt.AFIIPv4
		p := addr.As4()
		copy(ipBuf[0:4], p[:])
		l := localAddr.As4()
		copy(ipBuf[4:8], l[:])
		hdr.PeerIP = ipBuf[0:4]
		hdr.LocalIP = ipBuf[4:8]
	}
	return hdr
}

func localSubtype(subtype uint16) uint16 {
	switch subtype {
	case mrtfmt.BGP4MPMessageAS4:
		return mrtfmt.BGP4MPMessageAS4Local
	case mrtfmt.BGP4MPMessageAS4AP:
		return mrtfmt.BGP4MPMessageAS4LocalAP
	case mrtfmt.BGP4MPMessage:
		return mrtfmt.BGP4MPMessageLocal
	case mrtfmt.BGP4MPMessageAP:
		return mrtfmt.BGP4MPMessageLocalAP
	}
	return subtype
}

func (c *Component) dumpRIB() {
	if c.routes == nil {
		return
	}
	if err := c.routes.Rotate(); err != nil {
		c.logger.Warn("mrt: rotate rib file", "error", err)
	}
	c.logger.Debug("mrt: periodic RIB dump (placeholder)")
}

func (c *Component) headerSize() int {
	if c.config.ExtendedTimestamp {
		return mrtfmt.CommonHeaderLen + mrtfmt.ExtTimestampLen
	}
	return mrtfmt.CommonHeaderLen
}

func (c *Component) bgp4mpTypeSubtype() (uint16, uint16) {
	typ := mrtfmt.TypeBGP4MP
	if c.config.ExtendedTimestamp {
		typ = mrtfmt.TypeBGP4MPET
	}
	subtype := mrtfmt.BGP4MPMessageAS4
	if c.config.AddPath {
		subtype = mrtfmt.BGP4MPMessageAS4AP
	}
	return typ, subtype
}
