// Design: docs/guide/l2tp.md -- negotiated snapshot formatting evidence.
package cmd

import "github.com/ze-software/ze/internal/component/l2tp"

// snapshotReadService carries the same copied snapshot for a formatting test's
// list and detail. It preserves the old SID match guard and exposes only the
// read-only capture: every native/mutation operation on it still refuses.
type snapshotReadService struct {
	*fakeService
	captured cqmCommandCapture
}

func (service *snapshotReadService) CaptureSession(selector l2tp.SessionSelector) (l2tp.CapturedSession, error) {
	if selector.SID != service.captured.snapshot.LocalSID {
		return nil, l2tp.ErrSessionNotFound
	}
	return &service.captured, nil
}
