// Design: docs/architecture/core-design.md -- moving Go declarations between files
//
// Package goextract moves named top-level declarations from one Go file to
// another. It works on SOURCE LINES rather than by re-printing an AST, so what
// lands in the destination is byte for byte what left the source: the same
// comments, the same alignment, the same blank lines. An AST rewrite would
// reformat every declaration it touched, and a file split would then arrive as
// a diff nobody can review.
//
// The AST is still parsed, because that is the only reliable way to find where
// a declaration starts and ends. It is read for POSITIONS and never printed.
//
// A move is computed in full before anything is written (PlanMove), and the
// destination is written before the source (Move). Both halves exist for one
// reason: this tool DELETES text from a working file, so a failure part way
// through must never be able to leave a declaration in neither file.
package goextract

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// filePerm is what a Go source file this tool creates is written with. It is
// the mode a checked-out source file already carries.
const filePerm fs.FileMode = 0o644

// Request is one move: the file to take declarations out of, the file to put
// them in, and the declarations to move.
type Request struct {
	// Source is the file the declarations leave. It is read, parsed and
	// rewritten without them.
	Source string
	// Dest is the file they arrive in. It is appended to when it exists and
	// created with a package clause when it does not.
	Dest string
	// Symbols names what moves: a function, a method, a type, a var or a const.
	// A declaration group moves whole when any name in it is named here, which
	// is what keeps a `const (...)` block from being split down the middle.
	Symbols []string
}

// Format runs an import fixer over one file, in place.
//
// It is a parameter of Move rather than a call inside it, so a test drives a
// move with no goimports on PATH, and so a formatter that fails is a value the
// caller can report rather than a process that exits.
type Format func(ctx context.Context, path string) error

// Goimports is the formatter this tool uses: the `goimports -w` a developer
// runs by hand after a split, which adds the imports the moved code needs to
// the destination and removes the ones the source no longer uses.
//
// goimports cannot resolve an ALIASED import, so a moved declaration that used
// one still needs its alias added by hand.
func Goimports(ctx context.Context, path string) error {
	// G204: the executable is the fixed name goimports, resolved on PATH, and
	// the one variable argument is the file the operator asked to split.
	cmd := exec.CommandContext(ctx, "goimports", "-w", path) //nolint:gosec // fixed executable, operator-named file
	var errOut bytes.Buffer
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(errOut.String())
		if detail == "" {
			return fmt.Errorf("goimports %s: %w", path, err)
		}
		return fmt.Errorf("goimports %s: %w: %s", path, err, detail)
	}
	return nil
}

// Plan is what a move WOULD write: the two file bodies and the record of what
// moved. Nothing on disk changes.
//
// It is separate from the write because every way this tool can refuse -- an
// unparsable source, an unreadable destination, a symbol that is not there --
// is knowable before the first byte is written. A tool that discovers a refusal
// half way through a two-file write has already destroyed something.
type Plan struct {
	// Report is what the move will have done.
	Report Report
	// Source is the whole new body of the source file.
	Source string
	// Dest is the whole new body of the destination file.
	Dest string
}

// lineRange is one declaration's extent in the source, 1-indexed and inclusive,
// with the name that matched it.
type lineRange struct {
	symbol string
	start  int
	end    int
}

// PlanMove reads the two files and works out the move, writing nothing.
func PlanMove(req Request) (Plan, error) {
	if len(req.Symbols) == 0 {
		return Plan{}, errors.New("name at least one symbol to move")
	}
	// One file cannot be both halves of a move. The destination is written
	// first so that a failure cannot lose a declaration, and if the two paths
	// name one file that ordering writes the extracted text and then overwrites
	// it with the remainder.
	if filepath.Clean(req.Source) == filepath.Clean(req.Dest) {
		return Plan{}, fmt.Errorf("source and dest name one file: %s", req.Source)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, req.Source, nil, parser.ParseComments)
	if err != nil {
		return Plan{}, fmt.Errorf("parse %s: %w", req.Source, err)
	}

	srcBytes, err := os.ReadFile(req.Source)
	if err != nil {
		return Plan{}, fmt.Errorf("read %s: %w", req.Source, err)
	}
	lines := strings.Split(string(srcBytes), "\n")

	ranges, err := match(fset, file, req.Source, req.Symbols)
	if err != nil {
		return Plan{}, err
	}

	extracted, remaining := split(lines, ranges)

	dest, err := destBody(req.Dest, file.Name.Name, extracted)
	if err != nil {
		return Plan{}, err
	}

	moved := make([]Moved, 0, len(ranges))
	for _, r := range ranges {
		moved = append(moved, Moved{Symbol: r.symbol, FirstLine: r.start, LastLine: r.end})
	}

	return Plan{
		Report: Report{
			Source:  req.Source,
			Dest:    req.Dest,
			Lines:   len(extracted),
			Symbols: moved,
		},
		Source: strings.Join(remaining, "\n"),
		Dest:   dest,
	}, nil
}

// Move plans the move and carries it out.
//
// The DESTINATION is written first, and that order is the whole safety of this
// tool. A failure before it leaves both files as they were. A failure between
// the two writes leaves the declarations in both files, which the compiler
// refuses loudly on the next build and a developer repairs by deleting one
// copy. The other order -- the source first -- answers a failed destination
// write by having already deleted the only copy there was.
//
// format runs after both writes, on the source and then the destination. A
// formatter that fails is reported with the report of what has already moved,
// because the files are written by then and a caller told only "it failed"
// would go looking for work that is on disk.
func Move(ctx context.Context, req Request, format Format) (Report, error) {
	plan, err := PlanMove(req)
	if err != nil {
		return Report{}, err
	}

	if err := os.WriteFile(req.Dest, []byte(plan.Dest), filePerm); err != nil {
		return Report{}, fmt.Errorf("write %s: %w", req.Dest, err)
	}
	if err := os.WriteFile(req.Source, []byte(plan.Source), filePerm); err != nil {
		return Report{}, fmt.Errorf("write %s: %w", req.Source, err)
	}

	if format != nil {
		for _, path := range []string{req.Source, req.Dest} {
			if err := format(ctx, path); err != nil {
				return plan.Report, err
			}
		}
	}

	return plan.Report, nil
}

// match finds the line extent of every declaration the request names, in source
// order.
//
// A symbol nobody declares is an ERROR that names it, and nothing is written.
// The script this replaced refused only when NO symbol matched, so a typo in
// one name of three moved the other two and reported success: the developer was
// then left with a half-split file and a message saying it had worked.
func match(fset *token.FileSet, file *ast.File, path string, symbols []string) ([]lineRange, error) {
	wanted := make(map[string]bool, len(symbols))
	for _, symbol := range symbols {
		wanted[symbol] = true
	}

	var ranges []lineRange
	for _, decl := range file.Decls {
		name, ok := declSymbol(decl, wanted)
		if !ok {
			continue
		}

		start := fset.Position(decl.Pos()).Line
		end := fset.Position(decl.End()).Line

		// The doc comment belongs to the declaration, so it travels with it.
		if doc := declDoc(decl); doc != nil {
			if docLine := fset.Position(doc.Pos()).Line; docLine < start {
				start = docLine
			}
		}

		ranges = append(ranges, lineRange{symbol: name, start: start, end: end})
		delete(wanted, name)
	}

	if len(wanted) > 0 {
		missing := make([]string, 0, len(wanted))
		for symbol := range wanted {
			missing = append(missing, symbol)
		}
		// Sorted, so two runs over one file report one thing.
		slices.Sort(missing)
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("not declared in ").Str(path).Str(": ").Join(missing, ", ").String())
	}

	slices.SortFunc(ranges, func(a, b lineRange) int { return cmp.Compare(a.start, b.start) })
	return ranges, nil
}

// split divides the source lines into the ones that move and the ones that
// stay.
//
// Blank lines BETWEEN two adjacent extracted declarations move with them, so
// the destination keeps the spacing the source had and the source does not keep
// a hole where the pair used to be. A run of three or more blank lines is
// collapsed to two on both sides, which is what an extraction from the middle
// of a file otherwise leaves behind.
func split(lines []string, ranges []lineRange) (extracted, remaining []string) {
	moving := make(map[int]bool)
	for _, r := range ranges {
		for line := r.start; line <= r.end; line++ {
			moving[line] = true
		}
	}

	for i := 0; i+1 < len(ranges); i++ {
		gapStart := ranges[i].end + 1
		gapEnd := ranges[i+1].start - 1
		if !allBlank(lines, gapStart, gapEnd) {
			continue
		}
		for line := gapStart; line <= gapEnd; line++ {
			moving[line] = true
		}
	}

	for i, line := range lines {
		if moving[i+1] {
			extracted = append(extracted, line)
			continue
		}
		remaining = append(remaining, line)
	}

	return collapseBlankLines(extracted), collapseBlankLines(remaining)
}

// allBlank reports whether every line from start to end, 1-indexed and
// inclusive, is empty or whitespace. An empty span is blank: two adjacent
// declarations have no gap to judge.
func allBlank(lines []string, start, end int) bool {
	for line := start; line <= end; line++ {
		if line-1 < len(lines) && strings.TrimSpace(lines[line-1]) != "" {
			return false
		}
	}
	return true
}

// destBody answers the whole new body of the destination file.
//
// A destination that does not exist is created with the source file's package
// clause. A destination that exists is appended to, with one blank line between
// what was there and what arrives.
//
// Any OTHER read failure refuses the move. The script this replaced read every
// failure as "the file is not there" and then wrote a fresh file over it, so a
// destination it could not read was a destination it destroyed.
func destBody(path, pkg string, extracted []string) (string, error) {
	existing, err := os.ReadFile(path) //nolint:gosec // the destination the operator named
	switch {
	case errors.Is(err, fs.ErrNotExist):
		var tb textbuf.Buffer
		return tb.Str("package ").Str(pkg).Str("\n\n").Join(extracted, "\n").String(), nil
	case err != nil:
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	var tb textbuf.Buffer
	tb.Str(string(existing))
	if !bytes.HasSuffix(existing, []byte("\n")) {
		tb.Byte('\n')
	}
	return tb.Byte('\n').Join(extracted, "\n").String(), nil
}

// declSymbol answers the name a declaration matched, if it matched.
//
// A function or a method matches on its own name. A type, var or const group
// matches when ANY name it declares is wanted, and the whole group then moves:
// half a `const (...)` block is not a thing a file can hold.
func declSymbol(decl ast.Decl, wanted map[string]bool) (string, bool) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if wanted[d.Name.Name] {
			return d.Name.Name, true
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if wanted[s.Name.Name] {
					return s.Name.Name, true
				}
			case *ast.ValueSpec:
				for _, name := range s.Names {
					if wanted[name.Name] {
						return name.Name, true
					}
				}
			}
		}
	}
	return "", false
}

// declDoc answers a declaration's doc comment group, or nil when it has none.
func declDoc(decl ast.Decl) *ast.CommentGroup {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return d.Doc
	case *ast.GenDecl:
		return d.Doc
	}
	return nil
}

// collapseBlankLines replaces a run of three or more blank lines with two.
func collapseBlankLines(lines []string) []string {
	var result []string
	blanks := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blanks++
			if blanks <= 2 {
				result = append(result, line)
			}
			continue
		}
		blanks = 0
		result = append(result, line)
	}
	return result
}
