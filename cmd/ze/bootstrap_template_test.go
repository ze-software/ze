// Design: plan/spec-fixit-appliance-evidence-config.md — template becomes effective config

//go:build ze_core

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// VALIDATES: bootstrapConfigFromTemplate makes a build-time template the
// effective boot config when no active config exists -- exactly the state left
// by `ze init --seed` + the template write in the appliance build. AC-2 of
// spec-fixit-appliance-evidence-config: the appliance's web+l2tp template is
// applied, not shadowed. (Full end-to-end proof is the L2TP evidence test, AC-3.)
// PREVENTS: a regression where init's active config shadows the template so
// web/l2tp never start on the appliance.
func TestBootstrapConfigFromTemplateAppliesWebL2TP(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewBlob(filepath.Join(dir, "database.zefs"), dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test

	template := "set environment web enabled true\n" +
		"set environment web server default ip 0.0.0.0\n" +
		"set environment ssh enabled true\n" +
		"set interface dhcp-auto true\n" +
		"set l2tp enabled true\n" +
		"set environment l2tp server main port 1701\n"
	if err := store.WriteFile(zefs.KeyFileTemplate.Key("ze.conf"), []byte(template), 0); err != nil {
		t.Fatalf("write template: %v", err)
	}

	// Precondition: no file/active/ze.conf -- the --seed seed-DB state.
	if store.Exists("ze.conf") {
		t.Fatal("precondition: active config must be absent (would shadow the template)")
	}

	if !bootstrapConfigFromTemplate(store, "ze.conf") {
		t.Fatal("bootstrapConfigFromTemplate returned false; want true (template present)")
	}

	got, err := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	if err != nil {
		t.Fatalf("read effective active config: %v", err)
	}
	active := string(got)
	for _, want := range []string{
		"set environment web enabled true",
		"set l2tp enabled true",
		"set environment l2tp server main port 1701",
	} {
		if !strings.Contains(active, want) {
			t.Errorf("effective active config missing %q; the template was shadowed or dropped\n--- active ---\n%s", want, active)
		}
	}
}
