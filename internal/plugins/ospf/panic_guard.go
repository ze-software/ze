// Design: docs/architecture/ospf/ospf-4-component-config.md -- panic containment in the engine
// Related: opaque_registry.go -- safeOpaqueCall, a registered consumer's callback
// Related: ri_registry.go -- invokeRITLVBuilder, a registered Router Information TLV builder
// Related: ext_subtlv.go -- invokeExtSubTLVBuild and renderExtSubTLV, registered sub-TLV codecs
// Related: decode_view.go -- safeOpaqueDetailDecode, a show-command opaque decoder
// Related: decode_view_v3.go -- safeV3Decode, a show-command OSPFv3 LSA decoder

package ospf

import (
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// enginePanicTasks is the OSPF engine's panic boundary: one taskguard.Guard and
// the tasks it declares. A contained panic is logged at ERROR with its stack,
// counted in ze_task_panics_total{component="ospf",task} and crash-recorded,
// whatever the caller does next. The transport's interface tasks have their own
// guard (transport/panic_guard.go) with distinct task names.
type enginePanicTasks struct {
	guard *taskguard.Guard
	// consumer is one call into a callback another package registered with the
	// engine (an opaque consumer, a Router Information TLV builder, an Extended
	// Link or Prefix sub-TLV codec). Its panic drops that callback's
	// contribution to the one LSA or origination; the consumer's own error
	// counter rises, and the engine and every adjacency keep running (RFC 5250
	// framework isolation).
	consumer taskguard.Task
	// showDecode is one decode or render of a stored LSA body for a show
	// command. Its panic leaves that row undecoded; nothing else changes.
	showDecode taskguard.Task
	// receive is the engine's receive worker handling one packet (OSPFv2 and
	// OSPFv3 share it). Its panic takes the packet's neighbor Down (KillNbr,
	// endReceivePanic); the worker resumes with the next packet and every other
	// neighbor stays up.
	receive taskguard.Task
}

const (
	consumerRestart   = "the registered consumer's contribution is dropped for this LSA or origination only, and it is called again on the next one"
	showDecodeRestart = "the show command answers that LSA undecoded, and the next show command decodes it again"
	receiveRestart    = "the packet is dropped and its sending neighbor goes Down (KillNbr); its next Hello brings it back, and every other neighbor stays up"
)

// enginePanics answers the engine's panic boundary, built on first use so the
// guard logs through the logger the plugin installed with setLogger.
var enginePanics = sync.OnceValue(newEnginePanicTasks)

func newEnginePanicTasks() *enginePanicTasks {
	guard := taskguard.New("ospf", logger())
	return &enginePanicTasks{
		guard:      guard,
		consumer:   guard.Task("consumer", consumerRestart),
		showDecode: guard.Task("show-decode", showDecodeRestart),
		receive:    guard.Task("receive", receiveRestart),
	}
}

// receivePackets dispatches received packets until the engine is canceled or
// recv closes, and then answers false. A panic while one packet is handled is
// contained by the receive task: that packet's neighbor goes Down
// (endReceivePanic) and the pass answers true, so the worker resumes with the
// next packet. The guard is deferred once per pass, not once per packet.
func (e *engine) receivePackets(recv <-chan transport.RawPacket) (resume bool) {
	var current transport.RawPacket
	defer enginePanics().receive.Contain(
		func() taskguard.Context { return describeReceivePanic(current) },
		func(*taskguard.Panic) {
			e.endReceivePanic(current)
			resume = true
		})

	for {
		select {
		case <-e.ctx.Done():
			return false
		case rp, ok := <-recv:
			if !ok {
				return false
			}
			current = rp
			e.dispatch.dispatch(rp)
		}
	}
}

// Offsets of the fields endReceivePanic reads from the raw OSPF packet header.
// RFC 2328 Appendix A.3.1 (OSPFv2) and RFC 5340 Appendix A.3.1 (OSPFv3) place
// the Router ID at octets 4-7 and the Area ID at octets 8-11 of the common
// header in both versions.
const (
	headerRouterIDOffset = 4
	headerAreaIDOffset   = 8
	headerAreaIDEnd      = 12
)

// describeReceivePanic names the packet a contained receive panic ended: its
// arrival interface index and the sender's Router ID, read at fixed offsets so
// describing never decodes the packet again.
// It runs once per contained panic, never per packet.
func describeReceivePanic(rp transport.RawPacket) taskguard.Context {
	unit := "packet on ifindex " + strconv.Itoa(rp.IfIndex)
	if id, ok := panickedRouterID(rp.Payload); ok {
		unit += " from router " + id.String()
	}
	return taskguard.Context{Unit: unit}
}

// panickedRouterID reads the sender's Router ID from the raw header at its fixed
// offset, without the codec, so a defective decoder cannot panic here again.
func panickedRouterID(payload []byte) (types.RouterID, bool) {
	var id types.RouterID
	if len(payload) < headerAreaIDOffset {
		return id, false
	}
	copy(id[:], payload[headerRouterIDOffset:headerAreaIDOffset])
	return id, true
}

// endReceivePanic ends the packet a contained receive panic interrupted by
// taking its sending neighbor Down, and only that neighbor: the panic may have
// left that neighbor's state half-updated, while every other neighbor's state is
// untouched. The neighbor is found the way the handlers find it (receiveTarget,
// which also resolves a virtual link), from a header built from the raw octets
// rather than decoded. receiveTarget takes e.mu by defer, and every e.mu hold on
// the receive path releases by defer (engine_lock_panic_test.go), so the lock is
// free here.
//
// RFC 2328 Section 10.2: "KillNbr
// This  is  an  indication that  all  communication  with  the
// neighbor  is now  impossible,  forcing  the  neighbor  to
// revert  to  Down  state."
// RFC 2328 Section 10.3, Event KillNbr: "Action:  The Link state retransmission
// list, Database summary list and Link state request list are cleared of
// LSAs.  Also, the Inactivity Timer is disabled." Nothing is sent to the
// neighbor first; its next Hello starts the adjacency again.
func (e *engine) endReceivePanic(rp transport.RawPacket) {
	id, ok := panickedRouterID(rp.Payload)
	if !ok || len(rp.Payload) < headerAreaIDEnd {
		e.log.Warn("ospf: a contained receive panic left no neighbor to take down: the packet is too short to name its sender, so it is only dropped",
			"ifindex", rp.IfIndex, "octets", len(rp.Payload))
		return
	}
	var h Header
	h.RouterID = id
	copy(h.AreaID[:], rp.Payload[headerAreaIDOffset:headerAreaIDEnd])
	name, _, found := e.receiveTarget(rp.IfIndex, h)
	if !found {
		e.log.Warn("ospf: a contained receive panic left no neighbor to take down: the packet's interface is not running OSPF, so it is only dropped",
			"ifindex", rp.IfIndex, "router-id", id.String())
		return
	}
	if e.neighbors == nil {
		e.log.Warn("ospf: a contained receive panic left no neighbor to take down: the engine has no neighbor table yet",
			"interface", name, "router-id", id.String())
		return
	}
	// RFC 2328 Section 10.3: KillNbr.
	e.neighbors.NeighborDown(name, id)
	e.log.Warn("ospf: the neighbor whose packet panicked the receive worker is Down; its next Hello brings it back",
		"interface", name, "router-id", id.String())
}

// onContained adapts step to the guard's end function: the guard has already
// logged, counted and recorded the panic when step runs, so step only resets the
// caller's result or steps the caller's own error counter.
func onContained(step func()) func(*taskguard.Panic) {
	return func(*taskguard.Panic) { step() }
}

// unitContext names the unit a contained panic ended, for the guard's log line
// and crash record.
func unitContext(unit string) func() taskguard.Context {
	return func() taskguard.Context { return taskguard.Context{Unit: unit} }
}
