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
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// sdkEventEmitTimeout bounds one event hand-off to the engine. The engine
// enqueues to each subscriber and answers, so only a dead engine connection
// reaches it.
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
// concurrent use: the SDK serializes its engine calls.
type sdkEventBus struct {
	plugin *sdk.Plugin
}

var _ ze.EventBus = (*sdkEventBus)(nil)

// Emit marshals payload once (a string passes through, as the daemon bus
// treats it) and emits it under namespace and eventType.
func (b *sdkEventBus) Emit(namespace, eventType string, payload any) (int, error) {
	event, err := sdkEventPayload(payload)
	if err != nil {
		return 0, fmt.Errorf("encode %s/%s: %w", namespace, eventType, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), sdkEventEmitTimeout)
	defer cancel()
	delivered, err := b.plugin.EmitEvent(ctx, namespace, eventType, "", "", event)
	if err != nil {
		return delivered, fmt.Errorf("emit %s/%s: %w", namespace, eventType, err)
	}
	return delivered, nil
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
