// Design: docs/architecture/web-interface.md -- independent backup editing.
// Related: register_backup_workspace.go -- production subtree registration.
package web

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	backupWorkspacePath            = "/backups/"
	backupArtifactBytesMax   int64 = 64 << 20
	backupWorkspaceBytesMax  int64 = 256 << 20
	backupTextBytesMax             = 4 << 20
	backupWorkspaceCountMax        = 32
	backupWorkspaceUserMax         = 8
	backupWorkspaceConfigMax       = 128
	backupWorkspaceIdle            = time.Hour
)

// backupWorkspaces owns only private uploaded artifacts. Its single cleanup
// worker starts on the first accepted upload; Close MUST stop and join it.
// Requests hold mu through storage and response I/O, so removal cannot race an
// edit or a binary download. Safe concurrently; MUST NOT be copied after use.
type backupWorkspaces struct {
	mu            sync.Mutex
	items         map[string]*backupWorkspace
	root          string
	parentTemp    string
	bytes         int64
	closed        bool
	authorizer    aaa.Authorizer
	now           func() time.Time
	stop          chan struct{}
	done          chan struct{}
	worker        sync.WaitGroup
	workerStarted bool
	closeError    error
	removeAll     func(string) error
	cleanupDebt   string
	debtBytes     int64
}

// backupWorkspace is admitted only after the complete artifact has passed its
// integrity check. All access is under its owner's mutex; zero is not valid.
type backupWorkspace struct {
	id      string
	user    string
	label   string
	path    string
	bytes   int64
	configs []string
	used    time.Time
}

// HandleBackupWorkspace builds one authenticated backup subtree. Its caller
// MUST install the authentication wrapper named by the route registration.
// Mutations also pass through the shared same-origin guard. The handler never
// obtains the live editor manager or its publication callbacks.
func HandleBackupWorkspace(deps RouteDeps) http.Handler {
	return &backupWorkspaces{items: make(map[string]*backupWorkspace),
		authorizer: deps.Authorizer, now: time.Now,
		removeAll: os.RemoveAll,
		stop:      make(chan struct{}), done: make(chan struct{})}
}

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
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		backupRefuse(w, http.StatusGone, "The backup workspace service has closed.",
			"No further artifact operation was admitted.", "Reconnect to /backups/ after the web service is available")
		return
	}
	if err := m.expire(m.now()); err != nil {
		reportBackupStorageFailure()
	}
	if r.Method != http.MethodGet {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "GET, POST")
			backupRefuse(w, http.StatusMethodNotAllowed, "This backup action does not accept that request method.",
				"The artifact was not changed.", "Open /backups/ and use its form controls")
			return
		}
	}
	tail := strings.TrimPrefix(r.URL.Path, backupWorkspacePath)
	if tail == "" {
		if r.Method == http.MethodPost {
			m.upload(w, r)
			return
		}
		m.page(w, backupPageData{}, http.StatusOK)
		return
	}
	parts := strings.Split(strings.TrimSuffix(tail, "/"), "/")
	workspace, ok := m.items[parts[0]]
	if !ok {
		backupMissing(w)
		return
	}
	if workspace.user != GetUsernameFromRequest(r) {
		backupMissing(w)
		return
	}
	workspace.used = m.now()
	if len(parts) == 2 {
		if parts[1] == "download" {
			if r.Method == http.MethodGet {
				m.download(w, r, workspace)
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
	m.edit(w, r, workspace)
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

func (m *backupWorkspaces) ensureRoot() error {
	if m.root != "" {
		return nil
	}
	root, err := os.MkdirTemp(m.parentTemp, "ze-backup-workspaces-")
	if err != nil {
		return err
	}
	m.root = root
	return nil
}

func backupWorkspaceID() (string, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(random[:]), nil
}

func (m *backupWorkspaces) startWorker() {
	if m.workerStarted {
		return
	}
	m.workerStarted = true
	m.worker.Add(1)
	go m.cleanupWorker()
}

func (m *backupWorkspaces) cleanupWorker() {
	defer m.worker.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	// One long-lived worker per owner; the stop channel bounds its lifetime.
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.mu.Lock()
			err := errors.Join(m.expire(m.now()), m.retryCleanupDebt())
			m.mu.Unlock()
			if err != nil {
				reportBackupStorageFailure()
			}
		}
	}
}

func (m *backupWorkspaces) expire(now time.Time) error {
	var failures []error
	// The admission cap bounds this walk to backupWorkspaceCountMax entries.
	for id, workspace := range m.items {
		if now.Sub(workspace.used) < backupWorkspaceIdle {
			continue
		}
		if err := m.removeAll(filepath.Dir(workspace.path)); err != nil {
			failures = append(failures, err)
			continue
		}
		delete(m.items, id)
		m.bytes -= workspace.bytes
	}
	return errors.Join(failures...)
}

// retryCleanupDebt retries the single failed-upload directory. New uploads are
// sealed until it is removed; its conservative byte charge also bounds saves.
// Caller MUST hold mu, as for artifact admission and publication.
func (m *backupWorkspaces) retryCleanupDebt() error {
	if m.cleanupDebt == "" {
		return nil
	}
	if err := m.removeAll(m.cleanupDebt); err != nil {
		return err
	}
	m.cleanupDebt = ""
	m.debtBytes = 0
	return nil
}

// Close ends admission, joins the cleanup worker and removes this owner's
// private uploads. The server MUST join active requests before calling Close;
// direct users MUST call Close when finished. Safe for concurrent callers.
func (m *backupWorkspaces) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		<-m.done
		return m.closeError
	}
	m.closed = true
	close(m.stop)
	m.mu.Unlock()
	m.worker.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root != "" {
		if err := m.removeAll(m.root); err != nil {
			m.closeError = opererr.Wrap(err, "Private backup workspace cleanup did not finish.",
				"Some private uploaded or working-copy files may remain. No live configuration was changed.",
				"Run ze show doctor and repair temporary-directory access before removing the reported private workspace files")
			reportBackupStorageFailure()
		}
	}
	m.items = nil
	m.bytes = 0
	close(m.done)
	return m.closeError
}
