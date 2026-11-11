// Design: docs/architecture/appliance/iso-installer.md -- truthful write completion.

package disk

import (
	"bytes"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
)

func TestISOWriteFinalizationSyncsAndRereadsBeforeCompletion(t *testing.T) {
	var calls []string
	var output bytes.Buffer
	oldSync, oldReread, oldLogger := syncFS, blkRereadPart, slog.Default()
	syncFS = func() { calls = append(calls, "sync") }
	blkRereadPart = func(disk string) error {
		calls = append(calls, "reread "+disk)
		if output.Len() != 0 {
			t.Error("completion was logged before partition reread")
		}
		return nil
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() {
		syncFS, blkRereadPart = oldSync, oldReread
		slog.SetDefault(oldLogger)
	})
	if err := finishISOWrite("/dev/vda"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"sync", "reread /dev/vda"}) {
		t.Errorf("write finalization=%v", calls)
	}
	if !strings.Contains(output.String(), "image written, partition table re-read") {
		t.Errorf("successful ISO write did not report actual completion: %s", output.String())
	}
}

func TestISOWriteFinalizationRefusesRereadFailure(t *testing.T) {
	var output bytes.Buffer
	oldSync, oldReread, oldLogger := syncFS, blkRereadPart, slog.Default()
	syncFS = func() {}
	want := errors.New("partition reread failed")
	blkRereadPart = func(string) error { return want }
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() {
		syncFS, blkRereadPart = oldSync, oldReread
		slog.SetDefault(oldLogger)
	})
	if err := finishISOWrite("/dev/vda"); !errors.Is(err, want) {
		t.Errorf("partition reread failure=%v", err)
	}
	if strings.Contains(output.String(), "image written, partition table re-read") {
		t.Errorf("failed ISO finalization reported completion: %s", output.String())
	}
}
