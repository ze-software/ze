// Design: docs/guide/l2tp.md -- complete CoA owner fixture identities
// Related: rfc5176_atomic_owners_test.go -- original partial VLAN failure

package l2tpauthradius

import (
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

// newCoASubscriberOrigin constructs this fixture's original session before
// publication, registration or queued Up. Every invocation mints a distinct
// Birth; numeric labels remain the existing fixture's diagnostic coordinates.
// No event reader reconstructs identity and no global identity map is added.
func newCoASubscriberOrigin(t *testing.T) *fakeL2TPService {
	t.Helper()
	return oneSessionService(t)
}

// Goal: a mixed-control refusal reaches its intended partial native write,
// rather than passing at a missing-ID/Birth gate. Method: the real UDP request
// consumes the exact configured VLAN failure before the NAK is observed.
// Existing tagged positive/negative tests retain all map and rate assertions.
func TestCoAMixedFailureReachesOriginalVLANOwner(t *testing.T) {
	addr, secret, owners := coaMixedFixture(t)
	owners.mapsBackend.mu.Lock()
	owners.mapsBackend.failWrites = 1
	owners.mapsBackend.mu.Unlock()
	response := sendAppliedCoA(t, addr, radius.CodeCoARequest, secret, mixedCoAAttributes())
	wantNAK(t, response, radius.CodeCoANAK, radius.ErrorCauseResourcesUnavailable)
	owners.mapsBackend.mu.Lock()
	remaining := owners.mapsBackend.failWrites
	owners.mapsBackend.mu.Unlock()
	if remaining != 0 {
		t.Fatal("mixed-control negative refused before its original partial VLAN write")
	}
}
