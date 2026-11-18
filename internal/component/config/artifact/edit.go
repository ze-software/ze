// Design: docs/architecture/web-interface.md -- shared detached artifact editing.
// Related: owner.go -- authoritative workspace lifetime and serialization.
package artifact

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
)

type EditResult struct {
	Base   string `json:"base"`
	Review string `json:"review"`
	Diff   string `json:"diff"`
	Status string `json:"message"`
	Saved  bool   `json:"saved"`
}

func readConfig(w *workspace, name string) ([]byte, error) {
	if !slices.Contains(w.configs, name) {
		return nil, missing()
	}
	store, err := storage.OpenBlob(w.path, false)
	if err != nil {
		return nil, storageFailure(err)
	}
	content, readErr := storage.ReadActiveConfig(store, name)
	if err := errors.Join(readErr, store.Close()); err != nil {
		return nil, storageFailure(err)
	}
	if len(content) > TextBytesMax {
		return nil, tooLarge()
	}
	return content, nil
}

// Config returns detached committed bytes. It never resumes a saved draft.
func (m *Owner) Config(ctx context.Context, user, id, name string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return nil, err
	}
	w, err := m.find(user, id)
	if err != nil {
		return nil, err
	}
	return readConfig(w, name)
}

// Edit consumes a completed text source under the same lock as publication.
// The caller MUST Abort the source after the operation on success or failure.
func (m *Owner) Edit(ctx context.Context, user, id, name, text, base, review, action string) (result EditResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return result, err
	}
	w, err := m.find(user, id)
	if err != nil {
		return result, err
	}
	original, err := readConfig(w, name)
	if err != nil {
		return result, err
	}
	result.Base = Digest(original)
	content, err := m.readText(user, text)
	if err != nil {
		return result, err
	}
	if !utf8.Valid(content) {
		return result, refuse(400, "The configuration is not valid UTF-8.", "The artifact was not changed.", "Correct the text and validate it again")
	}
	if base != result.Base {
		return result, refuse(409, "This configuration changed after the form opened.", "Your submitted text is retained.", "Review it against the current saved configuration before saving again")
	}
	prepared, warnings, err := prepare(original, string(content), name)
	if err != nil {
		return result, &Refusal{Status: 400, Cause: opererr.Wrap(err,
			"The configuration could not be accepted.", "The artifact was not changed and your submitted text is retained.",
			"Correct the reported configuration problem and select Validate again")}
	}
	defer func() {
		if closeErr := prepared.editor.Close(); closeErr != nil {
			err = errors.Join(err, storageFailure(closeErr))
		}
	}()
	if action == "validate" {
		result.Status = "Offline configuration validation passed. Target-host and runtime acceptance were not tested." + warnings
		return result, nil
	}
	if action == "review" {
		result.Diff = prepared.editor.Diff()
		if result.Diff == "" {
			result.Diff = "No configuration changes."
		}
		result.Review = ReviewToken(base, string(content))
		result.Status = "Review the changes before saving them into the backup." + warnings
		return result, nil
	}
	if action != "save" {
		return result, refuse(400, "This backup edit form is not recognized.", "The artifact was not changed.", "Open the backup workspace again and use its Validate, Review changes or Save into backup button")
	}
	if review != ReviewToken(base, string(content)) {
		return result, refuse(409,
			"The submitted text has not been reviewed in this form.", "The artifact was not changed.", "Select Review changes before saving")
	}
	saved, saveWarnings, err := prepared.serialize()
	if err != nil {
		return result, &Refusal{Status: 400, Cause: opererr.Wrap(err,
			"The configuration could not be prepared for saving.", "The artifact was not changed and your text is retained.",
			"Correct the reported problem and review the configuration again")}
	}
	if err := m.publish(ctx, w, name, saved); err != nil {
		return result, storageFailure(err)
	}
	result.Saved = true
	result.Base = Digest(saved)
	result.Status = "Configuration saved into the backup. Download the updated backup to keep it."
	if warnings != "" {
		result.Status += "\nInput validation advisories:" + warnings
	}
	result.Status += warningsText(saveWarnings)
	return result, nil
}

func (m *Owner) publish(ctx context.Context, w *workspace, name string, content []byte) error {
	return m.changeStore(ctx, w, func(target storage.Storage) error {
		_, err := storage.PublishArtifactConfig(target, name, content)
		return err
	})
}

type preparedEdit struct {
	editor   *cli.Editor
	validate func(string) error
}

func (prepared *preparedEdit) serialize() ([]byte, []string, error) {
	var content bytes.Buffer
	prepared.editor.SetStdoutSink(&content)
	warnings, err := prepared.editor.Save()
	if err != nil {
		return nil, nil, err
	}
	if content.Len() > TextBytesMax {
		return nil, nil, errors.New("the saved text, including generated hashes and schema stamp, exceeds 4 MiB; reduce the configuration or use ze config edit --backup")
	}
	if err := prepared.validate(content.String()); err != nil {
		return nil, nil, err
	}
	return content.Bytes(), warnings, nil
}

func prepare(original []byte, content, name string) (*preparedEdit, string, error) {
	validator, err := cli.NewConfigValidator()
	if err != nil {
		return nil, "", err
	}
	validate := func(candidate string) error {
		result := validator.ValidateTransition(string(original), candidate)
		if result.HasErrors() {
			messages := make([]string, 0, min(len(result.Errors), 32))
			// Refusal text is capped to 32 diagnostics and 16 KiB of displayed text.
			for _, problem := range result.Errors[:min(len(result.Errors), 32)] {
				messages = append(messages, problem.Text(nil))
			}
			return fmt.Errorf("%s", boundText(strings.Join(messages, "\n")))
		}
		return nil
	}
	if err := validate(content); err != nil {
		return nil, "", err
	}
	editor, err := cli.NewEditorFromContent(original, name)
	if err != nil {
		return nil, "", err
	}
	loaded, err := editor.ParseLoad(content, nil)
	if err != nil {
		return nil, "", errors.Join(err, editor.Close())
	}
	if err := editor.LoadReplace(nil, loaded); err != nil {
		return nil, "", errors.Join(err, editor.Close())
	}
	result := validator.ValidateTransition(string(original), content)
	warnings := make([]string, 0, min(len(result.Warnings), 32))
	for _, warning := range result.Warnings[:min(len(result.Warnings), 32)] {
		warnings = append(warnings, warning.Text(nil))
	}
	return &preparedEdit{editor: editor, validate: validate}, warningsText(warnings), nil
}

func warningsText(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}
	return "\n" + boundText(strings.Join(warnings, "\n"))
}

func boundText(content string) string {
	const max = 16 << 10
	if len(content) <= max {
		return content
	}
	return strings.ToValidUTF8(content[:max], "") + "\nAdditional diagnostics were omitted."
}

func Digest(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func ReviewToken(base, content string) string {
	return Digest([]byte(base + "\x00" + content))
}
