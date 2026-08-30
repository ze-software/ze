// Design: ai/rules/plugins.md -- ze_l2tp partition of the raw-capture display
// Related: capture_raw.go -- the always-on raw-capture dispatcher this fills in

//go:build ze_l2tp

package cmd

import (
	"bytes"
	"encoding/base64"

	"github.com/ze-software/ze/internal/component/l2tp"
)

func rawEntriesToJSON(entries []l2tp.RawCaptureEntry) []map[string]any {
	rows := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		dir := "in"
		if e.Direction == 1 {
			dir = "out"
		}
		rows = append(rows, map[string]any{
			keyTimestamp: e.Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			keyDirection: dir,
			keyBytes:     len(e.Data),
		})
	}
	return rows
}
func exportL2TPPcap(entries []l2tp.RawCaptureEntry) ([]byte, error) {
	var buf bytes.Buffer
	if err := writePcapHeader(&buf, 1500, LinkTypeRaw); err != nil {
		return nil, err
	}
	for i := len(entries) - 1; i >= 0; i-- {
		e := &entries[i]
		if err := writePcapPacket(&buf, e.Timestamp, e.Data); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// captureRawL2TPStart enables l2tp raw capture when the subsystem is running:
// the moved l2tp branch of captureRawStart.
func captureRawL2TPStart(started []string, protocol string) []string {
	if protocol != "" && protocol != capL2TP {
		return started
	}
	if svc := l2tp.LookupService(); svc != nil {
		svc.EnableRawCapture()
		started = append(started, capL2TP)
	}
	return started
}

// captureRawL2TPStop disables l2tp raw capture: the moved l2tp branch of
// captureRawStop.
func captureRawL2TPStop(stopped []string, protocol string) []string {
	if protocol != "" && protocol != capL2TP {
		return stopped
	}
	if svc := l2tp.LookupService(); svc != nil {
		svc.DisableRawCapture()
		stopped = append(stopped, capL2TP)
	}
	return stopped
}

// captureRawL2TPDump fills the l2tp section of `capture-raw dump`: the moved
// l2tp branch of captureRawDump.
func captureRawL2TPDump(result map[string]any, protocol, format string, limit int) {
	if protocol != "" && protocol != capL2TP {
		return
	}
	svc := l2tp.LookupService()
	if svc == nil {
		result["l2tp"] = msgSubsystemNotRunning
		return
	}
	entries := svc.RawCaptureSnapshot(limit)
	switch {
	case entries == nil:
		result["l2tp"] = "raw capture not enabled (use capture-raw start l2tp)"
	case format == fmtPcap:
		pcapData, err := exportL2TPPcap(entries)
		if err != nil {
			result["l2tp-error"] = err.Error()
		} else {
			result["l2tp-pcap"] = base64.StdEncoding.EncodeToString(pcapData)
			result["l2tp-packets"] = len(entries)
		}
	default:
		result["l2tp"] = rawEntriesToJSON(entries)
		result["l2tp-count"] = len(entries)
	}
}

// captureRawL2TPNote has nothing to add in a ze_l2tp build: an empty started
// list already means "subsystem not running", the pre-existing semantics.
func captureRawL2TPNote(string) string { return "" }
