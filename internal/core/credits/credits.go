// Design: docs/architecture/third-party-credits.md -- what a binary lists, and the licence document
// Related: internal/plugins/thanks/thanks.go -- the commands that answer from it
//
// Package credits is the record of the third-party works this binary carries:
// the list `ze thanks` answers and the licence texts `ze thanks licence`
// prints. It copies the core-leaf registry shape of internal/core/ipsecinventory:
// it imports the standard library only, a package that carries a work which is
// not a Go module declares it present from its own init() (DeclarePresent), and
// a reader asks for value-type records.
//
// The records are generated from website/data/dependencies.json into
// internal/core/credits/data by `./le deps credits update`, and embedded here.
// What a binary lists is what it carries: a module is listed when the binary's
// build information links it, at the version that information states; Go
// itself is listed in every binary, at the build information's Go version; any
// other work that is not a module is listed when a package declared it present.

package credits

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
)

// data is the generated record and its licence texts. Nobody edits it by hand.
//
//go:embed data
var data embed.FS

// Work is one credited third-party work as this binary carries it. It is a
// self-contained value type: it crosses a component boundary and holds no
// pointer into this package's state.
type Work struct {
	// Name is the work's exact name: the module path of a Go module, or the
	// declared name of a work that is not one. It is what `| name` selects on.
	Name string
	// Version is the version this binary carries: the build information's for
	// a module and for Go itself, the declared one for any other work, which
	// may declare none.
	Version string
	// Group is the heading of the curated list the work is listed under.
	Group string
	// Licence is the licence identifier the declaration records, or
	// NotRecorded.
	Licence string
	// Copyright is the copyright holder the declaration records.
	Copyright string
	// CopyrightUnstated is set when the work's own files state no holder.
	CopyrightUnstated bool
	// Why is the sentence that says why Ze uses the work.
	Why string
	// Source is where the work's source can be obtained.
	Source string
	// Unrecorded is set on a module the build information links that the
	// generated record does not hold: it is listed, never omitted, with its
	// version and the licence NotRecorded, because a missing record is a
	// defect of the build, not a module the binary does not carry.
	Unrecorded bool
}

// NotRecorded is the licence and group of a linked module with no record.
const NotRecorded = "not recorded"

// unstatedHolder is what the licence document prints for a work whose files
// state no copyright holder.
const unstatedHolder = "no copyright holder stated"

// The record's two kinds of work, as the generator spells them.
const (
	kindModule = "module"
	kindWork   = "work"
)

// recordName is the record file inside the data directory.
const recordName = "credits.json"

// ErrNoData is the answer of every reader when the credits record compiled
// into this binary cannot be read: it is missing or does not parse, a work has
// an unknown kind or no file, it names a text that is missing, or it marks no
// work as the Go toolchain. It is distinct from any answer a binary with a usable record gives,
// so a reader never mistakes it for a list with no work in it.
var ErrNoData = errors.New("the credits record compiled into this binary is unusable")

// ErrUnknownWork is the answer of LicenceOf for a name that names no work this
// binary carries. Callers match it with errors.Is; the error LicenceOf returns
// names the work asked for.
var ErrUnknownWork = errors.New("no work of that name is carried by this binary")

// ErrNotRecorded is the answer of LicenceOf for a module this binary links
// whose record the generated data lacks, so no licence text can be printed.
var ErrNotRecorded = errors.New("this binary links the module, and its credits record holds no entry for it")

// ErrNoBuildInfo is the answer when the binary carries no build information:
// without it the binary cannot tell which modules it links, and a list from
// the record alone would credit what it may not carry.
var ErrNoBuildInfo = errors.New("this binary carries no build information, so it cannot list the modules it links")

// UnrecordedError is the answer beside a complete list or document when
// modules this binary links have no record: the list still holds a row for
// each, and the caller exits non-zero naming them.
type UnrecordedError struct {
	// Modules are the module paths with no record, sorted.
	Modules []string
}

// Error names every module with no record.
func (e *UnrecordedError) Error() string {
	return "no credits record for " + strings.Join(e.Modules, ", ")
}

// record is data/credits.json, decoded exactly as internal/le/deps/credits
// (record, workRecord, fileRecord) writes it. An unknown key is refused, so
// the two sides cannot drift apart in silence.
type record struct {
	GeneratedBy string       `json:"generated-by"`
	Declaration string       `json:"declaration-sha256"`
	Groups      []string     `json:"groups"`
	Works       []workRecord `json:"works"`
}

// workRecord is one shipped work of the record.
type workRecord struct {
	Name              string       `json:"name"`
	Kind              string       `json:"kind"`
	Version           string       `json:"version"`
	Toolchain         bool         `json:"toolchain"`
	Group             string       `json:"group"`
	Licence           string       `json:"licence"`
	Copyright         string       `json:"copyright"`
	CopyrightUnstated bool         `json:"copyright-unstated"`
	Why               string       `json:"why"`
	Source            string       `json:"source"`
	Files             []fileRecord `json:"files"`
}

// fileRecord is one licence or notice file of a work.
type fileRecord struct {
	Name   string `json:"name"`
	Origin string `json:"origin"`
	Text   string `json:"text"`
}

// catalog is a decoded record with the file system its texts are read
// from. Only loadCatalog makes one, so every text it names exists.
type catalog struct {
	rec   record
	texts fs.FS
}

// loadCatalog decodes the record in texts and checks that every text it
// names is there, that each work has a known kind and a file, and that one
// work is marked as the Go toolchain.
func loadCatalog(texts fs.FS) (catalog, error) {
	raw, err := fs.ReadFile(texts, recordName)
	if err != nil {
		return catalog{}, noData("read the credits record: " + err.Error())
	}
	var rec record
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rec); err != nil {
		return catalog{}, noData("the credits record does not parse: " + err.Error())
	}
	toolchain := false
	for index := range rec.Works {
		work := &rec.Works[index]
		if work.Kind != kindModule && work.Kind != kindWork {
			return catalog{}, noData("the credits record gives " + work.Name + " the unknown kind " + work.Kind)
		}
		if len(work.Files) == 0 {
			return catalog{}, noData("the credits record lists no licence file for " + work.Name)
		}
		for _, file := range work.Files {
			if _, err := fs.Stat(texts, file.Text); err != nil {
				return catalog{}, noData("the credits record names " + file.Text + " for " + work.Name + ", which is missing: " + err.Error())
			}
		}
		if work.Toolchain {
			toolchain = true
		}
	}
	// Go itself is the one row every binary lists, so a record with no
	// toolchain work would let a binary that links no module and carries no
	// declared work answer an empty list as if it carried nothing.
	if !toolchain {
		return catalog{}, noData("the credits record marks no work as the Go toolchain, which every binary lists")
	}
	return catalog{rec: rec, texts: texts}, nil
}

// noData is ErrNoData with the reason the record is unusable.
func noData(reason string) error {
	return &dataError{reason: reason}
}

// dataError wraps ErrNoData with its reason.
type dataError struct{ reason string }

// Error states the reason.
func (e *dataError) Error() string { return ErrNoData.Error() + ": " + e.reason }

// Unwrap answers ErrNoData.
func (e *dataError) Unwrap() error { return ErrNoData }

// work answers the record of the work named name, and false when the record
// holds none.
func (c catalog) work(name string) (workRecord, bool) {
	for index := range c.rec.Works {
		if c.rec.Works[index].Name == name {
			return c.rec.Works[index], true
		}
	}
	return workRecord{}, false
}

// compiled is the embedded catalog, decoded on first use.
var compiled = sync.OnceValues(func() (catalog, error) {
	texts, err := fs.Sub(data, "data")
	if err != nil {
		return catalog{}, noData(err.Error())
	}
	return loadCatalog(texts)
})

// presence is the set of works declared present. It is written from init()
// and read after, under its lock.
var presence = struct {
	sync.Mutex
	names map[string]bool
}{names: make(map[string]bool)}

// DeclarePresent records that the calling package carries the work named
// name, a work that is not a Go module (a web asset, a copied or ported
// source).
//
// It MUST be called from the carrying package's init(), with a literal name
// (the check reads the literals). Two packages may declare the same work. A
// name the generated data does not hold as such a work is a defect in the
// declaration or in the data, found before the binary answers anything, so it
// panics with BUG as the other core registries do for a registration they
// refuse. So does the toolchain work: every binary lists Go without a
// declaration, and one would be a second route nothing reads. Safe for
// concurrent use.
func DeclarePresent(name string) {
	known, err := compiled()
	if err != nil {
		panic("BUG: credits.DeclarePresent(" + name + "): " + err.Error())
	}
	declarePresentIn(known, name)
	presence.Lock()
	presence.names[name] = true
	presence.Unlock()
}

// declarePresentIn panics unless known holds name as a work that is not a
// module and not the toolchain.
func declarePresentIn(known catalog, name string) {
	work, held := known.work(name)
	if !held || work.Kind != kindWork {
		panic("BUG: credits.DeclarePresent(" + name + "): the generated credits data holds no work of that name; " +
			"declare it in website/data/dependencies.json and run ./le deps credits update")
	}
	if work.Toolchain {
		panic("BUG: credits.DeclarePresent(" + name + "): this is the Go toolchain's work, which every binary lists " +
			"without a declaration; remove the call")
	}
}

// isPresent answers whether a package declared name present.
func isPresent(name string) bool {
	presence.Lock()
	defer presence.Unlock()
	return presence.names[name]
}

// Works answers one record per third-party work this binary carries, ordered
// by group as the declaration orders its groups, then by name. Beside the full
// list it answers an *UnrecordedError when linked modules have no record, and
// ErrNoBuildInfo with no list when the binary carries no build information.
// Safe for concurrent use.
func Works() ([]Work, error) {
	known, err := compiled()
	if err != nil {
		return nil, err
	}
	info, ok := debug.ReadBuildInfo()
	carried, err := known.carried(info, ok, isPresent)
	return publicWorks(carried), err
}

// LicenceDocument answers the licence document of every work this binary
// carries, with the same errors as Works. Safe for concurrent use.
func LicenceDocument() ([]byte, error) {
	known, err := compiled()
	if err != nil {
		return nil, err
	}
	info, ok := debug.ReadBuildInfo()
	carried, unrecorded := known.carried(info, ok, isPresent)
	if errors.Is(unrecorded, ErrNoBuildInfo) {
		return nil, unrecorded
	}
	document, err := known.document(carried)
	if err != nil {
		return nil, err
	}
	return document, unrecorded
}

// LicenceOf answers the licence document of the one carried work whose name
// is exactly its argument: ErrUnknownWork when no carried work has that name,
// ErrNotRecorded for a linked module with no record. Safe for concurrent use.
func LicenceOf(name string) ([]byte, error) {
	known, err := compiled()
	if err != nil {
		return nil, err
	}
	info, ok := debug.ReadBuildInfo()
	carried, unrecorded := known.carried(info, ok, isPresent)
	if errors.Is(unrecorded, ErrNoBuildInfo) {
		return nil, unrecorded
	}
	return known.licenceOf(carried, name)
}

// carriedWork is a carried work with the files its licence document prints.
type carriedWork struct {
	Work
	files []fileRecord
}

// carried joins the record with what this binary carries: each module the
// build information links, at its version, the toolchain work at the build
// information's Go version, and each other work declared present. A linked
// module with no record is a row marked Unrecorded, and the answer beside the
// full list is then an *UnrecordedError naming every such module.
func (c catalog) carried(info *debug.BuildInfo, ok bool, present func(string) bool) ([]carriedWork, error) {
	// ReadBuildInfo answers not ok for a binary built without module support:
	// the only source of what it links is missing, so there is no list.
	if !ok || info == nil {
		return nil, ErrNoBuildInfo
	}
	var works []carriedWork
	var unrecorded []string
	for _, dep := range info.Deps {
		module := dep
		// A replaced module links the replacement's code.
		if dep.Replace != nil {
			module = dep.Replace
		}
		work, held := c.work(dep.Path)
		if !held || work.Kind != kindModule {
			unrecorded = append(unrecorded, dep.Path)
			works = append(works, carriedWork{
				Name: dep.Path, Version: module.Version, Group: NotRecorded, Licence: NotRecorded, Unrecorded: true,
			})
			continue
		}
		works = append(works, carriedWork{Work: publicWork(&work, module.Version), files: work.Files})
	}
	for index := range c.rec.Works {
		work := &c.rec.Works[index]
		if work.Kind != kindWork {
			continue
		}
		// Go itself is built into every binary, whatever it links, at the
		// toolchain's version, which the build information states and the
		// declaration cannot. No package declares it, so no binary can miss it.
		if work.Toolchain {
			works = append(works, carriedWork{Work: publicWork(work, info.GoVersion), files: work.Files})
			continue
		}
		if !present(work.Name) {
			continue
		}
		works = append(works, carriedWork{Work: publicWork(work, work.Version), files: work.Files})
	}
	slices.SortFunc(works, c.compare)
	if len(unrecorded) == 0 {
		return works, nil
	}
	slices.Sort(unrecorded)
	return works, &UnrecordedError{Modules: unrecorded}
}

// compare orders two works by the declaration's group order, a group the
// record does not order (NotRecorded) last, then by name.
func (c catalog) compare(a, b carriedWork) int {
	rankA, rankB := c.groupRank(a.Group), c.groupRank(b.Group)
	if rankA != rankB {
		return rankA - rankB
	}
	return strings.Compare(a.Name, b.Name)
}

// groupRank is the position of group in the declaration's order, and the
// position after every group for one it does not order.
func (c catalog) groupRank(group string) int {
	rank := slices.Index(c.rec.Groups, group)
	if rank < 0 {
		return len(c.rec.Groups)
	}
	return rank
}

// publicWork is the Work of a record at version.
func publicWork(work *workRecord, version string) Work {
	return Work{
		Name: work.Name, Version: version, Group: work.Group, Licence: work.Licence,
		Copyright: work.Copyright, CopyrightUnstated: work.CopyrightUnstated,
		Why: work.Why, Source: work.Source,
	}
}

// publicWorks strips the files from carried works.
func publicWorks(carried []carriedWork) []Work {
	if carried == nil {
		return nil
	}
	works := make([]Work, 0, len(carried))
	for _, work := range carried {
		works = append(works, work.Work)
	}
	return works
}

// The lines that frame each text of the licence document.
const (
	textHeading = "================================================================================\n"
	textRule    = "--------------------------------------------------------------------------------\n"
)

// document renders the licence document of works. For each distinct text,
// in the order the works and their files list them: a heading naming every
// work it covers with its version and file name, its copyright line and its
// source location, then the text byte for byte. Works sharing an identical
// text share one copy. A module with no record ends the document, named
// with the reason it has no text.
func (c catalog) document(works []carriedWork) ([]byte, error) {
	type textBlock struct {
		text   string
		header bytes.Buffer
	}
	var blocks []*textBlock
	var unrecorded []string
	for _, work := range works {
		if work.Unrecorded {
			unrecorded = append(unrecorded, work.Name)
			continue
		}
		for _, file := range work.files {
			index := slices.IndexFunc(blocks, func(block *textBlock) bool { return block.text == file.Text })
			if index < 0 {
				blocks = append(blocks, &textBlock{text: file.Text})
				index = len(blocks) - 1
			}
			writeWorkHeader(&blocks[index].header, work.Work, file.Name)
		}
	}
	var out bytes.Buffer
	for _, block := range blocks {
		text, err := fs.ReadFile(c.texts, block.text)
		if err != nil {
			return nil, errors.New("read the licence text " + block.text + ": " + err.Error())
		}
		out.WriteString(textHeading)
		out.Write(block.header.Bytes())
		out.WriteString(textRule)
		out.Write(text)
		// The text is printed as it ships; a separator line follows it only
		// when it does not end one itself.
		if !bytes.HasSuffix(text, []byte("\n")) {
			out.WriteByte('\n')
		}
	}
	if len(unrecorded) != 0 {
		out.WriteString(textHeading)
		out.WriteString("No licence text is recorded for these modules this binary links:\n")
		for _, name := range unrecorded {
			out.WriteString("  " + name + "\n")
		}
	}
	return out.Bytes(), nil
}

// writeWorkHeader writes the lines naming one work above a text it ships as
// file.
func writeWorkHeader(header *bytes.Buffer, work Work, file string) {
	header.WriteString(work.Name)
	if work.Version != "" {
		header.WriteString(" " + work.Version)
	}
	header.WriteString(", " + file + "\n")
	holder := work.Copyright
	if work.CopyrightUnstated {
		holder = unstatedHolder
	}
	header.WriteString("  Copyright: " + holder + "\n")
	header.WriteString("  Source: " + work.Source + "\n")
}

// licenceOf renders the document of the one carried work named name.
func (c catalog) licenceOf(works []carriedWork, name string) ([]byte, error) {
	for _, work := range works {
		if work.Name != name {
			continue
		}
		if work.Unrecorded {
			return nil, &workError{name: name, cause: ErrNotRecorded}
		}
		return c.document([]carriedWork{work})
	}
	return nil, &workError{name: name, cause: ErrUnknownWork}
}

// workError is a LicenceOf refusal: it names the work asked for and wraps
// ErrUnknownWork or ErrNotRecorded, which callers match with errors.Is.
type workError struct {
	name  string
	cause error
}

// Error names the work and the cause.
func (e *workError) Error() string { return e.name + ": " + e.cause.Error() }

// Unwrap answers the cause.
func (e *workError) Unwrap() error { return e.cause }
