// VALIDATES: rpki is a valid BGP event type
// PREVENTS: rpki events rejected by subscription validation
package plugin

import "testing"

func TestValidBgpEventsIncludesRPKI(t *testing.T) {
	if !ValidBgpEvents[EventRPKI] {
		t.Fatal("rpki should be a valid BGP event type")
	}
	if EventRPKI != "rpki" {
		t.Fatalf("expected EventRPKI = %q, got %q", "rpki", EventRPKI)
	}
}
