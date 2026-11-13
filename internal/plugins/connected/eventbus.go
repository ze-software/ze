// Design: docs/guide/redistribution.md -- connected startup inventory.
// Related: connected.go -- routeObserver and pooled route-change publication.

package connected

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/internal/core/diagnostic"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redistevents"
	connectedevents "github.com/ze-software/ze/internal/plugins/connected/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const connectedReplaySubject = "route-change replay"
const connectedReplayFix = "ze show log recent level warn component connected"

// followInterfaceAddresses installs the same event path in both transports.
// Startup subscriptions MUST be registered before Run; OnStarted then asks
// only an already-active address owner to replay its retained inventory.
func followInterfaceAddresses(ctx context.Context, p *sdk.Plugin, obs *routeObserver) {
	obs.publish = func(batch *redistevents.RouteChangeBatch) (int, error) {
		// This is the registered route-change payload, encoded as the engine's
		// payloadToJSON does. The JSON owns its bytes before emitID releases
		// the pooled batch, including when the engine queues event delivery.
		body, err := json.Marshal(batch)
		if err != nil {
			return 0, err
		}
		return p.EmitEvent(ctx, connectedevents.Namespace, redistevents.EventType, "", "", string(body))
	}
	// The observer serializes replay reporting with publication. Only
	// acknowledged transitions change this flag; a refused report remains an
	// event application error, and a repeated failure does not flood the bus.
	warningRaised := false
	obs.reportReplay = func(cause error) error {
		if cause == nil {
			if !warningRaised {
				return nil
			}
			if err := p.ClearIssue(ctx, diagnostic.CodeDoctorPluginPanic, connectedReplaySubject); err != nil {
				return opererr.Wrap(err, "Connected replay warning could not be cleared",
					"The replay succeeded, but the daemon did not acknowledge its recovery", connectedReplayFix)
			}
			warningRaised = false
			return nil
		}
		if warningRaised {
			return nil
		}
		issue := opererr.Wrap(cause, "Connected route replay failed",
			"Redistribution destinations may be missing interface prefixes; inspect the publication error", connectedReplayFix)
		if err := p.RaiseIssue(ctx, sdk.Issue{
			Severity: rpc.ReportWarning, Code: diagnostic.CodeDoctorPluginPanic,
			Subject: connectedReplaySubject, Message: issue.Error(), Fix: issue.Fix(),
		}); err != nil {
			return opererr.Wrap(err, "Connected replay warning could not be raised",
				"The daemon did not acknowledge the diagnostic for the failed replay", connectedReplayFix)
		}
		warningRaised = true
		return nil
	}
	p.SetStartupSubscriptionsIn(ifaceevents.Namespace,
		[]string{ifaceevents.EventAddrAdded, ifaceevents.EventAddrRemoved}, nil, "")
	p.AddStartupSubscriptionsIn(redistevents.ReplayNamespace, []string{redistevents.ReplayRequestEventType})
	p.SetEnvelope(true)
	p.OnEvent(obs.handleEvent)
	p.OnBye(func(string) error { return obs.retire() })
	p.OnStarted(func(started context.Context) error {
		// Delivered counts process subscribers, not engine-side owner
		// callbacks. Zero therefore says nothing about provider presence or
		// inventory completion. There is no wait for a nonexistent provider.
		_, err := p.EmitEvent(started, ifaceevents.Namespace, ifaceevents.EventAddressReplayRequest, "", "", "")
		return err
	})
}

func (o *routeObserver) handleEvent(text string) error {
	envelope, err := rpc.ParseEventEnvelope(text)
	if err != nil {
		return fmt.Errorf("decode connected event envelope: %w", err)
	}
	if envelope.Namespace == ifaceevents.Namespace {
		switch envelope.Event {
		case ifaceevents.EventAddrAdded:
			o.handleAddrAdded([]byte(envelope.Payload))
			return nil
		case ifaceevents.EventAddrRemoved:
			o.handleAddrRemoved([]byte(envelope.Payload))
			return nil
		}
	}
	if envelope.Namespace == redistevents.ReplayNamespace {
		if envelope.Event == redistevents.ReplayRequestEventType {
			var request redistevents.ReplayRequest
			if err := json.Unmarshal(envelope.Payload, &request); err != nil {
				return fmt.Errorf("decode connected redistribution replay: %w", err)
			}
			return o.reemitAll(request.ReplayID)
		}
	}
	return fmt.Errorf("connected received an unsubscribed event %s/%s", envelope.Namespace, envelope.Event)
}
