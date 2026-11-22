// Design: docs/architecture/plugin/rib-storage-design.md -- FlowSpec authorization across the process boundary
// Related: rib.go -- runRIBPlugin installs this bus when the RIB runs forked
// Related: rib_bestchange.go -- publishBestChanges, replayBestPaths, replayRedistribute
// Related: rib_flowspec_validation.go -- emitFlowSpecChanges, emitFlowSpec

package rib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// sdkEventEmitTimeout bounds one emitted frame: the engine enqueues it to each
// subscriber and answers, so only a dead engine connection reaches it. It is
// per frame, never per hand-off: a whole family is many frames, and a live
// engine acknowledging each one in turn is never cut off mid-family.
const sdkEventEmitTimeout = 30 * time.Second

// errSDKBusSubscribe is the refusal a forked RIB's bus gives a subscriber:
// what this RIB hears arrives through its startup subscriptions instead.
var errSDKBusSubscribe = errors.New("bgp-rib: a forked RIB subscribes through its startup subscriptions, not the event bus")

// sdkEventBus is the event bus of a bgp-rib running forked, where no daemon
// bus exists (ConfigureEventBus runs only in the daemon process). Emit hands
// the payload to the engine as emit-event through the SDK, which decodes it
// into the event's registered type for the daemon's subscribers and passes
// the JSON to every plugin-process subscriber: the same events the daemon bus
// carries for an in-daemon RIB, reaching the same subscribers. Safe for
// concurrent use: emitMu keeps every chunk of one emission together.
type sdkEventBus struct {
	plugin *sdk.Plugin
	emitMu sync.Mutex
}

var _ ze.EventBus = (*sdkEventBus)(nil)

// Emit preserves each domain batch's shape while cutting oversized payloads
// into ordered frames. Each frame gets its own sdkEventEmitTimeout, so a dead
// engine connection fails the hand-off within one frame's deadline while a
// live engine is never cut off by the size of the family. An acknowledged
// prefix is never retried when a later chunk fails.
func (b *sdkEventBus) Emit(namespace, eventType string, payload any) (int, error) {
	return b.emitFrames(sdkEventEmitTimeout, namespace, eventType, payload)
}

// emitFrames is Emit with its per-frame deadline named, so a test proves the
// bound without waiting sdkEventEmitTimeout.
func (b *sdkEventBus) emitFrames(frameTimeout time.Duration, namespace, eventType string, payload any) (int, error) {
	b.emitMu.Lock()
	defer b.emitMu.Unlock()
	event, err := sdkEventPayload(payload)
	if err != nil {
		return 0, fmt.Errorf("encode %s/%s: %w", namespace, eventType, err)
	}
	input := rpc.EmitEventInput{Namespace: namespace, EventType: eventType, Event: event}
	entries := sdkEventEntries(payload)
	if entries < 2 {
		// Unknown shapes and indivisible values keep the actual writer's
		// decision, not a refusal inferred from a widest-ID size proof.
		return b.emitSDKEvent(frameTimeout, &input)
	}
	budget := input.CanonicalPayloadBudget()
	if len(event) <= budget && entries <= ribevents.FrameEntriesMax {
		return b.emitSDKEvent(frameTimeout, &input)
	}
	open, close, err := sdkEventArray(payload, event)
	if err != nil {
		return 0, err
	}
	prefix, suffix := event[:open+1], event[close:]
	available := budget - len(prefix) - len(suffix)
	if available <= 0 {
		return b.emitSDKEvent(frameTimeout, &input)
	}
	contents := event[open+1 : close]
	delivered := 0
	var chunk textbuf.Buffer
	for start := 0; start < len(contents); {
		end := sdkEventChunkEnd(contents, start, available, ribevents.FrameEntriesMax)
		size := len(prefix) + end - start + len(suffix)
		if size > rpc.MaxMessageSize {
			return delivered, fmt.Errorf("event has %d canonical JSON bytes before RPC framing, beyond the %d-byte frame ceiling", size, rpc.MaxMessageSize)
		}
		// String transfers ownership. A queued DirectBridge subscriber may
		// still read this payload after the emission has been acknowledged.
		input.Event = chunk.Reset(size).Str(prefix).Str(contents[start:end]).Str(suffix).String()
		count, err := b.emitSDKEvent(frameTimeout, &input)
		if err != nil {
			return delivered, err
		}
		delivered += count
		start = end
		if start < len(contents) {
			start++ // The comma between this chunk and the next.
		}
	}
	return delivered, nil
}

// emitSDKEvent hands one frame to the engine under a deadline of its own.
func (b *sdkEventBus) emitSDKEvent(frameTimeout time.Duration, input *rpc.EmitEventInput) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), frameTimeout)
	defer cancel()
	delivered, err := b.plugin.EmitEvent(ctx, input.Namespace, input.EventType, "", "", input.Event)
	if err != nil {
		return delivered, fmt.Errorf("emit %s/%s: %w", input.Namespace, input.EventType, err)
	}
	return delivered, nil
}

// sdkEventEntries names only this producer's existing batch shapes. Arbitrary
// events stay indivisible; infrastructure never guesses how to split a payload.
func sdkEventEntries(payload any) int {
	switch batch := payload.(type) {
	case *ribevents.BestChangeBatch:
		if batch == nil {
			return 0
		}
		return len(batch.Changes)
	case *redistevents.RouteChangeBatch:
		if batch == nil {
			return 0
		}
		return len(batch.Entries)
	case []ribevents.ValidationRoute:
		return len(batch)
	}
	return 0
}

// sdkEventArray locates only the known domain array in JSON we just marshaled.
// Header decoding never walks or allocates a result for any route. The original
// prefix and suffix, including all replay metadata, are reused verbatim.
func sdkEventArray(payload any, event string) (int, int, error) {
	var name string
	switch payload.(type) {
	case *ribevents.BestChangeBatch:
		name = "changes"
	case *redistevents.RouteChangeBatch:
		name = "Entries"
	case []ribevents.ValidationRoute:
		close, err := sdkEventArrayClose(event, 0)
		return 0, close, err
	default:
		return 0, 0, errors.New("bgp-rib: event has no domain array")
	}
	decoder := json.NewDecoder(strings.NewReader(event))
	token, err := decoder.Token()
	if err != nil {
		return 0, 0, err
	}
	if token != json.Delim('{') {
		return 0, 0, errors.New("bgp-rib: domain batch is not a JSON object")
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return 0, 0, err
		}
		if key == name {
			token, err := decoder.Token()
			if err != nil {
				return 0, 0, err
			}
			if token != json.Delim('[') {
				return 0, 0, errors.New("bgp-rib: domain entries are not a JSON array")
			}
			open := int(decoder.InputOffset()) - 1
			close, err := sdkEventArrayClose(event, open)
			return open, close, err
		}
		var header json.RawMessage
		if err := decoder.Decode(&header); err != nil {
			return 0, 0, err
		}
	}
	return 0, 0, errors.New("bgp-rib: domain entries are absent from the encoded batch")
}

// sdkEventArrayClose walks canonical, already-valid JSON, never peer text.
// Strings may contain escaped brackets; only structural delimiters count.
func sdkEventArrayClose(event string, open int) (int, error) {
	depth := 1
	quoted := false
	for i := open + 1; i < len(event); i++ {
		if quoted {
			if event[i] == '\\' {
				i++
				continue
			}
			if event[i] == '"' {
				quoted = false
			}
			continue
		}
		switch event[i] {
		case '"':
			quoted = true
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, errors.New("bgp-rib: encoded domain array has no closing bracket")
}

// sdkEventChunkEnd cuts only between complete canonical entries. An indivisible
// entry wider than the conservative budget travels alone to the actual writer.
// The walk is bounded by contents and never decodes or re-encodes a route. A
// chunk ends at entriesMax entries even when bytes remain, so a subscriber's
// work per frame stays bounded.
func sdkEventChunkEnd(contents string, start, budget, entriesMax int) int {
	depth := 0
	quoted := false
	last := start
	entries := 0
	for i := start; i < len(contents); i++ {
		if quoted {
			if contents[i] == '\\' {
				i++
				continue
			}
			if contents[i] == '"' {
				quoted = false
			}
			continue
		}
		switch contents[i] {
		case '"':
			quoted = true
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		case ',':
			if depth != 0 {
				continue
			}
			if i-start > budget {
				if last > start {
					return last
				}
				return i
			}
			last = i
			entries++ // The entry this comma closes.
			if entries == entriesMax {
				return i
			}
		}
	}
	if len(contents)-start > budget {
		if last > start {
			return last
		}
	}
	return len(contents)
}

// Subscribe refuses: a forked RIB hears its events through the startup
// subscriptions runRIBPlugin declares, so a bus subscription here is a defect
// that would otherwise never deliver.
func (b *sdkEventBus) Subscribe(namespace, eventType string, _ func(payload any)) func() {
	logger().Error("bgp-rib: event bus subscription refused", "namespace", namespace, "event-type", eventType, "error", errSDKBusSubscribe)
	return func() {}
}

// sdkEventPayload is the JSON text of payload, as the daemon bus produces it
// for a plugin-process subscriber.
func sdkEventPayload(payload any) (string, error) {
	if text, ok := payload.(string); ok {
		return text, nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// dispatchReplayRequest answers an enveloped replay request and reports
// whether text was one: bgp-rib/replay-request replays the whole best-path
// table on the broadcast hop (replayBestPaths), redistribute/replay-request
// on the redistribution hop for one target (replayRedistribute). Both arrive
// through the startup subscriptions, so a forked RIB answers them as an
// in-daemon one does.
func (r *RIBManager) dispatchReplayRequest(text string) bool {
	if text == "" {
		return false
	}
	if text[0] != '{' {
		return false
	}
	envelope, err := rpc.ParseEventEnvelope(text)
	if err != nil {
		return false //nolint:nilerr // a BGP event is not an envelope: not this event, nothing refused
	}
	if envelope.Event != ribevents.EventReplayRequest {
		return false
	}
	var req replay.Request
	if err := json.Unmarshal(envelope.Payload, &req); err != nil {
		logger().Warn("bgp-rib: replay-request refused", "namespace", envelope.Namespace, "error", err)
		return true
	}
	switch envelope.Namespace {
	case ribevents.Namespace:
		r.replayBestPaths(&req)
		return true
	case redistevents.ReplayNamespace:
		r.replayRedistribute(&req)
		return true
	}
	return false
}
