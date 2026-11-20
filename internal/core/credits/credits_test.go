package credits

import (
	"errors"
	"io/fs"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// TestPresenceOfAnUndeclaredWorkPanics: a package that declares a work the
// generated data does not hold is a defect, refused at start-up with a BUG
// panic naming the work, never accepted in silence. Method: declare a name no
// declaration will ever carry and recover the panic.
func TestPresenceOfAnUndeclaredWorkPanics(t *testing.T) {
	const name = "a work no declaration names"
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("DeclarePresent(%q) was accepted", name)
		}
		message, isText := recovered.(string)
		if !isText {
			t.Fatalf("DeclarePresent(%q) panicked with %#v, want a BUG string", name, recovered)
		}
		if !strings.HasPrefix(message, "BUG: ") || !strings.Contains(message, name) {
			t.Fatalf("DeclarePresent(%q) panicked with %q, want a BUG naming the work", name, message)
		}
	}()
	DeclarePresent(name)
}

// TestPresenceOfTheToolchainWorkPanics: Go is listed in every binary without a
// declaration, so a package that declares it present writes a second route
// nothing reads, refused at start-up with a BUG panic. Method: declare the
// fixture's toolchain work against the fixture record and recover the panic.
func TestPresenceOfTheToolchainWorkPanics(t *testing.T) {
	const name = "Go standard library and runtime"
	known := fixtureCatalog(t)
	defer func() {
		message, isText := recover().(string)
		if !isText || !strings.HasPrefix(message, "BUG: ") || !strings.Contains(message, "toolchain") {
			t.Fatalf("declaring %q present panicked with %q, want a BUG naming the toolchain", name, message)
		}
	}()
	declarePresentIn(known, name)
}

// toolchainWork is the work the embedded record marks toolchain, read from
// the record rather than spelled here.
func toolchainWork(t *testing.T, known catalog) workRecord {
	t.Helper()
	for index := range known.rec.Works {
		if known.rec.Works[index].Toolchain {
			return known.rec.Works[index]
		}
	}
	t.Fatal("the embedded credits record marks no work toolchain")
	return workRecord{}
}

// TestRowsListGoWithNothingDeclared asks whether a binary in which no package
// declares any work present still lists Go, at the version of the toolchain
// that built it (AC-2), with no error: this package's own test binary is such
// a binary, and it links no module either (it imports the standard library
// only), so nothing is unrecorded. A binary that does not link the thanks
// plugin must never answer an empty list as if it carried no third-party
// work. Method: call the public reader in this test binary.
func TestRowsListGoWithNothingDeclared(t *testing.T) {
	known, err := compiled()
	if err != nil {
		t.Fatalf("the embedded credits record is unusable: %v", err)
	}
	goWork := toolchainWork(t, known)
	works, err := Works()
	if err != nil {
		t.Fatalf("Works() error = %v, want none: this test binary links no module", err)
	}
	if len(works) == 0 {
		t.Fatal("Works() answered no row, want Go")
	}
	index := slices.IndexFunc(works, func(work Work) bool { return work.Name == goWork.Name })
	if index < 0 {
		t.Fatalf("Works() = %+v, want a row for %q", works, goWork.Name)
	}
	if works[index].Version != runtime.Version() {
		t.Errorf("%s version = %q, want %q", goWork.Name, works[index].Version, runtime.Version())
	}
}

// TestLicenceListsGoWithNothingDeclared asks whether the licence document of
// a binary in which nothing is declared present holds Go's licence texts byte
// for byte under Go's heading, never an empty document with no error, and
// whether a module this binary does not link is refused as unknown rather
// than printed. Method: this package's test binary, the texts read from the
// embedded record.
func TestLicenceListsGoWithNothingDeclared(t *testing.T) {
	known, err := compiled()
	if err != nil {
		t.Fatalf("the embedded credits record is unusable: %v", err)
	}
	goWork := toolchainWork(t, known)
	document, err := LicenceDocument()
	if err != nil {
		t.Fatalf("LicenceDocument() error = %v, want none: this test binary links no module", err)
	}
	if len(document) == 0 {
		t.Fatal("LicenceDocument() answered an empty document")
	}
	for _, file := range goWork.Files {
		text, err := fs.ReadFile(known.texts, file.Text)
		if err != nil {
			t.Fatalf("read %s of %s: %v", file.Text, goWork.Name, err)
		}
		if !strings.Contains(string(document), string(text)) {
			t.Errorf("the document does not hold %s of %s:\n%s", file.Name, goWork.Name, document)
		}
		heading := goWork.Name + " " + runtime.Version() + ", " + file.Name + "\n"
		if !strings.Contains(string(document), heading) {
			t.Errorf("the document does not name %q:\n%s", heading, document)
		}
	}
	one, err := LicenceOf("github.com/miekg/dns")
	if !errors.Is(err, ErrUnknownWork) || one != nil {
		t.Errorf("LicenceOf(github.com/miekg/dns) = %q, %v, want no bytes and ErrUnknownWork", one, err)
	}
}

// TestRowsListTheToolchainUndeclared asks whether the join lists the
// toolchain work when the presence function declares nothing, at the build
// information's Go version, and leaves out every other work nobody declared.
// Method: the fixture record joined with a go1.99 build that links no module
// and a presence that declares nothing.
func TestRowsListTheToolchainUndeclared(t *testing.T) {
	works, err := fixtureCatalog(t).carried(&debug.BuildInfo{GoVersion: "go1.99"}, true, declared())
	if err != nil {
		t.Fatalf("carried() error = %v", err)
	}
	if len(works) != 1 {
		t.Fatalf("carried() = %+v, want the Go row alone", works)
	}
	if works[0].Name != "Go standard library and runtime" || works[0].Version != "go1.99" || works[0].Group != "Go Toolchain" {
		t.Errorf("row = %+v, want Go standard library and runtime go1.99 in Go Toolchain", works[0].Work)
	}
}

// The fixture record: two modules sharing one BSD text, Go itself, a web
// asset whose files state no holder, and a work no package declares present.
// The groups are declared in an order that is not their sorted order.
const fixtureRecord = `{
  "generated-by": "./le deps credits update",
  "declaration-sha256": "00",
  "groups": ["Networking", "Go Toolchain", "Web"],
  "works": [
    {"name": "Go standard library and runtime", "kind": "work", "toolchain": true, "group": "Go Toolchain",
     "licence": "BSD-3-Clause", "copyright": "The Go Authors", "why": "The language.", "source": "https://github.com/golang/go",
     "files": [{"name": "LICENSE", "origin": "third_party/go/LICENSE", "text": "texts/go"}]},
    {"name": "absent", "kind": "work", "version": "1.0", "group": "Web", "licence": "MIT", "copyright": "Nobody",
     "source": "https://example.org/absent", "files": [{"name": "LICENSE", "origin": "third_party/absent/LICENSE", "text": "texts/absent"}]},
    {"name": "example.com/dns", "kind": "module", "group": "Networking", "licence": "BSD-3-Clause", "copyright": "Miek",
     "why": "DNS.", "source": "https://example.com/dns",
     "files": [{"name": "LICENSE", "origin": "vendor/example.com/dns/LICENSE", "text": "texts/bsd"}]},
    {"name": "example.com/net", "kind": "module", "group": "Networking", "licence": "BSD-3-Clause", "copyright": "The Net Authors",
     "source": "https://example.com/net",
     "files": [{"name": "LICENSE.txt", "origin": "vendor/example.com/net/LICENSE.txt", "text": "texts/bsd"}]},
    {"name": "htmx", "kind": "work", "version": "4.0.0", "group": "Web", "licence": "0BSD", "copyright": "",
     "copyright-unstated": true, "why": "The web interface.", "source": "https://github.com/bigskysoftware/htmx",
     "files": [{"name": "htmx.LICENSE", "origin": "third_party/web/htmx/htmx.LICENSE", "text": "texts/zero"}]}
  ]
}
`

// The fixture texts; bsdText has no trailing newline, as a shipped file may.
const (
	bsdText    = "Copyright (c) Miek. All rights reserved.\nRedistribution and use in source and binary forms are permitted."
	goText     = "Copyright 2009 The Go Authors.\n"
	zeroText   = "Zero-Clause BSD\n"
	absentText = "MIT License\n"
)

// fixtureCatalog loads fixtureRecord and its texts through loadCatalog,
// so the tests read the record as the embedded one is read.
func fixtureCatalog(t *testing.T) catalog {
	t.Helper()
	known, err := loadCatalog(fstest.MapFS{
		recordName:     {Data: []byte(fixtureRecord)},
		"texts/bsd":    {Data: []byte(bsdText)},
		"texts/go":     {Data: []byte(goText)},
		"texts/zero":   {Data: []byte(zeroText)},
		"texts/absent": {Data: []byte(absentText)},
	})
	if err != nil {
		t.Fatalf("load the fixture record: %v", err)
	}
	return known
}

// fixtureBuild is a binary built with go1.99 that links the two recorded
// modules, one of them replaced, and extra modules with no record.
func fixtureBuild(extra ...string) *debug.BuildInfo {
	info := &debug.BuildInfo{GoVersion: "go1.99", Deps: []*debug.Module{
		{Path: "example.com/net", Version: "v0.4.0"},
		{Path: "example.com/dns", Version: "v1.0.0", Replace: &debug.Module{Path: "example.com/dns", Version: "v1.2.3"}},
	}}
	for _, module := range extra {
		info.Deps = append(info.Deps, &debug.Module{Path: module, Version: "v9.0.0"})
	}
	return info
}

// declared answers present for the named works only.
func declared(names ...string) func(string) bool {
	return func(name string) bool { return slices.Contains(names, name) }
}

// TestTheEmbeddedRecordLoads asks whether the REAL embedded record decodes
// with this package's strict decoder, holds works, and names only texts that
// are there and not empty. The completeness of the record against what a
// binary links is TestEveryLinkedModuleHasACredit in internal/plugins/thanks,
// whose test binary links the product's packages; this package's own test
// binary links no module. Method: load the compiled catalog and read every
// text it names.
func TestTheEmbeddedRecordLoads(t *testing.T) {
	known, err := compiled()
	if err != nil {
		t.Fatalf("the embedded credits record is unusable: %v", err)
	}
	if len(known.rec.Works) == 0 || len(known.rec.Groups) == 0 {
		t.Fatalf("the embedded record holds %d works in %d groups", len(known.rec.Works), len(known.rec.Groups))
	}
	for _, work := range known.rec.Works {
		for _, file := range work.Files {
			text, err := fs.ReadFile(known.texts, file.Text)
			if err != nil || len(text) == 0 {
				t.Errorf("%s: licence text %s for %s is missing or empty: %v", work.Name, file.Text, file.Name, err)
			}
		}
	}
}

// TestRowsFollowTheBuildAndThePresence asks whether the list is what the
// binary carries (AC-1, AC-2, AC-4): modules at the build information's
// version (a replacement's when replaced), Go at the build's Go version, a
// work only when declared present, ordered by the declaration's groups and
// then by name. Method: join the fixture record with a fixture build and a
// presence set that leaves out "absent".
func TestRowsFollowTheBuildAndThePresence(t *testing.T) {
	works, err := fixtureCatalog(t).carried(fixtureBuild(), true, declared("htmx"))
	if err != nil {
		t.Fatalf("carried() error = %v", err)
	}
	type row struct{ group, name, version string }
	var got []row
	for _, work := range works {
		got = append(got, row{work.Group, work.Name, work.Version})
	}
	want := []row{
		{"Networking", "example.com/dns", "v1.2.3"},
		{"Networking", "example.com/net", "v0.4.0"},
		{"Go Toolchain", "Go standard library and runtime", "go1.99"},
		{"Web", "htmx", "4.0.0"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %v\nwant   %v", got, want)
	}
}

// TestRowsMarkAModuleWithNoRecord asks whether a linked module the record
// lacks is still a row (AC-5): its version, the licence "not recorded", and
// an *UnrecordedError beside the full list naming it, so the command exits
// non-zero. Method: a fixture build linking one module with no record.
func TestRowsMarkAModuleWithNoRecord(t *testing.T) {
	const missing = "example.com/missing"
	works, err := fixtureCatalog(t).carried(fixtureBuild(missing), true, declared())
	var unrecorded *UnrecordedError
	if !errors.As(err, &unrecorded) {
		t.Fatalf("carried() error = %v, want an *UnrecordedError", err)
	}
	if !slices.Equal(unrecorded.Modules, []string{missing}) {
		t.Errorf("unrecorded modules = %q, want %q", unrecorded.Modules, missing)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q does not name %s", err, missing)
	}
	if len(works) != 4 {
		t.Fatalf("carried() answered %d rows, want the two recorded modules, Go and the unrecorded one", len(works))
	}
	last := works[len(works)-1]
	if last.Name != missing || !last.Unrecorded || last.Version != "v9.0.0" || last.Licence != NotRecorded {
		t.Errorf("unrecorded row = %+v, want %s v9.0.0 marked unrecorded with licence %q", last.Work, missing, NotRecorded)
	}
}

// TestRowsRefuseWithoutBuildInformation asks whether a binary with no build
// information is refused rather than answered from the record alone (AC-6).
// Method: join the fixture record with a build information read that
// answers not ok.
func TestRowsRefuseWithoutBuildInformation(t *testing.T) {
	works, err := fixtureCatalog(t).carried(nil, false, declared("htmx"))
	if !errors.Is(err, ErrNoBuildInfo) {
		t.Fatalf("carried() error = %v, want ErrNoBuildInfo", err)
	}
	if works != nil {
		t.Fatalf("carried() answered %d rows beside its error", len(works))
	}
}

// TestLicenceDocumentGroupsIdenticalTexts asks whether works sharing a
// byte-identical text share one copy, each named above it with its version,
// file name, copyright and source, and whether the text is printed byte for
// byte (AC-7). Method: the two fixture modules ship the same BSD text under
// different file names; the document must hold it once, after both headers.
func TestLicenceDocumentGroupsIdenticalTexts(t *testing.T) {
	known := fixtureCatalog(t)
	works, err := known.carried(fixtureBuild(), true, declared("htmx"))
	if err != nil {
		t.Fatalf("carried() error = %v", err)
	}
	raw, err := known.document(works)
	if err != nil {
		t.Fatalf("document() error = %v", err)
	}
	document := string(raw)
	if count := strings.Count(document, bsdText); count != 1 {
		t.Fatalf("the shared text appears %d times, want once:\n%s", count, document)
	}
	text := strings.Index(document, bsdText)
	for _, header := range []string{
		"example.com/dns v1.2.3, LICENSE\n  Copyright: Miek\n  Source: https://example.com/dns\n",
		"example.com/net v0.4.0, LICENSE.txt\n  Copyright: The Net Authors\n  Source: https://example.com/net\n",
	} {
		at := strings.Index(document, header)
		if at < 0 || at > text {
			t.Errorf("header %q is not above the shared text:\n%s", header, document)
		}
	}
	if !strings.Contains(document, "htmx 4.0.0, htmx.LICENSE\n  Copyright: "+unstatedHolder+"\n") {
		t.Errorf("htmx is not named with %q:\n%s", unstatedHolder, document)
	}
	if strings.Contains(document, absentText) {
		t.Errorf("the document prints the text of a work no package declared present:\n%s", document)
	}
}

// TestLicenceOfAnswersOneWorkOrRefuses asks whether one work's document holds
// that work's text only (AC-8), and whether a name no carried work has, and a
// linked module with no record, are refused with their own errors. Method:
// ask the fixture for htmx, for a work not declared present, and for an
// unrecorded module.
func TestLicenceOfAnswersOneWorkOrRefuses(t *testing.T) {
	known := fixtureCatalog(t)
	works, _ := known.carried(fixtureBuild("example.com/missing"), true, declared("htmx")) //nolint:errcheck // the UnrecordedError is this test's third case
	one, err := known.licenceOf(works, "htmx")
	if err != nil {
		t.Fatalf("licenceOf(htmx) error = %v", err)
	}
	if !strings.Contains(string(one), zeroText) || strings.Contains(string(one), bsdText) {
		t.Errorf("licenceOf(htmx) = %q, want htmx's text alone", one)
	}
	if _, err := known.licenceOf(works, "absent"); !errors.Is(err, ErrUnknownWork) || !strings.Contains(err.Error(), "absent") {
		t.Errorf("licenceOf(absent) error = %v, want ErrUnknownWork naming it", err)
	}
	if _, err := known.licenceOf(works, "example.com/missing"); !errors.Is(err, ErrNotRecorded) {
		t.Errorf("licenceOf(example.com/missing) error = %v, want ErrNotRecorded", err)
	}
}

// TestRecordWithAnUnknownKeyIsRefused asks whether a record key this package
// does not know is refused, so the generator and this decoder cannot drift
// apart in silence. Method: add a key to the fixture record.
func TestRecordWithAnUnknownKeyIsRefused(t *testing.T) {
	changed := strings.Replace(fixtureRecord, `"groups":`, `"color": "blue", "groups":`, 1)
	_, err := loadCatalog(fstest.MapFS{recordName: {Data: []byte(changed)}})
	if !errors.Is(err, ErrNoData) || !strings.Contains(err.Error(), "color") {
		t.Fatalf("loadCatalog() error = %v, want ErrNoData naming the key", err)
	}
}

// loadFixtureRecord loads record with every fixture text present, so only
// the record itself can make the load fail.
func loadFixtureRecord(record string) error {
	_, err := loadCatalog(fstest.MapFS{
		recordName:     {Data: []byte(record)},
		"texts/bsd":    {Data: []byte(bsdText)},
		"texts/go":     {Data: []byte(goText)},
		"texts/zero":   {Data: []byte(zeroText)},
		"texts/absent": {Data: []byte(absentText)},
	})
	return err
}

// requireNoData fails unless err is ErrNoData and its message names cause.
func requireNoData(t *testing.T, err error, cause string) {
	t.Helper()
	if !errors.Is(err, ErrNoData) || !strings.Contains(err.Error(), cause) {
		t.Fatalf("loadCatalog() error = %v, want ErrNoData naming %q", err, cause)
	}
}

// TestRecordWithNoToolchainWorkIsRefused asks whether a record that marks no
// work as the Go toolchain is refused when it is loaded: Go is the one row
// every binary lists, so without it a binary linking no module and carrying
// no declared work would answer an empty list and an empty document with no
// error. Method: the fixture record with its Go entry cut out.
func TestRecordWithNoToolchainWorkIsRefused(t *testing.T) {
	start := strings.Index(fixtureRecord, `{"name": "Go standard library and runtime"`)
	end := strings.Index(fixtureRecord, `{"name": "absent"`)
	if start < 0 || end < start {
		t.Fatal("the fixture record no longer holds the Go entry before absent")
	}
	changed := fixtureRecord[:start] + fixtureRecord[end:]
	if strings.Contains(changed, `"toolchain"`) {
		t.Fatal("the cut record still marks a toolchain work")
	}
	requireNoData(t, loadFixtureRecord(changed), "marks no work as the Go toolchain")
}

// TestMissingRecordIsRefused asks whether a data directory with no record is
// refused rather than read as a record with no work. Method: load a file
// system holding only a text.
func TestMissingRecordIsRefused(t *testing.T) {
	_, err := loadCatalog(fstest.MapFS{"texts/go": {Data: []byte(goText)}})
	requireNoData(t, err, "read the credits record")
}

// TestRecordWithAnUnknownKindIsRefused asks whether a work whose kind is
// neither module nor work is refused, naming the work and the kind, rather
// than left out of every list. Method: give the fixture's "absent" work the
// kind "asset".
func TestRecordWithAnUnknownKindIsRefused(t *testing.T) {
	changed := strings.Replace(fixtureRecord, `"name": "absent", "kind": "work"`, `"name": "absent", "kind": "asset"`, 1)
	if changed == fixtureRecord {
		t.Fatal("the fixture record no longer spells absent's kind")
	}
	requireNoData(t, loadFixtureRecord(changed), "gives absent the unknown kind asset")
}

// TestRecordWithAWorkWithNoFileIsRefused asks whether a work listing no
// licence file is refused, naming the work, since the document could print
// no text for it. Method: empty the files of the fixture's "absent" work.
func TestRecordWithAWorkWithNoFileIsRefused(t *testing.T) {
	changed := strings.Replace(fixtureRecord,
		`"files": [{"name": "LICENSE", "origin": "third_party/absent/LICENSE", "text": "texts/absent"}]`, `"files": []`, 1)
	if changed == fixtureRecord {
		t.Fatal("the fixture record no longer spells absent's files")
	}
	requireNoData(t, loadFixtureRecord(changed), "lists no licence file for absent")
}

// TestRecordNamingAMissingTextIsRefused asks whether a record that names a
// text the data lacks is refused when it is loaded, before any document
// prints without it. Method: load the fixture record without texts/zero.
func TestRecordNamingAMissingTextIsRefused(t *testing.T) {
	_, err := loadCatalog(fstest.MapFS{
		recordName:     {Data: []byte(fixtureRecord)},
		"texts/bsd":    {Data: []byte(bsdText)},
		"texts/go":     {Data: []byte(goText)},
		"texts/absent": {Data: []byte(absentText)},
	})
	if !errors.Is(err, ErrNoData) || !strings.Contains(err.Error(), "texts/zero") {
		t.Fatalf("loadCatalog() error = %v, want ErrNoData naming texts/zero", err)
	}
}
