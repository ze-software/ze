package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
)

// drainEvents returns the data of every event queued for client, in order.
func drainEvents(client *sseClient) []string {
	var events []string
	for {
		select {
		case ev := <-client.ch:
			events = append(events, ev.data)
		default:
			return events
		}
	}
}

// requireOneEvent asserts client received exactly one event holding want.
func requireOneEvent(t *testing.T, client *sseClient, want string) {
	t.Helper()
	events := drainEvents(client)
	require.Len(t, events, 1, "one event, holding %q", want)
	assert.Contains(t, events[0], want)
}

// TestWindowNoticesPushedOverSSE is owner decision (d) of
// spec-session-editor-file-mode-parity: a web user sees what an SSH user's
// status line shows, the confirm countdown, how a window ended, and that a
// forced commit discarded their change, pushed over the /events stream.
//
// GOAL: each web user gets, over SSE, the words the SSH draft poll shows that
// same user, and only that user gets them.
// METHOD: a promoting manager with a real daemon window, one SSE client per
// user, and WindowNotices.push run by hand, one pass per step.
//
// VALIDATES: the owner sees "Confirm within", another user sees whose window
// is pending; an abort the owner typed in the web terminal tells bob the
// window was closed by another session and tells alice nothing new; a forced
// commit leaves bob the discard notice the SSH editor shows, once.
// PREVENTS: the web terminal showing no countdown and keeping a stale tree
// after another user's forced commit (review round 1, Remaining rows).
func TestWindowNoticesPushedOverSSE(t *testing.T) {
	mgr, schema, _ := newWindowEditorManager(t)
	broker := NewEventBroker(0)
	t.Cleanup(broker.Close)
	alice := broker.Subscribe("alice")
	bob := broker.Subscribe("bob")
	require.NotNil(t, alice)
	require.NotNil(t, bob)
	notices := NewWindowNotices(mgr, broker)

	notices.push()
	assert.Empty(t, drainEvents(alice), "no window, nothing to say")
	assert.Empty(t, drainEvents(bob))

	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.1"))
	assert.Contains(t, webCommit(schema, mgr, "alice", "confirmed", "60"), "commit accept")

	notices.push()
	requireOneEvent(t, alice, "Confirm within")
	requireOneEvent(t, bob, "A confirmed commit by alice is pending")

	assert.Contains(t, webCommit(schema, mgr, "alice", "abort"), "rolled back")
	notices.push()
	assert.Empty(t, drainEvents(alice), "alice aborted it herself, from the web terminal")
	requireOneEvent(t, bob, contract.CommitClosedElsewhere)

	require.NoError(t, mgr.SetValue("bob", []string{"bgp"}, "router-id", "10.0.0.2"))
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.9"))
	require.Equal(t, terminalOutputCommitSuccessful, webCommit(schema, mgr, "alice", "now", "force"))

	notices.push()
	requireOneEvent(t, bob, "Your change at bgp router-id was discarded by alice")
	assert.Empty(t, drainEvents(alice))
	assert.NotContains(t, mgr.ContentAtPath("bob", []string{"bgp"}), "10.0.0.2",
		"the notice rebuilt bob's tree without the discarded value")

	notices.push()
	assert.Empty(t, drainEvents(bob), "the notice is shown once")
}

// TestEventStreamSubscribesItsUser proves a browser's /events stream is
// registered under the authenticated user, so WindowNotices can reach it.
//
// VALIDATES: ServeHTTP subscribes the request's user; Users names that user
// while the stream is open and forgets it once the stream ends.
// PREVENTS: notices sent to a user whose streams were all registered as "".
func TestEventStreamSubscribesItsUser(t *testing.T) {
	broker := NewEventBroker(0)
	t.Cleanup(broker.Close)
	ctx, cancel := context.WithCancel(withUsername(context.Background(), "carol"))
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/events", http.NoBody)
	served := make(chan struct{})
	go func() {
		defer close(served)
		broker.ServeHTTP(httptest.NewRecorder(), request)
	}()

	require.Eventually(t, func() bool { return slices.Contains(broker.Users(), "carol") },
		5*time.Second, 10*time.Millisecond, "the stream is registered under its user")
	cancel()
	<-served
	assert.Empty(t, broker.Users(), "a closed stream is forgotten")
}

// TestWindowNoticesSkipAUserWhoseCommitIsInFlight is review round 2 ISSUE 1
// of spec-session-editor-file-mode-parity: the notices tick runs on its own
// clock, so it can look between the moment a web user's own accept closes the
// window and the moment runCommit forgets the user's watch.
//
// GOAL: a user is never told "closed by another session" about a window the
// user's own commit closed.
// METHOD: alice opens a window from the web terminal; with her commit marked
// in flight, as runCommit marks it, the window is accepted under her name and
// push runs in that gap. The SSH editor guards the same poll with
// dispatchQueue.busy().
//
// VALIDATES: push says nothing to alice while her commit is in flight, and
// still tells bob the window closed.
// PREVENTS: a web user's own accept or abort reported as another session's.
func TestWindowNoticesSkipAUserWhoseCommitIsInFlight(t *testing.T) {
	mgr, schema, window := newWindowEditorManager(t)
	broker := NewEventBroker(0)
	t.Cleanup(broker.Close)
	alice := broker.Subscribe("alice")
	bob := broker.Subscribe("bob")
	notices := NewWindowNotices(mgr, broker)

	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.1"))
	assert.Contains(t, webCommit(schema, mgr, "alice", "confirmed", "60"), "commit accept")
	notices.push()
	requireOneEvent(t, alice, "Confirm within")
	requireOneEvent(t, bob, "A confirmed commit by alice is pending")

	mgr.beginWindowCommit("alice")
	require.NoError(t, window.Accept("alice"))
	notices.push()
	assert.Empty(t, drainEvents(alice), "alice's own accept is still in flight")
	requireOneEvent(t, bob, contract.CommitClosedElsewhere)
	mgr.setWindowWatch("alice", nil)
	mgr.endWindowCommit("alice")

	notices.push()
	assert.Empty(t, drainEvents(alice), "her commit forgot the window it closed")
}

// TestDiscardNoticeKeptWhileTheStreamIsFull is review round 2 ISSUE 4 of
// spec-session-editor-file-mode-parity: push took the notice before sending
// it, and a full client buffer dropped the event, so the notice was lost.
//
// GOAL: a discard notice reaches the user even when the stream was full at
// the first attempt.
// METHOD: bob's stream buffer is filled, alice forces over bob's change, push
// runs; the buffer is drained and push runs again.
//
// VALIDATES: the notice arrives at the second push, and only once.
// PREVENTS: a forced-commit notice silently lost to a slow browser.
func TestDiscardNoticeKeptWhileTheStreamIsFull(t *testing.T) {
	mgr, schema, _ := newWindowEditorManager(t)
	broker := NewEventBroker(0)
	t.Cleanup(broker.Close)
	bob := broker.Subscribe("bob")
	notices := NewWindowNotices(mgr, broker)

	require.NoError(t, mgr.SetValue("bob", []string{"bgp"}, "router-id", "10.0.0.2"))
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.9"))
	require.Equal(t, terminalOutputCommitSuccessful, webCommit(schema, mgr, "alice", "now", "force"))

	for broker.SendTo("bob", "filler", "x") {
	}
	notices.push()
	for _, event := range drainEvents(bob) {
		assert.NotContains(t, event, "discarded", "the full buffer refused the notice")
	}

	notices.push()
	requireOneEvent(t, bob, "Your change at bgp router-id was discarded by alice")
	notices.push()
	assert.Empty(t, drainEvents(bob), "the notice is shown once")
}

// TestDiscardNoticeOncePerTab is review round 3 NOTE 3 of
// spec-session-editor-file-mode-parity: with two tabs of one user and one tab
// full, the notice was resent to both at the next tick, so the tab that had
// taken it showed it twice.
//
// GOAL: each tab shows a discard notice once, whatever the other tab does.
// METHOD: bob has two streams; the second is full when alice forces; push
// runs, the full stream is drained, push runs twice more.
//
// VALIDATES: the first tab gets the notice at the first push only, the
// second at the second push only.
// PREVENTS: a tab told twice because another tab was slow.
func TestDiscardNoticeOncePerTab(t *testing.T) {
	mgr, schema, _ := newWindowEditorManager(t)
	broker := NewEventBroker(0)
	t.Cleanup(broker.Close)
	first := broker.Subscribe("bob")
	second := broker.Subscribe("bob")
	notices := NewWindowNotices(mgr, broker)

	require.NoError(t, mgr.SetValue("bob", []string{"bgp"}, "router-id", "10.0.0.2"))
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.9"))
	require.Equal(t, terminalOutputCommitSuccessful, webCommit(schema, mgr, "alice", "now", "force"))

	for broadcastEvent(second, sseEvent{eventType: "filler", data: "x"}) {
	}
	notices.push()
	requireOneEvent(t, first, "Your change at bgp router-id was discarded by alice")
	drainEvents(second)

	notices.push()
	assert.Empty(t, drainEvents(first), "the first tab already showed the notice")
	requireOneEvent(t, second, "Your change at bgp router-id was discarded by alice")
	notices.push()
	assert.Empty(t, drainEvents(first), "the notice is shown once per tab")
	assert.Empty(t, drainEvents(second), "the notice is shown once per tab")
}
