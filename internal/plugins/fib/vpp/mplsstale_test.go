// Design: docs/architecture/mpls/mpls-kernel.md -- label-switching entries on the VPP data plane, stale-label sweep tests
package fibvpp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/mpls"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
)

// testMPLSDumpCtx answers an MplsRouteDump from the channel's mplsDump, one
// route per reply, then the end of the dump.
type testMPLSDumpCtx struct {
	ch    *testChannel
	msg   api.Message
	index int
}

func (r *testMPLSDumpCtx) ReceiveReply(reply api.Message) (bool, error) {
	if r.ch.sendErr != nil {
		return false, r.ch.sendErr
	}
	if _, ok := r.msg.(*mpls.MplsRouteDump); !ok {
		return false, fmt.Errorf("testChannel serves no dump of %T", r.msg)
	}
	details, ok := reply.(*mpls.MplsRouteDetails)
	if !ok {
		return false, fmt.Errorf("MplsRouteDump reply decoded into %T", reply)
	}
	if r.index == len(r.ch.mplsDump) {
		return true, nil
	}
	details.MrRoute = r.ch.mplsDump[r.index]
	r.index++
	return false, nil
}

// bothPositions is the two VPP routes of one Ze swap or pop entry.
func bothPositions(label uint32) []mpls.MplsRoute {
	return []mpls.MplsRoute{
		{MrTableID: mplsTableZero, MrLabel: label, MrEos: 1},
		{MrTableID: mplsTableZero, MrLabel: label, MrEos: 0},
	}
}

// labelDeletes lists the MplsRouteAddDel deletions sent from index from on.
func labelDeletes(ch *testChannel, from int) []labelPosition {
	var out []labelPosition
	for _, m := range ch.requests[from:] {
		if req, ok := m.(*mpls.MplsRouteAddDel); ok && !req.MrIsAdd {
			out = append(out, labelPosition{inLabel: req.MrRoute.MrLabel, eos: req.MrRoute.MrEos})
		}
	}
	return out
}

// TestFibVPPSweepRemovesOnlyUnclaimedLabels proves the stale-label sweep
// removes exactly the label routes a previous run left in MPLS table 0 that no
// source re-claimed: an in-label a source re-applies in answer to the replay
// stays, a reserved label (VPP's own entry) and a multicast route are never
// touched, and the leftover goes at both stack positions.
//
// VALIDATES: Phase 13 stale-label sweep -- after a Ze restart with VPP still
// running, a label route nobody advertises stops forwarding.
// METHOD: the real GoVPP MPLS backend over the recording fake channel. The
// dump returns label 100 (stale), 200 (re-claimed by the replay), label 0 and
// a multicast 300; a replay subscriber re-applies 200 with the writer's token,
// as LDP does; then sweepStaleLabels.
func TestFibVPPSweepRemovesOnlyUnclaimedLabels(t *testing.T) {
	w := newLabelWriter(t, 0)
	w.ch.mplsDump = append(w.ch.mplsDump, bothPositions(100)...)
	w.ch.mplsDump = append(w.ch.mplsDump, bothPositions(200)...)
	w.ch.mplsDump = append(w.ch.mplsDump,
		mpls.MplsRoute{MrTableID: mplsTableZero, MrLabel: 0, MrEos: 1},
		mpls.MplsRoute{MrTableID: mplsTableZero, MrLabel: 300, MrEos: 1, MrIsMulticast: true})
	unsub := mplsfib.ReplayRequest.Subscribe(w.bus, func(r *replay.Request) {
		assert.NoError(t, mplsfib.ApplyReplay(w.bus, r.ReplayID, []mplsfib.Entry{swapEntry(200, []uint32{201})}))
	})
	t.Cleanup(unsub)

	w.fib.requestLabelReplay(w.bus)
	var dumps []*mpls.MplsRouteDump
	for _, m := range w.ch.requests {
		if d, ok := m.(*mpls.MplsRouteDump); ok {
			dumps = append(dumps, d)
		}
	}
	require.Len(t, dumps, 1, "the writer reads MPLS table 0 once when it opens")
	assert.Equal(t, uint32(mplsTableZero), dumps[0].Table.MtTableID)

	before := len(w.ch.requests)
	w.fib.sweepStaleLabels()
	assert.Equal(t, []labelPosition{{inLabel: 100, eos: 1}, {inLabel: 100, eos: 0}}, labelDeletes(w.ch, before),
		"only the unclaimed leftover is removed, at both stack positions")
	assert.Empty(t, w.fib.labelDoctorFindings(), "a sweep VPP accepted leaves nothing to report")

	before = len(w.ch.requests)
	w.fib.sweepStaleLabels()
	assert.Empty(t, labelDeletes(w.ch, before), "a second sweep finds nothing left to remove")
}

// TestFibVPPSweepSparesLabelClaimedAfterMark proves an in-label a source
// installs between the mark and the sweep, outside any replay, is the
// source's: the sweep leaves it, even after the source removed it again.
//
// VALIDATES: Phase 13 stale-label sweep -- the sweep never removes a route a
// source installed in this run.
// METHOD: mark with label 100 in the dump, apply and then withdraw 100, sweep.
func TestFibVPPSweepSparesLabelClaimedAfterMark(t *testing.T) {
	w := newLabelWriter(t, 0)
	w.ch.mplsDump = bothPositions(100)
	w.fib.requestLabelReplay(w.bus)
	require.NoError(t, w.apply(swapEntry(100, []uint32{101})))
	remove := swapEntry(100, nil)
	remove.Action = mplsfib.ActionRemove
	require.NoError(t, w.apply(remove))
	before := len(w.ch.requests)
	w.fib.sweepStaleLabels()
	assert.Empty(t, labelDeletes(w.ch, before))
}

// TestFibVPPSweepFailureReachesDoctor proves a leftover VPP refuses to remove
// is reported by the fib-vpp-label-entries check as an error under
// doctor-fib-vpp-mpls-stale-label, with the in-label, VPP's refusal and the
// command that runs the sweep again; and that a table 0 that cannot be read
// is reported the same way.
//
// VALIDATES: Phase 13 stale-label sweep -- a leftover that still forwards
// reaches `show doctor`, not only the log.
// METHOD: the mock MPLS backend with a dump of label 100 and a failing
// delete, then a second writer whose dump fails.
func TestFibVPPSweepFailureReachesDoctor(t *testing.T) {
	refusal := errors.New("mpls MplsRouteAddDel label 100 eos 1 retval=-6")
	backend := &mockMPLSBackend{dumped: []labelPosition{{inLabel: 100, eos: 1}}, labelDelErr: refusal}
	f := newFibVPP(&mockBackend{})
	f.mplsBackend = backend
	f.requestLabelReplay(&syncBus{})
	f.sweepStaleLabels()
	require.Equal(t, []labelPosition{{inLabel: 100, eos: 1}}, backend.positionDels)

	findings := f.labelDoctorFindings()
	require.Len(t, findings, 1)
	assert.Equal(t, codeStaleLabel, findings[0].Code)
	assert.Equal(t, string(diagnostic.SeverityError), findings[0].Severity)
	assert.Contains(t, findings[0].Message, "in-label 100")
	assert.Contains(t, findings[0].Message, refusal.Error())
	assert.Equal(t, staleLabelFix, findings[0].Fix)
	assert.NotNil(t, diagnostic.Lookup(codeStaleLabel), "ze show diagnostic describes the code")

	reclaim := swapEntry(100, []uint32{101})
	require.NoError(t, f.applyLabelEntryLocked(&reclaim))
	assert.Empty(t, f.labelDoctorFindings(), "a source that re-claims the in-label owns the route again")

	unreadable := errors.New("VPP API timeout")
	g := newFibVPP(&mockBackend{})
	g.mplsBackend = &mockMPLSBackend{dumpErr: unreadable}
	g.requestLabelReplay(&syncBus{})
	findings = g.labelDoctorFindings()
	require.Len(t, findings, 1)
	assert.Equal(t, codeStaleLabel, findings[0].Code)
	assert.Contains(t, findings[0].Message, unreadable.Error())
}

// TestFibVPPSweepFindingsBounded proves one answer lists at most
// labelFindingsMax leftovers and counts the rest.
//
// VALIDATES: Phase 13 stale-label sweep -- the doctor answer has a bound.
// METHOD: labelFindingsMax+3 failing positions.
func TestFibVPPSweepFindingsBounded(t *testing.T) {
	f := newFibVPP(&mockBackend{})
	f.labelSweepFailures = make(map[labelPosition]error)
	for i := range labelFindingsMax + 3 {
		f.labelSweepFailures[labelPosition{inLabel: uint32(1000 + i), eos: 1}] = errors.New("refused")
	}
	findings := f.labelDoctorFindings()
	require.Len(t, findings, labelFindingsMax+1)
	assert.Contains(t, findings[0].Message, "in-label 1000 ")
	assert.Contains(t, findings[labelFindingsMax].Message, "3 more")
}
