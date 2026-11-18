// Design: docs/architecture/l2tp/subscriber-session-model.md -- optional birth JSON contract
package subscriber

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/core/subscriberid"
)

// This serial test changes only entropy consumed by the actual constructor.
// It publishes no session, admission, receipt or mutation authority.
func TestL2TPBirthRejectsActualZeroEntropyDraw(t *testing.T) {
	previous := rand.Reader
	t.Cleanup(func() { rand.Reader = previous })
	rand.Reader = bytes.NewReader(make([]byte, 16))
	birth, err := subscriberid.NewL2TPBirth()
	if err == nil {
		t.Fatal("actual zero entropy draw produced an accepted birth")
	}
	if birth.IsValid() {
		t.Fatal("failed constructor returned a valid birth")
	}
	if birth != (subscriberid.L2TPBirth{}) {
		t.Fatal("zero draw fabricated another birth")
	}
}

func TestSubscriberSnapshotJSONOmitsAbsentL2TPBirthForPPPoE(t *testing.T) {
	original := Session{ID: SessionID(AccessPPPoE, 65537, 7), AccessType: AccessPPPoE, PPPoESID: 7, AccessIfIndex: 65537}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("valid PPPoE snapshot cannot serialize absent L2TP birth: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if _, present := fields["l2tp-birth"]; present {
		t.Fatal("PPPoE snapshot serialized an invalid L2TP birth")
	}
	var decoded Session
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.L2TPBirth != (subscriberid.L2TPBirth{}) {
		t.Fatal("PPPoE roundtrip minted an L2TP birth")
	}
	if decoded.ID != original.ID {
		t.Fatal("PPPoE roundtrip changed canonical identity")
	}
	if decoded.AccessIfIndex != original.AccessIfIndex {
		t.Fatal("PPPoE roundtrip truncated its access index")
	}
	if decoded.PPPoESID != original.PPPoESID {
		t.Fatal("PPPoE roundtrip changed SID")
	}
}

func TestSubscriberSnapshotJSONPreservesActualL2TPBirth(t *testing.T) {
	birth, err := subscriberid.NewL2TPBirth()
	if err != nil {
		t.Fatal(err)
	}
	original := Session{ID: birth.String(), L2TPBirth: birth, AccessType: AccessL2TP, TunnelID: 19, SessionID: 7}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	wire, present := fields["l2tp-birth"]
	if !present {
		t.Fatal("L2TP snapshot omitted its actual birth")
	}
	var text string
	if err := json.Unmarshal(wire, &text); err != nil {
		t.Fatal(err)
	}
	if text != birth.String() {
		t.Fatal("wire boundary changed canonical birth text")
	}
	var decoded Session
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.L2TPBirth != birth {
		t.Fatal("roundtrip changed packed identity")
	}
	if decoded.ID != original.ID {
		t.Fatal("roundtrip changed cached DTO")
	}
	if decoded.TunnelID != original.TunnelID {
		t.Fatal("roundtrip changed diagnostic TID")
	}
	if decoded.SessionID != original.SessionID {
		t.Fatal("roundtrip changed diagnostic SID")
	}
}
