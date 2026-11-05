// Design: docs/guide/graceful-restart.md -- the bgp-gr/retention event.
// Package retention carries bgp-gr's per-DOWN retention verdict to the
// forwarding owners (bgp-rr, bgp-rs) over the event bus, so each of the three
// can run internal or forked. It holds the event's identity, its payload type
// and the ledger an asker records verdicts in; it imports no plugin.
package retention

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Namespace and EventType name the event bgp-gr emits on every DOWN it handles.
const (
	Namespace = "bgp-gr"
	EventType = "retention"
)

// Verdict is the event payload: the families bgp-gr retains for the departed
// peer at the end of its DOWN handler. An empty Families is a verdict too: the
// peer's routes are not retained (a NOTIFICATION close, no GR capability, a
// removed peer). Families are `<afi>/<safi>` names.
type Verdict struct {
	PeerAddress string   `json:"peer-address"`
	Families    []string `json:"families"`
}

// Event is the registered bgp-gr/retention event. The registration makes the
// (namespace, event type) pair valid for emit-event and subscribe-events.
var Event = events.Register[*Verdict](Namespace, EventType)

// Encode renders the verdict for peerAddr as the JSON an emit-event carries.
func Encode(peerAddr string, families []family.Family) (string, error) {
	verdict := Verdict{PeerAddress: peerAddr, Families: make([]string, 0, len(families))}
	for _, fam := range families {
		verdict.Families = append(verdict.Families, fam.String())
	}
	encoded, err := json.Marshal(&verdict)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// Ledger holds verdicts only for the asker's active state lifetimes. The asker
// MUST call Begin on each UP and Take on each DOWN, including attached/removed
// lifetimes and empty departures. Global GR verdicts for other peers cannot
// create records. The engine delivers state events in reverse dependency order
// (bgp-gr first) and each process receives events FIFO, so an owned verdict is
// recorded before its DOWN consumes it. Safe for concurrent use; the zero value
// is ready. Storage is bounded by the owner's active UP-to-DOWN lifetimes.
type Ledger struct {
	mu       sync.Mutex
	verdicts map[string]map[family.Family]bool
}

// Begin admits a peer's new state lifetime, discarding any earlier verdict.
// The asker MUST call it on each UP, including an attached session, and MUST
// call Take on that lifetime's DOWN even when the peer owns no routes.
func (l *Ledger) Begin(peerAddr string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.verdicts == nil {
		l.verdicts = make(map[string]map[family.Family]bool)
	}
	l.verdicts[peerAddr] = nil
}

// Accepts reports whether the delivered event string is an enveloped
// bgp-gr/retention event, answering its envelope. An asker MUST opt into
// enveloped delivery (sdk.Plugin.SetEnvelope) for its events to carry one.
func Accepts(delivered string) (rpc.EventEnvelope, bool) {
	if delivered == "" || delivered[0] != '{' {
		return rpc.EventEnvelope{}, false
	}
	envelope, err := rpc.ParseEventEnvelope(delivered)
	if err != nil {
		return rpc.EventEnvelope{}, false
	}
	if envelope.Namespace != Namespace {
		return rpc.EventEnvelope{}, false
	}
	if envelope.Event != EventType {
		return rpc.EventEnvelope{}, false
	}
	return envelope, true
}

// errNoPeer refuses a verdict that names no peer.
var errNoPeer = errors.New("bgp-gr/retention verdict names no peer-address")

// Record stores a verdict only within the peer's admitted state lifetime,
// replacing its earlier verdict. A payload that does not decode or names no
// peer is refused. An unowned peer is ignored; an owned verdict naming an
// unregistered family is refused whole, without changing its earlier verdict.
func (l *Ledger) Record(envelope rpc.EventEnvelope) error {
	var verdict Verdict
	if err := json.Unmarshal(envelope.Payload, &verdict); err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("bgp-gr/retention payload does not decode: ").Err(err).String())
	}
	if verdict.PeerAddress == "" {
		return errNoPeer
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	// refuse-by-default: guards Take -- only an admitted peer has an owning DOWN to consume its verdict.
	if _, active := l.verdicts[verdict.PeerAddress]; !active {
		return nil
	}
	var retained map[family.Family]bool
	if len(verdict.Families) > 0 {
		retained = make(map[family.Family]bool, len(verdict.Families))
	}
	for _, name := range verdict.Families {
		fam, ok := family.LookupFamily(name)
		if !ok {
			var tb textbuf.Buffer
			return errors.New(tb.Str("bgp-gr/retention verdict for ").Str(verdict.PeerAddress).
				Str(" names unknown family ").Str(name).String())
		}
		retained[fam] = true
	}
	l.verdicts[verdict.PeerAddress] = retained
	return nil
}

// Take closes the peer's admitted state lifetime and consumes its verdict, so
// neither a later DOWN nor a late verdict can revive it. After Begin, the asker
// MUST call Take on every DOWN it handles, an empty departure included. Without
// a verdict it answers an empty set: GR retained no families for this DOWN.
func (l *Ledger) Take(peerAddr string) map[family.Family]bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	retained := l.verdicts[peerAddr]
	delete(l.verdicts, peerAddr)
	return retained
}
