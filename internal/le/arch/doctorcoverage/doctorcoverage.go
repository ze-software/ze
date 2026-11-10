// Design: docs/architecture/doctor-and-health-checks.md -- the doctor-coverage gate
//
// Detail: sites.go -- which calls are problem sites, and the code each names
// Detail: codes.go -- the covered codes the tree's doctor check declarations list
// Detail: report.go -- what check and report answer
//
// doctorcoverage.go finds the problems an operator can hit. A Warn or Error
// log line, a report-bus raise and a health row are each a problem site: the
// operator meets the problem in the log, in `show warnings` or in the health
// endpoint. ze doctor reports every problem with its cause and fix
// (ai/META-DECISIONS.md), so each site names a doctor code that a registered
// doctor check declares, and that check is what answers for it.
//
// The covered codes are READ from the doctor check declarations in the same
// parsed tree (codes.go), never from a list and never from the registries
// linked into le: a check is a cover in the tree side that declares it, so a
// commit adding a check and its first use is judged against that check. There
// is no exempt or transient marker: a transient class raises
// its event on the report bus under its code, so the event is reported under
// it rather than only logged. A comment is never
// read, so no comment can exempt a site.

package archdoctorcoverage

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// modulePath is the import path prefix of the tree's own packages.
const modulePath = "github.com/ze-software/ze"

// parseRoots are the trees whose shipped Go is parsed: sites come from part of
// them, constants from all of them (a code is declared in internal/core).
var parseRoots = []string{"cmd", "internal", "pkg"}

// siteRoots are the trees whose problem sites the gate judges: the product's
// components, plugins and programs. internal/core holds libraries whose
// callers decide what an operator sees, so its calls are not judged here.
var siteRoots = []string{"internal/component", "internal/plugins", "cmd/ze"}

// excludedDirs are the subtrees under parseRoots that do not ship.
var excludedDirs = []string{"internal/le", "internal/test"}

// Kind is the shape of a problem site.
type Kind uint8

// The site kinds. The zero value is no kind, so a Site nobody classified
// cannot read as a log line.
const (
	KindUnspecified Kind = iota
	KindLogWarn
	KindLogError
	KindReportWarning
	KindReportError
	KindHealthRow
)

// String names the kind as report prints it.
func (k Kind) String() string {
	switch k {
	case KindLogWarn:
		return "log-warn"
	case KindLogError:
		return "log-error"
	case KindReportWarning:
		return "report-warning"
	case KindReportError:
		return "report-error"
	case KindHealthRow:
		return "health-row"
	case KindUnspecified:
		return "unspecified"
	}
	return "unknown-kind-" + strconv.Itoa(int(k))
}

// MarshalText renders the kind by name in JSON and YAML.
func (k Kind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// Status is what one site answers about its doctor code.
type Status uint8

// The site statuses. The zero value is no status, so a Site nobody judged
// cannot read as covered.
const (
	StatusUnspecified Status = iota
	// StatusCovered names a code a doctor check declaration in the tree lists.
	StatusCovered
	// StatusNoCode names no code: a log line with no `code` attribute.
	StatusNoCode
	// StatusCodeDynamic names a code the scan cannot read from the source: a
	// variable, a call, a field. Only a literal or a constant can be judged.
	StatusCodeDynamic
	// StatusCodeNotDoctor names a code no doctor check declaration in the tree
	// lists.
	StatusCodeNotDoctor
	// StatusHealthRow is a health registration. It carries a row name and no
	// code field, so it cannot name a code at the site; the audit maps each
	// one to its doctor check by hand, and the check form does not refuse it.
	StatusHealthRow
)

// String names the status as report prints it.
func (s Status) String() string {
	switch s {
	case StatusCovered:
		return "covered"
	case StatusNoCode:
		return "no-code"
	case StatusCodeDynamic:
		return "code-dynamic"
	case StatusCodeNotDoctor:
		return "code-not-doctor"
	case StatusHealthRow:
		return "health-row"
	case StatusUnspecified:
		return "unspecified"
	}
	return "unknown-status-" + strconv.Itoa(int(s))
}

// MarshalText renders the status by name in JSON and YAML.
func (s Status) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// refused reports whether the check form refuses a site of this status on a
// changed line.
func (s Status) refused() bool {
	switch s {
	case StatusNoCode, StatusCodeDynamic, StatusCodeNotDoctor:
		return true
	case StatusCovered, StatusHealthRow:
		return false
	case StatusUnspecified:
		panic("BUG: doctor-coverage site judged with no status")
	}
	panic("BUG: doctor-coverage site with unknown status " + strconv.Itoa(int(s)))
}

// Site is one problem site. Paths are relative to the scanned tree,
// slash-separated.
type Site struct {
	File string `json:"file"`
	Line int    `json:"line"`
	// EndLine is the last line of the call, so a change to any argument line
	// reaches the site.
	EndLine int  `json:"end-line"`
	Kind    Kind `json:"kind"`
	// Call is the called function as written: `log.Warn`, `report.RaiseError`.
	Call string `json:"call"`
	// Message is the log message, the report message or the health row name:
	// the literal when the source holds one, otherwise the expression.
	Message string `json:"message"`
	// CodeExpr is the code argument as written, empty when the site has none.
	CodeExpr string `json:"code-expr"`
	// Code is the code CodeExpr resolves to, empty when it does not resolve.
	Code   string `json:"code"`
	Status Status `json:"status"`
	// Checks names the doctor checks that declare Code, when it is covered.
	Checks []string `json:"checks"`
	// Package is the owner package: the directory holding File.
	Package string `json:"package"`
	// Doors are the door pages covering File (ai/doors/README.md).
	Doors []string `json:"doors"`
}

// Inventory is a scan's whole answer.
type Inventory struct {
	Sites []Site `json:"sites"`
	// Codes is how many codes the covered set held.
	Codes int `json:"codes"`
}

// Scan reads every shipped Go file under tree's parse roots, reads the covered
// set from the doctor check declarations among them, finds the problem sites
// under the site roots, and judges each site's code against that set. A
// covered set below codesFloor codes is an error (checkCodesFloor). doorsOf
// names the door pages covering a file.
//
// A file that does not parse is an error rather than a file with no site: a
// gate that skips what it cannot read reports a clean tree for it.
func Scan(tree string, codesFloor int, doorsOf func(file string) []string) (Inventory, error) {
	return scan(tree, nil, codesFloor, doorsOf)
}

// scan is Scan with an overlay: each overlaid file is read from the overlay
// instead of the working tree, so the commit seam can scan the tree as the
// commit leaves it and as its parent holds it (report.go, judgeChange). The
// covered set comes from the same overlaid tree, so each side is judged by the
// checks that side declares.
func scan(tree string, overlay map[string]overlayFile, codesFloor int, doorsOf func(file string) []string) (Inventory, error) {
	index, err := indexTree(tree, overlay)
	if err != nil {
		return Inventory{}, err
	}
	covered := index.coveredCodes()
	if err := checkCodesFloor(covered, codesFloor); err != nil {
		return Inventory{}, err
	}
	inventory := Inventory{Sites: []Site{}, Codes: len(covered)}
	for _, file := range index.files {
		if !underAny(file.path, siteRoots) {
			continue
		}
		sites := index.sitesOf(file)
		for i := range sites {
			site := &sites[i]
			judge(site, covered)
			site.Package = file.dir
			site.Doors = doorsOf(file.path)
			inventory.Sites = append(inventory.Sites, *site)
		}
	}
	slices.SortFunc(inventory.Sites, func(a, b Site) int {
		if a.File != b.File {
			return strings.Compare(a.File, b.File)
		}
		return a.Line - b.Line
	})
	return inventory, nil
}

// judge sets a site's status from its code and the covered set.
func judge(site *Site, covered Covered) {
	if site.Kind == KindHealthRow {
		site.Status = StatusHealthRow
		return
	}
	if site.CodeExpr == "" {
		site.Status = StatusNoCode
		return
	}
	if site.Code == "" {
		site.Status = StatusCodeDynamic
		return
	}
	checks, declared := covered[site.Code]
	if !declared {
		site.Status = StatusCodeNotDoctor
		return
	}
	site.Status = StatusCovered
	site.Checks = checks
}

// sourceFile is one parsed shipped file.
type sourceFile struct {
	// path is relative to the tree, slash-separated; dir is its directory.
	path string
	dir  string
	ast  *ast.File
	// imports maps each name the file refers to an import by, to its path.
	imports map[string]string
}

// treeIndex is every parsed shipped file and the constants they declare. It
// is built once per scan and only read after; it is not safe for concurrent
// use while indexTree builds it.
type treeIndex struct {
	fset  *token.FileSet
	files []*sourceFile
	// constants maps a directory to the constants its files declare.
	constants map[string]map[string]constRef
	// variables maps a directory to the package-level variables its files
	// declare with a value: a doctor check's code list may be one (codes.go).
	variables map[string]map[string]constRef
	// packageNames maps a directory to the package name its files declare.
	packageNames map[string]string
}

// constRef is one declared constant or package-level variable: its value
// expression and its file.
type constRef struct {
	expr ast.Expr
	file *sourceFile
}

// overlayFile replaces one file's working-tree bytes for a scan: a commit's
// carried text, or a path that side does not hold (present false).
type overlayFile struct {
	text    string
	present bool
}

// indexTree parses every shipped Go file under tree's parse roots. A path the
// overlay holds is read from it: present, its text replaces the working-tree
// file, or adds a file the tree does not hold; absent, the file is left out.
func indexTree(tree string, overlay map[string]overlayFile) (*treeIndex, error) {
	paths, err := shippedPaths(tree)
	if err != nil {
		return nil, err
	}
	for path, file := range overlay {
		if !shipped(path) {
			continue
		}
		if !file.present {
			continue
		}
		if slices.Contains(paths, path) {
			continue
		}
		paths = append(paths, path)
	}
	slices.Sort(paths)

	index := &treeIndex{
		fset:         token.NewFileSet(),
		constants:    make(map[string]map[string]constRef),
		variables:    make(map[string]map[string]constRef),
		packageNames: make(map[string]string),
	}
	for _, path := range paths {
		var source any
		if replaced, isOverlaid := overlay[path]; isOverlaid {
			if !replaced.present {
				continue
			}
			source = replaced.text
		}
		parsed, err := parser.ParseFile(index.fset, filepath.Join(tree, filepath.FromSlash(path)), source, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		file := &sourceFile{path: path, dir: dirOf(path), ast: parsed}
		index.files = append(index.files, file)
		index.packageNames[file.dir] = parsed.Name.Name
		index.addValues(file)
	}
	for _, file := range index.files {
		file.imports = index.importNames(file.ast)
	}
	return index, nil
}

// addValues records every constant and package-level variable a file
// declares with a value. One with no value of its own (an iota repetition, a
// variable declared by type) is not a string or a list a code can be read
// from, so it is not recorded.
func (index *treeIndex) addValues(file *sourceFile) {
	for _, decl := range file.ast.Decls {
		gen, isGen := decl.(*ast.GenDecl)
		if !isGen {
			continue
		}
		values := index.constants
		if gen.Tok == token.VAR {
			values = index.variables
		} else if gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, isValue := spec.(*ast.ValueSpec)
			if !isValue {
				continue
			}
			for i, name := range value.Names {
				if i >= len(value.Values) {
					break
				}
				if values[file.dir] == nil {
					values[file.dir] = make(map[string]constRef)
				}
				values[file.dir][name.Name] = constRef{expr: value.Values[i], file: file}
			}
		}
	}
}

// importNames maps each name a file refers to an import by to its path: the
// alias when it has one, else the package name the tree declares for that
// directory, else the path's last element (the standard library, vendor).
// A blank or dot import gives no name.
func (index *treeIndex) importNames(file *ast.File) map[string]string {
	names := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if spec.Name != nil {
			if spec.Name.Name == "_" || spec.Name.Name == "." {
				continue
			}
			names[spec.Name.Name] = path
			continue
		}
		if dir, inTree := strings.CutPrefix(path, modulePath+"/"); inTree {
			if name, known := index.packageNames[dir]; known {
				names[name] = path
				continue
			}
		}
		names[path[strings.LastIndexByte(path, '/')+1:]] = path
	}
	return names
}

// shippedPaths walks tree's parse roots for shipped Go files, sorted. A root
// the tree does not hold is skipped: a fixture tree holds one or two.
func shippedPaths(tree string) ([]string, error) {
	var paths []string
	for _, root := range parseRoots {
		err := filepath.WalkDir(filepath.Join(tree, root), func(full string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(tree, full)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if entry.IsDir() {
				if skippedDir(rel) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(rel, ".go") {
				return nil
			}
			if strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			paths = append(paths, rel)
			return nil
		})
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	slices.Sort(paths)
	return paths, nil
}

// shipped reports whether a slash-separated path is a shipped Go file under
// the parse roots, the set shippedPaths walks.
func shipped(rel string) bool {
	if !strings.HasSuffix(rel, ".go") {
		return false
	}
	if strings.HasSuffix(rel, "_test.go") {
		return false
	}
	if skippedDir(dirOf(rel)) {
		return false
	}
	for _, root := range parseRoots {
		if strings.HasPrefix(rel, root+"/") {
			return true
		}
	}
	return false
}

// skippedDir reports whether a directory holds no shipped code.
func skippedDir(rel string) bool {
	for segment := range strings.SplitSeq(rel, "/") {
		if segment == "vendor" {
			return true
		}
		if segment == "testdata" {
			return true
		}
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return underAny(rel, excludedDirs)
}

// underAny reports whether rel is one of bases or sits below one, on whole
// path components.
func underAny(rel string, bases []string) bool {
	for _, base := range bases {
		if rel == base {
			return true
		}
		if strings.HasPrefix(rel, base+"/") {
			return true
		}
	}
	return false
}

// dirOf answers the directory of a slash-separated path.
func dirOf(path string) string {
	dir, _, found := strings.CutLast(path, "/")
	if !found {
		return "."
	}
	return dir
}

// ErrThinScan names a scan that found fewer sites than the tree is known to
// hold, so it judged too little to answer for it.
var ErrThinScan = errors.New("the doctor-coverage scan found too few problem sites to judge the tree")

// checkSitesFloor refuses an inventory below floor sites. A scan that matched
// no call shape (a renamed logger method, a moved report package) would
// otherwise answer zero uncovered sites, which is what a clean tree answers.
func checkSitesFloor(inventory Inventory, floor int) error {
	if len(inventory.Sites) >= floor {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str(": found ").Int(int64(len(inventory.Sites))).Str(", at least ").Int(int64(floor)).
		Str(" expected. A call shape in internal/le/arch/doctorcoverage/sites.go no longer matches the logger or report bus it names; update the shape to the current call")
	return errors.New(ErrThinScan.Error() + tb.String())
}
