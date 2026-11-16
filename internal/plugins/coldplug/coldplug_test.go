package coldplug

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The device and module names the fake trees share. The e1000 alias and
// modalias are the shape the kernel writes for an Intel 82540EM, the NIC QEMU
// gives a guest by default.
const (
	e1000Alias    = "pci:v00008086d0000100Esv*sd*bc*sc*i*"
	e1000Modalias = "pci:v00008086d0000100Esv00008086sd0000001Ebc02sc00i00"
	e1000Device   = "devices/pci0000:00/0000:00:03.0/modalias"
	serialDevice  = "devices/platform/serial8250/modalias"
	e1000File     = "kernel/drivers/net/ethernet/intel/e1000/e1000.ko"
)

// fakeKernel is one fake /sys and /lib/modules/<release> pair under a test
// directory.
type fakeKernel struct {
	sysRoot     string
	modulesRoot string
}

// newFakeKernel writes sys (paths under /sys) and modules (paths under the
// modules root) into a fresh test directory. A nil modules map writes no
// modules tree at all; an empty one writes the directory and no file.
func newFakeKernel(t *testing.T, sys, modules map[string]string) fakeKernel {
	t.Helper()
	root := t.TempDir()
	kernel := fakeKernel{sysRoot: filepath.Join(root, "sys"), modulesRoot: filepath.Join(root, "lib", "modules", "7.3.0-ze")}
	writeFiles(t, kernel.sysRoot, sys)
	if modules != nil {
		require.NoError(t, os.MkdirAll(kernel.modulesRoot, 0o755))
		writeFiles(t, kernel.modulesRoot, modules)
	}
	return kernel
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
}

// recordingLoader stands in for finit_module: it records each file it is
// handed and answers the error the test assigned to that module's file name.
type recordingLoader struct {
	modulesRoot string
	answers     map[string]error
	files       []string
}

func (l *recordingLoader) load(file string) error {
	relative, err := filepath.Rel(l.modulesRoot, file)
	if err != nil {
		return err
	}
	l.files = append(l.files, relative)
	return l.answers[filepath.Base(relative)]
}

// run runs one pass over the fake kernel and answers the report and what the
// pass logged, at every level.
func (k fakeKernel) run(t *testing.T, loaded map[string]bool, loader *recordingLoader) (passReport, string) {
	t.Helper()
	var logged bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))
	loader.modulesRoot = k.modulesRoot
	report := coldplug(k.sysRoot, k.modulesRoot, loaded, loader.load, log)
	require.NotEqual(t, passUnspecified, report.outcome, "every pass ends with a named outcome")
	return report, logged.String()
}

// TestColdplugLoadsDependenciesFirst proves the pass loads the module a
// present device's alias names after every module it needs: its soft "pre"
// module, its modules.dep dependencies in dependency order, and its soft
// "post" module last. A device no alias matches loads nothing.
//
// Method: e1000 needs mii and libphy-core, and mii needs libphy-core, but
// modules.dep lists mii first on e1000's line, so a pass that loads a line in
// its written order loads mii before the module mii needs.
// MUTATION: loading file.deps in written order without recursing puts mii
// before libphy_core; appending the module before its deps puts e1000 first;
// skipping softdeps drops phy_realtek and e1000_post; a match that ignores
// the pattern loads something for the serial port.
func TestColdplugLoadsDependenciesFirst(t *testing.T) {
	kernel := newFakeKernel(t, map[string]string{
		e1000Device:  e1000Modalias + "\n",
		serialDevice: "platform:serial8250\n",
	}, map[string]string{
		aliasFile: "# Aliases extracted from modules themselves.\nalias " + e1000Alias + " e1000\n",
		depFile: e1000File + ": kernel/drivers/net/mii.ko kernel/lib/libphy-core.ko\n" +
			"kernel/drivers/net/mii.ko: kernel/lib/libphy-core.ko\n" +
			"kernel/lib/libphy-core.ko:\n" +
			"kernel/drivers/net/phy/phy-realtek.ko:\n" +
			"kernel/drivers/net/e1000-post.ko:\n",
		softdepFile: "softdep e1000 pre: phy-realtek post: e1000-post\n",
	})
	loader := &recordingLoader{}

	report, _ := kernel.run(t, map[string]bool{}, loader)

	assert.Equal(t, passRan, report.outcome)
	assert.Equal(t, []string{
		"kernel/drivers/net/phy/phy-realtek.ko",
		"kernel/lib/libphy-core.ko",
		"kernel/drivers/net/mii.ko",
		e1000File,
		"kernel/drivers/net/e1000-post.ko",
	}, loader.files)
	assert.Equal(t, []string{"phy_realtek", "libphy_core", "mii", "e1000", "e1000_post"}, report.loaded)
	assert.Empty(t, report.failed)
}

// TestColdplugSkipsBuiltinAndLoaded proves the pass loads nothing the kernel
// already has: a module built into the kernel, a module /proc/modules lists,
// and a module the kernel reports as loaded when the pass asks (another
// loader got there between the list and the load). None of the three is a
// failure.
//
// Method: e1000 has a soft "pre" on a built-in module and a hard dependency
// on mii, which the loaded set holds; a second device's driver answers
// errAlreadyLoaded.
// MUTATION: dropping the builtin check hands crc32c to the loader; dropping
// the loaded check hands mii to it; reading errAlreadyLoaded as a failure
// puts virtio_extra in failed.
func TestColdplugSkipsBuiltinAndLoaded(t *testing.T) {
	kernel := newFakeKernel(t, map[string]string{
		e1000Device:                    e1000Modalias + "\n",
		"devices/virtual/one/modalias": "virtual:one\n",
	}, map[string]string{
		aliasFile:   "alias " + e1000Alias + " e1000\nalias virtual:* virtio_extra\n",
		depFile:     e1000File + ": kernel/drivers/net/mii.ko\nkernel/drivers/net/mii.ko:\nkernel/drivers/virtio/virtio-extra.ko:\n",
		builtinFile: "kernel/lib/crc32c.ko\n",
		softdepFile: "softdep e1000 pre: crc32c\n",
	})
	loader := &recordingLoader{answers: map[string]error{"virtio-extra.ko": errAlreadyLoaded}}

	report, _ := kernel.run(t, map[string]bool{"mii": true}, loader)

	assert.Equal(t, passRan, report.outcome)
	assert.Equal(t, []string{e1000File, "kernel/drivers/virtio/virtio-extra.ko"}, loader.files)
	assert.Equal(t, []string{"e1000"}, report.loaded)
	assert.Empty(t, report.failed)
}

// TestColdplugWithoutIndexDoesNothing proves a kernel with no module tree, or
// a tree without its index, is not a fault: the pass loads nothing and logs
// nothing at warning level, because a kernel with every driver built in and a
// container both look like this.
//
// MUTATION: reading an absent modules.dep as a broken index logs a WARN and
// answers passIndexBroken; walking the devices first and loading on a missing
// index hands the loader a file.
func TestColdplugWithoutIndexDoesNothing(t *testing.T) {
	trees := map[string]map[string]string{
		"no modules tree":    nil,
		"no modules.dep":     {aliasFile: "alias " + e1000Alias + " e1000\n"},
		"no modules.alias":   {depFile: e1000File + ":\n"},
		"empty modules tree": {},
	}
	for name, modules := range trees {
		t.Run(name, func(t *testing.T) {
			kernel := newFakeKernel(t, map[string]string{e1000Device: e1000Modalias + "\n"}, modules)
			loader := &recordingLoader{}

			report, logged := kernel.run(t, map[string]bool{}, loader)

			assert.Equal(t, passNoIndex, report.outcome)
			assert.Empty(t, loader.files)
			assert.NotContains(t, logged, "level=WARN")
		})
	}
}

// TestColdplugFailureDoesNotStopThePass proves one fault costs only the
// devices that need the failing module: the pass goes on to the next module,
// does not try a module whose dependency failed, and logs each fault at
// warning level under the doctor code with the module and the device.
//
// Method: three devices. a fails to load, c needs a, b loads, and missing is
// named by an alias but held by no modules.dep line.
// MUTATION: returning on the first load error leaves b unloaded; trying c
// anyway hands c.ko to the loader; dropping the missing-module report leaves
// missing out of failed.
func TestColdplugFailureDoesNotStopThePass(t *testing.T) {
	kernel := newFakeKernel(t, map[string]string{
		"devices/virtual/a/modalias":       "virtual:a\n",
		"devices/virtual/b/modalias":       "virtual:b\n",
		"devices/virtual/c/modalias":       "virtual:c\n",
		"devices/virtual/missing/modalias": "virtual:missing\n",
	}, map[string]string{
		aliasFile: "alias virtual:a a\nalias virtual:b b\nalias virtual:c c\nalias virtual:missing missing\n",
		depFile:   "kernel/a.ko:\nkernel/b.ko:\nkernel/c.ko: kernel/a.ko\n",
	})
	loader := &recordingLoader{answers: map[string]error{"a.ko": errors.New("unknown symbol in module")}}

	report, logged := kernel.run(t, map[string]bool{}, loader)

	assert.Equal(t, passRan, report.outcome)
	assert.Equal(t, []string{"kernel/a.ko", "kernel/b.ko"}, loader.files)
	assert.Equal(t, []string{"b"}, report.loaded)
	assert.Equal(t, []string{"missing", "a", "c"}, report.failed)
	for _, module := range []string{"module=a", "module=c", "module=missing"} {
		line := lineWith(logged, module)
		assert.Contains(t, line, "level=WARN", module)
		assert.Contains(t, line, "code=doctor-module-missing", module)
		assert.Contains(t, line, "device=virtual:", module)
		assert.Contains(t, line, "fix=", module)
	}
}

// TestColdplugReportsABrokenIndex proves an index file that is present and
// does not parse is reported, at warning level with its fix, and loads
// nothing, rather than being read as an empty index.
//
// MUTATION: skipping a malformed line loads e1000 and answers passRan.
func TestColdplugReportsABrokenIndex(t *testing.T) {
	kernel := newFakeKernel(t, map[string]string{e1000Device: e1000Modalias + "\n"}, map[string]string{
		aliasFile: "alias " + e1000Alias + " e1000\nalias truncated\n",
		depFile:   e1000File + ":\n",
	})
	loader := &recordingLoader{}

	report, logged := kernel.run(t, map[string]bool{}, loader)

	assert.Equal(t, passIndexBroken, report.outcome)
	assert.Empty(t, loader.files)
	assert.Contains(t, logged, "level=WARN")
	assert.Contains(t, logged, "modules.alias line 2")
}

// TestWalkDevicesDoesNotFollowLinks proves the device walk reads each
// modalias once and does not follow the links sysfs puts in every device
// directory, which point back up the tree and would loop.
//
// MUTATION: a walk that follows directory links reaches the device again
// through the link (or never ends); one that keeps duplicates answers two
// aliases.
func TestWalkDevicesDoesNotFollowLinks(t *testing.T) {
	kernel := newFakeKernel(t, map[string]string{e1000Device: e1000Modalias + "\n"}, nil)
	device := filepath.Join(kernel.sysRoot, filepath.Dir(e1000Device))
	require.NoError(t, os.Symlink("../..", filepath.Join(device, "subsystem")))
	writeFiles(t, kernel.sysRoot, map[string]string{"devices/pci0000:00/0000:00:04.0/modalias": e1000Modalias + "\n"})

	walk, err := walkDevices(kernel.sysRoot)

	require.NoError(t, err)
	assert.Equal(t, []string{e1000Modalias}, walk.aliases)
	assert.Zero(t, walk.unreadable)
}

// lineWith answers the first logged line holding fragment, or "" when none
// does, which every assertion on it then fails.
func lineWith(logged, fragment string) string {
	for line := range strings.SplitSeq(logged, "\n") {
		if strings.Contains(line, fragment+" ") {
			return line
		}
	}
	return ""
}
