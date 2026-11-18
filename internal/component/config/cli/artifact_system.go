// Design: docs/architecture/web-interface.md -- registered current-system backup transfers.
package cli

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/artifact"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

func artifactSystemSnapshot(ctx context.Context, owner *artifact.Owner, user string, _ command.ValidatedArgs) (plugin.ResponseData, error) {
	transfer, err := owner.SnapshotSystem(ctx, user, func(path string, limit int64) error {
		err := storagecli.BackupSystemTo(ctx, path, limit)
		if errors.Is(err, storagecli.ErrSystemBackupUnavailable) {
			return &artifact.Refusal{Status: 503, Cause: opererr.New("This system has no store available to back up.",
				"No backup was created; this process serves no current device store.", "Open /backups/ on the Ze process serving the device store")}
		}
		if errors.Is(err, storage.ErrBackupAllowance) {
			return &artifact.Refusal{Status: 413, Cause: opererr.New("The complete system backup exceeds the available download allowance.",
				"No backup was retained; complete snapshots need room for every key and its storage framing.", "Close unused backup workspaces and retry, or use request data backup path <absolute-file>")}
		}
		return err
	})
	return artifactTransfer(transfer), err
}
