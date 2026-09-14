// Design: docs/features/ai-first.md -- doctor check registration
//
// Overview: enumeration.go -- the area this finding is answered by
//
// doctorcheck.go answers the second shape of the same defect. The doctor
// component holds a check registry, and the runner reaches about forty of its
// checks by writing each name out instead. A check that arrives has to be added
// to that runner by hand, and a check that leaves has to be found there, which
// is what registration exists to end (ai/rules/principles.md).
//
// The two anchors are named rather than derived, and each one fails the gate
// closed when it stops resolving: the package directory the checks live in, and
// the dispatcher that runs the REGISTERED ones. The runner itself is derived as
// the function that calls that dispatcher, so renaming the runner changes
// nothing here.

package enumeration

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

const (
	// doctorDir is where the doctor component's check functions live.
	doctorDir = "internal/component/doctor"
	// doctorDispatcher runs the checks the registry holds. The function that
	// calls it is the runner, so the runner needs no name here.
	doctorDispatcher = "runDoctorChecks"
	// doctorCheckPrefix is the doctor component's own name for a check. It is
	// half the test: a function that resolves storage also answers diagnostics,
	// and it is not a check.
	doctorCheckPrefix = "check"
	// doctorDiagnosticType is the result every check answers.
	doctorDiagnosticType = "diagnostic.Diagnostic"
	// doctorCheckFloor is the least check functions checkDefinitions must
	// resolve before the gate believes its two signs still match the component.
	// The component declared 15 distinct check names on 2026-09-15 (18
	// declarations, three of them twice for linux and !linux), so the floor
	// fires on signs that resolve nothing rather than on a component that lost
	// a check. A fixture test passes 0, as a fixture walk passes 0 to Check.
	doctorCheckFloor = 8
)

// ErrNoDoctorRunner names a doctor package this gate could not read: the
// directory is absent, or no function calls the phase dispatcher.
//
// It is an error rather than an empty answer because the two are the same
// silence otherwise. A gate that cannot find the runner reports no hand-called
// check, which is exactly what a fixed tree reports.
var ErrNoDoctorRunner = errors.New("the doctor check runner was not found")

// ErrFewDoctorChecks names a doctor package in which the two check signs
// resolved fewer functions than the floor. It is an error for the reason
// ErrNoDoctorRunner is: a renamed result type resolves zero checks, and zero
// checks are zero hand-called checks, which is what a repaired component
// answers.
var ErrFewDoctorChecks = errors.New("the doctor package declares fewer check functions than the floor")

// handCalledDoctorChecks answers every check function in the doctor component
// that the runner reaches by writing its name out, and that no registration
// names. checkFloor is the least check definitions the package must resolve:
// the action passes doctorCheckFloor and a test over a fixture passes 0.
func handCalledDoctorChecks(tree string, checkFloor int) (Findings, error) {
	dir := filepath.Join(tree, filepath.FromSlash(doctorDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %w", ErrNoDoctorRunner, doctorDir, err)
	}

	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, fmt.Errorf("parse %s/%s: %w", doctorDir, name, parseErr)
		}
		files = append(files, file)
	}

	definitions := checkDefinitions(fset, files)
	if len(definitions) < checkFloor {
		return nil, fmt.Errorf("%w: %d in %s, below the floor of %d: the signs %q and %q resolve no check",
			ErrFewDoctorChecks, len(definitions), doctorDir, checkFloor, doctorCheckPrefix, doctorDiagnosticType)
	}
	registered := registeredChecks(files)
	runner, called, err := handCalls(files, definitions)
	if err != nil {
		return nil, err
	}

	found := make(Findings, 0, len(called))
	for _, name := range called {
		if registered[name] {
			continue
		}
		site := definitions[name]
		found = append(found, Finding{
			Kind:   KindDoctorCheck,
			File:   site.file,
			Line:   site.line,
			Symbol: name,
			Detail: "reached only by a hand-written call in " + runner + ": register it, so a check that arrives needs no edit to that runner",
		})
	}
	found.sort()
	return found, nil
}

// definition is where one check function is declared.
type definition struct {
	file string
	line int
}

// checkDefinitions answers every function that is a doctor check by both of the
// component's own signs: the name it goes by, and the result it answers.
func checkDefinitions(fset *token.FileSet, files []*ast.File) map[string]definition {
	defined := map[string]definition{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if !strings.HasPrefix(fn.Name.Name, doctorCheckPrefix) {
				continue
			}
			if !answersDiagnostics(fn) {
				continue
			}
			position := fset.Position(fn.Pos())
			defined[fn.Name.Name] = definition{
				file: doctorDir + "/" + filepath.Base(position.Filename),
				line: position.Line,
			}
		}
	}
	return defined
}

// answersDiagnostics reports whether fn returns a diagnostic slice, in any of
// its results. checkPlatform answers the platform beside its diagnostics, and
// it is still a check.
func answersDiagnostics(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, result := range fn.Type.Results.List {
		slice, ok := result.Type.(*ast.ArrayType)
		if !ok || slice.Len != nil {
			continue
		}
		selector, ok := slice.Elt.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok {
			continue
		}
		if pkg.Name+"."+selector.Sel.Name == doctorDiagnosticType {
			return true
		}
	}
	return false
}

// registeredChecks answers every function named as the Check field of a
// registration, whatever registry it is registered with. A check that has been
// moved onto the registry stops being a finding here.
func registeredChecks(files []*ast.File) map[string]bool {
	registered := map[string]bool{}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			pair, ok := node.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := pair.Key.(*ast.Ident)
			if !ok || key.Name != "Check" {
				return true
			}
			if value, ok := pair.Value.(*ast.Ident); ok {
				registered[value.Name] = true
			}
			return true
		})
	}
	return registered
}

// handCalls answers the runner's name and every check function it calls by
// name. The runner is the function that calls the phase dispatcher, so this
// gate holds no copy of the runner's name.
func handCalls(files []*ast.File, defined map[string]definition) (string, []string, error) {
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if !callsByName(fn, doctorDispatcher) {
				continue
			}
			return fn.Name.Name, calledChecks(fn, defined), nil
		}
	}
	return "", nil, fmt.Errorf("%w: no function in %s calls %s", ErrNoDoctorRunner, doctorDir, doctorDispatcher)
}

// callsByName reports whether fn calls the named function directly.
func callsByName(fn *ast.FuncDecl, name string) bool {
	called := false
	ast.Inspect(fn, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == name {
			called = true
		}
		return !called
	})
	return called
}

// calledChecks answers every check function fn calls by name, once each.
func calledChecks(fn *ast.FuncDecl, defined map[string]definition) []string {
	seen := map[string]bool{}
	var called []string
	ast.Inspect(fn, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		if _, isCheck := defined[ident.Name]; !isCheck || seen[ident.Name] {
			return true
		}
		seen[ident.Name] = true
		called = append(called, ident.Name)
		return true
	})
	return called
}
