// Design: docs/architecture/api/process-protocol.md -- BGP UPDATE forwarding receipts.
// Related: events.go -- per-message subscription formatting and enqueue.
package server

import (
	"encoding/json"
	"fmt"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// updateEnvelopeCache serializes each envelope once per format/encoding and its
// receipt once per message. It borrows RawMessage only while wrap runs; queued
// strings own their complete payload and receipt. Not safe for concurrent use.
type updateEnvelopeCache struct {
	outputs  formatCache
	metadata json.RawMessage
}

func (cache *updateEnvelopeCache) reset() {
	cache.outputs.reset()
	cache.metadata = nil
}

func (cache *updateEnvelopeCache) wrap(peer *plugin.PeerInfo, message *bgptypes.RawMessage, subscriber *process.Process, output string) (string, error) {
	if message.Type != msgtype.TypeUPDATE {
		return output, nil
	}
	if !subscriber.Envelope() {
		return output, nil
	}
	key := subscriber.FormatCacheKey()
	if encoded, ok := cache.outputs.get(key); ok {
		return encoded, nil
	}
	if cache.metadata == nil {
		// Marshal the engine's own receipt, never message.Meta. The list is
		// consumed synchronously here; no slice alias crosses the queue.
		metadata, err := json.Marshal(bgptypes.UpdateReceipt{
			Peer: peer.Address, MessageID: message.MessageID,
			SourcePolicyEpoch: message.SourcePolicyEpoch,
			ReactorForwarded:  message.ReactorForwarded, FastPathSkipped: message.FastPathSkipped,
		})
		if err != nil {
			return "", fmt.Errorf("marshal BGP UPDATE receipt: %w", err)
		}
		cache.metadata = metadata
	}
	var payload json.RawMessage
	if subscriber.Encoding() == plugin.EncodingText {
		encoded, err := json.Marshal(output)
		if err != nil {
			return "", fmt.Errorf("quote BGP UPDATE text payload: %w", err)
		}
		payload = encoded
	} else {
		payload = json.RawMessage(output)
	}
	encoded, err := json.Marshal(rpc.EventEnvelope{
		Namespace: bgpevents.Namespace, Event: bgpevents.EventUpdate,
		Payload: payload, Metadata: cache.metadata,
	})
	if err != nil {
		return "", fmt.Errorf("marshal BGP UPDATE envelope: %w", err)
	}
	result := string(encoded)
	cache.outputs.set(key, result)
	return result, nil
}
