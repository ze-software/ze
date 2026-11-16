// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- subscriber identity isolates authorization profiles

package l2tp

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
)

// TestAuthMetadataSeparatesTransportIdentities stores different profiles for
// equal driver pairs in L2TP and PPPoE, then clears either namespace without
// changing the other subscriber's authorization.
func TestAuthMetadataSeparatesTransportIdentities(t *testing.T) {
	firstID := subscriber.SessionID(subscriber.AccessL2TP, 7, 42)
	secondID := subscriber.SessionID(subscriber.AccessPPPoE, 7, 42)
	first := &AuthMetadata{FramedPool: "l2tp-profile", FramedIP: netip.MustParseAddr("192.0.2.10")}
	second := &AuthMetadata{FramedPool: "pppoe-profile", FramedIP: netip.MustParseAddr("192.0.2.20")}
	StoreSessionMetadata(firstID, first)
	StoreSessionMetadata(secondID, second)
	t.Cleanup(func() {
		ClearSessionMetadata(firstID)
		ClearSessionMetadata(secondID)
	})
	if profile := LoadSessionMetadata(firstID); profile != first {
		t.Fatalf("L2TP profile changed: %+v", profile)
	}
	if profile := LoadSessionMetadata(secondID); profile != second {
		t.Fatalf("PPPoE profile changed: %+v", profile)
	}
	ClearSessionMetadata(secondID)
	if profile := LoadSessionMetadata(firstID); profile != first {
		t.Fatalf("PPPoE cleanup changed L2TP profile: %+v", profile)
	}
	if profile := LoadSessionMetadata(secondID); profile != nil {
		t.Fatalf("PPPoE profile remained after cleanup: %+v", profile)
	}
	StoreSessionMetadata(secondID, second)
	ClearSessionMetadata(firstID)
	if profile := LoadSessionMetadata(secondID); profile != second {
		t.Fatalf("L2TP cleanup changed PPPoE profile: %+v", profile)
	}
}

// TestAuthMetadataRefusesMissingIdentity prevents an unnamespaced store from
// creating a shared authorization entry or changing an existing subscriber.
func TestAuthMetadataRefusesMissingIdentity(t *testing.T) {
	id := subscriber.SessionID(subscriber.AccessL2TP, 7, 42)
	profile := &AuthMetadata{FramedPool: "retained"}
	StoreSessionMetadata(id, profile)
	t.Cleanup(func() { ClearSessionMetadata(id) })
	StoreSessionMetadata("", &AuthMetadata{FramedPool: "unnamespaced"})
	if _, present := sessionMeta.Load(""); present {
		t.Fatal("missing subscriber identity entered the authorization store")
	}
	if got := LoadSessionMetadata(id); got != profile {
		t.Fatalf("invalid store changed the valid subscriber profile: %+v", got)
	}
}
