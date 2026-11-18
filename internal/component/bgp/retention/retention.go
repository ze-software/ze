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

// Namespace and EventType name the event bgp-gr emits on every physical DOWN it handles.
const (
	Namespace = "bgp-gr"
	EventType = "retention"
)

// Verdict is the event payload: the families bgp-gr retains for the departed
// peer at the end of its DOWN handler. An empty Families is a verdict too: the
// peer's routes are not retained (a NOTIFICATION close, no GR capability, a
// departed physical connection). Families are `<afi>/<safi>` names.
type Verdict struct {
	ConnectionID uint64   `json:"connection-id,string"`
	PeerOwner    uint64   `json:"peer-owner"`
	PeerAddress  string   `json:"peer-address"`
	Families     []string `json:"families"`
}

// Event is the registered bgp-gr/retention event. The registration makes the
// (namespace, event type) pair valid for emit-event and subscribe-events.
var Event = events.Register[*Verdict](Namespace, EventType)

// Encode renders the verdict for the original peerOwner at peerAddr as the JSON an emit-event carries.
func Encode(peerOwner, connectionID uint64, peerAddr string, families []family.Family) (string, error) {
	if err := validateLifetime(peerOwner, connectionID); err != nil {
		return "", err
	}
	if peerAddr == "" {
		return "", errNoPeer
	}
	verdict := Verdict{ConnectionID: connectionID, PeerOwner: peerOwner, PeerAddress: peerAddr, Families: make([]string, 0, len(families))}
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
// MUST call Begin on each physical UP and Take on each transport DOWN,
// including attached lifetimes and empty departures. Peer removal MUST use End,
// even without a current Session. Global GR verdicts for other peers cannot
// create records. The engine delivers state events in reverse dependency order
// (bgp-gr first) and each process receives events FIFO, so an owned verdict is
// recorded before its DOWN consumes it. Safe for concurrent use; the zero value
// is ready. Storage is bounded by the owner's active UP-to-DOWN lifetimes.
type Ledger struct {
	mu       sync.Mutex
	verdicts map[uint64]retentionLifetime
}

// retentionLifetime keeps only the one admitted connection of an owner.
// Successor Begin replaces this record; no completed connection history stays.
type retentionLifetime struct {
	connectionID uint64
	families     map[family.Family]bool
}

func validateLifetime(peerOwner, connectionID uint64) error {
	if peerOwner == 0 {
		return errNoOwner
	}
	if connectionID == 0 {
		return errNoConnection
	}
	return nil
}

// Begin admits a peer's new state lifetime, discarding any earlier verdict.
// The asker MUST call it on each UP, including an attached session, and MUST
// call Take on that connection's transport DOWN even with no routes, or End
// on owner-terminal peer removal.
func (l *Ledger) Begin(peerOwner, connectionID uint64) error {
	if err := validateLifetime(peerOwner, connectionID); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.verdicts == nil {
		l.verdicts = make(map[uint64]retentionLifetime)
	}
	l.verdicts[peerOwner] = retentionLifetime{connectionID: connectionID}
	return nil
}

// End closes an actual Peer owner's lifetime on peer-removed, including
// removal without a current Session. Callers MUST use this only for that
// owner-terminal event, never for a physical transport DOWN. It authorizes no
// route mutation and leaves every other owner's admitted lifetime unchanged.
func (l *Ledger) End(peerOwner uint64) error {
	if peerOwner == 0 {
		return errNoOwner
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.verdicts, peerOwner)
	return nil
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
var errNoConnection = errors.New("bgp-gr/retention verdict names no original connection-id")

var errNoOwner = errors.New("bgp-gr/retention verdict names no original peer-owner")

var errNoPeer = errors.New("bgp-gr/retention verdict names no peer-address")

// Record stores a verdict only within the original owner and connection's admitted state lifetime,
// replacing its earlier verdict. A payload that does not decode or names no
// peer is refused. An unowned peer is ignored; an owned verdict naming an
// unregistered family is refused whole, without changing its earlier verdict.
func (l *Ledger) Record(envelope rpc.EventEnvelope) error {
	var verdict Verdict
	if err := json.Unmarshal(envelope.Payload, &verdict); err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("bgp-gr/retention payload does not decode: ").Err(err).String())
	}
	if err := validateLifetime(verdict.PeerOwner, verdict.ConnectionID); err != nil {
		return err
	}
	if verdict.PeerAddress == "" {
		return errNoPeer
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	// refuse-by-default: guards Take -- only an admitted peer has an owning DOWN to consume its verdict.
	lifetime, active := l.verdicts[verdict.PeerOwner]
	if !active {
		return nil
	}
	// refuse-by-default: guards Record -- an old event cannot reopen the
	// successor's admitted physical connection, even with the same owner.
	if lifetime.connectionID != verdict.ConnectionID {
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
	lifetime.families = retained
	l.verdicts[verdict.PeerOwner] = lifetime
	return nil
}

// Take closes the original owner and connection's admitted state lifetime and consumes its verdict, so
// neither a later DOWN nor a late verdict can revive it. After Begin, the asker
// MUST call Take on every transport DOWN, an empty departure included;
// owner-terminal peer removal uses End. Without
// a verdict it answers an empty set: GR retained no families for this DOWN.
func (l *Ledger) Take(peerOwner, connectionID uint64) (map[family.Family]bool, error) {
	if err := validateLifetime(peerOwner, connectionID); err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	lifetime, active := l.verdicts[peerOwner]
	if !active {
		return nil, nil
	}
	// A delayed original DOWN cannot consume a successor's valid verdict.
	if lifetime.connectionID != connectionID {
		return nil, nil
	}
	delete(l.verdicts, peerOwner)
	return lifetime.families, nil
}

// Active reads only the asker's current original UP-to-DOWN lifetime. It
// creates no entry, and neither a missing owner nor connection zero is an
// admitted source. Take, End and a successor Begin revoke the old answer.
// Safe for concurrent use; the zero Ledger answers false.
func (l *Ledger) Active(peerOwner, connectionID uint64) bool {
	if peerOwner == 0 {
		return false
	}
	if connectionID == 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	lifetime, active := l.verdicts[peerOwner]
	if !active {
		return false
	}
	return lifetime.connectionID == connectionID
}
