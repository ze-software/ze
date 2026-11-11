// Design: docs/architecture/mpls/mpls-kernel.md -- the one label validation every label writer applies
// Related: events.go -- Entry, the value this file judges
// Related: pathmtu.go -- MaxLabelStack, the stack depth bound
// RFC: rfc/short/rfc3031.md
// RFC: rfc/short/rfc3032.md

package mplsfib

import (
	"errors"
	"fmt"
)

// MaxLabel is the largest label value a label stack entry can carry.
//
// RFC 3032 Section 2.1: "Label:  Label Value, 20 bits".
const MaxLabel uint32 = 1<<20 - 1

// ErrEmptyLabelStack refuses a swap, or a labeled route, that imposes no label.
var ErrEmptyLabelStack = errors.New("mpls: empty label stack")

// ValidateOutLabels judges an outgoing label stack: one to MaxLabelStack
// labels, each within 20 bits. Every label writer (fib-kernel, fib-vpp) calls
// it, for a swap entry, a labeled push entry and a labeled best change alike,
// so the rule is declared once. An empty stack is refused: a caller that
// accepts one (an Implicit NULL push) decides that before it calls.
func ValidateOutLabels(labels []uint32) error {
	if len(labels) == 0 {
		return ErrEmptyLabelStack
	}
	// MaxLabelStack also sizes the path MTU floor producers apply.
	if len(labels) > MaxLabelStack {
		return fmt.Errorf("mpls: label stack depth %d exceeds limit %d", len(labels), MaxLabelStack)
	}
	for _, label := range labels {
		if label > MaxLabel {
			return fmt.Errorf("mpls: label %d exceeds 20-bit maximum %d", label, MaxLabel)
		}
	}
	return nil
}

// ValidateEntry judges one entry with the rules every label writer shares,
// before the writer looks at its own ownership map or programs anything. A
// writer adds only what its own data plane cannot do (fib-kernel's private
// table namespace, fib-vpp's refusal of a private table) after this returns
// nil.
//
// An add is judged in full. A remove is judged on its key alone, because it
// carries no stack. ActionRemoveLabelSource names only a source and passes.
func ValidateEntry(e *Entry) error {
	switch e.Action {
	case ActionAdd:
		if err := validateEntryKey(e); err != nil {
			return err
		}
		return validateEntryStack(e)
	case ActionRemove:
		return validateEntryKey(e)
	case ActionRemoveLabelSource:
		return nil
	case ActionUnspecified:
		return fmt.Errorf("mpls-fib: unsupported entry action %d", e.Action)
	default:
		// Entry actions are open inputs from MPLS forwarding producers.
		return fmt.Errorf("mpls-fib: unsupported entry action %d", e.Action)
	}
}

// validateEntryKey judges what identifies an entry: a swap or pop is keyed by
// its incoming label in the one label space, so it carries no table and a
// label within 20 bits. A push is keyed by its FEC and table, which the writer
// judges against what its data plane can hold.
func validateEntryKey(e *Entry) error {
	switch e.Op {
	case OpPush:
		return nil
	case OpSwap, OpPop:
		if e.TableID != 0 {
			return errors.New("mpls-fib: swap/pop require table zero")
		}
		if e.InLabel > MaxLabel {
			return fmt.Errorf("mpls-fib: in-label %d exceeds 20 bits", e.InLabel)
		}
		return nil
	case OpUnspecified:
		return fmt.Errorf("mpls-fib: unsupported label operation %d", e.Op)
	default:
		// Label operations are open inputs from MPLS forwarding producers.
		return fmt.Errorf("mpls-fib: unsupported label operation %d", e.Op)
	}
}

// validateEntryStack judges the outgoing stack an add imposes.
func validateEntryStack(e *Entry) error {
	switch e.Op {
	case OpPush:
		// RFC 3032 Section 2.1: Implicit NULL is a label "which never actually
		// appears in the encapsulation". An empty push stack forwards the FEC
		// as plain IP toward the next hop, so only a non-empty one is judged.
		if len(e.OutLabels) == 0 {
			return nil
		}
		return ValidateOutLabels(e.OutLabels)
	case OpSwap:
		// RFC 3031 Section 3.10: "replace the label at the top of the label
		// stack with a specified new label, and then push one or more
		// specified new labels onto the label stack."
		return ValidateOutLabels(e.OutLabels)
	case OpPop:
		// RFC 3031 Section 3.10: "pop the label stack". A pop imposes nothing,
		// so a pop that carries an out-label is a producer defect.
		if len(e.OutLabels) != 0 {
			return fmt.Errorf("mpls-fib: pop of in-label %d carries %d out-labels", e.InLabel, len(e.OutLabels))
		}
		return nil
	case OpUnspecified:
		return fmt.Errorf("mpls-fib: unsupported label operation %d", e.Op)
	default:
		// Label operations are open inputs from MPLS forwarding producers.
		return fmt.Errorf("mpls-fib: unsupported label operation %d", e.Op)
	}
}
