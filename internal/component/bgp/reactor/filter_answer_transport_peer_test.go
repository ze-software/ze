// Design: docs/architecture/core-design.md -- Policy Filter Chain, the export step
// Related: filter_answer_transport_test.go -- answerTransport, the calls it records

package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// callsFor answers how many filter-update calls the transport answered for
// the route sent toward peer, the destination address an export chain run
// names (input.Peer). Safe for concurrent use, as calls is.
func (a *answerTransport) callsFor(peer string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, input := range a.sent {
		if input.Peer == peer {
			n++
		}
	}
	return n
}

// answerPolicyAt compiles a policy whose members are the answerPlugin filters
// names, in order, at attachment under final, as bgp/config builds a chain
// that names plugin filters. The attachment decides which fields a member may
// set: answerPolicy passes answerAttachment, and a filter that sets the
// AS_PATH needs one that allows it (libraryIntentExport).
func answerPolicyAt(t testing.TB, attachment string, final routepolicy.Final, names ...string) filterapi.Policy {
	t.Helper()
	reg, err := routepolicy.NewRegistry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	members := make([]policychain.Member, len(names))
	refs := make([]routepolicy.Ref, len(names))
	for i, name := range names {
		members[i] = policychain.Member{Name: name, Plugin: answerPlugin}
		refs[i] = routepolicy.Ref{Name: answerPlugin + ":" + name}
	}
	chain, err := policychain.Compile(attachment, reg, members, final, policychain.Bases{LocalPreference: 100})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return mustPolicy(t, refs, final).WithLibrary(chain)
}
