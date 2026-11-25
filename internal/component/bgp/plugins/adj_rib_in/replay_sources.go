// Design: docs/architecture/plugin/rib-storage-design.md -- original source catch-up before replay.
// Related: source_state.go -- committed progress of each original physical ingress.
package adj_rib_in

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// replaySourceProgressCommand reads only committed source progress. No route
// walk or relay runs before the owner knows its required sources are covered.
func (r *AdjRIBInManager) replaySourceProgressCommand(cursor uint64, payload string) (string, any, error) {
	if cursor != 0 {
		return statusError, "", errors.New("adj-rib-in replay source progress requires cursor zero")
	}
	var sources []ribevents.ReplaySource
	if err := json.Unmarshal([]byte(payload), &sources); err != nil {
		return statusError, "", fmt.Errorf("decode adj-rib-in replay sources: %w", err)
	}
	if len(sources) > ribevents.ReplaySourceBatchMax {
		return statusError, "", errors.New("adj-rib-in replay source progress batch exceeds its limit")
	}
	for _, source := range sources {
		if !source.Address.IsValid() {
			return statusError, "", errors.New("adj-rib-in replay source progress requires a source address")
		}
		if source.PeerOwner == 0 {
			return statusError, "", errors.New("adj-rib-in replay source progress requires an original peer owner")
		}
		if source.ConnectionID == 0 {
			return statusError, "", errors.New("adj-rib-in replay source progress requires an original physical connection")
		}
		if source.MessageID == 0 {
			return statusError, "", errors.New("adj-rib-in replay source progress requires a received UPDATE ID")
		}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.terminal != nil {
		return statusError, "", r.terminal
	}
	covered := true
	for _, source := range sources {
		current, live := r.sourceLive[source.PeerOwner]
		if !live {
			covered = false
			continue
		}
		if current.address != source.Address {
			return statusError, "", errors.New("adj-rib-in replay source progress names a different original address")
		}
		if current.connectionID != source.ConnectionID {
			covered = false
			continue
		}
		if current.processedMsgID < source.MessageID {
			covered = false
		}
	}
	return statusDone, ribevents.ReplaySourceProgress{Covered: &covered}, nil
}
