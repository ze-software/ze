package thanks

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/credits"
)

// fixtureCredits answers a fixed record, so these tests do not depend on the
// data generated into internal/core/credits. It records the name LicenceOf
// was asked for.
type fixtureCredits struct {
	works    []credits.Work
	document []byte
	texts    map[string][]byte
	err      error
	asked    *string
}

func (f fixtureCredits) Works() ([]credits.Work, error) { return f.works, f.err }

func (f fixtureCredits) LicenceDocument() ([]byte, error) { return f.document, f.err }

func (f fixtureCredits) LicenceOf(name string) ([]byte, error) {
	if f.asked != nil {
		*f.asked = name
	}
	if f.err != nil {
		return nil, f.err
	}
	text, carried := f.texts[name]
	if !carried {
		return nil, fmt.Errorf("%w: %s", credits.ErrUnknownWork, name)
	}
	return text, nil
}

// withName answers args carrying the declared operations named, one `name`
// each, as the partition hands them to the handler.
func withName(names ...[]string) command.ValidatedArgs {
	operations := make([]command.PipeOperation, 0, len(names))
	for _, words := range names {
		operations = append(operations, command.PipeOperation{Name: operationName, Args: words})
	}
	return command.ValidatedArgs{}.WithPipeOperations(operations)
}

// TestThanksListColumns: the list declares its columns in the order the spec
// names (group, name, version, licence, copyright, why, source) on both of its
// paths, and every row's JSON keys are exactly those columns, kebab-case.
// Method: read the column registry for both paths, then encode one row the
// handler answers and compare its key set to the declared order.
func TestThanksListColumns(t *testing.T) {
	want := []string{"group", "name", "version", "licence", "copyright", "why", "source"}
	for _, path := range []string{"thanks", "show thanks list"} {
		orders := command.ColumnsForCommand(path)
		if len(orders) != 1 || !slices.Equal([]string(orders[0]), want) {
			t.Fatalf("columns of %q = %v, want %v", path, orders, want)
		}
	}
	if orders := command.ColumnsForCommand("thanks licence"); len(orders) != 0 {
		t.Fatalf("the licence document inherited columns %v from the list", orders)
	}

	source := fixtureCredits{works: []credits.Work{{
		Name: "github.com/miekg/dns", Version: "v1.1.73", Group: "DNS", Licence: "BSD-3-Clause",
		Copyright: "Miek Gieben", Why: "DNS messages.", Source: "https://github.com/miekg/dns",
	}}}
	answer := listHandler(source)(command.ValidatedArgs{})
	if answer.Code != 0 {
		t.Fatalf("list refused: %q", answer.Diagnostic)
	}
	encoded, err := json.Marshal(answer.Payload)
	if err != nil {
		t.Fatalf("encode the list: %v", err)
	}
	var decoded map[string][]map[string]string
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode the list %s: %v", encoded, err)
	}
	rows := decoded[keyWorks]
	if len(rows) != 1 {
		t.Fatalf("list rows = %s, want one row under %q", encoded, keyWorks)
	}
	keys := make([]string, 0, len(rows[0]))
	for key := range rows[0] {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	sorted := slices.Sorted(slices.Values(want))
	if !slices.Equal(keys, sorted) {
		t.Fatalf("row keys = %v, want exactly the columns %v", keys, sorted)
	}
	if rows[0][keyName] != "github.com/miekg/dns" || rows[0][keyLicence] != "BSD-3-Clause" {
		t.Fatalf("row = %v, want the fixture's work", rows[0])
	}
}

// TestLicenceNameSelectsOneWork: `| name <work>` answers that work's text and
// nothing else; with no operation the whole document is answered; an unknown
// name is refused, naming `show thanks list` as the fix. Method: a fixture
// record of two texts and a document, asked through the handler each way.
func TestLicenceNameSelectsOneWork(t *testing.T) {
	var asked string
	source := fixtureCredits{
		document: []byte("WHOLE DOCUMENT"),
		texts: map[string][]byte{
			"github.com/miekg/dns":            []byte("BSD TEXT"),
			"Go standard library and runtime": []byte("GO TEXT"),
		},
		asked: &asked,
	}
	handler := licenceHandler(source)

	one := handler(withName([]string{"github.com/miekg/dns"}))
	if blob, isBlob := one.Payload.(command.Blob); one.Code != 0 || !isBlob || string(blob) != "BSD TEXT" {
		t.Fatalf("| name github.com/miekg/dns = %#v, want the one work's text", one)
	}
	if asked != "github.com/miekg/dns" {
		t.Fatalf("LicenceOf was asked %q", asked)
	}

	several := handler(withName([]string{"Go", "standard", "library", "and", "runtime"}))
	if blob, isBlob := several.Payload.(command.Blob); several.Code != 0 || !isBlob || string(blob) != "GO TEXT" {
		t.Fatalf("| name of several words = %#v, want the work whose name is those words", several)
	}

	whole := handler(command.ValidatedArgs{})
	if blob, isBlob := whole.Payload.(command.Blob); whole.Code != 0 || !isBlob || string(blob) != "WHOLE DOCUMENT" {
		t.Fatalf("no operation = %#v, want the whole document", whole)
	}

	unknown := handler(withName([]string{"example.com/not-carried"}))
	if unknown.Code == 0 || unknown.Payload != nil {
		t.Fatalf("an unknown name was answered: %#v", unknown)
	}
	if !strings.Contains(unknown.Diagnostic, "example.com/not-carried") ||
		!strings.HasSuffix(strings.TrimSuffix(unknown.Diagnostic, "\n"), commandList) {
		t.Fatalf("unknown-name refusal = %q, want the name and %q as its last line", unknown.Diagnostic, commandList)
	}

	twice := handler(withName([]string{"github.com/miekg/dns"}, []string{"Go"}))
	if twice.Code == 0 || twice.Payload != nil {
		t.Fatalf("two name operations were answered: %#v", twice)
	}
}

// TestLicenceDocumentStaysUnderTheRemoteCap asks whether the whole licence
// document of the REAL embedded record fits the size a daemon surface
// answers (spec R-4), so `show thanks licence` is never cut on SSH, REST or
// MCP. Method: build the document this test binary answers and compare its
// length with command.BlobRemoteOctetsMax.
func TestLicenceDocumentStaysUnderTheRemoteCap(t *testing.T) {
	document, err := credits.LicenceDocument()
	if err != nil {
		t.Fatalf("LicenceDocument() error = %v", err)
	}
	if len(document) == 0 {
		t.Fatal("the licence document is empty")
	}
	if len(document) >= command.BlobRemoteOctetsMax {
		t.Fatalf("the licence document is %d bytes, at or over the remote cap of %d", len(document), command.BlobRemoteOctetsMax)
	}
}

// TestListMarksAModuleWithNoRecordAndExitsNonZero asks whether `ze thanks`
// still lists a linked module the record lacks, with its licence cell
// "not recorded", and exits non-zero naming it (AC-5). Method: a fixture
// whose Works answers the full list beside an *UnrecordedError.
func TestListMarksAModuleWithNoRecordAndExitsNonZero(t *testing.T) {
	const missing = "example.com/missing"
	source := fixtureCredits{
		works: []credits.Work{
			{Name: "github.com/miekg/dns", Version: "v1.1.73", Group: "DNS", Licence: "BSD-3-Clause"},
			{Name: missing, Version: "v9.0.0", Group: credits.NotRecorded, Licence: credits.NotRecorded, Unrecorded: true},
		},
		err: &credits.UnrecordedError{Modules: []string{missing}},
	}
	answer := listHandler(source)(command.ValidatedArgs{})
	if answer.Code == 0 {
		t.Fatalf("the list exited 0 with a module unrecorded: %#v", answer)
	}
	if !strings.Contains(answer.Diagnostic, missing) ||
		!strings.HasSuffix(strings.TrimSuffix(answer.Diagnostic, "\n"), "./le deps credits update") {
		t.Fatalf("diagnostic = %q, want the module and the fix on its last line", answer.Diagnostic)
	}
	payload, isMap := answer.Payload.(map[string]any)
	if !isMap {
		t.Fatalf("payload = %#v, want the rows", answer.Payload)
	}
	rows, isRows := payload[keyWorks].([]workRow)
	if !isRows || len(rows) != 2 {
		t.Fatalf("rows = %#v, want both works", payload[keyWorks])
	}
	if rows[1].Name != missing || rows[1].Version != "v9.0.0" || rows[1].Licence != "not recorded" {
		t.Fatalf("unrecorded row = %+v, want %s v9.0.0 with licence %q", rows[1], missing, "not recorded")
	}
}

// TestThanksRefusesWithoutBuildInformation asks whether a binary with no
// build information refuses both commands, naming the launcher that builds
// it with that information (AC-6). Method: a fixture answering
// credits.ErrNoBuildInfo.
func TestThanksRefusesWithoutBuildInformation(t *testing.T) {
	source := fixtureCredits{err: credits.ErrNoBuildInfo}
	for name, answer := range map[string]command.LocalAnswer{
		"list":     listHandler(source)(command.ValidatedArgs{}),
		"document": licenceHandler(source)(command.ValidatedArgs{}),
	} {
		if answer.Code == 0 || answer.Payload != nil {
			t.Errorf("%s answered without build information: %#v", name, answer)
			continue
		}
		if !strings.Contains(answer.Diagnostic, "no build information") ||
			!strings.HasSuffix(strings.TrimSuffix(answer.Diagnostic, "\n"), "./ze thanks") {
			t.Errorf("%s refusal = %q, want the cause and ./ze thanks on the last line", name, answer.Diagnostic)
		}
	}
}

// TestThanksRefusesWithoutCredits: when the record cannot answer, both
// commands refuse with an operator error and print no list and no document.
// Method: a fixture whose every reader answers credits.ErrNoData.
func TestThanksRefusesWithoutCredits(t *testing.T) {
	source := fixtureCredits{err: credits.ErrNoData}
	for name, answer := range map[string]command.LocalAnswer{
		"list":     listHandler(source)(command.ValidatedArgs{}),
		"document": licenceHandler(source)(command.ValidatedArgs{}),
		"one work": licenceHandler(source)(withName([]string{"github.com/miekg/dns"})),
	} {
		if answer.Code == 0 || answer.Payload != nil {
			t.Errorf("%s answered without credits: %#v", name, answer)
			continue
		}
		if !strings.Contains(answer.Diagnostic, credits.ErrNoData.Error()) ||
			!strings.HasSuffix(strings.TrimSuffix(answer.Diagnostic, "\n"), "./le deps credits update") {
			t.Errorf("%s refusal = %q, want the cause and the fix on the last line", name, answer.Diagnostic)
		}
	}
}

// TestThanksDaemonPathsMatchTheConstants: each registration's DaemonPath, a
// literal because the YANG/handler contract reads it from source, is the
// daemon path the refusals name and the shape and column declarations key on.
// Method: read both registrations back from the local command registry this
// package's init() filled, and compare each DaemonPath with its constant.
func TestThanksDaemonPathsMatchTheConstants(t *testing.T) {
	want := map[string]string{"thanks": commandList, pathLicence: commandLicence}
	seen := make(map[string]bool, len(want))
	for _, entry := range command.ListLocal() {
		daemonPath, registered := want[entry.Path]
		if !registered {
			continue
		}
		seen[entry.Path] = true
		if entry.Meta.DaemonPath != daemonPath {
			t.Errorf("registration %q names daemon path %q, want %q", entry.Path, entry.Meta.DaemonPath, daemonPath)
		}
	}
	for path := range want {
		if !seen[path] {
			t.Errorf("no local command is registered at %q", path)
		}
	}
}
