// Design: plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md -- the complete path re-presents on a policy-checked device
// Related: espform_tun_linux.go -- openESPFormTUNClone, createESPFormTUNNode and the refusals under test
//
// VALIDATES: opening the TUN clone device /dev/net/tun. A missing node is created as
// c 10 200, mode 0600, in /dev/net, and only on ENOENT; a node that exists is opened
// and never replaced or changed; every failure is an operator error that names its
// cause, says the SA is refused for the inbound policy check (RFC 4301 Section 5.2)
// rather than for the configuration, and gives the fix for that cause.
// PREVENTS: the refusal a container with no /dev/net/tun met in interop ("open
// /dev/net/tun ...: no such file or directory", no fix); an mknod attempted for an
// errno that does not mean "absent"; a /dev/net left behind by a refused creation.
// METHOD: the errno mapping and the creation order run through an injected
// espFormTUNNodeOps, with no privilege. The real system calls run twice: in a user
// namespace with an empty tmpfs on /dev, where the kernel refuses mknod
// (capable(CAP_MKNOD) is judged in the initial user namespace), and as root in a
// private mount namespace, where Ze creates the node and the fd it opens is the TUN
// driver.

//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/test/userns"
)

// fakeTUNFD is the fd the fake open answers on success.
const fakeTUNFD = 7

// fakeTUNNode answers the clone device's system calls from fixed answers and records
// each call, with the arguments of the creating ones.
type fakeTUNNode struct {
	opens    []error // the answer of each open, in order; nil opens fakeTUNFD
	mkdirErr error
	mknodErr error
	rmdirErr error

	calls     []string
	mkdirPath string
	mkdirMode uint32
	mknodPath string
	mknodMode uint32
	mknodDev  int
	rmdirPath string
}

func (f *fakeTUNNode) ops() espFormTUNNodeOps {
	return espFormTUNNodeOps{
		open: func(path string, _ int, _ uint32) (int, error) {
			f.calls = append(f.calls, "open "+path)
			if len(f.opens) == 0 {
				return -1, errors.New("fakeTUNNode: an open nobody scripted")
			}
			err := f.opens[0]
			f.opens = f.opens[1:]
			if err != nil {
				return -1, err
			}
			return fakeTUNFD, nil
		},
		mkdir: func(path string, mode uint32) error {
			f.calls = append(f.calls, "mkdir")
			f.mkdirPath, f.mkdirMode = path, mode
			return f.mkdirErr
		},
		mknod: func(path string, mode uint32, dev int) error {
			f.calls = append(f.calls, "mknod")
			f.mknodPath, f.mknodMode, f.mknodDev = path, mode, dev
			return f.mknodErr
		},
		rmdir: func(path string) error {
			f.calls = append(f.calls, "rmdir")
			f.rmdirPath = path
			return f.rmdirErr
		},
	}
}

// requireTUNRefusal checks err is the operator error for one cause: its problem and its
// fix carry the given text, its why says the SA is refused for the inbound policy check
// and not for the configuration, and its chain holds every errno given.
func requireTUNRefusal(t *testing.T, err error, problem, fix string, errnos ...unix.Errno) {
	t.Helper()
	refusal, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("err = %v (%T), want an operator error", err, err)
	}
	if !strings.Contains(refusal.Problem(), problem) {
		t.Errorf("problem = %q, want it to contain %q", refusal.Problem(), problem)
	}
	if !strings.Contains(refusal.Fix(), fix) {
		t.Errorf("fix = %q, want it to contain %q", refusal.Fix(), fix)
	}
	for _, want := range []string{"RFC 4301 Section 5.2", "This is not a configuration error"} {
		if !strings.Contains(refusal.Why(), want) {
			t.Errorf("why = %q, want it to contain %q", refusal.Why(), want)
		}
	}
	for _, errno := range errnos {
		if !errors.Is(err, errno) {
			t.Errorf("err = %v, want its chain to hold %v", err, errno)
		}
	}
}

func requireCalls(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Errorf("calls = %q, want %q", got, want)
	}
}

// TestESPFormTUNOpenRefusalNamesTheCause opens a node that exists and refuses, once per
// errno, and checks the refusal names that errno's cause and fix and that Ze never
// tries to create a node: only ENOENT means the node is absent.
func TestESPFormTUNOpenRefusalNamesTheCause(t *testing.T) {
	cases := []struct {
		name    string
		errno   unix.Errno
		problem string
		fix     string
	}{
		{"device cgroup refuses", unix.EPERM, "the device cgroup refuses Ze access", "docker run --device /dev/net/tun"},
		{"mode or nodev refuses", unix.EACCES, "Ze has no permission to open the TUN device", "mount -o remount,dev /dev"},
		{"no TUN driver", unix.ENODEV, "the kernel has no TUN driver", "modprobe tun, or use a kernel built with CONFIG_TUN"},
		{"no driver for the major", unix.ENXIO, "the kernel has no TUN driver", "modprobe tun"},
		{"not a device", unix.EISDIR, "Ze cannot open the TUN device", "ls -l /dev/net/tun"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := &fakeTUNNode{opens: []error{tc.errno}}
			fd, err := openESPFormTUNClone(node.ops())
			if err == nil {
				t.Fatalf("open answered fd %d, want the %v refusal", fd, tc.errno)
			}
			requireTUNRefusal(t, err, tc.problem, tc.fix, tc.errno)
			requireCalls(t, node.calls, "open /dev/net/tun")
		})
	}
}

// TestESPFormTUNMissingNodeCreated answers ENOENT to the first open and checks that Ze
// creates /dev/net 0755 and the node c 10 200 mode 0600 in it, then opens the node.
func TestESPFormTUNMissingNodeCreated(t *testing.T) {
	node := &fakeTUNNode{opens: []error{unix.ENOENT, nil}}
	fd, err := openESPFormTUNClone(node.ops())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if fd != fakeTUNFD {
		t.Errorf("fd = %d, want %d, the reopen's", fd, fakeTUNFD)
	}
	requireCalls(t, node.calls, "open /dev/net/tun", "mkdir", "mknod", "open /dev/net/tun")
	if node.mkdirPath != "/dev/net" || node.mkdirMode != 0o755 {
		t.Errorf("mkdir(%q, %#o), want mkdir(\"/dev/net\", 0755)", node.mkdirPath, node.mkdirMode)
	}
	if node.mknodPath != "/dev/net/tun" {
		t.Errorf("mknod path = %q, want /dev/net/tun", node.mknodPath)
	}
	if node.mknodMode != unix.S_IFCHR|0o600 {
		t.Errorf("mknod mode = %#o, want S_IFCHR|0600", node.mknodMode)
	}
	if uint64(node.mknodDev) != unix.Mkdev(10, 200) { //nolint:gosec // the fake recorded Mkdev(10, 200) or a wrong value, both far inside uint64
		t.Errorf("mknod dev = %#x, want Mkdev(10, 200) = %#x", node.mknodDev, unix.Mkdev(10, 200))
	}
}

// TestESPFormTUNNodeCreationOutcomes drives each answer of mkdir and mknod and checks
// what Ze opens, refuses and removes. A /dev/net Ze created is removed when the node
// cannot be made; one that existed is never touched.
func TestESPFormTUNNodeCreationOutcomes(t *testing.T) {
	t.Run("directory exists, node created", func(t *testing.T) {
		node := &fakeTUNNode{opens: []error{unix.ENOENT, nil}, mkdirErr: unix.EEXIST}
		if _, err := openESPFormTUNClone(node.ops()); err != nil {
			t.Fatalf("open: %v", err)
		}
		requireCalls(t, node.calls, "open /dev/net/tun", "mkdir", "mknod", "open /dev/net/tun")
	})
	t.Run("node created by another process meanwhile", func(t *testing.T) {
		node := &fakeTUNNode{opens: []error{unix.ENOENT, nil}, mknodErr: unix.EEXIST}
		if _, err := openESPFormTUNClone(node.ops()); err != nil {
			t.Fatalf("open: %v", err)
		}
		requireCalls(t, node.calls, "open /dev/net/tun", "mkdir", "mknod", "open /dev/net/tun")
	})
	t.Run("mknod refused, created directory removed", func(t *testing.T) {
		node := &fakeTUNNode{opens: []error{unix.ENOENT}, mknodErr: unix.EPERM}
		_, err := openESPFormTUNClone(node.ops())
		requireTUNRefusal(t, err, "/dev/net/tun does not exist, and Ze could not create it",
			"docker run --device /dev/net/tun", unix.ENOENT, unix.EPERM)
		requireCalls(t, node.calls, "open /dev/net/tun", "mkdir", "mknod", "rmdir")
		if node.rmdirPath != "/dev/net" {
			t.Errorf("rmdir(%q), want rmdir(\"/dev/net\")", node.rmdirPath)
		}
	})
	t.Run("mknod refused, existing directory kept", func(t *testing.T) {
		node := &fakeTUNNode{opens: []error{unix.ENOENT}, mkdirErr: unix.EEXIST, mknodErr: unix.EROFS}
		_, err := openESPFormTUNClone(node.ops())
		requireTUNRefusal(t, err, "does not exist, and Ze could not create it", "modprobe tun", unix.ENOENT, unix.EROFS)
		requireCalls(t, node.calls, "open /dev/net/tun", "mkdir", "mknod")
	})
	t.Run("directory cannot be made", func(t *testing.T) {
		node := &fakeTUNNode{opens: []error{unix.ENOENT}, mkdirErr: unix.EROFS}
		_, err := openESPFormTUNClone(node.ops())
		requireTUNRefusal(t, err, "does not exist, and Ze could not create it",
			"mknod /dev/net/tun c 10 200", unix.ENOENT, unix.EROFS)
		requireCalls(t, node.calls, "open /dev/net/tun", "mkdir")
	})
	t.Run("removal of the created directory fails too", func(t *testing.T) {
		node := &fakeTUNNode{opens: []error{unix.ENOENT}, mknodErr: unix.EPERM, rmdirErr: unix.EBUSY}
		_, err := openESPFormTUNClone(node.ops())
		requireTUNRefusal(t, err, "does not exist", "docker run --device /dev/net/tun", unix.ENOENT, unix.EPERM, unix.EBUSY)
	})
	t.Run("node created, kernel has no TUN driver", func(t *testing.T) {
		node := &fakeTUNNode{opens: []error{unix.ENOENT, unix.ENODEV}}
		_, err := openESPFormTUNClone(node.ops())
		requireTUNRefusal(t, err, "the kernel has no TUN driver", "CONFIG_TUN", unix.ENODEV)
		// The node Ze made is the right one, and modprobe tun makes it work: kept.
		requireCalls(t, node.calls, "open /dev/net/tun", "mkdir", "mknod", "open /dev/net/tun")
	})
	t.Run("node created, device cgroup refuses it", func(t *testing.T) {
		node := &fakeTUNNode{opens: []error{unix.ENOENT, unix.EPERM}}
		_, err := openESPFormTUNClone(node.ops())
		requireTUNRefusal(t, err, "the device cgroup refuses Ze access", "docker run --device /dev/net/tun", unix.EPERM)
	})
}

// TestESPFormTUNSetIffRefusalNamesTheCause checks each TUNSETIFF errno maps to its
// cause: EPERM to CAP_NET_ADMIN, EBUSY and EINVAL to a name another holder took.
func TestESPFormTUNSetIffRefusalNamesTheCause(t *testing.T) {
	cases := []struct {
		errno   unix.Errno
		problem string
		fix     string
	}{
		{unix.EPERM, "Ze has no permission to create the ze-espform TUN interface", privilege.DaemonCapabilitiesFix},
		{unix.EBUSY, "an interface named ze-espform already exists", "ip link delete ze-espform"},
		{unix.EINVAL, "an interface named ze-espform already exists", "ip link delete ze-espform"},
		{unix.ENOMEM, "Ze cannot create the ze-espform TUN interface", "ip link show ze-espform"},
	}
	for _, tc := range cases {
		t.Run(tc.errno.Error(), func(t *testing.T) {
			requireTUNRefusal(t, espFormTUNSetIffRefusal(tc.errno), tc.problem, tc.fix, tc.errno)
		})
	}
	refusal, ok := errors.AsType[*opererr.Error](espFormTUNSetIffRefusal(unix.EPERM))
	if !ok {
		t.Fatalf("the EPERM refusal must be an operator error: %v", refusal)
	}
	if !strings.Contains(refusal.Why(), "CAP_NET_ADMIN") {
		t.Errorf("the EPERM refusal must say CAP_NET_ADMIN is needed: %v", refusal)
	}
	if !strings.Contains(refusal.Why(), "in Docker, start the container with --cap-add NET_ADMIN") {
		t.Errorf("the EPERM refusal must retain the Docker remedy: %v", refusal)
	}
	if refusal.Fix() != privilege.DaemonCapabilitiesFix {
		t.Errorf("the EPERM fix = %q, want %q", refusal.Fix(), privilege.DaemonCapabilitiesFix)
	}
}

// TestESPFormTUNNodeUserNamespace runs the real system calls in a user namespace over
// an empty tmpfs on /dev. Goal: mknod is attempted only on ENOENT, a refused creation
// leaves nothing behind, and a node that exists is opened and left unchanged. The
// kernel refuses mknod here because vfs_mknod asks capable(CAP_MKNOD), which a user
// namespace's root does not hold in the initial user namespace.
func TestESPFormTUNNodeUserNamespace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	if err := unix.Mount("tmpfs", "/dev", "tmpfs", unix.MS_NOSUID, "mode=0755"); err != nil {
		t.Fatalf("mount an empty tmpfs on /dev in the test's own mount namespace: %v", err)
	}
	t.Cleanup(func() {
		if err := unix.Unmount("/dev", unix.MNT_DETACH); err != nil {
			t.Errorf("unmount the test's tmpfs from /dev: %v", err)
		}
	})

	t.Run("missing node is refused and nothing is left", func(t *testing.T) {
		_, err := openESPFormTUNClone(espFormTUNHostOps)
		requireTUNRefusal(t, err, "/dev/net/tun does not exist, and Ze could not create it",
			"docker run --device /dev/net/tun", unix.ENOENT, unix.EPERM)
		var st unix.Stat_t
		if lerr := unix.Lstat("/dev/net", &st); !errors.Is(lerr, unix.ENOENT) {
			t.Errorf("lstat /dev/net = %v, want ENOENT: the directory Ze created must be removed", lerr)
		}
	})
	t.Run("existing node is opened and left as it is", func(t *testing.T) {
		t.Cleanup(func() { removeDevNet(t) })
		placeRegularTUNFile(t, 0o640)
		fd, err := openESPFormTUNClone(espFormTUNHostOps)
		if err != nil {
			t.Fatalf("open an existing /dev/net/tun: %v", err)
		}
		if cerr := unix.Close(fd); cerr != nil {
			t.Errorf("close: %v", cerr)
		}
		var st unix.Stat_t
		if lerr := unix.Lstat("/dev/net/tun", &st); lerr != nil {
			t.Fatalf("lstat /dev/net/tun: %v", lerr)
		}
		if st.Mode&unix.S_IFMT != unix.S_IFREG {
			t.Errorf("/dev/net/tun type = %#o, want the regular file the test placed: Ze replaced it", st.Mode&unix.S_IFMT)
		}
		if st.Mode&0o777 != 0o640 {
			t.Errorf("/dev/net/tun mode = %#o, want 0640: Ze changed the mode of a node that exists", st.Mode&0o777)
		}
	})
	t.Run("directory at the node path is refused and nothing is created", func(t *testing.T) {
		t.Cleanup(func() { removeDevNet(t) })
		if err := os.MkdirAll("/dev/net/tun", 0o755); err != nil {
			t.Fatalf("make /dev/net/tun a directory: %v", err)
		}
		_, err := openESPFormTUNClone(espFormTUNHostOps)
		requireTUNRefusal(t, err, "Ze cannot open the TUN device", "ls -l /dev/net/tun", unix.EISDIR)
		var st unix.Stat_t
		if lerr := unix.Lstat("/dev/net/tun", &st); lerr != nil || st.Mode&unix.S_IFMT != unix.S_IFDIR {
			t.Errorf("/dev/net/tun after the refusal: lstat %v, type %#o; want the directory left alone", lerr, st.Mode&unix.S_IFMT)
		}
	})
}

// placeRegularTUNFile puts a regular file with the given mode at /dev/net/tun.
func placeRegularTUNFile(t *testing.T, mode uint32) {
	t.Helper()
	if err := os.MkdirAll("/dev/net", 0o755); err != nil {
		t.Fatalf("make /dev/net: %v", err)
	}
	fd, err := unix.Open("/dev/net/tun", unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY|unix.O_CLOEXEC, mode)
	if err != nil {
		t.Fatalf("create /dev/net/tun as a regular file: %v", err)
	}
	if err := unix.Close(fd); err != nil {
		t.Fatalf("close /dev/net/tun: %v", err)
	}
	// The umask may have narrowed the mode; the test sets the one it asserts.
	if err := unix.Chmod("/dev/net/tun", mode); err != nil {
		t.Fatalf("chmod /dev/net/tun %#o: %v", mode, err)
	}
}

func removeDevNet(t *testing.T) {
	t.Helper()
	if err := os.RemoveAll("/dev/net"); err != nil {
		t.Errorf("remove the test's /dev/net: %v", err)
	}
}

// tunNodeResult is what createTUNNodeInPrivateDev observed on its own thread.
type tunNodeResult struct {
	err      error
	node     unix.Stat_t
	dir      unix.Stat_t
	features uint32
}

// TestESPFormTUNNodeCreatedAsRoot hides /dev behind an empty tmpfs in a private mount
// namespace on one locked thread, and checks that Ze creates /dev/net and the node
// c 10 200 mode 0600, and that the fd it opens is the TUN driver (TUNGETFEATURES
// answers IFF_TUN). The host's /dev is never touched: the thread's mount namespace is
// its own, and the thread exits with the test's goroutine.
//
//ze:guest-root
func TestESPFormTUNNodeCreatedAsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires privileged QEMU: vfs_mknod creates a character device only for capable(CAP_MKNOD) in the initial user namespace, which userns.Enter cannot grant; run `./le test qemu all`")
	}
	results := make(chan tunNodeResult, 1)
	go createTUNNodeInPrivateDev(results)
	r := <-results
	if r.err != nil {
		t.Fatalf("create and open /dev/net/tun under an empty /dev: %v", r.err)
	}
	if r.dir.Mode&unix.S_IFMT != unix.S_IFDIR {
		t.Errorf("/dev/net type = %#o, want a directory", r.dir.Mode&unix.S_IFMT)
	}
	if r.node.Mode&unix.S_IFMT != unix.S_IFCHR {
		t.Errorf("/dev/net/tun type = %#o, want a character device", r.node.Mode&unix.S_IFMT)
	}
	if r.node.Mode&0o777 != 0o600 {
		t.Errorf("/dev/net/tun mode = %#o, want 0600", r.node.Mode&0o777)
	}
	if uint64(r.node.Rdev) != unix.Mkdev(10, 200) { //nolint:unconvert // Rdev is uint32 on some GOARCH values
		t.Errorf("/dev/net/tun device = %#x, want Mkdev(10, 200) = %#x", r.node.Rdev, unix.Mkdev(10, 200))
	}
	if r.features&unix.IFF_TUN == 0 {
		t.Errorf("TUNGETFEATURES = %#x, want IFF_TUN set: the fd Ze opened is not the TUN driver", r.features)
	}
}

// createTUNNodeInPrivateDev runs on a thread it locks and never unlocks: the thread
// holds a private mount namespace, so it must exit with this goroutine rather than go
// back to the scheduler, where another goroutine would see the empty /dev.
func createTUNNodeInPrivateDev(results chan<- tunNodeResult) {
	runtime.LockOSThread()
	var r tunNodeResult
	defer func() { results <- r }()

	if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
		r.err = fmt.Errorf("unshare a mount namespace: %w", err)
		return
	}
	if err := unix.Mount("", "/", "", unix.MS_PRIVATE|unix.MS_REC, ""); err != nil {
		r.err = fmt.Errorf("make the thread's mounts private: %w", err)
		return
	}
	if err := unix.Mount("tmpfs", "/dev", "tmpfs", unix.MS_NOSUID, "mode=0755"); err != nil {
		r.err = fmt.Errorf("mount an empty tmpfs on /dev: %w", err)
		return
	}
	fd, err := openESPFormTUNClone(espFormTUNHostOps)
	if err != nil {
		r.err = err
		return
	}
	r.features, err = unix.IoctlGetUint32(fd, unix.TUNGETFEATURES)
	if cerr := unix.Close(fd); cerr != nil {
		r.err = errors.Join(r.err, fmt.Errorf("close: %w", cerr))
	}
	if err != nil {
		r.err = errors.Join(r.err, fmt.Errorf("TUNGETFEATURES: %w", err))
		return
	}
	if err := unix.Lstat("/dev/net/tun", &r.node); err != nil {
		r.err = errors.Join(r.err, fmt.Errorf("lstat /dev/net/tun: %w", err))
		return
	}
	if err := unix.Lstat("/dev/net", &r.dir); err != nil {
		r.err = errors.Join(r.err, fmt.Errorf("lstat /dev/net: %w", err))
	}
}
