// Design: docs/contributing/dependency-approval.md -- the ledger of approved third-party imports
// Related: approve.go -- the owner-only command that writes a row
// Related: check.go -- the whole-tree comparison of the tree with the ledger
// Related: ../commit/dependencies.go -- the commit gate that judges what a commit adds
//
// Package deps combines ordinary-module reuse with exact package/importer
// approvals. Existing modules may be used in full, except circl, whose uses
// remain individually approved (ai/META-DECISIONS.md, "Code and dependencies").
package deps

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// LedgerRel is the ledger's path from the checkout root. It sits beside
// go.mod, which declares modules; the ledger records use-specific approvals.
const LedgerRel = "dependency-ledger.txt"

// ApprovalsRel is the untracked record `deps approve` appends each approved
// row to. The commit gate admits a ledger row a commit adds only when this
// record holds the same row, so a row typed into the ledger by hand is refused.
const ApprovalsRel = "tmp/dependency-approvals.txt"

// ledgerHeader opens the ledger. Every line starting with '#' is a comment.
const ledgerHeader = "# Dependency ledger: exact third-party package/importer approvals.\n" +
	"# Ordinary modules already declared in go.mod may be used in full.\n" +
	"# Circl remains restricted to its approved package/importer uses.\n" +
	"# Three tab-separated fields: package, importer directory, reason.\n" +
	"# A new row is written by\n" +
	"# ./le deps approve, from Thomas's terminal, never by hand.\n" +
	"# Page: docs/contributing/dependency-approval.md\n"

// Row is one approval: Package may be imported by Go files in Importer.
type Row struct {
	Package  string `json:"package"`
	Importer string `json:"importer"`
	Reason   string `json:"reason"`
}

// key is the identity of a row; the reason does not change what is allowed.
type key struct{ pkg, importer string }

// Ledger is a parsed ledger. Build one with Parse; the zero value allows
// nothing, which is the answer an empty ledger gives.
type Ledger struct {
	rows []Row
}

// Parse reads ledger text. A malformed row is an error naming its line,
// because a row the reader skipped would silently withdraw an approval.
func Parse(text string) (Ledger, error) {
	var ledger Ledger
	seen := make(map[key]int)
	for index, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		row, err := parseRow(line)
		if err != nil {
			return Ledger{}, errors.New(LedgerRel + " line " + strconv.Itoa(index+1) + ": " + err.Error())
		}
		identity := key{row.Package, row.Importer}
		if first, dup := seen[identity]; dup {
			return Ledger{}, errors.New(LedgerRel + " line " + strconv.Itoa(index+1) + ": repeats the row of line " + strconv.Itoa(first))
		}
		seen[identity] = index + 1
		ledger.rows = append(ledger.rows, row)
	}
	return ledger, nil
}

// parseRow reads one tab-separated row and refuses an incomplete one.
func parseRow(line string) (Row, error) {
	fields := strings.Split(line, "\t")
	if len(fields) != 3 {
		return Row{}, errors.New("want three tab-separated fields (package, importer, reason), got " + strconv.Itoa(len(fields)))
	}
	row := Row{Package: fields[0], Importer: fields[1], Reason: strings.TrimSpace(fields[2])}
	if err := ValidRow(row); err != nil {
		return Row{}, err
	}
	return row, nil
}

// ValidRow refuses a row whose fields cannot be judged: an empty or spaced
// package, an importer that is not a clean relative directory, an empty reason.
func ValidRow(row Row) error {
	if row.Package == "" {
		return errors.New("the package is empty")
	}
	if strings.ContainsAny(row.Package, " \t\r\n") {
		return errors.New("the package " + strconv.Quote(row.Package) + " holds white space")
	}
	if err := validImporter(row.Importer); err != nil {
		return err
	}
	if row.Reason == "" {
		return errors.New("the reason is empty")
	}
	if strings.ContainsAny(row.Reason, "\t\r\n") {
		return errors.New("the reason must be one line with no tab")
	}
	return nil
}

// validImporter accepts a checkout-relative directory in slash form, "." for
// the root, and refuses anything that could name a place outside the checkout.
func validImporter(importer string) error {
	if importer == "" {
		return errors.New("the importer is empty")
	}
	if path.IsAbs(importer) {
		return errors.New("the importer " + strconv.Quote(importer) + " must be relative to the checkout")
	}
	if path.Clean(importer) != importer {
		return errors.New("the importer " + strconv.Quote(importer) + " is not a clean directory (want " + strconv.Quote(path.Clean(importer)) + ")")
	}
	if importer == ".." {
		return errors.New("the importer " + strconv.Quote(importer) + " leaves the checkout")
	}
	if strings.HasPrefix(importer, "../") {
		return errors.New("the importer " + strconv.Quote(importer) + " leaves the checkout")
	}
	if strings.ContainsAny(importer, " \t\r\n\\") {
		return errors.New("the importer " + strconv.Quote(importer) + " holds white space or a backslash")
	}
	return nil
}

// Allows answers whether Go files in importer may import pkg.
func (l Ledger) Allows(pkg, importer string) bool {
	for _, row := range l.rows {
		if row.Package == pkg && row.Importer == importer {
			return true
		}
	}
	return false
}

// Rows answers a copy of the ledger's rows in file order.
func (l Ledger) Rows() []Row { return slices.Clone(l.rows) }

// With answers the ledger holding row, replacing the reason of a row with the
// same package and importer, sorted by package then importer.
func (l Ledger) With(row Row) Ledger {
	rows := make([]Row, 0, len(l.rows)+1)
	for _, existing := range l.rows {
		if existing.Package == row.Package && existing.Importer == row.Importer {
			continue
		}
		rows = append(rows, existing)
	}
	rows = append(rows, row)
	slices.SortFunc(rows, compareRows)
	return Ledger{rows: rows}
}

// compareRows orders rows by package, then importer.
func compareRows(a, b Row) int {
	if order := strings.Compare(a.Package, b.Package); order != 0 {
		return order
	}
	return strings.Compare(a.Importer, b.Importer)
}

// Render answers the ledger file's text: the header, then one line per row.
func (l Ledger) Render() string {
	var text textbuf.Buffer
	text.Str(ledgerHeader)
	for _, row := range l.rows {
		text.Str(RowLine(row)).Byte('\n')
	}
	return text.String()
}

// RowLine renders one row as the ledger and the approval record spell it.
func RowLine(row Row) string {
	return row.Package + "\t" + row.Importer + "\t" + row.Reason
}

// ModulePath reads the module path from go.mod text. The Ze module is first
// party, so an import under it is never judged.
func ModulePath(goMod string) (string, error) {
	for line := range strings.SplitSeq(goMod, "\n") {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), "module ")
		if !found {
			continue
		}
		module := strings.Trim(strings.TrimSpace(rest), `"`)
		if module != "" {
			return module, nil
		}
	}
	return "", errors.New("go.mod declares no module line")
}

// ThirdParty answers whether importPath is a third-party package: its first
// element holds a dot, which no standard-library path does, and it is not the
// module itself or under it.
func ThirdParty(module, importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	if !strings.Contains(first, ".") {
		return false
	}
	if importPath == module {
		return false
	}
	return !strings.HasPrefix(importPath, module+"/")
}

// Imports answers the import paths of one Go source, whatever its build
// constraints: a file built only under one tag still links its imports.
func Imports(name string, src []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, errors.New(name + ": import " + spec.Path.Value + " is not a quoted path")
		}
		paths = append(paths, value)
	}
	return paths, nil
}

// Added answers, sorted and once each, the third-party paths after holds that
// before does not. A file that keeps or drops an import adds nothing to judge.
func Added(module string, before, after []string) []string {
	var added []string
	for _, importPath := range after {
		if !ThirdParty(module, importPath) {
			continue
		}
		if slices.Contains(before, importPath) {
			continue
		}
		if slices.Contains(added, importPath) {
			continue
		}
		added = append(added, importPath)
	}
	slices.Sort(added)
	return added
}

// Importer answers the ledger's importer for a checkout-relative Go file: its
// directory, "." for a file at the root.
func Importer(rel string) string { return path.Dir(rel) }

// Judged answers whether a checkout-relative path is Go source of the Ze
// module. vendor/ is the dependencies themselves, testdata/ is never built
// by the go command, and a directory holding its own go.mod (an example, a
// gokrazy build directory) is another module whose imports Ze does not link.
// A go.mod that cannot be read answers an error rather than a guess.
func Judged(root, rel string) (bool, error) {
	return JudgedWith(rel, func(moduleFile string) (bool, error) {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(moduleFile)))
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	})
}

// JudgedWith classifies source using the caller's nested-module boundaries.
// A commit MUST supply existence from its prospective content; Judged uses
// the current filesystem for a whole-tree check.
func JudgedWith(rel string, moduleExists func(string) (bool, error)) (bool, error) {
	if !strings.HasSuffix(rel, ".go") {
		return false, nil
	}
	for element := range strings.SplitSeq(path.Dir(rel), "/") {
		if element == "vendor" {
			return false, nil
		}
		if element == "testdata" {
			return false, nil
		}
	}
	for dir := path.Dir(rel); dir != "." && dir != "/"; dir = path.Dir(dir) {
		present, err := moduleExists(path.Join(dir, "go.mod"))
		if err != nil {
			return false, err
		}
		if present {
			return false, nil
		}
	}
	return true, nil
}

// VendorModulesRel is the go command's list of vendored modules.
const VendorModulesRel = "vendor/modules.txt"

// ModuleOf answers the vendored module that provides pkg, read from the text
// of vendor/modules.txt: the longest `# <module> <version>` path pkg is in.
// An empty answer means no vendored module provides it, and the refusal says
// so rather than naming a guess.
func ModuleOf(vendorModules, pkg string) string {
	module := ""
	for line := range strings.SplitSeq(vendorModules, "\n") {
		rest, found := strings.CutPrefix(line, "# ")
		if !found {
			continue
		}
		candidate, _, _ := strings.Cut(rest, " ")
		if pkg != candidate && !strings.HasPrefix(pkg, candidate+"/") {
			continue
		}
		if len(candidate) > len(module) {
			module = candidate
		}
	}
	return module
}

// ApproveCommand renders the command Thomas runs to approve one use.
func ApproveCommand(pkg, importer string) string {
	return "./le deps approve package " + pkg + " importer " + importer + " reason \"<why Ze needs it here>\""
}
