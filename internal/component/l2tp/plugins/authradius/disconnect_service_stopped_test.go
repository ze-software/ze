// Design: docs/guide/l2tp.md -- RADIUS Disconnect-Request answers
// Related: coa.go -- handleDisconnect, oneSession, findSessions
// Related: rfc5176_coa_branches_test.go -- teardownRefusingService, the other located-but-not-removed path

package l2tpauthradius

import (
	"slices"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/radius"
)

// stopsAfterLookupService is the one-session service that stops the L2TP
// subsystem (unpublishes itself) as soon as the listener has read its sessions,
// so the Disconnect-Request's session is located and the service that could
// remove it is gone by the time the listener asks for the teardown.
type stopsAfterLookupService struct {
	*fakeL2TPService
	teardownAttempts atomic.Int32
}

func (s *stopsAfterLookupService) Snapshot() l2tp.Snapshot {
	l2tp.PublishService(nil)
	return s.fakeL2TPService.Snapshot()
}

func (s *stopsAfterLookupService) TeardownSession(uint16) error {
	s.teardownAttempts.Add(1)
	return nil
}

// TestRFC5176DisconnectLocatedButServiceStoppedIsNotRemovable checks the answer
// to a Disconnect-Request whose session was located, but whose L2TP service
// stopped before the teardown. RFC 5176 Section 3.5: "Session Context Not
// Removable" is "sent in response to a Disconnect-Request if the NAS was able to
// locate the session context, but could not remove it for some reason", while
// "Session Context Not Found" is for a session context that "does not exist on
// the NAS". The session was found, so the answer is a Disconnect-NAK carrying
// Error-Cause 504 alone, and no teardown is attempted.
//
// Method: the fake service unpublishes itself inside Snapshot, the call that
// locates the session, so the second service lookup in handleDisconnect finds
// none.
func TestRFC5176DisconnectLocatedButServiceStoppedIsNotRemovable(t *testing.T) {
	secret := []byte("test-rfc5176-stopped-secret")
	svc := &stopsAfterLookupService{fakeL2TPService: oneSessionService()}
	l2tp.PublishService(svc)
	t.Cleanup(func() { l2tp.PublishService(nil) })
	addr := coaTestListener(t, secret, nil, nil)

	resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
	})
	if resp.Code != radius.CodeDisconnectNAK {
		t.Errorf("code: got %d, want %d (Disconnect-NAK)", resp.Code, radius.CodeDisconnectNAK)
	}
	if got := errorCauses(t, resp); !slices.Equal(got, []uint32{radius.ErrorCauseSessionNotRemovable}) {
		t.Errorf("Error-Cause: got %v, want [504] (Session Context Not Removable)", got)
	}
	if got := svc.teardownAttempts.Load(); got != 0 {
		t.Errorf("teardown attempts with the service stopped: got %d, want 0", got)
	}
}
