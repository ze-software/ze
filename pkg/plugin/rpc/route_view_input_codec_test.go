// Design: docs/plugin-development/protocol.md -- routing-view request presence.

package rpc

import (
	"encoding/json"
	"math"
	"net/netip"
	"reflect"
	"strings"
	"testing"
)

func TestRouteViewInputRequiresPresence(t *testing.T) {
	tests := []struct {
		name     string
		control  string
		required []string
		current  any
	}{
		{"read", `{"session":"s","cursor":0,"limit":1,"wait-ms":0}`, []string{"session", "cursor", "limit", "wait-ms"}, RouteViewReadInput{Session: "current", Cursor: 8, Limit: 9, WaitMillis: 10}},
		{"candidates", `{"session":"s","cursor":0,"limit":1}`, []string{"session", "cursor", "limit"}, RouteViewCandidatesInput{Session: "current", Cursor: 8, Limit: 9}},
		{"distances", `{"session":"s","operation":1,"distances":[]}`, []string{"session", "operation"}, RouteViewDistancesInput{Session: "current", Operation: 8, Distances: []RouteViewDistance{{Protocol: "static", Distance: 120}}}},
		{"publish", `{"session":"s","cursor":0,"operation":1,"batch":{}}`, []string{"session", "cursor", "operation", "batch"}, RouteViewPublishInput{Session: "current", Cursor: 8, Operation: 9, Batch: json.RawMessage(`{"current":true}`)}},
		{"close", `{"session":"s"}`, []string{"session"}, RouteViewCloseInput{Session: "current"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			control := reflect.New(reflect.TypeOf(test.current)).Interface()
			if err := json.Unmarshal([]byte(test.control), control); err != nil {
				t.Fatalf("valid request refused: %v", err)
			}
			for _, field := range test.required {
				for _, form := range []string{"missing", "null"} {
					t.Run(field+"/"+form, func(t *testing.T) {
						var fields map[string]json.RawMessage
						if err := json.Unmarshal([]byte(test.control), &fields); err != nil {
							t.Fatal(err)
						}
						delete(fields, field)
						if form == "null" {
							fields[field] = json.RawMessage("null")
						}
						input, err := json.Marshal(fields)
						if err != nil {
							t.Fatal(err)
						}
						got := reflect.New(reflect.TypeOf(test.current))
						got.Elem().Set(reflect.ValueOf(test.current))
						if err := json.Unmarshal(input, got.Interface()); err == nil {
							t.Errorf("%s %s accepted", form, field)
						}
						if !reflect.DeepEqual(got.Elem().Interface(), test.current) {
							t.Error("rejected request changed receiver")
						}
					})
				}
			}
			for _, input := range []string{"null", strings.TrimSuffix(test.control, "}") + `,"undeclared":true}`, test.control + `{}`} {
				got := reflect.New(reflect.TypeOf(test.current))
				got.Elem().Set(reflect.ValueOf(test.current))
				if err := json.Unmarshal([]byte(input), got.Interface()); err == nil {
					t.Errorf("malformed request accepted: %s", input)
				}
				if !reflect.DeepEqual(got.Elem().Interface(), test.current) {
					t.Error("malformed request changed receiver")
				}
			}
		})
	}
}

func TestRouteViewInputDecodeBudget(t *testing.T) {
	input := `{"session":"` + strings.Repeat("s", MaxMessageSize) + `","cursor":0,"limit":1,"wait-ms":0}`
	want := RouteViewReadInput{Session: "current", Cursor: 8, Limit: 9, WaitMillis: 10}
	got := want
	if err := json.Unmarshal([]byte(input), &got); err == nil {
		t.Error("request exceeding the complete frame bound accepted")
	}
	if got != want {
		t.Error("oversized request changed receiver")
	}
}

func TestRouteViewInputBudgetIncludesRequestFraming(t *testing.T) {
	tests := []struct {
		method string
		input  string
		value  any
	}{
		{MethodRouteViewOpen, `{}`, RouteViewOpenInput{}},
		{MethodRouteViewRead, `{"session":"s","cursor":0,"limit":1,"wait-ms":0}`, RouteViewReadInput{}},
		{MethodRouteViewCandidates, `{"session":"s","cursor":0,"limit":1}`, RouteViewCandidatesInput{}},
		{MethodRouteViewDistances, `{"session":"s","operation":1}`, RouteViewDistancesInput{}},
		{MethodRouteViewPublish, `{"session":"s","cursor":0,"operation":1,"batch":{}}`, RouteViewPublishInput{}},
		{MethodRouteViewClose, `{"session":"s"}`, RouteViewCloseInput{}},
	}
	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			// AppendRequest is the wire producer, including its widest ID/header.
			header := len(AppendRequest(nil, math.MaxUint64, test.method, json.RawMessage("0"))) - 1
			for _, extra := range []int{0, 1} {
				input := strings.TrimSuffix(test.input, "}") + strings.Repeat(" ", MaxMessageSize-header-len(test.input)+extra) + "}"
				if width := len(AppendRequest(nil, math.MaxUint64, test.method, json.RawMessage(input))); width != MaxMessageSize+extra {
					t.Fatalf("request frame width %d, want %d", width, MaxMessageSize+extra)
				}
				got := reflect.New(reflect.TypeOf(test.value)).Interface()
				err := json.Unmarshal([]byte(input), got)
				if extra == 0 {
					if err != nil {
						t.Fatalf("last valid request refused: %v", err)
					}
					continue
				}
				if err == nil {
					t.Error("first request exceeding its complete frame accepted")
				}
				if !reflect.DeepEqual(reflect.ValueOf(got).Elem().Interface(), test.value) {
					t.Error("oversized request changed receiver")
				}
			}
		})
	}
}

func TestRouteViewInputRoundTrip(t *testing.T) {
	inputs := []any{
		&RouteViewOpenInput{},
		&RouteViewReadInput{Session: "s", Cursor: 7, Limit: 4, WaitMillis: 100},
		&RouteViewCandidatesInput{Session: "s", Snapshot: "pinned", Cursor: 9, Limit: 3, AFI: 1, SAFI: 1, Prefix: netip.MustParsePrefix("192.0.2.0/24")},
		&RouteViewDistancesInput{Session: "s", Operation: 4, Distances: []RouteViewDistance{{Protocol: "static", Distance: 0}, {Protocol: "ospf", Distance: 110}}},
		&RouteViewPublishInput{Session: "s", Cursor: 12, Operation: 5, Batch: json.RawMessage(`{"family":"ipv4-unicast","changes":[]}`)},
		&RouteViewCloseInput{Session: "s"},
	}
	for _, input := range inputs {
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		got := reflect.New(reflect.TypeOf(input).Elem()).Interface()
		if err := json.Unmarshal(encoded, got); err != nil {
			t.Fatalf("valid %T refused: %v", input, err)
		}
		if !reflect.DeepEqual(got, input) {
			t.Fatalf("%T changed: got %+v, want %+v", input, got, input)
		}
	}
}

func FuzzRouteViewInput(f *testing.F) {
	for _, seed := range []string{
		`null`, `{}`, `{"session":"s"}`,
		`{"session":"s","cursor":0,"limit":1,"wait-ms":0}`,
		`{"session":"s","cursor":0,"limit":1}`,
		`{"session":"s","operation":1,"distances":[{"protocol":"static","distance":0}]}`,
		`{"session":"s","cursor":0,"operation":1,"batch":{}}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		inputs := []any{&RouteViewOpenInput{}, &RouteViewReadInput{}, &RouteViewCandidatesInput{}, &RouteViewDistancesInput{}, &RouteViewPublishInput{}, &RouteViewCloseInput{}}
		for _, input := range inputs {
			before := reflect.ValueOf(input).Elem().Interface()
			if err := json.Unmarshal(data, input); err != nil {
				if !reflect.DeepEqual(reflect.ValueOf(input).Elem().Interface(), before) {
					t.Fatalf("refused %T changed receiver", input)
				}
				continue
			}
			encoded, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			got := reflect.New(reflect.TypeOf(input).Elem()).Interface()
			if err := json.Unmarshal(encoded, got); err != nil {
				t.Fatalf("accepted %T cannot be decoded after encoding: %v", input, err)
			}
		}
	})
}
