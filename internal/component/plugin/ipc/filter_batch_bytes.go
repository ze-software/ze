// Design: docs/plugin-development/protocol.md -- The filter-update batch.
// Related: filter_batch.go -- the worker's queue, exchangeOne and reply ownership.
// Related: ../../../../pkg/plugin/rpc/filter_batch_size.go -- schema-owned JSON bound.
package ipc

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// filterBatchWireFixed includes the JSON envelope and the widest request ID,
// method and separating space. It excludes the routes and their commas.
var filterBatchWireFixed = filterBatchWireFixedSize()

func filterBatchWireFixedSize() int {
	empty, err := json.Marshal(&rpc.FilterUpdateBatchInput{Routes: []rpc.FilterUpdateInput{}})
	if err != nil {
		panic("BUG: filter-update batch size probe is not encodable")
	}
	var scratch [128]byte
	// Nonempty params make AppendRequest include the payload's separator;
	// MaxMessageSize excludes the newline, as AppendRequest does.
	prefix := rpc.AppendRequest(scratch[:0], math.MaxUint64, methodFilterUpdate, json.RawMessage("0"))
	return len(empty) + len(prefix) - 1
}

// exchange preserves the worker's FIFO group while splitting only when its
// serialized request might exceed the frame bound. Ordinary batches use the
// schema-owned length bound and are encoded only by CallRPC, as before.
func (b *filterBatcher) exchange(pc *PluginConn) {
	if b.frameFits() {
		b.exchangeOne(pc, b.requests)
		return
	}
	// Only the size-risk path measures canonical JSON. Keep lengths, not
	// another copy of the routes' encoded bodies, across exchanges.
	var sizes [rpc.FilterUpdateBatchMax]int
	for i, req := range b.requests {
		encoded, err := json.Marshal(req.input)
		if err != nil {
			answerFilterRequests(b.requests, fmt.Errorf("filter-update: the batch of %d routes failed: %w", len(b.requests), err))
			return
		}
		sizes[i] = len(encoded)
	}
	for start := 0; start < len(b.requests); {
		if sizes[start] > rpc.MaxMessageSize-filterBatchWireFixed {
			// The widest-ID bound cannot prove fit, but a shorter actual
			// ID may fit. Let the ordinary writer judge this singleton,
			// without contaminating either adjacent route's exchange.
			b.exchangeOne(pc, b.requests[start:start+1])
			start++
			continue
		}
		size := filterBatchWireFixed
		end := start
		for end < len(b.requests) {
			remaining := rpc.MaxMessageSize - size
			if end > start {
				remaining-- // The next route needs a comma.
			}
			if sizes[end] > remaining {
				break
			}
			size += sizes[end]
			if end > start {
				size++
			}
			end++
		}
		b.exchangeOne(pc, b.requests[start:end])
		start = end
	}
}

// frameFits proves fit without encoding any route a second time. Every sum is
// bounded before addition; a saturated schema bound selects exact sizing.
func (b *filterBatcher) frameFits() bool {
	size := filterBatchWireFixed
	for i, req := range b.requests {
		remaining := rpc.MaxMessageSize - size
		if i > 0 {
			remaining--
		}
		bound := req.input.JSONSizeBound()
		if bound > remaining {
			return false
		}
		size += bound
		if i > 0 {
			size++
		}
	}
	return true
}
