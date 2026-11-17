// Design: docs/architecture/web-interface.md -- offline configuration review and save.
// Related: ../cli/editor.go -- shared parsing, masked diff and save serialization.
package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/zefs"
)

type backupPageData struct {
	Path     string
	Label    string
	Names    []string
	Selected string
	Content  string
	Base     string
	Review   string
	Diff     string
	Status   string
	Error    string
	CanSave  bool
}

// backupPrepared carries one parsed and validated edit. Only backupPrepare
// constructs it. serialize revalidates the actual bytes after shared hashing
// and schema stamping; Save's stdout branch does not use a precommit hook.
type backupPrepared struct {
	editor   *cli.Editor
	validate func(string) error
}

func (prepared *backupPrepared) serialize() ([]byte, []string, error) {
	var content bytes.Buffer
	prepared.editor.SetStdoutSink(&content)
	warnings, err := prepared.editor.Save()
	if err != nil {
		return nil, nil, err
	}
	if content.Len() > backupTextBytesMax {
		return nil, nil, errors.New("the saved text, including generated hashes and schema stamp, exceeds 4 MiB; reduce the configuration or use ze config edit --backup")
	}
	if err := prepared.validate(content.String()); err != nil {
		return nil, nil, err
	}
	return content.Bytes(), warnings, nil
}

func (m *backupWorkspaces) page(w http.ResponseWriter, data backupPageData, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := writeComponent(w, "backup workspace", pageBackupWorkspace(data)); err != nil {
		reportBackupStorageFailure()
	}
}

func (m *backupWorkspaces) edit(w http.ResponseWriter, r *http.Request, workspace *backupWorkspace) {
	selected := r.URL.Query().Get("config")
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, backupTextBytesMax*3+(64<<10))
		if err := r.ParseForm(); err != nil {
			backupRefuse(w, http.StatusBadRequest, "The backup edit form could not be read.",
				"The submitted form is malformed or exceeds its size allowance; the artifact was not changed.", "Open the backup workspace again and submit at most 4 MiB of configuration text")
			return
		}
		// The request-body bound limits input; five fields are the whole form.
		if len(r.PostForm) > 5 {
			backupFormRefusal(w)
			return
		}
		for _, values := range r.PostForm {
			if len(values) != 1 {
				backupFormRefusal(w)
				return
			}
		}
		if r.PostForm.Get("action") == "close" {
			m.closeWorkspace(w, r, workspace)
			return
		}
		selected = r.PostForm.Get("config")
	}
	if selected == "" {
		selected = workspace.configs[0]
	}
	if !slices.Contains(workspace.configs, selected) {
		backupRefuse(w, http.StatusNotFound, "This configuration is not in the uploaded backup.",
			"The artifact was not changed.", "Choose a configuration from the backup workspace's Configuration list")
		return
	}
	original, err := backupReadConfig(workspace, selected)
	if err != nil {
		backupStorageFailure(w, err)
		return
	}
	if len(original) > backupTextBytesMax {
		backupRefuse(w, http.StatusRequestEntityTooLarge, "This configuration exceeds the browser editing limit.",
			"The artifact remains intact, but the editor accepts at most 4 MiB of configuration text.", "Use ze config edit --backup with this artifact to edit the larger configuration")
		return
	}
	data := backupPageData{Path: backupWorkspaceURL(workspace), Label: workspace.label,
		Names: workspace.configs, Selected: selected, Content: string(original), Base: backupDigest(original)}
	if r.Method == http.MethodGet {
		m.page(w, data, http.StatusOK)
		return
	}
	data.Content = r.PostForm.Get("content")
	if len(data.Content) > backupTextBytesMax {
		backupRefuse(w, http.StatusRequestEntityTooLarge, "The submitted configuration exceeds 4 MiB.",
			"The artifact was not changed.", "Reduce the configuration text or edit the artifact with ze config edit --backup")
		return
	}
	if !utf8.ValidString(data.Content) {
		data.Error = "The configuration is not valid UTF-8. The artifact was not changed. Correct the text and validate it again."
		m.page(w, data, http.StatusBadRequest)
		return
	}
	if r.PostForm.Get("base") != data.Base {
		data.Error = "This configuration changed after the form opened. Your submitted text is retained. Review it against the current saved configuration before saving again."
		m.page(w, data, http.StatusConflict)
		return
	}
	prepared, warnings, err := backupPrepare(original, data.Content, selected)
	if err != nil {
		data.Error = opererr.Wrap(err, "The configuration could not be accepted.",
			"The artifact was not changed and your submitted text is retained.", "Correct the reported configuration problem and select Validate again").Error()
		m.page(w, data, http.StatusBadRequest)
		return
	}
	defer func() {
		if err := prepared.editor.Close(); err != nil {
			reportBackupStorageFailure()
		}
	}()
	action := r.PostForm.Get("action")
	switch action {
	case "validate":
		data.Status = "Offline configuration validation passed. Target-host and runtime acceptance were not tested." + warnings
	case "review":
		data.Diff = prepared.editor.Diff()
		if data.Diff == "" {
			data.Diff = "No configuration changes."
		}
		data.Review = backupReviewToken(data.Base, data.Content)
		data.CanSave = true
		data.Status = "Review the changes before saving them into the backup." + warnings
	case verbSave:
		if r.PostForm.Get("review") != backupReviewToken(data.Base, data.Content) {
			data.Error = "The submitted text has not been reviewed in this form. The artifact was not changed. Select Review changes before saving."
			m.page(w, data, http.StatusConflict)
			return
		}
		content, saveWarnings, saveErr := prepared.serialize()
		if saveErr != nil {
			data.Error = opererr.Wrap(saveErr, "The configuration could not be prepared for saving.",
				"The artifact was not changed and your text is retained.", "Correct the reported problem and review the configuration again").Error()
			m.page(w, data, http.StatusBadRequest)
			return
		}
		if err := m.publish(workspace, selected, content); err != nil {
			data.Error = opererr.Wrap(err, "The configuration could not be saved into this backup.",
				"The previous artifact remains available and your text is retained.", "Run ze show doctor, then review the configuration and retry the save").Error()
			reportBackupStorageFailure()
			m.page(w, data, http.StatusInternalServerError)
			return
		}
		data.Content = string(content)
		data.Base = backupDigest(content)
		data.Status = "Configuration saved into the backup. Download the updated backup to keep it."
		if warnings != "" {
			data.Status += "\nInput validation advisories:" + warnings
		}
		data.Status += backupWarnings(saveWarnings)
	default:
		backupFormRefusal(w)
		return
	}
	m.page(w, data, http.StatusOK)
}

func backupFormRefusal(w http.ResponseWriter) {
	backupRefuse(w, http.StatusBadRequest, "This backup edit form is not recognized.",
		"The artifact was not changed.", "Open the backup workspace again and use its Validate, Review changes or Save into backup button")
}

func backupReadConfig(workspace *backupWorkspace, name string) ([]byte, error) {
	artifact, err := storage.OpenBlob(workspace.path, false)
	if err != nil {
		return nil, err
	}
	content, readErr := storage.ReadActiveConfig(artifact, name)
	return content, errors.Join(readErr, artifact.Close())
}

func backupPrepare(original []byte, content, name string) (*backupPrepared, string, error) {
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
			return fmt.Errorf("%s", backupBoundText(strings.Join(messages, "\n")))
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
	return &backupPrepared{editor: editor, validate: validate}, backupWarnings(warnings), nil
}

func backupWarnings(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}
	return "\n" + backupBoundText(strings.Join(warnings, "\n"))
}

func backupBoundText(content string) string {
	const max = 16 << 10
	if len(content) <= max {
		return content
	}
	return strings.ToValidUTF8(content[:max], "") + "\nAdditional diagnostics were omitted."
}

func backupDigest(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func backupReviewToken(base, content string) string {
	return backupDigest([]byte(base + "\x00" + content))
}

func (m *backupWorkspaces) publish(workspace *backupWorkspace, name string, content []byte) error {
	path := filepath.Join(filepath.Dir(workspace.path), "save.zefs")
	if err := m.export(workspace, path); err != nil {
		return err
	}
	defer m.removeStage(path)
	artifact, err := storage.OpenBlob(path, true, zefs.Spare(0))
	if err != nil {
		return err
	}
	_, publishErr := storage.PublishArtifactConfig(artifact, name, content)
	if err := errors.Join(publishErr, artifact.Close()); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > backupArtifactBytesMax {
		return errors.New("the saved artifact would exceed 64 MiB; reduce the configuration or use ze config edit --backup")
	}
	if m.bytes-workspace.bytes+info.Size()+m.debtBytes > backupWorkspaceBytesMax {
		return errors.New("the saved artifact would exceed the shared 256 MiB allowance; close unused workspaces and retry")
	}
	if err := os.Rename(path, workspace.path); err != nil {
		return err
	}
	m.bytes += info.Size() - workspace.bytes
	workspace.bytes = info.Size()
	return nil
}

func (m *backupWorkspaces) closeWorkspace(w http.ResponseWriter, r *http.Request, workspace *backupWorkspace) {
	if err := m.removeAll(filepath.Dir(workspace.path)); err != nil {
		backupStorageFailure(w, err)
		return
	}
	delete(m.items, workspace.id)
	m.bytes -= workspace.bytes
	http.Redirect(w, r, backupWorkspacePath, http.StatusSeeOther)
}
