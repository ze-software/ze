// Design: docs/architecture/api/process-protocol.md -- original Peer event lifetime.
// Related: register_source_lease.go -- terminal removal joins before applied DOWN.
package reactor

import "context"

// BeginSentObservation admits only this original Peer's wire observation.
// It spans every original Session, even a raw send before negotiated source
// receipt capture, and creates no source receipt, lease or retention right.
// The caller MUST capture recipients only after an accepted begin and MUST
// pair it with EndSentObservation after all SENT deliveries have been enqueued.
func (p *Peer) BeginSentObservation() bool {
	p.sentObservationMu.Lock()
	defer p.sentObservationMu.Unlock()
	if p.stopping.Load() {
		return false
	}
	p.sentObservations++
	return true
}

// EndSentObservation releases one accepted synchronous publication. The last
// completion wakes permanent removal only after its SENT deliveries enqueue.
// It MUST match exactly one accepted BeginSentObservation and MUST follow
// enqueueing all SENT deliveries for that observation.
func (p *Peer) EndSentObservation() {
	p.sentObservationMu.Lock()
	defer p.sentObservationMu.Unlock()
	if p.sentObservations == 0 {
		panic("BUG: sent observation completed without admission")
	}
	p.sentObservations--
	if p.sentObservations == 0 && p.sentObservationIdle != nil {
		close(p.sentObservationIdle)
		p.sentObservationIdle = nil
	}
}

// joinSentObservations MUST follow the permanent stopping seal. It runs with
// no reactor, peer or session lock held. Only this cold join allocates an idle
// channel; ordinary SENT publication performs no allocation or global lookup.
func (p *Peer) joinSentObservations(ctx context.Context) error {
	p.sentObservationMu.Lock()
	if !p.stopping.Load() {
		p.sentObservationMu.Unlock()
		panic("BUG: sent observation join preceded permanent Peer seal")
	}
	if p.sentObservations != 0 && p.sentObservationIdle == nil {
		p.sentObservationIdle = make(chan struct{})
	}
	idle := p.sentObservationIdle
	p.sentObservationMu.Unlock()
	return waitSourceReceiptWrites(ctx, idle)
}
