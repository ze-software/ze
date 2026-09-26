// Design: docs/architecture/storage-backends.md -- content-addressed history findings in ze doctor

package doctor

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/zefs"
)

// The doctor's store check raises the history findings `ze data check`
// prints: an entry whose object is gone is an error naming the entry, an
// object no entry names is a warning, and a healthy history raises nothing.
// Driven from checkStoreIntegrity, the entry the doctor registry calls.
func TestCheckStoreIntegrityReportsHistory(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	kept := time.Date(2026, 9, 26, 10, 0, 0, 0, time.Local)
	lost := kept.Add(time.Hour)
	if err := store.WriteVersion("router.conf", []byte("kept\n"), kept); err != nil {
		t.Fatal(err)
	}
	if diags := checkStoreIntegrity(dir); len(diags) != 0 {
		store.Close() //nolint:errcheck // test cleanup before failing
		t.Fatalf("healthy history raised %+v", diags)
	}
	if err := store.WriteVersion("router.conf", []byte("lost\n"), lost); err != nil {
		t.Fatal(err)
	}
	lostSum := sha256.Sum256([]byte("lost\n"))
	orphanSum := sha256.Sum256([]byte("orphan\n"))
	orphan := zefs.KeyObject.Key(hex.EncodeToString(orphanSum[:]))
	if err := store.RemoveKey(zefs.KeyObject.Key(hex.EncodeToString(lostSum[:]))); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteKey(orphan, []byte("orphan\n")); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	dangling := "file/" + storage.FormatVersionStamp(lost) + "/router.conf"
	want := map[string]diagnostic.Severity{
		storage.HistoryDanglingEntry + ": " + dangling: diagnostic.SeverityError,
		storage.HistoryOrphanObject + ": " + orphan:    diagnostic.SeverityWarning,
	}
	diags := checkStoreIntegrity(dir)
	for _, diag := range diags {
		if diag.Code != diagnostic.CodeDoctorStoreIntegrity {
			t.Fatalf("finding %q carries code %v", diag.Message, diag.Code)
		}
		for prefix, severity := range want {
			if strings.HasPrefix(diag.Message, prefix) && diag.Severity == severity {
				delete(want, prefix)
			}
		}
	}
	if len(want) != 0 {
		t.Fatalf("doctor findings %+v miss %v", diags, want)
	}
	if !strings.HasSuffix(diags[0].Path, filepath.FromSlash(dangling)) && !strings.HasSuffix(diags[1].Path, filepath.FromSlash(dangling)) {
		t.Fatalf("no finding points at %s: %+v", dangling, diags)
	}
}
