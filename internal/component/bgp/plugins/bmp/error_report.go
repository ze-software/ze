// Design: docs/architecture/doctor-and-health-checks.md -- BMP runtime error reporting and bounded rate-limiting
// Related: register_doctor_codes.go -- doctor event code registrations
// Related: metrics.go -- Prometheus counters for BMP route-inject and sender drops

package bmp

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	bmpWarnWindow         = time.Minute
	receiverInjectWarnCap = 1024
	senderWarnCap         = 256

	bmpMsgKindPeerUp          = "peer-up"
	bmpMsgKindPeerDown        = "peer-down"
	bmpMsgKindRouteMirroring  = "route-mirroring"
	bmpMsgKindRouteMonitoring = "route-monitoring"
	bmpMsgKindLocRIBPeerUp    = "loc-rib-peer-up"
	bmpMsgKindLocRIBEOR       = "loc-rib-eor"
	bmpMsgKindLocRIBPeerDown  = "loc-rib-peer-down"
	bmpMsgKindDrain           = "drain"
	bmpMsgKindStatistics      = "statistics"

	dropReasonDisconnected  = "disconnected"
	dropReasonQueueOverflow = "queue-overflow"
	dropReasonWriteError    = "write-error"
)

type senderWarnKey struct {
	collector string
	message   string
}

func senderDropReason(err error) string {
	if errors.Is(err, errNotConnected) {
		return dropReasonDisconnected
	}
	if errors.Is(err, errQueueOverflow) {
		return dropReasonQueueOverflow
	}
	return dropReasonWriteError
}

func fitReportText(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	const suffix = "..."
	if maxBytes <= len(suffix) {
		cut := maxBytes
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		return s[:cut]
	}
	cut := maxBytes - len(suffix)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + suffix
}

func allowBoundedWarn[K comparable](m map[K]time.Time, key K, limit int, now time.Time) bool {
	if last, ok := m[key]; ok {
		if now.Sub(last) < bmpWarnWindow {
			return false
		}
	}
	if _, exists := m[key]; !exists {
		if len(m) >= limit {
			for k, ts := range m {
				if now.Sub(ts) >= bmpWarnWindow {
					delete(m, k)
				}
			}
			if len(m) >= limit {
				var oldestKey K
				var oldestTime time.Time
				first := true
				for k, ts := range m {
					if first || ts.Before(oldestTime) {
						oldestKey = k
						oldestTime = ts
						first = false
					}
				}
				delete(m, oldestKey)
			}
		}
	}
	m[key] = now
	return true
}

func (bp *BMPPlugin) allowReceiverInjectWarn(peerKey string) bool {
	bp.warnMu.Lock()
	defer bp.warnMu.Unlock()
	if bp.receiverInjectWarned == nil {
		bp.receiverInjectWarned = make(map[string]time.Time)
	}
	return allowBoundedWarn(bp.receiverInjectWarned, peerKey, receiverInjectWarnCap, time.Now())
}

func (bp *BMPPlugin) pruneReceiverInjectWarnPeer(peerKey string) {
	bp.warnMu.Lock()
	delete(bp.receiverInjectWarned, peerKey)
	bp.warnMu.Unlock()
}

func (bp *BMPPlugin) pruneReceiverInjectWarnRouter(remote string) {
	prefix := remote + ":"
	bp.warnMu.Lock()
	for k := range bp.receiverInjectWarned {
		if strings.HasPrefix(k, prefix) {
			delete(bp.receiverInjectWarned, k)
		}
	}
	bp.warnMu.Unlock()
}

func (bp *BMPPlugin) allowSenderDeliveryWarn(collector, message string) bool {
	bp.warnMu.Lock()
	defer bp.warnMu.Unlock()
	if bp.senderDeliveryWarned == nil {
		bp.senderDeliveryWarned = make(map[senderWarnKey]time.Time)
	}
	return allowBoundedWarn(bp.senderDeliveryWarned, senderWarnKey{collector: collector, message: message}, senderWarnCap, time.Now())
}

func (bp *BMPPlugin) pruneSenderDeliveryWarn(collector string) {
	bp.warnMu.Lock()
	for k := range bp.senderDeliveryWarned {
		if k.collector == collector {
			delete(bp.senderDeliveryWarned, k)
		}
	}
	bp.warnMu.Unlock()
}

func (bp *BMPPlugin) clearSenderDeliveryWarn() {
	bp.warnMu.Lock()
	clear(bp.senderDeliveryWarned)
	bp.warnMu.Unlock()
}

func (bp *BMPPlugin) issueQueue() *issuequeue.Queue {
	if bp.awaitingHandshake.Load() {
		return nil
	}
	return bp.issues
}

func (bp *BMPPlugin) reportReceiverRIBSyncFailure(subject, operation, trigger string, err error) {
	if bp.isStopping() {
		return
	}
	var msgBuf textbuf.Buffer
	msgBuf.Str("BMP receiver failed to ").Str(operation).Str(" for ").Str(subject).
		Str(" during ").Str(trigger).
		Str("; the BMP RIB view may lag or keep stale routes. Cause: ").Str(err.Error())
	msg := fitReportText(msgBuf.String(), rpc.ReportMessageMax)
	subj := fitReportText(subject, rpc.ReportSubjectMax)

	failed := func(reportErr error) {
		logger().Warn("BMP receiver could not deliver RIB sync failure report to the daemon; check daemon health",
			"code", codeDoctorBMPRIBSyncFailed,
			"subject", subject,
			"operation", operation,
			"error", reportErr,
			"fix", fixShowBMPPeers,
		)
	}
	reportErr := bp.issueQueue().RaiseError(codeDoctorBMPRIBSyncFailed, subj, msg, fixShowBMPPeers, failed)
	logger().Warn("BMP receiver failed to sync monitored routes with the BGP RIB; the BMP RIB view may lag or keep stale routes",
		"code", codeDoctorBMPRIBSyncFailed,
		"subject", subject,
		"operation", operation,
		"trigger", trigger,
		"error", err,
		"fix", fixShowBMPPeers,
		issuequeue.Outcome(reportErr),
	)
}

func (bp *BMPPlugin) reportReceiverSessionError(remote string, msgType uint8, err error) {
	if bp.isStopping() {
		return
	}
	var msgBuf textbuf.Buffer
	msgBuf.Str("BMP receiver failed to read message body (type ").Uint8(msgType).
		Str(") from router ").Str(remote).
		Str("; closing session and withdrawing its routes. Cause: ").Str(err.Error())
	msg := fitReportText(msgBuf.String(), rpc.ReportMessageMax)
	subj := fitReportText(remote, rpc.ReportSubjectMax)

	failed := func(reportErr error) {
		logger().Warn("BMP receiver could not deliver session read failure report to the daemon; check daemon health",
			"code", codeDoctorBMPReceiverSessionError,
			"remote", remote,
			"error", reportErr,
			"fix", fixShowBMPSessions,
		)
	}
	reportErr := bp.issueQueue().RaiseError(codeDoctorBMPReceiverSessionError, subj, msg, fixShowBMPSessions, failed)
	logger().Warn("BMP receiver failed to read message body from monitored router; closing session",
		"code", codeDoctorBMPReceiverSessionError,
		"remote", remote,
		"msg-type", msgType,
		"error", err,
		"fix", fixShowBMPSessions,
		issuequeue.Outcome(reportErr),
	)
}

func (bp *BMPPlugin) reportSenderWriteFailure(collector, message string, err error) {
	if bp.isStopping() {
		return
	}
	reason := senderDropReason(err)
	incMessageDropMetric(collector, message, reason)
	if !bp.allowSenderDeliveryWarn(collector, message) {
		return
	}

	var subjBuf textbuf.Buffer
	subjBuf.Str(collector).Byte('/').Str(message)
	subject := fitReportText(subjBuf.String(), rpc.ReportSubjectMax)

	var msgBuf textbuf.Buffer
	msgBuf.Str("BMP sender could not deliver ").Str(message).
		Str(" to collector ").Str(collector).
		Str(" (").Str(reason).
		Str("); collector visibility may lag until the session recovers. Cause: ").Str(err.Error())
	msg := fitReportText(msgBuf.String(), rpc.ReportMessageMax)

	failed := func(reportErr error) {
		logger().Warn("BMP sender could not deliver failure report to the daemon; check daemon health",
			"code", codeDoctorBMPSenderDeliveryFailed,
			"collector", collector,
			"message", message,
			"error", reportErr,
			"fix", fixShowBMPCollectors,
		)
	}
	reportErr := bp.issueQueue().RaiseError(codeDoctorBMPSenderDeliveryFailed, subject, msg, fixShowBMPCollectors, failed)
	logger().Warn("BMP sender could not deliver monitoring message to collector; collector visibility may lag",
		"code", codeDoctorBMPSenderDeliveryFailed,
		"collector", collector,
		"message", message,
		"reason", reason,
		"error", err,
		"fix", fixShowBMPCollectors,
		issuequeue.Outcome(reportErr),
	)
}
