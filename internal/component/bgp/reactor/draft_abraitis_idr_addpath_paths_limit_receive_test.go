// Design: docs/guide/add-path.md -- the receive ceiling under Ze's own PATHS-LIMIT
// RFC: rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt -- Section 3
// Related: session_paths_limit_recv_test.go -- the receive ceiling fixtures these tests use

package reactor

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestReceiveCeilingAdmitsUpToLimit proves that ceiling 2 admits two path
// identifiers for one prefix, refuses the third, and leaves every other prefix
// in the same UPDATE untouched. Method: one UPDATE carrying three paths for one
// prefix and one path for another, read back from the rewritten body.
//
// VALIDATES: AC-8 at unit level.
//
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7 positive -- with a receive ceiling of 2, path identifiers 1 and 2 of one prefix and the path of another prefix in the same UPDATE are kept in the rewritten body, and a replacement of admitted identifier 2 at the ceiling is kept.
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7 negative -- a third path identifier for a prefix at a receive ceiling of 2 is removed from the rewritten body and counted once as refused.
func TestReceiveCeilingAdmitsUpToLimit(t *testing.T) {
	r := newRecvCeiling(t)
	announced, _ := r.receive(t, recvAnnounce(
		pathsLimitNLRI(1, pathsLimitRecvPrefix),
		pathsLimitNLRI(2, pathsLimitRecvPrefix),
		pathsLimitNLRI(3, pathsLimitRecvPrefix),
		pathsLimitNLRI(7, "203.0.113.0/24"),
	))
	if !equalIDs(announced, 1, 2, 7) {
		t.Fatalf("admitted %v, want [1 2 7]: the third path is refused and the other prefix kept", announced)
	}
	if got := r.refusedCount(t); got != 1 {
		t.Fatalf("refused counter %v, want 1", got)
	}
	// A replacement on an admitted identifier at the ceiling consumes no slot.
	announced, _ = r.receive(t, recvAnnounce(pathsLimitNLRI(2, pathsLimitRecvPrefix)))
	if !equalIDs(announced, 2) {
		t.Fatalf("replacement of admitted id 2 gave %v, want [2]", announced)
	}
}

// TestProcessMessageRefusesPathOverReceiveCeiling drives one UPDATE through
// processMessage: three paths for one route at ceiling 2 and one path for
// another route. The delivery callback must see the first two paths and the
// other route, never the third path, and the ceiling must hold what was
// delivered (AC-8).
//
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7 positive -- with a receive ceiling of 2, processMessage delivers path identifiers 1 and 2 of one route and the path of another route from the same UPDATE, and the ceiling holds both admitted identifiers.
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7 negative -- a third path identifier for a route at a receive ceiling of 2 is absent from the UPDATE processMessage delivers, and the ceiling tracks it as refused.
func TestProcessMessageRefusesPathOverReceiveCeiling(t *testing.T) {
	s, delivered := recvCeilingSession(t, 100, family.IPv4Unicast)
	recvProcess(t, s, prefixReceiveBody(family.IPv4Unicast, bytes.Join([][]byte{
		pathsLimitNLRI(1, pathsLimitRecvPrefix),
		pathsLimitNLRI(2, pathsLimitRecvPrefix),
		pathsLimitNLRI(3, pathsLimitRecvPrefix),
		pathsLimitNLRI(1, pathsLimitRecvOther),
	}, nil), nil))

	if len(*delivered) != 1 {
		t.Fatalf("delivered %d UPDATEs, want 1", len(*delivered))
	}
	got := strings.Join(mpReachPaths(t, family.IPv4Unicast, (*delivered)[0]), " ")
	want := pathsLimitRecvPrefix + "#1 " + pathsLimitRecvPrefix + "#2 " + pathsLimitRecvOther + "#1"
	if got != want {
		t.Fatalf("delivered paths %q, want %q", got, want)
	}
	if admitted, refused := recvSessionTracked(t, s, family.IPv4Unicast, pathsLimitRecvPrefix); admitted != 2 || refused != 1 {
		t.Fatalf("ceiling holds %d admitted, %d refused; want 2, 1", admitted, refused)
	}
	prefixReceiveCount(t, s, family.IPv4Unicast, 3)
}
