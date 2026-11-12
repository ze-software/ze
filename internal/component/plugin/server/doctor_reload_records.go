// Design: docs/architecture/doctor-and-health-checks.md -- reload and stop records a check reads
// Related: doctor_snapshot.go -- the published server this read shares
// Related: startup_removal.go -- pluginRemoval.recoveryErr, the removal record
// Related: reload_tx.go -- runTxCoordinator, which records and clears the restore failure
//
// The reload records follow the snapshot's shape: a runtime doctor check
// reads value types from the published server, and with none published the
// answer is ErrDoctorSnapshotNotPublished, never an empty list that reads as
// "every reload left things as they were".

package server

import (
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// RemovalUnrecovered is one plugin a refused reload removed whose restore
// failed: it stays removed until a later reload retries the restore
// (retryRemovalRecovery).
type RemovalUnrecovered struct {
	Plugin string
	Err    string
}

// CapabilityRestoreFailure is a refused reload whose capability declarations
// could not be put back, so an OPEN built since may carry what the refused
// candidate declared.
type CapabilityRestoreFailure struct {
	Err  string
	Time time.Time
}

// ReloadDoctorRecords is what the reload machinery left unrestored, as value
// types for the doctor checks.
type ReloadDoctorRecords struct {
	// RemovalsUnrecovered holds one entry per unrestored removal, by plugin.
	RemovalsUnrecovered []RemovalUnrecovered
	// CapabilitiesNotRestored holds the latest restore failure since the last
	// committed reload, or nothing: a committed reload installs the running
	// configuration's declarations again.
	CapabilitiesNotRestored []CapabilityRestoreFailure
}

// PluginReloadRecords answers the published server's reload records, or
// ErrDoctorSnapshotNotPublished when no server runs in this process. Safe for
// concurrent use.
func PluginReloadRecords() (ReloadDoctorRecords, error) {
	s := doctorSnapshotSource.Load()
	if s == nil {
		return ReloadDoctorRecords{}, ErrDoctorSnapshotNotPublished
	}
	return ReloadDoctorRecords{
		RemovalsUnrecovered:     s.removalsUnrecovered(),
		CapabilitiesNotRestored: s.capabilityRestore.read(),
	}, nil
}

// removalsUnrecovered reads every removal whose last restore failed, a retry
// in progress included: until it succeeds the plugin is still removed. The
// error's text is answered bounded (process.BoundDoctorDetail), because its
// chain can carry what a plugin answered; the error stays whole on the
// removal, one per configured plugin, for the retry and the log.
func (s *Server) removalsUnrecovered() []RemovalUnrecovered {
	s.removalMu.Lock()
	defer s.removalMu.Unlock()
	var records []RemovalUnrecovered
	for proc, attempt := range s.removals {
		if attempt.recoveryErr == nil {
			continue
		}
		records = append(records, RemovalUnrecovered{Plugin: proc.Name(), Err: process.BoundDoctorDetail(attempt.recoveryErr.Error())})
	}
	slices.SortFunc(records, func(a, b RemovalUnrecovered) int { return strings.Compare(a.Plugin, b.Plugin) })
	return records
}

// logRemovalUnrecovered is the log line of a failed removal restore. The
// record itself is attempt.recoveryErr, which the doctor check reads.
func logRemovalUnrecovered(plugin string, err error) {
	logger().Error("plugin removal recovery failed; the plugin stays removed until a reload retries it",
		"code", diagnostic.CodeDoctorPluginRemovalUnrecovered, "plugin", plugin, "error", err)
}

// capabilityRestoreRecord holds the latest capability restore failure since
// the last committed reload. Safe for concurrent use.
type capabilityRestoreRecord struct {
	mu      sync.Mutex
	failure []CapabilityRestoreFailure // at most one entry
}

// failed records and logs a restore that did not put the running
// configuration's declarations back after a refused reload. The log line
// keeps the whole error; the record keeps its text bounded
// (process.BoundDoctorDetail), because the chain can carry a plugin's answer.
func (r *capabilityRestoreRecord) failed(err error) {
	logger().Error("config tx: restore plugin capabilities failed after a refused reload",
		"code", diagnostic.CodeDoctorPluginCapabilitiesNotRestored, "error", err)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failure = []CapabilityRestoreFailure{{Err: process.BoundDoctorDetail(err.Error()), Time: time.Now().UTC()}}
}

// clear drops the record. A committed reload calls it: the declarations in
// force are then the committed configuration's own.
func (r *capabilityRestoreRecord) clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failure = nil
}

// read answers a copy of the record.
func (r *capabilityRestoreRecord) read() []CapabilityRestoreFailure {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.failure)
}
