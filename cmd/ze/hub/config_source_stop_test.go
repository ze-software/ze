// Design: docs/architecture/storage-backends.md -- the boot record
// Related: config_source.go -- awaitBootRecordAtStop

package hub

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
)

const stopRecordLine = "Stopping: the store is still recording the boot configuration"

// startHeldBootRecord arms and starts a boot record on a memory store whose
// record blocks until release is closed, as one does whose syncs wait behind
// another writer on a slow disk.
func startHeldBootRecord(t *testing.T) (*storage.BootRecord, chan struct{}) {
	t.Helper()
	bound := storage.BindConfigSource(storage.CreateMemory(), "/etc/ze/ze.conf", storage.ConfigSourceFile)
	record, err := storage.DeferBootRecord(bound, []byte("set environment log level warn\n"))
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	record.Start(func(storage.Storage) error {
		<-release
		return nil
	})
	return record, release
}

// awaitStop runs awaitBootRecordAtStop and closes the returned channel when it
// returns.
func awaitStop(record *storage.BootRecord, out *bytes.Buffer) chan struct{} {
	returned := make(chan struct{})
	go func() {
		awaitBootRecordAtStop(record, out)
		close(returned)
	}()
	return returned
}

// A stop arriving while the boot record is still running holds until the
// record returns, and tells the operator first what it waits for and how to
// force it, so "Ze stopped." is never printed while the process still writes.
// Method: hold the record, run the stop wait, prove it does not return while
// held, release, then read what it printed.
func TestStopWaitsForPendingBootRecordAndSaysSo(t *testing.T) {
	record, release := startHeldBootRecord(t)
	var out bytes.Buffer
	returned := awaitStop(record, &out)

	select {
	case <-returned:
		t.Fatal("the stop returned while the boot record was still running")
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	select {
	case <-returned:
	case <-time.After(10 * time.Second):
		t.Fatal("the stop did not return after the boot record did")
	}

	printed := out.String()
	for _, want := range []string{stopRecordLine, "the disk is slow", "Ctrl+C again", "the next start records it"} {
		if !strings.Contains(printed, want) {
			t.Fatalf("stop line does not say %q:\n%s", want, printed)
		}
	}
}

// A stop after the record landed prints nothing and does not wait.
func TestStopAfterBootRecordLandedIsSilent(t *testing.T) {
	record, release := startHeldBootRecord(t)
	close(release)
	select {
	case <-record.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the boot record never returned")
	}
	var out bytes.Buffer
	select {
	case <-awaitStop(record, &out):
	case <-time.After(10 * time.Second):
		t.Fatal("the stop waited for a record that had landed")
	}
	if out.Len() != 0 {
		t.Fatalf("stop printed over a landed record: %q", out.String())
	}
}
