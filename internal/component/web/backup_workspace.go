// Design: docs/architecture/web-interface.md -- authenticated HTTP adapter to shared artifact commands.
package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	backupWorkspacePath          = "/backups/"
	backupArtifactBytesMax int64 = 64 << 20
	backupTextBytesMax           = 4 << 20
)

// backupWorkspaces is a rendering and streaming adapter. The daemon command
// server owns every artifact; this handler owns no private files or worker.
type backupWorkspaces struct {
	authorizer aaa.Authorizer
	dispatch   CommandDispatcher
}

func HandleBackupWorkspace(deps RouteDeps) http.Handler {
	return &backupWorkspaces{authorizer: deps.Authorizer, dispatch: deps.Dispatch}
}

// Close has no private lifetime to end: the actual command Server stops its
// shared owner after command admission and requests drain.
func (m *backupWorkspaces) Close() error { return nil }

func (m *backupWorkspaces) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if GetUsernameFromRequest(r) == "" {
		backupRefuse(w, http.StatusUnauthorized, "Sign in before opening a backup.",
			"No authenticated account is available for this workspace.", "Open /login and sign in, then reopen /backups/")
		return
	}
	if !canEdit(r, m.authorizer) {
		backupRefuse(w, http.StatusForbidden, "This account cannot open backup artifacts.",
			"Backups can contain credentials and require configuration edit access.", "Sign in with an account authorized to edit configuration")
		return
	}
	controller := http.NewResponseController(w)
	deadline := time.Now().Add(2 * time.Minute)
	if err := backupDeadline(controller.SetReadDeadline(deadline)); err != nil {
		backupStorageFailure(w, err)
		return
	}
	if err := backupDeadline(controller.SetWriteDeadline(deadline)); err != nil {
		backupStorageFailure(w, err)
		return
	}
	defer func() {
		if err := backupDeadline(controller.SetReadDeadline(time.Time{})); err != nil {
			reportBackupStorageFailure()
		}
		if err := backupDeadline(controller.SetWriteDeadline(time.Time{})); err != nil {
			reportBackupStorageFailure()
		}
	}()
	RequireSameOrigin(http.HandlerFunc(m.serve)).ServeHTTP(w, r)
}

func backupDeadline(err error) error {
	// In-memory ResponseRecorders have no socket to bound. Production's writer
	// and the shared error-fragment wrapper expose ResponseController support.
	if errors.Is(err, http.ErrNotSupported) {
		return nil
	}
	return err
}

func (m *backupWorkspaces) serve(w http.ResponseWriter, r *http.Request) {
	if m.dispatch == nil {
		backupRefuse(w, http.StatusServiceUnavailable, "The daemon artifact commands are unavailable.", "No private workspace can start without its daemon owner.", "Reconnect to the running daemon and reopen /backups/")
		return
	}
	tail := strings.TrimPrefix(r.URL.Path, backupWorkspacePath)
	if tail == "system-download" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			backupRefuse(w, http.StatusMethodNotAllowed, "Creating a system backup requires its form submission.", "No backup was created.", "Open /backups/ and select Create backup of this system")
			return
		}
		m.downloadSystem(w, r)
		return
	}
	if r.Method != http.MethodGet {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "GET, POST")
			backupRefuse(w, http.StatusMethodNotAllowed, "This backup action does not accept that request method.", "The artifact was not changed.", "Open /backups/ and use its form controls")
			return
		}
	}
	if tail == "" {
		if r.Method == http.MethodPost {
			m.upload(w, r)
			return
		}
		response, err := m.response(r.Context(), r, "show config artifact list")
		if err != nil {
			backupCommandFailure(w, err)
			return
		}
		if response.Status == plugin.StatusError {
			_, err := decodeBackupAnswer(response)
			backupCommandFailure(w, err)
			return
		}
		m.page(w, backupPageData{}, http.StatusOK)
		return
	}
	parts := strings.Split(strings.TrimSuffix(tail, "/"), "/")
	if parts[0] == "sources" {
		m.sourceRoute(w, r, parts[1:])
		return
	}
	id := parts[0]
	if len(parts) == 2 {
		if parts[1] == "download" {
			if r.Method == http.MethodGet {
				m.download(w, r, id)
				return
			}
		}
		backupMissing(w)
		return
	}
	if len(parts) != 1 {
		backupMissing(w)
		return
	}
	m.edit(w, r, id)
}

func backupRefuse(w http.ResponseWriter, status int, problem, why, fix string) {
	http.Error(w, opererr.New(problem, why, fix).Error(), status)
}

func backupMissing(w http.ResponseWriter) {
	backupRefuse(w, http.StatusNotFound, "This backup workspace is unavailable.",
		"It has expired, closed, or belongs to another account.", "Open /backups/ and upload your backup again")
}

func backupStorageFailure(w http.ResponseWriter, err error) {
	reportBackupStorageFailure()
	http.Error(w, opererr.Wrap(err, "The backup workspace could not complete its storage operation.",
		"No live configuration was changed. The workspace may need to be uploaded again.",
		"Run ze show doctor, then reopen /backups/ and retry your backup").Error(), http.StatusInternalServerError)
}
