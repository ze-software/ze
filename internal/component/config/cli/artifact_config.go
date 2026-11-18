// Design: docs/architecture/web-interface.md -- native configuration operations on private artifacts.
// Related: config_detached.go -- the shared statement and serialization producers.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/artifact"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func artifactConfigs(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	var rows []configListRow
	err := owner.ViewStore(ctx, user, artifactValue(args, "artifact"), func(source storage.Storage) error {
		var err error
		rows, err = configListRows(source, false)
		return err
	})
	if err != nil {
		return nil, err
	}
	return plugin.Map{"configs": rows}, nil
}

func artifactHistory(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	result := plugin.Map{}
	err := owner.ViewStore(ctx, user, artifactValue(args, "artifact"), func(source storage.Storage) error {
		name, err := storage.StoredConfigName(source, artifactValue(args, "name"))
		if err != nil {
			return err
		}
		rows, err := configHistoryRows(source, name)
		result["revisions"] = rows
		return err
	})
	return result, err
}

func artifactDiff(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	other, hasOther := args.Keyed("other")
	revision, hasRevision := args.Keyed("revision")
	if hasOther == hasRevision {
		return nil, opererr.New("Choose one configuration or one revision to compare.",
			"The comparison needs exactly one source; neither source was selected or both were supplied.",
			"ze show config artifact diff help")
	}
	var payload plugin.ResponseData
	err := owner.ViewStore(ctx, user, artifactValue(args, "artifact"), func(source storage.Storage) error {
		name, err := storage.StoredConfigName(source, artifactValue(args, "name"))
		if err != nil {
			return err
		}
		var inputs diffInputs
		var diagnostic textbuf.Buffer
		if hasOther {
			other, err = storage.StoredConfigName(source, other)
			if err != nil {
				return err
			}
			// The modeled other selector names a configuration even when its
			// filename is numeric. Only the explicit revision selector indexes history.
			inputs.first, err = readDiffSource(source, other)
			if err != nil {
				return err
			}
			inputs.second, inputs.secondErr = readDiffSource(source, name)
			inputs.firstName, inputs.secondName = other, name
		} else {
			var code int
			inputs, code = readDiffInputs(source, []string{revision, name}, &diagnostic)
			if code != exitOK {
				return errors.New(diagnostic.String())
			}
		}
		if len(inputs.first) > artifact.TextBytesMax {
			return fmt.Errorf("configuration %s exceeds the 4 MiB workspace text limit", inputs.firstName)
		}
		if len(inputs.second) > artifact.TextBytesMax {
			return fmt.Errorf("configuration %s exceeds the 4 MiB workspace text limit", inputs.secondName)
		}
		diff, code := compareDiffInputs(inputs, &diagnostic)
		if code != exitOK {
			return errors.New(diagnostic.String())
		}
		encoded, err := json.Marshal(diffAnswer(diff).Payload)
		payload = plugin.RawJSON(encoded)
		return err
	})
	return payload, err
}

func artifactSet(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	return artifactConfigMutation(ctx, owner, user, args, detachedSet)
}

func artifactActivate(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	return artifactConfigMutation(ctx, owner, user, args, detachedActivate)
}

func artifactDeactivate(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	return artifactConfigMutation(ctx, owner, user, args, detachedDeactivate)
}

func artifactConfigMutation(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs, mutation detachedMutation) (plugin.ResponseData, error) {
	statement, err := owner.Text(ctx, user, artifactValue(args, "text"))
	if err != nil {
		return nil, err
	}
	tokens, err := detachedStatement(statement, mutation)
	if err != nil {
		return nil, err
	}
	return artifactConfigEdit(ctx, owner, user, args, func(before []byte, name string) (*editor.Editor, []string, error) {
		ed, err := editor.NewEditorFromContent(before, name)
		if err != nil {
			return nil, nil, err
		}
		if err := applyDetachedMutation(ed, tokens, mutation); err != nil {
			return nil, nil, errors.Join(err, ed.Close())
		}
		return ed, nil, nil
	})
}

func artifactReplace(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	after, err := owner.Text(ctx, user, artifactValue(args, "text"))
	if err != nil {
		return nil, err
	}
	return artifactConfigEdit(ctx, owner, user, args, func(before []byte, name string) (*editor.Editor, []string, error) {
		return prepareDetachedReplacement(before, after, name)
	})
}

func artifactConfigEdit(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs, prepare func([]byte, string) (*editor.Editor, []string, error)) (plugin.ResponseData, error) {
	result := plugin.Map{"saved": false}
	run := owner.ChangeStore
	if args.Flag("dry-run") {
		run = owner.ViewStore
	}
	err := run(ctx, user, artifactValue(args, "artifact"), func(target storage.Storage) (err error) {
		name, err := storage.StoredConfigName(target, artifactValue(args, "name"))
		if err != nil {
			return err
		}
		before, err := storage.ReadActiveConfig(target, name)
		if err != nil {
			return err
		}
		if len(before) > artifact.TextBytesMax {
			return errors.New("the configuration exceeds the 4 MiB workspace text limit")
		}
		ed, warnings, err := prepare(before, name)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, ed.Close()) }()
		result["diff"] = ed.Diff()
		result["warnings"] = warnings
		if args.Flag("dry-run") {
			return nil
		}
		content, saveWarnings, err := serializeDetachedEditor(ed)
		if err != nil {
			return err
		}
		if len(content) > artifact.TextBytesMax {
			return errors.New("the serialized configuration exceeds the 4 MiB workspace text limit")
		}
		result["warnings"] = append(warnings, saveWarnings...)
		stamp, err := storage.PublishArtifactConfig(target, name, content)
		if err != nil {
			return err
		}
		result["version"] = stamp
		return nil
	})
	if err != nil {
		return nil, err
	}
	result["saved"] = !args.Flag("dry-run")
	return result, nil
}

func artifactRollback(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	revision, err := strconv.Atoi(artifactValue(args, "revision"))
	if err != nil {
		return nil, err
	}
	result := plugin.Map{}
	err = owner.ChangeStore(ctx, user, artifactValue(args, "artifact"), func(target storage.Storage) error {
		name, err := storage.StoredConfigName(target, artifactValue(args, "name"))
		if err != nil {
			return err
		}
		selected, stamp, err := storage.RollbackConfig(target, name, revision)
		if err != nil {
			return err
		}
		result["name"], result["revision"], result["source-version"], result["version"] = name, revision, selected.Stamp, stamp
		return nil
	})
	return result, err
}
