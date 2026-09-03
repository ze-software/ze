// Related: ianaasn.go -- the generator these tests call as a function

package ianaasn

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr"
)

// delegation is one RIR's file, in the format the five publish: pipe-separated
// records, a header line, and rows for every resource type.
const delegation = "2|ripencc|20260826|1|0|20260826|+0000\n" +
	"ripencc|*|asn|*|3|summary\n" +
	"ripencc|EU|asn|1|1|19930901|allocated\n" +
	"ripencc|EU|asn|2|2|19930901|assigned\n" +
	"ripencc|EU|asn|100|1|19930901|reserved\n" +
	"ripencc|EU|ipv4|10.0.0.0|256|19930901|allocated\n" +
	"# a comment\n" +
	"\n"

// registryTokens are the five registries a run reads, spelled the way a
// delegation URL spells them.
//
// A fixture recognizes a file by the token in the URL it is handed, never by
// the URL itself: the addresses belong to the irr package, and a test that
// copied them would be a second declaration of them.
var registryTokens = []string{"ripencc", "arin", "apnic", "afrinic", "lacnic"}

// tokenFor answers which registry a delegation URL names, and fails the test
// for a URL no fixture registry answers. A run that asks for a sixth file has
// changed what a seed is built from, and that MUST be seen rather than served.
func tokenFor(t *testing.T, delegationURL string) string {
	t.Helper()

	for _, token := range registryTokens {
		if strings.Contains(delegationURL, "/"+token+"/") {
			return token
		}
	}
	t.Fatalf("no fixture registry answers %s", delegationURL)
	return ""
}

// fetchFrom answers a fetch that serves one body per registry and records which
// registries were asked for, in order.
func fetchFrom(t *testing.T, bodies map[string]string, asked *[]string) irr.DelegationFetch {
	t.Helper()

	return func(_ context.Context, delegationURL string) (io.ReadCloser, error) {
		token := tokenFor(t, delegationURL)
		*asked = append(*asked, token)
		body, held := bodies[token]
		if !held {
			return nil, errors.New("no such file")
		}
		return io.NopCloser(strings.NewReader(body)), nil
	}
}

// everyRIR answers a body for each of the five delegation files, so a whole run
// reaches the write.
func everyRIR(body string) map[string]string {
	bodies := map[string]string{}
	for _, token := range registryTokens {
		bodies[token] = body
	}
	return bodies
}

func tree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(outputFile)), 0o755); err != nil {
		t.Fatalf("create the irr package directory: %v", err)
	}
	return root
}

// VALIDATES: a whole run fetches all five delegation files and writes the table.
// PREVENTS: a seed built from a subset, which hands one RIR's blocks to whichever
// registry did answer.
func TestWriteFetchesEveryRegistryAndWritesTheTable(t *testing.T) {
	root := tree(t)
	var asked []string

	report, err := Write(context.Background(), root, fetchFrom(t, everyRIR(delegation), &asked))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	for _, token := range registryTokens {
		if !slices.Contains(asked, token) {
			t.Errorf("the run never fetched the %s delegation file, it fetched %v", token, asked)
		}
	}
	if len(asked) != len(registryTokens) {
		t.Errorf("the run fetched %d files, want %d", len(asked), len(registryTokens))
	}
	if report.Records != 2*len(registryTokens) {
		t.Errorf("the run took %d records, want two per registry", report.Records)
	}
	if report.Ranges != 1 {
		t.Errorf("the run wrote %d ranges, want one collapsed range", report.Ranges)
	}

	body, err := os.ReadFile(filepath.Join(root, outputFile))
	if err != nil {
		t.Fatalf("read the generated table: %v", err)
	}
	if !strings.Contains(string(body), "1 3 ripencc\n") {
		t.Errorf("the generated table does not carry the collapsed range:\n%s", body)
	}
}

// VALIDATES: a run that parsed no ASN record writes NOTHING and answers an
// error.
// PREVENTS: an empty source set must not replace the compiled table. Five HTTP
// 200 responses carrying no ASN records would otherwise leave an empty table
// committed while the update still reported success.
func TestARunThatParsedNothingWritesNothing(t *testing.T) {
	root := tree(t)
	var asked []string

	_, err := Write(context.Background(), root, fetchFrom(t, everyRIR("2|ripencc|20260826|0|0|20260826|+0000\n"), &asked))
	if err == nil {
		t.Fatal("Write accepted five answers holding no ASN record")
	}
	if _, statErr := os.Stat(filepath.Join(root, outputFile)); !os.IsNotExist(statErr) {
		t.Errorf("Write created the table from nothing: %v", statErr)
	}
}

// VALIDATES: a registry that does not answer stops the run before anything is
// written.
// PREVENTS: a table missing one RIR's whole delegation, which reads as "this ASN
// belongs to nobody" for every AS number that registry holds.
func TestOneRegistryFailingStopsTheRun(t *testing.T) {
	root := tree(t)
	bodies := everyRIR(delegation)
	delete(bodies, "apnic")
	var asked []string

	if _, err := Write(context.Background(), root, fetchFrom(t, bodies, &asked)); err == nil {
		t.Fatal("Write accepted a run where one registry did not answer")
	}
	if _, statErr := os.Stat(filepath.Join(root, outputFile)); !os.IsNotExist(statErr) {
		t.Errorf("Write left a partial table behind: %v", statErr)
	}
}

// VALIDATES: the table is built in memory and written once.
// PREVENTS: a truncated file. os.Create truncates before the first byte is
// written, so a generator that streams and fails midway leaves the irr package
// holding half a table that does not compile.
func TestTheTableIsWrittenInOneCall(t *testing.T) {
	root := tree(t)
	// A read-only directory: the write fails, and the point is that it fails
	// BEFORE the existing file is touched.
	write(t, root, outputFile, "# Generated: 2026-08-16\n1 3 ripencc\n")
	before, err := os.ReadFile(filepath.Join(root, outputFile))
	if err != nil {
		t.Fatalf("read the existing table: %v", err)
	}

	var asked []string
	bodies := everyRIR(delegation)
	delete(bodies, "ripencc")
	if _, writeErr := Write(context.Background(), root, fetchFrom(t, bodies, &asked)); writeErr == nil {
		t.Fatal("Write accepted a failed fetch")
	}

	after, err := os.ReadFile(filepath.Join(root, outputFile))
	if err != nil {
		t.Fatalf("read the table after the failed run: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("a failed run rewrote the table:\n%s", after)
	}
}

// VALIDATES: the report renders in the words the script printed.
// PREVENTS: a report a developer cannot read at a terminal.
func TestTheReportRendersTheScriptWording(t *testing.T) {
	report := WriteReport{File: outputFile, Ranges: 12, Records: 34}
	want := "wrote internal/component/resolve/irr/rir-delegation.txt (12 collapsed ranges from 34 records)\n"
	if got := report.Text(); got != want {
		t.Errorf("the report renders\n%q\nwant\n%q", got, want)
	}
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create the fixture directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write the fixture %s: %v", rel, err)
	}
}

// eachRegistryOnce answers one body for each delegation file, each registry
// delegating a different block of AS numbers, and the ranges the five together
// declare. Different blocks are what let a round trip tell the five apart.
func eachRegistryOnce(t *testing.T) (map[string]string, []irr.RIREntry) {
	t.Helper()

	registries := []struct {
		token string
		rir   string
		whois string
		start uint32
		count uint32
	}{
		{"ripencc", irr.RIRRIPE, irr.WhoisRIPE, 1000, 10},
		{"arin", irr.RIRARIN, irr.WhoisARIN, 2000, 5},
		{"apnic", irr.RIRAPNIC, irr.WhoisAPNIC, 3000, 1},
		{"afrinic", irr.RIRAFRINIC, irr.WhoisAFRINIC, 4000, 10},
		{"lacnic", irr.RIRLACNIC, irr.WhoisLACNIC, 5000, 2},
	}

	if len(registries) != len(registryTokens) {
		t.Fatalf("the fixture declares %d registries, want %d", len(registries), len(registryTokens))
	}

	bodies := map[string]string{}
	var want []irr.RIREntry

	for _, registry := range registries {
		bodies[registry.token] = fmt.Sprintf("2|%s|20260826|1|0|20260826|+0000\n%s|EU|asn|%d|%d|19930901|allocated\n",
			registry.token, registry.token, registry.start, registry.count)
		want = append(want, irr.RIREntry{
			Start: registry.start,
			End:   registry.start + registry.count - 1,
			RIR:   registry.rir,
			Whois: registry.whois,
		})
	}

	slices.SortFunc(want, func(a, b irr.RIREntry) int { return cmp.Compare(a.Start, b.Start) })
	return bodies, want
}

// VALIDATES: a whole run writes the file the irr package renders for the
// ranges the five registries declared, under a header carrying the generation
// date and one Source line for each file (AC-11).
// PREVENTS: the generator and the runtime disagreeing about the format. Two
// parsers is how they came to disagree about a guard, and a file the lookup
// cannot read is every AS number unanswerable rather than a smaller table.
//
// The bytes are compared against irr.RenderDelegationTable, which is the irr
// package's own writer, and TestARenderedTableParsesBackToWhatItHeld there
// proves that package reads back what it writes. The table's reader is
// unexported, so this test asserts the writer's half here and takes the
// reader's half from the package that owns it.
func TestWriteEmitsAFileTheIRRParserReads(t *testing.T) {
	root := tree(t)
	bodies, want := eachRegistryOnce(t)
	var asked []string

	before := time.Now().UTC()
	if _, err := Write(context.Background(), root, fetchFrom(t, bodies, &asked)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	after := time.Now().UTC()

	body, err := os.ReadFile(filepath.Join(root, outputFile))
	if err != nil {
		t.Fatalf("read the written table: %v", err)
	}

	if got := strings.Count(string(body), "\n# Source: "); got != len(registryTokens) {
		t.Errorf("the header carries %d Source lines, want %d", got, len(registryTokens))
	}
	if got := "# Generated: " + before.Format(time.DateOnly); !strings.Contains(string(body), got) {
		t.Errorf("the header does not carry %q:\n%s", got, body)
	}

	// The run stamps the table from its own clock, so a run that crosses
	// midnight UTC writes the next day's date. Both readings are accepted here,
	// and no other file is.
	for _, stamp := range []time.Time{before, after} {
		rendered, renderErr := irr.RenderDelegationTable(irr.DelegationTable{Generated: stamp, Ranges: want})
		if renderErr != nil {
			t.Fatalf("RenderDelegationTable: %v", renderErr)
		}
		if bytes.Equal(body, rendered) {
			return
		}
	}
	t.Errorf("the run wrote\n%s\nwhich is not what the irr package renders for %+v", body, want)
}

// VALIDATES: a delegation record whose last AS number does not fit in a uint32
// reaches no table, and a run holding nothing else writes nothing (AC-12).
// PREVENTS: the wrapped range the generator used to publish. Its own parser
// had no overflow guard, so a record starting near the top of the space became
// a range ending below its start, which the lookup's binary search reads as a
// hole rather than as a defect.
func TestAnOversizedRecordReachesNoTable(t *testing.T) {
	root := tree(t)
	var asked []string

	oversized := "2|ripencc|20260826|1|0|20260826|+0000\n" +
		"ripencc|EU|asn|4294967290|10|19930901|allocated\n"

	if _, err := Write(context.Background(), root, fetchFrom(t, everyRIR(oversized), &asked)); err == nil {
		t.Fatal("Write accepted a run whose only record ends above a uint32")
	}
	if _, statErr := os.Stat(filepath.Join(root, outputFile)); !os.IsNotExist(statErr) {
		t.Errorf("Write created a table from a record it cannot represent: %v", statErr)
	}
}
