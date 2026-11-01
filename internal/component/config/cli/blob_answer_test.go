package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// blobTestConfig is a valid config written out of fmt's indentation, so fmt
// has something to change. The firewall needs no feature tag.
const blobTestConfig = "firewall {\ntable wan {\nfamily inet\n}\n}\n"

// captureBlobStdio runs fn with stdout and stderr redirected to pipes and
// answers its exit code and what it wrote to each. Not safe for parallel tests.
func captureBlobStdio(t *testing.T, fn func() int) (code int, stdout, stderr string) {
	t.Helper()
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	savedOut, savedErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outWrite, errWrite
	outDone := make(chan string, 1)
	errDone := make(chan string, 1)
	go drainPipe(outRead, outDone)
	go drainPipe(errRead, errDone)
	code = fn()
	os.Stdout, os.Stderr = savedOut, savedErr
	if closeErr := outWrite.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if closeErr := errWrite.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	return code, <-outDone, <-errDone
}

// drainPipe reads one pipe to its end, so a large write cannot block the
// function under test, and sends what it read.
func drainPipe(r *os.File, done chan<- string) {
	data, _ := io.ReadAll(r) //nolint:errcheck // a short read shows as a mismatch in the test
	r.Close()                //nolint:errcheck // test pipe
	done <- string(data)
}

// answerSilently calls a handler with stdout and stderr captured and fails the
// test if it wrote anything: everything it has to say comes back in its
// answer (command.LocalDataHandler).
func answerSilently(t *testing.T, name string, handler func() command.LocalAnswer) command.LocalAnswer {
	t.Helper()
	var answer command.LocalAnswer
	_, stdout, stderr := captureBlobStdio(t, func() int {
		answer = handler()
		return 0
	})
	if stdout != "" || stderr != "" {
		t.Errorf("%s wrote stdout %q stderr %q; a handler writes nothing", name, stdout, stderr)
	}
	return answer
}

func writeBlobTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blob.conf")
	if err := os.WriteFile(path, []byte(blobTestConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestShowConfigFmtAnswersTheFormattedConfig proves `show config fmt` answers
// its output as a value and the local route prints exactly what the printing
// command printed before the conversion: the formatted text, unchanged.
//
// VALIDATES: the handler writes nothing; its payload is a Blob of
// configFmtBytes' output; `ze config fmt` prints those bytes and nothing on
// stderr; --check and --diff report on stderr with their old exit codes.
// PREVENTS: the answer going to the daemon's stdout when the daemon serves
// the command, and the local output changing shape.
func TestShowConfigFmtAnswersTheFormattedConfig(t *testing.T) {
	path := writeBlobTestConfig(t)
	want, changed, err := configFmtBytes(path, []byte(blobTestConfig))
	if err != nil || !changed {
		t.Fatalf("configFmtBytes = changed %v, err %v; the test config must need formatting", changed, err)
	}

	answer := answerSilently(t, "fmtAnswer", func() command.LocalAnswer { return fmtAnswer([]string{path}) })
	blob, isBlob := answer.Payload.(command.Blob)
	if !isBlob || string(blob) != want || answer.Code != exitOK || answer.Diagnostic != "" {
		t.Errorf("fmtAnswer = %+v; want Blob of the formatted config, code 0, no diagnostic", answer)
	}

	code, stdout, stderr := captureBlobStdio(t, func() int { return cmdFmt([]string{path}) })
	if code != exitOK || stdout != want || stderr != "" {
		t.Errorf("ze config fmt: code %d stdout %q stderr %q; want 0, the formatted config, nothing", code, stdout, stderr)
	}

	code, stdout, stderr = captureBlobStdio(t, func() int { return cmdFmt([]string{"--check", path}) })
	if code != 1 || stdout != "" || stderr != path+" needs formatting\n" {
		t.Errorf("--check: code %d stdout %q stderr %q; want 1 and the verdict on stderr", code, stdout, stderr)
	}

	code, stdout, stderr = captureBlobStdio(t, func() int { return cmdFmt([]string{"--diff", path}) })
	if code != exitOK || stdout != "" || !strings.HasPrefix(stderr, "--- "+path+" (original)\n+++ "+path+" (formatted)\n@@ -1,") {
		t.Errorf("--diff: code %d stdout %q stderr %q; want 0 and the hunks on stderr", code, stdout, stderr)
	}
}

// TestShowConfigFmtRefusalsAreReturned proves every refusal of `show config
// fmt` comes back in the answer, with the bytes and exit code the flag package
// and the printing command produced before.
func TestShowConfigFmtRefusalsAreReturned(t *testing.T) {
	const note = "\nFormats current config files only. For old configs, run \"ze config migrate\" first.\n"
	cases := []struct {
		name   string
		args   []string
		code   int
		prefix string
	}{
		{"no file", nil, exitError, "error: missing config file (use - for stdin)\n"},
		{"help", []string{"-h"}, exitOK, ""},
		{"unknown flag", []string{"-nope", "x"}, exitError, "flag provided but not defined: -nope\n"},
		{"missing file", []string{filepath.Join(t.TempDir(), "absent.conf")}, exitError, "error: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answer := answerSilently(t, "fmtAnswer", func() command.LocalAnswer { return fmtAnswer(tc.args) })
			if answer.Payload != nil || answer.Code != tc.code || !strings.HasPrefix(answer.Diagnostic, tc.prefix) {
				t.Errorf("answer = %+v; want no payload, code %d, diagnostic starting %q", answer, tc.code, tc.prefix)
			}
			if tc.name == "missing file" {
				return
			}
			if !strings.Contains(answer.Diagnostic, "ze config fmt [options] <config-file>") ||
				!strings.HasSuffix(answer.Diagnostic, note) {
				t.Errorf("diagnostic %q: want the help page and its note", answer.Diagnostic)
			}
		})
	}
}

// TestShowConfigGraphAnswersIndentedJSON proves `show config graph` answers
// the graph's indented JSON text as a value, and the local route prints
// exactly what json.Encoder with a two-space indent printed before.
func TestShowConfigGraphAnswersIndentedJSON(t *testing.T) {
	path := writeBlobTestConfig(t)
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := config.NewParser(schema).Parse(blobTestConfig)
	if err != nil {
		t.Fatal(err)
	}
	config.PruneInactive(tree, schema)
	indented, err := json.MarshalIndent(config.BuildGraph(tree, schema), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want := string(indented) + "\n"

	answer := answerSilently(t, "graphAnswer", func() command.LocalAnswer { return graphAnswer([]string{path}) })
	blob, isBlob := answer.Payload.(command.Blob)
	if !isBlob || string(blob) != want || answer.Code != exitOK || answer.Diagnostic != "" {
		t.Errorf("graphAnswer = %+v; want Blob %q", answer, want)
	}

	code, stdout, stderr := captureBlobStdio(t, func() int { return cmdGraph([]string{path}) })
	if code != exitOK || stdout != want || stderr != "" {
		t.Errorf("ze config graph: code %d stdout %q stderr %q; want 0 and the JSON", code, stdout, stderr)
	}

	refusal := answerSilently(t, "graphAnswer", func() command.LocalAnswer { return graphAnswer(nil) })
	if refusal.Payload != nil || refusal.Code != exitError ||
		!strings.HasPrefix(refusal.Diagnostic, "error: missing config file (use - for stdin)\n") ||
		!strings.Contains(refusal.Diagnostic, "ze config graph <config-file>") {
		t.Errorf("graph with no file = %+v; want the refusal and the help page", refusal)
	}
}

// TestShowConfigCatAnswersTheStoredBytes proves `show config cat` answers the
// stored file's bytes as a value, binary included, and the local route prints
// them unchanged, as os.Stdout.Write(data) did.
func TestShowConfigCatAnswersTheStoredBytes(t *testing.T) {
	store := storage.CreateMemory()
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test store
	stored := []byte("bgp {\n}\n\xff\x00")
	if err := store.WriteFile("blob.conf", stored, 0o600); err != nil {
		t.Fatal(err)
	}

	answer := answerSilently(t, "catAnswer", func() command.LocalAnswer {
		return catAnswer(store, []string{"blob.conf"})
	})
	blob, isBlob := answer.Payload.(command.Blob)
	if !isBlob || string(blob) != string(stored) || answer.Code != exitOK {
		t.Errorf("catAnswer = %+v; want Blob of the stored bytes", answer)
	}

	code, stdout, stderr := captureBlobStdio(t, func() int { return cmdCatWithStorage(store, []string{"blob.conf"}) })
	if code != exitOK || stdout != string(stored) || stderr != "" {
		t.Errorf("ze config cat: code %d stdout %q stderr %q; want the stored bytes", code, stdout, stderr)
	}

	usage := answerSilently(t, "catAnswer", func() command.LocalAnswer { return catAnswer(store, nil) })
	if usage.Payload != nil || usage.Code != 1 || usage.Diagnostic != "usage: ze config cat <key>\n" {
		t.Errorf("cat with no key = %+v; want the usage line and code 1", usage)
	}
	missing := answerSilently(t, "catAnswer", func() command.LocalAnswer {
		return catAnswer(store, []string{"absent.conf"})
	})
	if missing.Payload != nil || missing.Code != exitError || !strings.HasPrefix(missing.Diagnostic, "error: ") {
		t.Errorf("cat of a missing key = %+v; want an error line and code %d", missing, exitError)
	}
}
