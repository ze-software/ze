// Design: docs/architecture/mpls/mpls-kernel.md -- MPLS forwarding-entry input to the data plane's writer
// Related: label.go -- the validation every writer applies to an entry
//
// mplsfib is the leaf event package that carries MPLS label-switching entries
// from label-distribution sources (RSVP-TE, LDP) to the writer of the selected
// data plane: fib-kernel on the Linux kernel, fib-vpp on VPP. The writer of a
// data plane owns all of its forwarding state, labels included, so a producer
// names no writer: it declares Registration.NeedsDataPlane and the engine
// loads the one the configured backend selects. The channel does not abuse
// sysrib's prefix best-path machinery for what are locally-unique, label-keyed
// entries.
//
// Push entries (FEC prefix -> label) are prefix-keyed and could also flow via
// sysrib; swap/pop entries are in-label-keyed and do not fit a best-path
// table, so this dedicated channel carries all three uniformly.
//
// A writer that opens late (VPP connects after the producers started, or
// reconnects after it lost its FIB) asks the producers to re-apply what they
// want installed with a targeted (mpls-fib, replay-request), in the shared
// replay vocabulary (internal/core/replay).
package mplsfib

import (
	"errors"
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/replay"
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

	// FEC is the destination prefix for push entries. On a pop with no
	// NextHop (egress disposition) it names the payload's network layer
	// protocol, which a writer that cannot hand the payload back to the
	// kernel's IP lookup needs: RFC 3032 Section 2.2, "The LSR which pops the
	// last label off the stack must therefore be able to identify the
	// packet's network layer protocol." fib-kernel ignores it on a swap or a
	// pop.
	FEC netip.Prefix
	// TableID isolates a push forwarding context. Zero selects the ordinary IP
	// FIB; nonzero installs a private table selected by an equal packet mark.
	// Swap/pop entries are in-label keyed and require zero.
	TableID uint32

	// OutLabels is the outgoing label stack for push/swap. Empty for pop. An
	// empty push stack forwards FEC traffic as plain IP toward NextHop: the
	// downstream label was Implicit NULL, which "never actually appears in the
	// encapsulation" (RFC 3032 Section 2.1).
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
	// ReplayID is zero on an incremental batch, which every running writer
	// consumes. A nonzero token marks the answer to one replay request: only
	// the writer that minted that token consumes and acknowledges it, and
	// every other writer leaves it alone (Addresses). In-process only, like
	// Acknowledge.
	ReplayID uint64 `json:"-"`
}

// Addresses reports whether the writer holding token consumes b. A writer
// that holds no outstanding replay request passes 0. An incremental batch
// addresses every writer; a Broadcast batch too; a targeted batch addresses
// only the writer whose request carried its token, so a kernel writer running
// beside the VPP one never re-installs what it already holds and never
// acknowledges what it did not program.
func (b *EntryBatch) Addresses(token uint64) bool {
	if !replay.IsReplay(b.ReplayID) {
		return true
	}
	if b.ReplayID == replay.Broadcast {
		return true
	}
	return b.ReplayID == token
}

// EntryChange is the typed handle for (mpls-fib, entry). Producers Apply; the
// writer of each data plane (fib-kernel, fib-vpp) Subscribes.
var EntryChange = events.Register[*EntryBatch](Namespace, EventEntry)

// EventReplayRequest is the event type a label writer emits to ask the
// producers to re-apply every entry they want installed.
const EventReplayRequest = "replay-request"

// ReplayRequest is the typed handle for (mpls-fib, replay-request). A label
// writer emits it with a token from NewReplayToken once its subscription to
// EntryChange exists and it can program; each producer answers with
// ApplyReplay, tagged with that token. A request whose token is zero asks for
// nothing and is ignored.
var ReplayRequest = events.Register[*replay.Request](Namespace, EventReplayRequest)

// replayTokens hands out targeted replay tokens for the whole process. It
// starts at 1 so a token is never 0 (incremental) and never reaches
// replay.Broadcast in any realistic run: the redistribute orchestrator's
// pattern.
var replayTokens atomic.Uint64

// NewReplayToken returns a targeted replay token no other caller in the
// process holds. Safe for concurrent use.
func NewReplayToken() uint64 {
	return replayTokens.Add(1)
}

var errNoOwnerAcknowledgement = errors.New("mpls-fib: no native forwarding owner acknowledged the batch")

// Apply returns only after the native forwarding owner has attempted the batch.
// A successful Emit is not an installation acknowledgement. A batch may be
// partially applied on error; producers must not advertise the failed result.
func Apply(bus ze.EventBus, entries []Entry) error {
	return apply(bus, 0, entries)
}

// ApplyReplay answers the replay request that carried token: it applies
// entries with the batch tagged by token, so only the writer that asked
// consumes and acknowledges it. It fails as Apply does, and also when no
// writer holds token any more (the writer closed after it asked).
func ApplyReplay(bus ze.EventBus, token uint64, entries []Entry) error {
	if !replay.IsReplay(token) {
		return errors.New("mpls-fib: a replay answer needs the request's nonzero token")
	}
	return apply(bus, token, entries)
}

func apply(bus ze.EventBus, token uint64, entries []Entry) error {
	if bus == nil {
		return errNoOwnerAcknowledgement
	}
	var result error
	acknowledged := false
	batch := &EntryBatch{
		Entries:  entries,
		ReplayID: token,
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
