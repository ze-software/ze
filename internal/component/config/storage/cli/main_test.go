// VALIDATES: the ze-data CLI dispatch and pure arg helpers — extractPathFlag
// parses --path/--path= and returns the remaining args, defaults the store to
// the resolved config dir when --path is absent, Run returns the right exit
// codes for empty/help/unknown subcommands, and filePathToKey maps a file path
// to its active-config storage key by basename.
// PREVENTS: a --path flag leaking into the subcommand args, an unknown subcommand
// being dispatched, a full path (not the basename) becoming the storage key, or
// `ze data` resolving a store that `ze init` never wrote.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/pkg/zefs"

	"github.com/ze-software/ze/internal/component/command/commandtest"
)

// setConfigDirEnv pins ze.config.dir for the duration of the test.
func setConfigDirEnv(t *testing.T, value string) {
	t.Helper()
	orig := env.Get("ze.config.dir")
	t.Cleanup(func() { _ = env.Set("ze.config.dir", orig) })
	if err := env.Set("ze.config.dir", value); err != nil {
		t.Fatalf("env.Set ze.config.dir: %v", err)
	}
}

func TestExtractPathFlag(t *testing.T) {
	path, rem := extractPathFlag([]string{"--path", "/var/db.zefs", "list", "meta"})
	if path != "/var/db.zefs" {
		t.Errorf("path = %q, want /var/db.zefs", path)
	}
	if len(rem) != 2 || rem[0] != "list" || rem[1] != "meta" {
		t.Errorf("remaining = %v, want [list meta]", rem)
	}

	path, rem = extractPathFlag([]string{"cat", "--path=/x/y.zefs", "key"})
	if path != "/x/y.zefs" {
		t.Errorf("path= form: path = %q, want /x/y.zefs", path)
	}
	if len(rem) != 2 || rem[0] != "cat" || rem[1] != "key" {
		t.Errorf("path= form: remaining = %v, want [cat key]", rem)
	}
}

// The data CLI and init must resolve the same configured store directory.
func TestExtractPathFlag_DefaultsToConfigDirEnv(t *testing.T) {
	pinned := t.TempDir()
	setConfigDirEnv(t, pinned)

	path, rem := extractPathFlag([]string{"check"})
	if want := filepath.Join(pinned, defaultStoreName); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	if len(rem) != 1 || rem[0] != "check" {
		t.Errorf("remaining = %v, want [check]", rem)
	}
}

// VALIDATES: an explicit --path still wins over ze.config.dir.
// PREVENTS: the env override hijacking a store the operator named explicitly.
func TestExtractPathFlag_ExplicitPathBeatsEnv(t *testing.T) {
	setConfigDirEnv(t, t.TempDir())

	explicit := filepath.Join(t.TempDir(), "explicit.zefs")
	path, _ := extractPathFlag([]string{"check", "--path", explicit})
	if path != explicit {
		t.Errorf("path = %q, want %q (--path must win over env)", path, explicit)
	}
}

func TestRunDispatchCodes(t *testing.T) {
	if rc := Run(nil); rc != 1 {
		t.Errorf("Run(nil) = %d, want 1 (usage)", rc)
	}
	if rc := Run([]string{"help"}); rc != 0 {
		t.Errorf("Run(help) = %d, want 0", rc)
	}
	if rc := Run([]string{"definitely-not-a-subcommand"}); rc != 1 {
		t.Errorf("Run(unknown) = %d, want 1", rc)
	}
}

func TestFilePathToKey(t *testing.T) {
	got := filePathToKey("/etc/ze/ze.conf")
	want := zefs.KeyFileActive.Key("ze.conf")
	if got != want {
		t.Errorf("filePathToKey = %q, want %q (basename-keyed)", got, want)
	}
}

// Raw namespaces and recursive listing must survive the CLI, on both encodings.
func TestDataRawKeyParity(t *testing.T) {
	for _, blob := range []bool{false, true} {
		name := "tree"
		if blob {
			name = "blob"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "database")
			var owner storage.Storage
			var err error
			if blob {
				path = filepath.Join(dir, "artifact.zefs")
				owner, err = storage.CreateBlob(path)
			} else {
				owner, err = storage.Create(dir)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(t.TempDir(), "input.conf")
			value := []byte("raw-value\n")
			if err := os.WriteFile(source, value, 0o600); err != nil {
				t.Fatal(err)
			}
			want := []string{"custom/nested/value", "file/active/input.conf", "meta/example"}
			for _, key := range []string{want[0], want[2]} {
				if code := cmdWrite(path, []string{key, source}); code != 0 {
					t.Fatalf("write %s returned %d", key, code)
				}
			}
			if code := cmdImport(path, []string{source}); code != 0 {
				t.Fatalf("import returned %d", code)
			}
			reader, err := openStore(path, false)
			if err != nil {
				t.Fatal(err)
			}
			keys, err := reader.ListKeys("")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(keys, want) {
				t.Fatalf("keys = %v, want %v", keys, want)
			}
			for _, key := range want {
				data, err := reader.ReadKey(key)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(data, value) {
					t.Fatalf("%s = %q", key, data)
				}
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			listAnswer := dataList(commandtest.Args("--path", path, "custom/"))
			answer, code := listAnswer.Payload, listAnswer.Code
			if code != 0 {
				t.Fatalf("list returned %d", code)
			}
			fields, ok := answer.(map[string]any)
			if !ok {
				t.Fatalf("list answer = %T", answer)
			}
			rows, ok := fields["keys"].([]map[string]any)
			if !ok {
				t.Fatalf("list keys = %T", fields["keys"])
			}
			if len(rows) != 1 {
				t.Fatalf("recursive list = %v", rows)
			}
			if rows[0]["key"] != want[0] {
				t.Fatalf("recursive list = %v", rows)
			}
			if code := cmdRm(path, []string{want[0]}); code != 0 {
				t.Fatalf("remove returned %d", code)
			}
			reader, err = openStore(path, false)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			if _, err := reader.ReadKey(want[0]); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("removed key read: %v", err)
			}
		})
	}
}

// Offline mutations must refuse a live owner while read-only inspection works.
func TestDataRefusesOwnedStoreMutation(t *testing.T) {
	dir := t.TempDir()
	owner, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Close() }()
	if err := owner.WriteKey("meta/held", []byte("keep")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "database")
	if code := cmdRm(path, []string{"meta/held"}); code != 2 {
		t.Fatalf("owned-store remove returned %d", code)
	}
	if code := dataList(commandtest.Args("--path", path)).Code; code != 0 {
		t.Fatalf("read-only inspection returned %d", code)
	}
	data, err := owner.ReadKey("meta/held")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep" {
		t.Fatalf("held key = %q", data)
	}
}

// ownedStore creates a live store in a temporary folder, writes keys into it,
// and keeps it open as its owner for the rest of the test, as a running daemon
// holds its store. It answers the owner and the path ze data is given.
func ownedStore(t *testing.T, keys ...string) (storage.Storage, string) {
	t.Helper()
	dir := t.TempDir()
	owner, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	for _, key := range keys {
		if err := owner.WriteKey(key, []byte("keep")); err != nil {
			t.Fatal(err)
		}
	}
	return owner, filepath.Join(dir, "database")
}

// VALIDATES: AC-3, local half. While another process owns the store, ze data
// rm sends each key to the daemon as `delete data key <key>`, one command per
// key, to the daemon of the store it was given, and prints the lines and the
// exit code the local route prints. A key the daemon refuses is an error line
// carrying the daemon's reason, and the next key is still sent. When every key
// is refused the exit code is 2. The stub removes through the owner's handle,
// as the daemon's handler does.
// PREVENTS: a busy store answered with the bare refusal while a daemon could
// remove the key, a daemon removal printed differently from a local one, or a
// refused key ending the walk.
func TestDataRmForwardsToDaemonWhenBusy(t *testing.T) {
	owner, path := ownedStore(t, "meta/held", "meta/other")
	var sent []string
	ask := func(folder, line string) ([]byte, error) {
		if folder != filepath.Dir(path) {
			t.Errorf("asked the daemon of %s, want the daemon of %s", folder, filepath.Dir(path))
		}
		sent = append(sent, line)
		key, found := strings.CutPrefix(line, "delete data key ")
		if !found {
			return nil, &daemonRefusal{message: "unexpected command " + line}
		}
		if err := owner.RemoveKey(key); err != nil {
			return nil, &daemonRefusal{message: "delete data key: the store holds no key " + key + "\nfix: show data list"}
		}
		return json.Marshal(map[string]string{"key": key})
	}

	var stdout, stderr bytes.Buffer
	code := removeKeys(path, []string{"meta/held", "meta/absent", "meta/other"}, ask, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("partial removal returned %d, want 0; stderr:\n%s", code, stderr.String())
	}
	wantSent := []string{"delete data key meta/held", "delete data key meta/absent", "delete data key meta/other"}
	if !reflect.DeepEqual(sent, wantSent) {
		t.Fatalf("sent %q, want %q", sent, wantSent)
	}
	wantOut := "removed meta/held\nremoved meta/other\n2 entries removed from " + path + "\n"
	if stdout.String() != wantOut {
		t.Fatalf("stdout = %q, want %q", stdout.String(), wantOut)
	}
	if !strings.Contains(stderr.String(), "error: delete data key: the store holds no key meta/absent") {
		t.Fatalf("stderr lacks the daemon's refusal of meta/absent:\n%s", stderr.String())
	}
	for _, key := range []string{"meta/held", "meta/other"} {
		if _, err := owner.ReadKey(key); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s after its removal: %v", key, err)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := removeKeys(path, []string{"meta/absent"}, ask, &stdout, &stderr); code != 2 {
		t.Fatalf("all keys refused returned %d, want 2", code)
	}
	if want := "0 entries removed from " + path + "\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

// VALIDATES: AC-3 survivor. A busy store whose daemon does not answer keeps
// the refusal: the store's own refusal, why the daemon did not answer with its
// address and the cause, the fix alone on the last line, exit 2, nothing on
// stdout, and the key kept. The walk ends at the first unanswered key.
// PREVENTS: a busy store with no daemon reported as a removal, or a refusal
// that does not say why the daemon route did not run.
func TestDataRmBusyWithNoDaemonNamesTheOwner(t *testing.T) {
	owner, path := ownedStore(t, "meta/held")
	asked := 0
	ask := func(_, _ string) ([]byte, error) {
		asked++
		return nil, &daemonUnanswered{
			why:   "no daemon listens at 127.0.0.1:2222",
			fix:   fixDaemonStatus,
			cause: errors.New("connection refused"),
		}
	}

	var stdout, stderr bytes.Buffer
	if code := removeKeys(path, []string{"meta/held", "meta/other"}, ask, &stdout, &stderr); code != 2 {
		t.Fatalf("unanswered removal returned %d, want 2", code)
	}
	if asked != 1 {
		t.Fatalf("asked the daemon %d times, want 1: no later key can reach a daemon that did not answer", asked)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want nothing", stdout.String())
	}
	text := stderr.String()
	for _, want := range []string{"error: ze data rm: ", storage.ErrBusy.Error(), "no daemon listens at 127.0.0.1:2222", "connection refused"} {
		if !strings.Contains(text, want) {
			t.Fatalf("refusal lacks %q:\n%s", want, text)
		}
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if last := lines[len(lines)-1]; last != "fix: "+fixDaemonStatus {
		t.Fatalf("last line = %q, want the fix alone", last)
	}
	data, err := owner.ReadKey("meta/held")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep" {
		t.Fatalf("held key = %q", data)
	}
}

// VALIDATES: an answer the daemon gives that is not JSON is read as no answer,
// never as success, with the command to run by hand as the fix.
// PREVENTS: a removal printed as done when nothing says it ran.
func TestSendToDaemonUnreadableAnswerIsNoAnswer(t *testing.T) {
	ask := func(_, _ string) ([]byte, error) { return []byte("removed, probably"), nil }
	var answer keyAnswer
	err := sendToDaemon(ask, "/var/lib/ze/database", "delete data key meta/x", &answer)
	unanswered, ok := errors.AsType[*daemonUnanswered](err)
	if !ok {
		t.Fatalf("err = %v, want a *daemonUnanswered", err)
	}
	if want := `ze cli --user <name> -c "delete data key meta/x"`; unanswered.fix != want {
		t.Fatalf("fix = %q, want %q", unanswered.fix, want)
	}
}

// TestDaemonRouteTextsHoldNoContent proves a line that carries a content never
// reaches a text the local route shows the operator.
//
// GOAL: when the daemon gives no usable answer to `update data key <k>
// content <v>`, the refusal printed, its why and its fix hold the placeholder
// and not the value.
// METHOD: answer the line with bytes that are not JSON, build the refusal the
// local command prints, and read every part of it; read daemonCommandFix alone.
// VALIDATES: AC-8 (a refusal that echoes an argument): sendToDaemon,
// daemonCommandFix, unansweredRefusal.
// PREVENTS: the base64 of a credential printed on the terminal and into any
// log of stderr.
func TestDaemonRouteTextsHoldNoContent(t *testing.T) {
	const content = "c2VjcmV0LWNyZWRlbnRpYWw="
	line := "update data key meta/test/state content " + content
	ask := func(_, _ string) ([]byte, error) { return []byte("written, probably"), nil }
	var answer keyAnswer
	err := sendToDaemon(ask, "/var/lib/ze/database", line, &answer)
	unanswered, ok := errors.AsType[*daemonUnanswered](err)
	if !ok {
		t.Fatalf("err = %v, want a *daemonUnanswered", err)
	}
	refusal := unansweredRefusal("ze data write", storage.ErrBusy, unanswered).Error()
	for name, text := range map[string]string{"why": unanswered.why, "fix": unanswered.fix, "refusal": refusal} {
		if strings.Contains(text, content) {
			t.Errorf("%s holds the content: %q", name, text)
		}
	}
	if !strings.Contains(refusal, "content "+redact.Placeholder) {
		t.Errorf("refusal = %q, want the placeholder after content", refusal)
	}
	want := `ze cli --user <name> -c "update data key meta/test/state content ` + redact.Placeholder + `"`
	if fix := daemonCommandFix(line); fix != want {
		t.Errorf("daemonCommandFix = %q, want %q", fix, want)
	}
}
