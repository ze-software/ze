package filter_irr

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// routeFields writes the prefix and withdrawn route fields the engine sends a
// filter that reads them (docs/plugin-development/protocol.md, "The Route
// Fields of filter-update"): per family, the NLRI the UPDATE announces and
// withdraws. An element opening with '{' is written as that JSON object (the
// ADD-PATH and labeled-unicast shapes); any other is a prefix string.
func routeFields(announced, withdrawn map[string][]string) []byte {
	shape := func(byFamily map[string][]string) map[string][]json.RawMessage {
		if byFamily == nil {
			return nil
		}
		out := make(map[string][]json.RawMessage, len(byFamily))
		for name, elements := range byFamily {
			values := make([]json.RawMessage, 0, len(elements))
			for _, element := range elements {
				if element != "" && element[0] == '{' {
					values = append(values, json.RawMessage(element))
					continue
				}
				quoted, err := json.Marshal(element)
				if err != nil {
					panic("BUG: a string always marshals: " + err.Error())
				}
				values = append(values, quoted)
			}
			out[name] = values
		}
		return out
	}
	fields := struct {
		Prefix    map[string][]json.RawMessage `json:"prefix,omitempty"`
		Withdrawn map[string][]json.RawMessage `json:"withdrawn,omitempty"`
	}{shape(announced), shape(withdrawn)}
	encoded, err := json.Marshal(fields)
	if err != nil {
		panic("BUG: the route fields always marshal: " + err.Error())
	}
	return encoded
}

// unicastFamilies groups prefixes by the unicast family of their address. A
// text that is no prefix, which a test sends to see it refused, goes under
// ipv4/unicast.
func unicastFamilies(prefixes []string) map[string][]string {
	out := map[string][]string{}
	for _, text := range prefixes {
		name := "ipv4/unicast"
		if prefix, err := netip.ParsePrefix(text); err == nil && prefix.Addr().Is6() {
			name = "ipv6/unicast"
		}
		out[name] = append(out[name], text)
	}
	return out
}

// announceFields is the route fields of an UPDATE announcing prefixes as
// unicast NLRI and withdrawing nothing.
func announceFields(prefixes ...string) []byte {
	return routeFields(unicastFamilies(prefixes), nil)
}

// withdrawFields is the route fields of an UPDATE withdrawing prefixes as
// unicast NLRI and announcing nothing.
func withdrawFields(prefixes ...string) []byte {
	return routeFields(nil, unicastFamilies(prefixes))
}

// admitsAll reports whether list authorizes every prefix of one UPDATE that
// announces them as unicast NLRI: the verdict the evaluator accepts unmodified.
func admitsAll(t *testing.T, list *irrPrefixList, prefixes ...string) bool {
	t.Helper()
	judged, err := judgeNLRI(announceFields(prefixes...), list)
	if err != nil {
		t.Fatalf("judgeNLRI(%v): %v", prefixes, err)
	}
	return judged.rejected == 0
}

// removedValues answers the value of each prefix remove an answer carries, in
// order, and fails the test on a set of any other field or action.
func removedValues(t *testing.T, out *sdk.FilterUpdateOutput) []string {
	t.Helper()
	values := make([]string, 0, len(out.Sets))
	for _, set := range out.Sets {
		if set.Field != fieldPrefix || set.Action != rpc.FilterSetActionRemove {
			t.Fatalf("set %+v, want only prefix removes", set)
		}
		values = append(values, string(set.Value))
	}
	return values
}
