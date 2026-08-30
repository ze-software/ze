// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPFv2 event bus types
// Related: register.go -- registers the namespace and wires the EventBus
package ospf

import (
	"github.com/ze-software/ze/internal/core/events"
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
)

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
	NeighborUp     = events.Register[*neighborEvent](Namespace, EventNeighborUp)
	NeighborDown   = events.Register[*neighborEvent](Namespace, EventNeighborDown)
	spfRun         = events.Register[*spfRunEvent](Namespace, EventSPFRun)
	lsdbChange     = events.Register[*lsdbChangeEvent](Namespace, EventLSDBChange)
	InterfaceState = events.Register[*interfaceEvent](Namespace, EventInterfaceState)
	DRChange       = events.Register[*interfaceEvent](Namespace, EventDRChange)
	NeighborChange = events.Register[*interfaceEvent](Namespace, EventNeighborChange)
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
		logger().Debug("ospf: event emit", "event", label, "interface", snap.Interface, "neighbor", snap.RouterID, "err", err)
	}
}

func (s *eventSink) emit(handle *events.Event[*interfaceEvent], label string, snap ospfiface.Snapshot) {
	if s == nil || s.bus == nil {
		return
	}
	if _, err := handle.Emit(s.bus, interfaceEventOf(snap)); err != nil {
		logger().Debug("ospf: event emit", "event", label, "interface", snap.Name, "err", err)
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
