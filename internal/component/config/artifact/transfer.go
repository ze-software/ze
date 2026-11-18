// Design: docs/architecture/web-interface.md -- complete bounded private transfers.
// Related: owner.go -- shared admission, accounting, expiry and shutdown.
package artifact

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/zefs"
)

type upload struct {
	id, user, label, path string
	bytes, total, limit   int64
	kind                  uploadKind
	used                  time.Time
}

// uploadKind is selected only by the three admission methods below. Its zero
// value is invalid; completed text and binary uploads share the source census.
type uploadKind uint8

const (
	uploadUnspecified uploadKind = iota
	uploadArtifact
	uploadText
	uploadBytes
)

func (kind uploadKind) source() bool {
	switch kind {
	case uploadArtifact:
		return false
	case uploadText, uploadBytes:
		return true
	case uploadUnspecified:
		panic("BUG: uninitialized artifact upload kind")
	default:
		panic("BUG: invalid artifact upload kind")
	}
}

func (kind uploadKind) allowance() int64 {
	switch kind {
	case uploadArtifact, uploadBytes:
		return ArtifactBytesMax
	case uploadText:
		return TextBytesMax
	case uploadUnspecified:
		panic("BUG: uninitialized artifact upload kind")
	default:
		panic("BUG: invalid artifact upload kind")
	}
}

// source is a completed immutable private snapshot; only completed transfers
// reach this map. Consumers MUST abort after their last read or edit.
type source struct {
	user, path string
	bytes      int64
	used       time.Time
}

// view is a completed immutable configuration snapshot in memory. It permits
// inspection during failed disk cleanup without allocating another disk file.
type view struct {
	user    string
	content []byte
	used    time.Time
}

type Transfer struct {
	ID       string `json:"transfer"`
	Total    int64  `json:"total"`
	Next     int64  `json:"next"`
	Complete bool   `json:"complete"`
}

// BeginBytes admits an arbitrary binary source for a private native operation.
// The caller MUST Abort the resulting transfer after consuming its bytes.
func (m *Owner) BeginBytes(ctx context.Context, user, label string, total int64) (Transfer, error) {
	return m.begin(ctx, user, label, total, uploadBytes)
}

// Bytes returns an independent value from a completed private byte source.
// The caller MUST Abort its token after the operation and retain no owner alias.
func (m *Owner) Bytes(ctx context.Context, user, id string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return nil, err
	}
	return m.readBytes(user, id)
}

// readBytes returns an owned value while the native operation retains m.mu.
// Its caller MUST have admitted the account and MUST retain the lock until its
// complete publication or snapshot has charged the resulting retained bytes.
func (m *Owner) readBytes(user, id string) ([]byte, error) {
	source, found := m.sources[id]
	if !found {
		return nil, missing()
	}
	if source.user != user {
		return nil, missing()
	}
	if source.bytes > ArtifactBytesMax {
		return nil, tooLarge()
	}
	file, err := os.Open(source.path)
	if err != nil {
		return nil, storageFailure(err)
	}
	content, readErr := io.ReadAll(io.LimitReader(file, source.bytes+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, storageFailure(err)
	}
	if int64(len(content)) != source.bytes {
		return nil, storageFailure(errors.New("the completed private byte source changed size"))
	}
	source.used = m.now()
	return content, nil
}

// Begin admits an upload without holding any caller-owned reader. total == -1
// means unknown streaming length, bounded by the same allowance; an exact total
// must match at Finish. Text transfers hold at most TextBytesMax bytes.
func (m *Owner) Begin(ctx context.Context, user, label string, total int64, text bool) (Transfer, error) {
	kind := uploadArtifact
	if text {
		kind = uploadText
	}
	return m.begin(ctx, user, label, total, kind)
}

func (m *Owner) begin(ctx context.Context, user, label string, total int64, kind uploadKind) (Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return Transfer{}, err
	}
	if err := m.capacity(user, kind.source()); err != nil {
		return Transfer{}, err
	}
	if label == "" {
		return Transfer{}, uploadRefusal(errors.New("no backup filename was supplied"))
	}
	if filepath.Base(label) != label {
		return Transfer{}, uploadRefusal(errors.New("the filename is a label without a directory"))
	}
	if len(label) > 128 {
		return Transfer{}, uploadRefusal(errors.New("the displayed backup filename exceeds 128 bytes"))
	}
	if strings.ContainsAny(label, "\r\n\x00") {
		return Transfer{}, uploadRefusal(errors.New("the backup filename contains control characters"))
	}
	limit := min(kind.allowance(), OwnerBytesMax-m.bytes-m.debtBytes)
	if total < -1 {
		return Transfer{}, uploadRefusal(errors.New("the declared length is invalid"))
	}
	if total > limit {
		return Transfer{}, tooLarge()
	}
	if err := m.ensureRoot(); err != nil {
		return Transfer{}, storageFailure(err)
	}
	id, err := identity()
	if err != nil {
		return Transfer{}, storageFailure(err)
	}
	directory := filepath.Join(m.root, id)
	if err := os.Mkdir(directory, 0o700); err != nil {
		return Transfer{}, storageFailure(err)
	}
	path := filepath.Join(directory, "backup.zefs")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Transfer{}, storageFailure(errors.Join(err, m.removeStage(directory, 0)))
	}
	if err := file.Close(); err != nil {
		return Transfer{}, storageFailure(errors.Join(err, m.removeStage(directory, 0)))
	}
	m.uploads[id] = &upload{id: id, user: user, label: label, path: path, total: total, limit: limit, kind: kind, used: m.now()}
	m.startWorker()
	return Transfer{ID: id, Total: total}, nil
}

func uploadRefusal(err error) error {
	return &Refusal{Status: 400, Cause: opererr.Wrap(err, "The backup upload could not be accepted.", "The uploaded file was not retained and no live configuration was changed.", "Select one .zefs backup in the Backup file field and upload it again")}
}

func tooLarge() error {
	return refuse(413, "This backup exceeds the workspace upload limit.",
		"An artifact may be at most 64 MiB, configuration text at most 4 MiB, and all private files share a 256 MiB allowance.", "Close unused workspaces or upload a smaller backup")
}

func (m *Owner) Append(ctx context.Context, user, id string, offset int64, content []byte) (Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return Transfer{}, err
	}
	u, ok := m.uploads[id]
	if !ok {
		return Transfer{}, missing()
	}
	if u.user != user {
		return Transfer{}, missing()
	}
	if offset != u.bytes {
		return Transfer{}, refuse(409, "The upload offset does not match its retained bytes.", "No bytes were appended.", "Retry at the last returned next offset or abort the transfer")
	}
	if len(content) > command.ContentOctetsMax {
		return Transfer{}, tooLarge()
	}
	next := u.bytes + int64(len(content))
	if next > u.limit {
		return Transfer{}, tooLarge()
	}
	if u.total >= 0 {
		if next > u.total {
			return Transfer{}, tooLarge()
		}
	}
	if m.bytes+int64(len(content))+m.debtBytes > OwnerBytesMax {
		return Transfer{}, tooLarge()
	}
	file, err := os.OpenFile(u.path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return Transfer{}, storageFailure(err)
	}
	written, writeErr := file.Write(content)
	closeErr := file.Close()
	u.bytes += int64(written)
	m.bytes += int64(written)
	u.used = m.now()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return Transfer{}, storageFailure(err)
	}
	if written != len(content) {
		return Transfer{}, storageFailure(io.ErrShortWrite)
	}
	return Transfer{ID: id, Total: u.total, Next: u.bytes}, nil
}

func (m *Owner) Finish(ctx context.Context, user, id string) (Info, Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return Info{}, Transfer{}, err
	}
	u, ok := m.uploads[id]
	if !ok {
		return Info{}, Transfer{}, missing()
	}
	if u.user != user {
		return Info{}, Transfer{}, missing()
	}
	if u.total >= 0 {
		if u.total != u.bytes {
			return Info{}, Transfer{}, uploadRefusal(errors.New("the transfer is incomplete"))
		}
	}
	if u.kind.source() {
		delete(m.uploads, id)
		m.sources[id] = &source{user: user, path: u.path, bytes: u.bytes, used: m.now()}
		return Info{}, Transfer{ID: id, Total: u.bytes, Next: u.bytes, Complete: true}, nil
	}
	names, err := inspectUpload(u.path)
	if err != nil {
		delete(m.uploads, id)
		m.bytes -= u.bytes
		if cleanupErr := m.removeStage(filepath.Dir(u.path), max(u.bytes, u.limit)); cleanupErr != nil {
			return Info{}, Transfer{}, storageFailure(errors.Join(err, cleanupErr))
		}
		return Info{}, Transfer{}, err
	}
	delete(m.uploads, id)
	w := &workspace{id: id, user: user, label: u.label, path: u.path, bytes: u.bytes, configs: names, used: m.now()}
	m.items[id] = w
	return workspaceInfo(w), Transfer{}, nil
}

func inspectUpload(path string) ([]string, error) {
	if err := storage.CheckArtifact(path); err != nil {
		return nil, uploadRefusal(err)
	}
	store, err := storage.OpenBlob(path, false)
	if err != nil {
		return nil, storageFailure(err)
	}
	names, listErr := storage.ArtifactConfigNames(store)
	if err := errors.Join(listErr, store.Close()); err != nil {
		return nil, storageFailure(err)
	}
	if len(names) == 0 {
		return nil, uploadRefusal(errors.New("the artifact contains no committed configuration"))
	}
	if len(names) > ConfigMax {
		return nil, uploadRefusal(fmt.Errorf("the artifact contains more than %d configurations", ConfigMax))
	}
	for _, name := range names {
		if len(name) > 256 {
			return nil, uploadRefusal(errors.New("a configuration name exceeds 256 bytes"))
		}
	}
	return names, nil
}

func (m *Owner) Abort(ctx context.Context, user, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return err
	}
	if v, ok := m.views[id]; ok {
		if v.user != user {
			return missing()
		}
		delete(m.views, id)
		m.bytes -= int64(len(v.content))
		return nil
	}
	if u, ok := m.uploads[id]; ok {
		if u.user != user {
			return missing()
		}
		delete(m.uploads, id)
		m.bytes -= u.bytes
		if err := m.removeStage(filepath.Dir(u.path), max(u.bytes, u.limit)); err != nil {
			return storageFailure(err)
		}
		return nil
	}
	s, ok := m.sources[id]
	if !ok {
		return missing()
	}
	if s.user != user {
		return missing()
	}
	delete(m.sources, id)
	m.bytes -= s.bytes
	if err := m.removeStage(filepath.Dir(s.path), s.bytes); err != nil {
		return storageFailure(err)
	}
	return nil
}

// Snapshot creates a complete immutable artifact or configuration source. The
// caller MUST Abort it after the final Read, including failed transport writes.
func (m *Owner) Snapshot(ctx context.Context, user, id, name string) (Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return Transfer{}, err
	}
	w, err := m.find(user, id)
	if err != nil {
		return Transfer{}, err
	}
	if name != "" {
		content, err := readConfig(w, name)
		if err != nil {
			return Transfer{}, err
		}
		if len(m.sources)+len(m.views)+len(m.uploads) >= CountMax {
			return Transfer{}, capacity()
		}
		owned := 0
		for _, s := range m.sources {
			if s.user == user {
				owned++
			}
		}
		for _, v := range m.views {
			if v.user == user {
				owned++
			}
		}
		for _, u := range m.uploads {
			if u.user == user {
				owned++
			}
		}
		if owned >= AccountMax {
			return Transfer{}, capacity()
		}
		if m.bytes+int64(len(content))+m.debtBytes > OwnerBytesMax {
			return Transfer{}, tooLarge()
		}
		transfer, err := identity()
		if err != nil {
			return Transfer{}, storageFailure(err)
		}
		m.views[transfer] = &view{user: user, content: content, used: m.now()}
		m.bytes += int64(len(content))
		return Transfer{ID: transfer, Total: int64(len(content))}, nil
	}
	if err := m.retryCleanupDebt(); err != nil {
		return Transfer{}, storageFailure(err)
	}
	if err := m.checkCapacity(user, true); err != nil {
		return Transfer{}, err
	}
	if m.bytes+w.bytes+m.debtBytes > OwnerBytesMax {
		return Transfer{}, tooLarge()
	}
	transfer, err := identity()
	if err != nil {
		return Transfer{}, storageFailure(err)
	}
	directory := filepath.Join(m.root, transfer)
	if err := os.Mkdir(directory, 0o700); err != nil {
		return Transfer{}, storageFailure(err)
	}
	path := filepath.Join(directory, "snapshot")
	err = export(w, path)
	if err != nil {
		return Transfer{}, storageFailure(errors.Join(err, m.removeStage(directory, 2*ArtifactBytesMax)))
	}
	info, err := os.Stat(path)
	if err != nil {
		return Transfer{}, storageFailure(errors.Join(err, m.removeStage(directory, 2*ArtifactBytesMax)))
	}
	if m.bytes+info.Size()+m.debtBytes > OwnerBytesMax {
		if err := m.removeStage(directory, info.Size()); err != nil {
			return Transfer{}, storageFailure(err)
		}
		return Transfer{}, tooLarge()
	}
	m.sources[transfer] = &source{user: user, path: path, bytes: info.Size(), used: m.now()}
	m.bytes += info.Size()
	return Transfer{ID: transfer, Total: info.Size()}, nil
}

func (m *Owner) Read(ctx context.Context, user, id string, offset int64) (Transfer, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return Transfer{}, nil, err
	}
	if v, ok := m.views[id]; ok {
		if v.user != user {
			return Transfer{}, nil, missing()
		}
		if offset < 0 {
			return Transfer{}, nil, uploadRefusal(errors.New("the read offset is negative"))
		}
		if offset > int64(len(v.content)) {
			return Transfer{}, nil, uploadRefusal(errors.New("the read offset exceeds the snapshot"))
		}
		next := min(offset+int64(command.ContentOctetsMax), int64(len(v.content)))
		content := slices.Clone(v.content[offset:next])
		v.used = m.now()
		return Transfer{ID: id, Total: int64(len(v.content)), Next: next, Complete: next == int64(len(v.content))}, content, nil
	}
	s, ok := m.sources[id]
	if !ok {
		return Transfer{}, nil, missing()
	}
	if s.user != user {
		return Transfer{}, nil, missing()
	}
	if offset < 0 {
		return Transfer{}, nil, uploadRefusal(errors.New("the read offset is negative"))
	}
	if offset > s.bytes {
		return Transfer{}, nil, uploadRefusal(errors.New("the read offset exceeds the snapshot"))
	}
	file, err := os.Open(s.path)
	if err != nil {
		return Transfer{}, nil, storageFailure(err)
	}
	content := make([]byte, min(int64(command.ContentOctetsMax), s.bytes-offset))
	count, readErr := file.ReadAt(content, offset)
	if count != len(content) {
		readErr = errors.Join(readErr, io.ErrUnexpectedEOF)
	}
	if err := errors.Join(readErr, file.Close()); err != nil {
		return Transfer{}, nil, storageFailure(err)
	}
	s.used = m.now()
	next := offset + int64(len(content))
	return Transfer{ID: id, Total: s.bytes, Next: next, Complete: next == s.bytes}, content, nil
}

func export(w *workspace, path string) error {
	store, err := storage.OpenBlob(w.path, true)
	if err != nil {
		return err
	}
	_, copyErr := storage.Backup(store, path, false, zefs.Spare(0))
	return errors.Join(copyErr, store.Close())
}

// Retain holds completed review output in the same bounded snapshot inventory
// as downloads. The consumer MUST Abort after its final Read.
func (m *Owner) Retain(ctx context.Context, user string, content []byte) (Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return Transfer{}, err
	}
	return m.retain(user, content)
}

// retain shares immutable output admission with owner-held native readers.
// The caller MUST hold m.mu and have admitted the authenticated account.
func (m *Owner) retain(user string, content []byte) (Transfer, error) {
	if err := m.capacity(user, true); err != nil {
		return Transfer{}, err
	}
	if len(content) > int(ArtifactBytesMax) {
		return Transfer{}, tooLarge()
	}
	if m.bytes+int64(len(content))+m.debtBytes > OwnerBytesMax {
		return Transfer{}, tooLarge()
	}
	id, err := identity()
	if err != nil {
		return Transfer{}, storageFailure(err)
	}
	if err := m.ensureRoot(); err != nil {
		return Transfer{}, storageFailure(err)
	}
	directory := filepath.Join(m.root, id)
	if err := os.Mkdir(directory, 0o700); err != nil {
		return Transfer{}, storageFailure(err)
	}
	path := filepath.Join(directory, "snapshot")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return Transfer{}, storageFailure(errors.Join(err, m.removeStage(directory, int64(len(content)))))
	}
	m.sources[id] = &source{user: user, path: path, bytes: int64(len(content)), used: m.now()}
	m.bytes += int64(len(content))
	return Transfer{ID: id, Total: int64(len(content))}, nil
}

// ExportTo copies a stable complete artifact while holding its owner lock.
// The lifecycle caller MUST own and close destination, verify its complete
// retained bytes and keep that copy outside this owner before acknowledging
// recovery. The caller MUST NOT pass a network writer: storage I/O is serialized.
func (m *Owner) ExportTo(ctx context.Context, user, id string, destination io.Writer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.admit(ctx, user); err != nil {
		return err
	}
	w, err := m.find(user, id)
	if err != nil {
		return err
	}
	file, err := os.Open(w.path)
	if err != nil {
		return storageFailure(err)
	}
	buffer := make([]byte, command.ContentOctetsMax)
	var copyErr error
	// The admitted artifact is at most 64 MiB. Check cancellation between
	// bounded disk reads; the caller closes its own private output on refusal.
	for remaining := w.bytes; remaining > 0; {
		if err := ctx.Err(); err != nil {
			copyErr = err
			break
		}
		count, err := io.ReadFull(file, buffer[:min(int64(len(buffer)), remaining)])
		if err != nil {
			copyErr = err
			break
		}
		written, err := destination.Write(buffer[:count])
		if err != nil {
			copyErr = err
			break
		}
		if written != count {
			copyErr = io.ErrShortWrite
			break
		}
		remaining -= int64(count)
	}
	return errors.Join(copyErr, file.Close())
}
