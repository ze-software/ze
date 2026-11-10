// Design: docs/contributing/error-inventory.md -- the inventory of user-visible message sites
//
// Detail: classify.go -- the status of the message a writer is handed
// Related: scan.go -- the parse of the tree the writers are found in
//
// writers.go is the one declaration of the surface writers this package owns:
// the calls and literals that hand a message to a user on a surface neither
// archoperatorerror (registered handler returns and error answers) nor
// archdoctorcoverage (log lines, report raises, health rows, doctor findings)
// lists. A new writer helper is one row in writers, and nothing else changes.

package archerrorinventory

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// The import paths the writer table names.
const (
	commandPath       = modulePath + "/internal/component/command"
	dispatchPath      = modulePath + "/cmd/ze"
	errorfragmentPath = modulePath + "/internal/core/errorfragment"
	fmtPath           = "fmt"
	grpcStatusPath    = "google.golang.org/grpc/status"
	httpPath          = "net/http"
	humanPath         = modulePath + "/internal/le/le/human"
	leactionPath      = modulePath + "/internal/le/le/action"
	lgPath            = modulePath + "/internal/component/lg"
	mcpPath           = modulePath + "/internal/component/mcp"
	restPath          = modulePath + "/internal/component/api/rest"
)

// errorFunc is the name of the writer function net/http and gRPC status both
// call Error.
const errorFunc = "Error"

// gnmiDir is the gNMI server's tree: a gRPC status error there is gNMI's,
// and one anywhere else is the gRPC command API's.
const gnmiDir = "internal/component/gnmi"

// findingSuffix ends the name of every ./le report's finding type.
const findingSuffix = "Finding"

// writerShape is how a writer is recognized in the syntax.
type writerShape uint8

// The writer shapes. The zero value is no shape, so a row of the table
// nobody shaped matches nothing.
const (
	shapeUnspecified writerShape = iota
	// shapeCall is a call of the function ident of the package at path:
	// qualified through an import, or unqualified inside that package.
	shapeCall
	// shapeMethod is a call of the method ident of the type receiver, on any
	// receiver expression, inside the package at path. The method is
	// unexported, so only that package can call it.
	shapeMethod
	// shapeStderrPrint is a call of path.ident whose first argument is
	// os.Stderr.
	shapeStderrPrint
	// shapeStderrMethod is a call of the method ident on os.Stderr.
	shapeStderrMethod
	// shapeLiteral is a composite literal of the type ident of the package
	// at path.
	shapeLiteral
	// shapeFinding is a composite literal of a struct type an ./le package
	// declares whose name ends in findingSuffix: the record each ./le report
	// prints one finding from.
	shapeFinding
)

// span is the arguments, or literal elements, that carry the message: from
// first up to but excluding last, where last 0 means through the end.
type span struct {
	first int
	last  int
}

// of answers the expressions of list inside the span. A list shorter than
// the span answers what it holds.
func (s span) of(list []ast.Expr) []ast.Expr {
	if s.first >= len(list) {
		return nil
	}
	if s.last == 0 {
		return list[s.first:]
	}
	if s.last > len(list) {
		return list[s.first:]
	}
	return list[s.first:s.last]
}

// writer is one row of the writer table.
type writer struct {
	surface Surface
	shape   writerShape
	path    string
	ident   string
	// receiver is the type a shapeMethod writer's method is declared on.
	receiver string
	message  span
	// within limits the writer to calls in these directories or below; none
	// is anywhere its surface allows.
	within []string
}

// writers is every surface writer this package owns, and the only list of
// them. The first row that matches a call or literal is its writer, so a
// row limited by within comes before the row of the same writer that is
// not. A SurfaceTooling writer matches only under internal/le, and every
// other writer only outside it: le's own stderr is development output, not a
// product surface. The writers take a string or a plain error, so each is
// judged by how its message argument was built (classify.go).
var writers = []writer{
	{surface: SurfaceLocalCLI, shape: shapeCall, path: dispatchPath, ident: "reportDispatchError", message: span{0, 1}},
	{surface: SurfaceLocalCLI, shape: shapeCall, path: commandPath, ident: "LocalRefusal", message: span{1, 2}},
	{surface: SurfaceLocalCLI, shape: shapeStderrPrint, path: fmtPath, ident: "Fprint", message: span{1, 0}},
	{surface: SurfaceLocalCLI, shape: shapeStderrPrint, path: fmtPath, ident: "Fprintf", message: span{1, 0}},
	{surface: SurfaceLocalCLI, shape: shapeStderrPrint, path: fmtPath, ident: "Fprintln", message: span{1, 0}},
	{surface: SurfaceLocalCLI, shape: shapeStderrMethod, path: "os", ident: "WriteString", message: span{0, 1}},
	{surface: SurfaceMCP, shape: shapeCall, path: mcpPath, ident: "refusalResult", message: span{0, 1}},
	{surface: SurfaceMCP, shape: shapeCall, path: mcpPath, ident: "ErrResult", message: span{0, 1}},
	{surface: SurfaceMCP, shape: shapeMethod, path: mcpPath, receiver: "*Streamable", ident: "fail", message: span{2, 3}},
	{surface: SurfaceREST, shape: shapeCall, path: restPath, ident: "writeError", message: span{2, 3}},
	{surface: SurfaceREST, shape: shapeCall, path: lgPath, ident: "writeJSONError", message: span{2, 3}},
	{surface: SurfaceWeb, shape: shapeCall, path: httpPath, ident: errorFunc, message: span{1, 2}},
	{surface: SurfaceWeb, shape: shapeCall, path: errorfragmentPath, ident: "Render", message: span{1, 2}},
	{surface: SurfaceGNMI, shape: shapeCall, path: grpcStatusPath, ident: errorFunc, message: span{1, 2}, within: []string{gnmiDir}},
	{surface: SurfaceGNMI, shape: shapeCall, path: grpcStatusPath, ident: "Errorf", message: span{1, 0}, within: []string{gnmiDir}},
	{surface: SurfaceGRPC, shape: shapeCall, path: grpcStatusPath, ident: errorFunc, message: span{1, 2}},
	{surface: SurfaceGRPC, shape: shapeCall, path: grpcStatusPath, ident: "Errorf", message: span{1, 0}},
	{surface: SurfaceTooling, shape: shapeCall, path: leactionPath, ident: "ReportError", message: span{0, 1}},
	{surface: SurfaceTooling, shape: shapeLiteral, path: humanPath, ident: "Refusal", message: span{0, 0}},
	{surface: SurfaceTooling, shape: shapeFinding, ident: findingSuffix, message: span{0, 0}},
}

// label answers how the report names the writer, derived from the row so the
// name never disagrees with what is matched: an exported function or type
// qualified by its package's last path element, an unexported one bare.
func (w *writer) label() string {
	qualified := w.path[strings.LastIndexByte(w.path, '/')+1:] + "." + w.ident
	switch w.shape {
	case shapeCall, shapeLiteral:
		if token.IsExported(w.ident) {
			return qualified
		}
		return w.ident
	case shapeMethod:
		return "(" + w.receiver + ")." + w.ident
	case shapeStderrPrint:
		return qualified + "(os.Stderr)"
	case shapeStderrMethod:
		return "os.Stderr." + w.ident
	case shapeFinding:
		return w.ident
	case shapeUnspecified:
		return nameUnspecified
	}
	return nameUnknown
}

// allows reports whether the writer may match in a file of dir: its surface
// decides the side of internal/le, and within narrows it further.
func (w *writer) allows(dir string) bool {
	if underAny(dir, []string{toolingDir}) != (w.surface == SurfaceTooling) {
		return false
	}
	if len(w.within) == 0 {
		return true
	}
	return underAny(dir, w.within)
}

// callWriter answers the writer a call hands its message to, if any.
func callWriter(file *sourceFile, call *ast.CallExpr) (*writer, bool) {
	for i := range writers {
		w := &writers[i]
		if !w.allows(file.dir) {
			continue
		}
		if w.matchesCall(file, call) {
			return w, true
		}
	}
	return nil, false
}

// matchesCall reports whether call is the writer's call shape.
func (w *writer) matchesCall(file *sourceFile, call *ast.CallExpr) bool {
	switch w.shape {
	case shapeCall:
		return file.refersTo(call.Fun, w.path, w.ident)
	case shapeMethod:
		selector, isSelector := ast.Unparen(call.Fun).(*ast.SelectorExpr)
		if !isSelector {
			return false
		}
		if selector.Sel.Name != w.ident {
			return false
		}
		return file.packagePath() == w.path
	case shapeStderrPrint:
		if !file.refersTo(call.Fun, w.path, w.ident) {
			return false
		}
		if len(call.Args) == 0 {
			return false
		}
		return file.isStderr(call.Args[0])
	case shapeStderrMethod:
		selector, isSelector := ast.Unparen(call.Fun).(*ast.SelectorExpr)
		if !isSelector {
			return false
		}
		if selector.Sel.Name != w.ident {
			return false
		}
		return file.isStderr(selector.X)
	case shapeLiteral, shapeFinding, shapeUnspecified:
		return false
	}
	return false
}

// literalWriter answers the writer a composite literal of type typ builds,
// and the name the report gives it. typ is the literal's own type, or for
// an element whose type the enclosing slice, array or map literal elides,
// that literal's element type.
func (index *treeIndex) literalWriter(file *sourceFile, typ ast.Expr) (*writer, string, bool) {
	if star, isStar := typ.(*ast.StarExpr); isStar {
		typ = star.X
	}
	for i := range writers {
		w := &writers[i]
		if !w.allows(file.dir) {
			continue
		}
		switch w.shape {
		case shapeLiteral:
			if file.refersTo(typ, w.path, w.ident) {
				label := w.label()
				return w, label, true
			}
		case shapeFinding:
			if index.isFindingType(file, typ) {
				return w, types.ExprString(typ), true
			}
		case shapeCall, shapeMethod, shapeStderrPrint, shapeStderrMethod, shapeUnspecified:
		}
	}
	return nil, "", false
}

// isFindingType reports whether typ names a struct type an ./le package
// declares with a name ending in findingSuffix: unqualified inside that
// package, or qualified through its import.
func (index *treeIndex) isFindingType(file *sourceFile, typ ast.Expr) bool {
	switch node := ast.Unparen(typ).(type) {
	case *ast.Ident:
		return index.findingTypes[file.dir][node.Name]
	case *ast.SelectorExpr:
		ident, isIdent := node.X.(*ast.Ident)
		if !isIdent {
			return false
		}
		dir, inTree := cutModule(file.imports[ident.Name])
		if !inTree {
			return false
		}
		return index.findingTypes[dir][node.Sel.Name]
	default:
		return false
	}
}
