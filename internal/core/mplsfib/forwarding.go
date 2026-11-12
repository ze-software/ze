// Design: docs/architecture/mpls/mpls-kernel.md -- `show mpls forwarding` on the selected data plane
// Related: events.go -- the (mpls-fib, entry) events whose writer each reader reads back
//
// `show mpls forwarding` answers from the data plane the config selects, not
// from the kernel alone. The code that owns a data plane's table registers
// its reader here, under the name `interface { backend }` gives that data
// plane ("netlink", "vpp"), which is also the name its writer declares in
// Registration.DataPlane. The command looks the reader up by that name, so it
// names no FIB plugin and no backend, and a data plane added later needs no
// edit to the command.
//
// The registry lives in this core package, in the shape of
// internal/core/ipsecinventory, because the readers sit behind other compile
// gates: fib-vpp is ze_vpp, the command is ze_mpls, and neither may import
// the other.

package mplsfib

import (
	"errors"
	"sync"
)

// labelImplicitNull is the reserved label value 3. An outgoing stack that is
// only this label makes the data plane pop instead of swap.
const labelImplicitNull = 3

// ForwardingEntry is one row of `show mpls forwarding`, the same shape on
// every data plane so a script never asks which one is underneath. A swap or
// pop entry is keyed by InLabel; a push entry is keyed by FEC (the IP prefix
// whose traffic gets the label stack imposed) and has no InLabel. It is a
// value type: a reader hands it across no pointer into its own state.
type ForwardingEntry struct {
	InLabel   int    `json:"in-label,omitempty"`
	FEC       string `json:"fec,omitempty"`
	Operation string `json:"operation"`
	OutLabels []int  `json:"out-labels,omitempty"`
	NextHop   string `json:"next-hop,omitempty"`
	Device    string `json:"device,omitempty"`
}

// ForwardingReader reads at most limit entries of one data plane's MPLS
// forwarding table, limit <= 0 meaning no cap. A reader that cannot read the
// table MUST return an error naming the cause and the fix, never an empty
// table: an empty answer reads as "no label is installed".
type ForwardingReader func(limit int) ([]ForwardingEntry, error)

var (
	forwardingMu      sync.RWMutex
	forwardingReaders = map[string]ForwardingReader{}
)

var (
	errForwardingNoDataPlane = errors.New("mplsfib: a forwarding reader names no data plane")
	errForwardingNilReader   = errors.New("mplsfib: a forwarding reader is nil")
	errForwardingDuplicate   = errors.New("mplsfib: a second forwarding reader for one data plane")
)

// RegisterForwardingReader records reader as the one that reads dataPlane's
// MPLS forwarding table. It refuses an empty name, a nil reader and a second
// reader for one data plane: one data plane has one table, so which reader
// serves a deployment is never decided by link order. Called from init().
// Safe for concurrent use.
func RegisterForwardingReader(dataPlane string, reader ForwardingReader) error {
	if dataPlane == "" {
		return errForwardingNoDataPlane
	}
	if reader == nil {
		return errForwardingNilReader
	}
	forwardingMu.Lock()
	defer forwardingMu.Unlock()
	if _, taken := forwardingReaders[dataPlane]; taken {
		return errors.Join(errForwardingDuplicate, errors.New("data plane "+dataPlane+" already has one"))
	}
	forwardingReaders[dataPlane] = reader
	return nil
}

// ForwardingReaderFor answers the reader registered for dataPlane, and false
// when this build holds none: the data plane's owner was compiled out, or no
// owner reads its table. A caller MUST refuse on false rather than answer an
// empty table. Safe for concurrent use.
func ForwardingReaderFor(dataPlane string) (ForwardingReader, bool) {
	forwardingMu.RLock()
	defer forwardingMu.RUnlock()
	reader, ok := forwardingReaders[dataPlane]
	return reader, ok
}

// LabelOperation names what a swap or pop entry does with its in-label, from
// its outgoing stack: "pop" for no outgoing label or Implicit NULL alone,
// "swap" otherwise. Every reader calls it, so one entry reads the same on the
// kernel and on VPP.
//
// RFC 3032 Section 2.1: "When an LSR would otherwise replace the label at the top of the stack with a new label, but the new label is "Implicit NULL", the LSR will pop the stack instead of doing the replacement."
func LabelOperation(outLabels []int) string {
	if len(outLabels) == 0 {
		return "pop"
	}
	if len(outLabels) == 1 && outLabels[0] == labelImplicitNull {
		return "pop"
	}
	return "swap"
}
