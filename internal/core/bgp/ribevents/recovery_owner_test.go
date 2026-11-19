// Design: docs/architecture/plugin/rib-storage-design.md -- exact recovery owners.
package ribevents

import (
	"encoding/json"
	"net/netip"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

func TestRecoveryExactOwnersRoundTrip(t *testing.T) {
	request := RecoveryRequest{Destination: netip.MustParseAddr("192.0.2.1"), DestinationOwner: 41,
		Family: family.IPv4Unicast, SentAddPath: true, Limit: 3,
		Keys: []RecoveryKey{{NLRI: []byte{24, 192, 0, 2}, Excluded: []uint64{42, 43}}}}
	args := AppendRecoveryArgs(nil, &request)
	parsed, err := ParseRecoveryArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parsed, request) {
		t.Fatalf("text round trip lost original owner: got %+v, want %+v", parsed, request)
	}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RecoveryRequest
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, request) {
		t.Fatalf("JSON round trip lost original owner: got %+v, want %+v", decoded, request)
	}
	route := RecoveryRoute{Source: request.Destination, SourceOwner: 42, SourceReceipt: 44,
		PathID: 0, AddPath: true, NLRI: []byte{0, 0, 0, 0, 24, 192, 0, 2}}
	raw, err = json.Marshal(route)
	if err != nil {
		t.Fatal(err)
	}
	var survivor RecoveryRoute
	if err := json.Unmarshal(raw, &survivor); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(survivor, route) {
		t.Fatalf("survivor JSON lost original owner, receipt or path zero: got %+v, want %+v", survivor, route)
	}
}

func TestRecoveryOwnerGrammarRejectsMissingAuthority(t *testing.T) {
	for _, token := range []string{"0", "192.0.2.1", "-1", "18446744073709551616"} {
		if _, err := ParseRecoveryArgs([]string{"192.0.2.1", token, "ipv4-unicast", "false", "false", "18c00002="}); err == nil {
			t.Errorf("accepted destination owner %q", token)
		}
		if _, err := ParseRecoveryKey("18c00002=" + token); err == nil {
			t.Errorf("accepted excluded owner %q", token)
		}
	}
}
