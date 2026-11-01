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

// Ledger holds the latest verdict bgp-gr emitted for each peer, until the
// asker's own DOWN for that peer takes it. The engine delivers a state event in
// reverse dependency order and waits for each process (bgp-gr first), and every
// delivery to one process is FIFO, so the verdict of a DOWN is recorded before
// that DOWN reaches the asker. Safe for concurrent use. The zero value is ready.
type Ledger struct {
	mu       sync.Mutex
	verdicts map[string]map[family.Family]bool
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

// Record stores the verdict the envelope carries, replacing the peer's earlier
// one. A payload that does not decode, names no peer or names a family that is
// not registered is refused whole and changes nothing: applying part of it
// would withdraw or keep families bgp-gr did not decide.
func (l *Ledger) Record(envelope rpc.EventEnvelope) error {
	var verdict Verdict
	if err := json.Unmarshal(envelope.Payload, &verdict); err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("bgp-gr/retention payload does not decode: ").Err(err).String())
	}
	if verdict.PeerAddress == "" {
		return errNoPeer
	}
	retained := make(map[family.Family]bool, len(verdict.Families))
	for _, name := range verdict.Families {
		fam, ok := family.LookupFamily(name)
		if !ok {
			var tb textbuf.Buffer
			return errors.New(tb.Str("bgp-gr/retention verdict for ").Str(verdict.PeerAddress).
				Str(" names unknown family ").Str(name).String())
		}
		retained[fam] = true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.verdicts == nil {
		l.verdicts = make(map[string]map[family.Family]bool)
	}
	l.verdicts[verdict.PeerAddress] = retained
	return nil
}

// Take consumes the peer's recorded verdict and clears it, so a later DOWN
// never reads this one. The asker MUST call it on every DOWN it handles for the
// peer, an empty departure included. No verdict answers an empty set: bgp-gr
// handled no DOWN for this peer, so it holds no GR state for it.
func (l *Ledger) Take(peerAddr string) map[family.Family]bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	retained := l.verdicts[peerAddr]
	delete(l.verdicts, peerAddr)
	return retained
}
