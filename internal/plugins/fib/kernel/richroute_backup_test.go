// VALIDATES: AC-11 / A-3 -- RichRoute carries the fast-reroute backup and
// changeToRichRoute copies it from the sysrib change; a backup makes the change
// "rich" so it routes through the rich-route programming path.
// PREVENTS: the backup being dropped between sysrib and the FIB backend.
package fibkernel

import (
	"net/netip"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
)

func TestRichRouteBackupNextHop(t *testing.T) {
	c := &incomingChange{
		Prefix:  netip.MustParsePrefix("10.20.0.0/24"),
		NextHop: netip.MustParseAddr("10.0.0.2"),
		Backup: []sysribevents.ECMPPath{{
			NextHop: netip.MustParseAddr("10.0.0.9"),
			Labels:  []uint32{16010, 24003},
		}},
	}
	if !hasRichFields(c) {
		t.Fatal("a backup next-hop must make the change rich")
	}
	r := changeToRichRoute(c)
	if len(r.Backup) != 1 {
		t.Fatalf("backup not copied into RichRoute: %+v", r.Backup)
	}
	if r.Backup[0].NextHop != netip.MustParseAddr("10.0.0.9") {
		t.Fatalf("backup next-hop = %v, want 10.0.0.9", r.Backup[0].NextHop)
	}
	if len(r.Backup[0].Labels) != 2 || r.Backup[0].Labels[0] != 16010 || r.Backup[0].Labels[1] != 24003 {
		t.Fatalf("backup repair labels = %v, want [16010 24003]", r.Backup[0].Labels)
	}
}
