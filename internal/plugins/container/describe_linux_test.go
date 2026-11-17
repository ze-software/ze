// Design: docs/architecture/container/runtime.md -- the container description

package container

import (
	"reflect"
	"slices"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/plugins/container/seccomp"
)

// TestDescriptionHasTheDefaultConfinement proves the description of a
// container with no grant holds the default confinement: its own six
// namespaces, the default mounts, masked paths, read-only paths and devices,
// the thirteen capabilities in each set that grants one (CAP_MKNOD absent from
// every set), no-new-privileges, the default system-call profile of that
// bounding set (EPERM for an unlisted call), the instance name as host name, the loopback device, and a control group of
// its own under the plugin's parent with the default limits.
//
// Method: describe one container and compare each field with the expected
// value, written out here rather than read back from the tables the code uses.
func TestDescriptionHasTheDefaultConfinement(t *testing.T) {
	config := describeContainer(instanceConfig{name: "web", enabled: true}, "/run/ze-container/root/web", nil)

	if config.Rootfs != "/run/ze-container/root/web" {
		t.Errorf("rootfs is %q, want the mount point", config.Rootfs)
	}
	for _, namespace := range []configs.NamespaceType{configs.NEWNS, configs.NEWUTS, configs.NEWIPC, configs.NEWPID, configs.NEWNET, configs.NEWCGROUP} {
		if !config.Namespaces.IsPrivate(namespace) {
			t.Errorf("the container does not get a %s namespace of its own", namespace)
		}
	}
	if config.Namespaces.Contains(configs.NEWUSER) {
		t.Error("the description asks for a user namespace, which the patched library refuses")
	}
	if len(config.Namespaces) != 6 {
		t.Errorf("the description holds %d namespaces, want 6", len(config.Namespaces))
	}

	capabilities := []string{
		"CAP_AUDIT_WRITE", "CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FOWNER", "CAP_FSETID", "CAP_KILL",
		"CAP_NET_BIND_SERVICE", "CAP_NET_RAW", "CAP_SETFCAP", "CAP_SETGID", "CAP_SETPCAP", "CAP_SETUID", "CAP_SYS_CHROOT",
	}
	for set, held := range map[string][]string{
		"bounding":  config.Capabilities.Bounding,
		"effective": config.Capabilities.Effective,
		"permitted": config.Capabilities.Permitted,
	} {
		if !slices.Equal(slices.Sorted(slices.Values(held)), capabilities) {
			t.Errorf("the %s set is %v, want the thirteen defaults %v", set, held, capabilities)
		}
		// CAP_MKNOD returns only together with the control group's device
		// allowlist: without one, root in the container could make a node for
		// any host device and open it.
		if slices.Contains(held, "CAP_MKNOD") {
			t.Errorf("the %s set holds CAP_MKNOD while no device allowlist exists", set)
		}
	}
	if len(config.Capabilities.Inheritable) != 0 || len(config.Capabilities.Ambient) != 0 {
		t.Errorf("the inheritable or ambient set is not empty: %v, %v", config.Capabilities.Inheritable, config.Capabilities.Ambient)
	}
	if !config.NoNewPrivileges {
		t.Error("no-new-privileges is not set")
	}
	// The default system-call profile, selected by the container's own
	// bounding set: a profile built for another set would allow the calls of
	// a capability the container does not hold.
	if config.Seccomp == nil {
		t.Fatal("the description holds no system-call profile")
	}
	if !reflect.DeepEqual(config.Seccomp, seccomp.DefaultProfile(config.Capabilities.Bounding)) {
		t.Error("the system-call profile is not the default profile of the container's bounding set")
	}
	if config.Seccomp.DefaultAction != configs.Errno {
		t.Errorf("the profile's default action is %v, want Errno", config.Seccomp.DefaultAction)
	}
	if config.Seccomp.DefaultErrnoRet == nil {
		t.Fatal("the profile's default carries no error number, want EPERM")
	}
	if *config.Seccomp.DefaultErrnoRet != uint(unix.EPERM) {
		t.Errorf("the profile's default error number is %d, want EPERM", *config.Seccomp.DefaultErrnoRet)
	}
	if config.Hostname != "web" {
		t.Errorf("the host name is %q, want the instance name", config.Hostname)
	}

	destinations := make([]string, 0, len(config.Mounts))
	for _, mount := range config.Mounts {
		destinations = append(destinations, mount.Destination)
	}
	wantMounts := []string{"/proc", "/dev", "/dev/pts", "/dev/shm", "/dev/mqueue", "/sys", "/sys/fs/cgroup"}
	if !slices.Equal(destinations, wantMounts) {
		t.Errorf("the mounts are %v, want %v", destinations, wantMounts)
	}

	devicePaths := make([]string, 0, len(config.Devices))
	for _, device := range config.Devices {
		devicePaths = append(devicePaths, device.Path)
	}
	wantDevices := []string{"/dev/null", "/dev/zero", "/dev/full", "/dev/random", "/dev/urandom", "/dev/tty"}
	if !slices.Equal(devicePaths, wantDevices) {
		t.Errorf("the devices are %v, want %v", devicePaths, wantDevices)
	}

	for _, path := range []string{"/proc/kcore", "/proc/keys", "/proc/timer_list", "/sys/firmware"} {
		if !slices.Contains(config.MaskPaths, path) {
			t.Errorf("%s is not masked", path)
		}
	}
	for _, path := range []string{"/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus", "/proc/fs"} {
		if !slices.Contains(config.ReadonlyPaths, path) {
			t.Errorf("%s is not read-only", path)
		}
	}

	if len(config.Networks) != 1 || config.Networks[0].Type != "loopback" {
		t.Errorf("the networks are %v, want the loopback device alone", config.Networks)
	}

	if config.Cgroups == nil || config.Cgroups.Resources == nil {
		t.Fatal("the description holds no control group")
	}
	if config.Cgroups.Path != "/ze-container/web" {
		t.Errorf("the control group is %q, want /ze-container/web", config.Cgroups.Path)
	}
	if config.Cgroups.Resources.Memory != 512<<20 {
		t.Errorf("the memory limit is %d, want 512 MiB", config.Cgroups.Resources.Memory)
	}
	if limit := config.Cgroups.Resources.PidsLimit; limit == nil || *limit != 256 {
		t.Errorf("the process limit is %v, want 256", limit)
	}
	if len(config.Cgroups.Resources.Devices) != 0 {
		t.Errorf("device rules are set (%d), which the control group manager of this build refuses", len(config.Cgroups.Resources.Devices))
	}
}

// TestDescriptionNeverSetsAParentDeathSignal proves no description carries a
// parent death signal: the patched library refuses one, because the standard
// library would make init kill itself inside its new pid namespace.
func TestDescriptionNeverSetsAParentDeathSignal(t *testing.T) {
	for _, instance := range []instanceConfig{{name: "web", enabled: true}, {name: "db", enabled: false}} {
		if signal := describeContainer(instance, "/run/ze-container/root/"+instance.name, nil).ParentDeathSignal; signal != 0 {
			t.Errorf("container %s is described with parent death signal %d, want none", instance.name, signal)
		}
	}
}

// TestDescriptionNeverFillsTheLibraryNetworkDevices proves the library's
// network-device list stays empty: a container's link is made by a
// participant at the created call, never moved in by the library.
func TestDescriptionNeverFillsTheLibraryNetworkDevices(t *testing.T) {
	if devices := describeContainer(instanceConfig{name: "web", enabled: true}, "/run/ze-container/root/web", nil).NetDevices; len(devices) != 0 {
		t.Errorf("the description moves network devices into the container: %v", devices)
	}
}

// TestDescriptionMountsContributedFilesReadOnly proves a file a participant
// contributed is in the description, after the default mounts: a bind of the
// host file at its container path, read-only, with no execution, no set-user
// program and no device. Before this, the description never read the
// contributions, and a contributed file was dropped in silence.
//
// Method: describe one container with two contributions and compare the
// mounts after the defaults with the expected entries, written out here.
func TestDescriptionMountsContributedFilesReadOnly(t *testing.T) {
	contributed := []FileMount{
		{HostPath: "/run/ze-container/network/web/resolv.conf", ContainerPath: "/etc/resolv.conf"},
		{HostPath: "/run/ze-container/network/web/hosts", ContainerPath: "/etc/hosts"},
	}
	config := describeContainer(instanceConfig{name: "web", enabled: true}, "/run/ze-container/root/web", contributed)

	defaults := len(defaultMounts())
	if len(config.Mounts) != defaults+len(contributed) {
		t.Fatalf("the description holds %d mounts, want the %d defaults and the %d contributions", len(config.Mounts), defaults, len(contributed))
	}
	flags := unix.MS_BIND | unix.MS_RDONLY | unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC
	for i, mount := range contributed {
		got := config.Mounts[defaults+i]
		want := configs.Mount{Source: mount.HostPath, Destination: mount.ContainerPath, Device: "bind", Flags: flags}
		if !reflect.DeepEqual(*got, want) {
			t.Errorf("contribution %d is described as %+v, want %+v", i, *got, want)
		}
	}
}

// TestRuntimeMountPointsCoverTheDefaultMounts holds the copy in
// runtimeMountPoints to the description: every default mount lies at or
// under one of its paths, so no contribution can land on a mount the runtime
// makes.
//
// Method: each default mount's destination against the list.
func TestRuntimeMountPointsCoverTheDefaultMounts(t *testing.T) {
	for _, mount := range defaultMounts() {
		covered := slices.ContainsFunc(runtimeMountPoints, func(point string) bool { return isAtOrUnder(mount.Destination, point) })
		if !covered {
			t.Errorf("the default mount at %s lies under no path of runtimeMountPoints %v", mount.Destination, runtimeMountPoints)
		}
	}
}

// TestEntrypointIsTheImageEntrypointThenItsCommand proves the image's
// entrypoint and command become the process's arguments in that order, its
// variables gain a search path only when they hold none, its working
// directory is the image's or the root, a numeric user is used as written,
// and an image that names nothing to run or a user by name is refused.
func TestEntrypointIsTheImageEntrypointThenItsCommand(t *testing.T) {
	process, err := describeEntrypoint("web", Ready{Entrypoint: []string{"/probe"}, Command: []string{"run", "now"}, User: "1000:100"})
	if err != nil {
		t.Fatalf("the description refused an image with an entrypoint: %v", err)
	}
	if !slices.Equal(process.Args, []string{"/probe", "run", "now"}) {
		t.Errorf("the arguments are %v, want the entrypoint then the command", process.Args)
	}
	if !slices.Contains(process.Env, defaultPath) {
		t.Errorf("the variables %v hold no search path", process.Env)
	}
	if process.Cwd != "/" {
		t.Errorf("the working directory is %q, want / for an image that declares none", process.Cwd)
	}
	if process.UID != 1000 || process.GID != 100 {
		t.Errorf("the user is %d:%d, want 1000:100", process.UID, process.GID)
	}
	if !process.Init {
		t.Error("the entrypoint is not described as the container's init")
	}

	declared, err := describeEntrypoint("web", Ready{Command: []string{"/bin/sh"}, Environment: []string{"PATH=/bin"}, WorkingDirectory: "/data"})
	if err != nil {
		t.Fatalf("the description refused an image with a command: %v", err)
	}
	if !slices.Equal(declared.Env, []string{"PATH=/bin"}) || declared.Cwd != "/data" {
		t.Errorf("the image's variables and directory became %v and %q", declared.Env, declared.Cwd)
	}

	if _, err := describeEntrypoint("web", Ready{}); err == nil {
		t.Error("an image with no entrypoint and no command was accepted")
	}
	if _, err := describeEntrypoint("web", Ready{Entrypoint: []string{"/probe"}, User: "redis"}); err == nil {
		t.Error("an image user given by name was accepted, which this build cannot resolve")
	}
}
