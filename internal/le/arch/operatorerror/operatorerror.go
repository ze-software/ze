// Design: docs/architecture/core-design.md -- registration; the operator boundary set is derived from it
//
// Detail: resolve.go -- the package index and the handler each registration names
// Detail: report.go -- what check and report answer
//
// operatorerror.go finds the refusals an operator reads. A function registered
// as a command handler, an RPC handler or a config verifier is an operator
// boundary: the error it returns is rendered on the CLI, in an API answer or
// in a plugin RPC error. A plain errors.New or fmt.Errorf there carries what
// went wrong and nothing else, so the operator reads a refusal with no fix
// (ai/rules/user-facing-errors.md). The operator error type in
// internal/core/opererr carries what, why and the fix.
//
// The boundary set is DERIVED from the registration sites, never from a list
// of packages (ai/rules/principles.md): a new handler is a boundary the moment
// it registers, wherever it lives. The registration shapes this scanner reads
// are the ones Ze's registries take an error-returning handler through:
//
//	pluginserver.RPCRegistration{Handler: h}           rpc-handler
//	pluginserver.RegisterStreamingHandler(_, _, h)     streaming-handler
//	pluginserver.RegisterPluginStreamingHandler(...,h) streaming-handler
//	registry.Registration{RPCHandlers: {"m": h}}       plugin-rpc-handler
//	registry.AddRPCHandlers({"m": h})                  plugin-rpc-handler
//	registry.Registration{InProcessConfigVerifier: h}  config-verifier
//	p.OnConfigVerify(h)                                sdk-config-verify
//	p.OnExecuteCommand(h)                              sdk-command-handler
//
// command.MustRegisterLocal* and MustRegisterLocalData take handlers that
// answer an exit code or a LocalAnswer, never an error, so no return site of
// theirs can be a plain error and they are not in this set.

package archoperatorerror

import (
	"errors"
	"go/ast"
	"go/types"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The import paths the registration shapes and the accepted type live at.
const (
	modulePath   = "github.com/ze-software/ze"
	serverPath   = modulePath + "/internal/component/plugin/server"
	registryPath = modulePath + "/internal/component/plugin/registry"
	opererrPath  = modulePath + "/internal/core/opererr"
)

// Status is what one return site of a handler answers.
type Status uint8

// The return-site statuses. The zero value is no status, so a Site nobody
// classified cannot read as typed.
const (
	StatusUnspecified Status = iota
	// StatusTyped is an error built by the opererr package: a call into it,
	// a method chain rooted in such a call, an opererr composite literal, or
	// an opererr sentinel.
	StatusTyped
	// StatusPlain is an errors.New or fmt.Errorf built at the return: the
	// refusal the check judges.
	StatusPlain
	// StatusPassThrough is any other error value: a callee's error variable,
	// a call into another function, a named result. It is not a construction
	// this site made, so the check leaves it to the producer that built it,
	// and report lists it so the inventory can follow it.
	StatusPassThrough
)

// String names the status as report prints it.
func (s Status) String() string {
	switch s {
	case StatusTyped:
		return "typed"
	case StatusPlain:
		return "plain"
	case StatusPassThrough:
		return "pass-through"
	case StatusUnspecified:
		return "unspecified"
	}
	return "unknown-status-" + strconv.Itoa(int(s))
}

// MarshalText renders the status by name in JSON and YAML.
func (s Status) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// Kind is the registration shape that made a function an operator boundary.
type Kind uint8

// The registration kinds, one per shape in the file comment.
const (
	KindUnspecified Kind = iota
	KindRPC
	KindStreaming
	KindPluginRPC
	KindConfigVerifier
	KindSDKConfigVerify
	KindSDKCommand
)

// String names the kind as report prints it.
func (k Kind) String() string {
	switch k {
	case KindRPC:
		return "rpc-handler"
	case KindStreaming:
		return "streaming-handler"
	case KindPluginRPC:
		return "plugin-rpc-handler"
	case KindConfigVerifier:
		return "config-verifier"
	case KindSDKConfigVerify:
		return "sdk-config-verify"
	case KindSDKCommand:
		return "sdk-command-handler"
	case KindUnspecified:
		return "unspecified"
	}
	return "unknown-kind-" + strconv.Itoa(int(k))
}

// MarshalText renders the kind by name in JSON and YAML.
func (k Kind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// Site is one return statement of a registered handler that returns an
// error value. Paths are relative to the scanned tree, slash-separated.
type Site struct {
	File string `json:"file"`
	Line int    `json:"line"`
	// Handler is the function the return sits in: `name`, `Type.Method`, or
	// the enclosing function with "(func literal)" for an inline handler.
	Handler string `json:"handler"`
	Kind    Kind   `json:"kind"`
	// Registration is the file:line of the registration that made Handler a
	// boundary (the first one, where several register it).
	Registration string `json:"registration"`
	Status       Status `json:"status"`
	// Expr is the returned error expression, as source.
	Expr string `json:"expr"`
}

// Registration is one registration site the scan read.
type Registration struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Kind Kind   `json:"kind"`
	// WireMethod is the RPC's wire method where the registration spells it as
	// a string literal or a constant, and "" where it does not.
	WireMethod string `json:"wire-method,omitempty"`
	// WirePrefix is the literal head of a wire method the registration
	// computes, `"ze-host:show-" + section` in a loop over section names,
	// and "" where WireMethod is spelled whole or nothing literal leads it.
	WirePrefix string `json:"wire-prefix,omitempty"`
	// Handlers are the functions the registration resolved to.
	Handlers []string `json:"handlers"`
}

// Unresolved is a registration whose handler the AST could not follow to a
// function body: a local variable, an index into a table, a factory whose
// returned function is not a literal. Its return sites are unknown, which is
// said here rather than read as "no plain error".
type Unresolved struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Kind   Kind   `json:"kind"`
	Expr   string `json:"expr"`
	Reason string `json:"reason"`
}

// Inventory is the whole scan: every registration, every return site of the
// handlers they name, and the registrations whose handler could not be read.
type Inventory struct {
	Registrations []Registration `json:"registrations"`
	Sites         []Site         `json:"sites"`
	Unresolved    []Unresolved   `json:"unresolved"`
}

// Sites answers every return site of every registered operator handler under
// tree, sorted by file and line. It is the list spec-rea-practices-2's error
// inventory consumes; Scan answers the registrations and the unresolved
// handlers beside it.
func Sites(tree string) ([]Site, error) {
	inventory, err := Scan(tree)
	if err != nil {
		return nil, err
	}
	return inventory.Sites, nil
}

// Scan reads every shipped Go file under tree's product roots, derives the
// operator boundaries from their registration sites, and classifies each
// boundary's return sites.
//
// A file that does not parse is an error rather than a file with no handler:
// a gate that skips what it cannot read reports a clean tree for it.
func Scan(tree string) (Inventory, error) { return scan(tree, nil) }

// scan is Scan with overlay replacing the bytes of the files it names, so a
// commit's carried blobs and its parent's are judged without touching the
// working tree.
func scan(tree string, overlay map[string]overlayFile) (Inventory, error) {
	index, err := indexTree(tree, overlay)
	if err != nil {
		return Inventory{}, err
	}
	inventory := Inventory{Registrations: []Registration{}, Sites: []Site{}, Unresolved: []Unresolved{}}
	seen := make(map[string]bool)
	for _, found := range index.registrations() {
		registration := found.registration
		for _, handler := range found.handlers {
			targets, reason := index.resolveHandler(handler)
			if len(targets) == 0 {
				inventory.Unresolved = append(inventory.Unresolved, Unresolved{
					File: registration.File, Line: registration.Line, Kind: registration.Kind,
					Expr: types.ExprString(handler.expr), Reason: reason,
				})
				continue
			}
			for _, target := range targets {
				registration.Handlers = append(registration.Handlers, target.name)
				for _, site := range returnSites(target) {
					key := site.File + ":" + strconv.Itoa(site.Line)
					if seen[key] {
						continue
					}
					seen[key] = true
					site.Kind = registration.Kind
					site.Registration = registration.File + ":" + strconv.Itoa(registration.Line)
					inventory.Sites = append(inventory.Sites, site)
				}
			}
		}
		inventory.Registrations = append(inventory.Registrations, registration)
	}
	slices.SortFunc(inventory.Sites, func(a, b Site) int {
		if a.File != b.File {
			return compareStrings(a.File, b.File)
		}
		return a.Line - b.Line
	})
	return inventory, nil
}

// compareStrings orders two strings for a sort.
func compareStrings(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// ErrThinScan names a scan that found fewer registrations than the tree is
// known to hold, so it judged too little to answer for it.
var ErrThinScan = errors.New("the operator-error scan found too few handler registrations to judge the tree")

// checkFloor refuses an inventory below floor registrations. A scan that
// matched no registration shape (a renamed registry, a moved package) would
// otherwise answer zero plain sites, which is what a clean tree answers.
func checkFloor(inventory Inventory, floor int) error {
	if len(inventory.Registrations) >= floor {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str(": found ").Int(int64(len(inventory.Registrations))).Str(", at least ").Int(int64(floor)).
		Str(" expected. A registration shape in internal/le/arch/operatorerror/operatorerror.go no longer matches the registry it names; update the shape to the registry's current call")
	return errors.New(ErrThinScan.Error() + tb.String())
}

// returnSites answers the error-returning return statements of one handler
// body. Returns inside a nested function literal belong to that literal, not
// to the handler, and are not read.
func returnSites(target handlerTarget) []Site {
	results := resultCount(target.typ)
	if results == 0 {
		return nil
	}
	var sites []Site
	ast.Inspect(target.body, func(node ast.Node) bool {
		switch statement := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			site, isSite := classifyReturn(target, statement, results)
			if isSite {
				sites = append(sites, site)
			}
		}
		return true
	})
	return sites
}

// classifyReturn answers the site one return statement makes, and false for a
// return whose error result is nil.
//
// The error is the LAST result: every registration shape above takes a
// handler whose final result is its error.
func classifyReturn(target handlerTarget, statement *ast.ReturnStmt, results int) (Site, bool) {
	site := Site{
		File:    target.file.path,
		Line:    target.file.index.fset.Position(statement.Return).Line,
		Handler: target.name,
	}
	if len(statement.Results) == 0 {
		// A naked return hands back the named results the body set.
		site.Status = StatusPassThrough
		site.Expr = "(named results)"
		return site, true
	}
	if len(statement.Results) < results {
		// `return f()` hands back a callee's results whole.
		site.Status = StatusPassThrough
		site.Expr = types.ExprString(statement.Results[0])
		return site, true
	}
	last := ast.Unparen(statement.Results[len(statement.Results)-1])
	if ident, isIdent := last.(*ast.Ident); isIdent {
		if ident.Name == "nil" {
			return Site{}, false
		}
	}
	site.Expr = types.ExprString(last)
	site.Status = classifyError(target.file, last)
	return site, true
}

// classifyError answers what one returned error expression is.
func classifyError(file *sourceFile, expr ast.Expr) Status {
	if builtByOpererr(file, expr) {
		return StatusTyped
	}
	call, isCall := expr.(*ast.CallExpr)
	if !isCall {
		return StatusPassThrough
	}
	if file.refersTo(call.Fun, "errors", "New") {
		return StatusPlain
	}
	if file.refersTo(call.Fun, "fmt", "Errorf") {
		return StatusPlain
	}
	return StatusPassThrough
}

// builtByOpererr reports whether expr is an error the opererr package built:
// a call into it, a method chain whose root is such a call, the address of or
// a composite literal of one of its types, or one of its exported values.
//
// The walk descends a method chain one receiver per step, so it is bounded by
// the expression's own depth, which the source author wrote.
func builtByOpererr(file *sourceFile, expr ast.Expr) bool {
	for {
		switch node := ast.Unparen(expr).(type) {
		case *ast.CallExpr:
			selector, isSelector := node.Fun.(*ast.SelectorExpr)
			if !isSelector {
				return false
			}
			if file.importsAs(selector.X, opererrPath) {
				return true
			}
			expr = selector.X
		case *ast.UnaryExpr:
			expr = node.X
		case *ast.CompositeLit:
			selector, isSelector := node.Type.(*ast.SelectorExpr)
			if !isSelector {
				return false
			}
			return file.importsAs(selector.X, opererrPath)
		case *ast.SelectorExpr:
			return file.importsAs(node.X, opererrPath)
		default:
			return false
		}
	}
}

// resultCount answers how many results a function type declares, counting
// each name of a named group.
func resultCount(typ *ast.FuncType) int {
	if typ.Results == nil {
		return 0
	}
	count := 0
	for _, field := range typ.Results.List {
		if len(field.Names) == 0 {
			count++
			continue
		}
		count += len(field.Names)
	}
	return count
}
