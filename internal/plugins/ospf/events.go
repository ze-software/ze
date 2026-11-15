// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPFv2 event bus types
// Related: register.go -- registers the namespace and wires the EventBus
// RFC: rfc/short/rfc5709.md (Section 3.2 last-key-expiration notification)
package ospf

import (
	"context"
	"log/slog"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	"github.com/ze-software/ze/pkg/ze"
)

// Namespace is the one name OSPF answers to: its event namespace, its plugin
// registration name, its component name and its configuration root.
const Namespace = "ospf"

const (
	EventNeighborUp     = "neighbor-up"
	EventNeighborDown   = "neighbor-down"
	EventSPFRun         = "spf-run"
	EventLSDBChange     = "lsdb-change"
	EventInterfaceState = "interface-state"
	EventDRChange       = "dr-change"
	EventNeighborChange = "neighbor-change"
	// EventLastKeyExpiration is RFC 5709 Section 3.2's "last Authentication Key
	// expiration" notification to the network manager.
	EventLastKeyExpiration = "last-key-expiration"
)

// lastKeyExpirationEvent names the interface whose last key expired, the key Ze keeps
// using as if its lifetime were infinite, and the direction that first met the expiry:
// "send" when the signer found no live send-lifetime, "receive" when the verifier found
// no live accept-lifetime.
type lastKeyExpirationEvent struct {
	Interface string `json:"interface"`
	KeyID     uint32 `json:"key-id"`
	Direction string `json:"direction"`
}

type neighborEvent struct {
	Interface  string `json:"interface"`
	NeighborID string `json:"neighbor-id"`
	State      string `json:"state"`
}

type interfaceEvent struct {
	Interface     string `json:"interface"`
	AreaID        string `json:"area-id"`
	State         string `json:"state"`
	NetworkType   string `json:"network-type"`
	DR            string `json:"dr"`
	BDR           string `json:"bdr"`
	NeighborCount int    `json:"neighbor-count"`
}

type spfRunEvent struct {
	AreaID   string `json:"area-id"`
	Reason   string `json:"reason"`
	Duration string `json:"duration"`
}

type lsdbChangeEvent struct {
	AreaID            string `json:"area-id"`
	LinkStateType     string `json:"link-state-type"`
	LinkStateID       string `json:"link-state-id"`
	AdvertisingRouter string `json:"advertising-router"`
	Action            string `json:"action"`
}

var (
	NeighborUp        = events.Register[*neighborEvent](Namespace, EventNeighborUp)
	NeighborDown      = events.Register[*neighborEvent](Namespace, EventNeighborDown)
	spfRun            = events.Register[*spfRunEvent](Namespace, EventSPFRun)
	lsdbChange        = events.Register[*lsdbChangeEvent](Namespace, EventLSDBChange)
	InterfaceState    = events.Register[*interfaceEvent](Namespace, EventInterfaceState)
	DRChange          = events.Register[*interfaceEvent](Namespace, EventDRChange)
	NeighborChange    = events.Register[*interfaceEvent](Namespace, EventNeighborChange)
	LastKeyExpiration = events.Register[*lastKeyExpirationEvent](Namespace, EventLastKeyExpiration)
)

type eventSink struct {
	bus ze.EventBus
}

func newEventSink(bus ze.EventBus) *eventSink { return &eventSink{bus: bus} }

func (s *eventSink) InterfaceStateChanged(snap ospfiface.Snapshot) {
	s.emit(InterfaceState, EventInterfaceState, snap)
}

func (s *eventSink) DRChanged(snap ospfiface.Snapshot) {
	s.emit(DRChange, EventDRChange, snap)
}

func (s *eventSink) NeighborChanged(snap ospfiface.Snapshot) {
	s.emit(NeighborChange, EventNeighborChange, snap)
}

func (s *eventSink) NeighborUp(snap ospfneighbor.Snapshot) {
	s.emitNeighbor(NeighborUp, EventNeighborUp, snap)
}

func (s *eventSink) NeighborDown(snap ospfneighbor.Snapshot) {
	s.emitNeighbor(NeighborDown, EventNeighborDown, snap)
}

func (s *eventSink) emitNeighbor(handle *events.Event[*neighborEvent], label string, snap ospfneighbor.Snapshot) {
	if s == nil || s.bus == nil {
		return
	}
	if _, err := handle.Emit(s.bus, &neighborEvent{Interface: snap.Interface, NeighborID: snap.RouterID, State: snap.State}); err != nil {
		warnEmitRefused(label, "neighbor "+snap.RouterID+" on interface "+snap.Interface, showNeighborFix, err)
	}
}

// emitRefusedWhy is why a refused event matters. The bus refuses only an event pair
// its registry does not hold or a payload it cannot encode (the plugin server's
// deliverEvent), and every OSPF event pair and payload is Ze's own, so only a defect in
// Ze gets here. The state machine that called the sink is unaffected.
const emitRefusedWhy = "Plugins that follow OSPF state missed this change, and only a defect in Ze refuses the event; " +
	"OSPF itself is unaffected"

// showNeighborFix and showInterfaceFix show the state a consumer that missed a
// neighbor or interface event can read instead.
const (
	showNeighborFix  = "ze " + cmdShowNeighbor
	showInterfaceFix = "ze " + cmdShowInterface
)

// warnEmitRefused reports an event the bus refused: a WARN under the internal-defect
// doctor code, and the error event queued under it for the daemon's report bus
// (issueQueue), as that code's other sites do. The emit has no caller that could act
// on an error: the state machine has already changed state, which is right whatever
// the bus says. A neighbor or interface changes state at most once per protocol
// exchange, so a line and a raise for each refusal flood neither the log nor the bus.
func warnEmitRefused(event, subject, fix string, err error) {
	issue := opererr.Wrap(err, "OSPF did not publish the "+event+" event of "+subject, emitRefusedWhy, fix)
	busSubject := event + " " + subject
	reportErr := issueQueue.Load().RaiseError(diagnostic.CodeDoctorInternalDefect, busSubject,
		issue.Problem(), issue.Fix(), internalDefectRaiseFailed(logger(), busSubject, issue.Fix()))
	logger().LogAttrs(context.Background(), slog.LevelWarn, issue.Problem(),
		issue.LogAttrs(slog.String("code", diagnostic.CodeDoctorInternalDefect), slog.String("event", event),
			issuequeue.Outcome(reportErr))...)
}

// lastKeyExpired emits RFC 5709 Section 3.2's "last Authentication Key expiration"
// notification. A nil sink or bus emits nothing; the caller has already logged it.
func (s *eventSink) lastKeyExpired(notice *lastKeyExpirationEvent) {
	if s == nil {
		return
	}
	if s.bus == nil {
		return
	}
	if _, err := LastKeyExpiration.Emit(s.bus, notice); err != nil {
		warnEmitRefused(EventLastKeyExpiration, "interface "+notice.Interface, showInterfaceFix, err)
	}
}

func (s *eventSink) emit(handle *events.Event[*interfaceEvent], label string, snap ospfiface.Snapshot) {
	if s == nil || s.bus == nil {
		return
	}
	if _, err := handle.Emit(s.bus, interfaceEventOf(snap)); err != nil {
		warnEmitRefused(label, "interface "+snap.Name, showInterfaceFix, err)
	}
}

func interfaceEventOf(snap ospfiface.Snapshot) *interfaceEvent {
	return &interfaceEvent{
		Interface:     snap.Name,
		AreaID:        snap.Area,
		State:         snap.State,
		NetworkType:   snap.NetworkType,
		DR:            snap.DR,
		BDR:           snap.BDR,
		NeighborCount: snap.NeighborCount,
	}
}
