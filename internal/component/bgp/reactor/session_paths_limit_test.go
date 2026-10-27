package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// TestPathsLimitSectionSharedAdmission proves the send side keeps its
// invariants after the admission was made direction-neutral: a refusal at the
// limit journals nothing, an unknown withdrawal passes without a change, a
// replacement at the limit passes, and rollback restores admitted ids.
// Method: drive the send admission directly and read its journal.
func TestPathsLimitSectionSharedAdmission(t *testing.T) {
	admission := newPathsLimitAdmission(
		map[family.Family]uint16{family.IPv4Unicast: 2, family.IPv6Unicast: 3},
		map[family.Family]capability.AddPathMode{family.IPv4Unicast: capability.AddPathSend, family.IPv6Unicast: capability.AddPathReceive},
		capability.AddPathSend)
	if admission[family.IPv6Unicast] != nil {
		t.Fatal("a family without the send direction must carry no send bound")
	}
	state := admission[family.IPv4Unicast]
	if state == nil {
		t.Fatal("the send-direction family must carry its bound")
	}
	if state.auditRefusals {
		t.Fatal("the send side must not audit refusals")
	}

	changes, _ := pathsLimitScratch.Get().(*pathsLimitChanges)
	defer changes.release()
	dst := make([]byte, message.ExtMsgLen)
	n, dropped, err := admission.filter(dst, recvAnnounce(
		pathsLimitNLRI(1, pathsLimitRecvPrefix),
		pathsLimitNLRI(2, pathsLimitRecvPrefix),
		pathsLimitNLRI(3, pathsLimitRecvPrefix),
	), changes)
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 1 {
		t.Fatalf("dropped %d, want 1", dropped)
	}
	if changes.n != 2 {
		t.Fatalf("journal holds %d entries, want 2: a send refusal is not journaled", changes.n)
	}
	if announced, _ := recvBodyPathIDs(t, dst[:n]); !equalIDs(announced, 1, 2) {
		t.Fatalf("sent %v, want [1 2]", announced)
	}

	n, _, err = admission.filter(dst, makeUpdateBody(pathsLimitNLRI(9, pathsLimitRecvPrefix), nil, pathsLimitNLRI(2, pathsLimitRecvPrefix)), changes)
	if err != nil {
		t.Fatal(err)
	}
	if changes.n != 2 {
		t.Fatalf("an unknown withdrawal or a replacement changed the journal to %d entries", changes.n)
	}
	announced, withdrawn := recvBodyPathIDs(t, dst[:n])
	if !equalIDs(announced, 2) {
		t.Fatalf("replacement at the limit gave %v, want [2]", announced)
	}
	if !equalIDs(withdrawn, 9) {
		t.Fatalf("unknown withdrawal gave %v, want [9]", withdrawn)
	}

	changes.rollback()
	if len(state.prefixes) != 0 {
		t.Fatalf("rollback left %d prefixes, want none", len(state.prefixes))
	}
}
