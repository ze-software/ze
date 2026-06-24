// Design: plan/learned/958-ospf-4-component-config.md -- OSPF packet dispatcher
// Design: plan/learned/972-ospf-af-unify.md -- Phase 2: header decode + checksum verify go
// through the Codec seam (neutral Header/PacketType), so the dispatcher is shared by
// the IPv4 and IPv6 families and only the codec differs.
// Related: instance.go -- the engine that registers handlers and owns the dispatcher

package ospf

import (
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/transport"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

type packetHandler func(transport.RawPacket, Header)

const dropReasonDecode = "decode"

type dispatcher struct {
	mu         sync.RWMutex
	codec      Codec
	handlers   map[PacketType]packetHandler
	droppedCnt uint64
	areaOK     func(ifindex int, id types.AreaID) bool
	// authOK verifies authentication for a received packet before it is routed to a
	// handler (ospf-12). It returns false to drop; nil means no auth enforcement.
	authOK func(rp transport.RawPacket, h Header) bool
	// instanceID is the OSPFv3 Instance ID this engine is configured for (RFC 5340 sec
	// 2.5); 0 for the OSPFv2 family (which has no Instance ID). A received packet whose
	// Instance ID does not match is discarded (the per-instance demux).
	instanceID uint8
}

func newDispatcher(codec Codec) *dispatcher {
	return &dispatcher{codec: codec, handlers: make(map[PacketType]packetHandler)}
}

func (d *dispatcher) register(pt PacketType, h packetHandler) {
	d.mu.Lock()
	d.handlers[pt] = h
	d.mu.Unlock()
}

// dispatch routes a received OSPF payload by the common-header Type field. It decodes
// the header and verifies the checksum through the codec (version-specific: OSPFv2
// checksums the datagram, OSPFv3 the IPv6 upper-layer pseudo-header), rejects wrong
// version/unknown type/bad checksum, and drops packets whose Area ID does not match the
// receiving interface.
func (d *dispatcher) dispatch(rp transport.RawPacket) {
	h, err := d.codec.DecodeHeader(rp.Payload)
	if err != nil {
		d.drop()
		return
	}
	if !d.codec.VerifyChecksum(rp.Payload, rp.Src, rp.Dst) {
		d.drop()
		return
	}
	d.mu.RLock()
	handler := d.handlers[h.Type]
	areaOK := d.areaOK
	authOK := d.authOK
	instanceID := d.instanceID
	d.mu.RUnlock()
	if handler == nil {
		d.drop()
		return
	}
	// RFC 5340 sec 4.2.2: a received OSPFv3 packet whose Instance ID does not match the
	// interface's configured Instance ID MUST be discarded -- the per-instance demux on a
	// link carrying multiple OSPFv3 instances. OSPFv2 has no Instance ID (both stay 0).
	if h.InstanceID != instanceID {
		d.drop()
		return
	}
	if areaOK != nil && !areaOK(rp.IfIndex, h.AreaID) {
		d.drop()
		return
	}
	// Authentication is the last gate before protocol processing: a packet that fails
	// verification is dropped before any ISM/NSM/LSDB handler runs (ospf-12).
	if authOK != nil && !authOK(rp, h) {
		d.drop()
		return
	}
	handler(rp, h)
}

func (d *dispatcher) drop() {
	d.mu.Lock()
	d.droppedCnt++
	d.mu.Unlock()
}

func (d *dispatcher) dropped() uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.droppedCnt
}
