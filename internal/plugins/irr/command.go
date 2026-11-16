// Design: docs/architecture/irr-owner.md -- show irr, update irr and clear irr
// Related: owner.go -- the worker every command runs on
// Related: register_commands.go -- the daemon forwarders
//
// Every command but show runs as a closure on the worker, so it sees the same
// state the schedule does and a fetch it starts is the only fetch of that set.
// show irr reads the snapshot the worker published, so it answers while the
// worker is fetching.

package irr

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const statusDone = "done"

// handleCommand dispatches one declared command.
func (o *owner) handleCommand(command string, args []string) (string, any, error) {
	var answer commandAnswer
	switch command {
	case cmdShowIRR:
		show, err := o.showIRR()
		if err != nil {
			return statusError, nil, err
		}
		return statusDone, show, nil
	case cmdUpdateIRRAll:
		answer = o.do(o.updateAll)
	case cmdUpdateIRRAsn:
		answer = o.withASN(cmdUpdateIRRAsn, args, o.updateASN)
	case cmdUpdateIRRAsSet:
		answer = o.withASSet(cmdUpdateIRRAsSet, args, o.updateASSet)
	case cmdClearIRRAsn:
		answer = o.withASN(cmdClearIRRAsn, args, o.clearASN)
	case cmdClearIRRAsSet:
		answer = o.withASSet(cmdClearIRRAsSet, args, o.clearASSet)
	default:
		return statusError, nil, errUnknownCommand
	}
	if answer.err != nil {
		return statusError, answer.data, answer.err
	}
	return statusDone, answer.data, nil
}

// withASN parses the one AS number argument and runs fn on the worker.
func (o *owner) withASN(command string, args []string, fn func(context.Context, uint32) commandAnswer) commandAnswer {
	if len(args) != 1 {
		return commandAnswer{err: errArgument(command, "<asn>")}
	}
	number, err := asn.Parse(args[0])
	if err != nil {
		return commandAnswer{err: opererr.Wrap(err, command+": "+args[0]+" is not an AS number",
			"The command names one AS number, in decimal or asdot notation.", command+" 65001")}
	}
	return o.do(func(ctx context.Context) commandAnswer { return fn(ctx, number) })
}

// withASSet parses the one AS-SET argument and runs fn on the worker.
func (o *owner) withASSet(command string, args []string, fn func(context.Context, string) commandAnswer) commandAnswer {
	if len(args) != 1 {
		return commandAnswer{err: errArgument(command, "<as-set>")}
	}
	asSet, err := store.ASSetName(args[0])
	if err != nil {
		return commandAnswer{err: opererr.Wrap(err, command+": "+args[0]+" cannot name a stored AS-SET",
			"An AS-SET is stored under its name, so a name no key can hold is never fetched.", command+" AS-EXAMPLE")}
	}
	return o.do(func(ctx context.Context) commandAnswer { return fn(ctx, asSet) })
}

func errArgument(command, operand string) error {
	return opererr.New(command+" needs one "+operand,
		"The command acts on exactly one name.", command+" "+operand)
}

// updateASSet fetches asSet now, whether or not a consumer names it, and
// releases a `clear irr` hold (AC-15).
func (o *owner) updateASSet(ctx context.Context, asSet string) commandAnswer {
	delete(o.heldSets, asSet)
	err := o.refreshSet(ctx, asSet)
	o.plan()
	return o.fetchAnswer(asSet, err)
}

// updateASN asks PeeringDB for the set of number, stores the binding, then
// fetches that set now.
func (o *owner) updateASN(ctx context.Context, number uint32) commandAnswer {
	delete(o.heldASNs, number)
	o.discoverASN(ctx, number)
	// The set to fetch is the one the binding moves to, which writeDataset
	// binds once it is stored; else the one it names.
	target, moving := o.movingTo[number]
	if !moving {
		binding, ok := o.validBinding(number)
		if !ok {
			// discoverASN binds the AS number on every path but a panic,
			// which discoverPanicked logged and raised.
			return commandAnswer{err: opererr.New("update irr asn "+asnText(number)+": the PeeringDB question about the AS number failed",
				"A defect in the irr plugin stopped the question, so no AS-SET is bound and nothing was fetched; the log holds the panic.",
				"update irr asn "+asnText(number))}
		}
		target = binding.ASSet
	}
	answer := o.updateASSet(ctx, target)
	if row, ok := answer.data.(datasetRow); ok {
		return commandAnswer{data: asnAnswer{ASN: number, datasetRow: row}, err: answer.err}
	}
	return answer
}

// updateAll asks PeeringDB again for every referenced AS number and fetches
// every referenced set now, releasing every hold. It answers every row, and
// an error naming each set that failed.
func (o *owner) updateAll(ctx context.Context) commandAnswer {
	clear(o.heldSets)
	clear(o.heldASNs)
	for number := range o.wantedASNs() {
		o.discoverASN(ctx, number)
	}
	wanted := o.wantedSets()
	sets := make([]string, 0, len(wanted))
	for asSet := range wanted {
		sets = append(sets, asSet)
	}
	slices.Sort(sets)
	rows := make([]datasetRow, 0, len(sets))
	var failed []string
	for _, asSet := range sets {
		if ctx.Err() != nil {
			return commandAnswer{err: ctx.Err()}
		}
		if err := o.refreshSet(ctx, asSet); err != nil && !isEmptyAnswer(err) {
			failed = append(failed, asSet)
		}
		rows = append(rows, o.rowOf(asSet, o.clock(), o.consumersOf(asSet)))
	}
	o.plan()
	data := map[string]any{"datasets": rows}
	if len(failed) > 0 {
		return commandAnswer{data: data, err: opererr.New("update irr all: "+joinNames(failed)+" did not refresh",
			"Each keeps the list stored before, and is fetched again every minute until the server answers.",
			"show irr")}
	}
	return commandAnswer{data: data}
}

// fetchAnswer is the answer of an update of asSet: its row, or the refusal.
func (o *owner) fetchAnswer(asSet string, err error) commandAnswer {
	_, stored := o.datasets[asSet]
	if err == nil {
		return commandAnswer{data: o.rowOf(asSet, o.clock(), o.consumersOf(asSet))}
	}
	if isEmptyAnswer(err) && stored {
		// The server named nothing: the stored list stays enforced, stale.
		return commandAnswer{data: o.rowOf(asSet, o.clock(), o.consumersOf(asSet))}
	}
	return commandAnswer{err: opererr.Wrap(err, "update irr as-set "+asSet+" stored nothing",
		"The IRR server gave no list Ze can store, so the list stored before, if any, stays enforced.",
		"update irr as-set "+asSet)}
}

// clearASSet removes the stored copy of asSet and names every consumer still
// referencing it (AC-16). The set is not fetched again until `update irr` or
// an interest that newly names it.
func (o *owner) clearASSet(ctx context.Context, asSet string) commandAnswer {
	consumers := o.consumersOf(asSet)
	removed, err := o.removeSet(ctx, asSet)
	if errors.Is(err, errRemovalUnannounced) {
		o.holdCleared(asSet, consumers)
		return commandAnswer{err: errUnannounced("clear irr as-set "+asSet, asSet, err)}
	}
	if err != nil {
		return commandAnswer{err: opererr.Wrap(err, "clear irr as-set "+asSet+" could not remove the stored copy",
			"The store refused the removal, so the consumers keep enforcing it.", "clear irr as-set "+asSet)}
	}
	o.holdCleared(asSet, consumers)
	return commandAnswer{data: clearAnswer{ASSet: asSet, Removed: removed, Consumers: nonNil(consumers)}}
}

// holdCleared holds a cleared set that consumers still reference, so no pass
// fetches it again (AC-16).
func (o *owner) holdCleared(asSet string, consumers []string) {
	if len(consumers) > 0 {
		o.heldSets[asSet] = struct{}{}
	}
}

// errUnannounced is the answer of a clear whose removal the consumers were
// not told of: the stored copy is gone, and the same clear tells them.
func errUnannounced(command, asSet string, err error) error {
	return opererr.Wrap(err, command+" removed the stored copy of "+asSet+", but the consumers were not told",
		"The store refused to raise the IRR generation, so each consumer keeps enforcing the removed list until it is told.",
		"clear irr as-set "+asSet)
}

// clearASN removes the binding of number and the stored copy of the set it
// names, and names every consumer still referencing either.
func (o *owner) clearASN(ctx context.Context, number uint32) commandAnswer {
	binding, bound := o.bindings[number]
	consumers := o.consumersOfASN(number)
	if bound {
		consumers = mergeNames(consumers, o.consumersOf(binding.ASSet))
	}
	if err := o.state.StateRemove(ctx, store.BindingKey(number)); err != nil {
		return commandAnswer{err: opererr.Wrap(err, "clear irr asn "+asnText(number)+" could not remove the stored binding",
			"The store refused the removal, so the AS number keeps its AS-SET.", "clear irr asn "+asnText(number))}
	}
	delete(o.bindings, number)
	delete(o.discoverDue, number)
	delete(o.movingTo, number)
	answer := clearAnswer{ASN: number, Removed: bound, Consumers: nonNil(consumers)}
	if bound {
		answer.ASSet = binding.ASSet
		_, err := o.removeSet(ctx, binding.ASSet)
		if err != nil && !errors.Is(err, errRemovalUnannounced) {
			return commandAnswer{err: opererr.Wrap(err, "clear irr asn "+asnText(number)+" could not remove the stored copy of "+binding.ASSet,
				"The store refused the removal, so the consumers keep enforcing it.", "clear irr as-set "+binding.ASSet)}
		}
		o.holdCleared(binding.ASSet, o.consumersOf(binding.ASSet))
		if err != nil {
			if len(consumers) > 0 {
				o.heldASNs[number] = struct{}{}
			}
			return commandAnswer{err: errUnannounced("clear irr asn "+asnText(number), binding.ASSet, err)}
		}
	}
	if len(consumers) > 0 {
		o.heldASNs[number] = struct{}{}
	}
	return commandAnswer{data: answer}
}

// clearAnswer is the answer of clear irr asn|as-set. Consumers lists every
// consumer whose interest still references what was removed; they stop
// enforcing it on the notice.
type clearAnswer struct {
	ASN       uint32   `json:"asn,omitempty"`
	ASSet     string   `json:"as-set,omitempty"`
	Removed   bool     `json:"removed"`
	Consumers []string `json:"consumers"`
}

// asnAnswer is the answer of update irr asn: the AS number and its set's row.
type asnAnswer struct {
	ASN uint32 `json:"asn"`
	datasetRow
}

// datasetRow is one row of show irr, and the answer of an update.
type datasetRow struct {
	ASSet       string   `json:"as-set"`
	Server      string   `json:"server,omitempty"`
	Status      string   `json:"status"`
	IPv4Count   int      `json:"ipv4-count"`
	IPv6Count   int      `json:"ipv6-count"`
	RefreshedAt string   `json:"refreshed-at,omitempty"`
	CheckedAt   string   `json:"checked-at,omitempty"`
	StaleSince  string   `json:"stale-since,omitempty"`
	NextRefresh string   `json:"next-refresh"`
	Generation  uint32   `json:"generation,omitempty"`
	Consumers   []string `json:"consumers"`
}

// The row statuses.
const (
	rowStored  = "stored"
	rowStale   = "stale"
	rowPending = "pending"
	rowFailing = "failing"
	rowHeld    = "cleared"
	// nextManual is next-refresh with refresh-interval 0 (AC-10).
	nextManual = "manual"
	// nextNone is next-refresh of a set nothing schedules: cleared, or stored
	// by update irr and named by no consumer.
	nextNone = "none"
)

// rowOf builds the show irr row of asSet from the worker's state; consumers
// names the consumers that reference it.
func (o *owner) rowOf(asSet string, now time.Time, consumers []string) datasetRow {
	row := datasetRow{ASSet: asSet, Status: rowPending, Consumers: nonNil(consumers), NextRefresh: nextNone}
	if info, ok := o.datasets[asSet]; ok {
		row.Server, row.Status = info.server, rowStored
		row.IPv4Count, row.IPv6Count = info.ipv4Count, info.ipv6Count
		row.RefreshedAt, row.CheckedAt, row.StaleSince = stamp(info.refreshedAt), stamp(info.checkedAt), stamp(info.staleSince)
		if !info.staleSince.IsZero() {
			row.Status = rowStale
		}
	}
	if pointer, ok := o.pointers[asSet]; ok {
		row.Generation = pointer.Generation
	}
	if _, failing := o.retryAt[asSet]; failing {
		row.Status = rowFailing
	}
	if _, held := o.heldSets[asSet]; held {
		row.Status = rowHeld
	}
	if at, ok := o.schedule.at(asSet); ok {
		if at.Before(now) {
			at = now
		}
		row.NextRefresh = stamp(at)
	} else if o.cfg.manual() {
		row.NextRefresh = nextManual
	}
	return row
}

// showAnswer is what show irr answers: the settings, then one row per stored
// or referenced AS-SET.
type showAnswer struct {
	Server          string       `json:"server"`
	RefreshInterval uint32       `json:"refresh-interval"`
	Schedule        string       `json:"schedule"`
	Datasets        []datasetRow `json:"datasets"`
}

// showIRR answers the snapshot the worker published, and refuses before the
// first one: the zero showAnswer names no server and interval 0, which is
// not the config, and no dataset, which is not what the store holds.
func (o *owner) showIRR() (showAnswer, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.view.published {
		return showAnswer{}, errNotLoaded()
	}
	return o.view.show, nil
}

// stamp writes t as RFC 3339 UTC, and an unset time as nothing.
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func asnText(number uint32) string {
	return textbuf.StringUint32(number)
}

func nonNil(names []string) []string {
	if names == nil {
		return []string{}
	}
	return names
}

// mergeNames returns the sorted union of two sorted name lists.
func mergeNames(first, second []string) []string {
	merged := append(slices.Clone(first), second...)
	slices.Sort(merged)
	return slices.Compact(merged)
}

func joinNames(names []string) string {
	var tb textbuf.Buffer
	for i, name := range names {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(name)
	}
	return tb.String()
}
