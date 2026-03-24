package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// VALIDATES: AC-1 — Load stores config, queryable via Get().
// PREVENTS: Config not stored after Load.
func TestLoad(t *testing.T) {
	mgr := config.NewProvider()

	// Write a test config file.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := map[string]any{
		"bgp": map[string]any{
			"router-id": "1.2.3.4",
		},
	}
	writeJSON(t, path, data)

	if err := mgr.Load(path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	tree, err := mgr.Get("bgp")
	if err != nil {
		t.Fatalf("Get('bgp') error: %v", err)
	}
	if tree == nil {
		t.Fatal("Get('bgp') returned nil after Load")
		return
	}
	if tree["router-id"] != "1.2.3.4" {
		t.Errorf("router-id = %v, want '1.2.3.4'", tree["router-id"])
	}
}

// VALIDATES: AC-2 — Get existing root returns correct subtree.
// PREVENTS: Wrong subtree returned.
func TestGet(t *testing.T) {
	mgr := config.NewProvider()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := map[string]any{
		"bgp": map[string]any{
			"local-as": float64(65000),
		},
		"hub": map[string]any{
			"socket": "/var/run/ze.sock",
		},
	}
	writeJSON(t, path, data)

	if err := mgr.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	bgp, err := mgr.Get("bgp")
	if err != nil {
		t.Fatalf("Get('bgp') error: %v", err)
	}
	if bgp["local-as"] != float64(65000) {
		t.Errorf("bgp local-as = %v, want 65000", bgp["local-as"])
	}

	hub, err := mgr.Get("hub")
	if err != nil {
		t.Fatalf("Get('hub') error: %v", err)
	}
	if hub["socket"] != "/var/run/ze.sock" {
		t.Errorf("hub socket = %v, want '/var/run/ze.sock'", hub["socket"])
	}
}

// VALIDATES: AC-3 — Get non-existing root returns empty map, nil.
// PREVENTS: Error on missing root.
func TestGetNonExisting(t *testing.T) {
	mgr := config.NewProvider()

	tree, err := mgr.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get('nonexistent') returned error: %v", err)
	}
	if len(tree) != 0 {
		t.Errorf("Get('nonexistent') returned %v, want empty map", tree)
	}
}

// VALIDATES: AC-4 — RegisterSchema adds module to schema.
// PREVENTS: Lost schema registrations.
func TestRegisterSchema(t *testing.T) {
	mgr := config.NewProvider()

	if err := mgr.RegisterSchema("bgp-rib", "module bgp-rib {}"); err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	schema := mgr.Schema()
	if len(schema.Modules) != 1 {
		t.Fatalf("got %d modules, want 1", len(schema.Modules))
	}
	if schema.Modules[0] != "bgp-rib" {
		t.Errorf("module = %q, want %q", schema.Modules[0], "bgp-rib")
	}
}

// VALIDATES: AC-5 — RegisterSchema with duplicate name returns error.
// PREVENTS: Silent schema name collision.
func TestRegisterSchemaDuplicate(t *testing.T) {
	mgr := config.NewProvider()

	if err := mgr.RegisterSchema("bgp-rib", "module bgp-rib {}"); err != nil {
		t.Fatalf("first RegisterSchema: %v", err)
	}

	err := mgr.RegisterSchema("bgp-rib", "module bgp-rib {}")
	if err == nil {
		t.Fatal("expected error for duplicate schema, got nil")
		return
	}
}

// VALIDATES: AC-6 — Validate with no errors returns empty slice.
// PREVENTS: False validation errors.
func TestValidate(t *testing.T) {
	mgr := config.NewProvider()

	errs := mgr.Validate()
	if len(errs) != 0 {
		t.Errorf("Validate() returned %d errors, want 0", len(errs))
	}
}

// VALIDATES: AC-7 — Save writes config to file.
// PREVENTS: Config not persisted.
func TestSave(t *testing.T) {
	mgr := config.NewProvider()

	dir := t.TempDir()
	loadPath := filepath.Join(dir, "input.json")
	data := map[string]any{
		"bgp": map[string]any{
			"router-id": "1.2.3.4",
		},
	}
	writeJSON(t, loadPath, data)

	if err := mgr.Load(loadPath); err != nil {
		t.Fatalf("Load: %v", err)
	}

	savePath := filepath.Join(dir, "output.json")
	if err := mgr.Save(savePath); err != nil {
		t.Fatalf("Save: %v", err)
	}

	saved, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(saved) == 0 {
		t.Fatal("saved file is empty")
	}
}

// VALIDATES: AC-8 — Watch channel receives ConfigChange on reload.
// PREVENTS: Lost reload notifications.
func TestWatch(t *testing.T) {
	mgr := config.NewProvider()

	ch := mgr.Watch("bgp")
	if ch == nil {
		t.Fatal("Watch returned nil channel")
		return
	}

	// Load config — watchers for "bgp" should be notified.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := map[string]any{
		"bgp": map[string]any{
			"router-id": "5.6.7.8",
		},
	}
	writeJSON(t, path, data)

	if err := mgr.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	select {
	case change := <-ch:
		if change.Root != "bgp" {
			t.Errorf("change.Root = %q, want %q", change.Root, "bgp")
		}
		if change.Tree["router-id"] != "5.6.7.8" {
			t.Errorf("change.Tree router-id = %v, want '5.6.7.8'", change.Tree["router-id"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for watch notification")
	}
}

// VALIDATES: AC-11 — Multiple watchers for same root all receive notification.
// PREVENTS: Lost notifications to some watchers.
func TestWatchMultiple(t *testing.T) {
	mgr := config.NewProvider()

	ch1 := mgr.Watch("bgp")
	ch2 := mgr.Watch("bgp")

	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := map[string]any{
		"bgp": map[string]any{
			"router-id": "9.10.11.12",
		},
	}
	writeJSON(t, path, data)

	if err := mgr.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	for i, ch := range []<-chan ze.ConfigChange{ch1, ch2} {
		select {
		case change := <-ch:
			if change.Root != "bgp" {
				t.Errorf("watcher %d: Root = %q, want %q", i, change.Root, "bgp")
			}
		case <-time.After(time.Second):
			t.Fatalf("watcher %d: timed out waiting for notification", i)
		}
	}
}

// VALIDATES: AC-12 — Second Load replaces config and notifies watchers.
// PREVENTS: Stale config after reload.
func TestLoadReplacesAndNotifies(t *testing.T) {
	mgr := config.NewProvider()

	ch := mgr.Watch("bgp")

	dir := t.TempDir()
	path1 := filepath.Join(dir, "v1.json")
	data1 := map[string]any{
		"bgp": map[string]any{
			"router-id": "1.1.1.1",
		},
	}
	writeJSON(t, path1, data1)

	if err := mgr.Load(path1); err != nil {
		t.Fatalf("Load v1: %v", err)
	}

	// Drain first notification.
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out on first notification")
	}

	// Load v2 — should replace and notify again.
	path2 := filepath.Join(dir, "v2.json")
	data2 := map[string]any{
		"bgp": map[string]any{
			"router-id": "2.2.2.2",
		},
	}
	writeJSON(t, path2, data2)

	if err := mgr.Load(path2); err != nil {
		t.Fatalf("Load v2: %v", err)
	}

	select {
	case change := <-ch:
		if change.Tree["router-id"] != "2.2.2.2" {
			t.Errorf("after reload: router-id = %v, want '2.2.2.2'", change.Tree["router-id"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out on second notification")
	}

	// Verify Get returns v2.
	tree, err := mgr.Get("bgp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tree["router-id"] != "2.2.2.2" {
		t.Errorf("Get router-id = %v, want '2.2.2.2'", tree["router-id"])
	}
}

// VALIDATES: AC-10 — Schema returns all registered module names.
// PREVENTS: Missing modules in schema listing.
func TestSchemaModules(t *testing.T) {
	mgr := config.NewProvider()

	names := []string{"bgp-gr", "bgp-rib", "bgp-rs"}
	for _, name := range names {
		if err := mgr.RegisterSchema(name, "module "+name+" {}"); err != nil {
			t.Fatalf("RegisterSchema(%s): %v", name, err)
		}
	}

	schema := mgr.Schema()
	if len(schema.Modules) != 3 {
		t.Fatalf("got %d modules, want 3", len(schema.Modules))
	}

	sort.Strings(schema.Modules)
	for i, name := range names {
		if schema.Modules[i] != name {
			t.Errorf("modules[%d] = %q, want %q", i, schema.Modules[i], name)
		}
	}
}

// VALIDATES: Full lifecycle: register-schema → load → get → watch → reload.
// PREVENTS: State machine bugs.
func TestLifecycle(t *testing.T) {
	mgr := config.NewProvider()

	// Register schema.
	if err := mgr.RegisterSchema("bgp-rib", "module bgp-rib {}"); err != nil {
		t.Fatalf("RegisterSchema: %v", err)
	}

	// Watch before load.
	ch := mgr.Watch("bgp")

	// Load config.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := map[string]any{
		"bgp": map[string]any{
			"router-id": "10.0.0.1",
			"local-as":  float64(65001),
		},
	}
	writeJSON(t, path, data)

	if err := mgr.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Watch notification received.
	select {
	case change := <-ch:
		if change.Root != "bgp" {
			t.Errorf("change.Root = %q, want %q", change.Root, "bgp")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for watch notification")
	}

	// Get returns loaded config.
	tree, err := mgr.Get("bgp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tree["router-id"] != "10.0.0.1" {
		t.Errorf("router-id = %v, want '10.0.0.1'", tree["router-id"])
	}

	// Validate returns no errors.
	errs := mgr.Validate()
	if len(errs) != 0 {
		t.Errorf("Validate: %d errors, want 0", len(errs))
	}

	// Schema includes registered module.
	schema := mgr.Schema()
	if len(schema.Modules) != 1 || schema.Modules[0] != "bgp-rib" {
		t.Errorf("Schema.Modules = %v, want [bgp-rib]", schema.Modules)
	}
}

// VALIDATES: AC-1 — SetRoot stores tree, Get retrieves it.
// PREVENTS: SetRoot not persisting data.
func TestProviderSetRoot(t *testing.T) {
	mgr := config.NewProvider()

	tree := map[string]any{
		"router-id": "10.0.0.1",
		"local-as":  float64(65000),
	}
	mgr.SetRoot("bgp", tree)

	got, err := mgr.Get("bgp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got["router-id"] != "10.0.0.1" {
		t.Errorf("router-id = %v, want '10.0.0.1'", got["router-id"])
	}
	if got["local-as"] != float64(65000) {
		t.Errorf("local-as = %v, want 65000", got["local-as"])
	}
}

// VALIDATES: AC-1 — SetRoot overwrites existing root.
// PREVENTS: Stale config after SetRoot.
func TestProviderSetRootOverwrite(t *testing.T) {
	mgr := config.NewProvider()

	mgr.SetRoot("bgp", map[string]any{"router-id": "1.1.1.1"})
	mgr.SetRoot("bgp", map[string]any{"router-id": "2.2.2.2"})

	got, err := mgr.Get("bgp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got["router-id"] != "2.2.2.2" {
		t.Errorf("router-id = %v, want '2.2.2.2'", got["router-id"])
	}
}

// VALIDATES: AC-1 — SetRoot notifies watchers.
// PREVENTS: Watchers not receiving updates from SetRoot.
func TestProviderSetRootNotifiesWatchers(t *testing.T) {
	mgr := config.NewProvider()

	ch := mgr.Watch("bgp")

	mgr.SetRoot("bgp", map[string]any{"router-id": "3.3.3.3"})

	select {
	case change := <-ch:
		if change.Root != "bgp" {
			t.Errorf("Root = %q, want %q", change.Root, "bgp")
		}
		if change.Tree["router-id"] != "3.3.3.3" {
			t.Errorf("Tree[router-id] = %v, want '3.3.3.3'", change.Tree["router-id"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for watch notification")
	}
}

// VALIDATES: SetRoot with nil tree stores empty map (no panic).
// PREVENTS: Nil pointer dereference on nil tree.
func TestProviderSetRootNil(t *testing.T) {
	mgr := config.NewProvider()

	mgr.SetRoot("bgp", nil)

	got, err := mgr.Get("bgp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Get returned %v, want empty map", got)
	}
}

// VALIDATES: ConfigProvider interface satisfaction.
// PREVENTS: Compile-time interface drift.
func TestProviderSatisfiesInterface(t *testing.T) {
	var _ ze.ConfigProvider = (*config.Provider)(nil)
}

// writeJSON is a test helper that writes data as JSON to a file.
func writeJSON(t *testing.T, path string, data map[string]any) {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
