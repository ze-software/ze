// Design: docs/architecture/third-party-credits.md -- the declaration, the generated copy and its layout
// Detail: check.go -- what `deps credits check` refuses
// Detail: update.go -- what `deps credits update` writes
// Detail: licences.go -- licence identifiers, holder lines and bundle banners
// Detail: presence.go -- the credits.DeclarePresent calls read from Go source
// Detail: golist.go -- the modules each shipped flavor links
// Detail: gate.go -- the commit gate on newly linked modules and changed licence files
//
// Package credits generates and checks the record of the third-party works the
// shipped ze binary carries. website/data/dependencies.json is the one
// declaration; internal/core/credits/data is the copy the binary embeds, which
// only `./le deps credits update` writes.

package credits

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// The paths and names the tool reads and writes, relative to the checkout.
const (
	// DeclarationRel is the curated declaration of every credited work.
	DeclarationRel = "website/data/dependencies.json"
	// DataRel is the directory the binary embeds (internal/core/credits).
	DataRel = "internal/core/credits/data"
	// RecordName is the record file inside DataRel.
	RecordName = "credits.json"
	// TextsDir is the directory inside DataRel holding the licence texts.
	TextsDir = "texts"
	// creditsPackage is the core package whose DeclarePresent a carrying
	// package calls, relative to the Ze module path.
	creditsPackage = "internal/core/credits"
	// vendorRoot is where the go command keeps each vendored module.
	vendorRoot = "vendor"
	// linkedGroup is the group of a module the `linked` list declares: the
	// heading of the website section that lists the shipped works the curated
	// groups do not.
	linkedGroup = "Also in the shipped binary"
	// updateCommand is the fix every refusal ends with: the declaration is the
	// only input a person edits, and update derives the rest from it.
	updateCommand = "./le deps credits update"
)

// Kinds of credited work, as the record spells them.
const (
	kindModule = "module"
	kindWork   = "work"
)

// declaration is website/data/dependencies.json as this tool reads it. The
// site build reads the same file into its own type (internal/le/site,
// dependencyData) for the page; this one reads the licence fields too.
type declaration struct {
	Categories []category       `json:"categories"`
	Linked     []declaredModule `json:"linked"`
	Works      []declaredWork   `json:"works"`
}

// category is one curated group of direct modules.
type category struct {
	Name    string           `json:"name"`
	Modules []declaredModule `json:"modules"`
}

// declaredModule is a Go module the declaration credits.
type declaredModule struct {
	Module            string `json:"module"`
	Why               string `json:"why"`
	Licence           string `json:"licence"`
	Copyright         string `json:"copyright"`
	CopyrightUnstated bool   `json:"copyright-unstated"`
	CopyrightFile     string `json:"copyright-file"`
	Source            string `json:"source"`
	// LicenceFiles are repository paths that state the module's licence
	// beside the licence files at its root, such as a README that marks the
	// work CC0. They are checked and copied after the root files.
	LicenceFiles []string `json:"licence-files"`
	// LicenceNote says what the licence identifier alone cannot, such as a
	// GPL version whose only or or-later variant upstream does not state.
	LicenceNote string `json:"licence-note"`
}

// declaredWork is a credited work that is not a vendored Go module: Go
// itself, a web asset, code copied or ported into Ze's tree.
type declaredWork struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	// Toolchain marks the work that is the Go toolchain itself: a binary
	// reads its version from its own build information, so the work declares
	// none.
	Toolchain         bool     `json:"toolchain"`
	Licence           string   `json:"licence"`
	Copyright         string   `json:"copyright"`
	CopyrightUnstated bool     `json:"copyright-unstated"`
	CopyrightFile     string   `json:"copyright-file"`
	Source            string   `json:"source"`
	Why               string   `json:"why"`
	Group             string   `json:"group"`
	Files             []string `json:"files"`
	CarriedBy         []string `json:"carried-by"`
	// Banner is a phrase the work's banner in a bundle's companion file
	// writes, when that banner does not name the package: "Recursive object
	// extending" is deep-extend's. It ties the banner to the work.
	Banner string `json:"banner"`
}

// readDeclaration parses the declaration. An unknown key is refused rather
// than skipped: a misspelled "copyright-unstated" read as absent would be a
// silent default.
func readDeclaration(raw []byte) (declaration, error) {
	var decl declaration
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decl); err != nil {
		return declaration{}, errors.New(DeclarationRel + " does not parse: " + err.Error())
	}
	return decl, nil
}

// entry is one declared work, module or not, in the form every check reads.
type entry struct {
	name      string
	kind      string
	list      string
	group     string
	version   string
	toolchain bool
	why       string
	licence   string
	copyright string
	unstated  bool
	// copyrightFile is a repository path holding the holder when no licence
	// file of the work states one.
	copyrightFile string
	source        string
	// declaredFiles are the repository paths a works entry lists, in order.
	declaredFiles []string
	// licenceFiles are a module's declared licence-files.
	licenceFiles []string
	carriedBy    []string
	// banner is a works entry's declared banner phrase.
	banner string
}

// The list names a refusal uses to say where an entry is declared.
const (
	listCurated = "categories"
	listLinked  = "linked"
	listWorks   = "works"
)

// entries answers every declared work in declaration order: the curated
// modules, then the linked ones, then the works.
func (d declaration) entries() []entry {
	var all []entry
	for _, group := range d.Categories {
		for index := range group.Modules {
			all = append(all, moduleEntry(&group.Modules[index], listCurated, group.Name))
		}
	}
	for index := range d.Linked {
		all = append(all, moduleEntry(&d.Linked[index], listLinked, linkedGroup))
	}
	for index := range d.Works {
		work := &d.Works[index]
		all = append(all, entry{
			banner: work.Banner,
			name:   work.Name, kind: kindWork, list: listWorks, group: work.Group, version: work.Version,
			toolchain: work.Toolchain,
			why:       work.Why, licence: work.Licence, copyright: work.Copyright, unstated: work.CopyrightUnstated,
			copyrightFile: work.CopyrightFile, source: work.Source,
			declaredFiles: work.Files, carriedBy: work.CarriedBy,
		})
	}
	return all
}

// moduleEntry is one declared module as an entry of list, under group.
func moduleEntry(module *declaredModule, list, group string) entry {
	return entry{
		name: module.Module, kind: kindModule, list: list, group: group, why: module.Why,
		licence: module.Licence, copyright: module.Copyright, unstated: module.CopyrightUnstated,
		copyrightFile: module.CopyrightFile, source: module.Source, licenceFiles: module.LicenceFiles,
	}
}

// record is internal/core/credits/data/credits.json. Its key order is the
// field order here and its works are sorted by name, so two runs over one
// tree write the same bytes.
type record struct {
	// GeneratedBy names the command that writes the file, because JSON
	// carries no comment to say the file is generated.
	GeneratedBy string `json:"generated-by"`
	// Declaration is the SHA-256 of the declaration bytes the record was
	// generated from, so the commit gate tells a declaration edited without
	// regenerating from one regenerated, with no go command.
	Declaration string `json:"declaration-sha256"`
	// Groups are the group names of the shipped works in the order the
	// declaration first names them (its categories, then the linked list,
	// then the works), so a binary lists its works under the groups in the
	// declaration's order, which sorted works do not carry.
	Groups []string     `json:"groups"`
	Works  []workRecord `json:"works"`
}

// workRecord is one shipped work as the binary reads it.
type workRecord struct {
	Name string `json:"name"`
	// Kind is "module" for a vendored Go module, whose version the binary
	// reads from its own build information, and "work" for anything else.
	Kind string `json:"kind"`
	// Version is the declared version of a work; a module has none here.
	Version string `json:"version,omitempty"`
	// Toolchain marks the Go toolchain's own work, whose version the binary
	// reads from its build information, as it does a module's.
	Toolchain         bool   `json:"toolchain,omitempty"`
	Group             string `json:"group"`
	Licence           string `json:"licence"`
	Copyright         string `json:"copyright"`
	CopyrightUnstated bool   `json:"copyright-unstated,omitempty"`
	Why               string `json:"why,omitempty"`
	Source            string `json:"source"`
	// Files are the work's licence and notice files, in the order they are
	// printed.
	Files []fileRecord `json:"files"`
}

// fileRecord is one licence or notice file of a work.
type fileRecord struct {
	// Name is the file's own name, as the work ships it (LICENSE, NOTICE).
	Name string `json:"name"`
	// Origin is the repository path the text was copied from.
	Origin string `json:"origin"`
	// Text is the stored copy, relative to the data directory:
	// texts/<SHA-256 of the bytes in hex>. Works sharing a byte-identical
	// text share the one copy.
	Text string `json:"text"`
}

// work answers the record of the work named name, and false when the
// record holds none.
func (r record) work(name string) (workRecord, bool) {
	for index := range r.Works {
		if r.Works[index].Name == name {
			return r.Works[index], true
		}
	}
	return workRecord{}, false
}

// generated is everything update writes under the data directory: the
// encoded record and each text, keyed by the path relative to it.
type generated struct {
	record record
	files  map[string][]byte
}

// textName is where a text is stored, relative to the data directory. The
// name is lowercase hex, so go:embed takes it: no leading dot or underscore,
// no capital letter and no separator from a module path.
func textName(content []byte) string {
	return path.Join(TextsDir, digest(content))
}

// digest is the lowercase hex SHA-256 of raw.
func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// encodeRecord renders the record as update writes it: two-space indent, no
// HTML escaping, a trailing newline.
func encodeRecord(rec record) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(rec); err != nil {
		return nil, errors.New("render " + RecordName + ": " + err.Error())
	}
	return buffer.Bytes(), nil
}

// decodeRecord parses a record update wrote.
func decodeRecord(raw []byte) (record, error) {
	var rec record
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rec); err != nil {
		return record{}, errors.New(path.Join(DataRel, RecordName) + " does not parse: " + err.Error())
	}
	return rec, nil
}

// licencePrefixes are the file-name prefixes, in capitals, of the licence and
// notice files a module carries at its root: the names `go mod vendor` keeps
// beside the code. Their order is the order a module's files print in.
var licencePrefixes = []string{"LICENSE", "LICENCE", "COPYING", "NOTICE", "PATENTS", "AUTHORS", "COPYRIGHT"}

// licenceRank answers the position of name's prefix in licencePrefixes, and
// false when name is not a licence or notice file.
func licenceRank(name string) (int, bool) {
	upper := strings.ToUpper(name)
	for rank, prefix := range licencePrefixes {
		if strings.HasPrefix(upper, prefix) {
			return rank, true
		}
	}
	return 0, false
}

// moduleFiles answers the repository paths of the licence and notice files at
// the root of a vendored module, in print order, and an error naming the
// directory when it cannot be read.
func moduleFiles(root, module string) ([]string, error) {
	dir := path.Join(vendorRoot, module)
	listing, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return nil, errors.New("read " + dir + ": " + err.Error())
	}
	type ranked struct {
		rank int
		name string
	}
	var found []ranked
	for _, item := range listing {
		if !item.Type().IsRegular() {
			continue
		}
		rank, licence := licenceRank(item.Name())
		if !licence {
			continue
		}
		found = append(found, ranked{rank: rank, name: item.Name()})
	}
	slices.SortFunc(found, func(a, b ranked) int {
		if a.rank != b.rank {
			return a.rank - b.rank
		}
		return strings.Compare(a.name, b.name)
	})
	files := make([]string, 0, len(found))
	for _, file := range found {
		files = append(files, path.Join(dir, file.name))
	}
	return files, nil
}

// readRel reads a checkout-relative file.
func readRel(root, rel string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // a path of the checkout this tool was pointed at
	if err != nil {
		return nil, errors.New("read " + rel + ": " + err.Error())
	}
	return raw, nil
}
