package registry

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// sealForTest seals the registry for one test and restores the state before
// it (seal included) when the test ends, so the next test can register.
func sealForTest(t *testing.T) {
	t.Helper()
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	Seal()
}

// validRegistration is a registration Register accepts.
func validRegistration(name string) Registration {
	return Registration{
		Name:        name,
		Description: "seal test plugin",
		Families:    []string{"ipv4/seal-" + name},
		FilterTypes: []string{"seal-filter-" + name},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}
}

// registryView is what a refused registration must leave unchanged: the
// names, the families and the filter types the registry answers.
type registryView struct {
	names       []string
	families    map[string]string
	filterTypes map[string]string
}

func viewRegistry() registryView {
	return registryView{names: Names(), families: AllFamilies(), filterTypes: FilterTypesMap()}
}

// TestRegisterAfterSealRefuses checks AC-1. Goal: Register after Seal refuses
// with ErrRegistrySealed, names the plugin and the rule, and changes nothing,
// while before the seal every existing refusal keeps its own error and a
// valid registration lands. Method: register before and after Seal on a
// reset registry and compare what the registry answers.
func TestRegisterAfterSealRefuses(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	Reset()

	missingEngine := validRegistration("seal-no-engine")
	missingEngine.RunEngine = nil
	if err := Register(missingEngine); !errors.Is(err, ErrNilRunEngine) {
		t.Fatalf("before the seal: Register without RunEngine = %v, want ErrNilRunEngine", err)
	}
	if err := Register(validRegistration("seal-early")); err != nil {
		t.Fatalf("before the seal: Register = %v, want nil", err)
	}

	before := viewRegistry()
	Seal()
	err := Register(validRegistration("seal-late"))
	if !errors.Is(err, ErrRegistrySealed) {
		t.Fatalf("after the seal: Register = %v, want ErrRegistrySealed", err)
	}
	for _, want := range []string{`"seal-late"`, "package initialization", "register.go"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %s", err, want)
		}
	}
	after := viewRegistry()
	if !reflect.DeepEqual(before, after) {
		t.Errorf("a refused Register changed the registry:\nbefore %+v\nafter  %+v", before, after)
	}
	if PluginForFamily("ipv4/seal-seal-late") != "" {
		t.Error("a refused Register reached the family index")
	}
}

// sealedCall is one registering function called after Seal: call makes the
// call, unchanged answers whether what it would have written still reads as
// before, and returnsError says whether it refuses by error or by panic.
type sealedCall struct {
	call         func() error
	unchanged    func() bool
	returnsError bool
}

// sealCallbacks implements the callback interfaces the seal tests install.
type sealCallbacks struct{}

func (sealCallbacks) DumpRIB(RIBDumpVisitor)                {}
func (sealCallbacks) OnBGPMessage(any, uint8, bool, []byte) {}
func (sealCallbacks) OnPeerEstablished(any)                 {}
func (sealCallbacks) OnPeerClosed(any, string)              {}

// sealedCalls holds one entry per registering function other than Register,
// keyed by its name. TestEveryRegistryWriterIsGuardedOrASeam proves the keys
// are exactly the functions that call the seal guard.
//
// A seam another test of this package installed may already be set, and the
// MRT and RIB setters ignore nil so it cannot be cleared, so each unchanged
// check compares with the value read when the table is built.
func sealedCalls() map[string]sealedCall {
	noError := func(fn func()) func() error { return func() error { fn(); return nil } }
	funcIdentity := func(fn any) uintptr { return reflect.ValueOf(fn).Pointer() }
	reactorFactoryBefore := funcIdentity(GetReactorFactory())
	packetDecoderBefore := funcIdentity(GetPacketDecoder())
	ribDumpBefore := GetRIBDumpCallback()
	mrtMessageBefore := GetMRTMessageCallback()
	mrtPeerBefore := GetMRTPeerCallback()
	return map[string]sealedCall{
		"AddConfigRead": {
			call:         func() error { return AddConfigRead("seal-reader", "seal-root") },
			unchanged:    func() bool { return !slices.Contains(Lookup("seal-reader").ConfigReads, "seal-root") },
			returnsError: true,
		},
		"RegisterBuiltinFamilies": {
			call:      noError(func() { RegisterBuiltinFamilies("seal-source", []string{"ipv4/seal-builtin"}) }),
			unchanged: func() bool { _, found := AllFamilies()["ipv4/seal-builtin"]; return !found },
		},
		"AddRPCHandlers": {
			call: noError(func() {
				AddRPCHandlers(map[string]func(json.RawMessage) (any, error){"seal-method": nil})
			}),
			unchanged: func() bool { _, found := CollectRPCHandlers()["seal-method"]; return !found },
		},
		"RegisterSessionFact": {
			call:      noError(func() { RegisterSessionFact("seal-fact") }),
			unchanged: func() bool { return !SessionFactRegistered("seal-fact") },
		},
		"RegisterDoctorPlatforms": {
			call:      noError(func() { RegisterDoctorPlatforms([]string{"seal-platform"}) }),
			unchanged: func() bool { return !validDoctorPlatform("seal-platform") },
		},
		"RecordSetup": {
			call: noError(func() { RecordSetup("seal-reader", SetupSucceeded, "") }),
			unchanged: func() bool {
				return !slices.ContainsFunc(SetupResults(), func(r SetupResult) bool { return r.Outcome == SetupSucceeded })
			},
		},
		"RegisterReactorFactory": {
			call: noError(func() {
				RegisterReactorFactory(func(CoordinatorAccessor, map[string]any) (BGPReactorHandle, error) { return nil, nil })
			}),
			unchanged: func() bool { return funcIdentity(GetReactorFactory()) == reactorFactoryBefore },
		},
		"SetPacketDecoder": {
			call:      noError(func() { SetPacketDecoder(func(string, string, string, bool) (string, error) { return "", nil }) }),
			unchanged: func() bool { return funcIdentity(GetPacketDecoder()) == packetDecoderBefore },
		},
		"SetRIBDumpCallback": {
			call:      noError(func() { SetRIBDumpCallback(sealCallbacks{}) }),
			unchanged: func() bool { return GetRIBDumpCallback() == ribDumpBefore },
		},
		"SetMRTMessageCallback": {
			call:      noError(func() { SetMRTMessageCallback(sealCallbacks{}) }),
			unchanged: func() bool { return GetMRTMessageCallback() == mrtMessageBefore },
		},
		"SetMRTPeerCallback": {
			call:      noError(func() { SetMRTPeerCallback(sealCallbacks{}) }),
			unchanged: func() bool { return GetMRTPeerCallback() == mrtPeerBefore },
		},
		"SetFilterDeclarationCheck": {
			call: noError(func() { SetFilterDeclarationCheck(func(string, []DeclaredFilter) error { return nil }) }),
			unchanged: func() bool {
				// The guard runs before the swap, so the judge installed
				// before the seal, the one that refuses, is still installed.
				judge := filterDeclarations.check
				return judge != nil && judge("", nil) != nil
			},
		},
	}
}

// TestRegistryMutatorsAfterSealRefuse checks AC-2. Goal: every registering
// function other than Register refuses after Seal (AddConfigRead with the
// sealed error naming the plugin and the root, the others with a "BUG:"
// panic naming the function) and writes nothing, while the runtime seams the
// daemon sets during start stay callable. Method: seal a registry holding one
// plugin, call each function, and read back what it would have written.
func TestRegistryMutatorsAfterSealRefuse(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	Reset()
	if err := Register(validRegistration("seal-reader")); err != nil {
		t.Fatalf("Register: %v", err)
	}
	previous := SetFilterDeclarationCheck(func(string, []DeclaredFilter) error { return errors.New("seal judge") })
	t.Cleanup(func() { filterDeclarations.check = previous })
	calls := sealedCalls()
	for name, entry := range calls {
		if !entry.unchanged() {
			t.Fatalf("%s: the state it writes already reads as written before the seal", name)
		}
	}
	Seal()

	for name, entry := range calls {
		t.Run(name, func(t *testing.T) {
			err, panicked := callRecovering(entry.call)
			if entry.returnsError {
				if !errors.Is(err, ErrRegistrySealed) {
					t.Fatalf("%s after Seal = %v, want ErrRegistrySealed", name, err)
				}
				for _, want := range []string{`"seal-reader"`, `"seal-root"`} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("refusal %q does not name %s", err, want)
					}
				}
			}
			if !entry.returnsError {
				text, isText := panicked.(string)
				if !isText {
					t.Fatalf("%s after Seal panicked with %v, want a BUG: message", name, panicked)
				}
				if !strings.HasPrefix(text, "BUG: registry."+name+"(") {
					t.Errorf("panic %q does not name registry.%s and its argument", text, name)
				}
				if !strings.Contains(text, ErrRegistrySealed.Error()) {
					t.Errorf("panic %q does not carry the sealed refusal", text)
				}
			}
			if !entry.unchanged() {
				t.Errorf("%s after Seal wrote what it was refused", name)
			}
		})
	}

	t.Run("runtime seams stay callable", func(t *testing.T) {
		seams := map[string]func(){
			"SetEventBus":             func() { SetEventBus(nil) },
			"SetPluginServer":         func() { SetPluginServer(nil) },
			"SetMetricsRegistry":      func() { SetMetricsRegistry(nil) },
			"InjectPluginMetrics":     func() { InjectPluginMetrics("seal-reader", nil) },
			"SetRuntimeSessionReady":  func() { SetRuntimeSessionReady(nil) },
			"SetRuntimeRegistryNames": func() { SetRuntimeRegistryNames(nil) },
			"SetNTPSyncProvider":      func() { SetNTPSyncProvider(nil) },
		}
		for name, call := range seams {
			if _, panicked := callRecovering(func() error { call(); return nil }); panicked != nil {
				t.Errorf("runtime seam %s panicked after Seal: %v", name, panicked)
			}
		}
	})
}

// callRecovering runs call and answers its error, or what it panicked with.
func callRecovering(call func() error) (err error, panicked any) {
	defer func() { panicked = recover() }()
	return call(), nil
}

// seamMarker is the sentence a runtime seam's doc comment carries.
const seamMarker = "It is a runtime seam, callable after Seal"

// TestEveryRegistryWriterIsGuardedOrASeam derives, from this package's
// source, every exported function that writes package state, and requires
// each to be exactly one of: guarded (it calls refuseSealedLocked or
// panicIfSealedLocked itself), a runtime seam (its doc comment carries
// seamMarker), test-only (its doc says "Only for use in tests"), or Seal. It
// then requires the guarded set to be Register plus the keys of sealedCalls,
// so a new registering function fails here until it has a sealed-call case.
// Method: go/parser over the non-test files; a write is an assignment, an
// increment, or a clear, delete or maps.Copy whose target is rooted at a
// package-level variable, followed through calls to package functions.
func TestEveryRegistryWriterIsGuardedOrASeam(t *testing.T) {
	files, packageVars := parseRegistrySource(t)
	funcs := map[string]*ast.FuncDecl{}
	for _, file := range files {
		for _, decl := range file.Decls {
			if fn, isFunc := decl.(*ast.FuncDecl); isFunc && fn.Recv == nil {
				funcs[fn.Name.Name] = fn
			}
		}
	}
	writes := writingFuncs(funcs, packageVars)

	guarded := map[string]bool{}
	for name, fn := range funcs {
		if !ast.IsExported(name) || !writes[name] {
			continue
		}
		isGuarded := callsAny(fn, "refuseSealedLocked", "panicIfSealedLocked")
		doc := fn.Doc.Text()
		isSeam := strings.Contains(strings.Join(strings.Fields(doc), " "), seamMarker)
		testOnly := strings.Contains(doc, "Only for use in tests")
		kinds := 0
		for _, holds := range []bool{isGuarded, isSeam, testOnly, name == "Seal"} {
			if holds {
				kinds++
			}
		}
		if kinds != 1 {
			t.Errorf("%s writes registry state and is guarded=%v, seam=%v, test-only=%v: "+
				"it must be exactly one (call the seal guard, or say %q in its doc comment)",
				name, isGuarded, isSeam, testOnly, seamMarker)
		}
		if isGuarded {
			guarded[name] = true
		}
	}

	want := map[string]bool{"Register": true}
	for name := range sealedCalls() {
		want[name] = true
	}
	if !maps.Equal(guarded, want) {
		t.Errorf("guarded functions %v, sealed-call cases %v: each guarded function needs a case, and each case a guard",
			slices.Sorted(maps.Keys(guarded)), slices.Sorted(maps.Keys(want)))
	}
}

// parseRegistrySource parses this package's non-test Go files and answers
// them with the names of its package-level variables.
func parseRegistrySource(t *testing.T) ([]*ast.File, map[string]bool) {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	packageVars := map[string]bool{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		file, parseErr := parser.ParseFile(fset, path, source, parser.ParseComments)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		files = append(files, file)
		for _, decl := range file.Decls {
			gen, isGen := decl.(*ast.GenDecl)
			if !isGen || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				for _, ident := range spec.(*ast.ValueSpec).Names { //nolint:forcetypeassert // a var GenDecl holds ValueSpecs only
					packageVars[ident.Name] = true
				}
			}
		}
	}
	if len(files) == 0 || len(packageVars) == 0 {
		t.Fatal("parsed no registry source: the test runs in the registry package directory")
	}
	return files, packageVars
}

// writingFuncs answers which functions write package state, directly or
// through a package function they call. The fixed point is bounded by the
// number of functions: each round adds at least one or stops.
func writingFuncs(funcs map[string]*ast.FuncDecl, packageVars map[string]bool) map[string]bool {
	writes := map[string]bool{}
	for name, fn := range funcs {
		writes[name] = writesDirectly(fn, packageVars)
	}
	for changed := true; changed; {
		changed = false
		for name, fn := range funcs {
			if writes[name] {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall {
					return true
				}
				if callee, isIdent := call.Fun.(*ast.Ident); isIdent && writes[callee.Name] {
					writes[name] = true
					changed = true
				}
				return true
			})
		}
	}
	return writes
}

func writesDirectly(fn *ast.FuncDecl, packageVars map[string]bool) bool {
	found := false
	rooted := func(expr ast.Expr) bool {
		root := rootIdent(expr)
		return root != nil && packageVars[root.Name]
	}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.AssignStmt:
			if n.Tok != token.DEFINE && slices.ContainsFunc(n.Lhs, rooted) {
				found = true
			}
		case *ast.IncDecStmt:
			if rooted(n.X) {
				found = true
			}
		case *ast.CallExpr:
			if writesThroughCall(n) && rooted(n.Args[0]) {
				found = true
			}
		}
		return !found
	})
	return found
}

// writesThroughCall answers whether call is clear, delete or maps.Copy, whose
// first argument it writes.
func writesThroughCall(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name == "clear" || fun.Name == "delete"
	case *ast.SelectorExpr:
		pkg, isIdent := fun.X.(*ast.Ident)
		return isIdent && pkg.Name == "maps" && fun.Sel.Name == "Copy"
	}
	return false
}

// rootIdent answers the variable an lvalue is rooted at: x for x, x.f, x[i]
// and *x. The depth is the expression's own, bounded by the source.
func rootIdent(expr ast.Expr) *ast.Ident {
	switch e := expr.(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return rootIdent(e.X)
	case *ast.IndexExpr:
		return rootIdent(e.X)
	case *ast.StarExpr:
		return rootIdent(e.X)
	case *ast.ParenExpr:
		return rootIdent(e.X)
	}
	return nil
}

func callsAny(fn *ast.FuncDecl, names ...string) bool {
	found := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if call, isCall := node.(*ast.CallExpr); isCall {
			if callee, isIdent := call.Fun.(*ast.Ident); isIdent && slices.Contains(names, callee.Name) {
				found = true
			}
		}
		return !found
	})
	return found
}

// filledRegistration answers a Registration whose every slice and map holds
// one filled element and whose every scalar is set, nested types included,
// so every reference the type can hold is present. Function fields stay nil:
// they are shared by design and reflect.DeepEqual can compare only nil ones.
// It is deterministic, so two calls answer equal values that share nothing.
func filledRegistration(t testing.TB, name string) Registration {
	t.Helper()
	var reg Registration
	fillValue(t, reflect.ValueOf(&reg).Elem(), "Registration")
	reg.Name = name
	return reg
}

// fillValue sets v, which holds its zero value, to a non-zero value of every
// kind it reaches; path names v in a failure. The depth is the type's.
func fillValue(t testing.TB, v reflect.Value, path string) {
	t.Helper()
	switch v.Kind() {
	case reflect.Struct:
		for i := range v.NumField() {
			fillValue(t, v.Field(i), path+"."+v.Type().Field(i).Name)
		}
	case reflect.Slice:
		filled := reflect.MakeSlice(v.Type(), 1, 1)
		fillValue(t, filled.Index(0), path+"[0]")
		v.Set(filled)
	case reflect.Map:
		filled := reflect.MakeMap(v.Type())
		key := reflect.New(v.Type().Key()).Elem()
		fillValue(t, key, path+" key")
		value := reflect.New(v.Type().Elem()).Elem()
		fillValue(t, value, path+" value")
		filled.SetMapIndex(key, value)
		v.Set(filled)
	case reflect.Pointer:
		filled := reflect.New(v.Type().Elem())
		fillValue(t, filled.Elem(), path)
		v.Set(filled)
	case reflect.Array:
		for i := range v.Len() {
			fillValue(t, v.Index(i), path+"[i]")
		}
	case reflect.String:
		v.SetString("filled")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Complex64, reflect.Complex128:
		v.SetComplex(1)
	case reflect.Func:
		// Shared by design: a function value cannot be changed through a copy.
	case reflect.Interface, reflect.Chan, reflect.UnsafePointer, reflect.Invalid:
		t.Fatalf("%s is a %s, which cloneRegistration cannot copy without sharing", path, v.Kind())
	}
}

// mutateValue writes a different value into every element v reaches: each
// scalar, each slice element, each map value, plus a new map key. A clone
// that shares any of it with the registry changes what the registry answers.
func mutateValue(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for i := range v.NumField() {
			mutateValue(v.Field(i))
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			mutateValue(v.Index(i))
		}
	case reflect.Map:
		entries := v.MapRange()
		for entries.Next() {
			value := reflect.New(v.Type().Elem()).Elem()
			value.Set(entries.Value())
			mutateValue(value)
			v.SetMapIndex(entries.Key(), value)
		}
		v.SetMapIndex(reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem())
	case reflect.Pointer:
		if !v.IsNil() {
			mutateValue(v.Elem())
		}
	case reflect.String:
		v.SetString("mutated")
	case reflect.Bool:
		v.SetBool(false)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(2)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v.SetUint(2)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(2)
	case reflect.Complex64, reflect.Complex128:
		v.SetComplex(2)
	case reflect.Func, reflect.Interface, reflect.Chan, reflect.UnsafePointer, reflect.Invalid:
		// Nothing to write through: fillValue refuses the last four kinds.
	}
}

// storeFilled files reg in the registry the way Register does, a clone under
// mu. It bypasses Register's validation, which refuses the arbitrary values
// filledRegistration writes (an unknown doctor platform, a data plane): the
// subject here is what Lookup and All hand out, not what Register accepts.
func storeFilled(reg *Registration) {
	mu.Lock()
	defer mu.Unlock()
	plugins[reg.Name] = cloneRegistration(reg)
	rebuildFamilyIndexLocked()
}

// TestLookupAndAllReturnCopies checks AC-3. Goal: a caller that writes into
// every field of what Lookup or All returned, before and after Seal, changes
// nothing a later Lookup or All answers; and a caller that writes into the
// slices it passed Register changes nothing either. Method: store a filled
// registration, mutate every reachable element of each answer by
// reflection, and compare the next answer with a fresh fill.
func TestLookupAndAllReturnCopies(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	Reset()

	filled := filledRegistration(t, "seal-filled")
	storeFilled(&filled)
	want := filledRegistration(t, "seal-filled")

	check := func(t *testing.T) {
		t.Helper()
		mutateValue(reflect.ValueOf(Lookup("seal-filled")).Elem())
		if got := Lookup("seal-filled"); !reflect.DeepEqual(*got, want) {
			t.Errorf("a write into a Lookup answer reached the registry:\n got %+v\nwant %+v", *got, want)
		}
		for _, reg := range All() {
			mutateValue(reflect.ValueOf(reg).Elem())
		}
		if got := All(); len(got) != 1 || !reflect.DeepEqual(*got[0], want) {
			t.Errorf("a write into an All answer reached the registry: got %+v", got)
		}
	}
	t.Run("before the seal", check)
	sealForTest(t)
	t.Run("after the seal", check)

	Reset()
	passed := validRegistration("seal-passed")
	if err := Register(passed); err != nil {
		t.Fatalf("Register: %v", err)
	}
	passed.Families[0] = "ipv4/changed-by-caller"
	if got := Lookup("seal-passed").Families; !slices.Equal(got, []string{"ipv4/seal-seal-passed"}) {
		t.Errorf("a write into the slice passed to Register reached the registry: %v", got)
	}
}

// TestRegistrationCloneCoversEveryReferenceField checks R-5 and AC-3 by
// reflection over the type. Goal: the clone equals the original and shares
// no slice array, map or pointer with it at any depth, and Registration
// holds no interface, channel or unsafe pointer the clone would share.
// Method: clone a filled registration and walk both in step.
func TestRegistrationCloneCoversEveryReferenceField(t *testing.T) {
	original := filledRegistration(t, "seal-filled")
	clone := cloneRegistration(&original)
	if !reflect.DeepEqual(*clone, original) {
		t.Fatalf("clone differs from the original:\n got %+v\nwant %+v", *clone, original)
	}
	requireUnshared(t, reflect.ValueOf(original), reflect.ValueOf(*clone), "Registration")
}

// requireUnshared fails on any slice, map or pointer that original and clone
// share. The depth is the type's.
func requireUnshared(t *testing.T, original, clone reflect.Value, path string) {
	t.Helper()
	switch original.Kind() {
	case reflect.Struct:
		for i := range original.NumField() {
			requireUnshared(t, original.Field(i), clone.Field(i), path+"."+original.Type().Field(i).Name)
		}
	case reflect.Slice:
		if original.Len() == 0 {
			t.Fatalf("%s is empty: the fill reaches every reference", path)
		}
		if original.Pointer() == clone.Pointer() {
			t.Errorf("%s shares its array with the clone", path)
		}
		for i := range original.Len() {
			requireUnshared(t, original.Index(i), clone.Index(i), path+"[i]")
		}
	case reflect.Map:
		if original.Pointer() == clone.Pointer() {
			t.Errorf("%s is the same map in the clone", path)
		}
		entries := original.MapRange()
		for entries.Next() {
			requireUnshared(t, entries.Value(), clone.MapIndex(entries.Key()), path+" value")
		}
	case reflect.Pointer:
		if original.Pointer() == clone.Pointer() {
			t.Errorf("%s points at the same value in the clone", path)
		}
		requireUnshared(t, original.Elem(), clone.Elem(), path)
	case reflect.Array:
		for i := range original.Len() {
			requireUnshared(t, original.Index(i), clone.Index(i), path+"[i]")
		}
	case reflect.Interface, reflect.Chan, reflect.UnsafePointer, reflect.Invalid:
		t.Errorf("%s is a %s, which the clone would share", path, original.Kind())
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.String, reflect.Func:
		// Copied by value, or shared by design (a function value).
	}
}

// TestSnapshotRestoreCarriesTheSeal checks AC-5. Goal: Snapshot records the
// seal, Restore puts it back, and Reset clears it, so a test that seals and
// restores leaves the next test able to register. Method: take snapshots on
// both sides of Seal and register after each Restore and after Reset.
func TestSnapshotRestoreCarriesTheSeal(t *testing.T) {
	open := Snapshot()
	t.Cleanup(func() { Restore(open) })
	Seal()
	closed := Snapshot()

	Restore(open)
	if err := Register(validRegistration("seal-after-restore")); err != nil {
		t.Errorf("Register after restoring an unsealed snapshot = %v, want nil", err)
	}
	Restore(closed)
	if err := Register(validRegistration("seal-after-sealed-restore")); !errors.Is(err, ErrRegistrySealed) {
		t.Errorf("Register after restoring a sealed snapshot = %v, want ErrRegistrySealed", err)
	}
	Reset()
	if err := Register(validRegistration("seal-after-reset")); err != nil {
		t.Errorf("Register after Reset = %v, want nil", err)
	}
}

// benchmarkRegistrySize is the population BenchmarkLookupAndAll files: about
// the number of plugins a full ze build registers.
const benchmarkRegistrySize = 110

// BenchmarkLookupAndAll measures AC-4: the bytes and allocations Lookup and
// All cost per call over a full-size registry of filled registrations, every
// slice and map holding one element, which over-states a typical plugin's
// registration (most leave most fields empty).
func BenchmarkLookupAndAll(b *testing.B) {
	snap := Snapshot()
	b.Cleanup(func() { Restore(snap) })
	Reset()
	for i := range benchmarkRegistrySize {
		reg := filledRegistration(b, "bench-"+string(rune('a'+i%26))+string(rune('a'+i/26)))
		storeFilled(&reg)
	}

	b.Run("Lookup", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if Lookup("bench-aa") == nil {
				b.Fatal("Lookup missed a filed registration")
			}
		}
	})
	b.Run("All", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if len(All()) != benchmarkRegistrySize {
				b.Fatal("All lost a registration")
			}
		}
	})
}
