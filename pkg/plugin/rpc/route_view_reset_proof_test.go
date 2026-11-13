// Design: docs/plugin-development/protocol.md -- exact publication reset proof.
// Related: route_view_codec.go -- atomic selected reply decoding.

package rpc

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRouteViewReadPublicationResetProofContract(t *testing.T) {
	control := `{"session":"lease","epoch":"daemon","cursor":2,"phase":"reset","complete":false,"rejected-publication":7}`
	var page RouteViewReadOutput
	if err := json.Unmarshal([]byte(control), &page); err != nil {
		t.Fatalf("valid reset proof refused: %v", err)
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	var retained struct {
		Operation uint64 `json:"rejected-publication"`
	}
	if err := json.Unmarshal(encoded, &retained); err != nil {
		t.Fatal(err)
	}
	if retained.Operation != 7 {
		t.Fatalf("exact reset proof was lost: %s", encoded)
	}
	for _, input := range []string{
		`{"session":"lease","epoch":"daemon","cursor":2,"phase":"reset","complete":false,"rejected-publication":0}`,
		`{"session":"lease","epoch":"daemon","cursor":2,"phase":"reset","complete":false,"rejected-publication":null}`,
		`{"session":"lease","epoch":"daemon","cursor":2,"phase":"delta","complete":true,"rejected-publication":7}`,
		`{"session":"lease","epoch":"daemon","snapshot":"snapshot","cursor":2,"phase":"snapshot","complete":true,"rejected-publication":7}`,
		`{"session":"lease","epoch":"daemon","cursor":2,"phase":"reset","complete":true,"rejected-publication":7}`,
		`{"session":"lease","epoch":"daemon","snapshot":"snapshot","cursor":2,"phase":"reset","complete":false,"rejected-publication":7}`,
	} {
		got := page
		if err := json.Unmarshal([]byte(input), &got); err == nil {
			t.Errorf("malformed reset proof accepted: %s", input)
		}
		if !reflect.DeepEqual(got, page) {
			t.Errorf("malformed reset proof changed receiver: %s", input)
		}
	}
}
