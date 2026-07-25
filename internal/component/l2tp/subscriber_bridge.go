// Design: plan/learned/760-subscriber-session-model.md -- L2TP subscriber event bridge
// Related: reactor_kernel.go -- emits L2TP session events

package l2tp

import (
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/ze"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

type subscriberBridge struct {
	registry *subscriber.Registry
	bus      ze.EventBus
	logger   *slog.Logger
	unsubs   []func()
}

func newSubscriberBridge(registry *subscriber.Registry, bus ze.EventBus, logger *slog.Logger) *subscriberBridge {
	b := &subscriberBridge{
		registry: registry,
		bus:      bus,
		logger:   logger,
	}
	b.subscribe()
	return b
}

func (b *subscriberBridge) subscribe() {
	b.unsubs = append(b.unsubs,
		l2tpevents.SessionUp.Subscribe(b.bus, b.onSessionUp),
		l2tpevents.SessionDown.Subscribe(b.bus, b.onSessionDown),
		l2tpevents.SessionIPAssigned.Subscribe(b.bus, b.onSessionIPAssigned),
		subevents.SessionAuthResult.Subscribe(b.bus, b.onAuthResult),
	)
}

func (b *subscriberBridge) stop() {
	for _, unsub := range b.unsubs {
		unsub()
	}
	b.unsubs = nil
}

func l2tpSessionID(tunnelID, sessionID uint16) string {
	var buf textbuf.Buffer
	return buf.Reset().Str("l2tp-").Uint16(tunnelID).Byte('-').Uint16(sessionID).String()
}

func (b *subscriberBridge) onSessionUp(p *l2tpevents.SessionUpPayload) {
	sess := subscriber.Session{
		ID:              l2tpSessionID(p.TunnelID, p.SessionID),
		AccessType:      subscriber.AccessL2TP,
		State:           subscriber.StateActive,
		TunnelID:        p.TunnelID,
		SessionID:       p.SessionID,
		PppInterface:    p.Interface,
		AccessInterface: p.AccessInterface,
		ActivatedAt:     time.Now(),
	}
	if meta := LoadSessionMetadata(p.TunnelID, p.SessionID); meta != nil {
		sess.PoolName = meta.FramedPool
	}
	sess.AcctSessionID = sess.ID
	b.registry.Add(&sess)
	subscriber.RecordSessionUp(subscriber.AccessL2TP)

	if _, err := subevents.SessionUp.Emit(b.bus, &subevents.SessionUpPayload{
		Session: sess,
	}); err != nil {
		b.logger.Warn("l2tp: subscriber session-up emit failed", "error", err)
	}
}

func (b *subscriberBridge) onSessionDown(p *l2tpevents.SessionDownPayload) {
	id := l2tpSessionID(p.TunnelID, p.SessionID)
	sess, ok := b.registry.Get(id)
	if !ok {
		sess = subscriber.Session{
			ID:         id,
			AccessType: subscriber.AccessL2TP,
			TunnelID:   p.TunnelID,
			SessionID:  p.SessionID,
			Username:   p.Username,
		}
	}
	sess.State = subscriber.StateTerminating
	b.registry.Remove(id)
	if ok {
		subscriber.RecordSessionDown(subscriber.AccessL2TP)
	}

	if _, err := subevents.SessionDown.Emit(b.bus, &subevents.SessionDownPayload{
		Session: sess,
		Reason:  "session-down",
	}); err != nil {
		b.logger.Warn("l2tp: subscriber session-down emit failed", "error", err)
	}
}

func (b *subscriberBridge) onAuthResult(p *subevents.SessionAuthResultPayload) {
	if p.SessionID == "" || !p.Accept {
		return
	}
	sess, ok := b.registry.Get(p.SessionID)
	if !ok {
		return
	}
	if p.Username != "" {
		sess.Username = p.Username
	}
	b.registry.Add(&sess)
}

func (b *subscriberBridge) onSessionIPAssigned(p *l2tpevents.SessionIPAssignedPayload) {
	id := l2tpSessionID(p.TunnelID, p.SessionID)
	sess, ok := b.registry.Get(id)
	if !ok {
		return
	}
	sess.Username = p.Username
	sess.PppInterface = p.PppInterface
	b.registry.Add(&sess)

	if _, err := subevents.SessionIPAssigned.Emit(b.bus, &subevents.SessionIPAssignedPayload{
		Session: sess,
	}); err != nil {
		b.logger.Warn("l2tp: subscriber session-ip-assigned emit failed", "error", err)
	}
}
