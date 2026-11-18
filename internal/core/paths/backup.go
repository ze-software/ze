// Design: docs/architecture/config/environment.md -- backup workspace directory override.
package paths

import "github.com/ze-software/ze/internal/core/env"

// BackupDirectoryKey names the OS override of the committed backup directory.
// Empty allows the configuration or platform data folder to select it.
const BackupDirectoryKey = "ze.backup.directory"

var _ = env.MustRegister(env.EnvEntry{Key: BackupDirectoryKey, Type: env.TypeString,
	Description: "Absolute backup workspace parent directory overriding committed configuration"})
