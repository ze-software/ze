package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCommandRefusalRecordedAtRegistration registers one command name from
// two plugins, and an alias over the first plugin's command (AC-13).
//
// VALIDATES: the second plugin's process records the refused command and the
// refused alias, each naming the plugin that owns the name.
// PREVENTS: a plugin running with a command it declared and never got, known
// only to a Warn line.
func TestCommandRefusalRecordedAtRegistration(t *testing.T) {
	registry := newCommandRegistry()
	owner := process.NewProcess(plugin.PluginConfig{Name: "first"})
	late := process.NewProcess(plugin.PluginConfig{Name: "second"})

	registerProcessCommands(registry, owner, &plugin.PluginRegistration{Commands: []string{"show alpha"}})
	registerProcessCommands(registry, late, &plugin.PluginRegistration{
		Commands:               []string{"show alpha", "show beta"},
		CommandDeprecatedNames: map[string][]string{"show beta": {"show alpha"}},
	})

	refusals := late.DoctorRecord(time.Now()).CommandRefusals
	if len(refusals) != 2 {
		t.Fatalf("want the command and the alias refused, got %+v", refusals)
	}
	for _, refusal := range refusals {
		if refusal.Command != "show alpha" || refusal.Owner != "first" {
			t.Fatalf("refusal does not name the owned name and its owner: %+v", refusal)
		}
	}
	if !refusals[1].Alias {
		t.Fatalf("the alias refusal is not marked an alias: %+v", refusals[1])
	}
	if got := owner.DoctorRecord(time.Now()).CommandRefusals; len(got) != 0 {
		t.Fatalf("the owner recorded refusals: %+v", got)
	}
}

// TestSubscriptionDropRecorded subscribes under a namespace nothing
// registered (AC-14).
//
// VALIDATES: resolveSubscriptionNamespace records the dropped block on the
// process, naming the namespace and the cause.
// PREVENTS: a plugin whose subscriptions match nothing reading as healthy.
func TestSubscriptionDropRecorded(t *testing.T) {
	proc := process.NewProcess(plugin.PluginConfig{Name: "subscriber"})
	if _, _, ok := (&Server{}).resolveSubscriptionNamespace(proc, "no-such-namespace"); ok {
		t.Fatal("an unknown namespace resolved")
	}
	drops := proc.DoctorRecord(time.Now()).SubscriptionDrops
	if len(drops) != 1 {
		t.Fatalf("want one drop, got %+v", drops)
	}
	if drops[0].Cause != process.SubscriptionDropUnknownNamespace || drops[0].Namespace != "no-such-namespace" {
		t.Fatalf("drop does not name the cause and namespace: %+v", drops[0])
	}
}

// TestEngineHandlerPanicRecorded dispatches to an engine handler that panics
// (AC-18).
//
// VALIDATES: the recovered panic is recorded with its namespace, event type
// and value, for the doctor snapshot.
// PREVENTS: an engine handler that panics on every event reaching only the log.
func TestEngineHandlerPanicRecorded(t *testing.T) {
	subs := newEngineEventSubscribers()
	ns, et := events.NamespaceID(1), events.EventTypeID(1)
	subs.register(ns, et, func(any) { panic("handler broke") })
	subs.dispatch(ns, et, nil)

	panics := subs.handlerPanics()
	if len(panics) != 1 {
		t.Fatalf("want one recorded panic, got %+v", panics)
	}
	if panics[0].Panic != "handler broke" || panics[0].Namespace == "" || panics[0].EventType == "" {
		t.Fatalf("panic record does not name the handler and the value: %+v", panics[0])
	}
}

// TestPostStartupFailureRecorded reports a failed post-startup delivery
// while the daemon runs, and one while it stops (AC-17).
//
// VALIDATES: the running failure records PostStartupUndelivered with the
// cause; a failure once the daemon stops records nothing.
// PREVENTS: a plugin that never ran its post-startup work reading as healthy.
func TestPostStartupFailureRecorded(t *testing.T) {
	proc := process.NewProcess(plugin.PluginConfig{Name: "late"})
	s := &Server{ctx: context.Background()}
	s.recordPostStartupOutcome(proc, errors.New("connection closed"))
	record := proc.DoctorRecord(time.Now())
	if record.PostStartup != process.PostStartupUndelivered || record.PostStartupError != "connection closed" {
		t.Fatalf("undelivered callback not recorded: %v %q", record.PostStartup, record.PostStartupError)
	}

	s.recordPostStartupOutcome(proc, nil)
	if got := proc.DoctorRecord(time.Now()).PostStartup; got != process.PostStartupDelivered {
		t.Fatalf("a delivered callback is not recorded delivered: %v", got)
	}

	stopping, cancel := context.WithCancel(context.Background())
	cancel()
	quiet := process.NewProcess(plugin.PluginConfig{Name: "quiet"})
	(&Server{ctx: stopping}).recordPostStartupOutcome(quiet, errors.New("closed"))
	if got := quiet.DoctorRecord(time.Now()).PostStartup; got == process.PostStartupUndelivered {
		t.Fatal("a failure while the daemon stops was recorded undelivered")
	}
}

// TestEnricherFailureRecorded registers one enricher key from two plugins
// (AC-21).
//
// VALIDATES: the second plugin's refused registration is recorded with its
// command and key, and the record goes with the plugin's cleanup.
// PREVENTS: a show field silently missing because its enricher never
// registered.
func TestEnricherFailureRecorded(t *testing.T) {
	decl := []rpc.EnricherDecl{{Command: "show doctor-test-enricher", Key: "extra"}}
	registerProxyEnrichers("enrich-first", decl, nil)
	t.Cleanup(func() { unregisterProxyEnrichers("enrich-first") })
	registerProxyEnrichers("enrich-second", decl, nil)
	t.Cleanup(func() { unregisterProxyEnrichers("enrich-second") })

	failures := enricherFailures("enrich-second")
	if len(failures) != 1 {
		t.Fatalf("want one enricher failure, got %+v", failures)
	}
	if failures[0].Command != "show doctor-test-enricher" || failures[0].Key != "extra" || failures[0].Error == "" {
		t.Fatalf("failure does not name command, key and cause: %+v", failures[0])
	}
	if got := enricherFailures("enrich-first"); len(got) != 0 {
		t.Fatalf("the plugin that registered recorded a failure: %+v", got)
	}
	unregisterProxyEnrichers("enrich-second")
	if got := enricherFailures("enrich-second"); len(got) != 0 {
		t.Fatalf("the failure outlived the plugin's cleanup: %+v", got)
	}
}

// TestDoctorSnapshotCarriesRecords publishes a server whose plugin process
// holds records, and reads the snapshot the doctor checks read.
//
// VALIDATES: the snapshot carries the process's records, its enricher
// failures, the recovered engine handler panics and the daemon's last
// committed transaction.
// PREVENTS: a record the producers keep that never reaches a check.
func TestDoctorSnapshotCarriesRecords(t *testing.T) {
	previous := doctorSnapshotSource.Swap(nil)
	t.Cleanup(func() { doctorSnapshotSource.Store(previous) })

	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	pm := process.NewProcessManager([]plugin.PluginConfig{{Name: "recorded", Internal: true}})
	proc := process.NewProcess(plugin.PluginConfig{Name: "recorded", Internal: true})
	pm.AddProcess("recorded", proc)
	s.procManager.Store(pm)
	publishDoctorSnapshot(s)

	proc.RecordCommandRefusal(process.CommandRefusal{Command: "show alpha", Owner: "first"})
	recordEnricherFailure("recorded", EnricherFailure{Command: "show peer", Key: "extra", Registration: true, Error: "duplicate key"})
	t.Cleanup(func() { forgetEnricherFailures("recorded") })
	s.recordCommitted("tx-3")
	if s.engineSubscribers == nil {
		t.Fatal("NewServer left no engine subscribers")
	}
	s.engineSubscribers.recordHandlerPanic(events.NamespaceID(1), events.EventTypeID(1), "boom")

	snapshot, err := PluginDoctorSnapshot()
	if err != nil {
		t.Fatalf("PluginDoctorSnapshot: %v", err)
	}
	if len(snapshot.Plugins) != 1 {
		t.Fatalf("want one plugin, got %+v", snapshot.Plugins)
	}
	state := snapshot.Plugins[0]
	if len(state.Records.CommandRefusals) != 1 || state.Records.CommandRefusals[0].Owner != "first" {
		t.Fatalf("the process records did not reach the snapshot: %+v", state.Records)
	}
	if len(state.Enrichers) != 1 {
		t.Fatalf("the enricher failure did not reach the snapshot: %+v", state.Enrichers)
	}
	if snapshot.LastCommittedTransaction != "tx-3" {
		t.Fatalf("last committed transaction %q, want tx-3", snapshot.LastCommittedTransaction)
	}
	if len(snapshot.HandlerPanics) != 1 || snapshot.HandlerPanics[0].Panic != "boom" {
		t.Fatalf("the engine handler panic did not reach the snapshot: %+v", snapshot.HandlerPanics)
	}
}

// TestConfigUnappliedRecordedOnCommitFailure runs a reload against a plugin
// that never answers config-committed (AC-22).
//
// VALIDATES: the plugin's process records the transaction it was not told of,
// with the cause, and the server records that transaction as the daemon's
// last committed one.
// PREVENTS: a plugin keeping its rollback gate open with nothing but an
// Error line to show for it.
func TestConfigUnappliedRecordedOnCommitFailure(t *testing.T) {
	t.Parallel()

	oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
	newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
	reactor := &mockReloadReactor{tree: oldTree}
	s := newTestReloadServer(t, reactor, []pluginDef{{name: "rib", roots: []string{"bgp"}, ignoreCommitted: true}})

	if err := s.ReloadConfig(context.Background(), newTree); err != nil {
		t.Fatalf("reload: %v", err)
	}
	proc := s.procManager.Load().GetProcess("rib")
	if proc == nil {
		t.Fatal("no rib process")
	}
	commit := proc.DoctorRecord(time.Now()).ConfigCommit
	if commit.Transaction == "" || commit.Error == "" {
		t.Fatalf("the unanswered config-committed was not recorded: %+v", commit)
	}
	if got := s.lastCommittedTransaction(); got != commit.Transaction {
		t.Fatalf("daemon last committed %q, plugin record %q", got, commit.Transaction)
	}
}
