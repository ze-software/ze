// Design: docs/architecture/core-design.md -- the IKE path probe leaf
// Related: internal/component/ike/engine/probe.go -- the one registrant
// Related: internal/core/ipsecinventory/registry.go -- the leaf shape this copies
//
// The IKE path probe is how the MTU diagnostic asks a live IKE SA to measure its
// path without importing the IKE engine. The engine registers one prober at init();
// a caller asks Probe with a peer name and a wire size and gets a typed outcome. The
// package imports the standard library and the core probe leaf only, so the engine
// registers from init() with no cycle, and a build that drops the IKE component still
// compiles every caller: that caller then gets ErrNotRegistered rather than a refusal
// (ai/rules/principles.md).

package ikeprobe

import (
	"context"
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/core/probe"
)

// Outcome is what one padded INFORMATIONAL exchange answered. The zero value is an
// outcome nobody set, so a completed probe never carries it.
type Outcome uint8

const (
	OutcomeUnspecified Outcome = iota
	// OutcomeFits: the peer answered the copy that carried Don't Fragment, so a
	// datagram of the requested size crosses the path whole.
	OutcomeFits
	// OutcomeTooBig: the peer answered only after the copy sent with Don't Fragment
	// clear, so the requested size crosses the path fragmented.
	OutcomeTooBig
	// OutcomeSAFailed: no copy was answered inside the retransmit budget, so the IKE
	// SA was deemed failed (RFC 7296 Section 2.1) and the engine is re-establishing it.
	OutcomeSAFailed
	// OutcomeRefused: the engine declined to send, before anything was built. Refusal
	// names why.
	OutcomeRefused
)

// String is for display only; never compare with it.
func (o Outcome) String() string {
	switch o {
	case OutcomeFits:
		return "fits"
	case OutcomeTooBig:
		return "too-big"
	case OutcomeSAFailed:
		return "sa-failed"
	case OutcomeRefused:
		return "refused"
	default:
		return "unspecified"
	}
}

// Refusal is why the engine declined a request. It is meaningful only under
// OutcomeRefused. The zero value is a reason nobody set.
type Refusal uint8

const (
	RefusalUnspecified Refusal = iota
	// RefusalSADown: the peer has no established IKE SA, or is not configured, so
	// there is no owner loop to build on.
	RefusalSADown
	// RefusalRekeyPending: the owner loop has a CREATE_CHILD_SA request outstanding,
	// which holds the one request window (RFC 7296 Section 2.3).
	RefusalRekeyPending
	// RefusalRekeyHeld: the peer answered a rekey with TEMPORARY_FAILURE and the loop
	// is waiting the hold out (RFC 7296 Section 2.25); the peer is mid-rekey and a
	// strongSwan in IKE_REKEYED drops an INFORMATIONAL that is not a Delete.
	RefusalRekeyHeld
	// RefusalWindowHeld: another request (a Delete, a DPD probe, an earlier path
	// probe) holds the one request window (RFC 7296 Section 2.3).
	RefusalWindowHeld
	// RefusalSize: the requested wire size is below the smallest datagram the SA's
	// cipher suite can produce, above the ceiling, or not a size the suite's block
	// rounding can reach exactly.
	RefusalSize
	// RefusalFamily: the SA's peer address is not IPv4, the only family the IKE
	// transport sends on today.
	RefusalFamily
	// RefusalSendFailed: the request could not be built or written, so nothing left
	// the host, no message id was spent and the window was handed back.
	RefusalSendFailed
	// RefusalRekeyed: the request left on an IKE SA the peer then rekeyed, and the
	// retired SA cannot be answered on. The exchange was spent (WireOctets names the
	// size sent) and says nothing about the path; the same size is asked again on
	// the SA that replaced it.
	RefusalRekeyed
)

// String is for display only; never compare with it.
func (r Refusal) String() string {
	switch r {
	case RefusalSADown:
		return "sa-down"
	case RefusalRekeyPending:
		return "rekey-pending"
	case RefusalRekeyHeld:
		return "rekey-held"
	case RefusalWindowHeld:
		return "window-held"
	case RefusalSize:
		return "size"
	case RefusalFamily:
		return "family"
	case RefusalSendFailed:
		return "send-failed"
	case RefusalRekeyed:
		return "rekeyed"
	default:
		return "unspecified"
	}
}

// Request is one measurement the caller asks for: the configured peer name, the
// size of the whole datagram on the wire (IP header to the last octet of padding),
// and whether the kernel's cached path MTU is honored or bypassed.
//
// It is a value type: it crosses a component boundary and carries no pointer
// (ai/rules/plugins.md).
type Request struct {
	Peer       string
	WireOctets uint16
	DF         probe.DFMode
}

// Result is what one Request answered. Refusal holds a value only under
// OutcomeRefused. MTU is the next-hop MTU a router's Fragmentation Needed, or the
// kernel's own path cache, reported for the Don't Fragment copy; it is meaningful
// only under OutcomeTooBig and is 0 when no figure was reported.
type Result struct {
	Outcome Outcome
	Refusal Refusal
	// WireOctets is the size of the datagram the exchange sent, which is the size
	// the outcome is about. It is the requested size, or the largest size at or
	// below it the SA's cipher suite produces (a CBC suite sends on a 16-octet
	// grid); never larger. Zero under a refusal: nothing was sent.
	WireOctets uint16
	MTU        uint32
}

// Prober runs one padded exchange. It is called on the caller's goroutine and MUST
// hand the request to the SA's owner loop rather than touch SA state itself. An error
// is a failure to ask (the context ended); a refusal is a Result, never an error.
type Prober func(ctx context.Context, req Request) (Result, error)

// ErrNotRegistered is the answer when no IKE engine registered a prober: the
// component is not in this build. It is distinct from a registered engine refusing a
// request, which answers OutcomeRefused and no error.
var ErrNotRegistered = errors.New("no ike path probe is registered: the ike component is not in this build")

var (
	mu       sync.RWMutex
	provider Prober
)

// Register installs the one prober. It is called from an owner's init(), so a nil
// function or a second registrant is a programmer error and panics.
func Register(fn Prober) {
	if fn == nil {
		panic("BUG: ikeprobe.Register called with a nil prober")
	}
	mu.Lock()
	defer mu.Unlock()
	if provider != nil {
		panic("BUG: ikeprobe.Register called twice")
	}
	provider = fn
}

// Probe asks the registered prober, or answers ErrNotRegistered when no engine
// registered one. Safe for concurrent use.
func Probe(ctx context.Context, req Request) (Result, error) {
	mu.RLock()
	fn := provider
	mu.RUnlock()
	if fn == nil {
		return Result{}, ErrNotRegistered
	}
	return fn(ctx, req)
}

// ResetForTest clears the registration so a test can register its own prober.
func ResetForTest() {
	mu.Lock()
	provider = nil
	mu.Unlock()
}
