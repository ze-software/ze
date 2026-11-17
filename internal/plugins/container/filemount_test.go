// Design: docs/architecture/container/runtime.md -- the description contribution: what a participant may mount

package container

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// hostFiles makes, in a fresh directory, a regular file, a directory and a
// symbolic link to the file, and answers their paths.
func hostFiles(t *testing.T) (file, directory, link string) {
	t.Helper()
	root := t.TempDir()
	file = filepath.Join(root, "resolv.conf")
	if err := os.WriteFile(file, []byte("nameserver 192.0.2.53\n"), 0o600); err != nil {
		t.Fatalf("write the host file: %v", err)
	}
	directory = filepath.Join(root, "directory")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("make the host directory: %v", err)
	}
	link = filepath.Join(root, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Fatalf("make the host link: %v", err)
	}
	return file, directory, link
}

// TestCheckFileMountAdmitsOnlyAReadOnlyRegularFile proves what a participant
// may contribute: a regular host file at a clean absolute host path, mounted
// at a clean absolute container path outside the runtime's own mounts and
// named once. Each refusal names what is wrong.
//
// Method: one admitted mount, then each way out of the rule, one at a time.
func TestCheckFileMountAdmitsOnlyAReadOnlyRegularFile(t *testing.T) {
	file, directory, link := hostFiles(t)
	if err := checkFileMount(FileMount{HostPath: file, ContainerPath: "/etc/resolv.conf"}, nil); err != nil {
		t.Fatalf("a regular file at /etc/resolv.conf was refused: %v", err)
	}

	admitted := []FileMount{{HostPath: file, ContainerPath: "/etc/hosts"}}
	cases := []struct {
		name  string
		mount FileMount
		says  string
	}{
		{"relative container path", FileMount{HostPath: file, ContainerPath: "etc/resolv.conf"}, "not absolute"},
		{"unclean container path", FileMount{HostPath: file, ContainerPath: "/etc/../resolv.conf"}, "not in clean form"},
		{"trailing separator", FileMount{HostPath: file, ContainerPath: "/etc/resolv.conf/"}, "not in clean form"},
		{"the root", FileMount{HostPath: file, ContainerPath: "/"}, "container's root"},
		{"under proc", FileMount{HostPath: file, ContainerPath: "/proc/sys/kernel/hostname"}, "/proc"},
		{"the device directory itself", FileMount{HostPath: file, ContainerPath: "/dev"}, "/dev"},
		{"under shared memory", FileMount{HostPath: file, ContainerPath: "/dev/shm/x"}, "/dev"},
		{"under the cgroup tree", FileMount{HostPath: file, ContainerPath: "/sys/fs/cgroup/x"}, "/sys"},
		{"a path contributed twice", FileMount{HostPath: file, ContainerPath: "/etc/hosts"}, "twice"},
		{"relative host path", FileMount{HostPath: "resolv.conf", ContainerPath: "/etc/resolv.conf"}, "not absolute"},
		{"unclean host path", FileMount{HostPath: filepath.Dir(file) + "/./resolv.conf", ContainerPath: "/etc/resolv.conf"}, "not in clean form"},
		{"a missing host file", FileMount{HostPath: file + ".absent", ContainerPath: "/etc/resolv.conf"}, "cannot be read"},
		{"a host directory", FileMount{HostPath: directory, ContainerPath: "/etc/resolv.conf"}, "not a regular file"},
		{"a host symbolic link", FileMount{HostPath: link, ContainerPath: "/etc/resolv.conf"}, "not a regular file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkFileMount(c.mount, admitted)
			if err == nil {
				t.Fatalf("the mount %+v was admitted", c.mount)
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Fatalf("the refusal %q does not say %q", err, c.says)
			}
		})
	}
}

// mountingParticipant contributes mounts and records nothing else.
type mountingParticipant struct {
	mounts []FileMount
}

func (p *mountingParticipant) Name() string { return "network" }

func (p *mountingParticipant) Contribute(context.Context, string) ([]FileMount, error) {
	return p.mounts, nil
}

func (p *mountingParticipant) Created(context.Context, CreatedCall) error { return nil }

func (p *mountingParticipant) Ended(context.Context, string) {}

// mountRecordingLibrary is a fakeLibrary that records the mounts of each
// create request.
type mountRecordingLibrary struct {
	*fakeLibrary
	mounts []FileMount
}

func (l *mountRecordingLibrary) create(ctx context.Context, request *startRequest) (createdContainer, error) {
	l.mounts = slices.Clone(request.mounts)
	return l.fakeLibrary.create(ctx, request)
}

// contributionFixture is a supervisor holding one enabled container, web,
// whose image the store holds, with one participant contributing mounts.
func contributionFixture(t *testing.T, mounts []FileMount) (*supervisor, *mountRecordingLibrary, *eventLog) {
	t.Helper()
	events := &eventLog{}
	store := &fakeStore{}
	store.setAnswer(readyImage())
	library := &mountRecordingLibrary{fakeLibrary: &fakeLibrary{events: events, initProcess: 4242, networkNamespace: "/proc/4242/ns/net"}}
	sup := newSupervisor(slogutil.DiscardLogger(), library, storeFactoryOf(store), []Participant{&mountingParticipant{mounts: mounts}})
	t.Cleanup(sup.close)
	if err := sup.install(configOf(instanceConfig{name: "web", enabled: true})); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	return sup, library, events
}

// TestAdmittedContributionReachesTheCreate proves a contributed mount is
// part of what the runtime library is asked to create, never dropped.
//
// Method: a participant contributes one regular file; the library records
// the request's mounts.
func TestAdmittedContributionReachesTheCreate(t *testing.T) {
	file, _, _ := hostFiles(t)
	mounts := []FileMount{{HostPath: file, ContainerPath: "/etc/resolv.conf"}}
	sup, library, _ := contributionFixture(t, mounts)

	if _, err := sup.start(context.Background(), "web"); err != nil {
		t.Fatalf("the start failed: %v", err)
	}
	if !slices.Equal(library.mounts, mounts) {
		t.Errorf("the create was asked for the mounts %+v, want %+v", library.mounts, mounts)
	}
	if err := sup.stop(context.Background(), "web"); err != nil {
		t.Fatalf("the stop failed: %v", err)
	}
}

// TestInadmissibleContributionFailsTheStart proves a mount the runtime does
// not admit fails the start before the create, naming the participant and
// what is wrong, rather than reaching the library or being dropped.
//
// Method: a participant contributes a file under /proc; the event log shows
// no create.
func TestInadmissibleContributionFailsTheStart(t *testing.T) {
	file, _, _ := hostFiles(t)
	sup, _, events := contributionFixture(t, []FileMount{{HostPath: file, ContainerPath: "/proc/self/environ"}})

	_, err := sup.start(context.Background(), "web")

	if err == nil {
		t.Fatalf("a start with a mount under /proc succeeded")
	}
	if !strings.Contains(err.Error(), "network") {
		t.Errorf("the refusal %q does not name the participant, network", err)
	}
	if !strings.Contains(err.Error(), "/proc") {
		t.Errorf("the refusal %q does not name the runtime's mount it lies under", err)
	}
	if got := events.all(); slices.Contains(got, "create web") {
		t.Fatalf("the library was asked to create the container: %v", got)
	}
}
