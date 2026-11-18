// Design: docs/architecture/config/environment.md -- backup workspace directory override.
package paths

import "github.com/ze-software/ze/internal/core/env"

// BackupDirectoryKey names the startup override shared by YANG environment
// plumbing and the private artifact owner. Empty uses the platform data folder.
const BackupDirectoryKey = "ze.backup.directory"

var _ = env.MustRegister(env.EnvEntry{Key: BackupDirectoryKey, Type: env.TypeString,
	Description: "Absolute backup workspace parent directory; change requires a daemon restart"})
