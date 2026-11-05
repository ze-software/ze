// Design: docs/architecture/api/process-protocol.md -- bounded event emission.
// Related: batch.go -- the subscriber's deliver-batch framing.
package rpc

import (
	"encoding/json"
	"math"
)

// eventRequestPrefix is the widest correlation ID and the actual request
// writer's method/separator overhead, excluding the one-byte probe payload.
var eventRequestPrefix = len(AppendRequest(nil, math.MaxUint64, MethodEmitEvent, json.RawMessage("0"))) - 1

// eventDeliveryPrefix includes a singleton batch's syntax and widest ID,
// excluding its one-byte probe event and newline.
var eventDeliveryPrefix = eventDeliveryPrefixSize()

func eventDeliveryPrefixSize() int {
	var size eventFrameCounter
	if err := WriteBatchFrame(&size, math.MaxUint64, [][]byte{[]byte("0")}); err != nil {
		panic("BUG: measuring an event frame failed")
	}
	return int(size) - 2
}

var eventFramesFixed = eventFramesFixedSize()

func eventFramesFixedSize() int {
	// Unkeyed, nonempty probes deliberately fail compilation when either
	// schema grows. A new field MUST join the bound before this compiles.
	requestProbe := EmitEventInput{"x", "x", "x", "x", "null"}
	request, err := json.Marshal(&requestProbe)
	if err != nil {
		panic("BUG: measuring emit-event syntax failed")
	}
	envelopeProbe := EventEnvelope{"x", "x", json.RawMessage("null"), json.RawMessage("{}")}
	envelope, err := json.Marshal(&envelopeProbe)
	if err != nil {
		panic("BUG: measuring event envelope syntax failed")
	}
	quoted, err := json.Marshal(string(envelope))
	if err != nil {
		panic("BUG: measuring delivered event syntax failed")
	}
	return max(eventRequestPrefix+len(request), eventDeliveryPrefix+len(quoted))
}

type eventFrameCounter int

func (size *eventFrameCounter) Write(data []byte) (int, error) {
	*size += eventFrameCounter(len(data))
	return len(data), nil
}

// CanonicalPayloadBudget bounds a canonical JSON event's byte length for both
// its emit-event request and an enveloped singleton deliver-batch, including
// the widest correlation ID. It allocates nothing and does not raise a limit.
//
// The caller MUST supply encoding/json's HTML-escaped, valid-UTF-8 output.
// That output has escaped controls, HTML and U+2028/U+2029 already; subsequent
// string quoting at most doubles quotes and backslashes. All payload syntax,
// brackets and commas MUST fit this budget. A larger indivisible value is not
// proved unencodable: the actual writer, with its actual ID, decides it.
//
// input MUST be non-nil. EmitEventInput carries no receipt metadata, so the
// engine builds its EventEnvelope with Metadata nil. The nonempty metadata
// probe includes its syntax conservatively, not another producer's receipt.
// Safe for concurrent use; the returned budget borrows nothing.
func (input *EmitEventInput) CanonicalPayloadBudget() int {
	remaining := MaxMessageSize - eventFramesFixed
	// A metadata byte needs at most six bytes per quoting layer. Reserve two
	// layers for every field, even the fields not repeated in an envelope.
	for _, value := range [...]string{input.Namespace, input.EventType, input.Direction, input.PeerAddress} {
		if len(value) > remaining/36 {
			return 0
		}
		remaining -= 36 * len(value)
	}
	return remaining / 2
}
