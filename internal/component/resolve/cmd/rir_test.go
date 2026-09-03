// Design: docs/architecture/resolve.md -- wiring tests for the two RIR commands
package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
