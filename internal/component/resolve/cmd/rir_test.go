// Design: docs/architecture/resolve.md -- wiring tests for the two RIR commands
package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	// The editor parses against the REGISTERED YANG modules, and a module no
	// binary imported leaves every block it declares unknown, which the
	// lenient parse then drops in silence. The daemon registers this one
	// through the composition root; this test binary registers it here.
	_ "github.com/ze-software/ze/internal/component/config/system/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// fixtureDelegation is one registry's delegation file: a header line and two
// adjacent ASN records, which collapse into the single range AS1 to AS3.
const fixtureDelegation = "2|ripencc|20260826|2|0|20260826|+0000\n" +
	"ripencc|EU|asn|1|1|19930901|allocated\n" +
	"ripencc|EU|asn|2|2|19930901|assigned\n"

// useDelegationFetch points the refresh at a test's own registries for the
// duration of that test. Without it the handler reaches the five public
// delegation files, so every test of the refresh MUST call it.
func useDelegationFetch(t *testing.T, fetch irr.DelegationFetch) {
	t.Helper()

	previous := delegationFetch
	delegationFetch = fetch
	t.Cleanup(func() { delegationFetch = previous })
}

// serveDelegation answers body for every registry, so a whole refresh
// completes.
func serveDelegation(body string) irr.DelegationFetch {
	return func(_ context.Context, _ string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(body)), nil
	}
}

// handlerFor finds the handler the command tree reaches for a wire method. It
// asks the registry rather than naming the Go function, so a handler that was
// written but never registered fails here instead of passing.
func handlerFor(t *testing.T, wireMethod string) pluginserver.Handler {
	t.Helper()
	for _, reg := range pluginserver.AllBuiltinRPCs() {
		if reg.WireMethod == wireMethod {
			return reg.Handler
		}
	}
	t.Fatalf("no handler registered for %s", wireMethod)
	return nil
}

// registerTempStore materializes an empty database.zefs and registers it as
// the process-wide state store, so a write round-trips through the real shared
// handle rather than a loose file. statestore never creates the store, so the
// test creates it first and resets to filesystem-fallback on cleanup.
func registerTempStore(t *testing.T) {
	t.Helper()
	bs, err := zefs.Create(filepath.Join(t.TempDir(), "database.zefs"))
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
	}
	statestore.SetStore(bs)
	t.Cleanup(func() {
		statestore.SetStore(nil)
		bs.Close() //nolint:errcheck // best-effort cleanup of a temp-dir store
	})
}

// TestShowResolveRIRReachesTheTable proves `show resolve rir <asn>` reaches
// the delegation table. Goal: the registered wire method, the handler, and the
// table are one path. Method: fetch the handler from the RPC registry by its
// wire method and require the payload to name the registry that holds AS15169,
// which is ARIN.
func TestShowResolveRIRReachesTheTable(t *testing.T) {
	resp, err := handlerFor(t, "ze-show:resolve-rir")(nil, []string{"15169"})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status %q, error %q", resp.Status, resp.Error)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("payload is %T, not a plugin.Map the pipe operators can render", resp.Data)
	}
	if data["registry"] != "ARIN" {
		t.Errorf("registry is %v, want ARIN", data["registry"])
	}
	if data["whois"] != "whois.arin.net" {
		t.Errorf("whois is %v, want whois.arin.net", data["whois"])
	}
}

// TestShowResolveRIRSeparatesNoRangeFromNoTable proves the daemon keeps the
// two failures apart (AC-2). Goal: "no registry holds this AS number" MUST NOT
// be answerable by a table that could not be read. Method: ask about AS0,
// which is in no delegated range, and require the message to name the range
// rather than the table.
func TestShowResolveRIRSeparatesNoRangeFromNoTable(t *testing.T) {
	resp, err := handlerFor(t, "ze-show:resolve-rir")(nil, []string{"0"})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status %q, want %q", resp.Status, plugin.StatusError)
	}
	if !strings.Contains(resp.Error, "no delegated range") {
		t.Errorf("error does not report an undelegated AS number: %q", resp.Error)
	}
	if strings.Contains(resp.Error, "unreadable") {
		t.Errorf("an undelegated AS number is reported as an unreadable table: %q", resp.Error)
	}
}

// TestUpdateResolveRIRWritesTheStoredCopy proves `update resolve rir` reaches
// the managed store (AC-4). Goal: the operator's refresh order ends in a blob
// under the registered meta/rir/delegation key, and the answer says how many
// ranges it stored and when the data was collected. Method: register a temp
// state store, serve the five registries from a fixture, run the handler the
// registry holds for the wire method, and read the key back.
func TestUpdateResolveRIRWritesTheStoredCopy(t *testing.T) {
	registerTempStore(t)
	useDelegationFetch(t, serveDelegation(fixtureDelegation))

	resp, err := handlerFor(t, "ze-update:resolve-rir")(nil, nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status %q, error %q", resp.Status, resp.Error)
	}

	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("payload is %T, not a plugin.Map the pipe operators can render", resp.Data)
	}
	if data["ranges"] != 1 {
		t.Errorf("the refresh reports %v ranges, and the fixture collapses to one", data["ranges"])
	}
	if want := time.Now().UTC().Format(time.DateOnly); data["generated"] != want {
		t.Errorf("the refresh reports %v as the generation date, want %q", data["generated"], want)
	}

	stored, ok := statestore.Get(zefs.KeyRIRDelegation.Pattern)
	if !ok {
		t.Fatalf("the refresh reported success and stored nothing under %s", zefs.KeyRIRDelegation.Pattern)
	}
	if !strings.Contains(string(stored), "1 3 ripencc\n") {
		t.Errorf("the stored delegation table does not carry the collapsed range:\n%s", stored)
	}
}

// TestRefreshReportsAnUnstoredWrite proves a refresh that stored nothing never
// reports success (AC-10, R-6). Goal: statestore.Put answers (false, nil) in
// filesystem-fallback mode, and that MUST surface as an error rather than as a
// done status. Method: leave no store registered, serve every registry so
// nothing else can fail, and require the handler to report the failure.
func TestRefreshReportsAnUnstoredWrite(t *testing.T) {
	statestore.SetStore(nil)
	useDelegationFetch(t, serveDelegation(fixtureDelegation))

	resp, err := handlerFor(t, "ze-update:resolve-rir")(nil, nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status %q, want %q: an unstored refresh reported success", resp.Status, plugin.StatusError)
	}
	if !strings.Contains(resp.Error, "not stored") {
		t.Errorf("the error does not say the table was not stored: %q", resp.Error)
	}
}

// TestRefreshStoresNothingWhenAFetchFails proves the refresh is all or nothing
// (AC-5). Goal: one registry that does not answer MUST leave the previously
// stored copy exactly as it was, and the error MUST name the file that failed,
// because an operator can act only if they know which registry it was. Method:
// store a copy, serve four registries and fail the fifth, then read the key
// back.
func TestRefreshStoresNothingWhenAFetchFails(t *testing.T) {
	registerTempStore(t)

	previous := []byte("# Generated: 2026-01-01\n1 10 arin\n")
	if _, err := statestore.Put(zefs.KeyRIRDelegation.Pattern, previous); err != nil {
		t.Fatalf("store the previous copy: %v", err)
	}

	// AFRINIC is the silent registry, recognized by the token in the URL the
	// run asks for. The URL itself belongs to the irr package, so the fetch
	// records the one it refused and the assertions below compare against that.
	var silent string
	useDelegationFetch(t, func(_ context.Context, delegationURL string) (io.ReadCloser, error) {
		if strings.Contains(delegationURL, "/afrinic/") {
			silent = delegationURL
			return nil, errors.New("connection refused")
		}
		return io.NopCloser(strings.NewReader(fixtureDelegation)), nil
	})

	resp, err := handlerFor(t, "ze-update:resolve-rir")(nil, nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if silent == "" {
		t.Fatal("the refresh never asked for the AFRINIC delegation file")
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status %q, want %q: a refresh missing one registry reported success", resp.Status, plugin.StatusError)
	}
	if !strings.Contains(resp.Error, silent) {
		t.Errorf("the error does not name the registry file that failed: %q", resp.Error)
	}

	stored, ok := statestore.Get(zefs.KeyRIRDelegation.Pattern)
	if !ok {
		t.Fatal("the failed refresh removed the previously stored copy")
	}
	if !bytes.Equal(stored, previous) {
		t.Errorf("the failed refresh rewrote the stored copy:\n%s", stored)
	}
}

// TestRefreshStoresNothingWhenNoRecordIsParsed proves five answers holding no
// ASN record store nothing (AC-6). Goal: five HTTP 200 responses that carry no
// delegation MUST NOT replace the table, because an empty table answers that
// nobody holds any AS number. Method: serve a header line and nothing else,
// then require the error to say no record was read and the key to be absent.
func TestRefreshStoresNothingWhenNoRecordIsParsed(t *testing.T) {
	registerTempStore(t)
	useDelegationFetch(t, serveDelegation("2|ripencc|20260826|0|0|20260826|+0000\n"))

	resp, err := handlerFor(t, "ze-update:resolve-rir")(nil, nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status %q, want %q: a refresh that read no record reported success", resp.Status, plugin.StatusError)
	}
	if !strings.Contains(resp.Error, "no ASN record") {
		t.Errorf("the error does not say the run parsed no record: %q", resp.Error)
	}
	if _, ok := statestore.Get(zefs.KeyRIRDelegation.Pattern); ok {
		t.Error("a refresh that read no record stored a table")
	}
}

// writeDelegationConfig writes a config file the refresh reads back at command
// time. It is a whole config rather than a fragment, because the editor parses
// it against the YANG schema, so a block the schema refuses fails here.
func writeDelegationConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config %s: %v", path, err)
	}
}

// TestRefreshReadsTheSourcesCommittedAfterStartup proves a delegation source
// committed after the daemon started reaches the next refresh, with no restart
// (AC-8, A-1). Goal: SystemConfig is built once at startup and is never
// re-applied to the resolvers, so the refresh MUST read the config tree when
// it RUNS rather than take a value injected at startup. Method: start a plugin
// server on a config that names no source, commit one into that file, run the
// refresh, and require the fetch to ask the committed URL for that registry's
// delegation file.
func TestRefreshReadsTheSourcesCommittedAfterStartup(t *testing.T) {
	const mirror = "http://127.0.0.1:8080/delegated-ripencc-extended-latest"

	registerTempStore(t)

	configPath := filepath.Join(t.TempDir(), "ze.conf")
	writeDelegationConfig(t, configPath, "system {\n\thost router1;\n}\n")

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{ConfigPath: configPath}, nil)
	if err != nil {
		t.Fatalf("plugin server: %v", err)
	}

	// The commit lands AFTER the server started, which is the case A-1 names.
	writeDelegationConfig(t, configPath, "system {\n\thost router1;\n\trir {\n\t\tdelegation-source ripencc {\n\t\t\turl \""+mirror+"\";\n\t\t}\n\t}\n}\n")

	var asked []string
	useDelegationFetch(t, func(_ context.Context, delegationURL string) (io.ReadCloser, error) {
		asked = append(asked, delegationURL)
		return io.NopCloser(strings.NewReader(fixtureDelegation)), nil
	})

	resp, err := handlerFor(t, "ze-update:resolve-rir")(&pluginserver.CommandContext{Server: srv}, nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status %q, error %q", resp.Status, resp.Error)
	}
	if !slices.Contains(asked, mirror) {
		t.Errorf("the refresh read %v, and none of them is the committed source %q", asked, mirror)
	}
	for _, delegationURL := range asked {
		if strings.Contains(delegationURL, "ftp.ripe.net") {
			t.Errorf("the refresh read RIPE's published file %q while a source was committed for it", delegationURL)
		}
	}
}

// TestRefreshStopsWhenTheConfigCannotBeRead proves the refresh tells an
// unreadable config from an unconfigured one (ai/rules/principles.md). Goal:
// an operator who committed a mirror MUST NOT be served the five published
// files because the tree could not be read, so the run stops and names the
// file it could not open. Method: point the server at a config path that does
// not exist, serve every registry so nothing else can fail, and require the
// refresh to fetch nothing and report the config.
// TestRefreshNamesAConfiguredSourceItCannotRead proves a mirror that does not
// answer stops the run, names the file, and leaves the stored table whole
// (AC-6). Goal: an operator whose mirror is down keeps the answer they had,
// and is told which URL failed rather than that "a refresh failed".
//
// VALIDATES: the refresh reports the configured URL it could not read, stores
// nothing, and the previously stored table is byte for byte as it was.
// PREVENTS: a half-written table, and an error an operator cannot act on
// because it does not say which of the five files was unreachable.
func TestRefreshNamesAConfiguredSourceItCannotRead(t *testing.T) {
	const mirror = "http://127.0.0.1:9/delegated-arin-extended-latest"

	registerTempStore(t)

	configPath := filepath.Join(t.TempDir(), "ze.conf")
	writeDelegationConfig(t, configPath, "system {\n\thost router1;\n\trir {\n\t\tdelegation-source arin {\n\t\t\turl \""+mirror+"\";\n\t\t}\n\t}\n}\n")

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{ConfigPath: configPath}, nil)
	if err != nil {
		t.Fatalf("plugin server: %v", err)
	}

	// A table is stored first, so the assertion is about what a failed refresh
	// LEAVES rather than about an empty store.
	useDelegationFetch(t, serveDelegation(fixtureDelegation))
	if resp, refreshErr := handlerFor(t, "ze-update:resolve-rir")(&pluginserver.CommandContext{Server: srv}, nil); refreshErr != nil || resp.Status != plugin.StatusDone {
		t.Fatalf("the first refresh did not store: %v, %q", refreshErr, resp.Error)
	}
	stored, held := statestore.Get(zefs.KeyRIRDelegation.Pattern)
	if !held {
		t.Fatal("the first refresh stored nothing")
	}

	useDelegationFetch(t, func(_ context.Context, delegationURL string) (io.ReadCloser, error) {
		if delegationURL == mirror {
			return nil, errors.New("connection refused")
		}
		return io.NopCloser(strings.NewReader(fixtureDelegation)), nil
	})

	resp, err := handlerFor(t, "ze-update:resolve-rir")(&pluginserver.CommandContext{Server: srv}, nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status %q, want an error for a mirror that does not answer", resp.Status)
	}
	if !strings.Contains(resp.Error, mirror) {
		t.Errorf("error %q does not name the file it could not read", resp.Error)
	}

	after, stillHeld := statestore.Get(zefs.KeyRIRDelegation.Pattern)
	if !stillHeld {
		t.Fatal("the failed refresh removed the stored table")
	}
	if !bytes.Equal(stored, after) {
		t.Error("the failed refresh changed the stored table")
	}
}

// TestRefreshRefusesASourceTheFetchRuleRefuses proves the refresh applies the
// fetch rule to every configured source before it reads a byte (AC-4). Goal:
// the leaf's ze:validate refuses such a URL at commit time, and this proves
// the SECOND guard, the one that meets a config which reached the tree another
// way -- a file written by an older binary, or a backup taken before the rule
// existed. Method: write a source on plain HTTP off the box, run the refresh,
// and require it to name the URL and fetch nothing.
//
// VALIDATES: handleRIRRefresh refuses the source itself, rather than trusting
// that the editor already did.
// PREVENTS: a delegation table read over an unauthenticated transport from a
// host the operator does not own, which decides which registry Ze names as
// holding an AS number.
func TestRefreshRefusesASourceTheFetchRuleRefuses(t *testing.T) {
	const mirror = "http://mirror.example.com/delegated-lacnic-extended-latest"

	registerTempStore(t)

	configPath := filepath.Join(t.TempDir(), "ze.conf")
	writeDelegationConfig(t, configPath,
		"system {\n\thost router1;\n\trir {\n\t\tdelegation-source lacnic {\n\t\t\turl \""+mirror+"\";\n\t\t}\n\t}\n}\n")

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{ConfigPath: configPath}, nil)
	if err != nil {
		t.Fatalf("plugin server: %v", err)
	}

	fetched := false
	useDelegationFetch(t, func(_ context.Context, _ string) (io.ReadCloser, error) {
		fetched = true
		return io.NopCloser(strings.NewReader(fixtureDelegation)), nil
	})

	resp, err := handlerFor(t, "ze-update:resolve-rir")(&pluginserver.CommandContext{Server: srv}, nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status %q, want %q: a plain-http source off the box was read", resp.Status, plugin.StatusError)
	}
	if !strings.Contains(resp.Error, mirror) {
		t.Errorf("the error does not name the source it refused: %q", resp.Error)
	}
	if !strings.Contains(resp.Error, "lacnic") {
		t.Errorf("the error does not name the registry the source belongs to: %q", resp.Error)
	}
	if fetched {
		t.Error("the refresh read a delegation file after refusing one of its sources")
	}
}

func TestRefreshStopsWhenTheConfigCannotBeRead(t *testing.T) {
	registerTempStore(t)

	configPath := filepath.Join(t.TempDir(), "absent.conf")
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{ConfigPath: configPath}, nil)
	if err != nil {
		t.Fatalf("plugin server: %v", err)
	}

	fetched := false
	useDelegationFetch(t, func(_ context.Context, _ string) (io.ReadCloser, error) {
		fetched = true
		return io.NopCloser(strings.NewReader(fixtureDelegation)), nil
	})

	resp, err := handlerFor(t, "ze-update:resolve-rir")(&pluginserver.CommandContext{Server: srv}, nil)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status %q, want %q: an unreadable config was read as an unconfigured one", resp.Status, plugin.StatusError)
	}
	if !strings.Contains(resp.Error, configPath) {
		t.Errorf("the error does not name the config it could not read: %q", resp.Error)
	}
	if fetched {
		t.Error("the refresh fetched a delegation file before it knew where to read it from")
	}
}

// TestTheConfigFileSourceReachesTheRefreshReader proves the whole read the
// refresh performs: the YANG accepts the block, the parser builds the keyed
// list, and the extraction answers the URL under its registry token (AC-2).
// Goal: the editor parses leniently, so a block the schema refuses is dropped
// in silence and every later assertion would read as "no source committed".
// This test is where that shows. Method: write a config naming one registry,
// read it through the same function the handler calls, and require the one
// source and no other.
func TestTheConfigFileSourceReachesTheRefreshReader(t *testing.T) {
	const mirror = "https://mirror.example.net/delegated-lacnic-extended-latest"

	configPath := filepath.Join(t.TempDir(), "ze.conf")
	writeDelegationConfig(t, configPath,
		"system {\n\trir {\n\t\tdelegation-source lacnic {\n\t\t\turl \""+mirror+"\";\n\t\t}\n\t}\n}\n")

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{ConfigPath: configPath}, nil)
	if err != nil {
		t.Fatalf("plugin server: %v", err)
	}

	sources, err := configuredDelegationSources(&pluginserver.CommandContext{Server: srv})
	if err != nil {
		t.Fatalf("read the committed sources: %v", err)
	}
	if sources["lacnic"] != mirror {
		t.Errorf("lacnic reads from %q, want the committed %q", sources["lacnic"], mirror)
	}
	if len(sources) != 1 {
		t.Errorf("the config names one source and the reader answers %v", sources)
	}
}
