// Design: docs/architecture/behavior/peer-lifecycle.md -- selected OPEN publication.
// Related: session_open_admission.go -- selection after validation and old-epoch cleanup.
// RFC 4271 Section 8.2.1 -- the learned state of one connection must not replace its sibling's.
package reactor

import (
	"bytes"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// privateNegotiationEpoch owns the decoded OPEN observations of one candidate.
// Its zero value is invalid: newPrivateNegotiationEpoch constructs the pending
// publication variant. Safe for concurrent read and write callbacks. Selection
// MUST join its reader and hold Session.openMu before taking its observations;
// those conditions exclude another OPEN from overtaking the selected replay.
type privateNegotiationEpoch struct {
	mu          sync.Mutex
	publication openPublication
}

func newPrivateNegotiationEpoch() *privateNegotiationEpoch {
	return &privateNegotiationEpoch{publication: &pendingOpenPublication{}}
}

// openPublication is a closed pending-or-selected publication state. The
// pending variant owns only two first observations, one per OPEN direction;
// duplicate OPENs remain the protocol reader's rejection, not another receipt.
type openPublication interface {
	retain(observedOpen) bool
	selectMessages() (selectedOpenPublication, []observedOpen)
}

type pendingOpenPublication struct {
	observed [2]observedOpen
	count    int
}

type selectedOpenPublication struct{}

// observedOpen owns a copy of actual callback bytes, never a prepared proposal
// or a read/write pool alias. Header validation bounds an OPEN to 4096 octets,
// so a private epoch retains at most two bounded control-message bodies.
type observedOpen struct {
	body      []byte
	context   bgpctx.ContextID
	direction rpc.MessageDirection
	source    string
	messageID uint64
}

func (p *pendingOpenPublication) retain(event observedOpen) bool {
	for _, prior := range p.observed[:p.count] {
		if prior.direction == event.direction {
			return true
		}
	}
	if p.count == len(p.observed) {
		return true
	}
	event.body = bytes.Clone(event.body)
	p.observed[p.count] = event
	p.count++
	return true
}

func (p *pendingOpenPublication) selectMessages() (selectedOpenPublication, []observedOpen) {
	return selectedOpenPublication{}, p.observed[:p.count]
}

func (selectedOpenPublication) retain(observedOpen) bool { return false }
func (s selectedOpenPublication) selectMessages() (selectedOpenPublication, []observedOpen) {
	return s, nil
}

// messageCallback leaves ordinary messages on their existing path. Only decoded
// OPEN publication waits for selection; wire capture and physical OPEN counters
// are separate callbacks and MUST remain at their original read/write sites.
// A nil callback means no decoded observer, exactly as on an ordinary Session.
func (p *privateNegotiationEpoch) messageCallback(next MessageCallback) MessageCallback {
	if next == nil {
		return nil
	}
	return func(addr netip.Addr, kind msgtype.MessageType, body []byte,
		update *wireu.WireUpdate, ctx bgpctx.ContextID, direction rpc.MessageDirection,
		buffer BufHandle, meta map[string]any, source string, id uint64) bool {
		if kind == msgtype.TypeOPEN {
			p.mu.Lock()
			retained := p.publication.retain(observedOpen{
				body: body, context: ctx, direction: direction, source: source, messageID: id,
			})
			p.mu.Unlock()
			if retained {
				return false // The bounded copy never takes the original pool handle.
			}
		}
		return next(addr, kind, body, update, ctx, direction, buffer, meta, source, id)
	}
}

// selectMessages MUST be called only after the private reader was joined and
// while Session.openMu excludes protocol OPEN writes. It changes publication
// once, then returns owned observations for replay outside the epoch mutex.
func (p *privateNegotiationEpoch) selectMessages() []observedOpen {
	p.mu.Lock()
	defer p.mu.Unlock()
	selected, observed := p.publication.selectMessages()
	p.publication = selected
	return observed
}

// publish replays a decoded observation, not a wire operation. It MUST NOT
// invoke OPEN wire counters or capture, and MUST run outside the epoch mutex.
func (event observedOpen) publish(s *Session) error {
	if event.direction == rpc.DirectionReceived {
		err, _ := s.deliverReceivedMessage(msgtype.TypeOPEN, event.body, nil, event.context,
			BufHandle{ID: noPoolBufID, Buf: event.body})
		return err
	}
	if s.onMessageReceived != nil {
		s.onMessageReceived(s.settings.Address, msgtype.TypeOPEN, event.body, nil, event.context,
			event.direction, BufHandle{}, nil, event.source, event.messageID)
	}
	return nil
}
