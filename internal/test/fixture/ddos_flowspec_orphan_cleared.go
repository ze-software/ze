// Design: docs/guide/ddos-mitigation.md -- what each end of the FlowSpec plugin does to a live announcement
// Related: ddos_flowspec_exit_linux.go -- the Linux-only flood drivers that share the victim, the record key and the report
// Related: register_ddos_flowspec_orphan_cleared.go -- registers the clearing barrier on every host

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ddosFlowspecExitVictim is the remote victim both FlowSpec exit tests flood.
// The FlowSpec responder defers a box-owned victim to on-host mitigation, so
// the victim sits behind the transit veth plugin/ddos-transit-forward-drop-setup
// builds, as for test/plugin/ddos-announce-rate-limit.ci. The detector names
// it as a host prefix (characterize.go characterizeTarget), so the rule the
// blackhole fallback announces is destination-ipv4 203.0.113.9/32 and nothing
// else.
const ddosFlowspecExitVictim = "203.0.113.9"

// ddosFlowspecExitRecord is the record key the FlowSpec plugin writes for its
// live announcement (internal/plugins/ddos/flowspec/register.go
// announcementKey). It is spelled here, not imported: a fixture links no
// plugin, and the key is a contract an operator reads in `ze data registered`.
const ddosFlowspecExitRecord = "meta/ddos-flowspec/announcement"

// ddosFlowspecOrphanCode is the doctor code of a FlowSpec rule no responder
// owns (internal/core/diagnostic CodeDoctorDDoSFlowspecOrphanAnnouncement),
// spelled here for the same reason as ddosFlowspecExitRecord: it is what an
// operator reads in `ze doctor`.
const ddosFlowspecOrphanCode = "doctor-ddos-flowspec-orphan-announcement"

// ddosFlowspecOrphanCleared waits until the fix `ze doctor` printed has
// cleared the finding: in one round, the daemon answers `show doctor` with no
// ddos-flowspec orphan finding AND answers `show data cat` for the record with
// the store's own refusal for a key that does not exist
// (ddosFlowspecRecordMissing). Both are asked in the same round, and the doctor
// answer must exit zero, so a daemon that is down never reads as cleared. Any
// other failed read of the record (a refused command, a daemon that is down, a
// store error) is not the record's absence, so it reads as not cleared yet.
//
// The restart the fix runs answers once the new process reached its running
// stage, and the plugin withdraws the record from OnStarted after that, so the
// clearing is a state to wait for, not one the restart's answer orders.
func ddosFlowspecOrphanCleared(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: plugin/ddos-flowspec-orphan-cleared")
	}
	environment := os.Environ()
	doctorCommand := "show doctor | json"
	recordCommand := "show data cat " + ddosFlowspecExitRecord
	var doctor, record awaitAnswerRun
	cleared := Poll(ctx, 120, 250*time.Millisecond, func() bool {
		doctor.code, doctor.stdout, doctor.stderr, doctor.err = runCaptured(ctx, environment, "", "ze", "cli", "-c", doctorCommand)
		if doctor.err != nil {
			return false
		}
		if doctor.code != 0 {
			return false
		}
		if strings.Contains(doctor.stdout, ddosFlowspecOrphanCode) {
			return false
		}
		record.code, record.stdout, record.stderr, record.err = runCaptured(ctx, environment, "", "ze", "cli", "-c", recordCommand)
		if record.err != nil {
			return false
		}
		if record.code == 0 {
			return false
		}
		return ddosFlowspecRecordMissing(record.stdout) || ddosFlowspecRecordMissing(record.stderr)
	})
	if !cleared {
		// Each start error is wrapped beside the outcome, and errors.Join drops
		// the nil ones, so a command that started names no start error.
		var doctorStart, recordStart error
		if doctor.err != nil {
			doctorStart = fmt.Errorf("the last `ze cli -c %s` did not start: %w", doctorCommand, doctor.err)
		}
		if record.err != nil {
			recordStart = fmt.Errorf("the last `ze cli -c %s` did not start: %w", recordCommand, record.err)
		}
		return errors.Join(fmt.Errorf("the fix ze doctor printed did not clear %s within 30s (cleared means a doctor answer with no such finding, and the store's not-exist refusal for the record); the last `ze cli -c %s` exited %d, stdout %q; the last `ze cli -c %s` exited %d, stdout %q, stderr %q",
			ddosFlowspecOrphanCode, doctorCommand, doctor.code, awaitAnswerTail(doctor.stdout),
			recordCommand, record.code, awaitAnswerTail(record.stdout), awaitAnswerTail(record.stderr)),
			doctorStart, recordStart)
	}
	return ddosFlowspecReport("FLOWSPEC-REPORT-CLEARED " + ddosFlowspecExitVictim + "/32")
}

// ddosFlowspecRecordMissing reports whether one stream of a `show data cat`
// answer carries the store's own refusal for a record key that does not
// exist. No other failure counts: a refused command, a daemon that is down and
// a store error each answer with another text.
//
// catAnswer (internal/component/config/storage/cli storage_data.go) answers a
// failed ReadKey with storageRefusal, one line, "error: " and the storage
// error. The store words a missing key in one of two ways:
//
//   - the live tree store (option=storage:value=disk): openNode
//     (storage/tree.go) wraps ENOENT as "storage node <path>: no such file or
//     directory". <path> ends in the key, or in the first of its folders that
//     is absent, because treeEncoding.walk opens each folder the same way;
//   - a blob store: zefs BlobStore.readFile answers an fs.PathError,
//     "read <key>: file does not exist".
//
// The SSH route writes the refusal on stderr as the command wrote it
// (ssh/answer.go writeExecError), and the client may trim its "error: "
// prefix, so both streams are read and the prefix is optional.
func ddosFlowspecRecordMissing(stream string) bool {
	const treeMissing = ": no such file or directory"
	blobMissing := "read " + ddosFlowspecExitRecord + ": file does not exist"
	for line := range strings.Lines(stream) {
		line = strings.TrimPrefix(strings.TrimSpace(line), "error: ")
		if line == blobMissing {
			return true
		}
		node, isNode := strings.CutPrefix(line, "storage node ")
		if !isNode {
			continue
		}
		path, isMissing := strings.CutSuffix(node, treeMissing)
		if !isMissing {
			continue
		}
		if ddosFlowspecRecordPath(path) {
			return true
		}
	}
	return false
}

// ddosFlowspecRecordPath reports whether path, a tree store node, is the
// record key or one of its folders: the key itself, meta/ddos-flowspec or
// meta, under the store root.
func ddosFlowspecRecordPath(path string) bool {
	// Bounded by the key's depth: each round drops its last segment.
	key := ddosFlowspecExitRecord
	for {
		if strings.HasSuffix(path, "/"+key) {
			return true
		}
		cut := strings.LastIndexByte(key, '/')
		if cut < 0 {
			return false
		}
		key = key[:cut]
	}
}

// ddosFlowspecReport writes one marker line on stderr. It goes to stderr: a
// plugin's stdout is the protocol channel the encoder owns.
func ddosFlowspecReport(marker string) error {
	var tb textbuf.Buffer
	if _, err := fmt.Fprintln(os.Stderr, tb.Str(marker).String()); err != nil {
		return fmt.Errorf("report %s: %w", marker, err)
	}
	return nil
}
