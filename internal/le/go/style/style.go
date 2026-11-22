// Design: docs/contributing/ze-go-style.md -- unified diff-scoped Go style checker
// Detail: actions.go -- the fast `./le go style check` command surface
// Related: register.go -- le area registration
//
// Package gostyle owns the single diff-scoped Go style and formatting engine
// shared across edit hooks (internal/le/hookruntime), commit gates
// (internal/le/repo), and the lightweight `./le go style check` command.
package gostyle

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"path"
	"strconv"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// localImportPrefix is the module prefix that goimports places in the local
// import group, matching .golangci.yml.
const (
	localImportPrefix = "github.com/ze-software/ze"
	pkgLog            = "log"
	funcPrintf        = "Printf"
)

// Severity classifies a style finding. Zero is invalid so every finding names
// its severity explicitly.
type Severity uint8

const (
	// SeverityUnspecified is the invalid zero value.
	SeverityUnspecified Severity = iota
	// SeverityBlock refuses an edit or commit.
	SeverityBlock
	// SeverityWarn is an advisory warning.
	SeverityWarn
)

// Finding is one Go formatting or newly added style defect.
type Finding struct {
	Path     string
	Line     int
	Severity Severity
	Err      *opererr.Error
}

// Options tunes which optional checks CheckAdded runs beyond the core blocking
// Go patterns.
type Options struct {
	// CheckDesignHeader requires a `// Design:` file header on newly created
	// non-exempt Go files (when before is empty).
	CheckDesignHeader bool
	// CheckWarnNames emits a SeverityWarn finding for newly added exported
	// functions whose name joins two responsibilities with `And`.
	CheckWarnNames bool
}

// IsGenerated reports whether source carries the Go generated-file marker in
// its leading bytes.
func IsGenerated(source string) bool {
	head := source
	if len(head) > 512 {
		head = head[:512]
	}
	return strings.Contains(head, "// Code generated ") && strings.Contains(head, "DO NOT EDIT")
}

// CheckFormat checks that source is gofmt-formatted and keeps local
// github.com/ze-software/ze imports separated into their own final group.
// Files with syntax errors return nil because the compiler reports them.
func CheckFormat(rel, source string) *Finding {
	formatted, err := format.Source([]byte(source))
	// refuse-by-default: guards runAllChecks -- syntax errors are reported by the compiler, not as a gofmt finding
	if err != nil {
		return nil
	}
	if !bytes.Equal(formatted, []byte(source)) {
		return &Finding{
			Path:     rel,
			Line:     1,
			Severity: SeverityBlock,
			Err: opererr.New(
				rel+" is not gofmt-formatted",
				"an unformatted Go file breaks scoped lint for every later session touching this package (plan/journal/lint-contract-not-applied.md)",
				"gofmt -w "+rel,
			),
		}
	}
	return checkLocalImportGrouping(rel, source)
}

// checkLocalImportGrouping verifies that local imports (github.com/ze-software/ze/...)
// are not mixed into a non-local import group or placed before non-local imports.
func checkLocalImportGrouping(rel, source string) *Finding {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, source, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	for _, decl := range file.Decls {
		genDecl, isGen := decl.(*ast.GenDecl)
		if !isGen {
			continue
		}
		if genDecl.Tok != token.IMPORT {
			continue
		}
		if len(genDecl.Specs) < 2 {
			continue
		}
		if finding := checkImportGroupOrder(fset, rel, genDecl.Specs); finding != nil {
			return finding
		}
	}
	return nil
}

func checkImportGroupOrder(fset *token.FileSet, rel string, specs []ast.Spec) *Finding {
	seenLocal := false
	prevLine := 0
	prevLocal := false
	for _, raw := range specs {
		spec, isImport := raw.(*ast.ImportSpec)
		if !isImport {
			continue
		}
		importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			continue
		}
		line := fset.Position(spec.Pos()).Line
		local := isLocalImport(importPath)
		sameGroup := prevLine > 0 && line-prevLine <= 1
		if sameGroup && local != prevLocal {
			return importGroupFinding(rel, line)
		}
		if seenLocal && !local {
			return importGroupFinding(rel, line)
		}
		if local {
			seenLocal = true
		}
		prevLine = line
		prevLocal = local
	}
	return nil
}

func isLocalImport(importPath string) bool {
	if importPath == localImportPrefix {
		return true
	}
	return strings.HasPrefix(importPath, localImportPrefix+"/")
}

func importGroupFinding(rel string, line int) *Finding {
	var tb textbuf.Buffer
	problem := tb.Str(rel).Byte(':').Int(int64(line)).Str(": imports are not grouped with local prefix ").Str(localImportPrefix).Str(" last").String()
	return &Finding{
		Path:     rel,
		Line:     line,
		Severity: SeverityBlock,
		Err: opererr.New(
			problem,
			"goimports in .golangci.yml requires local github.com/ze-software/ze imports in the final import group separated by a blank line",
			"goimports -local "+localImportPrefix+" -format-only -w "+rel,
		),
	}
}

// CheckAdded inspects newly added lines of after relative to before for Ze's
// forbidden Go style patterns. When after parses as complete Go, exact AST
// analysis runs so comments, string literals, and untouched lines never
// false-positive; when after is an incomplete fragment (such as a synthetic
// hook probe), token scanning still ignores comments and string literals.
func CheckAdded(rel, before, after string, opts Options) []Finding {
	if !isJudgedShippedGo(rel) {
		return nil
	}
	if IsGenerated(after) {
		return nil
	}
	added := AddedLineNumbers(before, after)
	if len(added) == 0 {
		return nil
	}

	var hits []Finding
	if opts.CheckDesignHeader && before == "" && requiresDesignHeader(rel) && !hasDesignHeader(after) {
		hits = append(hits, newFinding(rel, 1, SeverityBlock,
			"new Go file is missing a // Design: header",
			"every non-exempt shipped Go file names the design page that governs it (docs/contributing/go-conventions.md)",
			"add `// Design: <path> -- <topic>` as the first comment in "+rel,
		))
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, after, parser.ParseComments)
	if err != nil {
		return append(hits, checkFragmentTokens(rel, after, added, opts)...)
	}

	hits = append(hits, checkCommentsAST(fset, rel, file, added)...)
	hits = append(hits, checkNodesAST(fset, rel, file, added, opts)...)
	return hits
}

func isJudgedShippedGo(rel string) bool {
	norm := strings.TrimPrefix(rel, "./")
	if !strings.HasSuffix(norm, ".go") {
		return false
	}
	if strings.HasSuffix(norm, "_test.go") {
		return false
	}
	if strings.Contains(norm, "/testdata/") {
		return false
	}
	if strings.HasPrefix(norm, "vendor/") {
		return false
	}
	if strings.Contains(norm, "/vendor/") {
		return false
	}
	return true
}

func requiresDesignHeader(rel string) bool {
	norm := strings.TrimPrefix(rel, "./")
	underShipped := strings.HasPrefix(norm, "cmd/") || strings.HasPrefix(norm, "internal/") || strings.HasPrefix(norm, "pkg/")
	if !underShipped {
		return false
	}
	base := path.Base(norm)
	if strings.HasSuffix(base, "_gen.go") {
		return false
	}
	if strings.HasPrefix(base, "register") {
		return false
	}
	if base == "embed.go" {
		return false
	}
	if base == "doc.go" {
		return false
	}
	return true
}

func hasDesignHeader(source string) bool {
	lineNum := 0
	for line := range strings.SplitSeq(source, "\n") {
		lineNum++
		if lineNum > 20 {
			return false
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "// Design:") {
			return true
		}
		if strings.HasPrefix(trimmed, "package ") {
			return false
		}
	}
	return false
}

func checkCommentsAST(fset *token.FileSet, rel string, file *ast.File, added map[int]bool) []Finding {
	var hits []Finding
	for _, group := range file.Comments {
		for _, comment := range group.List {
			line := fset.Position(comment.Pos()).Line
			if !added[line] {
				continue
			}
			if reason := directiveNolintProblem(comment.Text); reason != "" {
				hits = append(hits, newFinding(rel, line, SeverityBlock,
					reason,
					"a //nolint directive must name a specific linter and explain why it cannot be fixed",
					"fix the lint finding or write `//nolint:<linter> // <reason>` in "+rel,
				))
			}
		}
	}
	return hits
}

func checkNodesAST(fset *token.FileSet, rel string, file *ast.File, added map[int]bool, opts Options) []Finding {
	var hits []Finding
	norm := strings.TrimPrefix(rel, "./")
	allowInit := initAllowed(norm)
	allowFprintf := fprintfAllowed(norm)
	allowSprintf := sprintfAllowed(norm)
	allowExit := osExitAllowed(norm)
	allowZeEnv := zeEnvAllowed(norm)
	importsLogStdlib := importsStdlibLog(file)

	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			line := fset.Position(typed.Pos()).Line
			if !added[line] {
				return true
			}
			if finding := checkCallExpr(norm, line, typed, allowSprintf, allowFprintf, allowExit, allowZeEnv, importsLogStdlib); finding != nil {
				hits = append(hits, *finding)
			}
		case *ast.GoStmt:
			line := fset.Position(typed.Pos()).Line
			if !added[line] {
				return true
			}
			if _, isLit := typed.Call.Fun.(*ast.FuncLit); isLit {
				hits = append(hits, newFinding(norm, line, SeverityBlock,
					"newly added anonymous goroutine (go func)",
					"anonymous goroutines hide ownership and shutdown lifecycle (ai/rules/goroutine-lifecycle.md)",
					"extract a named worker or helper method with explicit shutdown in "+norm,
				))
			}
		case *ast.SwitchStmt:
			line := fset.Position(typed.Pos()).Line
			if !added[line] {
				return true
			}
			if isSwitchArgsZero(typed.Tag) {
				hits = append(hits, newFinding(norm, line, SeverityBlock,
					"newly added switch args[0] dispatch",
					"Ze dispatches commands through registration tables, never manual switch args[0]",
					"register the command in the package's command/action table instead of switching on args[0] in "+norm,
				))
			}
		case *ast.FuncDecl:
			line := fset.Position(typed.Pos()).Line
			if !added[line] {
				return true
			}
			if typed.Recv == nil && typed.Name.Name == "init" && !allowInit {
				hits = append(hits, newFinding(norm, line, SeverityBlock,
					"newly added init() outside register*.go",
					"Ze keeps package registration in register*.go so startup side effects stay discoverable",
					"move init() into register.go in "+path.Dir(norm),
				))
			}
			if opts.CheckWarnNames && isExportedAndName(typed.Name.Name) {
				hits = append(hits, newFinding(norm, line, SeverityWarn,
					"exported function name joins two responsibilities with And ("+typed.Name.Name+")",
					"a function should own one responsibility (docs/contributing/ze-go-style.md)",
					"split "+typed.Name.Name+" or rename it to one verb in "+norm,
				))
			}
		case *ast.CompositeLit:
			line := fset.Position(typed.Pos()).Line
			if !added[line] {
				return true
			}
			if isFakeBufHandle(typed) {
				hits = append(hits, newFinding(norm, line, SeverityBlock,
					"newly added BufHandle backed by make()",
					"wire buffers must come from a bounded pool, not a fresh make() allocation (ai/rules/performance.md)",
					"acquire a pooled BufHandle instead of BufHandle{Buf: make(...)} in "+norm,
				))
			}
		case *ast.BinaryExpr:
			line := fset.Position(typed.Pos()).Line
			if !added[line] {
				return true
			}
			if isStringComparison(typed) {
				hits = append(hits, newFinding(norm, line, SeverityBlock,
					"newly added comparison on .String()",
					"String() is for human output; compare typed values directly instead of formatted strings (docs/contributing/ze-go-style.md)",
					"compare the typed values directly instead of calling .String() in "+norm,
				))
			}
		}
		return true
	})

	return hits
}

func checkCallExpr(
	rel string,
	line int,
	call *ast.CallExpr,
	allowSprintf bool,
	allowFprintf bool,
	allowExit bool,
	allowZeEnv bool,
	importsLogStdlib bool,
) *Finding {
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	// refuse-by-default: guards checkNodesAST -- non-selector calls carry no package-qualified banned symbol
	if !isSel {
		return nil
	}
	pkgIdent, isIdent := sel.X.(*ast.Ident)
	// refuse-by-default: guards checkNodesAST -- chained or non-identifier selectors are not package calls
	if !isIdent {
		return nil
	}
	switch pkgIdent.Name {
	case "fmt":
		switch sel.Sel.Name {
		case "Sprintf", funcPrintf:
			if allowSprintf {
				return nil
			}
			f := newFinding(rel, line, SeverityBlock,
				"newly added fmt."+sel.Sel.Name+" call",
				"Ze uses internal/core/textbuf.Buffer for string building and slogutil for logging",
				"replace fmt."+sel.Sel.Name+" with textbuf.Buffer or slogutil in "+rel,
			)
			return &f
		case "Fprintf":
			if allowFprintf {
				return nil
			}
			f := newFinding(rel, line, SeverityBlock,
				"newly added fmt.Fprintf call",
				"Ze writes structured text with internal/core/textbuf.Buffer instead of fmt.Fprintf",
				"replace fmt.Fprintf with textbuf.Buffer in "+rel,
			)
			return &f
		}
	case "strconv":
		switch sel.Sel.Name {
		case "FormatInt", "FormatUint":
			f := newFinding(rel, line, SeverityBlock,
				"newly added strconv."+sel.Sel.Name+" call",
				"Ze formats integers into internal/core/textbuf.Buffer (.Int / .Uint) to avoid heap allocations",
				"replace strconv."+sel.Sel.Name+" with textbuf.Buffer in "+rel,
			)
			return &f
		}
	case pkgLog:
		if importsLogStdlib && isLegacyLogFunc(sel.Sel.Name) {
			f := newFinding(rel, line, SeverityBlock,
				"newly added legacy log."+sel.Sel.Name+" call",
				"Ze logs through slogutil.Logger(\"subsystem\"), never the stdlib log package (ai/rules/go-standards.md)",
				"replace log."+sel.Sel.Name+" with slogutil.Logger in "+rel,
			)
			return &f
		}
	case "os":
		if sel.Sel.Name == "Exit" && !allowExit {
			f := osExitFinding(rel, line)
			return &f
		}
		if !allowZeEnv && isOSEnvFunc(sel.Sel.Name) && firstArgIsZeEnvKey(call) {
			f := newFinding(rel, line, SeverityBlock,
				"newly added os."+sel.Sel.Name+" on a Ze environment key",
				"Ze environment variables must be registered and accessed through internal/core/env (ai/rules/go-standards.md)",
				"register the key with env.MustRegister and read it through internal/core/env in "+rel,
			)
			return &f
		}
	}
	return nil
}

// checkFragmentTokens scans partial Go snippets that do not parse as a full
// file (such as hook unit probes). Because it uses go/scanner, comments and
// string literals are tokenized and skipped accurately.
func checkFragmentTokens(rel, source string, added map[int]bool, opts Options) []Finding {
	norm := strings.TrimPrefix(rel, "./")
	allowFprintf := fprintfAllowed(norm)
	allowSprintf := sprintfAllowed(norm)
	allowExit := osExitAllowed(norm)
	allowInit := initAllowed(norm)

	var scan scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile(norm, fset.Base(), len(source))
	scan.Init(file, []byte(source), func(token.Position, string) {}, scanner.ScanComments)

	type tok struct {
		line int
		kind token.Token
		lit  string
	}
	var tokens []tok
	var hits []Finding
	for {
		pos, kind, lit := scan.Scan()
		if kind == token.EOF {
			break
		}
		line := fset.Position(pos).Line
		if kind == token.COMMENT {
			if added[line] {
				if reason := directiveNolintProblem(lit); reason != "" {
					hits = append(hits, newFinding(norm, line, SeverityBlock,
						reason,
						"a //nolint directive must name a specific linter and explain why it cannot be fixed",
						"fix the lint finding or write `//nolint:<linter> // <reason>` in "+norm,
					))
				}
			}
			continue
		}
		tokens = append(tokens, tok{line: line, kind: kind, lit: lit})
	}

	for i := range tokens {
		t := tokens[i]
		if !added[t.line] {
			continue
		}
		// Match pkg.Func(
		if i+3 < len(tokens) && t.kind == token.IDENT && tokens[i+1].kind == token.PERIOD && tokens[i+2].kind == token.IDENT && tokens[i+3].kind == token.LPAREN {
			pkgName := t.lit
			fnName := tokens[i+2].lit
			switch pkgName {
			case "fmt":
				if (fnName == "Sprintf" || fnName == funcPrintf) && !allowSprintf {
					hits = append(hits, newFinding(norm, t.line, SeverityBlock,
						"newly added fmt."+fnName+" call",
						"Ze uses internal/core/textbuf.Buffer for string building and slogutil for logging",
						"replace fmt."+fnName+" with textbuf.Buffer or slogutil in "+norm,
					))
				}
				if fnName == "Fprintf" && !allowFprintf {
					hits = append(hits, newFinding(norm, t.line, SeverityBlock,
						"newly added fmt.Fprintf call",
						"Ze writes structured text with internal/core/textbuf.Buffer instead of fmt.Fprintf",
						"replace fmt.Fprintf with textbuf.Buffer in "+norm,
					))
				}
			case "strconv":
				if fnName == "FormatInt" || fnName == "FormatUint" {
					hits = append(hits, newFinding(norm, t.line, SeverityBlock,
						"newly added strconv."+fnName+" call",
						"Ze formats integers into internal/core/textbuf.Buffer (.Int / .Uint) to avoid heap allocations",
						"replace strconv."+fnName+" with textbuf.Buffer in "+norm,
					))
				}
			case pkgLog:
				if isLegacyLogFunc(fnName) {
					hits = append(hits, newFinding(norm, t.line, SeverityBlock,
						"newly added legacy log."+fnName+" call",
						"Ze logs through slogutil.Logger(\"subsystem\"), never the stdlib log package (ai/rules/go-standards.md)",
						"replace log."+fnName+" with slogutil.Logger in "+norm,
					))
				}
			case "os":
				if fnName == "Exit" && !allowExit {
					hits = append(hits, osExitFinding(norm, t.line))
				}
			}
		}
		// Match go func(
		if i+2 < len(tokens) && t.kind == token.GO && tokens[i+1].kind == token.FUNC && tokens[i+2].kind == token.LPAREN {
			hits = append(hits, newFinding(norm, t.line, SeverityBlock,
				"newly added anonymous goroutine (go func)",
				"anonymous goroutines hide ownership and shutdown lifecycle (ai/rules/goroutine-lifecycle.md)",
				"extract a named worker or helper method with explicit shutdown in "+norm,
			))
		}
		// Match switch args[0]
		if i+4 < len(tokens) && t.kind == token.SWITCH && tokens[i+1].kind == token.IDENT && tokens[i+1].lit == "args" &&
			tokens[i+2].kind == token.LBRACK && tokens[i+3].kind == token.INT && tokens[i+3].lit == "0" && tokens[i+4].kind == token.RBRACK {
			hits = append(hits, newFinding(norm, t.line, SeverityBlock,
				"newly added switch args[0] dispatch",
				"Ze dispatches commands through registration tables, never manual switch args[0]",
				"register the command in the package's command/action table instead of switching on args[0] in "+norm,
			))
		}
		// Match func init()
		if !allowInit && i+3 < len(tokens) && t.kind == token.FUNC && tokens[i+1].kind == token.IDENT && tokens[i+1].lit == "init" &&
			tokens[i+2].kind == token.LPAREN && tokens[i+3].kind == token.RPAREN {
			hits = append(hits, newFinding(norm, t.line, SeverityBlock,
				"newly added init() outside register*.go",
				"Ze keeps package registration in register*.go so startup side effects stay discoverable",
				"move init() into register.go in "+path.Dir(norm),
			))
		}
		// Match BufHandle{... Buf: make(
		if t.kind == token.IDENT && t.lit == "BufHandle" && i+1 < len(tokens) && tokens[i+1].kind == token.LBRACE {
			depth := 0
			for j := i + 1; j < len(tokens); j++ {
				if tokens[j].kind == token.LBRACE {
					depth++
				}
				if tokens[j].kind == token.RBRACE {
					depth--
					if depth == 0 {
						break
					}
				}
				if j+3 < len(tokens) && tokens[j].kind == token.IDENT && tokens[j].lit == "Buf" &&
					tokens[j+1].kind == token.COLON && tokens[j+2].kind == token.IDENT && tokens[j+2].lit == "make" && tokens[j+3].kind == token.LPAREN {
					hits = append(hits, newFinding(norm, t.line, SeverityBlock,
						"newly added BufHandle backed by make()",
						"wire buffers must come from a bounded pool, not a fresh make() allocation (ai/rules/performance.md)",
						"acquire a pooled BufHandle instead of BufHandle{Buf: make(...)} in "+norm,
					))
					break
				}
			}
		}
		// Match exported func FooAndBar(
		if opts.CheckWarnNames && i+2 < len(tokens) && t.kind == token.FUNC && tokens[i+1].kind == token.IDENT && isExportedAndName(tokens[i+1].lit) {
			hits = append(hits, newFinding(norm, t.line, SeverityWarn,
				"exported function name joins two responsibilities with And ("+tokens[i+1].lit+")",
				"a function should own one responsibility (docs/contributing/ze-go-style.md)",
				"split "+tokens[i+1].lit+" or rename it to one verb in "+norm,
			))
		}
	}
	return hits
}

func initAllowed(rel string) bool {
	base := path.Base(rel)
	if base == "init.go" {
		return true
	}
	if strings.HasPrefix(base, "register") {
		return true
	}
	return strings.HasPrefix(base, "all")
}

func sprintfAllowed(rel string) bool {
	return strings.HasPrefix(rel, "cmd/")
}

func fprintfAllowed(rel string) bool {
	if strings.HasPrefix(rel, "cmd/") {
		return true
	}
	return strings.HasPrefix(rel, "internal/le/hookruntime/")
}

func osExitAllowed(rel string) bool {
	base := path.Base(rel)
	if base == "main.go" {
		return true
	}
	if strings.HasPrefix(rel, "cmd/ze/pprof") {
		return true
	}
	if strings.HasPrefix(rel, "internal/core/env/") {
		return true
	}
	if strings.HasPrefix(rel, "internal/core/crashlog/") {
		return true
	}
	return strings.HasPrefix(rel, "internal/test/fixture/")
}

func isRegistrationFile(rel string) bool {
	if strings.HasPrefix(path.Base(rel), "register") {
		return true
	}
	return rel == "internal/component/config/doctor.go"
}

func osExitFinding(rel string, line int) Finding {
	if isRegistrationFile(rel) {
		return newFinding(rel, line, SeverityBlock,
			"newly added os.Exit() in registration file",
			"startup registration failures are compile-time Ze defects and must panic with BUG: rather than calling os.Exit (ai/rules/go-standards.md)",
			"call panic(\"BUG: <pkg>: registration failed: \" + err.Error()) instead of os.Exit in "+rel,
		)
	}
	return newFinding(rel, line, SeverityBlock,
		"newly added os.Exit() in handler",
		"handlers return errors rather than terminating the process directly (ai/rules/go-standards.md)",
		"return an error to the entrypoint instead of calling os.Exit in "+rel,
	)
}

func zeEnvAllowed(rel string) bool {
	if strings.HasPrefix(rel, "internal/core/env/") {
		return true
	}
	if strings.HasPrefix(rel, "internal/test/") {
		return true
	}
	if strings.HasPrefix(rel, "internal/le/") {
		return true
	}
	return strings.HasPrefix(rel, "internal/chaos/")
}

func importsStdlibLog(file *ast.File) bool {
	if len(file.Imports) == 0 {
		return true
	}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if importPath == pkgLog {
			return true
		}
	}
	return false
}

func isLegacyLogFunc(name string) bool {
	switch name {
	case "Print", funcPrintf, "Println", "Fatal", "Fatalf", "Fatalln", "Panic", "Panicf", "Panicln":
		return true
	default:
		return false
	}
}

func isOSEnvFunc(name string) bool {
	switch name {
	case "Getenv", "LookupEnv", "Setenv":
		return true
	default:
		return false
	}
}

func firstArgIsZeEnvKey(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	lit, isLit := call.Args[0].(*ast.BasicLit)
	if !isLit {
		return false
	}
	if lit.Kind != token.STRING {
		return false
	}
	key, err := strconv.Unquote(lit.Value)
	if err != nil {
		return false
	}
	if key == "ZE_PLUGIN_HUB_TOKEN" {
		return false
	}
	lower := strings.ToLower(key)
	if strings.HasPrefix(lower, "ze.") {
		return true
	}
	return strings.HasPrefix(lower, "ze_")
}

func isFakeBufHandle(lit *ast.CompositeLit) bool {
	typeName := ""
	switch typed := lit.Type.(type) {
	case *ast.Ident:
		typeName = typed.Name
	case *ast.SelectorExpr:
		typeName = typed.Sel.Name
	}
	if typeName != "BufHandle" {
		return false
	}
	for _, elt := range lit.Elts {
		kv, isKV := elt.(*ast.KeyValueExpr)
		if !isKV {
			continue
		}
		key, isKey := kv.Key.(*ast.Ident)
		if !isKey {
			continue
		}
		if key.Name != "Buf" {
			continue
		}
		call, isCall := kv.Value.(*ast.CallExpr)
		if !isCall {
			continue
		}
		fn, isFn := call.Fun.(*ast.Ident)
		if isFn && fn.Name == "make" {
			return true
		}
	}
	return false
}

func isStringComparison(expr *ast.BinaryExpr) bool {
	if expr.Op != token.EQL && expr.Op != token.NEQ {
		return false
	}
	if isStringMethodCall(expr.X) {
		return true
	}
	return isStringMethodCall(expr.Y)
}

func isStringMethodCall(expr ast.Expr) bool {
	call, isCall := ast.Unparen(expr).(*ast.CallExpr)
	if !isCall {
		return false
	}
	if len(call.Args) != 0 {
		return false
	}
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel {
		return false
	}
	return sel.Sel.Name == "String"
}

func isSwitchArgsZero(tag ast.Expr) bool {
	idx, ok := tag.(*ast.IndexExpr)
	if !ok {
		return false
	}
	ident, ok := idx.X.(*ast.Ident)
	if !ok {
		return false
	}
	if ident.Name != "args" {
		return false
	}
	lit, ok := idx.Index.(*ast.BasicLit)
	return ok && lit.Kind == token.INT && lit.Value == "0"
}

func isExportedAndName(name string) bool {
	if len(name) < 5 {
		return false
	}
	if !unicode.IsUpper(rune(name[0])) {
		return false
	}
	idx := strings.Index(name[1:], "And")
	if idx < 0 {
		return false
	}
	after := idx + 1 + len("And")
	if after >= len(name) {
		return false
	}
	return unicode.IsUpper(rune(name[after]))
}

// directiveNolintProblem reports why comment is an invalid lint-suppression
// directive, or "" if the comment is valid or not one.
func directiveNolintProblem(comment string) string {
	trimmed := strings.TrimSpace(comment)
	rest, isNolint := strings.CutPrefix(trimmed, "//nolint")
	if !isNolint {
		rest, isNolint = strings.CutPrefix(trimmed, "// nolint")
		// refuse-by-default: guards checkCommentsAST -- ordinary comments are not nolint directives
		if !isNolint {
			return ""
		}
	}
	if !strings.HasPrefix(rest, ":") {
		return "newly added //nolint without a specific linter and reason (names no linter)"
	}
	afterColon := rest[1:]
	linterList, reason, hasSlash := strings.Cut(afterColon, "//")
	if !hasSlash {
		return "newly added //nolint without a specific linter and reason (carries no reason)"
	}
	if strings.TrimSpace(linterList) == "" {
		return "newly added //nolint without a specific linter and reason (names no linter)"
	}
	if strings.TrimSpace(reason) == "" {
		return "newly added //nolint without a specific linter and reason (carries no reason)"
	}
	return ""
}

// AddedLineNumbers answers the 1-based line numbers in after whose trimmed text
// appears more times in after than in before, so newly added lines are judged
// without flagging pre-existing legacy lines.
func AddedLineNumbers(before, after string) map[int]bool {
	oldCounts := make(map[string]int)
	if before != "" {
		for line := range strings.SplitSeq(before, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				oldCounts[trimmed]++
			}
		}
	}
	added := make(map[int]bool)
	for idx, line := range strings.Split(after, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if oldCounts[trimmed] > 0 {
			oldCounts[trimmed]--
			continue
		}
		added[idx+1] = true
	}
	return added
}

func newFinding(rel string, line int, severity Severity, problem, why, fix string) Finding {
	var tb textbuf.Buffer
	fullProblem := tb.Str(rel).Byte(':').Int(int64(line)).Str(": ").Str(problem).String()
	return Finding{
		Path:     rel,
		Line:     line,
		Severity: severity,
		Err:      opererr.New(fullProblem, why, fix),
	}
}

// FormatFindings renders blocking findings for commit-gate refusal output, or
// "" when there are none.
func FormatFindings(findings []Finding) string {
	var blocking []Finding
	for _, f := range findings {
		if f.Severity == SeverityBlock {
			blocking = append(blocking, f)
		}
	}
	if len(blocking) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	tb.Str("Go style/formatting check refused what this commit adds:\n")
	for _, hit := range blocking {
		tb.Str("  ").Str(hit.Err.Error()).Byte('\n')
	}
	tb.Str("See docs/contributing/ze-go-style.md and plan/journal/lint-contract-not-applied.md.\n")
	return tb.String()
}
