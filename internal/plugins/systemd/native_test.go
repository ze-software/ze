package systemd

import (
	"reflect"
	"strings"
	"testing"
)

func TestNativeServicePreflightRejectsConflictingUnit(t *testing.T) {
	fake := newFakeServiceOps()
	fake.files[defaultUnitPath] = buildUnitFile(unitSpec{BinaryPath: "/opt/other/bin/ze", ConfigDir: "/etc/ze"})
	rt, _, _ := newTestRuntime(fake)
	service := nativeService{rt: rt}
	if err := service.Preflight("/usr/local/bin/ze", "/etc/ze"); err == nil {
		t.Fatal("different binary admitted")
	}
	if len(fake.runCalls) != 0 {
		t.Fatalf("preflight mutated service: %v", fake.runCalls)
	}
}

func TestNativeServiceReusesCompatibleAdministratorUnit(t *testing.T) {
	fake := newFakeServiceOps()
	original := strings.ReplaceAll(buildUnitFile(unitSpec{BinaryPath: "/usr/local/bin/ze", ConfigDir: "/etc/ze"}), "RestartSec=5", "RestartSec=12")
	fake.files[defaultUnitPath] = original
	rt, _, stderr := newTestRuntime(fake)
	service := nativeService{rt: rt}
	if err := service.Preflight("/usr/local/bin/ze", "/etc/ze"); err != nil {
		t.Fatal(err)
	}
	if code := service.Install("/usr/local/bin/ze", "/etc/ze"); code != 0 {
		t.Fatalf("install=%d: %s", code, stderr)
	}
	if fake.files[defaultUnitPath] != original {
		t.Fatal("administrator unit was rewritten")
	}
	if !reflect.DeepEqual(fake.runCalls, []string{"systemctl start ze.service"}) {
		t.Fatalf("unexpected reuse actions: %v", fake.runCalls)
	}
}

func TestNativeServiceRefusesPackagedVendorUnit(t *testing.T) {
	fake := newFakeServiceOps()
	fake.files["/usr/lib/systemd/system/ze.service"] = "vendor unit"
	rt, _, _ := newTestRuntime(fake)
	service := nativeService{rt: rt}
	if err := service.Preflight("/usr/local/bin/ze", "/etc/ze"); err == nil {
		t.Fatal("package service admitted")
	}
	if len(fake.runCalls) != 0 {
		t.Fatalf("package preflight mutated: %v", fake.runCalls)
	}
}

func TestNativeServicePreservesInactiveStateDuringUpgrade(t *testing.T) {
	fake := newFakeServiceOps()
	original := buildUnitFile(unitSpec{BinaryPath: "/usr/local/bin/ze", ConfigDir: "/etc/ze"})
	fake.files[defaultUnitPath] = original
	fake.outputs["systemctl show --property=ActiveState --value ze.service"] = "inactive\n"
	rt, _, stderr := newTestRuntime(fake)
	service := nativeService{rt: rt}
	stopped, err := service.Stop("/usr/local/bin/ze", "/etc/ze")
	if err != nil {
		t.Fatal(err)
	}
	if code := stopped.Install(); code != 0 {
		t.Fatalf("inactive upgrade=%d: %s", code, stderr)
	}
	if len(fake.runCalls) != 0 {
		t.Fatalf("inactive service policy changed: %v", fake.runCalls)
	}
	if fake.files[defaultUnitPath] != original {
		t.Fatal("inactive upgrade changed unit")
	}
}

func TestNativeServiceFailureRecoveryRestartsOnlyPriorActiveService(t *testing.T) {
	fake := newFakeServiceOps()
	fake.files[defaultUnitPath] = buildUnitFile(unitSpec{BinaryPath: "/usr/local/bin/ze", ConfigDir: "/etc/ze"})
	fake.outputs["systemctl show --property=ActiveState --value ze.service"] = "active\n"
	rt, _, _ := newTestRuntime(fake)
	stopped, err := (nativeService{rt: rt}).Stop("/usr/local/bin/ze", "/etc/ze")
	if err != nil {
		t.Fatal(err)
	}
	if err := stopped.Restore(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fake.runCalls, []string{"systemctl stop ze.service", "systemctl start ze.service"}) {
		t.Fatalf("recovery actions=%v", fake.runCalls)
	}
}
