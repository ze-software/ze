package filter_remove_private_as

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// removePrivateRoute builds the route the engine sends the filter: its
// declared as-path field and, when given, the raw UPDATE body.
func removePrivateRoute(filter, fields string, raw []byte) sdk.FilterRoute {
	return rpc.InputFilterRoute(&rpc.FilterUpdateInput{
		Filter: filter, PeerAS: 65001, Fields: json.RawMessage(fields), Raw: raw,
	})
}

// wantRemoveIntent fails unless out is a modify answering exactly the as-path
// remove intent with value.
func wantRemoveIntent(t *testing.T, out *sdk.FilterUpdateOutput, value string) {
	t.Helper()
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want modify", out.Action)
	}
	if out.Update != "" {
		t.Fatalf("text update = %q, want none", out.Update)
	}
	if len(out.Sets) != 1 {
		t.Fatalf("sets = %+v, want one as-path remove", out.Sets)
	}
	set := out.Sets[0]
	if set.Field != "as-path" || set.Action != rpc.FilterSetActionRemove || string(set.Value) != value {
		t.Fatalf("set = %s %s %s, want as-path remove %s", set.Field, set.Action, set.Value, value)
	}
}

// VALIDATES: a configured filter answers the remove-private intent when
// AS_PATH carries a Private Use ASN, in the definition's mode, and accepts a
// path that carries none.
// PREVENTS: a route passing unchanged when remove-private-as is configured.
func TestRemovePrivateASFilterUpdate(t *testing.T) {
	defs := map[string]*removePrivateASDef{
		"STRIP": {name: "STRIP", mode: removeModeStrip},
		"PEER":  {name: "PEER", mode: removeModePeerAS},
	}
	defsByName.Store(&defs)

	route := removePrivateRoute("STRIP", `{"as-path":[64496,64512,64497]}`, nil)
	out, err := evaluateRemovePrivateAS(&route)
	if err != nil {
		t.Fatal(err)
	}
	wantRemoveIntent(t, out, `"private"`)

	route = removePrivateRoute("PEER", `{"as-path":[64496,64512,64497]}`, nil)
	out, err = evaluateRemovePrivateAS(&route)
	if err != nil {
		t.Fatal(err)
	}
	wantRemoveIntent(t, out, `"private-peer-as"`)

	route = removePrivateRoute("STRIP", `{"as-path":[64496,64497]}`, nil)
	out, err = evaluateRemovePrivateAS(&route)
	if err != nil || out.Action != sdk.FilterAccept || len(out.Sets) != 0 {
		t.Fatalf("without private ASN = %+v, %v; want accept with no sets", out, err)
	}
}

// VALIDATES: raw AS4_PATH private ASNs draw the intent even when AS_PATH does not.
// PREVENTS: RFC 6996 AS4_PATH requirement being skipped.
func TestRemovePrivateASFilterUpdateAS4Path(t *testing.T) {
	defs := map[string]*removePrivateASDef{"STRIP": {name: "STRIP", mode: removeModeStrip}}
	defsByName.Store(&defs)

	route := removePrivateRoute("STRIP", `{"as-path":[23456]}`, buildRawPayloadWithAS4Path(4200000000))
	out, err := evaluateRemovePrivateAS(&route)
	if err != nil {
		t.Fatal(err)
	}
	wantRemoveIntent(t, out, `"private"`)
}

// VALIDATES: a route for a filter name no definition holds, or one that
// arrives before the configuration, fails under the filter's on-error rather
// than taking a verdict.
// PREVENTS: an unconfigured filter answering accept.
func TestRemovePrivateASUnknownFilterFails(t *testing.T) {
	defsByName.Store(nil)
	route := removePrivateRoute("STRIP", `{"as-path":[64512]}`, nil)
	if out, err := evaluateRemovePrivateAS(&route); err == nil {
		t.Fatalf("before configure = %+v, want an error", out)
	}
	defs := map[string]*removePrivateASDef{"STRIP": {name: "STRIP", mode: removeModeStrip}}
	defsByName.Store(&defs)
	route = removePrivateRoute("OTHER", `{"as-path":[64512]}`, nil)
	if out, err := evaluateRemovePrivateAS(&route); err == nil {
		t.Fatalf("unknown filter = %+v, want an error", out)
	}
}

// VALIDATES: the evaluator honors the fast-path contract on both transports
// (FR-9, AC-36): no blocking, the same answer twice, no view kept past the
// call, equal answers on the slot and over the batch, concurrent use.
// PREVENTS: an evaluator that is only correct when the bridge loop serialized it.
func TestRemovePrivateASConformance(t *testing.T) {
	defs := map[string]*removePrivateASDef{"STRIP": {name: "STRIP", mode: removeModeStrip}}
	defsByName.Store(&defs)
	routes := []sdk.ConformanceRoute{
		{Input: rpc.FilterUpdateInput{Filter: "STRIP", Fields: json.RawMessage(`{"as-path":[64496,64512]}`)}},
		{Input: rpc.FilterUpdateInput{Filter: "STRIP", Fields: json.RawMessage(`{"as-path":[64496]}`)}},
		{Input: rpc.FilterUpdateInput{Filter: "STRIP", Fields: json.RawMessage(`{"as-path":[23456]}`), Raw: buildRawPayloadWithAS4Path(4200000000)}},
	}
	if !sdk.CheckFilterConformance(t, "remove-private-as", evaluateRemovePrivateAS, routes) {
		t.Fatal("remove-private-as breaks the fast-path filter contract; the clause is reported above")
	}
}

func buildRawPayloadWithAS4Path(asn uint32) []byte {
	as4Value := make([]byte, 6)
	as4Value[0] = 2
	as4Value[1] = 1
	binary.BigEndian.PutUint32(as4Value[2:], asn)
	attr := append([]byte{0xC0, 17, byte(len(as4Value))}, as4Value...)
	payload := make([]byte, 4+len(attr))
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attr)))
	copy(payload[4:], attr)
	return payload
}
