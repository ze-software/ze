//go:build ignore

// Design: (none — build tool)
//
// go_extract: Move Go symbols (functions, types, vars, consts) between files.
// Preserves exact formatting by operating on source lines, not AST rewriting.
// Runs goimports on both files after extraction.
//
// Usage: go run scripts/dev/go_extract.go <source.go> <dest.go> <symbol1> [symbol2 ...]

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func die(msg string, args ...any) {
	os.Stderr.WriteString(fmt.Sprintf(msg+"\n", args...))
	os.Exit(1)
}

func main() {
	if len(os.Args) < 4 {
		die("usage: go run scripts/go-extract.go <source.go> <dest.go> <symbol1> [symbol2 ...]")
	}

	srcPath := os.Args[1]
	dstPath := os.Args[2]
	symbols := make(map[string]bool, len(os.Args)-3)
	for _, s := range os.Args[3:] {
		symbols[s] = true
	}

	// Parse source file AST.
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, srcPath, nil, parser.ParseComments)
	if err != nil {
		die("parse %s: %v", srcPath, err)
	}

	// Read source lines.
	srcBytes, err := os.ReadFile(srcPath)
	if err != nil {
		die("read %s: %v", srcPath, err)
	}
	lines := strings.Split(string(srcBytes), "\n")

	// Find line ranges for matching declarations (1-indexed, inclusive).
	type lineRange struct{ start, end int }
	var extractRanges []lineRange

	for _, decl := range file.Decls {
		name, ok := declSymbol(decl, symbols)
		if !ok {
			continue
		}
		_ = name

		start := fset.Position(decl.Pos()).Line
		end := fset.Position(decl.End()).Line

		// Include doc comment if present.
		if doc := declDoc(decl); doc != nil {
			docLine := fset.Position(doc.Pos()).Line
			if docLine < start {
				start = docLine
			}
		}

		extractRanges = append(extractRanges, lineRange{start, end})
	}

	if len(extractRanges) == 0 {
		die("no matching symbols found in %s", srcPath)
	}

	// Sort by start line.
	sort.Slice(extractRanges, func(i, j int) bool {
		return extractRanges[i].start < extractRanges[j].start
	})

	// Build extracted and remaining line sets.
	// Also include blank lines between adjacent extracted ranges (gap lines).
	extractSet := make(map[int]bool)
	for _, r := range extractRanges {
		for i := r.start; i <= r.end; i++ {
			extractSet[i] = true
		}
	}
	// Include blank-only gap lines between consecutive extracted ranges.
	for i := 0; i+1 < len(extractRanges); i++ {
		gapStart := extractRanges[i].end + 1
		gapEnd := extractRanges[i+1].start - 1
		allBlank := true
		for ln := gapStart; ln <= gapEnd; ln++ {
			if ln-1 < len(lines) && strings.TrimSpace(lines[ln-1]) != "" {
				allBlank = false
				break
			}
		}
		if allBlank {
			for ln := gapStart; ln <= gapEnd; ln++ {
				extractSet[ln] = true
			}
		}
	}

	var extractedLines []string
	var remainingLines []string

	for i, line := range lines {
		lineNum := i + 1 // 1-indexed
		if extractSet[lineNum] {
			extractedLines = append(extractedLines, line)
		} else {
			remainingLines = append(remainingLines, line)
		}
	}

	// Collapse runs of >2 blank lines (left behind after extraction).
	remainingLines = collapseBlankLines(remainingLines)
	extractedLines = collapseBlankLines(extractedLines)

	// Write remaining back to source.
	if err := os.WriteFile(srcPath, []byte(strings.Join(remainingLines, "\n")), 0o644); err != nil {
		die("write %s: %v", srcPath, err)
	}

	// Build destination content.
	var dstContent string
	if existing, err := os.ReadFile(dstPath); err == nil {
		// Append to existing file.
		dstContent = string(existing)
		if !strings.HasSuffix(dstContent, "\n") {
			dstContent += "\n"
		}
		dstContent += "\n" + strings.Join(extractedLines, "\n")
	} else {
		// New file: add package header.
		dstContent = "package " + file.Name.Name + "\n\n" + strings.Join(extractedLines, "\n")
	}

	if err := os.WriteFile(dstPath, []byte(dstContent), 0o644); err != nil {
		die("write %s: %v", dstPath, err)
	}

	// Run goimports on both files.
	for _, path := range []string{srcPath, dstPath} {
		cmd := exec.Command("goimports", "-w", path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			die("goimports %s: %v", path, err)
		}
	}

	fmt.Printf("extracted %d symbols (%d lines) from %s → %s\n",
		len(extractRanges), len(extractedLines), srcPath, dstPath)
}

// declSymbol returns the symbol name if decl matches any requested symbol.
// For functions/methods: matches the function name.
// For GenDecl (type/var/const): matches any spec name in the group.
func declSymbol(decl ast.Decl, symbols map[string]bool) (string, bool) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if symbols[d.Name.Name] {
			return d.Name.Name, true
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if symbols[s.Name.Name] {
					return s.Name.Name, true
				}
			case *ast.ValueSpec:
				for _, n := range s.Names {
					if symbols[n.Name] {
						return n.Name, true
					}
				}
			}
		}
	}
	return "", false
}

// declDoc returns the doc comment group for a declaration.
func declDoc(decl ast.Decl) *ast.CommentGroup {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return d.Doc
	case *ast.GenDecl:
		return d.Doc
	}
	return nil
}

// collapseBlankLines replaces runs of 3+ blank lines with 2.
func collapseBlankLines(lines []string) []string {
	var result []string
	blanks := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blanks++
			if blanks <= 2 {
				result = append(result, line)
			}
		} else {
			blanks = 0
			result = append(result, line)
		}
	}
	return result
}
