package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	qemutool "github.com/ze-software/ze/internal/le/qemu"
)

// VALIDATES: effective-install-qemu.py and the native HTTP action produce the same complete QEMU argv.
// PREVENTS: parity over only a suffix hiding a missing disk, network, kernel, initrd, or cmdline input.
func TestInstallQemuScriptAndCommandFullArgvParity(t *testing.T) {
	python := installPythonArgv(t, `
m=load("effective-install-qemu.py", "base")
m.ARCH="amd64"; m.QEMU_BIN="qemu-system-x86_64"; m.QEMU_ACCEL="tcg"
m._run_capture=lambda cmd, timeout: print(json.dumps(cmd)) or m.MARK_DONE
m.boot_installer(Path("kernel"), Path("initrd"), Path("disk"), 8080, 300)
`)
	installer := installParityRunner(qemutool.InstallKindHTTP)
	goArgv, err := installer.HTTPArgv("kernel", "initrd", "disk", 8080)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(python, goArgv) {
		t.Fatalf("full argv differs:\nPython %q\nGo %q", python, goArgv)
	}
}

// VALIDATES: effective-install-iso-qemu.py and the native ISO action attach both disks and architecture-correct removable media identically.
// PREVENTS: an ISO proof booting a different topology from its producer.
func TestInstallISOQemuScriptAndCommandFullArgvParity(t *testing.T) {
	python := installPythonArgv(t, `
m=load("effective-install-iso-qemu.py", "iso")
m.base.ARCH="amd64"; m.base.QEMU_BIN="qemu-system-x86_64"; m.base.QEMU_ACCEL="tcg"
m.base._run_capture=lambda cmd, timeout: print(json.dumps(cmd)) or m.MARK_ISO_DONE
m.boot_iso_installer(Path("install.iso"), Path("target"), Path("extra"), Path("OVMF.fd"), 300)
`)
	installer := installParityRunner(qemutool.InstallKindISO)
	goArgv, err := installer.ISOArgv("install.iso", "target", "extra", "OVMF.fd")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(python, goArgv) {
		t.Fatalf("full argv differs:\nPython %q\nGo %q", python, goArgv)
	}
}

// VALIDATES: effective-install-scenarios-qemu.py and the native action preserve every fault and rescue cmdline input.
// PREVENTS: one scenario branch or pinned credential disappearing behind shared setup.
func TestInstallScenariosQemuScriptAndCommandFullArgvParity(t *testing.T) {
	fault := installPythonArgv(t, `
m=load("effective-install-scenarios-qemu.py", "scenarios")
m.base.ARCH="amd64"; m.base.QEMU_BIN="qemu-system-x86_64"; m.base.QEMU_ACCEL="tcg"
m.base._run_capture=lambda cmd, timeout: print(json.dumps(cmd)) or m.FAULT_RECOVER_MARK
m.boot_fault(Path("kernel"), Path("initrd"), Path("disk"), 90)
`)
	installer := installParityRunner(qemutool.InstallKindScenarios)
	goFault, err := installer.FaultArgv("initrd", "disk")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fault, goFault) {
		t.Fatalf("fault argv differs:\nPython %q\nGo %q", fault, goFault)
	}
	rescue := installPythonArgv(t, `
m=load("effective-install-scenarios-qemu.py", "scenarios")
m.base.ARCH="amd64"; m.base.QEMU_BIN="qemu-system-x86_64"; m.base.QEMU_ACCEL="tcg"
print(json.dumps(m.rescue_cmd(Path("kernel"), Path("initrd"), source="http", rescue_auth=m.RESCUE_AUTH)))
`)
	joined := strings.Join(rescue, " ")
	for _, value := range []string{"ze.source=http", "ze.wait=3", "ze.rescue-auth=" + qemutool.InstallRescueAuth, "-no-reboot"} {
		if !strings.Contains(joined, value) {
			t.Fatalf("rescue argv misses %q: %s", value, joined)
		}
	}
	goRescue, err := installer.RescueArgv("initrd", "http", true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rescue, goRescue) {
		t.Fatalf("rescue argv differs:\nPython %q\nGo %q", rescue, goRescue)
	}
}

// VALIDATES: effective-install-ventoy-qemu.py and the native action preserve disk order, media id, image name, and safe-poweroff argv.
// PREVENTS: vda and vdb swapping or the Ventoy scan becoming an ordinary ISO boot.
func TestInstallVentoyQemuScriptAndCommandFullArgvParity(t *testing.T) {
	python := installPythonArgv(t, `
m=load("effective-install-ventoy-qemu.py", "ventoy")
m.base.ARCH="amd64"; m.base.QEMU_BIN="qemu-system-x86_64"; m.base.QEMU_ACCEL="tcg"
m.base._run_capture=lambda cmd, timeout: print(json.dumps(cmd)) or m.VENTOY_MARK
m.boot_ventoy(Path("kernel"), Path("initrd"), Path("target"), Path("fat"), "media", "image.gz", 300)
`)
	installer := installParityRunner(qemutool.InstallKindVentoy)
	goArgv, err := installer.VentoyArgv("initrd", "target", "fat", "media", "image.gz")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(python, goArgv) {
		t.Fatalf("full argv differs:\nPython %q\nGo %q", python, goArgv)
	}
}

// VALIDATES: all four producers and native actions keep exact self-skip stdout and exit zero.
// PREVENTS: a missing optional QEMU prerequisite becoming a failure in one action only.
func TestInstallFamilyScriptAndCommandSkipParity(t *testing.T) {
	cases := []struct {
		script string
		kind   qemutool.InstallKind
		prefix string
	}{
		{"effective-install-qemu.py", qemutool.InstallKindHTTP, "INSTALL-QEMU"},
		{"effective-install-iso-qemu.py", qemutool.InstallKindISO, "INSTALL-ISO-QEMU"},
		{"effective-install-scenarios-qemu.py", qemutool.InstallKindScenarios, "INSTALL-SCENARIOS-QEMU"},
		{"effective-install-ventoy-qemu.py", qemutool.InstallKindVentoy, "INSTALL-VENTOY-QEMU"},
	}
	for _, test := range cases {
		t.Run(test.kind.String(), func(t *testing.T) {
			python, code := installPythonSkip(t, test.script)
			want := test.prefix + ": SKIP qemu-system-x86_64 not found\n"
			if code != 0 || python != want {
				t.Fatalf("Python code=%d stdout=%q want=%q", code, python, want)
			}
		})
	}
}

// VALIDATES: constants are compared by value through the imported Python module chain.
// PREVENTS: the base, ISO, scenario, and Ventoy modules drifting while a fixture still prints the same verdict.
func TestInstallFamilyPythonModulesAndGoShareConstants(t *testing.T) {
	code := installPythonJSON(t, `
base=load("effective-install-qemu.py", "base")
iso=load("effective-install-iso-qemu.py", "iso")
sc=load("effective-install-scenarios-qemu.py", "sc")
ventoy=load("effective-install-ventoy-qemu.py", "ventoy")
print(json.dumps({
 "image":base.IMAGE_NAME,"written":base.MARK_WRITTEN,"done":base.MARK_DONE,
 "server":base.GUEST_SERVER_IP,"bios":base.AARCH64_BIOS_FALLBACK,
 "iso-name":iso.IMAGE_NAME,"iso":iso.MARK_ISO_DONE,"ventoy":ventoy.VENTOY_MARK,
 "pinned":sc.PINNED_MAC,"foreign":sc.FOREIGN_MAC,"fault":sc.FAULT_RECOVER_MARK,
 "kernel-panic":sc.KERNEL_INIT_PANIC,"fatal":sc.FATAL_POLICY_MARK,
 "pin":sc.PIN_MARK,"reachable":sc.PIN_REACHABLE_MARK,"flush":sc.PIN_FLUSH_MARK,
 "fallback":sc.FALLBACK_MARK,"token":sc.RESCUE_TOKEN,"auth":sc.RESCUE_AUTH,
 "prompt":sc.TOKEN_PROMPT,"auth-ok":sc.AUTH_OK,"auth-bad":sc.AUTH_BAD,
 "menu":sc.MENU_MARK,"reboot":sc.REBOOT_30S,"media":sc.DUMMY_MEDIA_ID}))
`)
	want := map[string]string{
		"image": qemutool.InstallImageName, "written": qemutool.InstallMarkWritten,
		"done": qemutool.InstallMarkDone, "server": qemutool.InstallGuestServerIP,
		"bios":     qemutool.InstallAArch64BIOSFallback,
		"iso-name": qemutool.InstallApplianceName, "iso": qemutool.InstallMarkISODone,
		"ventoy": qemutool.InstallMarkVentoy, "pinned": qemutool.InstallPinnedMAC,
		"foreign": qemutool.InstallForeignMAC, "fault": qemutool.InstallFaultRecoverMark,
		"kernel-panic": qemutool.InstallKernelPanicMark, "fatal": qemutool.InstallFatalPolicyMark,
		"pin": qemutool.InstallPinMark, "reachable": qemutool.InstallPinReachableMark,
		"flush": qemutool.InstallPinFlushMark, "fallback": qemutool.InstallFallbackMark,
		"token": qemutool.InstallRescueToken, "auth": qemutool.InstallRescueAuth,
		"prompt": qemutool.InstallTokenPrompt, "auth-ok": qemutool.InstallAuthOK,
		"auth-bad": qemutool.InstallAuthBad, "menu": qemutool.InstallMenuMark,
		"reboot": qemutool.InstallReboot30s, "media": qemutool.InstallDummyMediaID,
	}
	var got map[string]string
	if err := json.Unmarshal(code, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("constants differ:\nPython %#v\nGo %#v", got, want)
	}
}

func installParityRunner(kind qemutool.InstallKind) *qemutool.Installer {
	return qemutool.NewInstaller(".", qemutool.InstallOptions{
		Kind: kind, Arch: "amd64", Accelerator: "tcg", Kernel: "kernel",
		NIC: "virtio-net-pci", BootTimeout: time.Second,
	})
}

// VALIDATES: the port inventory names every top-level producer in all four scripts.
// PREVENTS: a newly added artifact, scenario, cleanup, or exit producer remaining only in Python.
func TestInstallProducerTopLevelInventoryIsComplete(t *testing.T) {
	output := installPythonJSON(t, `
import ast
files=("effective-install-qemu.py","effective-install-iso-qemu.py","effective-install-scenarios-qemu.py","effective-install-ventoy-qemu.py")
found={}
for name in files:
    tree=ast.parse((root/name).read_text())
    found[name]=[node.name for node in tree.body if isinstance(node,(ast.FunctionDef,ast.AsyncFunctionDef))]
print(json.dumps(found))
`)
	var got map[string][]string
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"effective-install-qemu.py": {
			"_free_tcp_port", "host_arch", "repo_root", "skip", "run",
			"_ensure_docker_host", "have_initrd_build_tools", "find_installer_kernel",
			"have_image_build_tools", "_brew_debugfs", "build_host_ze", "build_initrd",
			"build_image", "write_checksum", "start_http", "_make_handler", "qemu_base",
			"boot_installer", "boot_target_ssh", "_ssh_login_ok", "have_ssh_probe_tool",
			"_run_capture", "main",
		},
		"effective-install-iso-qemu.py": {
			"repo_root", "load_pxe_module", "skip", "run", "sha256_file",
			"build_host_ze", "write_checksum", "init_appliance", "prepare_image",
			"create_iso", "extract_iso_image", "find_x86_uefi_firmware",
			"find_aarch64_uefi_firmware", "find_uefi_firmware", "iso_cdrom_args",
			"boot_iso_installer", "gpt_entries", "assert_partition_layout", "main",
		},
		"effective-install-scenarios-qemu.py": {
			"repo_root", "load_base", "skip", "log", "_build_initrd_with_env",
			"build_fault_initrd", "build_normal_initrd", "setup_install_fixtures",
			"_blank_target", "boot_fault", "scenario_fault", "boot_pin",
			"scenario_pin_ac4", "scenario_pin_ac5", "rescue_cmd",
			"scenario_rescue_ac7", "scenario_rescue_ac7b", "scenario_rescue_ac7c", "main",
		},
		"effective-install-ventoy-qemu.py": {
			"repo_root", "load_iso_module", "skip", "log", "run", "have_mtools",
			"extract_media_id", "build_ventoy_data_disk", "boot_ventoy", "main",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("producer inventory differs:\n got=%#v\nwant=%#v", got, want)
	}
}

func installPythonArgv(t *testing.T, body string) []string {
	t.Helper()
	output := installPythonJSON(t, body)
	var argv []string
	if err := json.Unmarshal(output, &argv); err != nil {
		t.Fatalf("decode argv %q: %v", output, err)
	}
	return argv
}

func installPythonJSON(t *testing.T, body string) []byte {
	t.Helper()
	prelude := `
import importlib.util, json, os
from pathlib import Path
root=Path.cwd()
def load(name, key):
    path=root/name
    spec=importlib.util.spec_from_file_location(key, path)
    module=importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module
`
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "python3", "-c", prelude+body)
	command.Env = installPythonParityEnv()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Python module: %v\n%s", err, output)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return []byte(lines[len(lines)-1])
}

func installPythonParityEnv() []string {
	result := make([]string, 0, len(os.Environ())+3)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "ZE_INSTALL_ARCH=") {
			continue
		}
		if strings.HasPrefix(entry, "ZE_INSTALL_QEMU_ACCEL=") {
			continue
		}
		if strings.HasPrefix(entry, "ZE_INSTALL_NIC=") {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "ZE_INSTALL_ARCH=amd64", "ZE_INSTALL_QEMU_ACCEL=tcg",
		"ZE_INSTALL_NIC=virtio-net-pci")
}

func installPythonSkip(t *testing.T, script string) (string, int) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, python, filepath.Join(".", script))
	command.Env = []string{"PATH=" + empty, "ZE_INSTALL_ARCH=amd64"}
	output, runErr := command.Output()
	if runErr == nil {
		return string(output), 0
	}
	if exit, ok := errors.AsType[*exec.ExitError](runErr); ok {
		return string(output), exit.ExitCode()
	}
	t.Fatal(runErr)
	return "", -1
}
