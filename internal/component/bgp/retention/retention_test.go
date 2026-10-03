// Design: docs/guide/graceful-restart.md -- one in-process retention owner.
package retention

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestRetentionOwnerReplacement proves a stopped engine cannot erase its
// replacement, and that the query preserves source and family discrimination.
func TestRetentionOwnerReplacement(t *testing.T) {
	old := Publish(func(string, family.Family) bool { return false })
	defer old.Close()
	owner := Publish(func(peer string, fam family.Family) bool {
		return peer == "192.0.2.1" && fam == family.IPv4Unicast
	})
	defer owner.Close()
	old.Close()
	if !Family("192.0.2.1", family.IPv4Unicast) {
		t.Fatal("old owner erased replacement")
	}
	if Family("192.0.2.1", family.IPv6Unicast) {
		t.Fatal("unretained family inherited peer-wide retention")
	}
	if Family("192.0.2.2", family.IPv4Unicast) {
		t.Fatal("another source inherited retention")
	}
	owner.Close()
	if Family("192.0.2.1", family.IPv4Unicast) {
		t.Fatal("stopped owner remains published")
	}
}
