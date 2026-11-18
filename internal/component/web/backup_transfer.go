// Design: docs/architecture/web-interface.md -- HTTP streaming over shared artifact transfers.
package web

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
)

func (m *backupWorkspaces) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, backupArtifactBytesMax+(64<<10))
	reader, err := r.MultipartReader()
	if err != nil {
		backupUploadFailure(w, err)
		return
	}
	part, err := reader.NextPart()
	if err != nil {
		backupUploadFailure(w, err)
		return
	}
	if part.FormName() != "backup" {
		backupUploadFailure(w, errors.New("the upload field must be named backup"))
		return
	}
	if part.FileName() == "" {
		backupUploadFailure(w, errors.New("no backup file was selected"))
		return
	}
	received, err := m.sendChecked(r, "request config artifact upload begin name "+backupWord(filepath.Base(part.FileName())), part, func() error {
		if _, err := reader.NextPart(); !errors.Is(err, io.EOF) {
			return &backupCommandError{status: http.StatusBadRequest, failure: opererr.New("Upload exactly one backup file.",
				"The uploaded file was not retained.", "Open /backups/ and select one .zefs backup to upload again")}
		}
		return nil
	})
	if err != nil {
		backupCommandFailure(w, err)
		return
	}
	http.Redirect(w, r, backupWorkspacePath+received.Artifact+"/", http.StatusSeeOther)
}

func backupUploadFailure(w http.ResponseWriter, err error) {
	backupRefuse(w, http.StatusBadRequest, "The backup upload could not be accepted: "+err.Error(),
		"The uploaded file was not retained and no live configuration was changed.",
		"Select one .zefs backup in the Backup file field and upload it again")
}

func (m *backupWorkspaces) download(w http.ResponseWriter, r *http.Request, id string) {
	transfer, err := m.command(r.Context(), r, "request config artifact download begin artifact "+backupWord(id))
	if err != nil {
		backupCommandFailure(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="updated-backup.zefs"`)
	// Preserve the admitted snapshot's size so a failed later native read
	// cannot complete a truncated HTTP download successfully.
	w.Header().Set("Content-Length", strconv.FormatInt(transfer.Total, 10))
	if err := m.read(r, transfer.Transfer, w); err != nil {
		reportBackupStorageFailure()
	}
}

// Keep this boundary independent of any client-supplied path.
func backupWorkspaceURL(id string) string { return backupWorkspacePath + strings.Trim(id, "/") + "/" }
