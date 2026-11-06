// Design: docs/architecture/plugin/feature-gates.md -- membership is reachable registration, not literal census.
// Related: expressions.go -- exact metadata expressions; source.go -- target build population.
package repoinventory

import (
	"fmt"
	"go/ast"
	"path"
	"slices"
	"strings"
)

const registrationPackage = "internal/component/plugin/registry"

// registeredPlugin retains report-only metadata that Plugin's published shape
// does not carry, without dropping the family support fields of Inventory.
type registeredPlugin struct {
	plugin        Plugin
	yang          string
	routeEncoder  bool
	configBuilder bool
}

type registrations struct {
	eval             *expression
	active           map[*ast.FuncDecl]bool
	visited          map[*ast.FuncDecl]bool
	plugins          map[string]registeredPlugin
	modules          map[string]string
	moduleIdentities map[string]bool
	assembly         *assemblyIndex
	external         map[externalCallSite]bool
	diagnosticOnly   bool
}

// initializationScope distinguishes a function's lexical block from a package
// initializer expression, which has no enclosing local block.
type initializationScope struct {
	file *ast.File
	pkg  *sourcePackage
	body *ast.BlockStmt
}

// Plugins resolves every init-reachable registration in this committed build.
// It returns no partial list when a declaration or its ownership is unresolved.
func (s *Snapshot) Plugins() ([]Plugin, error) {
	registrations, err := s.registrations()
	if err != nil {
		return nil, err
	}
	result := make([]Plugin, 0, len(registrations))
	for _, registration := range registrations {
		result = append(result, registration.plugin)
	}
	return result, nil
}

func (s *Snapshot) registrations() ([]registeredPlugin, error) {
	reader, err := s.readRegistrations()
	if err != nil {
		return nil, err
	}
	result := make([]registeredPlugin, 0, len(reader.plugins))
	for _, registration := range reader.plugins {
		result = append(result, registration)
	}
	slices.SortFunc(result, func(a, b registeredPlugin) int { return strings.Compare(a.plugin.Name, b.plugin.Name) })
	return result, nil
}

func (s *Snapshot) readRegistrations() (*registrations, error) {
	certificate, err := s.externalCertificate()
	if err != nil {
		return nil, err
	}
	reader := &registrations{eval: s.evaluator(), active: make(map[*ast.FuncDecl]bool), visited: make(map[*ast.FuncDecl]bool), plugins: make(map[string]registeredPlugin), modules: make(map[string]string)}
	reader.external = certificate.sites
	reader.diagnosticOnly = certificate.refusal != nil
	reader.moduleIdentities = make(map[string]bool)
	packages := make([]string, 0, len(s.packages))
	for dir := range s.packages {
		packages = append(packages, dir)
	}
	slices.Sort(packages)
	for _, dir := range packages {
		for _, initializer := range s.packages[dir].initializerValues {
			scope := initializationScope{file: initializer.file, pkg: initializer.pkg}
			if err := reader.node(initializer.value, scope, false); err != nil {
				return nil, err
			}
		}
		for _, initializer := range s.packages[dir].initializers {
			if err := reader.function(initializer, false); err != nil {
				return nil, err
			}
		}
	}
	// Diagnostic traversal never upgrades an incomplete effect proof. Even a
	// complete-looking private population must be discarded on this path.
	if certificate.refusal != nil {
		return nil, certificate.refusal
	}
	if len(reader.plugins) == 0 {
		return nil, fmt.Errorf("committed Linux le at %s registers no plugins; commit its composition imports before publishing inventory", s.revision)
	}
	return reader, nil
}

// function follows the finite init call graph, never a callback merely passed
// as a value. A registration reached under runtime control is not a known member.
func (r *registrations) function(function sourceFunction, conditional bool) error {
	if r.active[function.decl] {
		return nil
	}
	if !conditional && r.visited[function.decl] {
		return nil
	}
	if len(r.active) == 128 {
		return fmt.Errorf("committed initialization exceeds 128 call steps at %s.%s; simplify registration initialization", function.pkg.dir, function.decl.Name.Name)
	}
	r.active[function.decl] = true
	defer delete(r.active, function.decl)
	if !conditional {
		r.visited[function.decl] = true
	}
	scope := initializationScope{file: function.file, pkg: function.pkg, body: function.decl.Body}
	return r.node(function.decl.Body, scope, conditional)
}

func (r *registrations) node(node ast.Node, function initializationScope, conditional bool) error {
	if node == nil {
		return nil
	}
	switch current := node.(type) {
	case *ast.FuncLit:
		return nil
	case *ast.IfStmt:
		if err := r.node(current.Init, function, conditional); err != nil {
			return err
		}
		if err := r.node(current.Cond, function, conditional); err != nil {
			return err
		}
		if err := r.node(current.Body, function, true); err != nil {
			return err
		}
		return r.node(current.Else, function, true)
	case *ast.ForStmt:
		if err := r.node(current.Init, function, conditional); err != nil {
			return err
		}
		if err := r.node(current.Cond, function, conditional); err != nil {
			return err
		}
		if err := r.node(current.Post, function, true); err != nil {
			return err
		}
		return r.node(current.Body, function, true)
	case *ast.RangeStmt:
		if err := r.node(current.X, function, conditional); err != nil {
			return err
		}
		return r.node(current.Body, function, true)
	case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return r.children(node, function, true)
	case *ast.CallExpr:
		if r.external[externalSite(r.eval.snapshot, current)] {
			if err := r.node(current.Fun, function, conditional); err != nil {
				return err
			}
			for _, argument := range current.Args {
				if err := r.node(argument, function, conditional); err != nil {
					return err
				}
			}
			return nil
		}
		// Execution MUST consume this normalized source and its declaration
		// classification, never dispatch the original alias/parentheses again.
		callee, declared, err := r.callableSource(current, function)
		if err != nil {
			if r.diagnosticOnly {
				// The saved unresolved proof already forbids publication. Keep
				// evaluating operands to reach a more specific retained-source
				// refusal, without inventing an owner for this invocation.
				if err := r.node(current.Fun, function, conditional); err != nil {
					return err
				}
				for _, argument := range current.Args {
					if err := r.node(argument, function, conditional); err != nil {
						return err
					}
				}
				return nil
			}
			return err
		}
		for _, argument := range current.Args {
			if err := r.node(argument, function, conditional); err != nil {
				return err
			}
		}
		if closure, immediate := callee.value.(*ast.FuncLit); immediate {
			return r.node(closure.Body, initializationScope{file: callee.file, pkg: callee.pkg, body: closure.Body}, conditional)
		}
		if declared && r.moduleCall(callee) {
			if conditional {
				return r.eval.problem(sourceExpr{value: current, file: function.file, pkg: function.pkg}, "YANG registration depends on an unresolved condition or iteration")
			}
			return r.registerModule(current, function)
		}
		if declared && r.registryCall(callee) {
			if conditional {
				return r.eval.problem(sourceExpr{value: current, file: function.file, pkg: function.pkg}, "plugin registration depends on an unresolved condition or iteration")
			}
			return r.register(current, function)
		}
		if declared {
			called, found := r.eval.function(callee)
			if !found {
				return r.eval.problem(sourceExpr{value: current, file: function.file, pkg: function.pkg}, "callable declaration has unresolved execution ownership")
			}
			if called.decl.Body == nil {
				if r.assembly == nil {
					evidence, err := r.eval.snapshot.assemblyEvidence()
					if err != nil {
						return r.eval.problem(sourceExpr{value: current, file: function.file, pkg: function.pkg}, "initialization calls "+called.pkg.dir+"."+called.decl.Name.Name+" without a proved retained Go or assembly body: "+err.Error())
					}
					r.assembly = evidence
				}
				if err := r.eval.snapshot.certifyAssembly(called, r.assembly); err != nil {
					return r.eval.problem(sourceExpr{value: current, file: function.file, pkg: function.pkg}, "initialization calls "+called.pkg.dir+"."+called.decl.Name.Name+" without a proved retained Go or assembly body: "+err.Error())
				}
			} else if err := r.function(called, conditional); err != nil {
				return err
			}
		}
		return nil
	}
	return r.children(node, function, conditional)
}

// children walks immediate children only; node owns control-flow handling and
// closure exclusion, so nesting cannot accidentally turn a callback into init.
func (r *registrations) children(node ast.Node, function initializationScope, conditional bool) error {
	var result error
	first := true
	ast.Inspect(node, func(child ast.Node) bool {
		if first {
			first = false
			return true
		}
		if child == nil || result != nil {
			return false
		}
		result = r.node(child, function, conditional)
		return false
	})
	return result
}

func (r *registrations) registryCall(callee sourceExpr) bool {
	if name, unqualified := callee.value.(*ast.Ident); unqualified {
		return callee.pkg.dir == registrationPackage && name.Name == "Register"
	}
	selector, ok := callee.value.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if selector.Sel.Name != "Register" {
		return false
	}
	directory, imported := r.eval.callableImport(callee, selector)
	if !imported {
		return false
	}
	return directory == registrationPackage
}

func (r *registrations) moduleCall(callee sourceExpr) bool {
	if name, unqualified := callee.value.(*ast.Ident); unqualified {
		return callee.pkg.dir == "internal/component/config/yang" && name.Name == "RegisterModule"
	}
	selector, ok := callee.value.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if selector.Sel.Name != "RegisterModule" {
		return false
	}
	directory, imported := r.eval.callableImport(callee, selector)
	if !imported {
		return false
	}
	return directory == "internal/component/config/yang"
}

func (r *registrations) registerModule(call *ast.CallExpr, function initializationScope) error {
	source := sourceExpr{value: call, file: function.file, pkg: function.pkg}
	if len(call.Args) != 2 {
		return r.eval.problem(source, "yang.RegisterModule requires name and content")
	}
	name, err := r.eval.text(sourceExpr{value: call.Args[0], file: function.file, pkg: function.pkg})
	if err != nil {
		return err
	}
	// The module's directory comes from the walk that resolved its content,
	// never from a search of committed .yang files by module name: that
	// search also finds test copies of a module, such as those under
	// internal/le/yang/migration/testdata.
	document, origin, err := r.eval.textFrom(sourceExpr{value: call.Args[1], file: function.file, pkg: function.pkg})
	if err != nil {
		return err
	}
	if yangModuleName(document) == "" {
		return r.eval.problem(source, "registered YANG has no module identity")
	}
	if _, duplicate := r.modules[name]; duplicate {
		return r.eval.problem(source, "multiple reachable YANG registrations claim "+name)
	}
	var directory string
	switch origin.kind {
	case textOriginEmbed:
		// The embedded file is the module's committed source.
		directory = path.Dir(origin.file)
	case textOriginGo:
		// A Go string names no committed file, so the module belongs to the
		// package whose registration makes it one.
		directory = function.pkg.dir
	case textOriginMixed:
		return r.eval.problem(source, "registered YANG content joins an embedded file with other text, so no committed file is its source: register the //go:embed variable alone")
	case textOriginUnspecified:
		return r.eval.problem(source, "registered YANG content has no resolved origin")
	}
	r.modules[name] = directory
	r.moduleIdentities[yangModuleName(document)] = true
	return nil
}

func (r *registrations) register(call *ast.CallExpr, function initializationScope) error {
	if len(call.Args) != 1 {
		return r.eval.problem(sourceExpr{value: call, file: function.file, pkg: function.pkg}, "registry.Register requires one declaration")
	}
	source := sourceExpr{value: call.Args[0], file: function.file, pkg: function.pkg}
	literal, err := r.literal(source, function)
	if err != nil {
		return err
	}
	registration, err := r.metadata(literal)
	if err != nil {
		return err
	}
	if _, duplicate := r.plugins[registration.plugin.Name]; duplicate {
		return r.eval.problem(source, "multiple reachable registrations claim plugin "+registration.plugin.Name)
	}
	r.plugins[registration.plugin.Name] = registration
	return nil
}

func (r *registrations) literal(source sourceExpr, function initializationScope) (sourceExpr, error) {
	for range 64 {
		if _, ok := source.value.(*ast.CompositeLit); ok {
			return source, nil
		}
		if name, ok := source.value.(*ast.Ident); ok {
			if err := r.refuseMutation(name, function); err != nil {
				return sourceExpr{}, err
			}
		}
		resolved, err := r.eval.target(source)
		if err != nil {
			return sourceExpr{}, err
		}
		source = resolved
	}
	return sourceExpr{}, r.eval.problem(source, "registration declaration exceeds 64 steps")
}

func (r *registrations) refuseMutation(name *ast.Ident, function initializationScope) error {
	var result error
	if function.body != nil {
		ast.Inspect(function.body, func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, left := range assignment.Lhs {
				field, ok := left.(*ast.SelectorExpr)
				if !ok || !metadataField(field.Sel.Name) {
					continue
				}
				owner, ok := field.X.(*ast.Ident)
				if ok && owner.Obj == name.Obj && owner.Name == name.Name {
					result = r.eval.problem(sourceExpr{value: left, file: function.file, pkg: function.pkg}, "registration metadata is mutated after declaration")
				}
			}
			return result == nil
		})
	}
	if result != nil {
		return result
	}
	source := sourceExpr{value: name, file: function.file, pkg: function.pkg}
	reference, found, _ := r.eval.reference(source)
	if !found {
		return r.eval.problem(source, "registration declaration has ambiguous mutation ownership")
	}
	return r.eval.writes(reference, registrationFieldWrites)
}

func metadataField(name string) bool {
	switch name {
	case "Name", "Description", "Families", "CapabilityCodes", "Dependencies", "OptionalDependencies", "StartAfter", "ConfigRoots", "RFCs", "Features", "YANG", "RunEngine", "InProcessNLRIDecoder", "InProcessNLRIEncoder", "InProcessRouteEncoder", "InProcessConfigNLRIBuilder":
		return true
	}
	return false
}

func (r *registrations) metadata(source sourceExpr) (registeredPlugin, error) {
	literal, ok := source.value.(*ast.CompositeLit)
	if !ok {
		return registeredPlugin{}, r.eval.problem(source, "registration is not a literal")
	}
	registration := registeredPlugin{}
	seen := make(map[string]bool)
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			return registeredPlugin{}, r.eval.problem(source, "registration uses positional metadata")
		}
		key, ok := pair.Key.(*ast.Ident)
		if !ok {
			return registeredPlugin{}, r.eval.problem(source, "registration field is not named")
		}
		if seen[key.Name] {
			return registeredPlugin{}, r.eval.problem(source, "registration repeats field "+key.Name)
		}
		seen[key.Name] = true
		value := sourceExpr{value: pair.Value, file: source.file, pkg: source.pkg}
		if err := r.field(&registration, key.Name, value); err != nil {
			return registeredPlugin{}, err
		}
	}
	if registration.plugin.Name == "" {
		return registeredPlugin{}, r.eval.problem(source, "plugin name is empty")
	}
	if registration.plugin.SourceDir == "" {
		return registeredPlugin{}, r.eval.problem(source, "plugin engine has no resolved defining package")
	}
	files, err := r.yangFiles(registration.plugin.SourceDir, registration.yang)
	if err != nil {
		return registeredPlugin{}, err
	}
	registration.plugin.YANGFiles = files
	registration.plugin.HasYANG = registration.yang != ""
	return registration, nil
}

func (r *registrations) field(registration *registeredPlugin, field string, source sourceExpr) error {
	p := &registration.plugin
	var err error
	switch field {
	case "Name":
		p.Name, err = r.eval.text(source)
	case "Description":
		p.Description, err = r.eval.text(source)
	case "Features":
		p.Features, err = r.eval.text(source)
	case "YANG":
		registration.yang, err = r.eval.text(source)
	case "Families":
		p.Families, err = r.eval.texts(source)
	case "Dependencies":
		p.Dependencies, err = r.eval.texts(source)
	case "OptionalDependencies":
		p.OptionalDependencies, err = r.eval.texts(source)
	case "StartAfter":
		p.StartAfter, err = r.eval.texts(source)
	case "ConfigRoots":
		p.ConfigRoots, err = r.eval.texts(source)
	case "RFCs":
		p.RFCs, err = r.eval.texts(source)
	case "CapabilityCodes":
		p.Capabilities, err = r.eval.octets(source)
	case "RunEngine":
		p.SourceDir, err = r.engineSource(source)
	case "InProcessNLRIDecoder":
		p.HasDecoder, err = r.functionPresent(source)
	case "InProcessNLRIEncoder":
		p.HasEncoder, err = r.functionPresent(source)
	case "InProcessRouteEncoder":
		registration.routeEncoder, err = r.functionPresent(source)
	case "InProcessConfigNLRIBuilder":
		registration.configBuilder, err = r.functionPresent(source)
	}
	return err
}

func (r *registrations) engineSource(source sourceExpr) (string, error) {
	if _, closure := source.value.(*ast.FuncLit); closure {
		return source.pkg.dir, nil
	}
	function, found := r.eval.function(source)
	if found {
		return function.pkg.dir, nil
	}
	return "", r.eval.problem(source, "RunEngine has an unsupported or ambiguous defining package")
}

func (r *registrations) functionPresent(source sourceExpr) (bool, error) {
	if predeclaredIdentifier(source, "nil") {
		return false, nil
	}
	_, err := r.engineSource(source)
	return err == nil, err
}

func (r *registrations) yangFiles(source, document string) ([]string, error) {
	directory := path.Join(source, yangDirectory)
	if document != "" {
		module := yangModuleName(document)
		if module == "" {
			return nil, fmt.Errorf("plugin schema from %s has no module identity; correct its committed YANG declaration", source)
		}
		found := ""
		for _, name := range r.eval.snapshot.paths {
			if !strings.HasSuffix(name, yangSuffix) {
				continue
			}
			if yangModuleName(string(r.eval.snapshot.blobs[name])) != module {
				continue
			}
			if found != "" {
				return nil, fmt.Errorf("committed schema %s has ambiguous sources %s and %s; give each module one source", module, found, name)
			}
			found = name
		}
		if found == "" {
			return nil, fmt.Errorf("committed schema %s from %s has no source blob; commit its YANG file", module, source)
		}
		directory = path.Dir(found)
	}
	var result []string
	for _, name := range r.eval.snapshot.directories[directory] {
		if strings.HasSuffix(name, yangSuffix) {
			result = append(result, name)
		}
	}
	slices.Sort(result)
	return result, nil
}
