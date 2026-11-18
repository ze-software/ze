// Design: docs/architecture/config/yang-config-design.md — module registration ownership

package yang_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

const readonlyModuleName = "ze-probe-readonly"

const readonlyModuleText = `module ze-probe-readonly {
    namespace "urn:ze:probe-readonly";
    prefix readonly;
    leaf trusted { type string; }
}`

// TestModuleRegistrarHasNoWritableField protects registration provenance at the
// Go API boundary. Method: reflection distinguishes an exported field from a
// private field or read-only method without depending on either representation.
func TestModuleRegistrarHasNoWritableField(t *testing.T) {
	field, found := reflect.TypeFor[yang.Module]().FieldByName("Registrar")
	if !found {
		return
	}
	if field.IsExported() {
		t.Fatal("Module.Registrar is a writable exported field; a caller can forge registration provenance")
	}
}

// TestModulesSnapshotPreservesRegistry proves a returned record cannot change
// the registered module set. Method: edit its name, text, or complete record,
// then compare the registry, registrar, cached schema and a fresh loader.
func TestModulesSnapshotPreservesRegistry(t *testing.T) {
	for _, change := range []struct {
		name   string
		mutate func(*yang.Module)
	}{
		{
			name: "name",
			mutate: func(module *yang.Module) {
				module.Name = "ze-forged-readonly.yang"
			},
		},
		{
			name: "content",
			mutate: func(module *yang.Module) {
				module.Content = `module ze-probe-readonly {
    namespace "urn:ze:probe-readonly";
    prefix readonly;
    leaf forged { type string; }
}`
			},
		},
		{
			name: "whole-record",
			mutate: func(module *yang.Module) {
				*module = yang.Module{Name: readonlyModuleName + ".yang", Content: readonlyModuleText}
			},
		},
	} {
		t.Run(change.name, func(t *testing.T) {
			t.Cleanup(yang.RegisterModuleForTest(readonlyModuleName+".yang", readonlyModuleText))
			want := slices.Clone(yang.Modules())
			shared, err := yang.DefaultLoader()
			if err != nil {
				t.Fatalf("DefaultLoader before snapshot mutation: %v", err)
			}
			snapshot := yang.Modules()
			index := slices.IndexFunc(snapshot, func(module yang.Module) bool {
				return module.Name == readonlyModuleName+".yang"
			})
			if index < 0 {
				t.Fatal("registered probe is absent from Modules")
			}
			change.mutate(&snapshot[index])
			if !slices.Equal(yang.Modules(), want) {
				t.Error("editing the Modules snapshot changed the registered module set")
			}
			assertReadonlyModuleRegistrar(t, readonlyModuleName,
				"github.com/ze-software/ze/internal/component/config/yang_test")
			if _, found := yang.ModuleRegistrar("ze-forged-readonly"); found {
				t.Error("snapshot mutation introduced a registrar for an unregistered module")
			}
			again, err := yang.DefaultLoader()
			if err != nil {
				t.Fatalf("DefaultLoader after snapshot mutation: %v", err)
			}
			if again != shared {
				t.Error("editing a snapshot changed the cached module-set generation")
			}
			loader := yang.NewLoader()
			if err := loader.LoadEmbedded(); err != nil {
				t.Fatalf("LoadEmbedded after snapshot mutation: %v", err)
			}
			if err := loader.LoadRegistered(); err != nil {
				t.Fatalf("LoadRegistered after snapshot mutation: %v", err)
			}
			resolved, err := loader.Resolve()
			if err != nil {
				t.Fatalf("Resolve after snapshot mutation: %v", err)
			}
			entry := resolved.GetEntry(readonlyModuleName)
			if entry == nil {
				t.Fatal("fresh loader lost the registered probe")
			}
			if entry.Dir["trusted"] == nil {
				t.Error("snapshot mutation removed the registered trusted leaf from a fresh loader")
			}
			if entry.Dir["forged"] != nil {
				t.Error("snapshot mutation added an unregistered forged leaf to a fresh loader")
			}
		})
	}
}

// TestModuleRegistrationRetainsProvenance proves both registration routes stamp
// the real caller and advance the schema generation. Method: register two valid
// modules inside one restore boundary, then restore and check the original set.
func TestModuleRegistrationRetainsProvenance(t *testing.T) {
	assertReadonlyModuleRegistrar(t, "ze-system-conf",
		"github.com/ze-software/ze/internal/component/config/system/yang")
	if _, found := yang.ModuleRegistrar(readonlyModuleName); found {
		t.Fatal("the test probe was registered before this test")
	}
	before := slices.Clone(yang.Modules())
	initial, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader before registration: %v", err)
	}
	restore := yang.RegisterModuleForTest(readonlyModuleName+".yang", readonlyModuleText)
	t.Cleanup(restore)
	assertReadonlyModuleRegistrar(t, readonlyModuleName,
		"github.com/ze-software/ze/internal/component/config/yang_test")
	probed, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader after test registration: %v", err)
	}
	if probed == initial {
		t.Error("RegisterModuleForTest did not change the module-set generation")
	}
	if probed.GetModule(readonlyModuleName) == nil {
		t.Error("test registration did not reach DefaultLoader")
	}
	yang.RegisterModule("ze-probe-readonly-direct.yang", `module ze-probe-readonly-direct {
    namespace "urn:ze:probe-readonly-direct";
    prefix direct;
    leaf trusted { type string; }
}`)
	assertReadonlyModuleRegistrar(t, "ze-probe-readonly-direct",
		"github.com/ze-software/ze/internal/component/config/yang_test")
	direct, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader after direct registration: %v", err)
	}
	if direct == probed {
		t.Error("RegisterModule did not change the module-set generation")
	}
	if direct.GetModule("ze-probe-readonly-direct") == nil {
		t.Error("direct registration did not reach DefaultLoader")
	}
	restore()
	if !slices.Equal(yang.Modules(), before) {
		t.Error("restore did not recover the complete original module registry")
	}
	for _, name := range []string{readonlyModuleName, "ze-probe-readonly-direct"} {
		if _, found := yang.ModuleRegistrar(name); found {
			t.Errorf("restore left registrar provenance for %s", name)
		}
	}
	restored, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader after restore: %v", err)
	}
	if restored == direct {
		t.Error("restore reused the schema of the temporary module set")
	}
	for _, name := range []string{readonlyModuleName, "ze-probe-readonly-direct"} {
		if restored.GetModule(name) != nil {
			t.Errorf("restore left %s in DefaultLoader", name)
		}
		if initial.GetModule(name) != nil {
			t.Errorf("registration mutated the initial schema to include %s", name)
		}
	}
}

func assertReadonlyModuleRegistrar(t *testing.T, module, want string) {
	t.Helper()
	got, found := yang.ModuleRegistrar(module)
	if !found {
		t.Errorf("ModuleRegistrar(%q) found no registering package", module)
		return
	}
	if got != want {
		t.Errorf("ModuleRegistrar(%q) = %q, want %q", module, got, want)
	}
}
