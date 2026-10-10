// Design: docs/architecture/fleet-config.md -- managed client runtime commit wiring
// Related: main.go -- hub startup wires the reload hook

package hub

import (
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/internal/core/audit"
)

// wireManagedCommit makes a hub push the daemon's config: staged as a
// candidate and applied by reload. While window, the daemon's confirmed-commit
// window, is open the push is refused with the window's refusal naming its
// owner, which the managed client returns to the server in its ACK: the
// window's revert would wipe the push (confirm.WriteOutside). A nil window
// getter, a daemon with no window, applies the push directly.
func wireManagedCommit(client *managed.ClientConfig, store storage.Storage, configPath string, reload func() error, recorder audit.Recorder, window func() *confirm.Window) {
	client.OnCommit = func(data []byte) error {
		var open *confirm.Window
		if window != nil {
			open = window()
		}
		return confirm.WriteOutside(open, func() error {
			return applyManagedPush(store, configPath, data, reload, recorder)
		})
	}
}

// applyManagedPush is a managed push once the window allows it.
func applyManagedPush(store storage.Storage, configPath string, data []byte, reload func() error, recorder audit.Recorder) error {
	if _, err := storage.WriteCandidateVersion(store, configPath, data, time.Now()); err != nil {
		return err
	}
	if err := reload(); err != nil {
		if clearErr := storage.ClearCandidate(store, configPath); clearErr != nil {
			return fmt.Errorf("%w (candidate cleanup failed: %w)", err, clearErr)
		}
		return err
	}
	recordDaemonReloadAudit(recorder, "managed", "local", audit.System, "managed config push")
	return nil
}
