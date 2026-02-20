//go:build ignore

// Design: (none — build tool)
//
// analyze-writeto.go - Static analysis for WriteTo buffer safety
//
// Analyzes Go source files to find WriteTo functions and verify they have
// matching Len() methods. Reports potential buffer overflow risks.
//
// Usage: go run scripts/analyze-writeto.go [path...]

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type TypeInfo struct {
	Name         string
	File         string
	HasLen       bool
	HasWriteTo   bool
	WriteToLine  int
	BufferWrites []BufferWrite
}

type BufferWrite struct {
	Line     int
	Pattern  string
	HasCheck bool
}

type Report struct {
	TypesWithWriteTo []TypeInfo
	MissingLen       []TypeInfo
	UncheckedWrites  []BufferWrite
}

func main() {
	paths := os.Args[1:]
	if len(paths) == 0 {
		paths = []string{"pkg/"}
	}

	report := &Report{}
	types := make(map[string]*TypeInfo)

	for _, root := range paths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			analyzeFile(path, types, report)
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error walking path %s: %v\n", root, err)
		}
	}

	// Analyze collected types
	for _, ti := range types {
		if ti.HasWriteTo {
			report.TypesWithWriteTo = append(report.TypesWithWriteTo, *ti)
			if !ti.HasLen {
				report.MissingLen = append(report.MissingLen, *ti)
			}
		}
	}

	// Sort for consistent output
	sort.Slice(report.TypesWithWriteTo, func(i, j int) bool {
		return report.TypesWithWriteTo[i].Name < report.TypesWithWriteTo[j].Name
	})

	printReport(report)
}

func analyzeFile(path string, types map[string]*TypeInfo, report *Report) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", path, err)
		return
	}

	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			return true
		}

		// Get receiver type name
		typeName := getReceiverTypeName(fn.Recv.List[0].Type)
		if typeName == "" {
			return true
		}

		key := path + ":" + typeName
		ti, exists := types[key]
		if !exists {
			ti = &TypeInfo{Name: typeName, File: path}
			types[key] = ti
		}

		switch fn.Name.Name {
		case "Len", "LenWithASN4", "LenWithContext":
			ti.HasLen = true
		case "WriteTo":
			ti.HasWriteTo = true
			ti.WriteToLine = fset.Position(fn.Pos()).Line
			analyzeWriteToBody(fn.Body, fset, ti, report)
		}

		return true
	})
}

func getReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

func analyzeWriteToBody(body *ast.BlockStmt, fset *token.FileSet, ti *TypeInfo, report *Report) {
	if body == nil {
		return
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.IndexExpr:
			// Look for buf[...] patterns
			if ident, ok := expr.X.(*ast.Ident); ok && ident.Name == "buf" {
				line := fset.Position(expr.Pos()).Line
				bw := BufferWrite{
					Line:    line,
					Pattern: fmt.Sprintf("%s buf[...]", ti.Name),
				}
				ti.BufferWrites = append(ti.BufferWrites, bw)
			}
		case *ast.SliceExpr:
			// Look for buf[off:...] patterns
			if ident, ok := expr.X.(*ast.Ident); ok && ident.Name == "buf" {
				line := fset.Position(expr.Pos()).Line
				bw := BufferWrite{
					Line:    line,
					Pattern: fmt.Sprintf("%s buf[off:...]", ti.Name),
				}
				ti.BufferWrites = append(ti.BufferWrites, bw)
			}
		}
		return true
	})
}

func printReport(report *Report) {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           WriteTo Buffer Safety Analysis Report              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Types with WriteTo
	fmt.Printf("📊 Types with WriteTo(): %d\n", len(report.TypesWithWriteTo))
	fmt.Println("─────────────────────────────────────────────────────────────────")
	for _, ti := range report.TypesWithWriteTo {
		lenStatus := "✅"
		if !ti.HasLen {
			lenStatus = "❌"
		}
		fmt.Printf("  %s %-25s %s:%d\n", lenStatus, ti.Name, ti.File, ti.WriteToLine)
	}
	fmt.Println()

	// Missing Len()
	if len(report.MissingLen) > 0 {
		fmt.Printf("🔴 HIGH RISK - Missing Len() method: %d\n", len(report.MissingLen))
		fmt.Println("─────────────────────────────────────────────────────────────────")
		for _, ti := range report.MissingLen {
			fmt.Printf("  ❌ %s - %s:%d\n", ti.Name, ti.File, ti.WriteToLine)
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("Summary:")
	fmt.Printf("  Total WriteTo types:     %d\n", len(report.TypesWithWriteTo))
	fmt.Printf("  Missing Len():           %d\n", len(report.MissingLen))
	fmt.Println()
	if len(report.MissingLen) == 0 {
		fmt.Println("✅ All WriteTo types have matching Len() methods")
	}
}
