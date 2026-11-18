package yang

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TEST-ONLY ORACLE: the structure check as it ran before AC-15 of
// plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md.
// It walks each module's goyang AST twice, once for the length check and once
// for the enum check, and reads the struct tags of every field of every node
// it meets. The production code no longer holds this shape; it is kept here
// only so the tests below can compare the new walk and the joined refusal
// text with what the old code produced. The leaf checks it calls
// (lengthOrderError, assignEnumValues, enumRestrictionErrors) are the
// production ones: AC-15 changed the walk and the composition, not them.
// ---------------------------------------------------------------------------

// oracleCheckStructure is checkStructure before AC-15.
func oracleCheckStructure(l *Loader) error {
	mods := l.modules.sourceModules()
	errs := make([]error, 0, len(mods))
	for _, mod := range mods {
		errs = append(errs, moduleIncludeErrors(mod)...)
		errs = append(errs, oracleLengthErrors(mod)...)
		errs = append(errs, oracleEnumErrors(mod)...)
		errs = append(errs, moduleExtensionSubstatementErrors(l.modules.source(mod))...)
	}
	return errors.Join(errs...)
}

// oracleLengthErrors is moduleLengthErrors before AC-15, walking mod itself.
func oracleLengthErrors(mod *yang.Module) []error {
	var errs []error
	for _, typ := range oracleTypeNodes(mod) {
		if typ.Length == nil {
			continue
		}
		err := lengthOrderError(typ)
		if err == nil {
			continue
		}
		errs = append(errs, fmt.Errorf("%w: module %s: %s: length %q: %w",
			ErrLengthOrder, mod.Name, yang.Source(typ.Length), typ.Length.Name, err))
	}
	return errs
}

// oracleEnumErrors is moduleEnumErrors before AC-15, walking mod itself.
func oracleEnumErrors(mod *yang.Module) []error {
	var errs []error
	for _, typ := range oracleTypeNodes(mod) {
		if len(typ.Enum) == 0 {
			continue
		}
		if typ.Name == enumerationType {
			if _, err := assignEnumValues(typ); err != nil {
				errs = append(errs, fmt.Errorf("module %s: %w", mod.Name, err))
			}
			continue
		}
		if typ.YangType == nil {
			continue
		}
		if typ.YangType.Kind != yang.Yenum {
			continue
		}
		errs = append(errs, enumRestrictionErrors(mod.Name, typ)...)
	}
	return errs
}

// oracleTypeNodes is moduleTypeNodes before AC-15.
func oracleTypeNodes(mod *yang.Module) []*yang.Type {
	var types []*yang.Type
	for _, node := range oracleWalk(mod) {
		if typ, isType := reflect.TypeAssert[*yang.Type](node); isType {
			types = append(types, typ)
		}
	}
	return types
}

// oracleWalk answers every non-nil node of mod's goyang AST in the order the
// walk before AC-15 met them: an explicit stack, the struct tags of every
// field of every node read as the node is met (isSubstatementField).
func oracleWalk(mod *yang.Module) []reflect.Value {
	var met []reflect.Value
	pending := []reflect.Value{reflect.ValueOf(mod)}
	for len(pending) > 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if node.IsNil() {
			continue
		}
		met = append(met, node)
		fields := node.Elem().Type()
		for i := range fields.NumField() {
			if !isSubstatementField(fields.Field(i)) {
				continue
			}
			value := node.Elem().Field(i)
			// goyang's substatement fields are pointers or slices of pointers; nothing else holds a node
			//exhaustive:ignore
			switch value.Kind() {
			case reflect.Pointer:
				pending = append(pending, value)
			case reflect.Slice:
				for j := range value.Len() {
					pending = append(pending, value.Index(j))
				}
			}
		}
	}
	return met
}

// ---------------------------------------------------------------------------
// End of the test-only oracle.
// ---------------------------------------------------------------------------

// structureFaultTexts is a module set that together triggers every refusal
// checkStructure can be driven to by module text, several in one module so
// the order inside a module is compared too: lens holds length and enum
// restriction faults interleaved in the tree, so the old order (every length
// refusal, then every enum refusal) is pinned; vals holds the two enum value
// faults; exts holds one statement of each extension substatement fault,
// each statement breaking one rule only, since extensionSubstatementError
// reports the first fault of a map walk; a and c-s include s, a submodule of
// b.
var structureFaultTexts = []string{
	`module lens { namespace "urn:lens"; prefix l;
		typedef e { type enumeration { enum x { value 1; } enum y { value 2; } } }
		typedef s { type string { length "2..10"; } }
		leaf a { type e { enum x { value 5; } } }
		leaf b { type string { length "1..5 | 3..8"; } }
		leaf c { type e { enum z; } }
		leaf d { type string { length "10..20 | 1..5"; } }
		leaf f { type s { length "min | 2"; } }
		grouping unused { leaf u { type s { length "10 | max"; } } }
	}`,
	`module vals { namespace "urn:vals"; prefix v;
		leaf g { type enumeration { enum p { value 2147483647; } enum q; } }
		leaf h { type enumeration { enum p { value -5; } enum q; enum r { value -4; } } }
	}`,
	`module exts { namespace "urn:exts"; prefix x; extension e { argument text; }
		leaf a { type string;
			x:e "1" { bogus "y"; }
			x:e "2" { description; }
			x:e "3" { config "maybe"; }
			x:e "4" { description "d" { leaf q { type string; } } }
			x:e "5" { refine "a"; }
			x:e "6" { leaf q { type string; type string; } }
			x:e "7" { leaf q; }
			x:e "8" { type string { length "1"; enum a; } }
			x:e "9" { input "y"; }
		}
	}`,
	`module a { namespace "urn:a"; prefix a; include s; }`,
	`submodule s { belongs-to b { prefix b; } leaf s { type string; } }`,
	`module b { namespace "urn:b"; prefix b; include s; }`,
	`module c { namespace "urn:c"; prefix c; include c-s; }`,
	`submodule c-s { belongs-to c { prefix c; } include s; }`,
}

// faultLoader answers a loader holding structureFaultTexts after Resolve
// refused them, so its set is processed as checkStructure sees it there.
func faultLoader(t *testing.T) *Loader {
	t.Helper()
	loader := NewLoader()
	for i, text := range structureFaultTexts {
		require.NoError(t, loader.AddModuleFromText(moduleFileName(text, i), text))
	}
	_, resolveErr := loader.Resolve()
	require.Error(t, resolveErr, "the fault set must be refused")
	return loader
}

// registeredLoader answers a loader over the module set DefaultLoader
// resolved for this test binary, with no module judged, so checkStructure
// reads every one of them.
func registeredLoader(t *testing.T) *Loader {
	t.Helper()
	resolved, err := DefaultLoader()
	require.NoError(t, err)
	loader := &Loader{modules: resolved.set()}
	require.Empty(t, loader.modules.judged, "DefaultLoader's set judges every module itself")
	return loader
}

// requireSameTypeNodes requires the new walk and the oracle to answer the same
// `type` nodes of each module of loader, the same pointers in the same order.
func requireSameTypeNodes(t *testing.T, loader *Loader) {
	t.Helper()
	mods := loader.modules.sourceModules()
	require.NotEmpty(t, mods)
	for _, mod := range mods {
		want, got := oracleTypeNodes(mod), moduleTypeNodes(mod)
		assert.True(t, slices.Equal(want, got), "type nodes of %s: the oracle meets %d, the walk %d, or another order",
			mod.Name, len(want), len(got))
	}
}

// TestCheckStructureMatchesThePerNodeWalk proves the one-walk checkStructure
// refuses exactly what the two-walk code before it refused, in the same
// order, byte for byte. Method: the fault set is resolved (Resolve refuses
// it, leaving the processed set on the loader), then checkStructure and
// oracleCheckStructure, the old code above, judge the same loader. The joined
// texts must be equal, and must carry every sentinel and each extension
// fault, so the set is known to reach every refusal. Each module's `type`
// nodes must also come from both walks as the same pointers in the same order.
//
// VALIDATES: AC-15, every refusal keeps its sentinel, text, location and order.
func TestCheckStructureMatchesThePerNodeWalk(t *testing.T) {
	loader := faultLoader(t)
	got := loader.checkStructure()
	want := oracleCheckStructure(loader)
	require.Error(t, got)
	require.Error(t, want)
	assert.Equal(t, want.Error(), got.Error(), "joined refusal text")

	for _, sentinel := range []error{ErrIncludeBelongsTo, ErrLengthOrder, ErrEnumValue, ErrEnumRestriction, ErrExtensionSubstatement} {
		assert.ErrorIs(t, got, sentinel, "the fault set must reach %v", sentinel)
	}
	for _, fault := range []string{
		`"bogus" is not a YANG statement`, "description requires an argument", "maybe",
		"leaf is not a substatement of description", "refine-stmt requires a block",
		"type appears 2 times in leaf", "leaf requires at least 1 type", "do not match type-stmt",
		"input takes no argument",
	} {
		assert.Contains(t, got.Error(), fault)
	}
	requireSameTypeNodes(t, loader)
}

// TestCheckStructureMatchesThePerNodeWalkOnRegisteredModules proves the same
// equivalence over the modules this test binary registers, which every check
// passed: both answer no refusal, and both walks the same `type` nodes, in
// the same order, for every module.
//
// VALIDATES: AC-15 over real modules.
func TestCheckStructureMatchesThePerNodeWalkOnRegisteredModules(t *testing.T) {
	loader := registeredLoader(t)
	assert.NoError(t, oracleCheckStructure(loader))
	assert.NoError(t, loader.checkStructure())
	requireSameTypeNodes(t, loader)
}

// declBody is the body of one top-level function, or the initializer of one
// package variable, with the name that owns it.
type declBody struct {
	owner string
	node  ast.Node
}

// productionCallers answers, for each function named in names, the functions
// and package variables of this package's non-test files whose bodies call
// it, sorted, one entry per call site. A walk count per module rests on call
// sites: a call in a function that runs once per module runs once per module.
func productionCallers(t *testing.T, names ...string) map[string][]string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	callers := map[string][]string{}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
		require.NoError(t, err, file)
		for _, body := range declBodies(parsed) {
			ast.Inspect(body.node, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall {
					return true
				}
				name := calledName(call)
				if slices.Contains(names, name) {
					callers[name] = append(callers[name], body.owner)
				}
				return true
			})
		}
	}
	for _, list := range callers {
		slices.Sort(list)
	}
	return callers
}

// declBodies answers the function bodies and variable initializers of file.
func declBodies(file *ast.File) []declBody {
	var bodies []declBody
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Body != nil {
				bodies = append(bodies, declBody{owner: d.Name.Name, node: d.Body})
			}
		case *ast.GenDecl:
			bodies = append(bodies, varInitializers(d)...)
		}
	}
	return bodies
}

// varInitializers answers the initializer of each package variable decl
// declares, none when decl declares no variable.
func varInitializers(decl *ast.GenDecl) []declBody {
	if decl.Tok != token.VAR {
		return nil
	}
	var bodies []declBody
	for _, spec := range decl.Specs {
		value, isValue := spec.(*ast.ValueSpec)
		if !isValue {
			continue
		}
		for i, name := range value.Names {
			if i < len(value.Values) {
				bodies = append(bodies, declBody{owner: name.Name, node: value.Values[i]})
			}
		}
	}
	return bodies
}

// calledName answers the name a call expression calls, a plain function or a
// selected one ("x.f" answers "f"), or "" for any other callee.
func calledName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

// TestStructureChecksWalkEachModuleOnce proves checkStructure walks each
// module's goyang AST once for both the length and the enum check. Method:
// the call sites of the production source. The walker (moduleTypeNodes) has
// one caller, newModuleTypes; newModuleTypes has one, moduleStructureErrors;
// and moduleStructureErrors has one, checkStructure, which calls it once per
// module. The length and enum checks are methods of the walk's result
// (moduleTypes), which holds no module, so neither can walk again. Before
// AC-15 the walker had two callers, moduleLengthErrors and moduleEnumErrors,
// and this test was red.
//
// VALIDATES: AC-15, one walk per module serves both checks.
func TestStructureChecksWalkEachModuleOnce(t *testing.T) {
	callers := productionCallers(t, "moduleTypeNodes", "newModuleTypes", "moduleStructureErrors")
	assert.Equal(t, []string{"newModuleTypes"}, callers["moduleTypeNodes"], "callers of the walker")
	assert.Equal(t, []string{"moduleStructureErrors"}, callers["newModuleTypes"], "callers of newModuleTypes")
	assert.Equal(t, []string{"checkStructure"}, callers["moduleStructureErrors"], "callers of moduleStructureErrors")
}

// TestSubstatementFieldsAreReadOncePerType proves the struct tags of a goyang
// node type are read once per type, not once per node. Method: the tags are
// read by isSubstatementField alone, whose one production caller is the
// table builder, whose one caller is the sync.OnceValue behind
// substatementFieldTable, and two calls of substatementFieldTable answer the
// same map. The table is then checked against the per-field rule (it holds
// every substatement field of a type and no other), against its bound (every
// entry a struct type of goyang's yang package), and against the walk: every
// node type the old walk meets over the registered modules and the fault set
// has an entry, so the walk never meets a type the table lacks. Before AC-15
// isSubstatementField was called per node by appendSubstatementNodes, and
// this test was red.
//
// VALIDATES: AC-15, substatement fields computed once per type, and the table's bound.
func TestSubstatementFieldsAreReadOncePerType(t *testing.T) {
	callers := productionCallers(t, "isSubstatementField", "buildSubstatementFields")
	assert.Equal(t, []string{"buildSubstatementFields"}, callers["isSubstatementField"], "callers of the tag reader")
	assert.Equal(t, []string{"substatementFieldTable"}, callers["buildSubstatementFields"], "callers of the table builder")

	first, second := substatementFieldTable(), substatementFieldTable()
	assert.Equal(t, reflect.ValueOf(first).UnsafePointer(), reflect.ValueOf(second).UnsafePointer(), "one table per process")

	goyang := reflect.TypeFor[yang.Module]().PkgPath()
	for node, indexes := range first {
		require.Equal(t, reflect.Struct, node.Kind(), "%v", node)
		assert.Equal(t, goyang, node.PkgPath(), "%v is no goyang node type", node)
		var want []int
		for i := range node.NumField() {
			if isSubstatementField(node.Field(i)) {
				want = append(want, i)
			}
		}
		assert.Equal(t, want, indexes, "substatement fields of %v", node)
	}

	mods := registeredLoader(t).modules.sourceModules()
	mods = append(mods, faultLoader(t).modules.sourceModules()...)
	met := map[reflect.Type]bool{}
	for _, mod := range mods {
		for _, node := range oracleWalk(mod) {
			met[node.Elem().Type()] = true
		}
	}
	require.NotEmpty(t, met)
	for node := range met {
		_, held := first[node]
		assert.True(t, held, "the walk meets %v, which the table lacks", node)
	}
}
