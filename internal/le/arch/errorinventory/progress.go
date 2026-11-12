// Design: docs/contributing/error-inventory.md -- declared non-error progress

package archerrorinventory

import (
	"go/ast"
	"go/token"
	"strconv"
)

// progressMessages declares fixed positive messages that stderr carries
// alongside errors. Every declaration names the complete site and literal;
// no function, printer or dynamic operand is exempt from error checks.
var progressMessages = []struct {
	file     string
	function string
	writer   string
	message  string
	reason   string
}{
	{"internal/appliance/cmd_push.go", "doPushUpdater", "Fprintln", "streaming boot image...", "The updater is about to stream the boot image; a stream failure is returned separately."},
	{"internal/appliance/cmd_push.go", "doPushUpdater", "Fprintln", "streaming boot code...", "The updater is about to stream boot code; a stream failure is returned separately."},
}

// declaredProgress recognizes only one reviewed fixed literal in its exact
// stderr writer call. It never resolves a variable or drops extra operands,
// so an error added to a progress call is still inventoried and refused.
func (sc scope) declaredProgress(w *writer, call *ast.CallExpr) bool {
	if w.shape != shapeStderrPrint {
		return false
	}
	if w.path != fmtPath {
		return false
	}
	if sc.fn == nil {
		return false
	}
	if sc.fn.Recv != nil {
		return false
	}
	if len(call.Args) != 2 {
		return false
	}
	if call.Ellipsis != token.NoPos {
		return false
	}
	literal, ok := call.Args[1].(*ast.BasicLit)
	if !ok {
		return false
	}
	if literal.Kind != token.STRING {
		return false
	}
	message, err := strconv.Unquote(literal.Value)
	if err != nil {
		return false
	}
	for _, declaration := range progressMessages {
		if declaration.file != sc.file.path {
			continue
		}
		if declaration.function != sc.fn.Name.Name {
			continue
		}
		if declaration.writer != w.ident {
			continue
		}
		if declaration.message != message {
			continue
		}
		return true
	}
	return false
}
