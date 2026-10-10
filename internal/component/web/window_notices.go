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
// SSH editor's: WindowWatch.Poll and Editor.PendingDiscardNotice produce both.
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
		n.pushDiscardNotice(user)
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

// pushDiscardNotice sends user what forced commits discarded from the user's
// changes, and marks it shown only once every stream of user took it: a
// notice a full client buffer refused is offered again at the next tick, to
// the streams that have not taken it (EventBroker.SendNotice). A notice with
// no stamp says the log could not be read, and is said at every tick.
func (n *WindowNotices) pushDiscardNotice(user string) {
	notice, through := n.mgr.pendingDiscardNotice(user)
	if notice == "" {
		return
	}
	if through == 0 {
		n.send(user, notice)
		return
	}
	banner, ok := renderNotice(user, notice)
	if !ok {
		return
	}
	if !n.broker.SendNotice(user, "config-change", banner, through) {
		return
	}
	if err := n.mgr.ackDiscardNotice(user, through); err != nil {
		var tb textbuf.Buffer
		n.send(user, tb.Str("your editor could not reload, so it may still show the discarded value: ").Err(err).String())
	}
}

// send renders line in the config-change banner, which the page's SSE client
// already swaps into the notification bar, and sends it to user alone. It
// answers whether every stream of user took it.
func (n *WindowNotices) send(user, line string) bool {
	banner, ok := renderNotice(user, line)
	if !ok {
		return false
	}
	return n.broker.SendTo(user, "config-change", banner)
}

// renderNotice renders line in the config-change banner; a failed render is
// logged and answers false.
func renderNotice(user, line string) (string, bool) {
	var buf bytes.Buffer
	data := notificationBannerData{Reason: line, RefreshURL: showPathPrefix}
	if err := notificationBanner(data).Render(context.Background(), &buf); err != nil {
		serverLogger.Warn("window notice render failed", "user", user, "error", err)
		return "", false
	}
	return buf.String(), true
}

// pollWindow is user's WindowWatch.Poll: the web keeps one watch per user,
// because the web user has no session of its own to keep it in. It says
// nothing while a commit of user's is in flight (beginWindowCommit).
func (m *EditorManager) pollWindow(window *confirm.Window, user string) (cli.WindowNews, bool) {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()
	// The guard: user's own commit is closing or opening the window, and its
	// answer updates the watch. Reading the window now would report the
	// user's own accept or abort as another session's (the SSH editor skips
	// the same poll while dispatchQueue.busy).
	if m.commitsInFlight[user] > 0 {
		return cli.WindowNews{}, false
	}
	watch := m.windowWatches[user]
	if watch == nil {
		watch = &cli.WindowWatch{}
		m.windowWatches[user] = watch
	}
	return watch.Poll(window, user)
}

// beginWindowCommit marks a window commit of user in flight, from before it
// reaches the window until its answer has updated user's watch. The caller
// MUST call endWindowCommit after.
func (m *EditorManager) beginWindowCommit(user string) {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()
	m.commitsInFlight[user]++
}

// endWindowCommit ends what beginWindowCommit began. It MUST be called once
// for each beginWindowCommit.
func (m *EditorManager) endWindowCommit(user string) {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()
	m.commitsInFlight[user]--
	if m.commitsInFlight[user] <= 0 {
		delete(m.commitsInFlight, user)
	}
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

// pendingDiscardNotice is user's discard notice not yet shown, from the
// user's editor; a user with no editor has no changes a forced commit could
// discard.
func (m *EditorManager) pendingDiscardNotice(user string) (string, int64) {
	m.mu.RLock()
	us, ok := m.sessions[user]
	m.mu.RUnlock()
	if !ok {
		return "", 0
	}
	us.mu.Lock()
	defer us.mu.Unlock()
	return us.editor.PendingDiscardNotice()
}

// ackDiscardNotice marks user's notice shown through the stamp
// pendingDiscardNotice returned, and rebuilds the user's view.
func (m *EditorManager) ackDiscardNotice(user string, through int64) error {
	m.mu.RLock()
	us, ok := m.sessions[user]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	us.mu.Lock()
	defer us.mu.Unlock()
	return us.editor.AckDiscardNotice(through)
}
