// Design: (none -- new platform detection for runtime environment)

//go:build linux

package host

import (
	"os"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/core/gokrazyutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// DetectPlatform identifies the runtime platform and probes capabilities.
// Detection order matters: gokrazy and container are checked first
// because they are more specific than systemd (gokrazy runs on Linux
// with a read-only root; containers may have systemd paths present).
func (d *Detector) DetectPlatform() (*PlatformInfo, error) {
	info := &PlatformInfo{}

	root := d.root()

	var tb textbuf.Buffer
	info.ReadOnlyRoot = isReadOnlyRoot(root)
	info.PermAvailable = isDir(tb.Str(root).Str("/perm").String())
	info.SystemdAvailable = isDir(tb.Reset().Str(root).Str("/run/systemd/system").String())
	info.GokrazyUpdateSocket = isSocket(tb.Reset().Str(root).Str(gokrazyutil.DefaultSocketPath).String())
	info.GokrazyUIAvailable = info.GokrazyUpdateSocket
	info.RebootAllowed = os.Getuid() == 0
	info.PersistentStorageWritable = isWritableDir(tb.Reset().Str(root).Str("/perm").String())
	probeFDLimits(info)

	info.Type = classifyPlatform(info, root)

	return info, nil
}

// classifyPlatform determines the platform type from probed capabilities.
// Order is most-specific first: gokrazy wins over container (a Docker
// container on a gokrazy host still reports gokrazy), container wins
// over systemd (many containers have /run/systemd/system present).
func classifyPlatform(info *PlatformInfo, root string) PlatformType {
	if isGokrazy(info, root) {
		return PlatformGokrazy
	}
	if isContainer(root) {
		return PlatformContainer
	}
	if info.SystemdAvailable {
		return PlatformSystemd
	}
	return PlatformPlainLinux
}

// isGokrazy detects the gokrazy appliance environment. Gokrazy has a
// read-only root squashfs, a /perm partition for persistent data, and
// the management HTTP socket.
func isGokrazy(info *PlatformInfo, root string) bool {
	if info.GokrazyUpdateSocket {
		return true
	}
	if info.PermAvailable && info.ReadOnlyRoot {
		return true
	}
	// /user/gokrazy directory exists on gokrazy images.
	var tb textbuf.Buffer
	return isDir(tb.Str(root).Str("/user/gokrazy").String())
}

// isContainer detects Docker, Podman, LXC, and similar container
// runtimes via well-known marker files and cgroup hints. Checks both
// cgroups v1 (/proc/1/cgroup) and v2 (/proc/1/mountinfo) because
// pure cgroups v2 writes only "0::/" to /proc/1/cgroup with no
// runtime hint strings.
func isContainer(root string) bool {
	var tb textbuf.Buffer
	if fileExists(tb.Str(root).Str("/.dockerenv").String()) {
		return true
	}
	if fileExists(tb.Reset().Str(root).Str("/run/.containerenv").String()) {
		return true
	}
	if hasCgroupContainerHint(tb.Reset().Str(root).Str("/proc/1/cgroup").String()) {
		return true
	}
	if hasMountinfoContainerHint(tb.Reset().Str(root).Str("/proc/1/mountinfo").String()) {
		return true
	}
	return false
}

func hasCgroupContainerHint(path string) bool {
	data, err := os.ReadFile(path) //nolint:gosec // path built from root
	if err != nil {
		return false
	}
	s := string(data)
	return strings.Contains(s, "docker") ||
		strings.Contains(s, "kubepods") ||
		strings.Contains(s, "lxc") ||
		strings.Contains(s, "containerd")
}

func hasMountinfoContainerHint(path string) bool {
	data, err := os.ReadFile(path) //nolint:gosec // path built from root
	if err != nil {
		return false
	}
	s := string(data)
	return strings.Contains(s, "docker/containers/") ||
		strings.Contains(s, "/kubepods/") ||
		strings.Contains(s, "/lxc/")
}

func isReadOnlyRoot(root string) bool {
	var stat syscall.Statfs_t
	path := root
	if path == "" {
		path = "/"
	}
	if err := syscall.Statfs(path, &stat); err != nil {
		return false
	}
	const stRdonly = 0x1
	return stat.Flags&stRdonly != 0
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func isSocket(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().Type() == os.ModeSocket
}

func isWritableDir(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || !fi.IsDir() {
		return false
	}
	// W_OK (0x2) checks write permission; distinct from O_RDWR which
	// is a file-open flag that happens to share the same numeric value.
	const wOK = 0x2
	return syscall.Access(path, wOK) == nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// probeFDLimits reads the current RLIMIT_NOFILE soft/hard limits and
// checks whether the soft limit can be raised (soft < hard).
func probeFDLimits(info *PlatformInfo) {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return
	}
	info.FDLimitSoftCurrent = limit.Cur
	info.FDLimitHardMax = limit.Max
	info.FDLimitRaisable = limit.Cur < limit.Max
}
