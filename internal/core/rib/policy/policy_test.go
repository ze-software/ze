// VALIDATES: the compiled snapshot the Loc-RIB reads. The source chain runs
// before the central chain, which sees the tag the source chain set and wins
// where both set a field; an answer that is not an accept rejects (fail
// closed); a snapshot holding no chain publishes as nil, the no-policy state;
// and a chain keyed on no source is refused.
// PREVENTS: a chain that cannot answer installing the route, and the hot-path
// nil check being defeated by an empty snapshot.

package policy

import (
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"
)

type chainFunc func(r Route) Verdict

func (f chainFunc) Evaluate(r Route) Verdict { return f(r) } //nolint:gocritic // hugeParam: the Chain signature passes Route by value

func TestSnapshotSourceThenCentral(t *testing.T) {
	source := redistevents.RegisterProtocol("ospf")
	setTag := chainFunc(func(Route) Verdict {
		return Verdict{Action: ActionAccept, Sets: Sets{Tag: 7, HasTag: true, Distance: 50, HasDistance: true}}
	})
	var centralSaw Route
	central := chainFunc(func(r Route) Verdict {
		centralSaw = r
		return Verdict{Action: ActionAccept, Sets: Sets{Distance: 90, HasDistance: true}}
	})
	snap, err := NewSnapshot(map[redistevents.ProtocolID]Chain{source: setTag}, central)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	v := snap.Evaluate(Route{Source: source})
	if v.Action != ActionAccept {
		t.Fatalf("action = %d, want accept", v.Action)
	}
	if !centralSaw.HasTag || centralSaw.Tag != 7 {
		t.Fatal("the central chain did not see the tag the source chain set")
	}
	if v.Sets.Distance != 90 || v.Sets.Tag != 7 {
		t.Fatalf("sets = %+v, want central's distance 90 and the source's tag 7", v.Sets)
	}

	// Another source passes only the central chain.
	other := redistevents.RegisterProtocol("static")
	if v := snap.Evaluate(Route{Source: other}); v.Sets.HasTag {
		t.Fatal("the ospf chain ran for a static route")
	}
}

func TestSnapshotFailsClosed(t *testing.T) {
	source := redistevents.RegisterProtocol("ospf")
	silent := chainFunc(func(Route) Verdict { return Verdict{} })
	for name, sources := range map[string]map[redistevents.ProtocolID]Chain{
		"source chain answers nothing":  {source: silent},
		"central chain answers nothing": nil,
	} {
		var central Chain
		if sources == nil {
			central = silent
		}
		snap, err := NewSnapshot(sources, central)
		if err != nil {
			t.Fatalf("%s: NewSnapshot: %v", name, err)
		}
		if v := snap.Evaluate(Route{Source: source}); v.Action != ActionReject {
			t.Errorf("%s: action = %d, want reject", name, v.Action)
		}
	}
}

func TestSetEmptySnapshotIsNoPolicy(t *testing.T) {
	t.Cleanup(func() { Set(nil) })
	empty, err := NewSnapshot(map[redistevents.ProtocolID]Chain{redistevents.RegisterProtocol("ospf"): nil}, nil)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	Set(empty)
	if Current() != nil {
		t.Fatal("a snapshot holding no chain was published: the insert path's nil check is defeated")
	}
	full, err := NewSnapshot(nil, chainFunc(func(Route) Verdict { return Verdict{Action: ActionAccept} }))
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	Set(full)
	if Current() != full {
		t.Fatal("a snapshot with a central chain was not published")
	}
}

func TestNewSnapshotRefusesUnspecifiedSource(t *testing.T) {
	accept := chainFunc(func(Route) Verdict { return Verdict{Action: ActionAccept} })
	if _, err := NewSnapshot(map[redistevents.ProtocolID]Chain{redistevents.ProtocolUnspecified: accept}, nil); err == nil {
		t.Fatal("a chain keyed on the unspecified protocol was accepted")
	}
}
