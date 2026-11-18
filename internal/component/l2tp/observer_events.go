// Design: docs/architecture/l2tp/l2tp-9-observer.md -- original-generation delivery admission
// Related: observer.go -- bounded rings and login contributors

package l2tp

import (
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/subscriberid"
	"github.com/ze-software/ze/pkg/ze"
)

type observerAdmission uint8

const (
	observerUnspecified observerAdmission = iota
	observerOpen
	observerRetained
	observerClosed
)

func validateObserverConfig(cfg observerConfig) error {
	if !cfg.Generation.IsValid() {
		return opererr.New("L2TP session monitoring could not start", "The original observation scope is absent", "ze doctor")
	}
	if cfg.MaxSessions < 0 {
		return opererr.New("L2TP session monitoring could not start", "Monitoring buffer sizes cannot be negative", "ze doctor")
	}
	if cfg.MaxLogins < 0 {
		return opererr.New("L2TP session monitoring could not start", "Monitoring buffer sizes cannot be negative", "ze doctor")
	}
	if cfg.EventRingSize < 0 {
		return opererr.New("L2TP session monitoring could not start", "Monitoring buffer sizes cannot be negative", "ze doctor")
	}
	if cfg.BucketCount < 0 {
		return opererr.New("L2TP session monitoring could not start", "Monitoring buffer sizes cannot be negative", "ze doctor")
	}
	if cfg.MaxSessions > 0 {
		if cfg.EventRingSize == 0 {
			return opererr.New("L2TP session monitoring could not start", "Enabled monitoring buffers require a positive ring size", "ze doctor")
		}
	}
	if cfg.MaxLogins > 0 {
		if cfg.BucketCount == 0 {
			return opererr.New("L2TP session monitoring could not start", "Enabled monitoring buffers require a positive ring size", "ze doctor")
		}
	}
	return nil
}

// observeEvent admits the entire delivery under the existing state lock.
// Callers MUST supply original generation and Birth; effects MUST remain inside
// this admission so Stop or a foreign delivery cannot slip between them.
func (o *Observer) observeEvent(generation l2tpevents.ObservationGeneration, ev ObserverEvent, login string) error {
	if o == nil {
		return nil
	}
	if !generation.IsValid() {
		return nil
	}
	if !ev.Birth.IsValid() {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.admission == observerUnspecified {
		return nil
	}
	if o.admission == observerClosed {
		return nil
	}
	if generation != o.generation {
		return nil
	}
	ring := o.sessions[ev.Birth]
	entry := o.logins[login]
	if o.admission == observerRetained {
		if ring == nil {
			// A full event pool can leave a useful selected quality contributor
			// without a ring. Its first Down MUST still finish that same phase.
			if ev.Type != ObserverEventSessionDown {
				return nil
			}
			if entry == nil {
				return nil
			}
			if entry.contributor != ev.Birth {
				return nil
			}
			if entry.downConsumed {
				return nil
			}
		}
	}
	if ev.Type == ObserverEventSessionDown {
		if ring != nil {
			ring.append(ev)
			o.releaseSessionLocked(ev.Birth)
		}
		if login == "" {
			return nil
		}
		if entry != nil {
			if entry.contributor != ev.Birth {
				return nil
			}
			if entry.downConsumed {
				return nil
			}
			o.promoteEntry(entry)
		} else {
			var err error
			entry, err = o.touchLogin(login, ev.Timestamp, ev.Birth)
			if err != nil {
				return err
			}
			if entry == nil {
				return nil
			}
		}
		entry.downConsumed = true
		entry.state = BucketStateDown
		entry.current.State = BucketStateDown
		return nil
	}
	o.recordEventLocked(ev)
	if login == "" {
		return nil
	}
	// Down was handled above; only assignment and echo change quality here.
	//exhaustive:ignore
	switch ev.Type {
	case ObserverEventSessionIPAssigned:
		entry, err := o.touchLogin(login, ev.Timestamp, ev.Birth)
		if err != nil {
			return err
		}
		if entry == nil {
			return nil
		}
		entry.contributor = ev.Birth
		entry.downConsumed = false
		entry.state = BucketStateEstablished
		entry.current.State = BucketStateEstablished
	case ObserverEventEchoRTT:
		entry, err := o.touchLogin(login, ev.Timestamp, ev.Birth)
		if err != nil {
			return err
		}
		if entry == nil {
			return nil
		}
		o.maybeCloseBucket(entry, ev.Timestamp)
		entry.current.addEcho(ev.RTT)
	}
	return nil
}

// recordEventLocked requires Observer.mu and an already admitted operation.
// Callers MUST establish its original scope; this helper MUST never look up labels.
func (o *Observer) recordEventLocked(ev ObserverEvent) {
	ring := o.sessions[ev.Birth]
	if ring == nil {
		ring = o.eventPool.acquire()
		if ring == nil {
			return
		}
		o.sessions[ev.Birth] = ring
	}
	ring.append(ev)
}

// recordDisconnect is optional CQM history for an already captured original.
// The capture MUST validate its row first; this lane MUST never admit a closed
// observer or change login quality, and is not a command-audit receipt.
func (o *Observer) recordDisconnect(generation l2tpevents.ObservationGeneration, birth subscriberid.L2TPBirth, tid, sid uint16, actor, reason string, cause uint32) {
	if o == nil {
		return
	}
	if !generation.IsValid() {
		return
	}
	if !birth.IsValid() {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if generation != o.generation {
		return
	}
	if o.admission == observerUnspecified {
		return
	}
	if o.admission == observerClosed {
		return
	}
	o.recordEventLocked(ObserverEvent{
		Birth: birth, Timestamp: time.Now(), Type: ObserverEventDisconnectRequested,
		TunnelID: tid, SessionID: sid, Actor: actor, Reason: reason, Cause: cause,
	})
}

// receiveEvent reports a real metric admission failure after observeEvent
// returned. Reporters MUST run without Observer.mu; custody MUST not call them.
func (o *Observer) receiveEvent(generation l2tpevents.ObservationGeneration, ev ObserverEvent, login string) {
	if err := o.observeEvent(generation, ev, login); err != nil {
		reportObserverMetricRefusal(ev.Birth, login, err)
	}
}

func (s *Subsystem) wireObserverSubscriptions(bus ze.EventBus) {
	obs := s.observer
	if obs == nil {
		return
	}
	if bus == nil {
		return
	}
	obs.addUnsub(l2tpevents.SessionUp.Subscribe(bus, func(p *l2tpevents.SessionUpPayload) {
		obs.receiveEvent(p.Generation, ObserverEvent{
			Birth: p.Birth, Timestamp: time.Now(), Type: ObserverEventSessionUp,
			TunnelID: p.TunnelID, SessionID: p.SessionID,
		}, "")
	}))
	obs.addUnsub(l2tpevents.SessionDown.Subscribe(bus, func(p *l2tpevents.SessionDownPayload) {
		obs.receiveEvent(p.Generation, ObserverEvent{
			Birth: p.Birth, Timestamp: time.Now(), Type: ObserverEventSessionDown,
			TunnelID: p.TunnelID, SessionID: p.SessionID,
		}, p.Username)
	}))
	obs.addUnsub(l2tpevents.SessionIPAssigned.Subscribe(bus, func(p *l2tpevents.SessionIPAssignedPayload) {
		obs.receiveEvent(p.Generation, ObserverEvent{
			Birth: p.Birth, Timestamp: time.Now(), Type: ObserverEventSessionIPAssigned,
			TunnelID: p.TunnelID, SessionID: p.SessionID,
		}, p.Username)
	}))
	obs.addUnsub(l2tpevents.EchoRTT.Subscribe(bus, func(p *l2tpevents.EchoRTTPayload) {
		obs.receiveEvent(p.Generation, ObserverEvent{
			Birth: p.Birth, Timestamp: time.Now(), Type: ObserverEventEchoRTT,
			TunnelID: p.TunnelID, SessionID: p.SessionID, RTT: p.RTT,
		}, p.Username)
	}))
}
