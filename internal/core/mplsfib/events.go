// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- MPLS forwarding-entry input to fib-kernel
//
// mplsfib is the leaf event package that carries MPLS label-switching entries
// from label-distribution sources (RSVP-TE, LDP) to the kernel FIB owner
// (fib-kernel). It keeps fib-kernel the single programmer of kernel forwarding
// state -- avoiding duplicated netlink code and preserving unified stale-sweep,
// re-assertion and metrics -- while not abusing sysrib's prefix best-path
// machinery for what are locally-unique, label-keyed entries.
//
// Push entries (FEC prefix -> label) are prefix-keyed and could also flow via
// sysrib; swap/pop entries are in-label-keyed (AF_MPLS) and do not fit a
// best-path table, so this dedicated channel carries all three uniformly.
package mplsfib

import (
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/ze"
)

// Namespace is the event namespace for MPLS forwarding entries.
const Namespace = "mpls-fib"

// EventEntry is the event type carrying a batch of MPLS forwarding entries.
const EventEntry = "entry"

// Action is the lifecycle change for an entry. Zero is invalid so an
// uninitialised entry surfaces immediately.
type Action uint8

const (
	ActionUnspecified Action = 0
	ActionAdd         Action = 1
	ActionRemove      Action = 2
	// ActionRemoveLabelSource retires retained swap/pop labels owned by Source.
	// Prefix-keyed push contexts keep their separate per-entry lifecycle.
	ActionRemoveLabelSource Action = 3
)

// Op is the MPLS label operation. Zero is invalid.
type Op uint8

const (
	OpUnspecified Op = 0
	OpPush        Op = 1 // impose a label stack on traffic to FEC (ingress)
	OpSwap        Op = 2 // swap InLabel for OutLabels toward NextHop (transit)
	OpPop         Op = 3 // pop InLabel and forward toward NextHop (egress/PHP)
)

// Entry is one MPLS forwarding entry. Value-typed with fixed-size fields so the
// batch backing array stays pool-stable and carries no pointer into producer
// memory (OutLabels is a small owned slice the consumer copies if it retains).
type Entry struct {
	Action Action
	Op     Op

	// InLabel keys swap/pop entries (the incoming label looked up in the
	// AF_MPLS table). Unused for push.
	InLabel uint32

	// FEC is the destination prefix for push entries. Unused for swap/pop.
	FEC netip.Prefix
	// TableID isolates a push forwarding context. Zero selects the ordinary IP
	// FIB; nonzero installs a private table selected by an equal packet mark.
	// Swap/pop entries are in-label keyed and require zero.
	TableID uint32

	// OutLabels is the outgoing label stack for push/swap. Empty for pop.
	OutLabels []uint32

	// NextHop is the downstream neighbor the labeled packet is sent to.
	NextHop netip.Addr

	// PathMTU is the minimum downstream frame payload MTU, including the
	// complete outgoing label stack. Zero means unknown: use the output
	// device's MTU. The kernel subtracts retained as well as imposed labels.
	PathMTU uint32

	// Source identifies the producing protocol for diagnostics/ownership.
	Source uint16
}

// EntryBatch is the payload of (mpls-fib, entry): a set of entries from one
// producer applied together.
type EntryBatch struct {
	Entries []Entry
	// Acknowledge reports the native FIB owner's synchronous install result.
	// The callback is immutable payload; only Apply owns the result it captures.
	// Process subscribers cannot acknowledge a native kernel installation.
	Acknowledge func(error) `json:"-"`
}

// EntryChange is the typed handle for (mpls-fib, entry). Producers Emit;
// fib-kernel Subscribes.
var EntryChange = events.Register[*EntryBatch](Namespace, EventEntry)

var errNoOwnerAcknowledgement = errors.New("mpls-fib: no native forwarding owner acknowledged the batch")

// Apply returns only after the native forwarding owner has attempted the batch.
// A successful Emit is not an installation acknowledgement. A batch may be
// partially applied on error; producers must not advertise the failed result.
func Apply(bus ze.EventBus, entries []Entry) error {
	if bus == nil {
		return errNoOwnerAcknowledgement
	}
	var result error
	acknowledged := false
	batch := &EntryBatch{
		Entries: entries,
		Acknowledge: func(err error) {
			acknowledged = true
			result = errors.Join(result, err)
		},
	}
	if _, err := EntryChange.Emit(bus, batch); err != nil {
		return err
	}
	if !acknowledged {
		return errNoOwnerAcknowledgement
	}
	return result
}

// RemoveLabelSource retires all AF_MPLS swap/pop labels retained by the native owner
// for source. It does not remove prefix-keyed push contexts. A failed deletion
// remains owned and is retried by the next call; other successful deletions are
// not rolled back. The caller must not reuse labels until this returns nil.
func RemoveLabelSource(bus ze.EventBus, source uint16) error {
	return Apply(bus, []Entry{{Action: ActionRemoveLabelSource, Source: source}})
}
