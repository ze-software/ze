// Design: docs/architecture/core-design.md -- final-only export policy reconciliation.
// Related: stale_readvertise_chain.go -- staleChainFor, runStaleChain, noteExportWrite
package reactor

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// writePreFiltered writes the batch on the rail of a decision taken ahead of the
// writer, stamped with generation, and answers its receipt.
func (f *staleChangeFixture) writePreFiltered(t *testing.T, generation uint64) uint64 {
	t.Helper()
	facts := announceFactsFor(f.peer, f.batch.Family, netip.MustParseAddr("10.0.0.1"), false, f.peer.negotiated.Load())
	update, err := f.a.buildBatchAnnounceUpdate(make([]byte, 4096), make([]byte, 4096), f.batch, facts)
	if err != nil {
		t.Fatal(err)
	}
	target := announceTarget{peer: f.peer, session: f.peer.session, exportGeneration: generation}
	if err := target.sendBatchUpdatePreFiltered(t.Context(), update, nil, message.MaxMsgLen, facts.addPath, &f.batch); err != nil {
		t.Fatal(err)
	}
	return f.peer.session.sentReceipt.MessageID()
}

// TestStaleRailAnswerAfterExportChange proves what a re-judged route gets: the
// whole export chain, the opted-in members last, and a withdrawal when the chain
// rejects a route with a sent receipt. A route with no receipt is withdrawn when the
// key holds it as this speaker's own, and withheld when the key holds another
// source's route, whether the chain rejected it or a member answered withdraw. That
// fence does not depend on the watermark: a Peer the reactor built starts above zero,
// and the no-watermark cases hold the fence to the receipt alone, with only the
// opted-in member asked. A route the current policy already judged runs only the
// opted-in members.
//
// VALIDATES: the current policy judges a route it never saw, by analogy with RFC
// 2918 Section 4, and RFC 4271 Section 9.1.3 withdraws what it excludes.
// PREVENTS: a route a reload's new member rejects staying on the wire with no
// LLGR_STALE through a stale pass (RFC 9494 Section 4.3), either unasked because
// the opted-in subset never asks that member, or asked and left where it is; and
// an unowned withdrawal removing a fresher route another source put under the key.
func TestStaleRailAnswerAfterExportChange(t *testing.T) {
	const keptBody = "000000184001010040020a02020000fde80000fde94003040a000001180a0000"
	const withdrawnBody = "0004180a00000000"
	cases := []struct {
		name          string
		nextMembers   []string
		noReload      bool // the export policy never changed, so the watermark stays zero
		seedAfter     bool // the route was sent after the reload
		unknown       bool // the batch names no sent owner
		foreign       bool // another source's route has taken the key since the send
		withdraws     bool // "ordinary" answers withdraw rather than reject
		railWithdraws bool // "stale" answers withdraw rather than accept
		wantCalls     []string
		wantBody      string
	}{
		{name: "sent-before-change-rejected", nextMembers: []string{"stale", "ordinary"},
			wantCalls: []string{"ordinary"}, wantBody: withdrawnBody},
		{name: "sent-before-change-accepted", nextMembers: []string{"stale", "passes"},
			wantCalls: []string{"passes", "stale"}, wantBody: keptBody},
		{name: "unknown-owner-after-change", nextMembers: []string{"stale", "ordinary"}, unknown: true,
			wantCalls: []string{"ordinary"}, wantBody: withdrawnBody},
		{name: "unknown-owner-another-sources-route", nextMembers: []string{"stale", "ordinary"}, unknown: true, foreign: true,
			wantCalls: []string{"ordinary"}},
		{name: "unknown-owner-member-withdraws", nextMembers: []string{"stale", "ordinary"}, unknown: true, withdraws: true,
			wantCalls: []string{"ordinary"}, wantBody: withdrawnBody},
		{name: "unknown-owner-member-withdraws-another-sources-route", nextMembers: []string{"stale", "ordinary"},
			unknown: true, foreign: true, withdraws: true, wantCalls: []string{"ordinary"}},
		{name: "no-watermark-unknown-owner-member-withdraws", noReload: true, unknown: true, railWithdraws: true,
			wantCalls: []string{"stale"}, wantBody: withdrawnBody},
		{name: "no-watermark-unknown-owner-member-withdraws-another-sources-route", noReload: true,
			unknown: true, foreign: true, railWithdraws: true, wantCalls: []string{"stale"}},
		{name: "sent-after-change", nextMembers: []string{"stale", "ordinary"}, seedAfter: true,
			wantCalls: []string{"stale"}, wantBody: keptBody},
		{name: "reload-rebuilt-the-same-chain", nextMembers: []string{"stale"},
			wantCalls: []string{"stale"}, wantBody: keptBody},
	}
	for _, typed := range []bool{false, true} {
		transportName := "forked"
		if typed {
			transportName = "typed"
		}
		for _, tc := range cases {
			t.Run(transportName+"/"+tc.name, func(t *testing.T) {
				f := newStaleChangeFixture(t, typed)
				next := &PeerSettings{ExportPolicy: answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, tc.nextMembers...)}
				var owner uint64
				if !tc.seedAfter {
					owner = f.seedOwner(t)
				}
				if !tc.noReload {
					mustApplyHotSwappableSettings(t, f.peer, next, exportPolicySwap)
				}
				if tc.seedAfter {
					owner = f.seedOwner(t)
				}
				if !tc.unknown {
					f.batch.SentOwnerMessage = owner
				}
				f.batch.Replay = true
				if tc.withdraws {
					f.answers = map[string]rpc.FilterUpdateOutput{"ordinary": {Action: rpc.FilterWithdraw}}
				}
				if tc.railWithdraws {
					f.answers = map[string]rpc.FilterUpdateOutput{"stale": {Action: rpc.FilterWithdraw}}
				}
				if tc.noReload && staleChainFor(f.peer, 0).judgeAll {
					t.Fatal("a no-watermark case ran under a chain that judges the whole route")
				}
				if tc.foreign {
					f.takeKeysForAnotherSource(t)
				}

				before := len(f.conn.written())
				sent, err := f.a.sendStaleReadvertise(t.Context(), announceTarget{peer: f.peer, session: f.peer.session},
					f.batch, netip.MustParseAddr("10.0.0.1"), false, f.peer.negotiated.Load())
				f.callMu.Lock()
				defer f.callMu.Unlock()
				if !slices.Equal(f.calls, tc.wantCalls) {
					t.Fatalf("members asked=%v, want %v", f.calls, tc.wantCalls)
				}
				if tc.wantBody == "" {
					if err != nil || sent != 0 || len(f.conn.written()) != before {
						t.Fatalf("send=%d err=%v wrote=%d: another source's route must not be withdrawn", sent, err, len(f.conn.written())-before)
					}
					if f.heldRoutes() != 1 {
						t.Fatalf("Adj-RIB-Out holds %d routes, want the other source's one left in place", f.heldRoutes())
					}
					return
				}
				if err != nil || sent != 1 {
					t.Fatalf("send=%d err=%v, want one message accepted", sent, err)
				}
				bodies := updateBodies(t, f.conn.written()[before:])
				if len(bodies) != 1 || !bytes.Equal(bodies[0], nlriHex(t, tc.wantBody)) {
					t.Fatalf("stale output=%x, want exactly %s", bodies, tc.wantBody)
				}
			})
		}
	}
}

// TestStaleRailWithdrawalOfAnEmptyKey proves a withdrawal naming no sent receipt sends
// nothing when the key holds no route on this session, on the whole-chain arm and on
// the opted-in member's arm alike. Method: run the rail with nothing seeded, then read
// the wire and the Adj-RIB-Out.
//
// VALIDATES: adjOutWrite.allow's own-route check on a key with no entry.
// PREVENTS: a withdrawal of a prefix this session never advertised reaching the peer,
// or a nil entry panicking the writer.
func TestStaleRailWithdrawalOfAnEmptyKey(t *testing.T) {
	cases := []struct {
		name      string
		reload    bool
		wantCalls []string
	}{
		{name: "whole-chain-rejects", reload: true, wantCalls: []string{"ordinary"}},
		{name: "member-withdraws", wantCalls: []string{"stale"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStaleChangeFixture(t, true)
			if tc.reload {
				next := &PeerSettings{ExportPolicy: answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, "stale", "ordinary")}
				mustApplyHotSwappableSettings(t, f.peer, next, exportPolicySwap)
			} else {
				f.answers = map[string]rpc.FilterUpdateOutput{"stale": {Action: rpc.FilterWithdraw}}
			}
			f.batch.Replay = true

			before := len(f.conn.written())
			sent, err := f.a.sendStaleReadvertise(t.Context(), announceTarget{peer: f.peer, session: f.peer.session},
				f.batch, netip.MustParseAddr("10.0.0.1"), false, f.peer.negotiated.Load())
			f.callMu.Lock()
			defer f.callMu.Unlock()
			if !slices.Equal(f.calls, tc.wantCalls) {
				t.Fatalf("members asked=%v, want %v", f.calls, tc.wantCalls)
			}
			if err != nil || sent != 0 || len(f.conn.written()) != before {
				t.Fatalf("send=%d err=%v wrote=%d: an empty key has nothing to withdraw", sent, err, len(f.conn.written())-before)
			}
			if f.heldRoutes() != 0 {
				t.Fatalf("Adj-RIB-Out holds %d routes, want none", f.heldRoutes())
			}
		})
	}
}

// takeKeysForAnotherSource hands every key the session holds to a route from another
// source, as a fresher forward does.
func (f *staleChangeFixture) takeKeysForAnotherSource(t *testing.T) {
	t.Helper()
	table := f.peer.session.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	if len(table.routes) == 0 {
		t.Fatal("the session holds no route for another source to take")
	}
	other := &Peer{}
	for key, entry := range table.routes {
		table.setOwner(&key, entry, adjOutPath{source: other})
	}
}

// heldRoutes answers how many routes the session's Adj-RIB-Out holds.
func (f *staleChangeFixture) heldRoutes() int {
	table := f.peer.session.adjOut
	table.mu.Lock()
	defer table.mu.Unlock()
	return len(table.routes)
}

// TestOldDecisionWrittenAfterChangeReadsAsOld proves a receipt reads as current
// only when the decision behind it read the current chain, whatever the order of
// the write and the reload. Each case decides, then lets the policy change, then
// writes, the order no lock forbids: a source goroutine inside the old chain, a
// stale pass whose chain was read before writeMu, or an egress gate reading facts
// the publish has not refreshed yet.
//
// VALIDATES: Session.noteExportWrite over the stamp a decision reads with its chain.
// PREVENTS: an old decision written after the reload holding a receipt above the
// watermark, so a later stale pass keeps the route unjudged by the current policy.
func TestOldDecisionWrittenAfterChangeReadsAsOld(t *testing.T) {
	change := func(t *testing.T, f *staleChangeFixture) {
		t.Helper()
		next := &PeerSettings{ExportPolicy: answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, "stale", "ordinary")}
		mustApplyHotSwappableSettings(t, f.peer, next, exportPolicySwap)
	}

	t.Run("decision-read-the-old-chain", func(t *testing.T) {
		f := newStaleChangeFixture(t, true)
		decided := staleChainFor(f.peer, 0).generation
		change(t, f)
		receipt := f.writePreFiltered(t, decided)
		if !staleChainFor(f.peer, receipt).judgeAll {
			t.Fatal("a write of a decision under the replaced chain read as decided under the new one")
		}
	})
	t.Run("decision-read-the-new-chain", func(t *testing.T) {
		f := newStaleChangeFixture(t, true)
		change(t, f)
		receipt := f.writePreFiltered(t, staleChainFor(f.peer, 0).generation)
		if staleChainFor(f.peer, receipt).judgeAll {
			t.Fatal("a write of a decision under the current chain was re-judged")
		}
	})
	t.Run("no-change-ever", func(t *testing.T) {
		f := newStaleChangeFixture(t, true)
		receipt := f.writePreFiltered(t, staleChainFor(f.peer, 0).generation)
		if f.peer.exportPolicyFrom.Load() != 0 || staleChainFor(f.peer, receipt).judgeAll {
			t.Fatal("a write raised the watermark with no policy change")
		}
	})
	t.Run("unstamped-write-after-change-fails-toward-rejudging", func(t *testing.T) {
		f := newStaleChangeFixture(t, true)
		change(t, f)
		receipt := f.writePreFiltered(t, 0)
		if !staleChainFor(f.peer, receipt).judgeAll {
			t.Fatal("a write nothing stamped read as decided under the current chain")
		}
	})
	t.Run("withdrawal-of-an-old-decision-leaves-the-watermark", func(t *testing.T) {
		f := newStaleChangeFixture(t, true)
		f.seedOwner(t)
		decided := staleChainFor(f.peer, 0).generation
		change(t, f)
		from := f.peer.exportPolicyFrom.Load()
		withdrawal := message.Update{WithdrawnRoutes: []byte{24, 10, 0, 0}}
		target := announceTarget{peer: f.peer, session: f.peer.session, exportGeneration: decided}
		if err := target.sendBatchUpdatePreFiltered(t.Context(), &withdrawal, nil, message.MaxMsgLen, false, &f.batch); err != nil {
			t.Fatal(err)
		}
		if got := f.peer.exportPolicyFrom.Load(); got != from {
			t.Fatalf("watermark moved from %d to %d for a write that left no route to re-judge", from, got)
		}
	})
}

// bucketWrites answers the items of a merge's result that still hold a body. The
// merge keeps each item it consumed in place with no body, because its done() and
// its pooled buffers are still owed, so those items write nothing.
func bucketWrites(items []fwdItem) []fwdItem {
	var writes []fwdItem
	for i := range items {
		if len(items[i].rawBodies) > 0 {
			writes = append(writes, items[i])
		}
	}
	return writes
}

// TestBucketMergeKeepsExportGeneration proves two forwards judged under different
// generations never share a write: the merged section carries one stamp.
//
// PREVENTS: a decision under the replaced chain riding in a section stamped current.
func TestBucketMergeKeepsExportGeneration(t *testing.T) {
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // origin IGP
	first, second := makeTestBody(attrs, []byte{24, 10, 0, 1}), makeTestBody(attrs, []byte{24, 10, 0, 2})
	maxBody := message.MaxMsgLen - message.HeaderLen

	same := bucketWrites(fwdBucketMerge([]fwdItem{
		{rawBodies: [][]byte{first}, exportGeneration: 7},
		{rawBodies: [][]byte{second}, exportGeneration: 7},
	}, maxBody))
	if len(same) != 1 || same[0].exportGeneration != 7 {
		t.Fatalf("same generation: %d sections, want one stamped 7", len(same))
	}

	apart := bucketWrites(fwdBucketMerge([]fwdItem{
		{rawBodies: [][]byte{first}, exportGeneration: 6},
		{rawBodies: [][]byte{second}, exportGeneration: 7},
	}, maxBody))
	if len(apart) != 2 || apart[0].exportGeneration != 6 || apart[1].exportGeneration != 7 {
		t.Fatalf("different generations: %d sections, want two keeping their stamps", len(apart))
	}
}

// TestExportWatermarkAtEveryPublishSite proves each site that publishes an export
// policy, or builds a Peer the RIB's sent routes outlive, sets the watermark.
//
// PREVENTS: a route the previous Peer or policy judged reading as current.
func TestExportWatermarkAtEveryPublishSite(t *testing.T) {
	t.Run("addPeerLocked", func(t *testing.T) {
		tree := map[string]any{
			"connection": map[string]any{
				"local":  map[string]any{"ip": "127.0.0.1", "port": strconv.Itoa(freePort(t))},
				"remote": map[string]any{"ip": "127.0.0.2", "port": strconv.Itoa(freePort(t)), "connect": "false"},
			},
			"session": map[string]any{"asn": map[string]any{"remote": "65001"}},
		}
		ps, err := parsePeerFromTree("peer1", tree, 65000, 0x0a000001)
		if err != nil {
			t.Fatal(err)
		}
		r := New(&Config{Port: freePort(t), LocalAS: 65000, Standalone: true})
		before := nextMsgID()
		if err := r.AddPeer(ps); err != nil {
			t.Fatal(err)
		}
		if from := r.peers[ps.PeerKey()].exportPolicyFrom.Load(); from <= before {
			t.Fatalf("a new Peer's watermark %d does not cover a receipt %d from the Peer before it", from, before)
		}
	})
	t.Run("createDynamicPeer", func(t *testing.T) {
		r := newTestReactor(t)
		dg := newTestDynamicGroup("ix-peers", []string{"185.1.69.0/24"}, 100)
		dg.Settings.Connection = ConnectionPassive
		r.dynamicGroups = []*DynamicGroupConfig{dg}
		before := nextMsgID()
		r.mu.Lock()
		peer, err := r.createDynamicPeer(dg, netip.MustParseAddr("185.1.69.42"))
		r.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		if from := peer.exportPolicyFrom.Load(); from <= before {
			t.Fatalf("a returning dynamic Peer's watermark %d does not cover a receipt %d", from, before)
		}
	})
	t.Run("publishDynamicResolution", func(t *testing.T) {
		f := newStaleChangeFixture(t, true)
		owner := f.seedOwner(t)
		resolved := answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, "stale", "ordinary")
		if !f.peer.publishDynamicResolution(f.peer.readDynamicTemplate(), 65001, filterapi.Policy{}, resolved) {
			t.Fatal("the resolution was not published")
		}
		if f.peer.exportGeneration.Load() != 1 || !staleChainFor(f.peer, owner).judgeAll {
			t.Fatal("a resolution that changed the chain left an older receipt reading as current")
		}

		again := answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, "stale", "ordinary")
		if !f.peer.publishDynamicResolution(f.peer.readDynamicTemplate(), 65001, filterapi.Policy{}, again) {
			t.Fatal("the resolution was not published")
		}
		if f.peer.exportGeneration.Load() != 1 {
			t.Fatal("a resolution to the same chain read as a change")
		}
	})
}

// staleChangeInProcess is a filter type with an evaluator: a compiled member holds
// a func, so a rebuilt chain never compares equal with reflect.DeepEqual.
const (
	staleChangeInProcess = "reactor-stale-change-inprocess"
	staleChangeAttach    = "reactor-stale-change attachment"
)

func init() {
	routepolicy.MustRegisterAttachment("bgp", staleChangeAttach, nil, nil)
	routepolicy.MustRegisterFilterType(staleChangeInProcess, nil, nil)
	policychain.MustRegisterEvaluator(staleChangeInProcess, rpc.OnErrorReject,
		func(string, map[string]any, routepolicy.InstanceResolver) (policychain.Evaluator, error) {
			return func(*policyview.View, *policychain.Sets) (policychain.Decision, error) {
				return policychain.DecisionPass, nil
			}, nil
		})
}

// inProcessPolicy compiles, from a fresh registry as each config parse does, a
// policy with one in-process member under config.
func inProcessPolicy(t *testing.T, config map[string]any) filterapi.Policy {
	t.Helper()
	reg, err := routepolicy.NewRegistry(routepolicy.Instance{Name: "m", Type: staleChangeInProcess, Config: config})
	if err != nil {
		t.Fatal(err)
	}
	chain, err := policychain.Compile(staleChangeAttach, reg, []policychain.Member{{Name: "m"}},
		routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
	if err != nil {
		t.Fatal(err)
	}
	return mustPolicy(t, []routepolicy.Ref{{Name: "m"}}, routepolicy.FinalAccept).WithLibrary(chain)
}

// TestExportChangeSeesThroughARebuiltInProcessChain proves a commit that leaves a
// peer's policy alone does not raise its watermark, though every reload compiles a
// new chain whose members hold new funcs, and that a changed instance does.
//
// VALIDATES: docs/guide/bgp-policy.md -- a route re-judges only when the policy changed.
// PREVENTS: an unrelated commit re-judging every held route, a non-idempotent set
// such as an AS-path prepend applying twice with no policy change behind it.
func TestExportChangeSeesThroughARebuiltInProcessChain(t *testing.T) {
	config := map[string]any{"value": "1"}
	cases := []struct {
		name string
		next map[string]any
		want uint64
	}{
		{name: "rebuilt-unchanged", next: map[string]any{"value": "1"}, want: 0},
		{name: "instance-changed", next: map[string]any{"value": "2"}, want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStaleChangeFixture(t, true)
			f.peer.settings.ExportPolicy = inProcessPolicy(t, config)
			next := &PeerSettings{ExportPolicy: inProcessPolicy(t, tc.next)}
			mustApplyHotSwappableSettings(t, f.peer, next, exportPolicySwap)
			if got := f.peer.exportGeneration.Load(); got != tc.want {
				t.Fatalf("export generation=%d, want %d", got, tc.want)
			}
			if tc.want == 0 && f.peer.exportPolicyFrom.Load() != 0 {
				t.Fatal("a rebuilt chain with no change raised the watermark")
			}
		})
	}
}

// llgrStaleBody is an IPv4 UPDATE body holding one route marked LLGR_STALE.
const llgrStaleBody = "0000000e" + "40010100" + "400200" + "c00804ffff0006" + "180a0000"

// staleKeepFixture is a stale rail chain of the opted-in "stale" and the member
// other, whose answers are the arguments, over a fixture that may set community
// and local-preference.
func staleKeepFixture(t *testing.T, stale, other rpc.FilterUpdateOutput) (*staleChangeFixture, staleChain) {
	t.Helper()
	f := newStaleChangeFixture(t, true, "community", "local-preference")
	f.answers = map[string]rpc.FilterUpdateOutput{"stale": stale, "other": other}
	f.peer.settings.ExportPolicy = answerPolicyAt(t, staleTestExport, routepolicy.FinalAccept, "stale", "other")
	// A receipt is named, so a rejection here has the authority to withdraw.
	chain := staleChainFor(f.peer, 1)
	chain.judgeAll = true
	return f, chain
}

// addStaleOnCall makes the opted member "stale" answer accept, and on its nth call
// over a route hand the community back as bgp-gr's llgr-egress does, recording the
// raw body it read on each call. Zero never adds.
func addStaleOnCall(f *staleChangeFixture, nth int, raws *[][]byte) {
	calls := 0
	f.respond = func(input rpc.FilterUpdateInput) (rpc.FilterUpdateOutput, bool) {
		if input.Filter != "stale" {
			return rpc.FilterUpdateOutput{}, false
		}
		calls++
		*raws = append(*raws, bytes.Clone(input.Raw))
		if calls != nth {
			return rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, true
		}
		return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{{
			Field: "community", Action: rpc.FilterSetActionAdd, Value: json.RawMessage(strconv.Quote("65535:6")),
		}}}, true
	}
}

// carriesLLGRStale is holdsLLGRStale where the caller has no parse failure to report:
// a body that does not parse holds no value the assertion can see.
func carriesLLGRStale(body []byte) bool {
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return false
	}
	_, _, value, found := attribute.AttrFind(sections.Attrs(body), attribute.AttrCommunity)
	return found && bytes.Contains(value, []byte{0xff, 0xff, 0x00, 0x06})
}

// holdsLLGRStale reports whether body's COMMUNITIES holds 65535:6.
func holdsLLGRStale(t *testing.T, body []byte) bool {
	t.Helper()
	if _, err := wire.ParseUpdateSections(body); err != nil {
		t.Fatalf("parse %x: %v", body, err)
	}
	return carriesLLGRStale(body)
}

// attributeValue answers the value of attribute code in body.
func attributeValue(t *testing.T, body []byte, code attribute.AttributeCode) []byte {
	t.Helper()
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatalf("parse %x: %v", body, err)
	}
	_, _, value, found := attribute.AttrFind(sections.Attrs(body), code)
	if !found {
		t.Fatalf("attribute %d is not in %x", code, body)
	}
	return value
}

// stripLLGRStale is a member's answer that removes LLGR_STALE.
var stripLLGRStale = rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{{
	Field: "community", Action: rpc.FilterSetActionRemove, Value: json.RawMessage(strconv.Quote("65535:6")),
}}}

// TestWholeChainKeepsWhatTheRFCOwes proves a member of the operator's chain cannot
// undo what RFC 9494 asks of a stale route when the rail runs the whole chain.
//
// VALIDATES: RFC 9494 Section 4.3 "The LLGR_STALE community MUST NOT be removed when
// the route is further advertised." and Section 4.6 "The stale routes MUST have their
// LOCAL_PREF set to zero."
// PREVENTS: a member asked for the first time after a reload stripping the mark, or
// raising the LOCAL_PREF llgr-egress lowered.
func TestWholeChainKeepsWhatTheRFCOwes(t *testing.T) {
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65000, LocalAS: 65000, IsIBGP: true}

	t.Run("member-strips-llgr-stale", func(t *testing.T) {
		f, chain := staleKeepFixture(t, rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, stripLLGRStale)
		var raws [][]byte
		addStaleOnCall(f, 1, &raws)
		input := nlriHex(t, llgrStaleBody)
		outcome, modified := f.a.decideStaleReadvertise(dest, chain, input, 2)
		if outcome != staleModify || !holdsLLGRStale(t, modified) {
			t.Fatalf("outcome=%d body=%x, want a modified body still carrying LLGR_STALE", outcome, modified)
		}
		// The opted member can only tell what was taken off from the route as the
		// chain received it, not from the body the other member left.
		if len(raws) != 1 || !bytes.Equal(raws[0], input) {
			t.Fatalf("the opted member read raw %x, want the route as the chain received it", raws)
		}
	})
	t.Run("member-raises-local-preference", func(t *testing.T) {
		raise := rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{{
			Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage("200"),
		}}}
		depreference := rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{staleNoExportSet, staleLocalPrefZeroSet}}
		f, chain := staleKeepFixture(t, depreference, raise)
		outcome, modified := f.a.decideStaleReadvertise(dest, chain, nlriHex(t, llgrStaleBody), 2)
		if outcome != staleModify {
			t.Fatalf("outcome=%d, want staleModify", outcome)
		}
		if lp := attributeValue(t, modified, attribute.AttrLocalPref); !bytes.Equal(lp, []byte{0, 0, 0, 0}) {
			t.Fatalf("LOCAL_PREF=%x, want zero: a member undid the depreference", lp)
		}
		f.callMu.Lock()
		defer f.callMu.Unlock()
		if !slices.Equal(f.calls, []string{"other", "stale"}) {
			t.Fatalf("members asked=%v, want the opted member last", f.calls)
		}
	})
	t.Run("member-rejects-a-route-the-peer-holds", func(t *testing.T) {
		f, chain := staleKeepFixture(t, rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, rpc.FilterUpdateOutput{Action: rpc.FilterReject})
		outcome, modified := f.a.decideStaleReadvertise(dest, chain, nlriHex(t, llgrStaleBody), 2)
		if outcome != staleWithdraw || modified != nil {
			t.Fatalf("outcome=%d body=%x, want staleWithdraw: holding back leaves an unmarked route on the peer", outcome, modified)
		}
	})
}

// TestForwardRailKeepsLLGRStale proves the export chain every other rail runs keeps
// LLGR_STALE on a route that carried it, and leaves alone a route that did not. The
// keeping is the opted member's, here a stand-in for llgr-egress, whose own answer
// TestLLGREgressKeepsLLGRStale proves in bgp-gr: its second call is the engine's pass
// over the body the stripping member left.
//
// VALIDATES: RFC 9494 Section 4.3 "The LLGR_STALE community MUST NOT be removed when
// the route is further advertised."
func TestForwardRailKeepsLLGRStale(t *testing.T) {
	f, chain := staleKeepFixture(t, rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, stripLLGRStale)
	var raws [][]byte
	addStaleOnCall(f, 2, &raws)
	session := policyview.ExportSession(filterapi.PeerFilterInfo{}, filterapi.PeerFilterInfo{}, false)
	run := func(body []byte) egressStepResult {
		update := wireu.NewWireUpdate(body, chain.ctxID)
		return f.a.r.runEgressPolicyChainASN4(chain.policy, "10.0.0.2", 65001, 65000, false, session, update, true)
	}

	res := run(nlriHex(t, llgrStaleBody))
	if !res.accept || res.wireOverride == nil || !carriesLLGRStale(res.wireOverride.Payload()) {
		t.Fatalf("accept=%v: the chain's output lost LLGR_STALE", res.accept)
	}

	// The same route with an ordinary community and no LLGR_STALE: nothing to keep.
	res = run(nlriHex(t, "0000000e"+"40010100"+"400200"+"c00804fde90001"+"180a0000"))
	if !res.accept {
		t.Fatal("the chain refused an unmarked route")
	}
	if res.wireOverride != nil && carriesLLGRStale(res.wireOverride.Payload()) {
		t.Fatal("the chain marked a route that never carried LLGR_STALE")
	}
}

// runForwardChain runs chain over body as the export step of a forward rail does.
func runForwardChain(f *staleChangeFixture, chain staleChain, body []byte) egressStepResult {
	session := policyview.ExportSession(filterapi.PeerFilterInfo{}, filterapi.PeerFilterInfo{}, false)
	update := wireu.NewWireUpdate(body, chain.ctxID)
	return f.a.r.runEgressPolicyChainASN4(chain.policy, "10.0.0.2", 65001, 65000, false, session, update, true)
}

// TestExportChainGivesOptedMembersTheLastWord proves the one thing the engine does
// for what a filter owes: it runs the members that declare judges-final again over a
// body the chain rewrote, and the keeping is theirs. Method: a judge that hands the
// community back on its second call, behind a member that strips it.
//
// VALIDATES: runFinalJudges, over the raw of the route as the chain received it.
// PREVENTS: LLGR_STALE lost on a forward rail (RFC 9494 Section 4.3), the engine
// holding a procedure of its own that runs with the plugin that owes it absent, and
// a filter that only opted into the stale rail running twice on every forward.
func TestExportChainGivesOptedMembersTheLastWord(t *testing.T) {
	strip := rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
	input := func(t *testing.T) []byte { t.Helper(); return nlriHex(t, llgrStaleBody) }
	// failOnSecond answers strip on the opted member's first call and answer after.
	failOnSecond := func(f *staleChangeFixture, answer rpc.FilterUpdateOutput) {
		calls := 0
		f.respond = func(input rpc.FilterUpdateInput) (rpc.FilterUpdateOutput, bool) {
			if input.Filter != "stale" {
				return rpc.FilterUpdateOutput{}, false
			}
			calls++
			if calls == 1 {
				return strip, true
			}
			return answer, true
		}
	}

	t.Run("second-pass-reads-the-input-and-keeps", func(t *testing.T) {
		f, chain := staleKeepFixture(t, strip, stripLLGRStale)
		var raws [][]byte
		addStaleOnCall(f, 2, &raws)
		body := input(t)
		res := runForwardChain(f, chain, body)
		if !res.accept || res.wireOverride == nil || !carriesLLGRStale(res.wireOverride.Payload()) {
			t.Fatalf("accept=%v failed=%v: the chain's output lost LLGR_STALE", res.accept, res.failed)
		}
		if len(raws) != 2 || !bytes.Equal(raws[1], body) {
			t.Fatalf("second pass read raw %x, want the route as the chain received it", raws)
		}
	})
	t.Run("no-opted-member-the-engine-keeps-nothing", func(t *testing.T) {
		f, chain := staleKeepFixture(t, strip, stripLLGRStale)
		chain.policy = answerPolicyAt(t, staleTestExport, routepolicy.FinalAccept, "other")
		res := runForwardChain(f, chain, input(t))
		if !res.accept || res.wireOverride == nil || carriesLLGRStale(res.wireOverride.Payload()) {
			t.Fatalf("accept=%v: with no member owing it, the engine must leave the operator's removal alone", res.accept)
		}
	})
	t.Run("an-unchanged-body-runs-no-second-pass", func(t *testing.T) {
		f, chain := staleKeepFixture(t, strip, rpc.FilterUpdateOutput{Action: rpc.FilterAccept})
		res := runForwardChain(f, chain, input(t))
		f.callMu.Lock()
		defer f.callMu.Unlock()
		if !res.accept || res.wireOverride != nil || !slices.Equal(f.calls, []string{"stale", "other"}) {
			t.Fatalf("accept=%v calls=%v, want one pass over an unchanged body", res.accept, f.calls)
		}
	})
	t.Run("second-pass-failure-withholds", func(t *testing.T) {
		f, chain := staleKeepFixture(t, strip, stripLLGRStale)
		// A field the filter does not declare: the answer fails under its on-error.
		failOnSecond(f, rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{{
			Field: "med", Action: rpc.FilterSetActionSet, Value: json.RawMessage("1"),
		}}})
		res := runForwardChain(f, chain, input(t))
		if res.accept || !res.failed {
			t.Fatalf("accept=%v failed=%v: a member that could not answer leaves a body it never judged", res.accept, res.failed)
		}
	})
	t.Run("second-pass-reject-suppresses", func(t *testing.T) {
		f, chain := staleKeepFixture(t, strip, stripLLGRStale)
		failOnSecond(f, rpc.FilterUpdateOutput{Action: rpc.FilterReject})
		if res := runForwardChain(f, chain, input(t)); res.accept {
			t.Fatal("the opted member's verdict over the rewritten body was ignored")
		}
	})
	t.Run("a-rail-only-filter-runs-once", func(t *testing.T) {
		f, chain := staleKeepFixture(t, strip, stripLLGRStale)
		f.answers["rail-only"] = rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
		chain.policy = answerPolicyAt(t, staleTestExport, routepolicy.FinalAccept, "rail-only", "other")
		res := runForwardChain(f, chain, input(t))
		f.callMu.Lock()
		defer f.callMu.Unlock()
		if !res.accept || !slices.Equal(f.calls, []string{"rail-only", "other"}) {
			t.Fatalf("accept=%v calls=%v: a filter that declared only readvertise ran again on a forward", res.accept, f.calls)
		}
	})
	t.Run("raw-answer-over-a-rewritten-body-refused", func(t *testing.T) {
		f, chain := staleKeepFixture(t, strip, stripLLGRStale)
		failOnSecond(f, rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: nlriHex(t, llgrStaleBody)})
		res := runForwardChain(f, chain, input(t))
		if res.accept || !res.failed {
			t.Fatalf("accept=%v failed=%v: a raw answer of the received body discards what the chain removed", res.accept, res.failed)
		}
	})
	t.Run("stale-rail-raw-answer-over-a-rewritten-body-refused", func(t *testing.T) {
		f, chain := staleKeepFixture(t, strip, stripLLGRStale)
		f.respond = func(input rpc.FilterUpdateInput) (rpc.FilterUpdateOutput, bool) {
			if input.Filter != "stale" {
				return rpc.FilterUpdateOutput{}, false
			}
			return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: input.Raw}, true
		}
		dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65000, LocalAS: 65000, IsIBGP: true}
		outcome, modified := f.a.decideStaleReadvertise(dest, chain, input(t), 2)
		if outcome != staleFilterFailed || modified != nil {
			t.Fatalf("outcome=%d body=%x, want staleFilterFailed: the raw answer discards the rewrite", outcome, modified)
		}
	})
}

// TestStaleRailFinalPass proves the pass the stale rail owes when more than one member
// runs there. A member outside the judges cannot undo a judge's answer; a judge behind
// another can, because the pass runs them in chain order: that is the limit the
// protocol page states. A lone judge runs once.
//
// VALIDATES: runStaleChain, RailEndsOnFinalJudge.
// PREVENTS: a member behind the judge on the rail undoing RFC 9494 Section 4.3, and a
// lone judge asked twice over a body it wrote itself.
func TestStaleRailFinalPass(t *testing.T) {
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 65000, LocalAS: 65000, IsIBGP: true}
	accept := rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
	rail := func(t *testing.T, members ...string) (*staleChangeFixture, staleChain) {
		t.Helper()
		f, chain := staleKeepFixture(t, accept, accept)
		chain.policy = answerPolicyAt(t, staleTestExport, routepolicy.FinalAccept, members...)
		chain.judgeAll = false
		return f, chain
	}
	// stripOnce makes filter strip LLGR_STALE on its first call alone, and "stale"
	// hand the community back on its second, recording the raw it read each time.
	stripOnce := func(f *staleChangeFixture, filter string, raws *[][]byte) {
		calls, stale := 0, 0
		f.respond = func(input rpc.FilterUpdateInput) (rpc.FilterUpdateOutput, bool) {
			if input.Filter == filter {
				calls++
				if calls == 1 {
					return stripLLGRStale, true
				}
				return accept, true
			}
			if input.Filter != "stale" {
				return rpc.FilterUpdateOutput{}, false
			}
			stale++
			*raws = append(*raws, bytes.Clone(input.Raw))
			if stale == 2 {
				return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{{
					Field: "community", Action: rpc.FilterSetActionAdd, Value: json.RawMessage(strconv.Quote("65535:6")),
				}}}, true
			}
			return accept, true
		}
	}

	t.Run("rail-only-member-behind-the-judge", func(t *testing.T) {
		f, chain := rail(t, "stale", "rail-only")
		var raws [][]byte
		stripOnce(f, "rail-only", &raws)
		body := nlriHex(t, llgrStaleBody)
		outcome, modified := f.a.decideStaleReadvertise(dest, chain, body, 2)
		if outcome != staleModify || !holdsLLGRStale(t, modified) {
			t.Fatalf("outcome=%d body=%x, want LLGR_STALE back from the pass", outcome, modified)
		}
		f.callMu.Lock()
		defer f.callMu.Unlock()
		if !slices.Equal(f.calls, []string{"stale", "rail-only", "stale"}) {
			t.Fatalf("members asked=%v: the pass runs the judge alone", f.calls)
		}
		if len(raws) != 2 || !bytes.Equal(raws[1], body) {
			t.Fatalf("the pass read raw %x, want the route as the chain received it", raws)
		}
	})
	t.Run("two-judges-the-second-strips-once", func(t *testing.T) {
		f, chain := rail(t, "stale", "stale-too")
		var raws [][]byte
		stripOnce(f, "stale-too", &raws)
		outcome, modified := f.a.decideStaleReadvertise(dest, chain, nlriHex(t, llgrStaleBody), 2)
		if outcome != staleModify || !holdsLLGRStale(t, modified) {
			t.Fatalf("outcome=%d body=%x, want LLGR_STALE back from the pass", outcome, modified)
		}
		f.callMu.Lock()
		defer f.callMu.Unlock()
		if !slices.Equal(f.calls, []string{"stale", "stale-too", "stale", "stale-too"}) {
			t.Fatalf("members asked=%v, want both judges in both passes, in chain order", f.calls)
		}
	})
	t.Run("a-judge-behind-a-judge-has-the-last-answer", func(t *testing.T) {
		f, chain := rail(t, "stale", "stale-too")
		f.answers["stale-too"] = stripLLGRStale
		var raws [][]byte
		addStaleOnCall(f, 2, &raws)
		outcome, modified := f.a.decideStaleReadvertise(dest, chain, nlriHex(t, llgrStaleBody), 2)
		if outcome != staleModify || holdsLLGRStale(t, modified) {
			t.Fatalf("outcome=%d body=%x: the limit changed, so its pages must change with it", outcome, modified)
		}
	})
	t.Run("a-lone-judge-runs-once", func(t *testing.T) {
		f, chain := rail(t, "stale")
		f.answers["stale"] = rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{staleNoExportSet, staleLocalPrefZeroSet}}
		outcome, _ := f.a.decideStaleReadvertise(dest, chain, nlriHex(t, llgrStaleBody), 2)
		f.callMu.Lock()
		defer f.callMu.Unlock()
		if outcome != staleModify || !slices.Equal(f.calls, []string{"stale"}) {
			t.Fatalf("outcome=%d members asked=%v, want one call", outcome, f.calls)
		}
	})
}

// TestDryRunShowsTheFinalPass proves show policy test reports the pass the live export
// step runs: a judge has a second trace entry drawn over the rewritten body, the result
// is the pass's, and the trace counts as display only.
//
// VALIDATES: traceLibraryChain over runFinalJudges.
// PREVENTS: a dry run that shows LLGR_STALE gone from a route the live path keeps it on.
func TestDryRunShowsTheFinalPass(t *testing.T) {
	f, chain := staleKeepFixture(t, rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, stripLLGRStale)
	var raws [][]byte
	addStaleOnCall(f, 2, &raws)
	body := nlriHex(t, llgrStaleBody)
	update := wireu.NewWireUpdate(body, chain.ctxID)
	before, err := displayUpdate(update)
	if err != nil {
		t.Fatal(err)
	}
	dry := libraryDryRun{wireUpdate: update, payload: body, ctxID: chain.ctxID, direction: directionExport, peer: "10.0.0.2",
		session: policyview.ExportSession(filterapi.PeerFilterInfo{}, filterapi.PeerFilterInfo{}, false),
		intent:  libraryIntentContext{asn4: true, localAS: 65000, peerAS: 65001, asPathEditable: true}}

	library := chain.policy.Library()
	result, after, trace, wireChanges := f.a.r.traceLibraryChain(library, &dry, &before)
	if result.Action != PolicyAccept || after == nil {
		t.Fatalf("action=%v after=%v, want an accept with a route", result.Action, after)
	}
	if len(trace) != 3 || trace[0].Filter != trace[2].Filter || trace[0].Filter == trace[1].Filter {
		t.Fatalf("trace=%+v, want the judge, the stripping member, then the judge again", trace)
	}
	if trace[1].TextAfter == trace[2].TextAfter {
		t.Fatalf("the judge's entry shows %q, the text the stripping member left: it was not drawn over the pass", trace[2].TextAfter)
	}
	if after.text() != trace[2].TextAfter {
		t.Fatalf("the route shown is %q, the last entry %q", after.text(), trace[2].TextAfter)
	}
	if len(wireChanges) != 2 {
		t.Fatalf("wire changes=%v, want one from each pass", wireChanges)
	}
	if len(raws) != 2 || !bytes.Equal(raws[1], body) {
		t.Fatalf("the pass read raw %x, want the route as the chain received it", raws)
	}
	if refused := library.Refused(); refused != 0 {
		t.Fatalf("a dry run refused %d routes", refused)
	}
}

// gateRail writes one route through a writer that runs the gate.
type gateRail struct {
	name  string
	write func(t *testing.T, f *staleChangeFixture)
}

// gateRails are the two writers that run the session's export gate.
func gateRails() []gateRail {
	return []gateRail{
		{name: "writeUpdateGated", write: func(t *testing.T, f *staleChangeFixture) {
			t.Helper()
			facts := announceFactsFor(f.peer, f.batch.Family, netip.MustParseAddr("10.0.0.1"), false, f.peer.negotiated.Load())
			update, err := f.a.buildBatchAnnounceUpdate(make([]byte, 4096), make([]byte, 4096), f.batch, facts)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.peer.session.SendUpdate(update); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "SendAnnounce", write: func(t *testing.T, f *staleChangeFixture) {
			t.Helper()
			route := bgptypes.RouteSpec{Prefix: netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1"))}
			if err := f.peer.session.SendAnnounce(route, netip.Addr{}, 65000, false, true, false); err != nil {
				t.Fatal(err)
			}
		}},
	}
}

// TestGateStampsTheFactsItRan proves a write through the session's own gate reads as
// current exactly when the facts the gate ran are current, on both writers.
//
// VALIDATES: Peer.egressGate. One write of a current decision moves no watermark, and
// one under facts a refresh has not caught up with does.
// PREVENTS: a single announce raising the watermark past every earlier receipt, so the
// whole table re-judges; and an old chain's write reading as current.
func TestGateStampsTheFactsItRan(t *testing.T) {
	changeTo := func(t *testing.T, f *staleChangeFixture, members ...string) {
		t.Helper()
		next := &PeerSettings{ExportPolicy: answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, members...)}
		mustApplyHotSwappableSettings(t, f.peer, next, exportPolicySwap)
	}
	for _, rail := range gateRails() {
		t.Run(rail.name+"/current-facts-stay-current", func(t *testing.T) {
			f := newStaleChangeFixture(t, true)
			f.peer.session.egressRouteFilter = f.peer.egressGate(f.peer.session)
			changeTo(t, f, "stale", "passes")
			f.peer.refreshForwardFacts()
			from := f.peer.exportPolicyFrom.Load()
			before := len(f.conn.written())
			rail.write(t, f)
			if len(f.conn.written()) == before {
				t.Fatal("the gate withheld the route: this write proves nothing")
			}
			if got := f.peer.exportPolicyFrom.Load(); got != from {
				t.Fatalf("watermark moved from %d to %d for a decision under the current chain", from, got)
			}
			if staleChainFor(f.peer, f.peer.session.sentReceipt.MessageID()).judgeAll {
				t.Fatal("a current gate write was re-judged")
			}
			if f.peer.session.gateGeneration != 0 {
				t.Fatalf("the stamp %d outlived its write, and a later unstamped write would read it", f.peer.session.gateGeneration)
			}
		})
		t.Run(rail.name+"/facts-behind-the-publish-read-as-old", func(t *testing.T) {
			f := newStaleChangeFixture(t, true)
			stamp := recordGateStamp(f)
			changeTo(t, f, "stale", "passes")
			older := f.peer.exportGeneration.Load()
			var lagging peerForwardFacts
			f.peer.captureOriginatedFacts(&lagging)
			if older == 0 || lagging.exportGeneration != older {
				t.Fatalf("the lagging facts hold generation %d, want the %d of the chain they read", lagging.exportGeneration, older)
			}
			changeTo(t, f, "stale", "passes", "ordinary")
			if f.peer.exportGeneration.Load() == older {
				t.Fatal("the second change moved no generation: the facts do not lag")
			}
			f.peer.fwdFacts.Store(&lagging)
			before := len(f.conn.written())
			rail.write(t, f)
			if len(f.conn.written()) == before {
				t.Fatal("the gate withheld the route: this write proves nothing")
			}
			if *stamp != older {
				t.Fatalf("the gate stamped generation %d, want the %d its facts read", *stamp, older)
			}
			if !staleChainFor(f.peer, f.peer.session.sentReceipt.MessageID()).judgeAll {
				t.Fatal("a gate write under facts the publish had not refreshed read as current")
			}
		})
		t.Run(rail.name+"/pending-facts-stamp-the-current-generation", func(t *testing.T) {
			f := newStaleChangeFixture(t, true)
			stamp := recordGateStamp(f)
			changeTo(t, f, "stale", "passes")
			f.peer.fwdFacts.Store(nil)
			current, from := f.peer.exportGeneration.Load(), f.peer.exportPolicyFrom.Load()
			before := len(f.conn.written())
			rail.write(t, f)
			if len(f.conn.written()) == before {
				t.Fatal("the gate withheld the route: this write proves nothing")
			}
			if current == 0 || *stamp != current {
				t.Fatalf("the gate stamped generation %d, want the current %d: the pending path reads it", *stamp, current)
			}
			if got := f.peer.exportPolicyFrom.Load(); got != from {
				t.Fatalf("watermark moved from %d to %d for a decision under the current chain", from, got)
			}
		})
	}
}

// recordGateStamp installs the session's export gate and answers where it records the
// generation the gate stamped: the writer clears the session's own at the write's end.
func recordGateStamp(f *staleChangeFixture) *uint64 {
	stamp := new(uint64)
	gate := f.peer.egressGate(f.peer.session)
	f.peer.session.egressRouteFilter = func(body []byte) (bool, []byte, []byte) {
		suppress, override, follow := gate(body)
		*stamp = f.peer.session.gateGeneration
		return suppress, override, follow
	}
	return stamp
}

// TestForwardRailsStampTheChainGeneration proves both forward rails carry to the
// writer the generation of the facts their decision read, the facts' and not the
// peer's newer one. Method: a pool that records each dispatched item's stamp.
//
// PREVENTS: every forward at generation above zero reading as old (a stamp lost), or
// an item decided under lagging facts reading as current (a stamp from the peer).
func TestForwardRailsStampTheChainGeneration(t *testing.T) {
	attrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00, 0x40, 0x03, 0x04, 192, 0, 2, 254}
	for _, tc := range []struct {
		name string
		rs   bool
	}{{"forwardUpdateCore", false}, {"reactorForwardRS", true}} {
		t.Run(tc.name, func(t *testing.T) {
			dest := a2Dest(t, "192.0.2.53", 65053, netip.Addr{}, false)
			dest.exportGeneration.Store(7)
			dest.refreshForwardFacts()
			dest.exportGeneration.Store(9)
			stamps := make(chan uint64, 8)
			configure := func(r *Reactor, _ *forwardSourceInfo) {
				pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
					for i := range items {
						stamps <- items[i].exportGeneration
						if items[i].done != nil {
							items[i].done()
						}
					}
				}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
				t.Cleanup(pool.Stop)
				pool.registerOutgoingPool(fwdKey{peerAddr: dest.Settings().PeerKey()}, 4096)
				r.fwdPool = pool
			}
			a2ForwardWith(t, tc.rs, configure, buildUpdatePayload(attrs, a2InlinePrefix), dest)
			select {
			case stamp := <-stamps:
				if stamp != 7 {
					t.Fatalf("the forwarded item carries generation %d, want the 7 its facts read", stamp)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the rail dispatched nothing")
			}
		})
	}
}

// TestRecoveryPackKeepsExportGeneration proves a recovery pack never joins two
// decisions under different generations, whose section carries one stamp, and that
// the stamp is not what stops two decisions under the same one.
func TestRecoveryPackKeepsExportGeneration(t *testing.T) {
	parts := bucketBodyParts{attrs: []byte{0x40, 0x01, 0x01, 0x00}, attrHash: 1}
	section, same, other := fwdItem{exportGeneration: 7}, fwdItem{exportGeneration: 7}, fwdItem{exportGeneration: 8}
	if !recoveryPackJoins(&section, &same, &parts, &parts) {
		t.Fatal("two decisions under one generation did not pack")
	}
	if recoveryPackJoins(&section, &other, &parts, &parts) {
		t.Fatal("decisions under two generations packed into one stamp")
	}
}

// TestExportWatermarkOnlyRises proves a writer's late raise, past a receipt it took
// before a reload, cannot lower the watermark that reload set.
func TestExportWatermarkOnlyRises(t *testing.T) {
	peer := &Peer{}
	peer.raiseExportPolicyFrom(10)
	peer.raiseExportPolicyFrom(4)
	if got := peer.exportPolicyFrom.Load(); got != 10 {
		t.Fatalf("watermark=%d, want 10", got)
	}
}
