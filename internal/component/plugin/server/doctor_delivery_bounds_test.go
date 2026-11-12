package server

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestEnricherFailuresPastTheBoundAreCounted records more distinct enricher
// failures for one plugin than the doctor keeps (R-9).
//
// VALIDATES: the first serverRecordMax failures are kept whole, each failing
// enricher past the bound is counted once however often it fails, a call
// that succeeds takes its enricher out of the count, and the plugin's
// snapshot state carries the count.
// PREVENTS: the AC-21 row listing sixteen enrichers while more fail, with
// nothing to say they exist.
func TestEnricherFailuresPastTheBoundAreCounted(t *testing.T) {
	const name = "enrich-bounded"
	t.Cleanup(func() { forgetEnricherFailures(name) })

	for i := range serverRecordMax + 3 {
		recordEnricherFailure(name, EnricherFailure{Command: "show bounded", Key: "key-" + strconv.Itoa(i), Error: "timeout"})
	}
	// A dropped enricher failing again is the same enricher, not one more.
	recordEnricherFailure(name, EnricherFailure{Command: "show bounded", Key: "key-" + strconv.Itoa(serverRecordMax), Error: "timeout"})

	if got := len(enricherFailures(name)); got != serverRecordMax {
		t.Fatalf("kept %d failures, want %d", got, serverRecordMax)
	}
	if got := enricherFailuresDropped(name); got != 3 {
		t.Fatalf("counted %d failures past the bound, want 3", got)
	}

	clearEnricherCallFailure(name, "show bounded", "key-"+strconv.Itoa(serverRecordMax+1))
	if got := enricherFailuresDropped(name); got != 2 {
		t.Fatalf("a dropped enricher whose call succeeded is still counted: %d, want 2", got)
	}

	s := &Server{}
	s.procManager.Store(process.NewProcessManager(nil))
	if got := s.pluginDoctorState(name, true).EnrichersDropped; got != 2 {
		t.Fatalf("the snapshot carries %d dropped enricher failures, want 2", got)
	}
}

// TestHandlerPanicsPastTheBoundReachTheSnapshot recovers more engine handler
// panics than the record keeps, then reads the record after the window.
//
// VALIDATES: the snapshot carries the newest panics kept and says the ring
// overwrote some inside the window; once the window has passed (the clock is
// injected, as TestDeliveryWindowForgetsOldProblems does) no panic remains
// and nothing is truncated, so the row leaves.
// PREVENTS: a panic row that never clears, and a full record that reads as
// the whole story.
func TestHandlerPanicsPastTheBoundReachTheSnapshot(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	s.procManager.Store(process.NewProcessManager(nil))
	for range serverRecordMax + 2 {
		s.engineSubscribers.recordHandlerPanic(events.NamespaceID(1), events.EventTypeID(1), "boom")
	}
	snapshot, err := s.doctorSnapshot()
	if err != nil {
		t.Fatalf("doctorSnapshot: %v", err)
	}
	if len(snapshot.HandlerPanics) != serverRecordMax || !snapshot.HandlerPanicsTruncated {
		t.Fatalf("snapshot holds %d panics, truncated %v, want %d and true",
			len(snapshot.HandlerPanics), snapshot.HandlerPanicsTruncated, serverRecordMax)
	}
	later, truncated := s.engineSubscribers.handlerPanicsAt(time.Now().Add(HandlerPanicWindow + time.Second))
	if len(later) != 0 || truncated {
		t.Fatalf("after the window %d panics remain, truncated %v", len(later), truncated)
	}
}

// enrichResponder answers each enrich-show request with the next outcome:
// an error string fails the call, an empty one answers data under key.
func enrichResponder(t *testing.T, key string, outcomes ...string) *ipc.PluginConn {
	t.Helper()
	engineEnd, pluginEnd := net.Pipe()
	t.Cleanup(func() {
		engineEnd.Close() //nolint:errcheck // test cleanup
		pluginEnd.Close() //nolint:errcheck // test cleanup
	})
	pluginConn := ipc.NewPluginConn(pluginEnd, pluginEnd)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		for _, outcome := range outcomes {
			req, err := pluginConn.ReadRequest(ctx)
			if err != nil {
				return
			}
			if outcome != "" {
				_ = pluginConn.SendError(ctx, req.ID, outcome)
				continue
			}
			_ = pluginConn.SendResult(ctx, req.ID, &rpc.EnrichShowOutput{Data: map[string]any{key: "enriched"}})
		}
	}()
	return ipc.NewPluginConn(engineEnd, engineEnd)
}

// TestProxyEnricherCallFailureRecordedAndCleared calls one proxy enricher
// whose plugin fails the first call and answers the second (AC-21, R-10).
//
// VALIDATES: makeProxyCall records the failed call as a call failure naming
// command, key and cause, and the next successful call clears it.
// PREVENTS: a `show` field silently missing on every call because its
// plugin's enricher fails, or a recovered enricher still reported failing.
func TestProxyEnricherCallFailureRecordedAndCleared(t *testing.T) {
	const name = "enrich-call"
	t.Cleanup(func() { forgetEnricherFailures(name) })
	call := makeProxyCall(name, "show doctor-call-enricher", "extra", "detail",
		enrichResponder(t, "extra", "enricher broke", ""))

	call(map[string]any{})
	failures := enricherFailures(name)
	if len(failures) != 1 {
		t.Fatalf("want the failed call recorded, got %+v", failures)
	}
	if failures[0].Registration || failures[0].Command != "show doctor-call-enricher" || failures[0].Key != "extra" || failures[0].Error == "" {
		t.Fatalf("the call failure does not name command, key and cause: %+v", failures[0])
	}

	base := map[string]any{}
	call(base)
	if base["extra"] != "enriched" {
		t.Fatalf("the second call did not answer the field: %+v", base)
	}
	if got := enricherFailures(name); len(got) != 0 {
		t.Fatalf("a successful call left its failure recorded: %+v", got)
	}
}

// TestConfigCommittedWithNoConnectionRecorded commits a transaction to a
// participant whose process the manager still runs with no connection, and
// to one whose process was replaced (AC-22, R-10).
//
// VALIDATES: the current connless process records the commit it was not
// told of, with errConfigCommittedNoConn; the replaced lifetime records
// nothing, since it is no longer the plugin the doctor reads.
// PREVENTS: a plugin never told of a commit reading as told, because the
// lookup that refuses a replaced lifetime also refused a missing connection.
func TestConfigCommittedWithNoConnectionRecorded(t *testing.T) {
	pm := process.NewProcessManager(nil)
	connless := process.NewProcess(plugin.PluginConfig{Name: "connless"})
	pm.AddProcess("connless", connless)
	replaced := process.NewProcess(plugin.PluginConfig{Name: "replaced"})
	pm.AddProcess("replaced", process.NewProcess(plugin.PluginConfig{Name: "replaced"}))
	s := &Server{}
	s.procManager.Store(pm)
	b := &configTxBridge{server: s, participants: map[string]reloadParticipant{
		"connless": {proc: connless},
		"replaced": {proc: replaced},
	}}

	b.dispatchCommitted(context.Background(), "tx-7", "connless")
	b.dispatchCommitted(context.Background(), "tx-7", "replaced")

	commit := connless.DoctorRecord(time.Now()).ConfigCommit
	if commit.Transaction != "tx-7" || commit.Error != errConfigCommittedNoConn.Error() {
		t.Fatalf("the connless participant's commit was not recorded: %+v", commit)
	}
	if got := replaced.DoctorRecord(time.Now()).ConfigCommit; got.Transaction != "" {
		t.Fatalf("a replaced lifetime recorded the commit: %+v", got)
	}
}

// TestConfigUnappliedClearsOnLaterCommitNotOnUnchangedReload drives a plugin
// that misses one config-committed through an unchanged reload, then through
// a reload that changes its root (R-8).
//
// VALIDATES: a reload of an unchanged config sends no config-committed and
// leaves the record failed, so `ze signal reload` is no fix for the row; the
// next transaction the plugin is told of replaces the record and clears it.
// PREVENTS: the doctor row naming a fix that never clears it.
func TestConfigUnappliedClearsOnLaterCommitNotOnUnchangedReload(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	thirdTree := map[string]any{"bgp": map[string]any{"router-id": "9.9.9.9"}}
	reactor := &mockReloadReactor{tree: oldTree}
	defs := []pluginDef{{name: "rib", roots: []string{"bgp"}, ignoreCommitted: true}}
	s := newTestReloadServer(t, reactor, defs)
	responder := defs[0].responder

	if err := s.ReloadConfig(context.Background(), newTree); err != nil {
		t.Fatalf("first reload: %v", err)
	}
	proc := s.procManager.Load().GetProcess("rib")
	if proc == nil {
		t.Fatal("no rib process")
	}
	missed := proc.DoctorRecord(time.Now()).ConfigCommit
	if missed.Transaction == "" || missed.Error == "" {
		t.Fatalf("the unanswered config-committed was not recorded: %+v", missed)
	}

	// The daemon now runs newTree: reloading it is the unchanged reload.
	reactor.mu.Lock()
	reactor.tree = newTree
	reactor.mu.Unlock()
	responder.mu.Lock()
	sent := responder.committedCalls
	responder.mu.Unlock()
	if err := s.ReloadConfig(context.Background(), newTree); err != nil {
		t.Fatalf("unchanged reload: %v", err)
	}
	responder.mu.Lock()
	resent := responder.committedCalls
	responder.ignoreCommitted = false
	responder.mu.Unlock()
	if resent != sent {
		t.Fatalf("an unchanged reload sent config-committed %d times", resent-sent)
	}
	if got := proc.DoctorRecord(time.Now()).ConfigCommit; got != missed {
		t.Fatalf("an unchanged reload changed the record: %+v, was %+v", got, missed)
	}

	if err := s.ReloadConfig(context.Background(), thirdTree); err != nil {
		t.Fatalf("third reload: %v", err)
	}
	told := proc.DoctorRecord(time.Now()).ConfigCommit
	if told.Error != "" || told.Transaction == "" || told.Transaction == missed.Transaction {
		t.Fatalf("the next told transaction did not clear the record: %+v", told)
	}
}

// TestRestartClearsConfigUnappliedRecord restarts a plugin whose process
// recorded a missed config-committed, through the restart that
// `ze request plugin restart` runs (R-8).
//
// VALIDATES: the restart replaces the process, and the plugin's doctor state
// after it holds no failed commit: the replacement's records start clean.
// PREVENTS: the AC-22 row naming a fix after which it still reports.
func TestRestartClearsConfigUnappliedRecord(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "commit-restart"
	err := registry.Register(registry.Registration{
		Name:        name,
		Description: "restart clears a missed config-committed",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Registered before the server, so it runs after the server's teardown
	// (cleanups run last-in first-out): once the server is canceled and its
	// processes stopped, wait for every goroutine it started, the restart's
	// supervise goroutine included, before the test returns.
	var s *Server
	t.Cleanup(func() {
		if s != nil {
			s.wg.Wait()
		}
	})
	s, _ = newLifecycleStartupServer(t)
	if err := s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}); err != nil {
		t.Fatalf("startup: %v", err)
	}
	s.dispatcher.Registry().Freeze()

	before := s.procManager.Load().GetProcess(name)
	if before == nil {
		t.Fatal("no process after startup")
	}
	before.RecordConfigCommit("tx-missed", errors.New("config-committed timed out"))
	if got := s.pluginDoctorState(name, true).Records.ConfigCommit; got.Error == "" {
		t.Fatalf("the missed commit is not in the doctor state: %+v", got)
	}

	if err := s.restartPlugin(name); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if after := s.procManager.Load().GetProcess(name); after == nil || after == before {
		t.Fatalf("the restart did not replace the process: %p, was %p", after, before)
	}
	state := s.pluginDoctorState(name, true)
	if !state.HasProcess {
		t.Fatalf("no process after the restart: %+v", state)
	}
	if state.Records.ConfigCommit.Error != "" {
		t.Fatalf("the restart left the missed commit recorded: %+v", state.Records.ConfigCommit)
	}
}
