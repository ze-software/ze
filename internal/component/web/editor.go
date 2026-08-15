// Design: docs/architecture/web-interface.md -- Per-user editor management
// Related: handler_config.go -- Config view and edit handlers
// Related: cli.go -- CLI bar command dispatch using Editor
// Related: auth.go -- Session authentication

package web

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// userSession holds the per-user editor state for web-based config editing.
// Each authenticated user gets an independent Editor instance with its own
// working tree and change tracking.
type userSession struct {
	editor       contract.Editor
	mu           sync.Mutex // Serializes Editor method calls for this user.
	lastActivity time.Time
}

// EditorManager manages per-user Editor instances for the web interface.
// Each user gets an independent Editor with its own working tree and session.
// The manager handles creation, eviction of idle sessions, and serialized
// access to each user's Editor.
//
// NOT safe for use without initialization via NewEditorManager.
type EditorManager struct {
	mu                 sync.RWMutex
	sessions           map[string]*userSession // Keyed by username.
	store              storage.Storage
	configPath         string
	editorFactory      contract.EditorFactory
	editSessionFactory contract.EditSessionFactory
	schema             *config.Schema
	maxSessions        int
	idleTimeout        time.Duration
	commitHook         func() error
}

// NewEditorManager creates an EditorManager for the given storage backend and config path.
// Default limits: 50 concurrent sessions, 1 hour idle timeout.
func NewEditorManager(store storage.Storage, configPath string, schema *config.Schema, editorFactory contract.EditorFactory, editSessionFactory contract.EditSessionFactory) *EditorManager {
	return &EditorManager{
		sessions:           make(map[string]*userSession),
		store:              store,
		configPath:         configPath,
		schema:             schema,
		maxSessions:        50,
		editorFactory:      editorFactory,
		editSessionFactory: editSessionFactory,
		idleTimeout:        time.Hour,
	}
}

// SetCommitHook sets a hook called after a successful web commit. The hook
// typically triggers a config reload so changes take effect without SIGHUP.
func (m *EditorManager) SetCommitHook(hook func() error) {
	m.mu.Lock()
	m.commitHook = hook
	m.mu.Unlock()
}

// GetOrCreate returns the existing userSession for the given username, or creates
// a new one backed by a fresh Editor and EditSession. When the session count exceeds
// maxSessions, idle sessions older than idleTimeout are evicted.
func (m *EditorManager) GetOrCreate(username string) (*userSession, error) {
	// Fast path: session already exists. Uses write lock because
	// lastActivity update is a write that races under RLock.
	m.mu.Lock()
	if us, ok := m.sessions[username]; ok {
		us.lastActivity = time.Now()
		m.mu.Unlock()
		return us, nil
	}

	// Slow path: create new session (already holding write lock).
	defer m.mu.Unlock()

	// Evict idle sessions if over capacity.
	if len(m.sessions) >= m.maxSessions {
		m.evictInactive()
	}

	// Recheck after eviction: still at capacity means no idle sessions were freed.
	if len(m.sessions) >= m.maxSessions {
		return nil, fmt.Errorf("maximum concurrent editor sessions reached (%d)", m.maxSessions)
	}

	ed, err := m.editorFactory(m.store, m.configPath)
	if err != nil {
		return nil, fmt.Errorf("editor create for %s: %w", username, err)
	}

	session := m.editSessionFactory(username, "web")
	ed.SetSession(session)

	us := &userSession{
		editor:       ed,
		lastActivity: time.Now(),
	}
	m.sessions[username] = us

	return us, nil
}

// CreateEntry creates an empty list entry at the given path in the user's working tree.
func (m *EditorManager) CreateEntry(username string, path []string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.CreateEntry(path)
}

// SetValue sets a leaf value at the given path in the user's working tree.
func (m *EditorManager) SetValue(username string, path []string, key, value string) error {
	// Never write the display placeholder back onto a secret leaf: the web form
	// prefills one with SecretDataPlaceholder (secret.go), so submitting the
	// field unchanged must be a no-op, not a clobber of the stored value. A new
	// bcrypt password is set through the plaintext-<name> sibling instead. The
	// commit-time guard (config.RejectMaskedSecretLeaves) is the backstop, and
	// it covers every secret leaf this one does.
	if value == config.SecretDataPlaceholder && m.schema != nil {
		if config.LeafHoldsSecret(findLeafNode(m.schema, path, key)) {
			return nil
		}
	}

	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.SetValue(path, key, value)
}

// DeleteValue removes a leaf value at the given path in the user's working tree.
func (m *EditorManager) DeleteValue(username string, path []string, key string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.DeleteValue(path, key)
}

// DeleteByPath removes whatever the schema says lives at path+leaf in the
// user's working tree: a leaf, a leaf-list member, a container, a whole list, or
// a single list entry. Every web surface that deletes an operator-named node
// (the list-table delete button, the CLI bar, the terminal) MUST use this rather
// than DeleteValue, which only ever removes a scalar leaf and silently no-ops on
// a list entry. Errors -- including the editor's schema guard -- are returned
// unchanged; there is deliberately no fallback to DeleteValue.
func (m *EditorManager) DeleteByPath(username string, path []string, leaf string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	// Copy rather than append: path is a sub-slice of the parsed request URL,
	// and appending into its spare capacity would rewrite the caller's segments.
	fullPath := make([]string, 0, len(path)+1)
	fullPath = append(fullPath, path...)
	fullPath = append(fullPath, leaf)

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.DeleteByPath(fullPath)
}

// RenameListEntry renames a list entry key in the user's working tree.
func (m *EditorManager) RenameListEntry(username string, parentPath []string, listName, oldKey, newKey string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.RenameListEntry(parentPath, listName, oldKey, newKey)
}

// Commit applies the user's pending changes to the configuration file.
// Returns a CommitResult describing conflicts or the number of applied changes.
func (m *EditorManager) Commit(username string) (*contract.CommitResult, error) {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return nil, err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	m.mu.RLock()
	hook := m.commitHook
	m.mu.RUnlock()
	if hook == nil {
		return us.editor.CommitSession()
	}
	result, content, err := us.editor.CommitSessionCandidate(time.Now())
	if err != nil || result == nil || len(result.Conflicts) > 0 || result.Applied == 0 {
		return result, err
	}
	if err := hook(); err != nil {
		_ = storage.ClearCandidate(m.store, m.configPath)
		return nil, err
	}
	us.editor.MarkCommittedContent(content)
	return result, nil
}

// committedConfig returns the current committed configuration file content.
// This is the on-disk config (the baseline the daemon runs), not any user's
// pending draft. Used by the web config-download endpoint (AC-3).
func (m *EditorManager) committedConfig() ([]byte, error) {
	data, err := m.store.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("reading committed config %s: %w", m.configPath, err)
	}
	return data, nil
}

// applyCommittedContent writes content as the committed configuration and runs
// the reload hook, replacing the whole configuration at once. It is the
// upload-endpoint counterpart of Commit (AC-4): the caller MUST validate content
// first (a full config, not a per-leaf draft). On reload-hook failure the prior
// content is restored so a rejected config never leaves the daemon running
// against config it could not load. Concurrency mirrors Commit's hook read.
func (m *EditorManager) applyCommittedContent(content string) error {
	m.mu.RLock()
	hook := m.commitHook
	m.mu.RUnlock()

	prev, prevErr := m.store.ReadFile(m.configPath)
	if err := m.store.WriteFile(m.configPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing config %s: %w", m.configPath, err)
	}
	if hook != nil {
		if err := hook(); err != nil {
			// Restore the previous content so the daemon is not left with config
			// its reload rejected. Best-effort: if the prior read failed there is
			// nothing to restore to.
			if prevErr == nil {
				_ = m.store.WriteFile(m.configPath, prev, 0o600)
			}
			return fmt.Errorf("reloading config after upload: %w", err)
		}
	}
	return nil
}

// Discard reverts the user's working tree to the original state and removes
// the session from the manager.
func (m *EditorManager) Discard(username string) error {
	m.mu.RLock()
	us, ok := m.sessions[username]
	m.mu.RUnlock()

	if !ok {
		return nil // No session to discard.
	}

	us.mu.Lock()
	err := us.editor.Discard()
	us.mu.Unlock()

	if err != nil {
		return fmt.Errorf("discard for %s: %w", username, err)
	}

	m.mu.Lock()
	if current, ok := m.sessions[username]; ok && current == us {
		delete(m.sessions, username)
	}
	m.mu.Unlock()

	return nil
}

// Diff returns a textual diff of the user's pending changes, with every secret
// value masked. It is the ONE diff the web renders: the commit page, the review
// modal, the audit detail and the terminal all read it.
// Returns an empty string if no session exists or no changes are pending.
//
// The editor's own text diff is NOT used. That one compares serialized config
// content, so it carries the stored value of every ze:sensitive leaf, and no
// leaf identity survives in it to mask by. Every editor this manager creates is
// given a session (GetOrCreate), and a session change names the leaf it edited.
func (m *EditorManager) Diff(username string) (string, error) {
	m.mu.RLock()
	us, ok := m.sessions[username]
	m.mu.RUnlock()

	if !ok {
		return "", nil
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	sid := us.editor.SessionID()
	if sid == "" {
		return "", nil
	}

	changes := us.editor.PendingChanges(sid)
	if len(changes) == 0 {
		return "", nil
	}

	var b textbuf.Buffer
	for _, change := range changes {
		previous := m.maskChangeValue(change.Path, change.Previous)
		value := m.maskChangeValue(change.Path, change.Value)

		switch change.Kind {
		case contract.PendingChangeRename:
			b.Str("~ rename ").Str(change.OldPath).Str(" to ").Str(change.NewPath).Byte('\n')
		case contract.PendingChangeDelete:
			b.Str("- ").Str(change.Path).Byte(' ')
			if change.Member != "" {
				b.Str(change.Member)
			} else {
				b.Str(previous)
			}
			b.Byte('\n')
		case contract.PendingChangeDeactivate:
			writeMemberDiffLine(&b, "~ deactivate ", change)
		case contract.PendingChangeActivate:
			writeMemberDiffLine(&b, "~ activate ", change)
		default:
			if change.Previous != "" {
				b.Str("- ").Str(change.Path).Byte(' ').Str(previous).Byte('\n')
			}
			b.Str("+ ").Str(change.Path).Byte(' ').Str(value).Byte('\n')
		}
	}
	return b.String(), nil
}

// maskChangeValue answers the value one pending change renders. The change
// names the leaf it edited, so the schema decides, exactly as it does for a
// rendered field (secret.go). Without it a commit review publishes the secret
// the operator replaced, and the new one beside it.
//
// A leaf-list member needs no mask: ValueOrArrayNode carries neither
// ze:sensitive nor ze:bcrypt, so the schema cannot mark one as a secret.
func (m *EditorManager) maskChangeValue(path, value string) string {
	if value == "" || m.schema == nil {
		return value
	}

	segments := strings.Fields(path)
	if len(segments) == 0 {
		return value
	}

	return maskSecretLeaf(findLeafNode(m.schema, segments[:len(segments)-1], segments[len(segments)-1]), value)
}

// writeMemberDiffLine writes a "<verb> <path> <member>" diff line for a
// leaf-list member deactivation or activation.
func writeMemberDiffLine(b *textbuf.Buffer, verb string, change contract.PendingChange) {
	b.Str(verb).Str(change.Path).Byte(' ').Str(change.Member).Byte('\n')
}

// pendingChangePaths returns the YANG paths of every pending change in the
// user's session. The workbench uses this to mark rows whose subtree has
// uncommitted edits. Returns nil when no session exists.
func (m *EditorManager) pendingChangePaths(username string) []string {
	m.mu.RLock()
	us, ok := m.sessions[username]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	sid := us.editor.SessionID()
	if sid == "" {
		return nil
	}

	pending := us.editor.PendingChanges(sid)
	if len(pending) == 0 {
		return nil
	}
	out := make([]string, 0, len(pending))
	for _, p := range pending {
		switch p.Kind {
		case contract.PendingChangeRename:
			// Both old and new locations carry the change.
			if p.OldPath != "" {
				out = append(out, p.OldPath)
			}
			if p.NewPath != "" {
				out = append(out, p.NewPath)
			}
		default:
			out = append(out, p.Path)
		}
	}
	return out
}

// ChangeCount returns the number of pending changes for the user's session.
// Returns 0 if no session exists.
func (m *EditorManager) ChangeCount(username string) int {
	m.mu.RLock()
	us, ok := m.sessions[username]
	m.mu.RUnlock()

	if !ok {
		return 0
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	sid := us.editor.SessionID()
	if sid == "" {
		return 0
	}

	count := len(us.editor.PendingChanges(sid))
	if count > 0 {
		return count
	}
	if us.editor.Dirty() {
		return 1
	}
	return 0
}

// Tree returns the user's working configuration tree for rendering.
// Returns nil if no session exists.
func (m *EditorManager) Tree(username string) *config.Tree {
	m.mu.RLock()
	us, ok := m.sessions[username]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	t, _ := us.editor.Tree().(*config.Tree)
	return t
}

// ContentAtPath returns the serialized config content at the given context path
// for the user's working tree, with every secret leaf masked for display.
// Returns an empty string if no session exists.
//
// This is a DISPLAY path (the web CLI-bar `show` verb) and it is not edit-authz
// gated, so no secret MUST reach it. It serializes the tree here rather than
// through the editor's DisplayContentAtPath, which masks the ze:bcrypt half
// alone and published every ze:sensitive leaf. A session with no tree to mask
// gets nothing, which is the fail-closed answer.
func (m *EditorManager) ContentAtPath(username string, path []string) string {
	m.mu.RLock()
	us, ok := m.sessions[username]
	m.mu.RUnlock()

	if !ok {
		return ""
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	tree, _ := us.editor.Tree().(*config.Tree)
	if tree == nil || m.schema == nil {
		// Fail closed. Without a tree and a schema nothing decides which leaf
		// holds a secret, so this path answers nothing. Editor.DisplayContentAtPath
		// was the old answer, and its first branch returns e.workingContent with
		// no mask at all (internal/component/cli/editor_mask.go). newEditor
		// substitutes an empty tree when the config does not parse, so only
		// another editor implementation reaches this today. The guard does not
		// rest on that.
		serverLogger.Warn("cli show: the session has no tree to mask, answering nothing",
			"user", username, "schema", m.schema != nil)
		return ""
	}

	// A path that resolves to nothing renders nothing, which is what the web CLI
	// terminal answers for the same input. DisplayContentAtPath rendered the
	// whole configuration there, so a mistyped path dumped every section.
	return serializeTreeAtPath(tree, m.schema, path)
}

// CopyListEntry copies a list entry in the user's working tree.
func (m *EditorManager) CopyListEntry(username string, parentPath []string, listName, srcKey, dstKey string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.CopyListEntry(parentPath, listName, srcKey, dstKey)
}

// DeactivatePath marks a node inactive in the user's working tree.
func (m *EditorManager) DeactivatePath(username string, path []string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.DeactivatePath(path)
}

// ActivatePath re-activates a node in the user's working tree.
func (m *EditorManager) ActivatePath(username string, path []string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.ActivatePath(path)
}

// SaveDraft saves the user's pending changes to the draft file.
func (m *EditorManager) SaveDraft(username string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.SaveDraft()
}

// ListBackups returns the available config backups.
func (m *EditorManager) ListBackups(username string) ([]contract.BackupInfo, error) {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return nil, err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.ListBackups()
}

// Rollback restores a backup by path.
func (m *EditorManager) Rollback(username, backupPath string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.Rollback(backupPath)
}

// DisconnectSession disconnects another editing session.
func (m *EditorManager) DisconnectSession(username, sessionID string) error {
	us, err := m.GetOrCreate(username)
	if err != nil {
		return err
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	return us.editor.DisconnectSession(sessionID)
}

// ActiveSessions returns a summary of active web editing sessions.
// Each entry is formatted as "user@web%timestamp - N pending changes".
func (m *EditorManager) ActiveSessions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, 0, len(m.sessions))
	for _, us := range m.sessions {
		us.mu.Lock()
		sid := us.editor.SessionID()
		count := 0
		if sid != "" {
			count = len(us.editor.PendingChanges(sid))
		}
		us.mu.Unlock()

		changeWord := "changes"
		if count == 1 {
			changeWord = "change"
		}
		var bLine textbuf.Buffer
		result = append(result, bLine.Reset().Str(sid).Str(" - ").Int(int64(count)).Str(" pending ").Str(changeWord).String())
	}
	return result
}

// evictInactive removes sessions with lastActivity older than idleTimeout.
// Caller MUST hold m.mu in write mode.
func (m *EditorManager) evictInactive() {
	cutoff := time.Now().Add(-m.idleTimeout)
	for name, us := range m.sessions {
		if us.lastActivity.Before(cutoff) {
			us.mu.Lock()
			if discardErr := us.editor.Discard(); discardErr != nil {
				serverLogger.Debug("evict discard failed", "user", name, "error", discardErr)
			}
			us.mu.Unlock()
			delete(m.sessions, name)
		}
	}
}
