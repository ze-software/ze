// Design: docs/architecture/plugin/feature-gates.md -- symbol manufacture is outside external authority.
package repoinventory

import (
	"go/ast"
	"strings"
)

// indexLinkname resolves only the local declaration identity. It does not
// trust the remote spelling or infer an inert implementation from its name.
func (e *externalEffects) indexLinkname(comment string, source sourceExpr) {
	fields := strings.Fields(comment)
	if len(fields) == 0 {
		return
	}
	if fields[0] != "//go:linkname" {
		return
	}
	if len(fields) < 2 {
		e.linkInvalid[source.pkg.dir] = source
		return
	}
	if len(fields) > 3 {
		e.linkInvalid[source.pkg.dir] = source
		return
	}
	object := callablePackageBinding(source.pkg, fields[1])
	if object == nil {
		e.linkInvalid[source.pkg.dir] = source
		return
	}
	if _, duplicate := e.linknames[object]; duplicate {
		e.linkInvalid[source.pkg.dir] = source
		return
	}
	switch declaration := object.Decl.(type) {
	case *ast.FuncDecl:
		source.value = declaration.Name
		e.linknames[object] = source
		if declaration.Body == nil {
			return
		}
	case *ast.ValueSpec:
		if object.Kind != ast.Var {
			e.linkInvalid[source.pkg.dir] = source
			return
		}
		e.linknames[object] = source
	default:
		e.linkInvalid[source.pkg.dir] = source
		return
	}
	// A retained body or storage definition can be reached through its linked
	// name even when the ordinary local name is private and unused.
	e.linkDefinitions[source.pkg.dir] = append(e.linkDefinitions[source.pkg.dir], source)
}

func (e *externalEffects) linkedObject(object *ast.Object) bool {
	linked, found := e.linknames[object]
	if !found {
		return false
	}
	e.issue(linked, "linkname authority is unsupported", externalUnsupported)
	return true
}

// linkageDefinitions runs only for certain or potentially importable packages.
// A private bodyless alias is checked when read/called/exposed, not here.
func (e *externalEffects) linkageDefinitions(directory string) {
	if source, invalid := e.linkInvalid[directory]; invalid {
		e.issue(source, "linkname declaration identity is unresolved", externalUnsupported)
	}
	for _, source := range e.linkDefinitions[directory] {
		e.issue(source, "linkname definition exposes retained authority", externalUnsupported)
	}
}
