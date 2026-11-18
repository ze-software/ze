// Design: docs/architecture/api/commands.md -- explicit draft and history source selection.

package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestPendingValidationReadsTheSelectedDraft keeps a valid committed source
// beside an invalid draft, proving both invocation forms judge the draft.
func TestPendingValidationReadsTheSelectedDraft(t *testing.T) {
	file := confinedConfigDir(t, showTestConfig)
	if err := os.WriteFile(file+".draft", []byte("invalid draft syntax"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := cmdValidate([]string{"--pending", "-q", file}); code != exitInvalid {
		t.Fatalf("local draft control returned %d", code)
	}
	answer := answerOn(t, "show config validation", []string{"file", file, "pending"}, true)
	if answer.Code != exitInvalid {
		t.Fatalf("daemon did not validate the draft: %+v", answer)
	}
}

// TestPendingValidationAcceptsAValidDraft proves pending validation can succeed
// even when the neighboring committed source is not valid.
func TestPendingValidationAcceptsAValidDraft(t *testing.T) {
	file := confinedConfigDir(t, "invalid committed syntax")
	if err := os.WriteFile(file+".draft", []byte(validConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := cmdValidate([]string{"--pending", "-q", file}); code != exitOK {
		t.Fatalf("valid local draft returned %d", code)
	}
	answer := answerOn(t, "show config validation", []string{"file", file, "pending"}, true)
	if answer.Code != exitOK || answer.Payload == nil {
		t.Fatalf("valid daemon draft refused: %+v", answer)
	}
}

// TestPendingValidationConfinesTheDraftItReads checks the selected draft,
// rather than merely approving its neighboring committed filename.
func TestPendingValidationConfinesTheDraftItReads(t *testing.T) {
	file := confinedConfigDir(t, showTestConfig)
	outside := writeTestConfig(t, showTestConfig)
	if err := os.Symlink(outside, file+".draft"); err != nil {
		t.Fatal(err)
	}
	answer := answerOn(t, "show config validation", []string{"file", file, "pending"}, true)
	if answer.Code == exitOK {
		t.Fatal("a draft symlink escaped the daemon's configuration directories")
	}
	if answer.Payload != nil {
		t.Fatal("draft confinement refusal retained source content")
	}
}

// TestHistoryFileSelectorMatchesPositionalSource checks the modeled file
// keyword reaches the same typed census as its existing positional spelling.
func TestHistoryFileSelectorMatchesPositionalSource(t *testing.T) {
	file := confinedConfigDir(t, showTestConfig)
	store, err := storage.Create(filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteVersion(file, []byte(showTestConfig), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	positional := answerOn(t, "show config history", []string{file}, true)
	if positional.Code != exitOK {
		t.Fatalf("history control failed: %+v", positional)
	}
	keyed := answerOn(t, "show config history", []string{"file", file}, true)
	if encodedAnswer(t, keyed) != encodedAnswer(t, positional) {
		t.Fatalf("modeled selector changed history: %+v; want %+v", keyed, positional)
	}
}
