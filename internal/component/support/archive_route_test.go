package support

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// useStateStore gives the test a live store as the daemon's statestore and
// restores the previous one after.
func useStateStore(t *testing.T) {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	previous := statestore.Store()
	statestore.SetStore(store)
	t.Cleanup(func() {
		statestore.SetStore(previous)
		store.Close() //nolint:errcheck // test cleanup
	})
}

// archiveLeaves judges tokens against the four leaves of the command's YANG
// node, as the model does on every route.
func archiveLeaves(t *testing.T, tokens ...string) command.ValidatedArgs {
	t.Helper()
	defs := make([]command.ArgDef, 0, 4)
	for _, name := range []string{leafExclude, leafModule, leafReason, leafSince} {
		defs = append(defs, commandtest.Must(command.NewStringArg(name, nil, nil, command.ArgOptions{})))
	}
	validated, err := command.ValidateArgs(tokens, defs, nil)
	if err != nil {
		t.Fatalf("judge %q: %v", tokens, err)
	}
	return validated
}

// tarEntries reads every entry of a gzip-compressed tar.
func tarEntries(t *testing.T, archive []byte) map[string][]byte {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	entries := make(map[string][]byte)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return entries
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("tar entry %s: %v", hdr.Name, err)
		}
		entries[hdr.Name] = data
	}
}

// TestSupportArchiveDaemonRouteStoresUnderKey collects on the daemon route
// with the module and reason leaves.
//
// VALIDATES: AC-15. The answer is the manifest naming a key under the
// registered support/archive pattern and the archive's size; the store holds
// exactly that many bytes under that key, a tar.gz whose manifest names the
// same key and holds only the selected module.
// PREVENTS: the daemon writing the archive to its own working directory, a
// path the remote caller can never read.
func TestSupportArchiveDaemonRouteStoresUnderKey(t *testing.T) {
	useStateStore(t)

	answer := command.AnswerRemote(Answer, archiveLeaves(t, "module", "version", "reason", "remote case"))
	if answer.Code != 0 {
		t.Fatalf("daemon route refused: %q", answer.Diagnostic)
	}
	manifest, ok := answer.Payload.(*SupportManifest)
	if !ok {
		t.Fatalf("payload %T, want *SupportManifest", answer.Payload)
	}
	if !strings.HasPrefix(manifest.ArchiveKey, zefs.KeySupportArchive.Dir()) {
		t.Fatalf("archive key %q is not under %q", manifest.ArchiveKey, zefs.KeySupportArchive.Dir())
	}
	if !zefs.IsRegistered(manifest.ArchiveKey) {
		t.Fatalf("archive key %q matches no registered pattern", manifest.ArchiveKey)
	}
	if manifest.ArchivePath != "" {
		t.Errorf("archive-path %q on the daemon route, want none", manifest.ArchivePath)
	}
	if manifest.Reason != "remote case" {
		t.Errorf("reason %q, want the reason leaf", manifest.Reason)
	}

	stored, found, err := statestore.Read(manifest.ArchiveKey)
	if err != nil || !found {
		t.Fatalf("store read %q: found=%v err=%v", manifest.ArchiveKey, found, err)
	}
	if len(stored) != manifest.ArchiveOctets {
		t.Errorf("stored %d bytes, answer says %d", len(stored), manifest.ArchiveOctets)
	}
	entries := tarEntries(t, stored)
	if _, has := entries["version.json"]; !has {
		t.Errorf("archive lacks version.json: %v", entries)
	}
	if len(entries) != 2 {
		t.Errorf("archive holds %d entries, want manifest and the one selected module", len(entries))
	}
	var inner SupportManifest
	if err := json.Unmarshal(entries["manifest.json"], &inner); err != nil {
		t.Fatalf("archive manifest: %v", err)
	}
	if inner.ArchiveKey != manifest.ArchiveKey {
		t.Errorf("archive manifest names key %q, answer %q", inner.ArchiveKey, manifest.ArchiveKey)
	}
}

// TestSupportArchiveOutputRefusedOnDaemonRoute gives --output on the daemon
// route.
//
// VALIDATES: AC-15. --output is refused by name, the refusal names the local
// command, and nothing is stored.
// PREVENTS: the daemon writing into a directory the caller named on another
// host, which on the daemon's host is some other directory or none.
func TestSupportArchiveOutputRefusedOnDaemonRoute(t *testing.T) {
	useStateStore(t)

	answer := command.AnswerRemote(Answer, commandtest.Args("--output", t.TempDir()))
	if answer.Code == 0 || answer.Payload != nil {
		t.Fatalf("--output on the daemon route answered %+v, want a refusal", answer)
	}
	if !strings.Contains(answer.Diagnostic, "--output") {
		t.Errorf("refusal %q does not name --output", answer.Diagnostic)
	}
	if !strings.Contains(answer.Diagnostic, "'ze request support archive --output <dir>'") {
		t.Errorf("refusal %q does not name the local command", answer.Diagnostic)
	}
	keys, err := statestore.List(zefs.KeySupportArchive.Dir())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("refused call stored %v", keys)
	}
}

// TestSupportArchiveLeafAndFlagRefused gives the module leaf and --module
// together on the local route.
//
// VALIDATES: a flag making the choice its leaf already made is refused,
// naming both, rather than one silently winning.
func TestSupportArchiveLeafAndFlagRefused(t *testing.T) {
	answer := command.AnswerLocal(Answer, archiveLeaves(t, "module", "version", "--module", "host"))
	if answer.Code == 0 || answer.Payload != nil {
		t.Fatalf("leaf beside flag answered %+v, want a refusal", answer)
	}
	if !strings.Contains(answer.Diagnostic, "--module") || !strings.Contains(answer.Diagnostic, "module leaf") {
		t.Errorf("refusal %q does not name the flag and the leaf", answer.Diagnostic)
	}
}

// TestSupportArchiveStopsWithTheRequest collects for a request that already
// ended.
//
// VALIDATES: R-4. Collection honors the request context: no module runs,
// the answer refuses saying the request ended, and nothing is stored.
// PREVENTS: a collection that outlives its caller and holds a dispatcher slot.
func TestSupportArchiveStopsWithTheRequest(t *testing.T) {
	useStateStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ran := false
	modules := map[string]moduleCollector{"probe": func(_ *collectOptions) (any, error) {
		ran = true
		return map[string]any{}, nil
	}}
	answer := storeAnswer(ctx, modules, &collectOptions{}, "")
	if answer.Code == 0 {
		t.Fatalf("ended request answered %+v, want a refusal", answer)
	}
	if !strings.Contains(answer.Diagnostic, "the request ended") {
		t.Errorf("refusal %q does not say the request ended", answer.Diagnostic)
	}
	if ran {
		t.Error("a module ran after the request ended")
	}
	keys, err := statestore.List(zefs.KeySupportArchive.Dir())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("ended request stored %v", keys)
	}
}

// TestSupportArchiveOverBoundRefused collects an archive larger than one
// remote answer carries.
//
// VALIDATES: R-7. The archive is refused before it is stored, and the
// refusal names its size and the local command that writes it to a file.
func TestSupportArchiveOverBoundRefused(t *testing.T) {
	useStateStore(t)
	noise := make([]byte, command.BlobRemoteOctetsMax+command.BlobRemoteOctetsMax/2)
	if _, err := rand.Read(noise); err != nil {
		t.Fatalf("random: %v", err)
	}
	modules := map[string]moduleCollector{"noise": func(_ *collectOptions) (any, error) {
		return base64.StdEncoding.EncodeToString(noise), nil
	}}
	answer := storeAnswer(context.Background(), modules, &collectOptions{}, "")
	if answer.Code == 0 {
		t.Fatalf("oversized archive answered %+v, want a refusal", answer)
	}
	if !strings.Contains(answer.Diagnostic, strconv.Itoa(command.BlobRemoteOctetsMax)) {
		t.Errorf("refusal %q does not name the bound", answer.Diagnostic)
	}
	if !strings.Contains(answer.Diagnostic, "'ze request support archive --output <dir>'") {
		t.Errorf("refusal %q does not name the local command", answer.Diagnostic)
	}
	keys, err := statestore.List(zefs.KeySupportArchive.Dir())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("oversized archive was stored: %v", keys)
	}
}

// TestRealArchiveFitsTheRemoteBound collects every module on this host into
// memory, as the daemon route does.
//
// VALIDATES: R-7, measured. A real archive fits under
// command.BlobRemoteOctetsMax with room; the size is logged so the bound's
// comment can be checked against a run.
//
// ze.config.dir points at an empty directory: the developer's own live store
// may be held by a running daemon, and a test never waits on it.
func TestRealArchiveFitsTheRemoteBound(t *testing.T) {
	t.Setenv("ze.config.dir", t.TempDir())
	_, archive, err := collectToMemory(context.Background(), moduleRegistry, &collectOptions{}, "")
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	t.Logf("a real archive of every module on this host: %d bytes (bound %d)", len(archive), command.BlobRemoteOctetsMax)
	if len(archive)*4 > command.BlobRemoteOctetsMax {
		t.Errorf("a real archive is %d bytes, over a quarter of the %d bound", len(archive), command.BlobRemoteOctetsMax)
	}
}
