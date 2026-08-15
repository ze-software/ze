// Related: workbench_pages.go -- renderPageContent, the one door onto a page
// Related: secret.go -- the masking rule this door enforces

package web

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkbenchPagesReceiveOnlyTheMaskedTree verifies the page dispatcher hands
// every page the masked display tree, and builds it only for a page that reads
// config.
//
// VALIDATES: no call inside renderPageContent passes the raw working tree, the
// mask is built in one place, and the call that builds it sits inside the
// dispatch rather than above it.
// PREVENTS: two failures at once. A page added later that reads the raw tree
// publishes every ze:sensitive leaf it renders. Nothing else in this package
// catches that, because the secret tests name the pages that exist today.
// Masking above the dispatch deep-copies the whole configuration on every
// workbench GET. The tools, logs, firewall and VPN pages read no config, and
// the fall-through to the generic view masks per leaf.
//
// It reads the source because both properties are about the shape of one
// function. A rendered page cannot show which tree the handler was given.
func TestWorkbenchPagesReceiveOnlyTheMaskedTree(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "workbench_pages.go", nil, 0)
	require.NoError(t, err)

	var fn *ast.FuncDecl

	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Name.Name == "renderPageContent" {
			fn = d
			break
		}
	}
	require.NotNil(t, fn, "workbench_pages.go must declare renderPageContent")

	raw := fn.Type.Params.List[3].Names[0].Name

	calls, masks := 0, 0

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		calls++

		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "MaskSecrets" {
			masks++
		}

		for _, arg := range call.Args {
			ident, ok := arg.(*ast.Ident)
			if !ok || ident.Name != raw {
				continue
			}
			// The mask itself is the one reader of the raw tree.
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "MaskSecrets" {
				continue
			}

			assert.Fail(t, "a page is handed the raw configuration tree",
				"renderPageContent passes %s to %s at %s; pass the masked tree instead (secret.go)",
				raw, callName(call.Fun), fset.Position(call.Pos()))
		}

		return true
	})

	assert.Positive(t, calls, "no call was scanned; renderPageContent has stopped dispatching")
	assert.Equal(t, 1, masks, "the mask must be built in exactly one place inside renderPageContent")

	// Laziness: the mask is reached through the memo, and the memo is read only
	// from inside the dispatch.
	for _, stmt := range fn.Body.List {
		if _, ok := stmt.(*ast.SwitchStmt); ok {
			break
		}

		ast.Inspect(stmt, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "MaskSecrets" {
				// The call inside the memo's own body is the definition, not a use.
				return true
			}
			if ident, ok := call.Fun.(*ast.Ident); ok {
				assert.NotEqual(t, "display", ident.Name,
					"the masked tree is built before the dispatch, so a page that reads no config pays for it")
			}

			return true
		})
	}
}

// callName answers a readable name for a call's function expression.
func callName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}

	return "a call"
}
