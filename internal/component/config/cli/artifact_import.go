// Design: docs/architecture/storage-backends.md -- native raw import into a private artifact.
package cli

import (
	"context"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/artifact"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

func artifactImport(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	name := artifactValue(args, "name")
	if !storage.PlainConfigName(name) {
		return nil, opererr.New("The import needs one configuration basename.",
			"A private artifact import does not read a daemon-host path.",
			"ze request config artifact import help")
	}
	count, err := owner.ChangeStoreFromSource(ctx, user, artifactValue(args, "artifact"), artifactValue(args, "transfer"), func(target storage.Storage, content []byte) error {
		selected, err := storage.StoredConfigName(target, name)
		if err != nil {
			return err
		}
		return storagecli.ImportConfig(target, selected, content)
	})
	return plugin.Map{"name": name, artifactFieldBytes: count}, err
}
