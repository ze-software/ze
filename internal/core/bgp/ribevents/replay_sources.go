// Design: docs/architecture/plugin/rib-storage-design.md -- original source catch-up before replay.
// Related: replay.go -- paginated reads after the store covers the captured sources.
package ribevents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

// ReplaySource identifies a physical ingress and its last required UPDATE.
// An ingress's serial read loop makes this a causal position for that ingress.
type ReplaySource struct {
	Address      netip.Addr `json:"address"`
	PeerOwner    uint64     `json:"peer-owner,string"`
	ConnectionID uint64     `json:"connection-id,string"`
	MessageID    uint64     `json:"msg-id,string"`
}

// ReplaySourceProgressArgument selects the projection-only replay command.
const ReplaySourceProgressArgument = "source-progress"

// ReplaySourceProgress answers whether every requested ingress is covered.
// Covered is always present in the producer's response.
type ReplaySourceProgress struct {
	Covered *bool `json:"covered"`
}

// ReplaySourceBatchMax bounds one source-progress projection near 120 KiB.
const ReplaySourceBatchMax = 1024

// ErrReplayStoreAbsent distinguishes an unregistered command from a configured
// store's refused or incompatible answer.
var ErrReplayStoreAbsent = errors.New("receive-store replay command is not registered")

// ReadReplaySourceProgress asks the actual receive store before any relay is
// admitted. Only a positive reply to every finite batch covers these sources.
func ReadReplaySourceProgress(ctx context.Context, dispatch ReplayDispatch, destination string, sources []ReplaySource) (bool, error) {
	for batch := range slices.Chunk(sources, ReplaySourceBatchMax) {
		payload, err := json.Marshal(batch)
		if err != nil {
			return false, fmt.Errorf("encode replay source progress: %w", err)
		}
		status, data, err := dispatch(ctx, ReplayRoutesCommand, destination, "0", ReplaySourceProgressArgument, string(payload))
		if err != nil {
			// Both dispatch transports carry this exact unregistered-command
			// diagnostic. A registered store can separately refuse its args.
			if strings.Contains(err.Error(), `plugin command "`+ReplayRoutesCommand+`" not registered`) {
				return false, ErrReplayStoreAbsent
			}
			return false, err
		}
		if status != "done" {
			return false, fmt.Errorf("replay source progress answered status %q, want done", status)
		}
		var progress ReplaySourceProgress
		if err := json.Unmarshal(data, &progress); err != nil {
			return false, fmt.Errorf("decode replay source progress: %w", err)
		}
		if progress.Covered == nil {
			return false, errors.New("receive store omitted replay source progress")
		}
		if !*progress.Covered {
			return false, nil
		}
	}
	return true, nil
}
