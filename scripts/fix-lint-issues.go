// Design: (none — build tool)
//
// fix-lint-issues.go - Automated lint fixes using AST
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

var (
	dryRun  = flag.Bool("dry-run", false, "Show changes without applying")
	verbose = flag.Bool("v", false, "Verbose output")
)

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Println("Usage: go run scripts/fix-lint-issues.go <files or dirs...>")
		os.Exit(1)
	}

	for _, path := range flag.Args() {
		if err := processPath(path); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", path, err)
		}
	}
}

func processPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
				return processFile(p)
			}
			return nil
		})
	}

	return processFile(path)
}

func processFile(filename string) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	changed := false

	// Fix rangeValCopy - convert to index iteration
	ast.Inspect(node, func(n ast.Node) bool {
		rangeStmt, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}

		// Check if value is used (not blank identifier)
		if rangeStmt.Value != nil {
			if ident, ok := rangeStmt.Value.(*ast.Ident); ok && ident.Name != "_" {
				// Check if X is a large struct type
				// For now, convert any non-blank value iteration
				if *verbose {
					fmt.Printf("%s: Converting range-by-value to range-by-index\n", filename)
				}

				// Change to index-only iteration
				rangeStmt.Value = nil
				if rangeStmt.Key == nil {
					rangeStmt.Key = ast.NewIdent("i")
				} else if keyIdent, keyOk := rangeStmt.Key.(*ast.Ident); keyOk && keyIdent.Name == "_" {
					rangeStmt.Key = ast.NewIdent("i")
				}
				changed = true
			}
		}

		return true
	})

	// Fix shadow - rename variables in inner scopes
	// This requires scope analysis - skip for now

	if changed && !*dryRun {
		var buf bytes.Buffer
		if err := format.Node(&buf, fset, node); err != nil {
			return fmt.Errorf("format error: %w", err)
		}

		//nolint:gosec // G306: 0o644 is appropriate for Go source files, which are intended to be world-readable
		if err := os.WriteFile(filename, buf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("write error: %w", err)
		}

		fmt.Printf("✅ Fixed: %s\n", filename)
	} else if changed {
		fmt.Printf("🔍 Would fix: %s\n", filename)
	}

	return nil
}
