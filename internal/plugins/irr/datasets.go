// Design: docs/architecture/irr-owner.md -- what the owner reads from and writes to the store
// Related: owner.go -- the worker, the only caller
// Related: legacy.go -- the one-time conversion of meta/irr/{name}
//
// A dataset is stored under a key that ends in its own digest, so a write never
// replaces the bytes the current pointer names, and the pointer flip is the
// only switch. Every write follows one order, so a consumer never reads a
// pointer to a dataset that is not there: the dataset first, then the
// generation, then the pointer carrying that generation, then the notice. The
// generation is raised before the pointer is written because the pointer
// carries it; a pointer write that then fails leaves a generation no pointer
// names, which a consumer reads as a gap and recovers by reading every pointer
// again, and the old pointer still names the old dataset, intact.

package irr

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

// isEmptyAnswer reports whether a refresh error means the server answered
// and named no prefix. checked-at then advances: the server did answer.
func isEmptyAnswer(err error) bool {
	return errors.Is(err, store.ErrNoPrefixes)
}

// load reads what the store holds: the interest of every consumer, every
// pointer with the dataset it names, every binding. It runs once, first on
// the worker. A list that fails leaves that kind empty and says so: the
// sets it would have named are then fetched as if nothing were stored. Once
// the pointers are read, the datasets no pointer names are removed.
func (o *owner) load(ctx context.Context) {
	o.convertLegacy(ctx)
	o.readInterest(ctx)
	unread, listed := o.readPointers(ctx)
	if listed {
		o.removeUnpointed(ctx, unread)
	}
	o.readBindings(ctx)
	o.plan()
}

// readInterest re-reads every consumer's interest key. A name the new union
// holds and the old one did not releases a `clear irr` hold: a commit that
// newly names a cleared set fetches it again.
func (o *owner) readInterest(ctx context.Context) {
	keys, err := o.state.StateList(ctx, store.InterestPrefix)
	if err != nil {
		o.unreadable(store.InterestPrefix, err)
		return
	}
	o.resolve(store.InterestPrefix, codeIRRDatasetUnreadable)
	previousSets, previousASNs := o.wantedSets(), o.wantedASNs()
	next := make(map[string]store.Interest, len(keys))
	for _, key := range keys {
		consumer := strings.TrimPrefix(key, store.InterestPrefix)
		value, found, err := o.state.StateGet(ctx, key)
		if err != nil || !found {
			o.unreadable(key, errors.Join(err, errNotFound(found)))
			continue
		}
		in, err := store.DecodeInterest(value)
		if err != nil {
			o.unreadable(key, err)
			continue
		}
		o.resolve(key, codeIRRDatasetUnreadable)
		next[consumer] = in
	}
	o.interest = next
	for asSet := range o.wantedSets() {
		if _, before := previousSets[asSet]; !before {
			delete(o.heldSets, asSet)
		}
	}
	for asn := range o.wantedASNs() {
		if _, before := previousASNs[asn]; !before {
			delete(o.heldASNs, asn)
		}
	}
}

// errNotFound names a key a listing returned and a read did not find.
func errNotFound(found bool) error {
	if found {
		return nil
	}
	return errors.New("listed but not found")
}

// readPointers reads every pointer and the dataset it names, and seeds the
// resolver with each dataset of the configured server, so the next answer
// merges per family into what is enforced (AC-7). It answers the AS-SETs whose
// pointer is listed and could not be read, and whether the listing answered:
// removeUnpointed keeps every dataset of such a set, and removes nothing at
// all without a listing.
func (o *owner) readPointers(ctx context.Context) (unread map[string]bool, listed bool) {
	keys, err := o.state.StateList(ctx, store.PointerPrefix)
	if err != nil {
		o.unreadable(store.PointerPrefix, err)
		return nil, false
	}
	unread = make(map[string]bool)
	for _, key := range keys {
		asSet := strings.TrimPrefix(key, store.PointerPrefix)
		value, found, err := o.state.StateGet(ctx, key)
		if err != nil || !found {
			unread[asSet] = true
			o.unreadable(key, errors.Join(err, errNotFound(found)))
			continue
		}
		pointer, err := store.DecodePointer(value)
		if err != nil {
			unread[asSet] = true
			o.unreadable(key, err)
			continue
		}
		o.pointers[asSet] = pointer
		o.readDataset(ctx, asSet, pointer)
	}
	return unread, true
}

// removeUnpointed removes every stored dataset no pointer names: one a write
// left when its generation or pointer write failed and its own removal failed
// too, one a flip or a clear could not remove, or one written by a crash
// between two writes. A dataset of a set whose pointer is listed and could not
// be read is kept, because that pointer may name it. A key that is not a
// dataset key names nothing and is removed. A removal that fails is logged and
// tried again at the next start.
func (o *owner) removeUnpointed(ctx context.Context, unread map[string]bool) {
	keys, err := o.state.StateList(ctx, store.DatasetPrefix)
	if err != nil {
		o.unreadable(store.DatasetPrefix, err)
		return
	}
	for _, key := range keys {
		if asSet, ok := store.DatasetKeyASSet(key); ok {
			if unread[asSet] {
				continue
			}
			if o.pointerNames(asSet, key) {
				continue
			}
		}
		if err := o.state.StateRemove(ctx, key); err != nil {
			logger().Warn("irr: a stored dataset no pointer names could not be removed, the next start tries again",
				"key", key, "error", err, "code", codeIRRDatasetUnpublished, "fix", "update irr all")
			continue
		}
		logger().Info("irr: removed a stored dataset no pointer names", "key", key)
	}
}

// readDataset reads the dataset pointer names for asSet. A dataset that is
// missing, fails its digest or decode, or is not the one the pointer names is
// unreadable: the set is due at once, and the consumers keep what they enforce
// (AC-22).
func (o *owner) readDataset(ctx context.Context, asSet string, pointer store.Pointer) {
	key := pointer.DatasetKey(asSet)
	value, found, err := o.state.StateGet(ctx, key)
	if err != nil || !found {
		o.unreadable(key, errors.Join(err, errNotFound(found)))
		return
	}
	// The key ends in the pointer's digest, and DecodeDataset refuses a value
	// whose own digest is another (ErrDatasetIdentity): a dataset that is not
	// the one its pointer names is unreadable here.
	dataset, err := store.DecodeDataset(key, value)
	if err != nil {
		o.unreadable(key, err)
		return
	}
	o.resolve(key, codeIRRDatasetUnreadable)
	o.datasets[asSet] = infoOf(dataset)
	if dataset.Server() != o.serverKey {
		return
	}
	if err := o.res.seed(entryOf(dataset)); err != nil {
		o.unreadable(key, err)
	}
}

// readBindings reads every stored binding.
func (o *owner) readBindings(ctx context.Context) {
	keys, err := o.state.StateList(ctx, store.BindingPrefix)
	if err != nil {
		o.unreadable(store.BindingPrefix, err)
		return
	}
	for _, key := range keys {
		asn, ok := parseASNSegment(strings.TrimPrefix(key, store.BindingPrefix))
		if !ok {
			o.unreadable(key, errors.New("the key names no AS number"))
			continue
		}
		value, found, err := o.state.StateGet(ctx, key)
		if err != nil || !found {
			o.unreadable(key, errors.Join(err, errNotFound(found)))
			continue
		}
		binding, err := store.DecodeBinding(value)
		if err != nil {
			o.unreadable(key, err)
			continue
		}
		o.bindings[asn] = binding
	}
}

// unreadable records and logs a stored value the owner could not use.
func (o *owner) unreadable(key string, err error) {
	o.raise(key, codeIRRDatasetUnreadable, err)
	logger().Warn("irr: a stored value cannot be read, it is treated as absent",
		"key", key, "error", err, "code", codeIRRDatasetUnreadable, "fix", "update irr all")
}

// writeDataset stores entry as the dataset of asSet from the configured
// server, answered at checkedAt, then flips the pointer, writes each binding
// waiting to move to asSet (bindTo) and tells the consumers. A list over the
// caps is refused whole and the stored copy stays (AC-12); a store write that
// fails is retried in a minute. Either way the resolver goes back to the
// stored copy (restoreEnforced). A stored copy clears the unreadable findings
// of the pointer and of the dataset it replaced (AC-22).
func (o *owner) writeDataset(ctx context.Context, asSet string, entry store.CachedEntry, checkedAt time.Time) error {
	dataset, err := store.NewDataset(store.DatasetInput{
		ASSet: asSet, Server: o.cfg.server, IPv4: entry.IPv4, IPv6: entry.IPv6,
		RefreshedAt: entry.RefreshedAt, CheckedAt: checkedAt, StaleSince: entry.StaleSince,
	})
	if err != nil {
		o.restoreEnforced(ctx, asSet)
		o.answeredAt[asSet] = checkedAt
		o.raise(asSet, codeIRRDatasetOversized, err)
		logger().Warn("irr: the IRR answer cannot be stored, the stored list stays enforced",
			"as-set", asSet, "error", err, "code", codeIRRDatasetOversized, "fix", "clear irr as-set "+asSet)
		return err
	}
	o.resolve(asSet, codeIRRDatasetOversized)
	value := make([]byte, dataset.EncodedLen())
	dataset.WriteTo(value, 0)
	digest := store.DatasetDigest(value)
	// The generation is filled in once it is raised; the key needs only the
	// server and the digest, so the dataset is written first.
	pointer := store.Pointer{Server: dataset.Server(), Digest: hex.EncodeToString(digest[:])}
	key := pointer.DatasetKey(asSet)
	previous, had := o.pointers[asSet]
	// sharesKey is a guard: a value byte for byte the one the old pointer names
	// lands on that pointer's own key, which then must neither be removed as
	// the torn write's leftover nor as the dataset the flip replaced.
	sharesKey := had && previous.DatasetKey(asSet) == key
	if err := o.state.StatePut(ctx, key, value); err != nil {
		return o.unpublished(ctx, asSet, checkedAt, err)
	}
	generation, err := o.state.StateIncrement(ctx, store.GenerationKey())
	if err != nil {
		o.removeTorn(ctx, asSet, key, sharesKey)
		return o.unpublished(ctx, asSet, checkedAt, err)
	}
	pointer.Generation = generation
	encoded, err := store.EncodePointer(pointer)
	if err != nil {
		o.removeTorn(ctx, asSet, key, sharesKey)
		return o.unpublished(ctx, asSet, checkedAt, err)
	}
	if err := o.state.StatePut(ctx, store.PointerKey(asSet), encoded); err != nil {
		o.removeTorn(ctx, asSet, key, sharesKey)
		return o.unpublished(ctx, asSet, checkedAt, err)
	}
	o.pointers[asSet] = pointer
	o.datasets[asSet] = infoOf(dataset)
	delete(o.unannounced, asSet)
	o.resolve(asSet, codeIRRDatasetUnpublished)
	o.resolve(store.PointerKey(asSet), codeIRRDatasetUnreadable)
	o.resolve(key, codeIRRDatasetUnreadable)
	if had {
		o.resolve(previous.DatasetKey(asSet), codeIRRDatasetUnreadable)
	}
	if had && !sharesKey {
		// The pointer names the new copy now, so the old one is enforced by
		// nobody (R-7), whether the server changed or not.
		if err := o.state.StateRemove(ctx, previous.DatasetKey(asSet)); err != nil {
			logger().Warn("irr: the previous dataset could not be removed, no pointer names it; the next start removes it",
				"as-set", asSet, "error", err, "code", codeIRRDatasetUnpublished, "fix", "update irr as-set "+asSet)
		}
	}
	// A binding moves after the pointer of its new set, so a consumer never
	// reads a binding to a set with no pointer, and before the notice, which
	// is what makes a consumer read every binding again.
	o.moveBindingsTo(ctx, asSet)
	o.emitNotice(ctx, store.DatasetNotice{ASSet: asSet, Server: pointer.Server, Generation: generation, Digest: pointer.Digest})
	logger().Info("irr dataset stored", "as-set", asSet, "server", pointer.Server,
		"ipv4-count", len(dataset.IPv4()), "ipv6-count", len(dataset.IPv6()), "generation", generation)
	return nil
}

// pointerNames reports whether the pointer of asSet the owner read names key.
func (o *owner) pointerNames(asSet, key string) bool {
	pointer, pointed := o.pointers[asSet]
	if !pointed {
		return false
	}
	return pointer.DatasetKey(asSet) == key
}

// removeTorn removes the dataset a write stored under key before its
// generation or pointer write failed: no pointer names it, and every retry
// would otherwise leave one more. A value that shares the old pointer's key is
// that pointer's dataset and stays. A removal that fails is left to load's
// removeUnpointed at the next start.
func (o *owner) removeTorn(ctx context.Context, asSet, key string, sharesKey bool) {
	if sharesKey {
		return
	}
	if err := o.state.StateRemove(ctx, key); err != nil {
		logger().Warn("irr: an unpublished dataset could not be removed, no pointer names it; the next start removes it",
			"as-set", asSet, "key", key, "error", err, "code", codeIRRDatasetUnpublished, "fix", "update irr as-set "+asSet)
	}
}

// unpublished records a dataset the store refused, puts the resolver back to
// the stored copy and arms a retry in a minute.
func (o *owner) unpublished(ctx context.Context, asSet string, checkedAt time.Time, err error) error {
	o.restoreEnforced(ctx, asSet)
	o.retryAt[asSet] = checkedAt.Add(retryInterval)
	o.raise(asSet, codeIRRDatasetUnpublished, err)
	logger().Warn("irr: the IRR answer was not stored, the stored list stays enforced and the store is retried in a minute",
		"as-set", asSet, "error", err, "code", codeIRRDatasetUnpublished, "fix", "update irr as-set "+asSet)
	return err
}

// restoreEnforced puts the resolver's copy of asSet back to the stored
// dataset, after an answer the store refused: PrefixStore installed it in
// memory, and the next answer must merge into what is enforced.
func (o *owner) restoreEnforced(ctx context.Context, asSet string) {
	o.res.forget(asSet)
	o.seedStored(ctx, asSet)
}

// seedStored seeds the resolver with the dataset the pointer of asSet names,
// when that dataset is from the configured server: a copy from another server
// is no merge base for this one's answer. A dataset that cannot be read is
// logged and raised, and leaves the resolver without a copy of asSet.
func (o *owner) seedStored(ctx context.Context, asSet string) {
	pointer, ok := o.pointers[asSet]
	if !ok {
		return
	}
	if pointer.Server != o.serverKey {
		return
	}
	key := pointer.DatasetKey(asSet)
	value, found, err := o.state.StateGet(ctx, key)
	if err != nil || !found {
		o.unreadable(key, errors.Join(err, errNotFound(found)))
		return
	}
	dataset, err := store.DecodeDataset(key, value)
	if err != nil {
		o.unreadable(key, err)
		return
	}
	if err := o.res.seed(entryOf(dataset)); err != nil {
		o.unreadable(key, err)
	}
}

// moveBindingsTo writes each binding waiting to move to asSet, now that its
// pointer is stored.
func (o *owner) moveBindingsTo(ctx context.Context, asSet string) {
	for asn, target := range o.movingTo {
		if target != asSet {
			continue
		}
		delete(o.movingTo, asn)
		o.writeBinding(ctx, asn, asSet)
	}
}

// writeBinding stores the set asn is queried under. A refused write keeps
// the binding in memory, so this run is consistent; a restart asks again.
func (o *owner) writeBinding(ctx context.Context, asn uint32, asSet string) {
	binding := store.Binding{ASSet: asSet, PeeringDBURL: o.cfg.peeringDBURL, DiscoveredAt: o.clock()}
	o.bindings[asn] = binding
	encoded, err := store.EncodeBinding(binding)
	if err == nil {
		err = o.state.StatePut(ctx, store.BindingKey(asn), encoded)
	}
	if err != nil {
		key := store.BindingKey(asn)
		o.raise(key, codeIRRDatasetUnpublished, err)
		logger().Warn("irr: the AS-SET of an AS number was not stored, a restart asks PeeringDB again",
			"asn", asn, "as-set", asSet, "error", err, "code", codeIRRDatasetUnpublished, "fix", "update irr asn "+asnText(asn))
		return
	}
	o.resolve(store.BindingKey(asn), codeIRRDatasetUnpublished)
}

// errRemovalUnannounced marks a removal the store made and the consumers were
// not told of: the generation could not be raised.
var errRemovalUnannounced = errors.New("the consumers were not told of the removal")

// removeSet removes the pointer, then the dataset, of asSet, raises the
// generation and tells the consumers it is gone. It reports whether a stored
// copy existed. A refused pointer removal changes nothing: the set stays
// pointed, stored, scheduled and seeded as the next answer's merge base. A
// removal the consumers were not told of answers errRemovalUnannounced, and
// the next removeSet of the set, with nothing left to remove, tells them.
func (o *owner) removeSet(ctx context.Context, asSet string) (bool, error) {
	pointer, ok := o.pointers[asSet]
	if !ok {
		o.forgetSet(asSet)
		// Nothing is stored, so a refused write or fetch of the set is moot.
		delete(o.problems, asSet)
		if server, waiting := o.unannounced[asSet]; waiting {
			return false, o.announceRemoval(ctx, asSet, server)
		}
		return false, nil
	}
	if err := o.state.StateRemove(ctx, store.PointerKey(asSet)); err != nil {
		return false, err
	}
	o.forgetSet(asSet)
	delete(o.pointers, asSet)
	delete(o.datasets, asSet)
	// What was stored is gone, so every finding about it describes nothing.
	delete(o.problems, asSet)
	delete(o.problems, store.PointerKey(asSet))
	delete(o.problems, pointer.DatasetKey(asSet))
	if err := o.state.StateRemove(ctx, pointer.DatasetKey(asSet)); err != nil {
		logger().Warn("irr: the cleared dataset could not be removed, no pointer names it",
			"as-set", asSet, "error", err, "code", codeIRRDatasetUnpublished, "fix", "clear irr as-set "+asSet)
	}
	return true, o.announceRemoval(ctx, asSet, pointer.Server)
}

// forgetSet drops what the worker holds in memory of asSet for fetching it:
// the resolver's copy, its due time and its answer and retry times.
func (o *owner) forgetSet(asSet string) {
	o.res.forget(asSet)
	o.schedule.drop(asSet)
	delete(o.answeredAt, asSet)
	delete(o.retryAt, asSet)
}

// announceRemoval raises the generation and sends the notice that asSet,
// stored from server, is gone. A refused increment is recorded in
// unannounced and raised with the clear that retries it as its fix.
func (o *owner) announceRemoval(ctx context.Context, asSet, server string) error {
	generation, err := o.state.StateIncrement(ctx, store.GenerationKey())
	if err != nil {
		o.unannounced[asSet] = server
		fix := "clear irr as-set " + asSet
		o.raiseFix(asSet, codeIRRDatasetUnpublished, err, fix)
		logger().Warn("irr: the stored copy was removed but the consumers were not told, they keep enforcing it",
			"as-set", asSet, "error", err, "code", codeIRRDatasetUnpublished, "fix", fix)
		return fmt.Errorf("%w: %w", errRemovalUnannounced, err)
	}
	delete(o.unannounced, asSet)
	o.resolve(asSet, codeIRRDatasetUnpublished)
	o.emitNotice(ctx, store.DatasetNotice{ASSet: asSet, Server: server, Generation: generation, Removed: true})
	return nil
}

// emitNotice sends irr/dataset-stored. A notice that is not delivered is
// recovered by the consumers at the next notice's generation gap.
func (o *owner) emitNotice(ctx context.Context, notice store.DatasetNotice) {
	payload, err := json.Marshal(notice)
	if err == nil {
		_, err = o.notices.EmitEvent(ctx, store.NoticeNamespace, store.EventDatasetStored, "", "", string(payload))
	}
	if err != nil {
		o.raise(notice.ASSet, codeIRRDatasetUnpublished, err)
		logger().Warn("irr: the consumers were not told of a stored IRR dataset, they catch up at the next notice",
			"as-set", notice.ASSet, "error", err, "code", codeIRRDatasetUnpublished, "fix", "update irr as-set "+notice.ASSet)
	}
}

// infoOf is what the owner keeps of a dataset.
func infoOf(dataset store.Dataset) datasetInfo {
	return datasetInfo{
		server: dataset.Server(), ipv4Count: len(dataset.IPv4()), ipv6Count: len(dataset.IPv6()),
		refreshedAt: dataset.RefreshedAt(), checkedAt: dataset.CheckedAt(), staleSince: dataset.StaleSince(),
	}
}

// entryOf is the resolver's copy of a dataset.
func entryOf(dataset store.Dataset) store.CachedEntry {
	return store.CachedEntry{
		Name: dataset.ASSet(), ASSet: dataset.ASSet(), IPv4: dataset.IPv4(), IPv6: dataset.IPv6(),
		RefreshedAt: dataset.RefreshedAt(), StaleSince: dataset.StaleSince(),
	}
}
