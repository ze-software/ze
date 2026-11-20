// Design: docs/architecture/fleet-config.md -- managed client runtime commit wiring
// Related: main.go -- hub startup wires the reload hook

package hub

import (
	"time"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/zefs"
)

// wireManagedCommit makes a hub push the daemon's config: staged as a
// candidate and applied by reload. Every push, an ordinary one and a revert
// the hub's operator decided, stages over the client's check
// (storage.StageMachineCandidateOver): under the config lock the stage hands
// check the configuration active then, and stages nothing when check refuses
// it, answering check's error as is. The check refuses a configuration other
// than the one the client read before its fetch, so a local commit that
// landed during the fetch is never overwritten. While window, the daemon's
// confirmed-commit window, is open the push is refused with the window's
// refusal naming its owner before anything is read or staged, which the
// managed client returns to the server in its ACK: the window's revert would
// wipe the push (confirm.WriteOutside). A nil window getter, a daemon with no
// window, applies the push directly.
func wireManagedCommit(client *managed.ClientConfig, store storage.Storage, configPath string, reload func() error, recorder audit.Recorder, window func() *confirm.Window) {
	client.CommitOver = func(data []byte, check func(active []byte) error) error {
		var open *confirm.Window
		if window != nil {
			open = window()
		}
		return confirm.WriteOutside(open, func() error {
			return applyManagedPush(store, configPath, func() error {
				_, err := storage.StageMachineCandidateOver(store, configPath, data, time.Now(), check)
				return err
			}, reload, recorder)
		})
	}
}

// applyManagedPush is a managed push once the window allows it: stage writes
// the hub's configuration as the candidate, and reload makes it active. A push
// whose reload succeeded is the hub's configuration made active, so its hash
// becomes the baseline (recordPushBaseline). A push that was refused, by the
// stage or by the reload, made nothing active and leaves the baseline alone;
// the stage's refusal is answered as is, so a caller can tell its cause.
//
// The push stages through the store's machine entry: a managed device freezes
// an operator's change to its own configuration, and the hub's push is the
// one change it exists to take (storage.StageMachineCandidateOver).
func applyManagedPush(store storage.Storage, configPath string, stage, reload func() error, recorder audit.Recorder) error {
	if err := stage(); err != nil {
		return err
	}
	if err := reload(); err != nil {
		return withUnwind(err, unwindStep{unwindCandidateCleanup, storage.ClearCandidate(store, configPath)})
	}
	recordPushBaseline(store)
	recordDaemonReloadAudit(recorder, "managed", "local", audit.System, "managed config push")
	return nil
}

// recordPushBaseline records the configuration a hub push made active as the
// device's baseline: the hash of what the store now holds as active, read
// back, never the hash of the bytes the hub sent.
//
// A baseline that cannot be recorded does not fail the push: the daemon runs
// the pushed configuration, and telling the hub otherwise would be false. The
// failure is logged under doctor-managed-diverged, because the device then
// compares its configuration with an older baseline and holds at the next hub
// change. The next fetch the hub answers "current" records the baseline
// (managed fetchAgainstBaseline), and that fetch is the one the managed client
// makes when it next connects.
func recordPushBaseline(store storage.Storage) {
	if _, err := managed.RecordActiveAsBaseline(store); err != nil {
		slogutil.Logger("hub.managed").Warn("the hub's configuration is active and its baseline could not be recorded, "+
			"so this device holds at the next hub change unless it reconnects to its hub first; check the store with: ze show data check",
			"code", diagnostic.CodeDoctorManagedDiverged, "key", zefs.KeyManagedBaseVersion.Pattern, "error", err)
	}
}
