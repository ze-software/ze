package storage

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// bootRecordFixture binds a memory store to an explicit source and arms a boot
// record that writes a candidate, closes wrote, then blocks until release is
// closed before it promotes, as the hub's initializeConfigSource does for an
// explicit file. A test acts while the record is pending, with the record's own
// transient candidate already in the store.
func bootRecordFixture(t *testing.T, content []byte, recordErr error) (Storage, *BootRecord, chan struct{}) {
	t.Helper()
	store, record, _, release := bootRecordFixtureStaged(t, content, recordErr)
	return store, record, release
}

// bootRecordFixtureStaged is bootRecordFixture with the wrote channel exposed.
func bootRecordFixtureStaged(t *testing.T, content []byte, recordErr error) (Storage, *BootRecord, chan struct{}, chan struct{}) {
	t.Helper()
	raw := CreateMemory()
	t.Cleanup(func() { _ = raw.Close() })
	store := BindConfigSource(raw, "/etc/ze/router.conf", ConfigSourceFile)
	record, err := DeferBootRecord(store, content)
	if err != nil {
		t.Fatalf("DeferBootRecord: %v", err)
	}
	wrote := make(chan struct{})
	release := make(chan struct{})
	record.Start(func(view Storage) error {
		if recordErr != nil {
			close(wrote)
			<-release
			return recordErr
		}
		_, err := WriteCandidateVersion(view, "/etc/ze/router.conf", content, time.Now())
		close(wrote)
		<-release
		if err != nil {
			return err
		}
		return PromoteCandidate(view, "/etc/ze/router.conf")
	})
	return store, record, wrote, release
}

// TestBootRecordPendingReadAnswersBootBytes: while the record is held after it
// wrote its transient candidate, the bound name reads as the boot bytes with no
// candidate, although the store holds no active version and does hold that
// candidate, so both answers come from the gate; another key's lock is not
// held back.
func TestBootRecordPendingReadAnswersBootBytes(t *testing.T) {
	content := []byte("system { host-name a; }\n")
	store, record, wrote, release := bootRecordFixtureStaged(t, content, nil)
	defer func() { close(release); record.Wait() }()
	<-wrote

	got, err := ReadActiveConfig(store, "/etc/ze/router.conf")
	if err != nil {
		t.Fatalf("ReadActiveConfig while pending: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("pending active = %q, want boot bytes %q", got, content)
	}
	if _, _, present, err := ReadCandidateConfig(store, "/etc/ze/router.conf"); err != nil || present {
		t.Fatalf("pending candidate present=%v err=%v, want absent", present, err)
	}
	guard, err := store.AcquireLock("state/ssh/host-key")
	if err != nil {
		t.Fatalf("lock of another key waited for the boot record: %v", err)
	}
	_ = guard.Release()
}

// TestBootRecordPendingReadFileAnswersBootBytes: a plain ReadFile of the bound
// name, the read an editor opens the configuration with, answers the boot bytes
// while the record is pending, although the store holds no file/active entry
// yet; once the record lands it answers the mirror the promotion wrote. Before
// the gate covered ReadFile, a runtime "update bgp config" issued during the
// window failed with "storage node .../file/active: no such file or directory".
func TestBootRecordPendingReadFileAnswersBootBytes(t *testing.T) {
	content := []byte("system { host-name a; }\n")
	store, record, wrote, release := bootRecordFixtureStaged(t, content, nil)
	<-wrote

	got, err := store.ReadFile("/etc/ze/router.conf")
	if err != nil {
		close(release)
		record.Wait()
		t.Fatalf("ReadFile of the bound name while pending: %v", err)
	}
	if !bytes.Equal(got, content) {
		close(release)
		record.Wait()
		t.Fatalf("pending ReadFile = %q, want boot bytes %q", got, content)
	}
	if _, err := store.ReadFile("/etc/ze/other.conf"); err == nil {
		close(release)
		record.Wait()
		t.Fatal("ReadFile of another name answered while the record is pending, want its own miss")
	}

	close(release)
	record.Wait()
	landed, err := store.ReadFile("/etc/ze/router.conf")
	if err != nil {
		t.Fatalf("ReadFile after the record landed: %v", err)
	}
	if !bytes.Equal(landed, content) {
		t.Fatalf("landed ReadFile = %q, want %q", landed, content)
	}
}

// TestBootRecordPendingExistsAnswersLikeReadFile: Exists of the bound name
// answers true while the record is pending, as ReadFile answers the boot bytes,
// and another name keeps its own miss. Before the gate covered Exists, the web
// server's startup check (cmd/ze/hub/service_web.go, startWebServer) saw no
// file in that window and wrote a "# ze config" placeholder over the name the
// daemon was booting from.
func TestBootRecordPendingExistsAnswersLikeReadFile(t *testing.T) {
	content := []byte("system { host-name a; }\n")
	store, record, wrote, release := bootRecordFixtureStaged(t, content, nil)
	<-wrote

	pendingBound := store.Exists("/etc/ze/router.conf")
	pendingOther := store.Exists("/etc/ze/other.conf")
	close(release)
	record.Wait()
	if !pendingBound {
		t.Fatal("Exists of the bound name while pending = false, want true: ReadFile answers its boot bytes")
	}
	if pendingOther {
		t.Fatal("Exists of another name while pending = true, want its own miss")
	}
	if !store.Exists("/etc/ze/router.conf") {
		t.Fatal("Exists of the bound name after the record landed = false, want true")
	}
}

// TestBootRecordWriteWaitsThenLands: a commit attempted while the record is
// pending does not run until the record lands, then lands on top of it; the
// active pointer first names the boot bytes, then the commit.
func TestBootRecordWriteWaitsThenLands(t *testing.T) {
	content := []byte("boot\n")
	store, record, release := bootRecordFixture(t, content, nil)

	committed := make(chan error, 1)
	go commitForTest(store, committed)
	select {
	case err := <-committed:
		t.Fatalf("commit ran before the boot record landed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-committed; err != nil {
		t.Fatalf("commit after the record: %v", err)
	}
	record.Wait()
	versions, err := store.ListVersions("/etc/ze/router.conf")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("versions = %d, want 2 (boot record, then the commit)", len(versions))
	}
	got, err := ReadActiveConfig(store, "/etc/ze/router.conf")
	if err != nil || string(got) != "commit\n" {
		t.Fatalf("active = %q err=%v, want the commit", got, err)
	}
}

func commitForTest(store Storage, result chan<- error) {
	if _, err := WriteCandidateVersion(store, "/etc/ze/router.conf", []byte("commit\n"), time.Now().Add(time.Second)); err != nil {
		result <- err
		return
	}
	result <- PromoteCandidate(store, "/etc/ze/router.conf")
}

// TestBootRecordWaitBound: a write still pending at the bound is refused with
// an error that names the cause and the retry, and writes nothing.
func TestBootRecordWaitBound(t *testing.T) {
	saved := bootRecordWaitMax
	bootRecordWaitMax = 50 * time.Millisecond
	t.Cleanup(func() { bootRecordWaitMax = saved })

	store, record, release := bootRecordFixture(t, []byte("boot\n"), nil)
	defer func() { close(release); record.Wait() }()

	_, err := WriteCandidateVersion(store, "/etc/ze/router.conf", []byte("x\n"), time.Now())
	if err == nil {
		t.Fatal("write past the bound succeeded")
	}
	if !strings.Contains(err.Error(), "still recording the boot configuration") || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("refusal = %q, want cause and retry", err)
	}
}

// TestBootRecordFailureRefusesWrites: a failed record is kept: every later
// write returns it, and reads keep answering the boot bytes the daemon runs.
func TestBootRecordFailureRefusesWrites(t *testing.T) {
	failure := errors.New("disk full")
	store, record, release := bootRecordFixture(t, []byte("boot\n"), failure)
	close(release)
	record.Wait()

	_, err := WriteCandidateVersion(store, "/etc/ze/router.conf", []byte("x\n"), time.Now())
	if !errors.Is(err, failure) {
		t.Fatalf("write after failed record = %v, want the record's failure", err)
	}
	if !strings.Contains(err.Error(), "ze data check") {
		t.Fatalf("refusal %q does not name the fix", err)
	}
	got, err := ReadActiveConfig(store, "/etc/ze/router.conf")
	if err != nil || string(got) != "boot\n" {
		t.Fatalf("active after failed record = %q err=%v, want boot bytes", got, err)
	}
}

// TestBootRecordNeverStarted: Wait on a record that never started releases
// every waiter with a refusal instead of leaving it to the bound. The bound is
// cut short, so a Wait that failed to release the gate fails at once with the
// still-recording refusal instead of hanging the test to its timeout.
func TestBootRecordNeverStarted(t *testing.T) {
	saved := bootRecordWaitMax
	bootRecordWaitMax = 50 * time.Millisecond
	t.Cleanup(func() { bootRecordWaitMax = saved })

	raw := CreateMemory()
	defer func() { _ = raw.Close() }()
	store := BindConfigSource(raw, "/etc/ze/router.conf", ConfigSourceFile)
	record, err := DeferBootRecord(store, []byte("boot\n"))
	if err != nil {
		t.Fatal(err)
	}
	record.Wait()
	if _, err := WriteCandidateVersion(store, "/etc/ze/router.conf", []byte("x\n"), time.Now()); !errors.Is(err, errBootRecordNotRun) {
		t.Fatalf("write after an unstarted record = %v, want errBootRecordNotRun", err)
	}
}

// TestBootRecordCoversFullKey: the gate concerns the store key the bound path
// resolves to. A namespaced key that only ends in the same base name is another
// key and is not gated; the resolved key itself, and the bound path, are.
func TestBootRecordCoversFullKey(t *testing.T) {
	raw := CreateMemory()
	defer func() { _ = raw.Close() }()
	store := BindConfigSource(raw, "/etc/ze/router.conf", ConfigSourceFile)
	record, err := DeferBootRecord(store, []byte("boot\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer record.Wait()
	cases := []struct {
		path string
		want bool
	}{
		{"/etc/ze/router.conf", true},
		{"file/active/router.conf", true},
		{"meta/history/router.conf", false},
		{"/etc/ze/other.conf", false},
		{"", false},
	}
	for _, c := range cases {
		if got := record.covers(c.path); got != c.want {
			t.Errorf("covers(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
