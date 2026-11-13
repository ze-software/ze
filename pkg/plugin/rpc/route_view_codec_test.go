// Design: docs/plugin-development/protocol.md -- canonical routing-view replies.

package rpc

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Ranking zeros are real values; omitted or null ranking fields are invalid.
func TestRouteViewPathRequiresRankingPresence(t *testing.T) {
	const control = `{"protocol":"ospf","instance":0,"distance":0,"metric":0}`
	var zero RouteViewPath
	if err := json.Unmarshal([]byte(control), &zero); err != nil {
		t.Fatalf("explicit ranking zero refused: %v", err)
	}
	if zero.Protocol != "ospf" || zero.Instance != 0 || zero.Distance != 0 || zero.Metric != 0 {
		t.Fatalf("ranking control changed: %+v", zero)
	}
	for _, field := range []string{"instance", "distance", "metric"} {
		for _, form := range []string{"missing", "null"} {
			t.Run(field+"/"+form, func(t *testing.T) {
				input := strings.Replace(control, `,"`+field+`":0`, "", 1)
				if form == "null" {
					input = strings.Replace(control, `"`+field+`":0`, `"`+field+`":null`, 1)
				}
				want := RouteViewPath{Protocol: "current", Instance: 9, Distance: 120, Metric: 42, Labels: []uint32{100}}
				got := want
				if err := json.Unmarshal([]byte(input), &got); err == nil {
					t.Errorf("%s %s accepted", form, field)
				}
				if !reflect.DeepEqual(got, want) {
					t.Error("rejected selected path changed receiver")
				}
			})
		}
	}
}

// A zero distance remains valid while missing or null row fields are refused.
func TestRouteViewDistanceRequiresPresence(t *testing.T) {
	var zero RouteViewDistance
	if err := json.Unmarshal([]byte(`{"protocol":"static","distance":0}`), &zero); err != nil {
		t.Fatalf("explicit zero distance refused: %v", err)
	}
	if zero.Protocol != "static" || zero.Distance != 0 {
		t.Fatalf("distance control changed: %+v", zero)
	}
	for _, input := range []string{
		`{"protocol":"static"}`, `{"protocol":"static","distance":null}`,
		`{"distance":0}`, `{"protocol":null,"distance":0}`,
		`{"protocol":"static","distance":0,"undeclared":true}`,
	} {
		want := RouteViewDistance{Protocol: "current", Distance: 120}
		got := want
		if err := json.Unmarshal([]byte(input), &got); err == nil {
			t.Errorf("incomplete distance row accepted: %s", input)
		}
		if got != want {
			t.Error("rejected distance row changed receiver")
		}
	}
}

func TestRouteViewNestedECMPPreservesLargeGroup(t *testing.T) {
	const count = 129
	data := []byte(`{"protocol":"static","instance":0,"distance":0,"metric":0,"ecmp":[` + strings.Repeat(`{"interface":"eth0"},`, count-1) + `{"interface":"eth0"}]}`)
	var path RouteViewPath
	if err := json.Unmarshal(data, &path); err != nil || len(path.ECMP) != count {
		t.Fatalf("valid ECMP count %d refused: %v", count, err)
	}
	for _, target := range path.ECMP {
		if target.Interface != "eth0" {
			t.Fatal("a selected target changed")
		}
	}
}

func TestRouteViewNestedECMPRejectsEmptyTarget(t *testing.T) {
	path := RouteViewPath{Protocol: "current"}
	if err := json.Unmarshal([]byte(`{"protocol":"static","instance":0,"distance":0,"metric":0,"ecmp":[{"interface":"eth0"},{}]}`), &path); err == nil || path.Protocol != "current" || path.ECMP != nil {
		t.Fatal("malformed ECMP changed the current path")
	}
}

func TestRouteViewReplyDecodeIsAtomic(t *testing.T) {
	want := RouteViewReadOutput{Session: "current", Epoch: "epoch", Cursor: 4, Phase: RouteViewPhaseDelta, Complete: true}
	for _, input := range []string{
		`null`,
		`{"session":"new","epoch":"epoch","cursor":5,"phase":"delta"}`,
		`{"session":"new","epoch":"epoch","cursor":5,"phase":"snapshot","complete":true}`,
		`{"session":"new","epoch":"epoch","cursor":5,"phase":"delta","complete":true,"entries":[{"prefix":"bad"}]}`,
	} {
		got := want
		if err := json.Unmarshal([]byte(input), &got); err == nil {
			t.Fatalf("invalid result accepted: %s", input)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal("rejected result changed the current page")
		}
	}
}

func FuzzRouteViewReply(f *testing.F) {
	for _, input := range []string{
		`null`, `{}`, `{"session":"owner"}`,
		`{"session":"owner","operation":1,"cursor":0}`,
		`{"session":"owner","epoch":"epoch","snapshot":"snapshot","cursor":0}`,
		`{"session":"owner","epoch":"epoch","snapshot":"snapshot","cursor":0,"phase":"snapshot","complete":true,"entries":[]}`,
	} {
		f.Add([]byte(input))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > RouteViewResultBudget() {
			return
		}
		for _, output := range []any{
			&RouteViewOpenOutput{}, &RouteViewReadOutput{}, &RouteViewCandidatesOutput{},
			&RouteViewDistancesOutput{}, &RouteViewPublishOutput{}, &RouteViewCloseOutput{},
		} {
			if err := json.Unmarshal(data, output); err != nil {
				continue
			}
			encoded, err := json.Marshal(output)
			if err != nil {
				t.Fatalf("accepted reply cannot be encoded: %v", err)
			}
			if err := json.Unmarshal(encoded, output); err != nil {
				t.Fatalf("accepted reply cannot be decoded again: %v", err)
			}
		}
	})
}
