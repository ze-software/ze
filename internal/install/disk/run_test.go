// VALIDATES: ISSUE-2 (bounded poll for partition node after BLKRRPART)
// PREVENTS: race where mount of part-4 fails because devtmpfs has not yet created the node

package disk

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWaitForPartitionNodeAppears(t *testing.T) {
	dir := t.TempDir()
	node := filepath.Join(dir, "sda4")

	go func() {
		time.Sleep(100 * time.Millisecond)
		f, err := os.Create(node)
		if err != nil {
			t.Error(err)
			return
		}
		f.Close() //nolint:errcheck // test
	}()

	origInterval := partPollInterval
	partPollInterval = 50 * time.Millisecond
	defer func() { partPollInterval = origInterval }()

	err := waitForPartitionNode(node, 2*time.Second)
	if err != nil {
		t.Fatalf("waitForPartitionNode: %v", err)
	}
}

func TestWaitForPartitionNodeTimeout(t *testing.T) {
	node := "/dev/nonexistent-test-partition-xyz"

	origInterval := partPollInterval
	partPollInterval = 50 * time.Millisecond
	defer func() { partPollInterval = origInterval }()

	err := waitForPartitionNode(node, 200*time.Millisecond)
	if err == nil {
		t.Fatal("waitForPartitionNode should fail when node never appears")
	}
}

// installTrace records the order in which an install reaches its seams. The
// image server answers on its own goroutine, so the record is locked.
type installTrace struct {
	mu    sync.Mutex
	steps []string
}

func (trace *installTrace) add(step string) {
	trace.mu.Lock()
	defer trace.mu.Unlock()
	trace.steps = append(trace.steps, step)
}

func (trace *installTrace) recorded() []string {
	trace.mu.Lock()
	defer trace.mu.Unlock()
	return slices.Clone(trace.steps)
}

// traceInstallSeams replaces every seam an install crosses with a fake that
// records its call, so runHTTP and runISO run whole in a test and touch no
// device. disk is a path in a temporary folder that the fakes answer as the
// target disk, and the persistent partition is mounted beside it.
func traceInstallSeams(t *testing.T, disk string) *installTrace {
	t.Helper()
	trace := &installTrace{}

	oldMount, oldUnmount, oldSync := mountFS, umountFS, syncFS
	oldReboot, oldPoweroff, oldReread := rebootFS, poweroffFS, blkRereadPart
	oldSelect, oldHTTPWrite, oldISOWrite := selectTargetDisk, writeHTTPImage, writeISOImage
	oldLocate, oldTable, oldResize, oldPerm := locateISOMedia, growPermTable, growPermFS, permMount
	t.Cleanup(func() {
		mountFS, umountFS, syncFS = oldMount, oldUnmount, oldSync
		rebootFS, poweroffFS, blkRereadPart = oldReboot, oldPoweroff, oldReread
		selectTargetDisk, writeHTTPImage, writeISOImage = oldSelect, oldHTTPWrite, oldISOWrite
		locateISOMedia, growPermTable, growPermFS, permMount = oldLocate, oldTable, oldResize, oldPerm
	})

	permMount = filepath.Join(filepath.Dir(disk), "perm")
	mountFS = func(_, target, _ string, _ bool) error {
		trace.add("mount " + filepath.Base(target))
		return nil
	}
	umountFS = func(target string) error {
		trace.add("unmount " + filepath.Base(target))
		return nil
	}
	syncFS = func() { trace.add("sync") }
	rebootFS = func() { trace.add("reboot") }
	poweroffFS = func() { trace.add("poweroff") }
	blkRereadPart = func(target string) error {
		trace.add("reread " + filepath.Base(target))
		return nil
	}
	selectTargetDisk = func(string, []string, string) (string, error) { return disk, nil }
	writeHTTPImage = func(_, target, _ string) error {
		trace.add("write " + filepath.Base(target))
		return nil
	}
	writeISOImage = func(_, _, target string) error {
		trace.add("write " + filepath.Base(target))
		return nil
	}
	locateISOMedia = func(string, string) (*isoMedia, error) {
		return &isoMedia{sourceDev: "/dev/sr0", sourceDisk: "sr0", mountPoint: isoMount}, nil
	}
	growPermTable = func(target string) error {
		trace.add("grow-table " + filepath.Base(target))
		return nil
	}
	growPermFS = func(mountPoint, partition string) error {
		trace.add("resize " + filepath.Base(mountPoint) + " " + filepath.Base(partition))
		return nil
	}
	return trace
}

// installTargetDisk answers the path of a target disk in a temporary folder
// and creates the node of its fourth partition, which the install waits for.
func installTargetDisk(t *testing.T) string {
	t.Helper()
	disk := filepath.Join(t.TempDir(), "disk")
	if err := os.WriteFile(partitionPath(disk, 4), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return disk
}

// TestRunHTTPGrowsPersistentPartition runs the HTTP install whole, from runHTTP
// to the reboot, against an image server and fakes of every seam.
//
// VALIDATES: the partition table is grown after the image is written and
// before the kernel rereads it, and the filesystem is resized inside the mount
// of the persistent partition, after the seed and before the unmount.
// PREVENTS: an install that leaves the persistent partition at the size the
// image gave it, or that resizes a filesystem whose partition the kernel has
// not reread.
func TestRunHTTPGrowsPersistentPartition(t *testing.T) {
	disk := installTargetDisk(t)
	trace := traceInstallSeams(t, disk)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/install/image/ze.img.sha256":
			// A valid digest, so the installer does not retry the download.
			if _, err := w.Write([]byte(strings.Repeat("a", 64) + "  ze.img\n")); err != nil {
				t.Error(err)
			}
		case "/install/database.zefs":
			trace.add("seed")
			if _, err := w.Write([]byte("bootstrap seed")); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	host, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}

	// ze.wait=0 skips the server probe, so no network seam is crossed.
	cfg := installConfig{Source: sourceHTTP, Server: host, Port: port, Image: "ze.img", Wait: "0"}
	if code := runHTTP(cfg); code != 0 {
		t.Fatalf("runHTTP = %d, want 0; steps %v", code, trace.recorded())
	}

	want := []string{
		"write disk", "sync",
		"grow-table disk",
		"reread disk",
		"mount perm", "seed", "sync",
		"resize perm disk4", "sync",
		"unmount perm",
		"reboot",
	}
	if got := trace.recorded(); !slices.Equal(got, want) {
		t.Errorf("HTTP install steps:\n got %v\nwant %v", got, want)
	}
}

// TestRunISOGrowsPersistentPartition runs the ISO install whole, from runISO to
// the poweroff, against fakes of every seam.
//
// VALIDATES: the ISO install grows the table before the reread, then mounts
// the persistent partition itself, resizes its filesystem and unmounts it,
// before it releases the medium and powers off.
// PREVENTS: an ISO install that never mounts the persistent partition and so
// never grows its filesystem.
func TestRunISOGrowsPersistentPartition(t *testing.T) {
	disk := installTargetDisk(t)
	trace := traceInstallSeams(t, disk)

	cfg := installConfig{Source: sourceISO, Image: "ze.img", MediaID: strings.Repeat("0", 32)}
	if code := runISO(cfg); code != 0 {
		t.Fatalf("runISO = %d, want 0; steps %v", code, trace.recorded())
	}

	want := []string{
		"write disk", "sync",
		"grow-table disk",
		"sync", "reread disk",
		"mount perm",
		"resize perm disk4", "sync",
		"unmount perm",
		"unmount iso",
		"poweroff",
	}
	if got := trace.recorded(); !slices.Equal(got, want) {
		t.Errorf("ISO install steps:\n got %v\nwant %v", got, want)
	}
}
