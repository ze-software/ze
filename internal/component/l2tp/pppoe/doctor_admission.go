// Design: docs/architecture/l2tp/bng-5-pppoe.md -- session admission failures
// Related: server.go -- provider selection and failed session construction

package pppoe

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// reportSessionAdmissionFailure preserves the actual provider refusal. It
// releases nothing; the discovery caller still owns rollback. Access identity
// is immutable for this server, so reporting needs no session-table lock.
func (s *InterfaceServer) reportSessionAdmissionFailure(sid uint16, cause error) {
	subject := fmt.Sprintf("access %s (index %d, control %s) session %d", s.ifName, s.ifIndex, s.device, sid)
	ppp.ReportAdmissionFailure(s.logger, subject, ppp.AdmissionSessionFactory, cause, map[string]any{
		"access-interface": s.ifName, "access-index": s.ifIndex,
		"control-device": s.device, "session-id": sid,
	})
}
