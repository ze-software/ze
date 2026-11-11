//go:build linux

package systemd

import (
	"os"
	"strings"
	"testing"
)

func TestNativeServiceRefusesUntrustedExecutablePathsBeforeMutation(t *testing.T) {
	for _, kind := range []string{"group-writable-ancestor", "world-writable-ancestor", "unowned-ancestor", "writable-executable", "unowned-executable"} {
		t.Run(kind, func(t *testing.T) {
			fake := newFakeServiceOps()
			binary := "/usr/local/bin/ze"
			fake.files[binary] = "old-binary"
			if kind == "group-writable-ancestor" {
				fake.statModes["/usr/local"] = os.ModeDir | 0o775
			}
			if kind == "world-writable-ancestor" {
				fake.statModes["/usr/local"] = os.ModeDir | 0o777
			}
			if kind == "unowned-ancestor" {
				fake.statUIDs["/usr/local"] = 1000
			}
			if kind == "writable-executable" {
				fake.statModes[binary] = 0o777
			}
			if kind == "unowned-executable" {
				fake.statUIDs[binary] = 1000
			}
			rt, _, _ := newTestRuntime(fake)
			if err := (nativeService{rt: rt}).Preflight(binary, "/etc/ze"); err == nil {
				t.Fatal("untrusted executable accepted")
			}
			if len(fake.runCalls) != 0 || len(fake.chownCalls) != 0 {
				t.Fatalf("preflight mutated system: %v %v", fake.runCalls, fake.chownCalls)
			}
			if fake.files[binary] != "old-binary" {
				t.Fatal("preflight changed executable")
			}
		})
	}
}

func TestNativeServiceRefusesRuntimePolicyAndOrphanDropins(t *testing.T) {
	for _, kind := range []string{"runtime-unit", "runtime-mask", "orphan-etc-dropin", "orphan-runtime-dropin"} {
		t.Run(kind, func(t *testing.T) {
			fake := newFakeServiceOps()
			if kind == "runtime-unit" {
				fake.files["/run/systemd/system/ze.service"] = "runtime policy"
			}
			if kind == "runtime-mask" {
				fake.files["/run/systemd/system/ze.service"] = "/dev/null"
				fake.statModes["/run/systemd/system/ze.service"] = os.ModeSymlink | 0o777
			}
			if kind == "orphan-etc-dropin" {
				fake.directories[defaultUnitPath+".d"] = []os.DirEntry{fakeDirEntry{name: "owner.conf"}}
			}
			if kind == "orphan-runtime-dropin" {
				fake.directories["/run/systemd/system/ze.service.d"] = []os.DirEntry{fakeDirEntry{name: "owner.conf"}}
			}
			rt, _, _ := newTestRuntime(fake)
			if err := (nativeService{rt: rt}).Preflight("/usr/local/bin/ze", "/etc/ze"); err == nil {
				t.Fatal("existing runtime/drop-in policy was ignored")
			}
			if len(fake.runCalls) != 0 {
				t.Fatalf("preflight changed service: %v", fake.runCalls)
			}
		})
	}
}

func TestNativeServiceRefusesIndirectConfigAndExecutionOwnership(t *testing.T) {
	for _, directive := range []string{"EnvironmentFile=/etc/other-ze", "UnsetEnvironment=ZE_CONFIG_DIR", "RootDirectory=/other-root", "RootImage=/other-root.img", "BindPaths=/other:/etc/ze", "PassEnvironment=ZE_CONFIG_DIR"} {
		t.Run(directive, func(t *testing.T) {
			fake := newFakeServiceOps()
			fake.files[defaultUnitPath] = buildUnitFile(unitSpec{BinaryPath: "/usr/local/bin/ze", ConfigDir: "/etc/ze"}) + "\n[Service]\n" + directive + "\n"
			rt, _, _ := newTestRuntime(fake)
			if err := (nativeService{rt: rt}).Preflight("/usr/local/bin/ze", "/etc/ze"); err == nil {
				t.Fatal("indirect service owner accepted")
			}
			if len(fake.runCalls) != 0 {
				t.Fatalf("preflight changed policy: %v", fake.runCalls)
			}
		})
	}
}

func TestNativeServicePreservesCompatibleDropin(t *testing.T) {
	fake := newFakeServiceOps()
	fake.files[defaultUnitPath] = buildUnitFile(unitSpec{BinaryPath: "/usr/local/bin/ze", ConfigDir: "/etc/ze"})
	path := defaultUnitPath + ".d/10-restart.conf"
	original := "[Service]\nRestartSec=17\n"
	fake.directories[defaultUnitPath+".d"] = []os.DirEntry{fakeDirEntry{name: "10-restart.conf"}}
	fake.files[path] = original
	rt, _, _ := newTestRuntime(fake)
	if err := (nativeService{rt: rt}).Preflight("/usr/local/bin/ze", "/etc/ze"); err != nil {
		t.Fatal(err)
	}
	if fake.files[path] != original {
		t.Fatal("compatible drop-in was rewritten")
	}
}

func TestNativeServiceRefusesUnknownEffectiveFragment(t *testing.T) {
	fake := newFakeServiceOps()
	fake.outputs["systemctl show --property=LoadState --property=FragmentPath --property=DropInPaths ze.service"] = "LoadState=loaded\nFragmentPath=/run/systemd/transient/ze.service\nDropInPaths=\n"
	rt, _, _ := newTestRuntime(fake)
	if err := (nativeService{rt: rt}).Preflight("/usr/local/bin/ze", "/etc/ze"); err == nil {
		t.Fatal("a higher-priority effective unit was ignored")
	}
	if len(fake.runCalls) != 0 {
		t.Fatalf("effective conflict mutated service: %v", fake.runCalls)
	}
}

func TestNativeServiceRefusesExplicitAlternateConfigInExecStart(t *testing.T) {
	fake := newFakeServiceOps()
	fake.files[defaultUnitPath] = strings.ReplaceAll(buildUnitFile(unitSpec{BinaryPath: "/usr/local/bin/ze", ConfigDir: "/etc/ze"}), " start'", " start /etc/other/ze.conf'")
	rt, _, _ := newTestRuntime(fake)
	if err := (nativeService{rt: rt}).Preflight("/usr/local/bin/ze", "/etc/ze"); err == nil {
		t.Fatal("alternate explicit config in start command was accepted")
	}
}

func TestNativeServicePreservesEffectiveGlobalTuning(t *testing.T) {
	fake := newFakeServiceOps()
	fake.files[defaultUnitPath] = buildUnitFile(unitSpec{BinaryPath: "/usr/local/bin/ze", ConfigDir: "/etc/ze"})
	path := "/usr/lib/systemd/system/service.d/10-timeout.conf"
	original := "[Service]\nTimeoutStopSec=95s\n"
	fake.files[path] = original
	fake.outputs["systemctl show --property=LoadState --property=FragmentPath --property=DropInPaths ze.service"] = "LoadState=loaded\nFragmentPath=" + defaultUnitPath + "\nDropInPaths=" + path + "\n"
	rt, _, _ := newTestRuntime(fake)
	if err := (nativeService{rt: rt}).Preflight("/usr/local/bin/ze", "/etc/ze"); err != nil {
		t.Fatal(err)
	}
	if fake.files[path] != original {
		t.Fatal("global tuning changed")
	}
}
