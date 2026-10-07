// Design: docs/architecture/config/syntax.md -- config schema stamp and rollback recovery
// Related: ../../component/config/stamp.go -- RecoverConfig, the behavior test/ui/config-schema-stamp-recovers.ci drives

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
)

func init() {
	Register("config/schema-stamp-seed", schemaStampSeed)
}

// schemaStampConfigName is the config the daemon in the scenario starts on.
const schemaStampConfigName = "router.conf"

// schemaStampCompatible is the rollback version recovery is expected to load.
// It carries NO stamp on purpose: an unstamped config is never newer than the
// running binary (version.IsNewerRelease), whereas any dated stamp would be
// judged newer than a "dev" build and skipped.
const schemaStampCompatible = "environment {\n\tcli {\n\t\tformat {\n\t\t\tdefault table;\n\t\t}\n\t}\n}\n"

// schemaStampNewer is the active config a newer release wrote. Its body does
// not parse under this release, which is what sends startup into recovery
// (cmd/ze/hub/main.go, the LoadConfig failure branch).
const schemaStampNewer = "# ze-schema: 99.12.31\nenvironment {\n\tleaf-from-a-newer-release on;\n}\n"

// schemaStampSeed writes the store a newer release would leave behind: an
// older compatible version in history, then the newer-stamped config promoted
// to active and written to the file the daemon is started on. Both versions are dated in the past so the walk order is fixed.
func schemaStampSeed(_ context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("config/schema-stamp-seed takes no arguments")
	}

	store, err := storage.Create(".")
	if err != nil {
		return fmt.Errorf("create the live store: %w", err)
	}

	now := time.Now().Truncate(time.Second)
	seedErr := schemaStampWrite(store, now)
	if closeErr := store.Close(); closeErr != nil {
		seedErr = errors.Join(seedErr, fmt.Errorf("close the live store: %w", closeErr))
	}
	if seedErr != nil {
		return seedErr
	}
	// `ze start router.conf` names an explicit file, so the daemon reads the
	// file itself and keeps the store for history (storage.ReadConfigSource).
	if err := os.WriteFile(schemaStampConfigName, []byte(schemaStampNewer), 0o600); err != nil {
		return fmt.Errorf("write the newer-stamped config file: %w", err)
	}
	fmt.Fprintln(os.Stdout, "seeded: a compatible rollback version and a newer-stamped active config") //nolint:errcheck // fixture progress output
	return nil
}

func schemaStampWrite(store storage.Storage, now time.Time) error {
	if err := store.WriteVersion(schemaStampConfigName, []byte(schemaStampCompatible), now.Add(-48*time.Hour)); err != nil {
		return fmt.Errorf("write the compatible rollback version: %w", err)
	}
	if _, err := storage.WriteCandidateVersion(store, schemaStampConfigName, []byte(schemaStampNewer), now.Add(-time.Hour)); err != nil {
		return fmt.Errorf("write the newer-stamped candidate: %w", err)
	}
	if err := storage.PromoteCandidate(store, schemaStampConfigName); err != nil {
		return fmt.Errorf("promote the newer-stamped config to active: %w", err)
	}
	return nil
}
