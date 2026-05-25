package service

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestServiceInstallGeneratesUnit(t *testing.T) {
	// VALIDATES: AC-1 ze service install writes the unit and enables ze.service.
	// VALIDATES: AC-9/AC-11 install creates ze account and chowns config dir/database.zefs.
	// PREVENTS: wiring the CLI to a partial installer that never reaches systemd or ownership setup.
	fake := newFakeServiceOps()
	rt, stdout, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install"})
	if code != 0 {
		t.Fatalf("install exit code = %d, stderr=%s", code, stderr.String())
	}

	unit := fake.files[defaultUnitPath]
	assertContains(t, unit, "ExecStart=/usr/local/bin/ze start")
	assertContains(t, unit, "WorkingDirectory=/etc/ze")
	assertCalls(t, fake.runCalls,
		"groupadd --system ze",
		"useradd --system --no-create-home --gid ze --home-dir /nonexistent --shell /usr/sbin/nologin ze",
		"systemctl daemon-reload",
		"systemctl enable ze.service",
	)
	assertCalls(t, fake.chownCalls,
		"/etc/ze ze:ze",
		"/etc/ze/database.zefs ze:ze",
	)
	assertContains(t, stderr.String(), "/run/ze/ze.socket")
	if stdout.Len() != 0 {
		t.Fatalf("install wrote unexpected stdout: %q", stdout.String())
	}
}

func TestServiceInstallStartRunsSystemctlStart(t *testing.T) {
	// VALIDATES: AC-2 ze service install --start starts ze.service after enabling it.
	// PREVENTS: accepting --start while only installing the unit file.
	fake := newFakeServiceOps()
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install", "--start"})
	if code != 0 {
		t.Fatalf("install --start exit code = %d, stderr=%s", code, stderr.String())
	}

	assertCalls(t, fake.runCalls, "systemctl start ze.service")
}

func TestServiceUninstallRemovesUnit(t *testing.T) {
	// VALIDATES: AC-3 ze service uninstall stops, disables, removes unit, and reloads systemd.
	// PREVENTS: leaving a stale enabled unit after uninstall.
	fake := newFakeServiceOps()
	fake.files[defaultUnitPath] = "unit"
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"uninstall"})
	if code != 0 {
		t.Fatalf("uninstall exit code = %d, stderr=%s", code, stderr.String())
	}

	if _, ok := fake.files[defaultUnitPath]; ok {
		t.Fatalf("unit file still exists after uninstall")
	}
	assertCalls(t, fake.runCalls,
		"systemctl stop ze.service",
		"systemctl disable ze.service",
		"systemctl daemon-reload",
	)
}

func TestServiceUninstallPurgeRemovesAccount(t *testing.T) {
	// VALIDATES: --purge removes the ze user and group after uninstalling the unit.
	// PREVENTS: stale system accounts remaining after full service removal.
	fake := newFakeServiceOps()
	fake.files[defaultUnitPath] = "unit"
	fake.outputs["getent passwd ze"] = "ze:x:999:999::/nonexistent:/usr/sbin/nologin"
	fake.outputs["getent group ze"] = "ze:x:999:"
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"uninstall", "--purge"})
	if code != 0 {
		t.Fatalf("uninstall --purge exit code = %d, stderr=%s", code, stderr.String())
	}

	assertCalls(t, fake.runCalls, "userdel ze", "groupdel ze")
	assertContains(t, stderr.String(), "user ze removed")
	assertContains(t, stderr.String(), "group ze removed")
}

func TestServiceUninstallPurgeSkipsMissingAccount(t *testing.T) {
	// VALIDATES: --purge succeeds when the ze user/group do not exist.
	// PREVENTS: uninstall --purge failing on a host where the account was already removed.
	fake := newFakeServiceOps()
	fake.files[defaultUnitPath] = "unit"
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"uninstall", "--purge"})
	if code != 0 {
		t.Fatalf("uninstall --purge no-account exit code = %d, stderr=%s", code, stderr.String())
	}

	for _, call := range fake.runCalls {
		if strings.Contains(call, "userdel") || strings.Contains(call, "groupdel") {
			t.Fatalf("should not call user/group delete when account missing: %s", call)
		}
	}
}

func TestServiceUninstallPurgeSkipsGroupOnUserdelFailure(t *testing.T) {
	// VALIDATES: --purge skips groupdel when userdel fails.
	// PREVENTS: groupdel failing with "cannot remove primary group of user" after userdel failure.
	fake := newFakeServiceOps()
	fake.files[defaultUnitPath] = "unit"
	fake.outputs["getent passwd ze"] = "ze:x:999:999::/nonexistent:/usr/sbin/nologin"
	fake.outputs["getent group ze"] = "ze:x:999:"
	fake.runErrors["userdel ze"] = errors.New("userdel: user ze is currently used by process 1234")
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"uninstall", "--purge"})
	if code != 1 {
		t.Fatalf("uninstall --purge userdel-fail exit code = %d, stderr=%s", code, stderr.String())
	}

	assertContains(t, stderr.String(), "skipping group removal")
	for _, call := range fake.runCalls {
		if strings.Contains(call, "groupdel") {
			t.Fatalf("should not call groupdel when userdel failed: %s", call)
		}
	}
}

func TestServiceUninstallPurgeWithoutUnit(t *testing.T) {
	// VALIDATES: --purge still removes user/group even when the unit is already gone.
	// PREVENTS: requiring a unit file to be present before cleaning up the account.
	fake := newFakeServiceOps()
	fake.outputs["getent passwd ze"] = "ze:x:999:999::/nonexistent:/usr/sbin/nologin"
	fake.outputs["getent group ze"] = "ze:x:999:"
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"uninstall", "--purge"})
	if code != 0 {
		t.Fatalf("uninstall --purge no-unit exit code = %d, stderr=%s", code, stderr.String())
	}

	assertCalls(t, fake.runCalls, "userdel ze", "groupdel ze")
}

func TestServiceStatusRuns(t *testing.T) {
	// VALIDATES: AC-5 ze service status runs systemctl status ze.service.
	// PREVENTS: status dispatch being registered but not connected to systemctl.
	fake := newFakeServiceOps()
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"status"})
	if code != 0 {
		t.Fatalf("status exit code = %d, stderr=%s", code, stderr.String())
	}

	assertCalls(t, fake.runCalls, "systemctl status ze.service")
}

func TestServiceRefusesNonLinux(t *testing.T) {
	// VALIDATES: AC-4 ze service install refuses on non-Linux platforms.
	// PREVENTS: attempting systemd writes on Darwin or other non-systemd hosts.
	fake := newFakeServiceOps()
	fake.linux = false
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install"})
	if code != 1 {
		t.Fatalf("install on non-linux exit code = %d", code)
	}
	assertContains(t, stderr.String(), "requires Linux")
}

func TestServiceRefusesNoSystemctl(t *testing.T) {
	// VALIDATES: AC-4 ze service install refuses on Linux hosts without systemctl.
	// PREVENTS: treating non-systemd Linux hosts like supported systemd targets.
	fake := newFakeServiceOps()
	delete(fake.lookPaths, "systemctl")
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install"})
	if code != 1 {
		t.Fatalf("install without systemctl exit code = %d", code)
	}
	assertContains(t, stderr.String(), "systemctl not found")
}

func TestServiceInstallCustomConfig(t *testing.T) {
	// VALIDATES: AC-6 --config writes custom WorkingDirectory and ZE_CONFIG_DIR.
	// PREVENTS: enabling a service pointed at the wrong database directory.
	fake := newFakeServiceOps()
	fake.files["/custom/path"] = ""
	fake.files["/custom/path/database.zefs"] = "zefs"
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install", "--config", "/custom/path"})
	if code != 0 {
		t.Fatalf("install custom config exit code = %d, stderr=%s", code, stderr.String())
	}

	unit := fake.files[defaultUnitPath]
	assertContains(t, unit, "WorkingDirectory=/custom/path")
	assertContains(t, unit, "Environment=ZE_CONFIG_DIR=/custom/path")
}

func TestServiceInstallExistingUnitRequiresForce(t *testing.T) {
	// VALIDATES: AC-7 install refuses existing unit unless --force is present.
	// PREVENTS: overwriting operator-edited systemd units by default.
	fake := newFakeServiceOps()
	fake.files[defaultUnitPath] = "existing"
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install"})
	if code != 1 {
		t.Fatalf("install existing unit exit code = %d", code)
	}
	assertContains(t, stderr.String(), "service already installed")

	stderr.Reset()
	code = rt.run([]string{"install", "--force"})
	if code != 0 {
		t.Fatalf("install --force exit code = %d, stderr=%s", code, stderr.String())
	}
	if fake.files[defaultUnitPath] == "existing" {
		t.Fatalf("--force did not overwrite existing unit")
	}
}

func TestUnitFilePrerequisite(t *testing.T) {
	// VALIDATES: AC-12 install refuses when ze init has not created database.zefs.
	// PREVENTS: enabling a daemon that cannot start because the database is absent.
	fake := newFakeServiceOps()
	delete(fake.files, "/etc/ze/database.zefs")
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install"})
	if code != 1 {
		t.Fatalf("install without zefs exit code = %d", code)
	}
	assertContains(t, stderr.String(), "ze init has not been run")
}

func TestInstallPrintsSocketHint(t *testing.T) {
	// VALIDATES: AC-15 install output explains how the CLI reaches /run/ze/ze.socket.
	// PREVENTS: operators installing a working daemon but not knowing how to connect to it.
	fake := newFakeServiceOps()
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install"})
	if code != 0 {
		t.Fatalf("install exit code = %d, stderr=%s", code, stderr.String())
	}
	assertContains(t, stderr.String(), "XDG_RUNTIME_DIR=/run/ze")
	assertContains(t, stderr.String(), "/run/ze/ze.socket")
}

func TestInstallWarnsDaemonUserConfig(t *testing.T) {
	// VALIDATES: install warns when existing config contains daemon { user }.
	// PREVENTS: systemd User=ze deployments failing later because ze tries to setuid again.
	fake := newFakeServiceOps()
	fake.activeConfigData = [][]byte{[]byte("daemon { user \"nobody\"; }\n")}
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install"})
	if code != 0 {
		t.Fatalf("install exit code = %d, stderr=%s", code, stderr.String())
	}
	assertContains(t, stderr.String(), "daemon { user }")
}

func TestServiceInstallDryRunPrintsUnitOnly(t *testing.T) {
	// VALIDATES: ze service install --dry-run prints the unit file and makes no system changes.
	// PREVENTS: dry-run requiring root/systemd or mutating system files during functional tests.
	fake := newFakeServiceOps()
	fake.linux = false
	fake.root = false
	delete(fake.lookPaths, "systemctl")
	rt, stdout, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install", "--dry-run", "--config", "/custom/path"})
	if code != 0 {
		t.Fatalf("dry-run exit code = %d, stderr=%s", code, stderr.String())
	}
	assertContains(t, stdout.String(), "ExecStart=/usr/local/bin/ze start")
	assertContains(t, stdout.String(), "WorkingDirectory=/custom/path")
	if len(fake.runCalls) != 0 {
		t.Fatalf("dry-run executed commands: %#v", fake.runCalls)
	}
	if _, ok := fake.files[defaultUnitPath]; ok {
		t.Fatalf("dry-run wrote unit file")
	}
}

func TestServiceInstallRejectsRelativeConfig(t *testing.T) {
	// VALIDATES: --config with a relative path is rejected before any system changes.
	// PREVENTS: generating a unit file with a relative WorkingDirectory.
	fake := newFakeServiceOps()
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install", "--config", "relative/path"})
	if code != 1 {
		t.Fatalf("install relative config exit code = %d", code)
	}
	assertContains(t, stderr.String(), "absolute path")
}

func TestServiceInstallRejectsNewlineInConfig(t *testing.T) {
	// VALIDATES: --config paths with newlines are rejected to prevent unit file injection.
	// PREVENTS: crafted paths injecting arbitrary systemd directives.
	fake := newFakeServiceOps()
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install", "--config", "/etc/ze\nExecStartPre=/bin/evil"})
	if code != 1 {
		t.Fatalf("install newline config exit code = %d", code)
	}
	assertContains(t, stderr.String(), "invalid characters")
}

func TestServiceInstallRejectsWhitespaceInConfig(t *testing.T) {
	// VALIDATES: --config paths with whitespace are rejected before unit generation.
	// PREVENTS: generating broken systemd directives that split paths on spaces.
	fake := newFakeServiceOps()
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install", "--config", "/etc/ze config"})
	if code != 1 {
		t.Fatalf("install whitespace config exit code = %d", code)
	}
	assertContains(t, stderr.String(), "invalid characters")
}

func TestServiceInstallRejectsWhitespaceInBinaryPath(t *testing.T) {
	// VALIDATES: resolved binary paths with whitespace are rejected before unit generation.
	// PREVENTS: installing a unit whose ExecStart command is parsed as the wrong binary.
	fake := newFakeServiceOps()
	fake.executablePath = "/usr/local/bin/ze test"
	rt, _, stderr := newTestRuntime(fake)

	code := rt.run([]string{"install", "--dry-run", "--config", "/etc/ze"})
	if code != 1 {
		t.Fatalf("install whitespace binary exit code = %d", code)
	}
	assertContains(t, stderr.String(), "invalid characters")
}

type fakeServiceOps struct {
	linux            bool
	root             bool
	executablePath   string
	files            map[string]string
	lookPaths        map[string]string
	outputs          map[string]string
	outputErrors     map[string]error
	runCalls         []string
	runErrors        map[string]error
	chownCalls       []string
	activeConfigData [][]byte
	activeConfigErr  error
}

func newFakeServiceOps() *fakeServiceOps {
	return &fakeServiceOps{
		linux:          true,
		root:           true,
		executablePath: "/usr/local/bin/ze",
		files: map[string]string{
			"/etc/ze":               "",
			"/etc/ze/database.zefs": "zefs",
			"/usr/sbin/nologin":     "",
		},
		lookPaths: map[string]string{
			"systemctl": "/bin/systemctl",
			"groupadd":  "/usr/sbin/groupadd",
			"useradd":   "/usr/sbin/useradd",
			"userdel":   "/usr/sbin/userdel",
			"groupdel":  "/usr/sbin/groupdel",
		},
		outputs:      map[string]string{},
		outputErrors: map[string]error{},
		runErrors:    map[string]error{},
	}
}

func newTestRuntime(fake *fakeServiceOps) (*serviceRuntime, *bytes.Buffer, *bytes.Buffer) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := &serviceRuntime{
		stdout:   &stdout,
		stderr:   &stderr,
		ops:      fake,
		unitPath: defaultUnitPath,
	}
	return rt, &stdout, &stderr
}

func (f *fakeServiceOps) isLinux() bool { return f.linux }

func (f *fakeServiceOps) isRoot() bool { return f.root }

func (f *fakeServiceOps) executable() (string, error) { return f.executablePath, nil }

func (f *fakeServiceOps) evalSymlinks(path string) (string, error) { return path, nil }

func (f *fakeServiceOps) lookPath(name string) (string, error) {
	if path, ok := f.lookPaths[name]; ok {
		return path, nil
	}
	return "", errors.New("not found")
}

func (f *fakeServiceOps) stat(path string) (fs.FileInfo, error) {
	if _, ok := f.files[path]; ok {
		return fakeFileInfo{name: path}, nil
	}
	return nil, os.ErrNotExist
}

func (f *fakeServiceOps) writeFile(path string, data []byte, _ fs.FileMode) error {
	f.files[path] = string(data)
	return nil
}

func (f *fakeServiceOps) remove(path string) error {
	delete(f.files, path)
	return nil
}

func (f *fakeServiceOps) run(name string, args ...string) error {
	call := strings.Join(append([]string{name}, args...), " ")
	f.runCalls = append(f.runCalls, call)
	if err, ok := f.runErrors[call]; ok {
		return err
	}
	return nil
}

func (f *fakeServiceOps) output(name string, args ...string) ([]byte, error) {
	call := strings.Join(append([]string{name}, args...), " ")
	if err, ok := f.outputErrors[call]; ok {
		return nil, err
	}
	if out, ok := f.outputs[call]; ok {
		return []byte(out), nil
	}
	return nil, errors.New("not found")
}

func (f *fakeServiceOps) chown(path, user, group string) error {
	f.chownCalls = append(f.chownCalls, path+" "+user+":"+group)
	return nil
}

func (f *fakeServiceOps) activeConfigs(string) ([][]byte, error) {
	return f.activeConfigData, f.activeConfigErr
}

type fakeFileInfo struct{ name string }

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode  { return 0o644 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

func assertCalls(t *testing.T, got []string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !slices.Contains(got, want) {
			t.Fatalf("missing call %q in %#v", want, got)
		}
	}
}
