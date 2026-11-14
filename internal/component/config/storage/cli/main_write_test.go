package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/redact"
)

// writeTestFile writes data to a file in a temporary folder and answers its
// path, the file ze data write is given.
func writeTestFile(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "value.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// daemonWriteStub answers update data key lines as the daemon's handler does,
// writing the decoded content through the owner's handle, and records every
// line it was sent.
func daemonWriteStub(t *testing.T, owner storage.Storage, sent *[]string) daemonAsk {
	t.Helper()
	return func(_, line string) ([]byte, error) {
		*sent = append(*sent, line)
		rest, found := strings.CutPrefix(line, pathUpdateDataKey+" ")
		if !found {
			return nil, &daemonRefusal{message: "unexpected command"}
		}
		key, encoded, found := strings.Cut(rest, " "+keywordContent+" ")
		if !found {
			return nil, &daemonRefusal{message: "no content"}
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, &daemonRefusal{message: "update data key: content is not base64"}
		}
		if err := owner.WriteKey(key, data); err != nil {
			return nil, &daemonRefusal{message: err.Error()}
		}
		return json.Marshal(map[string]any{keywordKey: key, keywordBytes: len(data)})
	}
}

// VALIDATES: AC-7, local half. While another process owns the store, ze data
// write sends the file to the daemon as `update data key <key> content
// <base64>`, one command, and prints the line and the exit code the local route
// prints. The key is created, then replaced, holding exactly the file's bytes,
// binary bytes included. The stub writes through the owner's handle, as the
// daemon's handler does.
// PREVENTS: a busy store answered with the bare refusal while the daemon could
// write the key, a daemon write printed differently from a local one, or the
// file sent in a form the daemon decodes to other bytes.
func TestDataWriteForwardsToDaemonWhenBusy(t *testing.T) {
	owner, path := ownedStore(t)
	var sent []string
	ask := daemonWriteStub(t, owner, &sent)

	for _, value := range [][]byte{[]byte("first value\n"), {0x00, 0xff, 0x10, '\n', 0x80}} {
		sent = nil
		var stdout, stderr bytes.Buffer
		code := writeKey(path, []string{updateTestKey, writeTestFile(t, value)}, ask, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("busy write returned %d, want 0; stderr:\n%s", code, stderr.String())
		}
		want := pathUpdateDataKey + " " + updateTestKey + " " + keywordContent + " " + base64.StdEncoding.EncodeToString(value)
		if len(sent) != 1 || sent[0] != want {
			t.Fatalf("sent %q, want the one line %q", sent, want)
		}
		wantOut := "wrote " + updateTestKey + " (" + strconv.Itoa(len(value)) + " bytes) into " + path + "\n"
		if stdout.String() != wantOut {
			t.Fatalf("stdout = %q, want %q", stdout.String(), wantOut)
		}
		stored, err := owner.ReadKey(updateTestKey)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(stored, value) {
			t.Fatalf("stored %q, want %q", stored, value)
		}
	}
}

// VALIDATES: AC-12 and the Boundary table, local half. A busy ze data write of
// 195,840 bytes is sent; one byte more is refused before the daemon is asked,
// with the daemon's own bound refusal (the bound in bytes of content and the
// offline command for a larger value), exit 2, nothing on stdout, the key
// unchanged, and the content not in the refusal.
// PREVENTS: an oversize write that connects and fails as "packet too large",
// read as a daemon that did not answer.
func TestDataWriteRefusesOversizeBeforeConnecting(t *testing.T) {
	owner, path := ownedStore(t)
	var sent []string
	ask := daemonWriteStub(t, owner, &sent)

	var stdout, stderr bytes.Buffer
	atBound := bytes.Repeat([]byte{'z'}, updateContentBytesMax)
	if code := writeKey(path, []string{updateTestKey, writeTestFile(t, atBound)}, ask, &stdout, &stderr); code != 0 {
		t.Fatalf("a write at the bound returned %d, want 0; stderr:\n%s", code, stderr.String())
	}
	if len(sent) != 1 {
		t.Fatalf("a write at the bound asked the daemon %d times, want once", len(sent))
	}

	sent = nil
	stdout.Reset()
	stderr.Reset()
	over := bytes.Repeat([]byte{'y'}, updateContentBytesMax+1)
	if code := writeKey(path, []string{updateTestKey, writeTestFile(t, over)}, ask, &stdout, &stderr); code != 2 {
		t.Fatalf("a write over the bound returned %d, want 2", code)
	}
	if len(sent) != 0 {
		t.Fatalf("a write over the bound asked the daemon %d times, want none", len(sent))
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want nothing", stdout.String())
	}
	text := stderr.String()
	for _, want := range []string{"error: ze data write: ", "195,840 bytes", "ze data write <key> <file> while ze is stopped", redact.Placeholder} {
		if !strings.Contains(text, want) {
			t.Fatalf("refusal lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, base64.StdEncoding.EncodeToString(over[:48])) {
		t.Fatalf("refusal holds the content:\n%s", text)
	}
	stored, err := owner.ReadKey(updateTestKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, atBound) {
		t.Fatal("the refused write changed the key")
	}
}
