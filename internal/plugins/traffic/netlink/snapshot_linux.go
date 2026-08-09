// Design: docs/architecture/traffic/tc-original-qdisc-restore.md -- tc original-qdisc restore
// Related: ops_linux.go -- tc operation seam used by snapshot checks
// Related: ai/rules/architecture.md -- the original-qdisc snapshot persists in
// the shared zefs store (database.zefs) via internal/core/statestore, not a loose
// file, so appliance state lives inside the managed, backed-up store.

//go:build linux

package trafficnetlink

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

var errLinuxBootIdIsEmpty = errors.New("linux boot id is empty")

const tcSnapshotVersion = 1

type tcSnapshotStore struct {
	Version    int                            `json:"version"`
	Interfaces map[string]tcInterfaceSnapshot `json:"interfaces"`
}

type tcInterfaceSnapshot struct {
	Interface    string          `json:"interface"`
	IfIndex      int             `json:"if-index"`
	HardwareAddr string          `json:"hardware-address"`
	BootID       string          `json:"boot-id"`
	Qdisc        tcQdiscSnapshot `json:"qdisc"`
}

type tcQdiscSnapshot struct {
	Type  string          `json:"type"`
	Attrs tcQdiscAttrs    `json:"attrs"`
	Data  json.RawMessage `json:"data"`
}

type tcQdiscAttrs struct {
	Handle       uint32  `json:"handle"`
	Parent       uint32  `json:"parent"`
	IngressBlock *uint32 `json:"ingress-block,omitempty"`
}

func currentBootID() (string, error) {
	b, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", fmt.Errorf("read linux boot id: %w", err)
	}
	id := strings.TrimSpace(string(b))
	if id == "" {
		return "", errLinuxBootIdIsEmpty
	}
	return id, nil
}

// loadTCSnapshots reads the persisted original-qdisc snapshots from the shared
// zefs store under KeyTrafficTCSnapshot. Best-effort: an unregistered store or a
// missing key yields an empty set (nothing to restore) with no error. A blob that
// is present but corrupt or of an unsupported version is rejected with an error, so
// the backend fails loudly rather than silently discarding restore state. The
// process-wide store is registered once at startup via statestore.SetStore; tests
// register a temp store.
func loadTCSnapshots() (map[string]tcInterfaceSnapshot, error) {
	data, ok := statestore.Get(zefs.KeyTrafficTCSnapshot.Pattern)
	if !ok {
		return map[string]tcInterfaceSnapshot{}, nil
	}
	var store tcSnapshotStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parse tc snapshot store: %w", err)
	}
	if store.Version != tcSnapshotVersion {
		return nil, fmt.Errorf("tc snapshot store: unsupported version %d", store.Version)
	}
	if store.Interfaces == nil {
		store.Interfaces = map[string]tcInterfaceSnapshot{}
	}
	return store.Interfaces, nil
}

// saveTCSnapshots persists the versioned snapshot store into the shared zefs store
// under KeyTrafficTCSnapshot. When no snapshots remain the key is removed so a stale
// blob does not linger. Persistence is best-effort: statestore no-ops when no store
// is registered, so a missing store never fails Apply/Close. Callers that must NOT
// destroy state without a durable snapshot (the qdisc-replace path) gate on
// statestore.Store() != nil BEFORE the destructive operation rather than relying on
// this best-effort save.
func saveTCSnapshots(snapshots map[string]tcInterfaceSnapshot) error {
	if len(snapshots) == 0 {
		return statestore.Remove(zefs.KeyTrafficTCSnapshot.Pattern)
	}
	store := tcSnapshotStore{Version: tcSnapshotVersion, Interfaces: snapshots}
	data, err := json.Marshal(store)
	if err != nil {
		return fmt.Errorf("marshal tc snapshot store: %w", err)
	}
	if _, err := statestore.Put(zefs.KeyTrafficTCSnapshot.Pattern, data); err != nil {
		return fmt.Errorf("persist tc snapshot store: %w", err)
	}
	return nil
}

func newInterfaceSnapshot(link netlink.Link, bootID string, qdisc netlink.Qdisc) (tcInterfaceSnapshot, error) {
	qs, err := newQdiscSnapshot(qdisc)
	if err != nil {
		return tcInterfaceSnapshot{}, err
	}
	attrs := link.Attrs()
	return tcInterfaceSnapshot{
		Interface:    attrs.Name,
		IfIndex:      attrs.Index,
		HardwareAddr: attrs.HardwareAddr.String(),
		BootID:       bootID,
		Qdisc:        qs,
	}, nil
}

func (s tcInterfaceSnapshot) validateLink(link netlink.Link, bootID string) error {
	attrs := link.Attrs()
	if s.BootID != bootID {
		return fmt.Errorf("snapshot boot id %q does not match current boot id %q", s.BootID, bootID)
	}
	if s.Interface != attrs.Name {
		return fmt.Errorf("snapshot interface %q does not match current interface %q", s.Interface, attrs.Name)
	}
	if s.IfIndex != attrs.Index {
		return fmt.Errorf("snapshot ifindex %d does not match current ifindex %d", s.IfIndex, attrs.Index)
	}
	if s.HardwareAddr != attrs.HardwareAddr.String() {
		return fmt.Errorf("snapshot hardware address %q does not match current hardware address %q", s.HardwareAddr, attrs.HardwareAddr.String())
	}
	return nil
}

// qdiscTypeNoqueue is the kernel's name for "this interface has no queueing
// discipline configured". It is the default root on every virtual interface --
// veth, dummy, bridge, and anything else the kernel gives no real queue -- so it
// is the state a QoS config is most often applied FROM, not an exotic corner.
const qdiscTypeNoqueue = "noqueue"

func newQdiscSnapshot(qdisc netlink.Qdisc) (tcQdiscSnapshot, error) {
	if generic, ok := qdisc.(*netlink.GenericQdisc); ok {
		// noqueue is exactly restorable even though it has no reconstructable
		// parameters, because it is the ABSENCE of a discipline: deleting whatever
		// root we installed returns the interface to it (restoreOriginalLocked).
		// Rejecting it meant ze refused to apply any traffic control to a veth,
		// dummy or bridge -- `qdisc "noqueue" cannot be snapshotted exactly by
		// backend tc` -- which is every container and VM deployment.
		//
		// Other GenericQdisc types (mq, clsact, ...) stay rejected: those DO carry
		// state, and this backend cannot reproduce it (ai/rules/protocol.md).
		if generic.Type() != qdiscTypeNoqueue {
			return tcQdiscSnapshot{}, fmt.Errorf("qdisc %q cannot be snapshotted exactly by backend tc", qdisc.Type())
		}
		return tcQdiscSnapshot{Type: qdiscTypeNoqueue, Attrs: snapshotAttrs(qdisc.Attrs())}, nil
	}
	switch qdisc.(type) {
	case *netlink.PfifoFast, *netlink.Prio, *netlink.Htb, *netlink.Hfsc,
		*netlink.Tbf, *netlink.Netem, *netlink.Fq, *netlink.FqCodel, *netlink.Sfq:
		b, err := json.Marshal(qdisc)
		if err != nil {
			return tcQdiscSnapshot{}, fmt.Errorf("snapshot qdisc %q: %w", qdisc.Type(), err)
		}
		return tcQdiscSnapshot{Type: qdisc.Type(), Attrs: snapshotAttrs(qdisc.Attrs()), Data: b}, nil
	default:
		return tcQdiscSnapshot{}, fmt.Errorf("qdisc %q cannot be restored exactly by backend tc", qdisc.Type())
	}
}

func snapshotAttrs(attrs *netlink.QdiscAttrs) tcQdiscAttrs {
	var ingressBlock *uint32
	if attrs.IngressBlock != nil {
		v := *attrs.IngressBlock
		ingressBlock = &v
	}
	return tcQdiscAttrs{Handle: attrs.Handle, Parent: attrs.Parent, IngressBlock: ingressBlock}
}

func (a tcQdiscAttrs) toNetlink(linkIndex int) netlink.QdiscAttrs {
	return netlink.QdiscAttrs{LinkIndex: linkIndex, Handle: a.Handle, Parent: a.Parent, IngressBlock: a.IngressBlock}
}

// restoredByDelete reports whether returning the interface to this snapshot is
// done by DELETING the root qdisc rather than by replacing it.
//
// Only noqueue qualifies. Deleting the root drops the interface back to the
// kernel default, and noqueue being what was observed there before ze touched
// anything is precisely the evidence that the default IS noqueue.
func (s tcQdiscSnapshot) restoredByDelete() bool { return s.Type == qdiscTypeNoqueue }

func (s tcQdiscSnapshot) toNetlink(linkIndex int) (netlink.Qdisc, error) {
	// Callers must route a delete-restored snapshot through qdiscDel; there is no
	// qdisc object to install for it. Guarded rather than assumed: a future caller
	// that forgets gets a named error, not a nil qdisc handed to the kernel.
	if s.restoredByDelete() {
		return nil, fmt.Errorf("qdisc %q is restored by deleting the root, not by replacing it", s.Type)
	}
	switch s.Type {
	case "pfifo_fast":
		var q netlink.PfifoFast
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "prio":
		var q netlink.Prio
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "htb":
		var q netlink.Htb
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "hfsc":
		var q netlink.Hfsc
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "tbf":
		var q netlink.Tbf
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "netem":
		var q netlink.Netem
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "fq":
		var q netlink.Fq
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "fq_codel":
		var q netlink.FqCodel
		return finishSnapshotQdisc(&q, s, linkIndex)
	case "sfq":
		var q netlink.Sfq
		return finishSnapshotQdisc(&q, s, linkIndex)
	default:
		return nil, fmt.Errorf("qdisc %q cannot be restored exactly by backend tc", s.Type)
	}
}

func finishSnapshotQdisc[T netlink.Qdisc](q T, s tcQdiscSnapshot, linkIndex int) (netlink.Qdisc, error) {
	if err := json.Unmarshal(s.Data, q); err != nil {
		return nil, fmt.Errorf("restore qdisc %q snapshot: %w", s.Type, err)
	}
	*q.Attrs() = s.Attrs.toNetlink(linkIndex)
	return q, nil
}
