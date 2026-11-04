// Design: docs/features/cli-commands.md -- bounded session history publication.
// Related: model_history_blocked_test.go -- real Update progress under held storage.

package cli

import (
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// historyDrainStore retains real storage semantics while holding the first
// publication. Close records accidental ownership of the borrowed store.
type historyDrainStore struct {
	storage.Storage
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	mu      sync.Mutex
	writes  []blockedHistoryWrite
	active  int
	peak    int
	calls   int
	closes  int
}

func (s *historyDrainStore) WriteFile(key string, data []byte, perm fs.FileMode) error {
	s.mu.Lock()
	s.active++
	s.calls++
	s.peak = max(s.peak, s.active)
	s.mu.Unlock()
	s.once.Do(func() { close(s.entered); <-s.release })
	err := s.Storage.WriteFile(key, data, perm)
	s.mu.Lock()
	s.writes = append(s.writes, blockedHistoryWrite{key: key, content: string(data)})
	s.active--
	s.mu.Unlock()
	return err
}

func (s *historyDrainStore) Close() error {
	s.mu.Lock()
	s.closes++
	s.mu.Unlock()
	return s.Storage.Close()
}

func newHistoryDrainStore(t *testing.T) *historyDrainStore {
	t.Helper()
	store := storage.CreateMemory()
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	return &historyDrainStore{Storage: store, entered: make(chan struct{}), release: make(chan struct{})}
}

// TestModelHistoryModeSnapshotsDrainInOrder admits a large alternating-mode burst
// through Update while the first write is held. Exact three-write output proves
// latest-per-mode replacement, immutable snapshots, bounded retention and order;
// browsing and reload prove that useful accepted commands were not dropped.
func TestModelHistoryModeSnapshotsDrainInOrder(t *testing.T) {
	model := newTestModel(t)
	t.Cleanup(func() {
		if err := model.editor.Close(); err != nil {
			t.Error(err)
		}
	})
	synctest.Test(t, func(t *testing.T) {
		store := newHistoryDrainStore(t)
		if err := store.Storage.WriteFile(historyKeyMax, []byte("3"), 0); err != nil {
			t.Fatal(err)
		}
		history := NewHistory(store, "snapshot-user")
		model.SetHistory(history)
		model.SetCommandExecutor(func(string) (CommandOutput, error) { return CommandOutput{Text: "ok"}, nil })
		defer model.Close() //nolint:errcheck // Test cleanup after releasing the writer.
		var release sync.Once
		defer release.Do(func() { close(store.release) })
		enter := func(input string) {
			model.SetInput(input)
			applyHistoryProgressMessage(t, model, tea.KeyPressMsg{Code: tea.KeyEnter})
			if model.InputValue() != "" {
				t.Errorf("input not cleared: %q", model.InputValue())
			}
			if got, ok := history.Up("draft"); !ok || got != input {
				t.Errorf("newest browsable entry = %q, %v; want %q", got, ok, input)
			}
			history.resetBrowsing()
		}
		enter("run show initial")
		synctest.Wait()
		select {
		case <-store.entered:
		default:
			t.Fatal("first accepted history never reached storage")
		}
		var config, operational []string
		var admittedConfig, admittedOperational []string
		for i := range 64 {
			config = append(config, "run show config-"+strconv.Itoa(i))
			enter(config[len(config)-1])
			admittedConfig = history.Entries()
			model.SetMode(ModeOperational)
			operational = append(operational, "show operational-"+strconv.Itoa(i))
			enter(operational[len(operational)-1])
			admittedOperational = history.Entries()
			model.SetMode(ModeConfig)
		}
		config = config[len(config)-3:]
		operational = operational[len(operational)-3:]
		if !slices.Equal(history.Entries(), config) {
			t.Errorf("retained config = %v, want %v", history.Entries(), config)
		}
		// Poison the actual final admission slices as well as later restored
		// states. Retaining either live admission instead of detached entries
		// would publish these mutations once the held writer is released.
		admittedConfig[0] = "submitted config mutation"
		admittedOperational[0] = "submitted operational mutation"
		history.entries[0] = "later config mutation"
		model.SetMode(ModeOperational)
		history.entries[0] = "later operational mutation"
		synctest.Wait()
		store.mu.Lock()
		calls, peak := store.calls, store.peak
		store.mu.Unlock()
		if calls != 1 || peak != 1 {
			t.Errorf("held writer calls/peak = %d/%d, want 1/1", calls, peak)
		}
		release.Do(func() { close(store.release) })
		if err := model.Close(); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(history.Load(ModeConfig.String()), config) {
			t.Errorf("config reload = %v, want %v", history.Load(ModeConfig.String()), config)
		}
		if !slices.Equal(history.Load(ModeOperational.String()), operational) {
			t.Errorf("operational reload = %v, want %v", history.Load(ModeOperational.String()), operational)
		}
		want := []blockedHistoryWrite{
			{key: historyKeyPrefix + "snapshot-user/config", content: "run show initial"},
			{key: historyKeyPrefix + "snapshot-user/config", content: strings.Join(config, "\n")},
			{key: historyKeyPrefix + "snapshot-user/operational", content: strings.Join(operational, "\n")},
		}
		store.mu.Lock()
		defer store.mu.Unlock()
		if !slices.Equal(store.writes, want) {
			t.Errorf("publication sequence = %v, want %v", store.writes, want)
		}
		if store.peak != 1 {
			t.Errorf("concurrent publications = %d, want one worker", store.peak)
		}
	})
}

// TestModelHistoryCloseDrainsCopiedOwner closes the initial model while a later
// copy has queued both modes and an unapplied config command result. Close must
// cancel dispatch immediately but wait for independent durable publication.
func TestModelHistoryCloseDrainsCopiedOwner(t *testing.T) {
	model := newTestModel(t)
	t.Cleanup(func() {
		if err := model.editor.Close(); err != nil {
			t.Error(err)
		}
	})
	synctest.Test(t, func(t *testing.T) {
		store := newHistoryDrainStore(t)
		model.SetHistory(NewHistory(store, "copy-user"))
		initial := *model
		var release sync.Once
		defer model.history.Close()
		defer release.Do(func() { close(store.release) })
		model.SetInput("show")
		command := applyHistoryProgressMessage(t, model, tea.KeyPressMsg{Code: tea.KeyEnter})
		if command == nil {
			t.Fatal("config command did not reserve a result turn")
		}
		_ = command() // Deliberately do not deliver the result to Update.
		synctest.Wait()
		select {
		case <-store.entered:
		default:
			t.Fatal("accepted command never reached storage")
		}
		model.SetMode(ModeOperational)
		model.SetInput("set cli format text")
		applyHistoryProgressMessage(t, model, tea.KeyPressMsg{Code: tea.KeyEnter})
		closed := make(chan error, 1)
		go func() { closed <- initial.Close() }()
		synctest.Wait()
		select {
		case err := <-closed:
			t.Fatalf("Close returned before held history drained: %v", err)
		default:
		}
		select {
		case <-model.dispatch.ctx.Done():
		default:
			t.Error("Close did not end dispatch before draining")
		}
		release.Do(func() { close(store.release) })
		if err := <-closed; err != nil {
			t.Fatal(err)
		}
		if err := initial.Close(); err != nil {
			t.Fatal(err)
		}
		if err := model.Close(); err != nil {
			t.Fatal(err)
		}
		store.mu.Lock()
		writes, closes := slices.Clone(store.writes), store.closes
		store.mu.Unlock()
		want := []blockedHistoryWrite{{key: historyKeyPrefix + "copy-user/config", content: "show"}, {key: historyKeyPrefix + "copy-user/operational", content: "set cli format text"}}
		if !slices.Equal(writes, want) {
			t.Errorf("drained writes = %v, want %v", writes, want)
		}
		if closes != 0 {
			t.Errorf("History closed borrowed store %d times", closes)
		}
		if err := store.Storage.WriteFile("meta/borrowed-still-open", []byte("usable"), 0); err != nil {
			t.Fatal(err)
		}
	})
}

// TestHistoryIdleAndNilStoreStayWorkerFree checks construction/close without a
// Save and the existing nil-store no-trim rule, including saves after Close.
func TestHistoryIdleAndNilStoreStayWorkerFree(t *testing.T) {
	store := newHistoryDrainStore(t)
	history := NewHistory(store, "idle")
	history.Close()
	history.Append("after close")
	history.Save(ModeConfig)
	if history.writer != nil {
		t.Fatal("idle/closed history started a worker")
	}
	memory := NewHistory(nil, "")
	memory.max = 1
	memory.Append("first")
	memory.Append("second")
	memory.Save(ModeConfig)
	memory.Close()
	if memory.writer != nil {
		t.Fatal("nil-store history started a worker")
	}
	if !slices.Equal(memory.Entries(), []string{"first", "second"}) {
		t.Errorf("nil Save trimmed useful memory entries: %v", memory.Entries())
	}
}

// TestModelSetHistoryDrainsReplacedOwner proves replacement cannot orphan the
// previous lazy worker; the new borrowed store stays unused until a real Save.
func TestModelSetHistoryDrainsReplacedOwner(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		old := newHistoryDrainStore(t)
		next := NewHistory(nil, "")
		model := NewCommandModel(FilesystemAuthorityUnknown)
		model.SetHistory(NewHistory(old, "old"))
		model.history.Append("old command")
		model.history.Save(ModeOperational)
		synctest.Wait()
		var release sync.Once
		defer release.Do(func() { close(old.release) })
		replaced := make(chan struct{})
		go func() { model.SetHistory(next); close(replaced) }()
		synctest.Wait()
		select {
		case <-replaced:
			t.Fatal("SetHistory orphaned a held old publication")
		default:
		}
		release.Do(func() { close(old.release) })
		<-replaced
		if err := model.Close(); err != nil {
			t.Fatal(err)
		}
		if got := NewHistory(old, "old").Load(ModeOperational.String()); !slices.Equal(got, []string{"old command"}) {
			t.Errorf("replaced owner lost history: %v", got)
		}
		if next.writer != nil {
			t.Fatal("replacement without a store started a worker")
		}
	})
}

// TestModelHistoryAutoConfigAttribution drives the real auto-config transition:
// an operational format command stays operational; a set switches to config;
// run stays in config; configure and empty Enter add no history.
func TestModelHistoryAutoConfigAttribution(t *testing.T) {
	model := newTestModel(t)
	t.Cleanup(func() {
		if err := model.editor.Close(); err != nil {
			t.Error(err)
		}
	})
	store := storage.CreateMemory()
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	model.SetHistory(NewHistory(store, "mode-user"))
	defer model.Close() //nolint:errcheck // Test cleanup.
	model.SetCommandExecutor(func(string) (CommandOutput, error) { return CommandOutput{Text: "ok"}, nil })
	model.SetMode(ModeOperational)
	model.SetInput("configure")
	applyHistoryProgressMessage(t, model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model.SetMode(ModeOperational)
	for _, input := range []string{"set cli format text", "set system host-name history", "run show", ""} {
		model.SetInput(input)
		applyHistoryProgressMessage(t, model, tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	if model.Mode() != ModeConfig {
		t.Fatalf("auto-config mode = %v, want config", model.Mode())
	}
	if err := model.Close(); err != nil {
		t.Fatal(err)
	}
	if got := model.history.Load(ModeConfig.String()); !slices.Equal(got, []string{"set system host-name history", "run show"}) {
		t.Errorf("config attribution = %v", got)
	}
	if got := model.history.Load(ModeOperational.String()); !slices.Equal(got, []string{"set cli format text"}) {
		t.Errorf("operational attribution = %v", got)
	}
}

// TestHistorySessionsKeepIndependentSnapshots preserves existing same-user
// behavior: the last completed session snapshot replaces, rather than merges.
func TestHistorySessionsKeepIndependentSnapshots(t *testing.T) {
	store := storage.CreateMemory()
	defer store.Close() //nolint:errcheck // Test cleanup.
	first, second := NewHistory(store, "same-user"), NewHistory(store, "same-user")
	defer first.Close()
	defer second.Close()
	first.Append("first session")
	second.Append("second session")
	first.Save(ModeConfig)
	first.Close()
	second.Save(ModeConfig)
	second.Close()
	if got := NewHistory(store, "same-user").Load(ModeConfig.String()); !slices.Equal(got, []string{"second session"}) {
		t.Errorf("independent last snapshot = %v", got)
	}
}

// TestHistoryPublicationRespectsMaxBoundaries reloads exact newest entries after
// default/custom/clamped maxima, rather than checking only the Max accessor.
func TestHistoryPublicationRespectsMaxBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, configured string
		maximum          int
	}{
		{name: "default", maximum: historyMaxDefault},
		{name: "custom", configured: "3", maximum: 3},
		{name: "zero", configured: "0", maximum: 1},
		{name: "negative", configured: "-1", maximum: 1},
		{name: "ceiling", configured: "10000", maximum: historyMaxCeiling},
		{name: "above ceiling", configured: "10001", maximum: historyMaxCeiling},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := storage.CreateMemory()
			defer store.Close() //nolint:errcheck // Store outlives history.
			if tc.configured != "" {
				if err := store.WriteFile(historyKeyMax, []byte(tc.configured), 0); err != nil {
					t.Fatal(err)
				}
			}
			history := NewHistory(store, "bounds")
			defer history.Close()
			var all []string
			for i := range tc.maximum + 2 {
				entry := "bounded-" + strconv.Itoa(i)
				all = append(all, entry)
				history.Append(entry)
			}
			history.Save(ModeConfig)
			history.Close()
			want := all[2:]
			if !slices.Equal(history.Entries(), want) {
				t.Errorf("retained %d entries, want exact newest %d", len(history.Entries()), tc.maximum)
			}
			if got := history.Load(ModeConfig.String()); !slices.Equal(got, want) {
				t.Errorf("reloaded %d entries, want exact newest %d", len(got), tc.maximum)
			}
		})
	}
}

// TestHistoryModeProjectionBoundsPublicationKeys exercises the existing String
// projection over enum casts too: Save may never enqueue arbitrary enum keys.
// Exactly one active and two newest waiting whole payloads reach storage.
func TestHistoryModeProjectionBoundsPublicationKeys(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := newHistoryDrainStore(t)
		history := NewHistory(store, "projection")
		defer history.Close()
		var release sync.Once
		defer release.Do(func() { close(store.release) })
		history.Append("initial")
		history.Save(ModeConfig)
		synctest.Wait()
		select {
		case <-store.entered:
		default:
			t.Fatal("initial publication never reached storage")
		}
		for i := range 128 {
			history.restore(historySnapshot{entries: []string{"projected-" + strconv.Itoa(i)}, idx: -1})
			history.Save(EditorMode(i + 2))
		}
		history.restore(historySnapshot{entries: []string{"operational"}, idx: -1})
		history.Save(ModeOperational)
		release.Do(func() { close(store.release) })
		history.Close()
		want := []blockedHistoryWrite{
			{key: historyKeyPrefix + "projection/" + ModeConfig.String(), content: "initial"},
			{key: historyKeyPrefix + "projection/" + ModeConfig.String(), content: "projected-127"},
			{key: historyKeyPrefix + "projection/" + ModeOperational.String(), content: "operational"},
		}
		store.mu.Lock()
		defer store.mu.Unlock()
		if !slices.Equal(store.writes, want) {
			t.Errorf("projected publication sequence = %v, want %v", store.writes, want)
		}
	})
}
