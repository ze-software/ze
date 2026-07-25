// Design: docs/architecture/fleet-config.md -- managed client runtime commit wiring
// Related: main.go -- hub startup wires the reload hook

package hub

import (
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/internal/core/audit"
)

func wireManagedCommit(client *managed.ClientConfig, store storage.Storage, configPath string, reload func() error, recorder audit.Recorder) {
	client.OnCommit = func(data []byte) error {
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
}
