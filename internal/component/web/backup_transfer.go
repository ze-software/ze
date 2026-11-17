// Design: docs/architecture/web-interface.md -- complete private backup transfer.
// Related: backup_workspace.go -- owner serialization and cleanup.
package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/zefs"
)

// backupReceived is an integrity-checked upload before account and URL
// admission. Only upload converts it into an addressable backupWorkspace.
// Its zero value is invalid; receive returns it only after validation.
type backupReceived struct {
	path    string
	label   string
	bytes   int64
	configs []string
}

func (m *backupWorkspaces) upload(w http.ResponseWriter, r *http.Request) {
	if err := m.retryCleanupDebt(); err != nil {
		reportBackupStorageFailure()
		http.Error(w, opererr.Wrap(err, "New backup uploads are waiting for private-file cleanup.",
			"An earlier partial upload could not be removed. Existing workspaces remain available.",
			"Run ze show doctor, repair temporary-directory access, then retry the upload").Error(), http.StatusServiceUnavailable)
		return
	}
	if len(m.items) >= backupWorkspaceCountMax {
		backupCapacity(w)
		return
	}
	user := GetUsernameFromRequest(r)
	owned := 0
	// Admission bounds the population to backupWorkspaceCountMax.
	for _, workspace := range m.items {
		if workspace.user == user {
			owned++
		}
	}
	if owned >= backupWorkspaceUserMax {
		backupCapacity(w)
		return
	}
	remaining := backupWorkspaceBytesMax - m.bytes - m.debtBytes
	if remaining <= 0 {
		backupCapacity(w)
		return
	}
	if err := m.ensureRoot(); err != nil {
		backupStorageFailure(w, err)
		return
	}
	id, err := backupWorkspaceID()
	if err != nil {
		backupStorageFailure(w, err)
		return
	}
	directory := filepath.Join(m.root, id)
	if err := os.Mkdir(directory, 0o700); err != nil {
		backupStorageFailure(w, err)
		return
	}
	received, err := m.receive(r, directory, min(backupArtifactBytesMax, remaining))
	if err != nil {
		if removeErr := m.removeAll(directory); removeErr != nil {
			m.cleanupDebt = directory
			// receive writes at most the admitted allowance plus its one-byte
			// overflow probe. Charge that entire bound while cleanup is owed.
			m.debtBytes = min(backupArtifactBytesMax, remaining) + 1
			backupStorageFailure(w, errors.Join(err, removeErr))
			return
		}
		if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
			backupRefuse(w, http.StatusRequestEntityTooLarge, "This backup exceeds the workspace upload limit.",
				"An artifact may be at most 64 MiB and all workspaces share a 256 MiB allowance.", "Close unused workspaces or upload a smaller backup")
			return
		}
		if refusal, refused := errors.AsType[*opererr.Error](err); refused {
			http.Error(w, refusal.Error(), http.StatusBadRequest)
			return
		}
		backupStorageFailure(w, err)
		return
	}
	workspace := &backupWorkspace{id: id, user: user, used: m.now(),
		path: received.path, label: received.label, bytes: received.bytes, configs: received.configs}
	m.items[id] = workspace
	m.bytes += workspace.bytes
	m.startWorker()
	http.Redirect(w, r, backupWorkspaceURL(workspace), http.StatusSeeOther)
}

func backupCapacity(w http.ResponseWriter) {
	backupRefuse(w, http.StatusServiceUnavailable, "The backup workspace allowance is in use.",
		"A server permits 32 workspaces, eight per account, with 256 MiB of retained artifacts.", "Close unused backup workspaces and upload the backup again")
}

func (m *backupWorkspaces) receive(r *http.Request, directory string, limit int64) (received *backupReceived, err error) {
	r.Body = http.MaxBytesReader(nil, r.Body, backupArtifactBytesMax+(64<<10))
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, backupUploadRefusal(err)
	}
	part, err := reader.NextPart()
	if err != nil {
		return nil, backupUploadRefusal(err)
	}
	if part.FormName() != "backup" {
		return nil, backupUploadRefusal(errors.New("the upload field must be named backup"))
	}
	if part.FileName() == "" {
		return nil, backupUploadRefusal(errors.New("no backup file was selected"))
	}
	label := filepath.Base(part.FileName())
	if len(label) > 128 {
		return nil, backupUploadRefusal(errors.New("the displayed backup filename exceeds 128 bytes"))
	}
	if strings.ContainsAny(label, "\r\n\x00") {
		return nil, backupUploadRefusal(errors.New("the backup filename contains control characters"))
	}
	path := filepath.Join(directory, "backup.zefs")
	// Fixed basenames stay beneath the owned directory descriptor. This request
	// MUST close it on every return, after closing any file opened beneath it.
	files, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, files.Close()) }()
	file, err := files.OpenFile("backup.zefs", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	size, copyErr := io.Copy(file, io.LimitReader(part, limit+1))
	closeErr := file.Close()
	if copyErr != nil {
		return nil, errors.Join(copyErr, closeErr)
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if size > limit {
		return nil, &http.MaxBytesError{Limit: limit}
	}
	if size == 0 {
		return nil, backupUploadRefusal(errors.New("the selected backup is empty"))
	}
	if _, err := reader.NextPart(); !errors.Is(err, io.EOF) {
		return nil, backupUploadRefusal(errors.New("upload exactly one backup file"))
	}
	if err := storage.CheckArtifact(path); err != nil {
		return nil, opererr.Wrap(err, "The selected file is not an intact Ze backup.",
			"The artifact was not opened for editing.", "Select an intact .zefs backup and upload it again")
	}
	artifact, err := storage.OpenBlob(path, false)
	if err != nil {
		return nil, err
	}
	names, namesErr := storage.ArtifactConfigNames(artifact)
	if err := errors.Join(namesErr, artifact.Close()); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, backupUploadRefusal(errors.New("the artifact contains no committed configuration"))
	}
	if len(names) > backupWorkspaceConfigMax {
		return nil, backupUploadRefusal(fmt.Errorf("the artifact contains more than %d configurations", backupWorkspaceConfigMax))
	}
	// Both the artifact-size and configuration-count bounds limit this walk.
	for _, name := range names {
		if len(name) > 256 {
			return nil, backupUploadRefusal(errors.New("a configuration name exceeds 256 bytes"))
		}
	}
	return &backupReceived{path: path, label: label, bytes: size, configs: names}, nil
}

func backupUploadRefusal(err error) error {
	return opererr.Wrap(err, "The backup upload could not be accepted.",
		"The uploaded file was not retained and no live configuration was changed.", "Select one .zefs backup in the Backup file field and upload it again")
}

func backupWorkspaceURL(workspace *backupWorkspace) string {
	return backupWorkspacePath + workspace.id + "/"
}

func (m *backupWorkspaces) download(w http.ResponseWriter, r *http.Request, workspace *backupWorkspace) {
	path := filepath.Join(filepath.Dir(workspace.path), "download.zefs")
	if err := m.export(workspace, path); err != nil {
		backupStorageFailure(w, err)
		return
	}
	defer m.removeStage(path)
	// This request MUST close the owned directory descriptor after its download
	// handle closes, before removeStage can remove the complete snapshot.
	files, err := os.OpenRoot(filepath.Dir(workspace.path))
	if err != nil {
		backupStorageFailure(w, err)
		return
	}
	defer func() {
		if err := files.Close(); err != nil {
			reportBackupStorageFailure()
		}
	}()
	file, err := files.Open("download.zefs")
	if err != nil {
		backupStorageFailure(w, err)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			reportBackupStorageFailure()
		}
	}()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="updated-backup.zefs"`)
	http.ServeContent(w, r, "updated-backup.zefs", time.Time{}, file)
}

func (m *backupWorkspaces) export(workspace *backupWorkspace, path string) (err error) {
	if err := m.retryCleanupDebt(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			m.removeStage(path)
		}
	}()
	if err := os.Remove(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	artifact, err := storage.OpenBlob(workspace.path, true)
	if err != nil {
		return err
	}
	_, copyErr := storage.Backup(artifact, path, false, zefs.Spare(0))
	return errors.Join(copyErr, artifact.Close())
}

// removeStage keeps a failed export/save snapshot in the same single cleanup
// debt slot as a refused upload. New exports and uploads first clear that
// slot, so repeated failures cannot accumulate one snapshot per workspace.
// Caller MUST hold the owner mutex and finish all handles using path first.
func (m *backupWorkspaces) removeStage(path string) {
	if err := m.removeAll(path); err != nil {
		m.cleanupDebt = path
		// The input artifact is at most 64 MiB and published text is at most
		// 4 MiB. A conservative two-artifact charge covers an unreadable stat.
		m.debtBytes = 2 * backupArtifactBytesMax
		if info, statErr := os.Stat(path); statErr == nil {
			m.debtBytes = info.Size()
		}
		reportBackupStorageFailure()
	}
}
