// Design: docs/architecture/plugin/feature-gates.md -- certified entries retain initializer closure.
package repoinventory

import "go/ast"

func (e *externalEffects) staticRoots() {
	if e.amb.unknown != "" {
		return
	}
	for identity := range e.amb.origins {
		if e.amb.unknown != "" {
			return
		}
		entries, err := e.source.entrySet(identity)
		if err != nil {
			e.problem(sourceExpr{}, err.Error())
			continue
		}
		for _, origin := range entries.origins {
			if e.amb.join(externalOrigin(origin)) {
				e.changed = true
			}
		}
		for _, directory := range entries.packages {
			if e.amb.unknown != "" {
				return
			}
			e.staticExports(directory)
		}
		for _, directory := range entries.initializers {
			if e.amb.unknown != "" {
				return
			}
			e.possibleInitializers(directory)
		}
	}
}

// staticExports exposes only certified entries. Internal dependencies retain
// their bodies and writes without acquiring independent public API authority.
// Permanent ambient refusal stops before another exported value/type is read.
func (e *externalEffects) staticExports(directory string) {
	if e.amb.unknown != "" {
		return
	}
	pkg := e.source.snapshot.packages[directory]
	e.linkageDefinitions(directory)
	for _, file := range pkg.files {
		for name, object := range file.File.Scope.Objects {
			if e.amb.unknown != "" {
				return
			}
			if !ast.IsExported(name) {
				continue
			}
			source := sourceExpr{value: ast.NewIdent(name), file: file.File, pkg: pkg}
			switch object.Kind {
			case ast.Con:
				e.expose(e.readObject(object))
			case ast.Var:
				e.expose(externalLocation(object))
			case ast.Fun:
				function, found := pkg.functions[name]
				if !found {
					e.problem(source, "static function ownership is unresolved")
					continue
				}
				e.expose(externalCallable(function.decl, source))
			case ast.Typ:
				e.expose(e.typeMethods(source, 0))
			}
		}
	}
}

// possibleInitializers receives only packages absent from the certain root
// population. Their entire retained import closure is supplied by entrySet.
func (e *externalEffects) possibleInitializers(directory string) {
	if e.amb.unknown != "" {
		return
	}
	pkg := e.source.snapshot.packages[directory]
	e.linkageDefinitions(directory)
	for _, initializer := range pkg.initializers {
		if e.amb.unknown != "" {
			return
		}
		e.body(initializer.decl, sourceExpr{file: initializer.file, pkg: pkg}, true)
	}
	for _, initializer := range pkg.initializerValues {
		if e.amb.unknown != "" {
			return
		}
		e.expose(e.value(initializer, true, 0))
	}
}
