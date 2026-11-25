// Design: docs/architecture/api/process-protocol.md -- engine-issued origin scope authority.
// Related: origin_build.go -- immutable original build data; origin_write.go -- final-use guards.
package reactor

import (
	"context"
	"errors"
	"math"
	"sync"

	"github.com/ze-software/ze/internal/component/plugin"
)

// originContributionKey identifies one source contribution in one configured
// Peer scope. Mutable preference values and the deciding process are not source
// identity: changing them replaces the same contribution instead of leaking one.
type originContributionKey struct {
	producer  originProducer
	source    originSourceIdentity
	path      fwdPathKey
	pathID    uint32
	hasPathID bool
	peerOwner uint64
}

// originScope is immutable engine-captured matching identity shared by pending
// issuance and issued records. It is neither a receipt nor permission to write.
type originScope struct {
	key      originContributionKey
	peer     *Peer
	provider plugin.SelectedOwner
}

// originIssue enrolls one actual call before permission preflight. Revocation
// seals it under store.mu; the call removes it on every return, so canceled
// intakes cannot become either fresh receipts or historical scope tombstones.
type originIssue struct {
	originScope
	revoked  bool                 // store.mu
	original *originReceiptRecord // trusted re-offer of this exact current original value
}

// originReceiptStore is an issuer, never a candidate RIB or selector. It keeps
// only current contributions, records with actual admitted writers and actual
// active issuance calls. Numeric IDs feed SelectedRoute.OriginReceipt without
// creating public authority. IDs never wrap or leave withdrawn-key tombstones.
//
// Constructor required: zero/nil is invalid. The owner MUST call close after
// canceling its reactor lifetime and MUST call explicit revoke barriers before
// acknowledging producer/Peer/grant removal. A permanent lifecycle removal MUST
// first publish the actual grant removal or original Peer/process stop; a
// deliberate regrant may admit fresh calls but cannot revive sealed old calls.
// mu is a leaf: never hold it across I/O, peer locks, engine calls or waits.
// Safe for concurrent use.
type originReceiptStore struct {
	mu         sync.Mutex
	lifetime   context.Context
	cancel     context.CancelFunc
	parentStop func() bool
	calls      map[*originIntakeCall]struct{}
	next       uint64
	closed     bool
	current    map[originContributionKey]*originReceiptRecord
	receipts   map[uint64]*originReceiptRecord
	intakes    map[*originIssue]struct{}
}

type originReceiptRecord struct {
	originScope
	store     *originReceiptStore
	id        uint64
	lineage   uint64 // First accepted receipt, preserved only by reissue.
	source    *originSource
	build     *originBuildValue
	ctx       context.Context
	cancel    context.CancelFunc
	revoked   bool // store.mu
	retained  bool // store.mu: only exact generation cleanup may retain original data
	writes    map[uint64]struct{}
	nextWrite uint64
	idle      chan struct{} // allocated only while revoking actual outstanding work
}

func newOriginReceiptStore(lifetime context.Context) (*originReceiptStore, error) {
	if lifetime == nil {
		return nil, errors.New("origin issuer lifetime is missing")
	}
	if lifetime.Done() == nil {
		return nil, errors.New("origin issuer lifetime cannot be canceled")
	}
	if err := lifetime.Err(); err != nil {
		return nil, err
	}
	return &originReceiptStore{lifetime: lifetime, current: make(map[originContributionKey]*originReceiptRecord), receipts: make(map[uint64]*originReceiptRecord), intakes: make(map[*originIssue]struct{}), calls: make(map[*originIntakeCall]struct{})}, nil
}

func (store *originReceiptStore) live() error {
	if store == nil {
		return errors.New("origin issuer is missing")
	}
	if store.lifetime == nil {
		return errors.New("origin issuer is uninitialized")
	}
	if store.closed {
		return errors.New("origin issuer has closed")
	}
	return store.lifetime.Err()
}

// beginIssue linearizes enrollment with granular revocation before any producer,
// grant or ownership preflight. It grants no authority and issues no numeric ID.
func (store *originReceiptStore) beginIssue(ctx context.Context, scope originScope) (*originIssue, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.live(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	intake := &originIssue{originScope: scope}
	store.intakes[intake] = struct{}{}
	return intake, nil
}

func (store *originReceiptStore) endIssue(intake *originIssue) {
	store.mu.Lock()
	delete(store.intakes, intake)
	store.mu.Unlock()
}

// issue owns a validated original contribution; it says nothing about retention,
// election or announcement. The intake adapter MUST await the deciding plugin's
// P1 admission/completion result before answering its caller. Replacing a source
// revokes old writers but does not claim they joined; those actual obligations
// stay indexed until complete and are covered by withdrawal/cleanup barriers.
// Every call MUST enroll before preflight and keep that enrollment through
// commit. A matching revoke seals this exact call even if permission is later
// restored; a fresh call under restored permission enrolls independently.
func (store *originReceiptStore) issue(ctx context.Context, producer originProducer, source *originSource, build *originBuildValue, peer *Peer, provider plugin.SelectedOwner) (uint64, error) {
	return store.issueOriginal(ctx, producer, source, build, peer, provider, nil)
}

// issueOriginal accepts an engine-owned original only when it is still current.
// A retained original never turns an ended Process into an ingress authority.
func (store *originReceiptStore) issueOriginal(ctx context.Context, producer originProducer, source *originSource, build *originBuildValue, peer *Peer, provider plugin.SelectedOwner, original *originReceiptRecord) (uint64, error) {
	if store == nil {
		return 0, errors.New("origin issuer is missing")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if source == nil {
		return 0, errors.New("origin source is missing")
	}
	if build == nil {
		return 0, errors.New("origin build data is missing")
	}
	if build.originIdentity == nil {
		return 0, errors.New("origin native identity is missing")
	}
	if _, bound := provider.ProcessContext(); !bound {
		return 0, errors.New("origin deciding process has no captured connection lifetime")
	}
	if peer == nil {
		return 0, errors.New("origin peer scope is missing")
	}
	if peer.sourceOwner == 0 {
		return 0, errors.New("origin original Peer identity is missing")
	}
	key := originKey(producer, source, build.originIdentity, peer.sourceOwner)
	intake, err := store.beginIssue(ctx, originScope{key: key, peer: peer, provider: provider})
	if err != nil {
		return 0, err
	}
	defer store.endIssue(intake)
	store.mu.Lock()
	intake.original = original
	producerErr := store.issueProducerCurrent(intake)
	store.mu.Unlock()
	if err := producerErr; err != nil {
		return 0, err
	}
	if peer.stopping.Load() {
		return 0, errors.New("origin original Peer has been removed")
	}
	// Same sender grant guard as getMatchingPeersSel, on its concrete Peer.
	// The ingress adapter must resolve the selector before it reaches this domain.
	peer.mu.RLock()
	permitted, refused := filterPermittedPeers([]*Peer{peer}, announceOrigin(producer.sender()))
	peer.mu.RUnlock()
	if refused != 0 {
		return 0, errors.New("origin producer has no update grant for this Peer")
	}
	if len(permitted) != 1 {
		return 0, errors.New("origin producer scope is not authorized")
	}
	publication := peer.selectedOwnerSnapshot()
	snapshot := publication
	if snapshot == nil {
		// A configured Peer is a valid admitted scope before TCP exists. Do not
		// publish forwarding facts for it: final output still needs a real Session.
		snapshot = peer.buildDisseminationSnapshot(nil, peer.settingsSnapshot())
	}
	if snapshot.owner.Explicit() {
		return 0, errors.New("origin ordinary intake cannot replace an explicit dissemination role")
	}
	if _, err := snapshot.ExportKind(provider, false); err != nil {
		return 0, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.live(); err != nil {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if intake.revoked {
		return 0, errors.New("origin intake was revoked before admission; submit a fresh contribution under current permission")
	}
	if peer.stopping.Load() {
		return 0, errors.New("origin original Peer has been removed")
	}
	if err := store.issueProducerCurrent(intake); err != nil {
		return 0, err
	}
	if err := provider.Current(); err != nil {
		return 0, err
	}
	if current := peer.selectedOwnerSnapshot(); current != publication {
		if publication != nil {
			return 0, errors.New("origin Peer ownership changed during admission")
		}
		if current == nil {
			return 0, errors.New("origin Peer ownership changed during admission")
		}
		if current.err != nil {
			return 0, errors.New("origin Peer ownership changed during admission")
		}
		if current.owner != snapshot.owner {
			return 0, errors.New("origin Peer ownership changed during admission")
		}
	}
	if store.next == math.MaxUint64 {
		return 0, errors.New("origin receipt identities exhausted; restart Ze before admitting another origin")
	}
	store.next++
	work, cancel := context.WithCancel(store.lifetime)
	record := &originReceiptRecord{originScope: intake.originScope, store: store, id: store.next, lineage: store.next,
		source: source, build: build, ctx: work, cancel: cancel}
	if original != nil {
		record.retained = original.retained
	}
	// An authenticated same-name successor replaces only an actual kept old
	// contribution, never grants itself the predecessor's ingress token.
	for _, old := range store.current {
		if !old.retained {
			continue
		}
		if sameOriginSource(old.key, key) {
			store.invalidate(old)
		}
	}
	if old := store.current[key]; old != nil {
		store.invalidate(old)
	}
	if intake.original != nil {
		record.lineage = intake.original.lineage
	}
	store.current[key], store.receipts[record.id] = record, record
	return record.id, nil
}

func originKey(producer originProducer, source *originSource, identity *originIdentity, peerOwner uint64) originContributionKey {
	return originContributionKey{producer: producer, source: source.identity, path: identity.key,
		pathID: identity.pathID, hasPathID: identity.hasPathID, peerOwner: peerOwner}
}

// invalidate requires store.mu. A retired record stays discoverable only while
// real writes retain it; a new issuer record cannot inherit their completion.
func (store *originReceiptStore) invalidate(record *originReceiptRecord) {
	if store.current[record.key] == record {
		delete(store.current, record.key)
	}
	if record.revoked {
		return
	}
	record.revoked = true
	record.cancel()
	if store.current[record.key] == record {
		delete(store.current, record.key)
	}
	if len(record.writes) != 0 {
		record.idle = make(chan struct{})
		return
	}
	delete(store.receipts, record.id)
}
