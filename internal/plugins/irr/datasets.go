// Design: docs/architecture/irr-owner.md -- what the owner reads from and writes to the store
// Related: owner.go -- the worker, the only caller
// Related: legacy.go -- the one-time conversion of meta/irr/{name}
//
// Every write follows one order, so a consumer never reads a pointer to a
// dataset that is not there: the dataset first, then the generation, then the
// pointer carrying that generation, then the notice. The generation is raised
// before the pointer is written because the pointer carries it; a pointer
// write that then fails leaves a generation no pointer names, which a
// consumer reads as a gap and recovers by reading every pointer again.

package irr

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
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
// sets it would have named are then fetched as if nothing were stored.
func (o *owner) load(ctx context.Context) {
	o.convertLegacy(ctx)
	o.readInterest(ctx)
	o.readPointers(ctx)
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
// merges per family into what is enforced (AC-7).
func (o *owner) readPointers(ctx context.Context) {
	keys, err := o.state.StateList(ctx, store.PointerPrefix)
	if err != nil {
		o.unreadable(store.PointerPrefix, err)
		return
	}
	for _, key := range keys {
		asSet := strings.TrimPrefix(key, store.PointerPrefix)
		value, found, err := o.state.StateGet(ctx, key)
		if err != nil || !found {
			o.unreadable(key, errors.Join(err, errNotFound(found)))
			continue
		}
		pointer, err := store.DecodePointer(value)
		if err != nil {
			o.unreadable(key, err)
			continue
		}
		o.pointers[asSet] = pointer
		o.readDataset(ctx, asSet, pointer)
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
	dataset, err := store.DecodeDataset(key, value)
	if err != nil {
		o.unreadable(key, err)
		return
	}
	digest := store.DatasetDigest(value)
	if hex.EncodeToString(digest[:]) != pointer.Digest {
		o.unreadable(key, errors.New("the dataset is not the one its pointer names: the digests differ"))
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
// server, answered at checkedAt, then flips the pointer and tells the
// consumers. A list over the caps is refused whole and the stored copy stays
// (AC-12); a store write that fails is retried in a minute.
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
	if err := o.state.StatePut(ctx, dataset.Key(), value); err != nil {
		return o.unpublished(asSet, checkedAt, err)
	}
	generation, err := o.state.StateIncrement(ctx, store.GenerationKey())
	if err != nil {
		return o.unpublished(asSet, checkedAt, err)
	}
	digest := store.DatasetDigest(value)
	pointer := store.Pointer{Server: dataset.Server(), Generation: generation, Digest: hex.EncodeToString(digest[:])}
	encoded, err := store.EncodePointer(pointer)
	if err != nil {
		return o.unpublished(asSet, checkedAt, err)
	}
	if err := o.state.StatePut(ctx, store.PointerKey(asSet), encoded); err != nil {
		return o.unpublished(asSet, checkedAt, err)
	}
	previous, had := o.pointers[asSet]
	o.pointers[asSet] = pointer
	o.datasets[asSet] = infoOf(dataset)
	o.resolve(asSet, codeIRRDatasetUnpublished)
	if had && previous.Server != pointer.Server {
		// The pointer names the new server's copy now, so the old one is
		// enforced by nobody (R-7).
		if err := o.state.StateRemove(ctx, previous.DatasetKey(asSet)); err != nil {
			logger().Warn("irr: the previous server's dataset could not be removed, no pointer names it",
				"as-set", asSet, "error", err, "code", codeIRRDatasetUnpublished, "fix", "update irr as-set "+asSet)
		}
	}
	o.emitNotice(ctx, store.DatasetNotice{ASSet: asSet, Server: pointer.Server, Generation: generation, Digest: pointer.Digest})
	logger().Info("irr dataset stored", "as-set", asSet, "server", pointer.Server,
		"ipv4-count", len(dataset.IPv4()), "ipv6-count", len(dataset.IPv6()), "generation", generation)
	return nil
}

// unpublished records a dataset the store or the notice refused, and arms a
// retry in a minute.
func (o *owner) unpublished(asSet string, checkedAt time.Time, err error) error {
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
	pointer, ok := o.pointers[asSet]
	if !ok {
		return
	}
	value, found, err := o.state.StateGet(ctx, pointer.DatasetKey(asSet))
	if err != nil || !found {
		return
	}
	dataset, err := store.DecodeDataset(pointer.DatasetKey(asSet), value)
	if err != nil || dataset.Server() != o.serverKey {
		return
	}
	if err := o.res.seed(entryOf(dataset)); err != nil {
		o.unreadable(pointer.DatasetKey(asSet), err)
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

// removeSet removes the pointer, then the dataset, of asSet, raises the
// generation and tells the consumers it is gone. It reports whether a stored
// copy existed.
func (o *owner) removeSet(ctx context.Context, asSet string) (bool, error) {
	pointer, ok := o.pointers[asSet]
	o.res.forget(asSet)
	o.schedule.drop(asSet)
	delete(o.answeredAt, asSet)
	delete(o.retryAt, asSet)
	if !ok {
		return false, nil
	}
	if err := o.state.StateRemove(ctx, store.PointerKey(asSet)); err != nil {
		return false, err
	}
	delete(o.pointers, asSet)
	delete(o.datasets, asSet)
	if err := o.state.StateRemove(ctx, pointer.DatasetKey(asSet)); err != nil {
		logger().Warn("irr: the cleared dataset could not be removed, no pointer names it",
			"as-set", asSet, "error", err, "code", codeIRRDatasetUnpublished, "fix", "clear irr as-set "+asSet)
	}
	generation, err := o.state.StateIncrement(ctx, store.GenerationKey())
	if err != nil {
		return true, err
	}
	o.emitNotice(ctx, store.DatasetNotice{ASSet: asSet, Server: pointer.Server, Generation: generation, Removed: true})
	return true, nil
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
