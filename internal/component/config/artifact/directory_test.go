// Design: docs/architecture/web-interface.md -- installed backup directory defaults.
package artifact

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/paths"
)

// TestBackupDirectoryInstalledDefaults derives the platform root from the one
// existing paths resolver, then requires the shared backup policy to use it.
func TestBackupDirectoryInstalledDefaults(t *testing.T) {
	for _, test := range []struct {
		binary, want string
	}{
		{binary: "/usr/bin/ze", want: "/var/lib/ze/backups"},
		{binary: "/user/ze", want: "/perm/var/lib/ze/backups"},
		{binary: "/development/ze", want: ""},
	} {
		t.Run(test.binary, func(t *testing.T) {
			directory, err := backupDirectory("", paths.DataDirFromBinary(test.binary))
			require.NoError(t, err)
			require.Equal(t, test.want, directory)
		})
	}
}
