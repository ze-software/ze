// Design: docs/architecture/mpls/mpls-kernel.md -- a label entry no writer installed
// Related: doctor.go -- CodeDoctorNoWriter, the code every finding carries
// Related: events.go -- Apply and ApplyReplay, whose refusal a source records here
//
// Every label source (LDP, RSVP-TE, OSPF Segment Routing, BGP EPE) learns
// from Apply or ApplyReplay that no writer carried out an entry it wants, and
// answers its own runtime doctor check with those entries. Refusals is the
// one record they keep: an entry per data-plane slot with the writer's cause,
// rendered as bounded doctor findings. The record lives in the source's own
// process, which may be a forked one, so the source answers the check itself
// through OnDoctorCheck.

package mplsfib

import (
	"cmp"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// refusalFindingsMax bounds the findings one answer lists: a source can want
// an entry per FEC of a large IGP, and one summary finding counts the rest.
const refusalFindingsMax = 32

// refusalSeverity is the doctor severity of an entry the data plane lacks:
// labeled traffic the source advertises a label for is not forwarded.
const refusalSeverity = string(diagnostic.SeverityError)

// refusalSlot is the data-plane slot an entry occupies: a push by FEC and
// table, a swap or pop by in-label. A writer holds one entry per slot, so a
// later record of the same slot replaces the earlier one.
type refusalSlot struct {
	push    bool
	inLabel uint32
	fec     netip.Prefix
	tableID uint32
}

func slotOf(e *Entry) refusalSlot {
	if e.Op == OpPush {
		return refusalSlot{push: true, fec: e.FEC, tableID: e.TableID}
	}
	return refusalSlot{inLabel: e.InLabel}
}

// refusal is one entry no writer carried out, and the writer's cause.
type refusal struct {
	entry Entry
	cause string
}

// Refusals records the label entries a source wants that no writer of the
// selected data plane carried out, each with the cause, until the source
// clears it after a later apply succeeds. The zero value is an empty record.
// Not safe for concurrent use: the source's own lock guards it.
type Refusals struct {
	held map[refusalSlot]refusal
}

// Record keeps e as not carried out, for cause. The entry is copied.
func (r *Refusals) Record(e *Entry, cause error) {
	if r.held == nil {
		r.held = make(map[refusalSlot]refusal)
	}
	stored := *e
	stored.OutLabels = slices.Clone(e.OutLabels)
	r.held[slotOf(e)] = refusal{entry: stored, cause: cause.Error()}
}

// Clear forgets the slot e occupies, once a writer carried out an entry for
// it.
func (r *Refusals) Clear(e *Entry) {
	delete(r.held, slotOf(e))
}

// ClearIf forgets every recorded entry drop answers true for.
func (r *Refusals) ClearIf(drop func(*Entry) bool) {
	for slot, held := range r.held {
		if drop(&held.entry) {
			delete(r.held, slot)
		}
	}
}

// Reset forgets every recorded entry, once a writer carried out all of them.
func (r *Refusals) Reset() {
	clear(r.held)
}

// Findings answers a source's label-entries doctor check: one ERROR under
// CodeDoctorNoWriter per recorded entry, swaps and pops by in-label then
// pushes by FEC and table, at most refusalFindingsMax of them and one more
// counting the rest. source opens each message and fix is the source's
// remedy. An empty record answers nil.
func (r *Refusals) Findings(source, fix string) []rpc.DoctorCheckDiagnostic {
	if len(r.held) == 0 {
		return nil
	}
	refusals := make([]refusal, 0, len(r.held))
	for _, held := range r.held {
		refusals = append(refusals, held)
	}
	slices.SortFunc(refusals, compareRefusal)

	shown := min(len(refusals), refusalFindingsMax)
	findings := make([]rpc.DoctorCheckDiagnostic, 0, shown+1)
	for i := range shown {
		findings = append(findings, rpc.DoctorCheckDiagnostic{
			Code:     CodeDoctorNoWriter,
			Severity: refusalSeverity,
			Message:  refusalMessage(source, &refusals[i]),
			Fix:      fix,
		})
	}
	if rest := len(refusals) - shown; rest > 0 {
		var tb textbuf.Buffer
		tb.Str(source).Str(": ").Int(int64(rest)).Str(" more label entries are not installed in the data plane the way ").
			Str(source).Str(" wants them")
		findings = append(findings, rpc.DoctorCheckDiagnostic{
			Code:     CodeDoctorNoWriter,
			Severity: refusalSeverity,
			Message:  tb.String(),
			Fix:      fix,
		})
	}
	return findings
}

// refusalMessage names the entry, what the data plane holds instead, and the
// writer's cause in one line.
func refusalMessage(source string, r *refusal) string {
	var tb textbuf.Buffer
	tb.Str(source)
	if r.entry.Action == ActionRemove {
		tb.Str(": the data plane may still hold the label ")
	} else {
		tb.Str(": the data plane does not hold the label ")
	}
	switch r.entry.Op {
	case OpPush:
		tb.Str("push for FEC ").Str(r.entry.FEC.String())
		if r.entry.TableID != 0 {
			tb.Str(" in table ").Int(int64(r.entry.TableID))
		}
	case OpSwap:
		tb.Str("swap of in-label ").Int(int64(r.entry.InLabel))
	case OpPop:
		tb.Str("pop of in-label ").Int(int64(r.entry.InLabel))
		if r.entry.FEC.IsValid() {
			tb.Str(" (FEC ").Str(r.entry.FEC.String()).Str(")")
		}
	case OpUnspecified:
		// Apply refuses an entry with no operation, so a source records one
		// only from a writer's answer; name it by its in-label.
		tb.Str("entry of in-label ").Int(int64(r.entry.InLabel))
	}
	if r.entry.NextHop.IsValid() {
		tb.Str(" toward ").Str(r.entry.NextHop.String())
	}
	if r.entry.Action == ActionRemove {
		tb.Str(" that ").Str(source).Str(" withdrew; the writer did not remove it: ")
	} else {
		tb.Str("; the writer did not install it: ")
	}
	tb.Str(r.cause)
	return tb.String()
}

// compareRefusal orders swaps and pops by in-label, before pushes by FEC
// address, length and table.
func compareRefusal(a, b refusal) int {
	aPush := a.entry.Op == OpPush
	bPush := b.entry.Op == OpPush
	if aPush != bPush {
		if aPush {
			return 1
		}
		return -1
	}
	if !aPush {
		return cmp.Compare(a.entry.InLabel, b.entry.InLabel)
	}
	if c := a.entry.FEC.Addr().Compare(b.entry.FEC.Addr()); c != 0 {
		return c
	}
	if c := cmp.Compare(a.entry.FEC.Bits(), b.entry.FEC.Bits()); c != 0 {
		return c
	}
	return cmp.Compare(a.entry.TableID, b.entry.TableID)
}
