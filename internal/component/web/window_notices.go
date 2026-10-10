// Design: docs/guide/config-editor.md -- Commit Confirmed: the daemon owns the window
// Overview: editor_commit_window.go -- the web commit through the daemon window
// Related: sse.go -- EventBroker, the /events stream the notices travel on
// Related: ../cli/model_commit_window.go -- WindowWatch, the poll the SSH editor shares

package web

import (
	"bytes"
	"context"
	"time"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// windowNoticeInterval is how often WindowNotices looks: the countdown a web
// user reads is at most this stale.
const windowNoticeInterval = time.Second

// WindowNotices pushes to each web user, over the /events stream, what the
// SSH draft poll shows that user on its status line: the daemon window's
// countdown or owner, how a window the user saw ended, and what another
// user's forced commit discarded from the user's changes. The words are the
// SSH editor's: WindowWatch.Poll and Editor.TakeDiscardNotice produce both.
//
// Lifecycle: NewWindowNotices, then `go Run()`; Run returns when the broker
// closes. Safe for concurrent use with the manager it reads.
type WindowNotices struct {
	mgr    *EditorManager
	broker *EventBroker
}

// NewWindowNotices builds the notices for mgr's users, sent through broker.
// The caller MUST start Run and MUST close the broker to stop it.
func NewWindowNotices(mgr *EditorManager, broker *EventBroker) *WindowNotices {
	return &WindowNotices{mgr: mgr, broker: broker}
}

// Run pushes the notices every windowNoticeInterval until the broker closes.
// It MUST be started once, by the owner of the broker.
func (n *WindowNotices) Run() {
	ticker := time.NewTicker(windowNoticeInterval)
	defer ticker.Stop()
	// Bounded by the broker's life: Close ends it.
	for {
		select {
		case <-n.broker.Done():
			return
		case <-ticker.C:
			n.push()
		}
	}
}

// push sends each user with a connected stream what there is to say now.
func (n *WindowNotices) push() {
	window := n.mgr.daemonWindow()
	for _, user := range n.broker.Users() {
		if window != nil {
			n.pushWindow(window, user)
		}
		if notice := n.mgr.takeDiscardNotice(user); notice != "" {
			n.send(user, notice)
		}
	}
}

// pushWindow sends user the window's news; a window that ended also rebuilds
// the user's view over the configuration it left, as the SSH editor does.
func (n *WindowNotices) pushWindow(window *confirm.Window, user string) {
	news, ok := n.mgr.pollWindow(window, user)
	if !ok {
		return
	}
	line := news.Line
	if news.Ended {
		if err := n.mgr.refreshCommittedView(user); err != nil {
			var tb textbuf.Buffer
			line = tb.Str(line).Str(" (view not refreshed: ").Err(err).Byte(')').String()
		}
	}
	n.send(user, line)
}

// send renders line in the config-change banner, which the page's SSE client
// already swaps into the notification bar, and sends it to user alone.
func (n *WindowNotices) send(user, line string) {
	var buf bytes.Buffer
	data := notificationBannerData{Reason: line, RefreshURL: showPathPrefix}
	if err := notificationBanner(data).Render(context.Background(), &buf); err != nil {
		serverLogger.Warn("window notice render failed", "user", user, "error", err)
		return
	}
	n.broker.SendTo(user, "config-change", buf.String())
}

// pollWindow is user's WindowWatch.Poll: the web keeps one watch per user,
// because the web user has no session of its own to keep it in.
func (m *EditorManager) pollWindow(window *confirm.Window, user string) (cli.WindowNews, bool) {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()
	watch := m.windowWatches[user]
	if watch == nil {
		watch = &cli.WindowWatch{}
		m.windowWatches[user] = watch
	}
	return watch.Poll(window, user)
}

// setWindowWatch records what user's own commit did to the window, so the
// notices never tell user that "another session" closed a window user
// closed. A nil watch forgets the window.
func (m *EditorManager) setWindowWatch(user string, watch *cli.WindowWatch) {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()
	if watch == nil {
		delete(m.windowWatches, user)
		return
	}
	m.windowWatches[user] = watch
}

// takeDiscardNotice is user's discard notice, taken from the user's editor;
// a user with no editor has no changes a forced commit could discard.
func (m *EditorManager) takeDiscardNotice(user string) string {
	m.mu.RLock()
	us, ok := m.sessions[user]
	m.mu.RUnlock()
	if !ok {
		return ""
	}
	us.mu.Lock()
	defer us.mu.Unlock()
	return us.editor.TakeDiscardNotice()
}
