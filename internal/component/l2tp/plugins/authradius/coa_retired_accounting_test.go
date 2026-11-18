// Design: docs/guide/l2tp.md -- issued accounting ID belongs to its original Birth
// Related: coa_captured_collision_test.go -- signed UDP and real rate-owner fixture

package l2tpauthradius

import (
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

// Goal: an old issued accounting ID cannot authorize a replacement that is
// already visible before matching. Method: retire the accounted row, publish
// its equal-coordinate successor, then send both signed request types through
// the real listener and check the useful survivor and both installed rates.
// MUTATION: key issued accounting IDs by reusable tunnel/SID instead of Birth.
func TestDynamicAuthorizationRetiredAccountingBirthKeepsReplacement(t *testing.T) {
	for _, request := range []struct {
		name string
		code uint8
		nak  uint8
	}{
		{name: "coa", code: radius.CodeCoARequest, nak: radius.CodeCoANAK},
		{name: "disconnect", code: radius.CodeDisconnectRequest, nak: radius.CodeDisconnectNAK},
	} {
		t.Run(request.name, func(t *testing.T) {
			address, secret, service, backend := newCoACollisionFixture(t)
			service.mu.Lock()
			service.originals[1].alive = false
			service.originals[2].alive = true
			service.mu.Unlock()
			attributes := coaCollisionAttributes(1)
			if request.code == radius.CodeCoARequest {
				attributes = append(attributes, radius.Attr{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit/8mbit")})
			}
			response := sendAppliedCoA(t, address, request.code, secret, attributes)
			wantNAK(t, response, request.nak, radius.ErrorCauseSessionNotFound)
			service.requireRows(t, [3]bool{true, false, true})
			for index := range 2 {
				coaCollisionRequireRates(t, backend, service.originals[index].snapshot.PppInterface, 10_000_000, 5_000_000)
			}
			service.mu.Lock()
			defer service.mu.Unlock()
			if service.captures != 0 {
				t.Errorf("old accounting ID selected %d current originals, want zero", service.captures)
			}
			if service.legacyOperations != 0 {
				t.Errorf("old accounting ID caused %d reusable-SID operations, want zero", service.legacyOperations)
			}
		})
	}
}
