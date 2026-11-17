// Design: docs/architecture/plugin/rib-storage-design.md -- bounded settlement failures.
// Related: rib_selected_barrier.go -- the immutable unresolved snapshot.
package rib

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type selectedUnsettled struct {
	id    selectedDestinationID
	cause error
}

// selectedBarrierCause renders a bounded projection while retaining every
// original cause for local errors.Is/As. It owns no destination or route state.
// The barrier MUST supply its immutable snapshot; Error MUST NOT reread workers.
type selectedBarrierCause struct {
	text  string
	cause error
}

func (cause *selectedBarrierCause) Error() string { return cause.text }

func (cause *selectedBarrierCause) Unwrap() error { return cause.cause }

// selectedBarrierError MUST receive the barrier's immutable unresolved snapshot.
// The snapshot bounds both loops by that barrier's admitted destinations. Only
// the rendered receipt is bounded by bytes; no owed identity or cause is dropped.
func selectedBarrierError(cause error, unresolved []selectedUnsettled) error {
	const (
		problem = "BGP selected output did not settle"
		why     = "The change is incomplete because selected output remains owed"
		fix     = "ze show bgp rib status"
	)
	sort.Slice(unresolved, func(i, j int) bool {
		comparison := unresolved[i].id.peer.Compare(unresolved[j].id.peer)
		if comparison == 0 {
			return unresolved[i].id.session < unresolved[j].id.session
		}
		return comparison < 0
	})
	causes := make([]error, 0, len(unresolved)+1)
	causes = append(causes, cause)
	for _, destination := range unresolved {
		causes = append(causes, fmt.Errorf("destination %s Session %d: %w", destination.id.peer, destination.id.session, destination.cause))
	}
	complete := errors.Join(causes...)
	answer := opererr.Wrap(complete, problem, why, fix)
	if len(answer.Error()) <= rpc.EventSettlementErrorBytesMax {
		return answer
	}

	// Reserve the complete operator envelope and the largest possible omission
	// count before adding whole identities. Never cut an address, Session or cause.
	envelope := len(opererr.Wrap(errors.New(""), problem, why, fix).Error())
	header := cause.Error() + "\n"
	summary := selectedBarrierOmission(len(unresolved), len(unresolved))
	remaining := rpc.EventSettlementErrorBytesMax - envelope - len(header) - len(summary)
	var names strings.Builder
	names.Grow(remaining)
	shown := 0
	for _, destination := range unresolved {
		name := fmt.Sprintf("destination %s Session %d\n", destination.id.peer, destination.id.session)
		if len(name) > remaining {
			break
		}
		names.WriteString(name)
		remaining -= len(name)
		shown++
	}
	projection := &selectedBarrierCause{cause: complete,
		text: header + names.String() + selectedBarrierOmission(len(unresolved)-shown, len(unresolved))}
	return opererr.Wrap(projection, problem, why, fix)
}

func selectedBarrierOmission(identities, details int) string {
	return fmt.Sprintf("%d additional destination identities omitted; %d failure details omitted. Read selected-output in the current RIB status for all live destinations.", identities, details)
}
