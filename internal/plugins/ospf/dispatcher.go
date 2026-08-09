// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPF packet dispatcher
// Design: docs/architecture/ospf/ospf-af-unify.md -- Phase 2: header decode + checksum verify go
// through the Codec seam (neutral Header/PacketType), so the dispatcher is shared by
// the IPv4 and IPv6 families and only the codec differs.
// Related: instance.go -- the engine that registers handlers and owns the dispatcher
// RFC: rfc/short/rfc2328.md, rfc/short/rfc5340.md, rfc/short/rfc6549.md, rfc/short/rfc5838.md (§2.1 AF ranges)

package ospf

import (
	"sync"

	"github.com/ze-software/ze/internal/plugins/ospf/transport"
)

type packetHandler func(transport.RawPacket, Header)

const dropReasonDecode = "decode"

type dispatcher struct {
	mu         sync.RWMutex
	codec      Codec
	handlers   map[PacketType]packetHandler
	droppedCnt uint64
	// areaOK reports whether a received packet's Area is valid on the receiving interface.
	// It takes the full Header (not just the Area ID) so a routed virtual-link packet, which
	// carries the backbone Area but arrives on the transit interface, can be accepted by its
	// source Router ID (RFC 2328 section 15).
	areaOK func(ifindex int, h Header) bool
	// authOK verifies authentication for a received packet before it is routed to a
	// handler (ospf-12). It returns false to drop; nil means no auth enforcement.
	authOK func(rp transport.RawPacket, h Header) bool
	// instanceID is the Instance ID this engine is configured for: OSPFv3 (RFC 5340 sec
	// 2.5) or OSPFv2 Multi-Instance (RFC 6549 sec 2 / 3.1). A received packet whose
	// Instance ID does not match is discarded before any handler runs (the per-instance
	// demux). 0 is the default (base) instance for both families.
	instanceID uint8
	// onInstanceMismatch, when set, is called for a packet dropped because its Instance ID
	// did not match this engine's configured Instance ID. The engine uses it to increment
	// ze_ospf_instance_mismatch_drops_total{interface} for observability of the demux.
	onInstanceMismatch func(rp transport.RawPacket)
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
	onMismatch := d.onInstanceMismatch
	d.mu.RUnlock()
	if handler == nil {
		d.drop()
		return
	}
	// RFC 6549 sec 2 / 3.1 (OSPFv2) and RFC 5340 sec 4.2.2 (OSPFv3): a received packet whose
	// Instance ID does not match one configured for the receiving interface MUST be discarded
	// -- the per-instance demux on a link carrying multiple OSPF instances. This engine owns
	// exactly one Instance ID, so a non-match is dropped here before any ISM/NSM/LSDB handler
	// runs (so no cross-instance adjacency can form). The base instance keeps instanceID 0.
	if h.InstanceID != instanceID {
		if onMismatch != nil {
			onMismatch(rp)
		}
		d.drop()
		return
	}
	if areaOK != nil && !areaOK(rp.IfIndex, h) {
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

// currentInstanceID returns the engine's configured OSPFv3 Instance ID under the lock, for
// the AF-aware show output (RFC 5838 §2 debugging: identify the address-family instance).
func (d *dispatcher) currentInstanceID() uint8 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.instanceID
}

// setInstanceID records the engine's configured Instance ID under the lock so the config-apply
// goroutine's write does not race the per-packet dispatch read of d.instanceID (RFC 6549 demux).
func (d *dispatcher) setInstanceID(id uint8) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.instanceID = id
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
