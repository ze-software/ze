// Design: docs/architecture/web-interface.md -- private artifact authority shared by command surfaces.
// Related: transfer.go -- bounded upload, text and immutable download transfers.
package artifact

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	ArtifactBytesMax int64 = 64 << 20
	OwnerBytesMax    int64 = 256 << 20
	TextBytesMax           = 4 << 20
	CountMax               = 32
	AccountMax             = 8
	ConfigMax              = 128
	Idle                   = time.Hour
	TransferIdle           = 2 * time.Minute
)

// Owner serializes private storage operations, never network I/O. New creates
// no resource; its server MUST register Close before exposing the owner. Close
// MUST seal admission, join the worker and remove owned files. Safe concurrently;
// MUST NOT be copied. Its zero value is invalid; use New.
type Owner struct {
	mu                  sync.Mutex
	items               map[string]*workspace
	uploads             map[string]*upload
	sources             map[string]*source
	views               map[string]*view
	root                string
	roots               map[string]struct{}
	parentTemp          string
	directoryConfig     string
	directoryGeneration uint64
	directoryGuard      func(string) error
	bytes               int64
	closed              bool
	now                 func() time.Time
	stop                chan struct{}
	done                chan struct{}
	worker              sync.WaitGroup
	workerStarted       bool
	closeError          error
	removeAll           func(string) error
	cleanupDebt         string
	debtBytes           int64
}

type workspace struct {
	id, user, label, path string
	bytes                 int64
	configs               []string
	used                  time.Time
}

// Info is an owned value, with no mutable alias into the owner's inventory.
type Info struct {
	ID     string   `json:"artifact"`
	Label  string   `json:"label"`
	Bytes  int64    `json:"bytes"`
	Names  []string `json:"names"`
	Digest string   `json:"digest"`
}

func New() *Owner {
	return &Owner{items: make(map[string]*workspace), uploads: make(map[string]*upload), roots: make(map[string]struct{}),
		sources: make(map[string]*source), views: make(map[string]*view), now: time.Now, removeAll: os.RemoveAll,
		stop: make(chan struct{}), done: make(chan struct{})}
}

// Refusal retains the HTTP category at the domain boundary without making HTTP
// the owner of admission or storage policy. Every surface receives the same cause.
type Refusal struct {
	Status int
	Cause  error
}

func (r *Refusal) Error() string { return r.Cause.Error() }
func (r *Refusal) Unwrap() error { return r.Cause }

func refuse(status int, problem, why, fix string) error {
	return &Refusal{Status: status, Cause: opererr.New(problem, why, fix)}
}

func storageFailure(err error) error {
	ReportStorageFailure()
	return &Refusal{Status: 500, Cause: opererr.Wrap(err,
		"The backup workspace could not complete its storage operation.",
		"No live configuration was changed. The workspace may need to be uploaded again.",
		"Run ze show doctor, then reopen /backups/ and retry your backup")}
}

func missing() error {
	return refuse(404, "This backup workspace is unavailable.",
		"It has expired, closed, or belongs to another account.", "Open /backups/ and upload your backup again")
}

func (m *Owner) admit(ctx context.Context, user string) error {
	if err := ctx.Err(); err != nil {
		return refuse(400, "The backup request was canceled.", "No further artifact operation was admitted.", "Retry the backup operation")
	}
	if user == "" {
		return refuse(401, "Sign in before opening a backup.", "No authenticated account is available for this workspace.", "Open /login and sign in, then reopen /backups/")
	}
	if m.closed {
		return refuse(410, "The backup workspace service has closed.", "No further artifact operation was admitted.", "Reconnect to /backups/ after the service is available")
	}
	if err := m.expire(m.now()); err != nil {
		ReportStorageFailure()
	}
	return nil
}

func (m *Owner) find(user, id string) (*workspace, error) {
	w, ok := m.items[id]
	if !ok {
		return nil, missing()
	}
	if w.user != user {
		return nil, missing()
	}
	w.used = m.now()
	return w, nil
}

func (m *Owner) List(ctx context.Context, user string) ([]Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return nil, err
	}
	rows := make([]Info, 0, len(m.items))
	for _, w := range m.items {
		if w.user == user {
			rows = append(rows, workspaceInfo(w))
		}
	}
	slices.SortFunc(rows, func(a, b Info) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return rows, nil
}

func workspaceInfo(w *workspace) Info {
	return Info{ID: w.id, Label: w.label, Bytes: w.bytes, Names: slices.Clone(w.configs)}
}

func (m *Owner) Inspect(ctx context.Context, user, id string) (Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return Info{}, err
	}
	w, err := m.find(user, id)
	if err != nil {
		return Info{}, err
	}
	file, err := os.Open(w.path)
	if err != nil {
		return Info{}, storageFailure(err)
	}
	hash := sha256.New()
	_, readErr := io.Copy(hash, io.LimitReader(file, ArtifactBytesMax+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return Info{}, storageFailure(err)
	}
	info := workspaceInfo(w)
	info.Digest = hex.EncodeToString(hash.Sum(nil))
	return info, nil
}

func (m *Owner) Remove(ctx context.Context, user, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return err
	}
	w, err := m.find(user, id)
	if err != nil {
		return err
	}
	if err := m.removeAll(filepath.Dir(w.path)); err != nil {
		return storageFailure(err)
	}
	delete(m.items, id)
	m.bytes -= w.bytes
	return nil
}

func (m *Owner) ensureRoot() error {
	if m.root != "" {
		return nil
	}
	parent := m.parentTemp
	configured := parent == ""
	if configured {
		var err error
		parent, err = configuredBackupParent(m.directoryConfig)
		if err != nil {
			return err
		}
	}
	if m.directoryGuard != nil {
		checked := parent
		if checked == "" {
			checked = os.TempDir()
		}
		if err := m.directoryGuard(checked); err != nil {
			return err
		}
	}
	if configured {
		if err := prepareBackupParent(parent); err != nil {
			return err
		}
	}
	root, err := os.MkdirTemp(parent, "ze-backup-workspaces-")
	if err != nil {
		return opererr.Wrap(err, "A private backup workspace could not be created.",
			"The daemon cannot write to its selected backup folder; no workspace was opened.",
			"Run ze show doctor, check the backup folder's permissions and free space, then retry")
	}
	m.parentTemp = parent
	m.root = root
	m.roots[root] = struct{}{}
	return nil
}

func identity() (string, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(random[:]), nil
}

func (m *Owner) capacity(user string, text bool) error {
	if err := m.retryCleanupDebt(); err != nil {
		ReportStorageFailure()
		return &Refusal{Status: 503, Cause: opererr.Wrap(err, "New backup uploads are waiting for private-file cleanup.",
			"An earlier partial upload could not be removed. Existing workspaces remain available.",
			"Run ze show doctor, repair temporary-directory access, then retry the upload")}
	}
	return m.checkCapacity(user, text)
}

func (m *Owner) checkCapacity(user string, text bool) error {
	owned := 0
	var count int
	if text {
		count = len(m.sources)
		for _, s := range m.sources {
			if s.user == user {
				owned++
			}
		}
		count += len(m.views)
		for _, v := range m.views {
			if v.user == user {
				owned++
			}
		}
	} else {
		count = len(m.items)
		for _, w := range m.items {
			if w.user == user {
				owned++
			}
		}
	}
	for _, u := range m.uploads {
		if u.kind.source() != text {
			continue
		}
		count++
		if u.user == user {
			owned++
		}
	}
	if count >= CountMax {
		return capacity()
	}
	if owned >= AccountMax {
		return capacity()
	}
	if m.bytes+m.debtBytes >= OwnerBytesMax {
		return capacity()
	}
	return nil
}

func capacity() error {
	return refuse(503, "The backup workspace allowance is in use.",
		"A server permits 32 workspaces, eight per account, with 256 MiB of retained artifacts.", "Close unused backup workspaces and upload the backup again")
}

func (m *Owner) startWorker() {
	if m.workerStarted {
		return
	}
	m.workerStarted = true
	m.worker.Add(1)
	go m.cleanupWorker()
}

func (m *Owner) cleanupWorker() {
	defer m.worker.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	// This single worker ends when its owner's stop channel closes.
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.mu.Lock()
			err := errors.Join(m.expire(m.now()), m.retryCleanupDebt())
			m.mu.Unlock()
			if err != nil {
				ReportStorageFailure()
			}
		}
	}
}

func (m *Owner) expire(now time.Time) error {
	var failures []error
	for id, w := range m.items {
		if now.Sub(w.used) < Idle {
			continue
		}
		if err := m.removeAll(filepath.Dir(w.path)); err != nil {
			failures = append(failures, err)
			continue
		}
		delete(m.items, id)
		m.bytes -= w.bytes
	}
	for id, u := range m.uploads {
		if now.Sub(u.used) < TransferIdle {
			continue
		}
		if err := m.removeAll(filepath.Dir(u.path)); err != nil {
			failures = append(failures, err)
			continue
		}
		delete(m.uploads, id)
		m.bytes -= u.bytes
	}
	for id, s := range m.sources {
		if now.Sub(s.used) < TransferIdle {
			continue
		}
		if err := m.removeAll(filepath.Dir(s.path)); err != nil {
			failures = append(failures, err)
			continue
		}
		delete(m.sources, id)
		m.bytes -= s.bytes
	}
	for id, v := range m.views {
		if now.Sub(v.used) < TransferIdle {
			continue
		}
		delete(m.views, id)
		m.bytes -= int64(len(v.content))
	}
	return errors.Join(errors.Join(failures...), m.pruneRetiredRoots())
}

func (m *Owner) retryCleanupDebt() error {
	if m.cleanupDebt == "" {
		return m.pruneRetiredRoots()
	}
	if err := m.removeAll(m.cleanupDebt); err != nil {
		return err
	}
	m.cleanupDebt = ""
	m.debtBytes = 0
	return m.pruneRetiredRoots()
}

// removeStage retains one conservative debt charge. Allocations MUST first call
// retryCleanupDebt, so a second failure cannot hide or overwrite the first debt.
func (m *Owner) removeStage(path string, charge int64) error {
	if err := m.removeAll(path); err != nil {
		m.cleanupDebt = path
		m.debtBytes = charge
		ReportStorageFailure()
		return err
	}
	return nil
}

// Close MUST be registered on the actual command server before the owner is
// exposed. It seals admission, serializes with admitted storage work, joins its
// worker, then removes only its own roots. Safe for concurrent callers.
func (m *Owner) Close() error {
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
	var failures []error
	if m.cleanupDebt != "" {
		if err := m.removeAll(m.cleanupDebt); err != nil {
			failures = append(failures, opererr.Wrap(err, "Private backup workspace cleanup did not finish.",
				"Some private uploaded or working-copy files may remain. No live configuration was changed.",
				"Run ze show doctor and repair backup-folder access before removing the reported private workspace files"))
			ReportStorageFailure()
		} else {
			m.cleanupDebt = ""
			m.debtBytes = 0
		}
	}
	for root := range m.roots {
		if err := m.removeAll(root); err != nil {
			failures = append(failures, opererr.Wrap(err, "Private backup workspace cleanup did not finish.",
				"Some private uploaded or working-copy files may remain. No live configuration was changed.",
				"Run ze show doctor and repair backup-folder access before removing the reported private workspace files"))
			ReportStorageFailure()
			continue
		}
		delete(m.roots, root)
	}
	m.closeError = errors.Join(failures...)
	m.items = nil
	m.uploads = nil
	m.sources = nil
	m.views = nil
	m.bytes = 0
	close(m.done)
	return m.closeError
}
