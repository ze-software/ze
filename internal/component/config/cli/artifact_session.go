// Design: docs/architecture/web-interface.md -- explicit native saved sessions in artifacts.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/artifact"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/zefs"
)

// artifactSessionOperation reopens native account/local-origin records for
// every explicit operation. Storage owns those records; no Editor survives a
// callback and committed reads never implicitly enter this session path.
func artifactSessionOperation(action string) artifactOperation {
	return func(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
		if err := editor.ValidateUser(user); err != nil {
			return nil, err
		}
		var text []byte
		if token, found := args.Keyed("text"); found {
			var err error
			text, err = owner.Text(ctx, user, token)
			if err != nil {
				return nil, err
			}
		}
		result := plugin.Map{"committed": false}
		err := owner.ChangeStore(ctx, user, artifactValue(args, "artifact"), func(target storage.Storage) (err error) {
			name, err := storage.StoredConfigName(target, artifactValue(args, "name"))
			if err != nil {
				return err
			}
			if err := boundedArtifactSession(target, name); err != nil {
				return err
			}
			ed, err := editor.NewIsolatedEditorWithStorage(target, name)
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, ed.Close()) }()
			ed.SetSession(editor.NewEditSession(user, "local"))
			if err := ed.ResumeSession(); err != nil {
				return err
			}
			ed.SetOfflinePromotion()
			ed.SetPreCommitValidate(func(candidate string) error {
				if len(candidate) > artifact.TextBytesMax {
					return errors.New("the candidate exceeds the 4 MiB artifact text limit")
				}
				return ValidateContent(candidate, name)
			})
			switch action {
			case "resume", "diff", "changes", "history":
			case "set", "activate", "deactivate":
				mutation := detachedSet
				if action == "activate" {
					mutation = detachedActivate
				}
				if action == "deactivate" {
					mutation = detachedDeactivate
				}
				tokens, err := detachedStatement(text, mutation)
				if err != nil {
					return err
				}
				if err := applyDetachedMutation(ed, tokens, mutation); err != nil {
					return err
				}
			case "replace", "merge":
				loaded, err := ed.ParseLoad(string(text), nil)
				if err != nil {
					return err
				}
				if action == "replace" {
					err = ed.LoadReplace(nil, loaded)
				} else {
					err = ed.LoadMerge(nil, loaded)
				}
				if err != nil {
					return err
				}
			case "delete":
				tokens, err := detachedEditingStatement(text, "delete", "ze request config artifact session delete help")
				if err != nil {
					return err
				}
				if err := ed.DeleteByPath(tokens); err != nil {
					return err
				}
			case "save":
				if err := ed.SaveDraft(); err != nil {
					return err
				}
			case "discard":
				if err := ed.DiscardSessionPath(nil); err != nil {
					return err
				}
			case "commit":
				validator, err := editor.NewConfigValidator()
				if err != nil {
					return err
				}
				validation := validator.ValidateTransition(ed.OriginalContent(), ed.WorkingContent())
				if len(validation.Errors) > 0 {
					return fmt.Errorf("the artifact candidate is invalid: %s", validation.Errors[0].Message)
				}
				if len(validation.Warnings) > 0 && !args.Flag("force") {
					return opererr.New("The artifact candidate has validation advisories.", validation.Warnings[0].Message,
						"ze request config artifact session commit help")
				}
				var committed *editor.CommitResult
				var content string
				if args.Flag("force") {
					committed, content, err = ed.CommitSessionCandidateForce(time.Now())
				} else {
					committed, content, err = ed.CommitSessionCandidate(time.Now())
				}
				if err != nil {
					return err
				}
				if len(committed.Conflicts) > 0 {
					return opererr.New("The artifact session has conflicting changes.",
						"No committed configuration changed; inspect and reconcile the saved session before retrying.", "ze request config artifact session changes help")
				}
				if committed.Applied > 0 {
					if err := ed.NotifyReload(); err != nil {
						return err
					}
					if err := ed.MarkCommittedContent(content); err != nil {
						committed.Warnings = append(committed.Warnings, err.Error())
					}
					result["committed"] = true
				}
				warnings := append([]string(nil), committed.Warnings...)
				for _, issue := range validation.Warnings {
					warnings = append(warnings, issue.Message)
				}
				result["warnings"], result["applied"] = warnings, committed.Applied
			default:
				panic("BUG: unregistered artifact session operation")
			}
			if action == "resume" || action == "diff" {
				result["diff"] = ed.Diff()
			}
			if action == "changes" {
				changes := ed.PendingChanges(ed.SessionID())
				rows := make([]map[string]any, 0, len(changes))
				for _, change := range changes {
					rows = append(rows, map[string]any{"path": change.Path, "kind": string(change.Kind), "detail": ed.PendingChangeSummary(change)})
				}
				result["changes"] = rows
			}
			if action == "history" {
				rows, err := configHistoryRows(target, name)
				if err != nil {
					return err
				}
				result["revisions"] = rows
			}
			result["operation"] = action
			encoded, err := json.Marshal(result)
			if err != nil {
				return err
			}
			if len(encoded) > command.BlobRemoteOctetsMax {
				return opererr.New("The complete artifact session answer exceeds the 8 MiB response limit.",
					"The staged operation was not published; no truncated answer or partial artifact was accepted.",
					"ze request config artifact download begin help")
			}
			return nil
		})
		return result, err
	}
}

// boundedArtifactSession bounds every native text record before its parser
// opens the saved state, including other accounts' change files in the draft.
func boundedArtifactSession(source storage.Storage, name string) error {
	prefix := zefs.KeyFileActive.Prefix()
	keys, err := source.ListKeys(prefix)
	if err != nil {
		return err
	}
	for _, key := range keys {
		file := strings.TrimPrefix(key, prefix)
		if file != name && file != editor.DraftPath(name) && !strings.HasPrefix(file, name+storage.EditorChangeSeparator) {
			continue
		}
		content, err := source.ReadKey(key)
		if err != nil {
			return err
		}
		if len(content) > artifact.TextBytesMax {
			return fmt.Errorf("saved artifact text %s exceeds the 4 MiB text limit", file)
		}
	}
	return nil
}
