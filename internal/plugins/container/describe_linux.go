// Design: docs/architecture/container/runtime.md -- the container description
// Related: library_linux.go -- the runtime library that creates from the description
// Related: rootfs_linux.go -- the root the description names

package container

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/cgroups"
	devices "github.com/opencontainers/cgroups/devices/config"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container/seccomp"
)

// cgroupParent is the control group, under the cgroup v2 mount, that holds one
// control group for each container. It holds no process itself, so the
// kernel lets it hand controllers to its children.
const cgroupParent = "/ze-container"

// The default limits of a container (the owner's ruling: limits are
// mandatory, with defaults). The nodes that set them land with the limits
// work; until then every container runs with these.
const (
	memoryDefaultBytes  = 512 << 20
	processLimitDefault = 256
)

// defaultCapabilities are the thirteen capabilities a container holds by
// default: the set published images expect (the owner's ruling), less
// CAP_MKNOD.
//
// CAP_MKNOD is withheld until the control group's device allowlist lands with
// the devices work: with no allowlist, root in the container could make a node
// for any host device and open it. The default device nodes need no grant: the
// library's init makes them while it prepares the root (prepareRootfs), before
// it drops to this set (finalizeNamespace), and /dev/ptmx is a symlink into
// the container's own devpts mount.
var defaultCapabilities = []string{
	"CAP_AUDIT_WRITE",
	"CAP_CHOWN",
	"CAP_DAC_OVERRIDE",
	"CAP_FOWNER",
	"CAP_FSETID",
	"CAP_KILL",
	"CAP_NET_BIND_SERVICE",
	"CAP_NET_RAW",
	"CAP_SETFCAP",
	"CAP_SETGID",
	"CAP_SETPCAP",
	"CAP_SETUID",
	"CAP_SYS_CHROOT",
}

// The masked and read-only paths of the runtime library's default
// description (upstream specconv.Example at v1.5.3): kernel files a
// container must neither read nor write.
var (
	defaultMaskedPaths = []string{
		"/proc/acpi",
		"/proc/asound",
		"/proc/kcore",
		"/proc/keys",
		"/proc/latency_stats",
		"/proc/timer_list",
		"/proc/timer_stats",
		"/proc/sched_debug",
		"/proc/scsi",
		"/sys/firmware",
	}
	defaultReadonlyPaths = []string{
		"/proc/bus",
		"/proc/fs",
		"/proc/irq",
		"/proc/sys",
		"/proc/sysrq-trigger",
	}
)

// defaultMountFlags are the flags of every pseudo-filesystem mount that holds
// no program and no device.
const defaultMountFlags = unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_NODEV

// defaultDevice describes one device node the library makes in the
// container's device directory.
func defaultDevice(path string, major, minor int64) *devices.Device {
	return &devices.Device{
		Type:        devices.CharDevice,
		Major:       major,
		Minor:       minor,
		Permissions: "rwm",
		Allow:       true,
		Path:        path,
		FileMode:    0o666,
	}
}

// describeContainer is the library's description of one container whose root
// is mounted at rootfs: its own pid, mount, network, ipc, uts and cgroup
// namespaces, the default mounts, masked and read-only paths and devices, the
// thirteen capabilities, no-new-privileges, the default system-call profile
// selected by those capabilities, its host name, its loopback device and its
// control group with the default limits. After the default mounts it mounts
// each file the participants contributed, read-only, at its path in the
// container; the supervisor admitted each one (checkFileMount).
//
// It never sets a parent death signal, which the patched library refuses,
// and never fills the library's network-device list: a container's link is
// made by a participant at the created call.
//
// The device filter of the control group is not set: the cgroups module's
// filter package links the systemd driver, and it lands with the devices
// work. Until then the device nodes are made, no device access is denied, and
// the container holds no CAP_MKNOD to make a node of its own.
func describeContainer(instance instanceConfig, rootfs string, contributed []FileMount) *configs.Config {
	memory := int64(memoryDefaultBytes)
	processLimit := int64(processLimitDefault)
	return &configs.Config{
		Rootfs: rootfs,
		Namespaces: configs.Namespaces([]configs.Namespace{
			{Type: configs.NEWNS},
			{Type: configs.NEWUTS},
			{Type: configs.NEWIPC},
			{Type: configs.NEWPID},
			{Type: configs.NEWNET},
			{Type: configs.NEWCGROUP},
		}),
		Capabilities: &configs.Capabilities{
			Bounding:  defaultCapabilities,
			Effective: defaultCapabilities,
			Permitted: defaultCapabilities,
		},
		NoNewPrivileges: true,
		// The profile's capability rules are selected by the bounding set,
		// so it is built from the set the container holds.
		Seccomp:  seccomp.DefaultProfile(defaultCapabilities),
		Hostname: instance.name,
		Mounts:   append(defaultMounts(), contributedMounts(contributed)...),
		Devices: []*devices.Device{
			defaultDevice("/dev/null", 1, 3),
			defaultDevice("/dev/zero", 1, 5),
			defaultDevice("/dev/full", 1, 7),
			defaultDevice("/dev/random", 1, 8),
			defaultDevice("/dev/urandom", 1, 9),
			defaultDevice("/dev/tty", 5, 0),
		},
		MaskPaths:     defaultMaskedPaths,
		ReadonlyPaths: defaultReadonlyPaths,
		// The library's init brings the loopback device up at the start of
		// its sequence, before it blocks.
		Networks: []*configs.Network{{Type: "loopback"}},
		Cgroups: &cgroups.Cgroup{
			Path: filepath.Join(cgroupParent, instance.name),
			Resources: &cgroups.Resources{
				Memory: memory,
				// Equal to Memory: no swap beyond the memory limit.
				MemorySwap: memory,
				PidsLimit:  &processLimit,
			},
		},
	}
}

// filesystemTmpfs is the memory-backed filesystem type of the device
// directory and of shared memory.
const filesystemTmpfs = "tmpfs"

// defaultMounts are the mounts of the runtime library's default description:
// proc, a device directory with its pseudo-terminals, shared memory and
// message queues, sysfs and the container's own cgroup tree, both read-only.
func defaultMounts() []*configs.Mount {
	return []*configs.Mount{
		{Source: "proc", Destination: "/proc", Device: "proc", Flags: defaultMountFlags},
		{Source: filesystemTmpfs, Destination: "/dev", Device: filesystemTmpfs, Flags: unix.MS_NOSUID | unix.MS_STRICTATIME, Data: "mode=755,size=65536k"},
		{Source: "devpts", Destination: "/dev/pts", Device: "devpts", Flags: unix.MS_NOSUID | unix.MS_NOEXEC, Data: "newinstance,ptmxmode=0666,mode=0620,gid=5"},
		{Source: "shm", Destination: "/dev/shm", Device: filesystemTmpfs, Flags: defaultMountFlags, Data: "mode=1777,size=65536k"},
		{Source: "mqueue", Destination: "/dev/mqueue", Device: "mqueue", Flags: defaultMountFlags},
		{Source: "sysfs", Destination: "/sys", Device: "sysfs", Flags: defaultMountFlags | unix.MS_RDONLY},
		{Source: "cgroup", Destination: "/sys/fs/cgroup", Device: "cgroup", Flags: defaultMountFlags | unix.MS_RELATIME | unix.MS_RDONLY},
	}
}

// contributedMountFlags are the flags of a contributed file mount: a bind of
// the host file, read-only, from which nothing executes, no set-user program
// gains a privilege and no device opens. The library binds, then remounts the
// bind with these flags, which is how a bind becomes read-only.
const contributedMountFlags = unix.MS_BIND | unix.MS_RDONLY | unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC

// deviceBind is the library's device word of a bind mount.
const deviceBind = "bind"

// contributedMounts are the library's mounts of the files the participants
// contributed, in their order. The library makes the file at the target when
// the image holds none.
func contributedMounts(contributed []FileMount) []*configs.Mount {
	mounts := make([]*configs.Mount, 0, len(contributed))
	for _, mount := range contributed {
		mounts = append(mounts, &configs.Mount{
			Source:      mount.HostPath,
			Destination: mount.ContainerPath,
			Device:      deviceBind,
			Flags:       contributedMountFlags,
		})
	}
	return mounts
}

// defaultPath is the search path of an entrypoint whose image declares none.
const defaultPath = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// describeEntrypoint is the library's description of a container's
// entrypoint, from the image's configuration as the store fetched it: the
// entrypoint followed by the command, the image's variables, its working
// directory and its user. It refuses an image that declares nothing to run,
// and a user given by name, which is resolved in the image's own account
// files by the work that reads them.
func describeEntrypoint(name string, ready Ready) (*libcontainer.Process, error) {
	args := make([]string, 0, len(ready.Entrypoint)+len(ready.Command))
	args = append(args, ready.Entrypoint...)
	args = append(args, ready.Command...)
	if len(args) == 0 {
		return nil, opererr.New(
			"container "+name+": its image declares no entrypoint and no command",
			"a container runs the program its image names, and this image names none",
			"ze show config running",
		)
	}
	uid, gid, err := numericUser(name, ready.User)
	if err != nil {
		return nil, err
	}
	env := ready.Environment
	if !declaresPath(env) {
		env = append(append(make([]string, 0, len(env)+1), env...), defaultPath)
	}
	cwd := ready.WorkingDirectory
	if cwd == "" {
		cwd = "/"
	}
	return &libcontainer.Process{
		Args: args,
		Env:  env,
		UID:  uid,
		GID:  gid,
		Cwd:  cwd,
		Init: true,
	}, nil
}

// declaresPath answers whether env holds a PATH variable.
func declaresPath(env []string) bool {
	for _, variable := range env {
		if strings.HasPrefix(variable, "PATH=") {
			return true
		}
	}
	return false
}

// numericUser reads the image's user as written: empty is root, a number is
// used as it is, and a number with a number for its group sets both. A name
// is refused: it is resolved in the image's account files, which this build
// does not read yet.
func numericUser(name, user string) (int, int, error) {
	if user == "" {
		return 0, 0, nil
	}
	userPart, groupPart, hasGroup := strings.Cut(user, ":")
	uid, err := strconv.ParseUint(userPart, 10, 31)
	if err != nil {
		return 0, 0, refuseImageUser(name, user)
	}
	if !hasGroup {
		return int(uid), int(uid), nil
	}
	gid, err := strconv.ParseUint(groupPart, 10, 31)
	if err != nil {
		return 0, 0, refuseImageUser(name, user)
	}
	return int(uid), int(gid), nil
}

func refuseImageUser(name, user string) error {
	return opererr.New(
		"container "+name+": its image runs as the user "+strconv.Quote(user)+", which ze cannot resolve yet",
		"a user given by name is looked up in the image's own account files, which this build does not read",
		"set container instance "+name+" enabled false",
	)
}
