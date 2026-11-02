package fixture

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// VALIDATES: every observer whose .ci scripts a BGP peer that expects ze's
// wire output waits, by name, address or count, on that peer's session before
// it ends, for each scripted peer, in every observer family of this package.
// A scripted peer is a stdin block a `le test peer` command line runs; the ze
// peer it stands for is the configured peer whose remote address is the
// address that command binds (127.0.0.1 unless --bind names one; a wildcard
// bind is matched by its port), or whose local address is the one a --dial
// command connects to.
// A wait is a CALL of one of the wait helpers observerWaitFamilies lists. The
// guard reads the code a registration runs: the driver or scenario the
// registration call names, each function value passed to a driver-building
// call that the builder runs (calls it, hands it to a package function that
// runs it, or returns it as the driver the registration runs), the table-row
// fields the registration runs, and every package function those bodies
// call. A value only returned runs only when its caller runs the result: a
// discarded result runs nothing. Each constant argument is bound to its
// parameter, unless the callee assigns it, shadows it or takes its address,
// so a peer name or count passed down through a helper is read where it is
// used. A function named but not called (`_ = waitPeerEOR08`), and a table no
// registration ranges over, wait for nothing. A helper names its peer by a
// string argument (a literal, a package constant or a bound parameter), by a
// fixed name or address, by a `show bgp peer <selector>` command, or by a
// count of distinct peers. A poll counts as a wait on the peer its predicate
// reads: the `show bgp peer <selector>` command a call in the predicate sends,
// or the selector a poll-read helper is given. The "*" selector names a peer
// only when the .ci scripts one, because a sum over several peers can reach
// its minimum from one of them.
// This test adds to the family-08 and family-12 guards
// (TestPluginObservers08WaitForTheScriptedPeer,
// TestPluginObservers12WaitForTheScriptedPeer); it replaces neither.
// LIMIT: the reading is static. A wait in a branch that never runs
// (`if false { waitPeerEOR08(ctx, p) }`), or a builder that returns its
// argument only under a condition that never holds, counts as a wait.
// Observe runs the scenario as soon as the plugins are ready, then quiesces and
// asks the daemon to stop. `request quiesce` reports a peer whose TCP connect
// has not completed as settled (docs/architecture/api/commands.md, "What the
// barrier does NOT cover"), and docs/functional-tests.md puts the wait on the
// observer: "Before requesting shutdown, an observer waits for the peer state
// or sent-payload counter its wire assertions require."
// PREVENTS: an observer that ends while its scripted peer is still connecting,
// so ze sends Cease to a peer still waiting for its messages.
// METHOD: parse the package source, find each registration call (Register, or
// a package function that calls Register with its own parameter) and each
// table a registration loop ranges over, read the .ci that runs the fixture,
// and compare the peers it scripts with the peers the registration waits on.
func TestPluginObserversWaitForTheScriptedPeer(t *testing.T) {
	findings, err := observerPeerWaitFindings(".", filepath.Join("..", "..", "..", "test"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range findings {
		t.Errorf("%s: its .ci peer expects ze's wire output, so the scenario must wait for that peer (a wait helper in observerWaitFamilies)", entry)
	}
}

// observerWaitKind is how a wait helper names the peer it waits on.
type observerWaitKind int

const (
	observerWaitUnspecified observerWaitKind = iota
	// observerWaitSelector reads the peer from the string argument at index:
	// a name, an address or "*". A variadic entry reads every argument from
	// index on.
	observerWaitSelector
	// observerWaitFixed waits on the peers the entry lists, whatever its
	// arguments say. It also applies to a scenario a registration names
	// directly, when that scenario is itself the wait.
	observerWaitFixed
	// observerWaitCommand reads the peer from a `show bgp peer <selector> ...`
	// command string at index.
	observerWaitCommand
	// observerWaitPoll is a poll whose predicate reads one peer's state
	// through a `show bgp peer <selector> ...` command written as a literal
	// in the predicate: waiting on that peer's state is waiting on the peer.
	observerWaitPoll
	// observerWaitPollRead reads one peer's state from the selector at index;
	// it is a wait only when a poll predicate calls it.
	observerWaitPollRead
	// observerWaitCount waits until a number of distinct peers have sent
	// their End-of-RIB: the entry's count, or else the integer at index.
	observerWaitCount
)

// observerWait describes one wait helper.
type observerWait struct {
	kind     observerWaitKind
	index    int
	variadic bool
	fixed    []string
	// count is a count wait's fixed number of peers, 0 to read it at index.
	count int
	// gated count waits wait only when the argument at gate is `true`.
	gated bool
	gate  int
}

// observerWaitFamily is one observer family: the files that hold it and the
// wait helpers it declares. Helpers are package functions, so a scenario of
// one family may call another family's helper; every family's helpers count
// for every registration.
type observerWaitFamily struct {
	// id names the family in a finding, "08" for plugin_fixture_08*.go.
	id string
	// prefix selects the family's files by name.
	prefix string
	waits  map[string]observerWait
	// strict refuses a scripted peer no configured peer matches, and a
	// registration that names its fixture with a non-constant no range
	// statement binds.
	strict bool
	// stdinNames also counts each `stdin=peer:` or `stdin=peerN:` block as
	// ze's peer1 or peerN, the convention the family's .ci files follow.
	stdinNames bool
	// exempt drivers end no run: an observer that stays alive until the
	// daemon says goodbye never asks for the shutdown the wait protects.
	exempt []string
}

// observerWaitFamilies is the data the guard reads. A family whose observers
// wait through a helper this table does not list reads as never waiting; add
// the helper here, beside its family.
var observerWaitFamilies = []observerWaitFamily{
	{id: "01", prefix: "plugin_fixture_01", waits: map[string]observerWait{
		"plugin01WaitCounter": {kind: observerWaitSelector, index: 2},
	}},
	{id: "02", prefix: "plugin_fixture_02", waits: map[string]observerWait{
		"eorSent02": {kind: observerWaitSelector, index: 2},
		// It reads sent UPDATE events until expectedPeers distinct peers
		// have been sent their End-of-RIB.
		"routeServerObserver02": {kind: observerWaitCount, index: 1},
		// It reads state events until a session comes up.
		"waitUp02": {kind: observerWaitFixed, fixed: []string{anyPeer}},
	}},
	{id: "03", prefix: "plugin_fixture_03", waits: map[string]observerWait{
		"waitPeerCounter03": {kind: observerWaitFixed, fixed: []string{addrLoopback}},
		"waitPeersEOR03":    {kind: observerWaitCount, index: 2},
	}},
	{id: "04", prefix: "plugin_fixture_04", waits: map[string]observerWait{
		"waitPeerEOR04": {kind: observerWaitFixed, fixed: []string{peerAddress04}},
		// With requireReplay it reads sent UPDATE events until expected
		// distinct peers have been sent their End-of-RIB; without, it waits
		// for no peer.
		"rsObserver04": {kind: observerWaitCount, index: 0, gated: true, gate: 2},
	}},
	{id: "05", prefix: "plugin_fixture_05", waits: map[string]observerWait{}},
	{id: "06", prefix: "plugin_fixture_06", waits: map[string]observerWait{
		"fixture06WaitEOR": {kind: observerWaitFixed, fixed: []string{"peer1"}},
		// It polls ze_peer_session_flaps_total{peer="127.0.0.1"} until the
		// session it established is torn down: the peer state its
		// NOTIFICATION assertion requires.
		"fixture06DeadpeerHolddown": {kind: observerWaitFixed, fixed: []string{addrLoopback}},
	}},
	{id: "07", prefix: "plugin_fixture_07", waits: map[string]observerWait{
		"waitEOR07": {kind: observerWaitCount, index: 2},
	}},
	{id: "08", prefix: "plugin_fixture_08", stdinNames: true, waits: map[string]observerWait{
		"waitPeersEOR08":    {kind: observerWaitSelector, index: 2, variadic: true},
		"waitPeerCounter08": {kind: observerWaitSelector, index: 2},
	}},
	{id: "09", prefix: "plugin_fixture_09", waits: map[string]observerWait{
		"waitL2TPBGPEOR09": {kind: observerWaitFixed, fixed: []string{"peer1"}},
	}, exempt: []string{"passiveObserve09"}},
	{id: "10", prefix: "plugin_fixture_10", waits: map[string]observerWait{
		"fixture10WaitEOR":     {kind: observerWaitSelector, index: 2},
		"fixture10PeerCounter": {kind: observerWaitPollRead, index: 2},
	}},
	{id: "11", prefix: "plugin_fixture_11", waits: map[string]observerWait{}},
	{id: "12", prefix: "plugin_fixture_12", strict: true, waits: map[string]observerWait{
		"p12WaitPeersEOR":    {kind: observerWaitSelector, index: 2, variadic: true},
		"p12WaitPeerCounter": {kind: observerWaitSelector, index: 2},
	}, exempt: []string{"p12WaitForDaemonDriver"}},
	{id: "13", prefix: "plugin_fixture_13", waits: map[string]observerWait{
		"waitEORSent13": {kind: observerWaitSelector, index: 2},
		// It reads sent UPDATE events until two distinct peers have been
		// sent their End-of-RIB.
		"routeServerReplay13": {kind: observerWaitCount, count: 2},
	}, exempt: []string{"passivePlugin13"}},
	{id: "14", prefix: "plugin_fixture_14", waits: map[string]observerWait{
		"plugin14PollCommand": {kind: observerWaitCommand, index: 3},
		// It polls `show bgp peer * detail` until every one of the
		// scenario's sessions has been sent its End-of-RIB.
		"blackholeReadvertiseFence": {kind: observerWaitCount, count: blackholeReadvertiseSessions},
	}},
	{id: "15", prefix: "plugin_fixture_15", waits: map[string]observerWait{
		"plugin15WaitEOR": {kind: observerWaitSelector, index: 2},
	}},
	{id: "16", prefix: "plugin_fixture_16", waits: map[string]observerWait{
		"plugin16WaitEOR": {kind: observerWaitCommand, index: 2},
	}},
	{id: "misc", prefix: "misc_fixture", waits: map[string]observerWait{
		"waitPeerEORSent": {kind: observerWaitSelector, index: 2},
	}},
	// Every other file. Its prefix matches any name, so it stays last.
	{id: "other", prefix: "", waits: map[string]observerWait{
		"Poll": {kind: observerWaitPoll},
		// It polls until the peer has a gr-state row, which appears once
		// that peer's graceful-restart session went down and its routes
		// were marked stale: a wait on that peer's session state.
		"awaitGRState": {kind: observerWaitSelector, index: 2},
	}},
}

// observerFamilyOf answers the family a file belongs to, "other" for a file
// no other family prefix selects.
func observerFamilyOf(name string) string {
	for _, family := range observerWaitFamilies {
		if strings.HasPrefix(name, family.prefix) {
			return family.id
		}
	}
	return "other"
}

// observerFamilyFlags answers whether any of a registration's families is
// strict, and whether any reads stdin block names.
func observerFamilyFlags(families []string) (strict, stdinNames bool) {
	for _, family := range observerWaitFamilies {
		if !slices.Contains(families, family.id) {
			continue
		}
		strict = strict || family.strict
		stdinNames = stdinNames || family.stdinNames
	}
	return strict, stdinNames
}

// observerPeerWaitFindings answers one line per scripted peer a registration
// never waits on, for the package in dir whose .ci files live under testRoot.
// A registration the scan cannot resolve is an error, never a pass.
func observerPeerWaitFindings(dir, testRoot string) ([]string, error) {
	source, err := parseObserverPackage(dir, testRoot)
	if err != nil {
		return nil, err
	}
	registered := 0
	checked := 0
	var missing []string
	for _, fixture := range slices.Sorted(maps.Keys(source.registrations)) {
		registration := source.registrations[fixture]
		registered++
		waited, exempt := source.waited(registration)
		if exempt {
			continue
		}
		ci := source.ci[fixture]
		if !strings.Contains(ci, "\nexpect=bgp:") {
			continue
		}
		scripted, err := scriptedPeers(fixture, ci, registration.families)
		if err != nil {
			return nil, err
		}
		if len(scripted) == 0 {
			continue
		}
		checked++
		for _, peer := range scripted {
			if peer.waitedOnBy(waited, len(scripted)) {
				continue
			}
			missing = append(missing, fixture+" (family "+strings.Join(registration.families, ",")+", "+registration.describe()+") never waits for "+peer.name)
		}
	}
	if registered == 0 {
		return nil, errors.New("no observer registration found: the scan no longer reads the registrations")
	}
	if checked == 0 {
		return nil, errors.New("no observer scripts a peer: the .ci scan no longer finds the tests")
	}
	return missing, nil
}

// observerRegistration is one fixture's registration: the driver expressions
// its registration call or table row passes, and the registrar helper
// (registerPlugin08) whose own body runs them.
type observerRegistration struct {
	families []string
	// drivers are the expressions the registration hands over to be run.
	drivers []ast.Expr
	// registrars are the helper functions the registration calls.
	registrars []string
}

func (registration *observerRegistration) describe() string {
	var names []string
	for _, driver := range registration.drivers {
		switch typed := ast.Unparen(driver).(type) {
		case *ast.Ident:
			names = append(names, typed.Name)
		case *ast.CallExpr:
			if callee, ok := typed.Fun.(*ast.Ident); ok {
				names = append(names, callee.Name+"(...)")
			}
		case *ast.FuncLit:
			names = append(names, "an inline scenario")
		}
	}
	if len(names) == 0 {
		return "an unnamed scenario"
	}
	return strings.Join(names, ", ")
}

// observerPackage is what the guard reads from the package source.
type observerPackage struct {
	testRoot  string
	functions map[string]*ast.FuncDecl
	constants map[string]string
	// structFields holds each package struct type's field names in order.
	structFields map[string][]string
	// types holds every package type name, so a conversion to one
	// (`ObserverScenario(observer.scenario)`) reads as the value it converts.
	types map[string]bool
	// variables holds each package variable's value, for a driver built
	// into one (`var bmpBestpath04 = withBMP04(...)`).
	variables     map[string]ast.Expr
	waits         map[string]observerWait
	exempt        map[string]bool
	registrars    map[string]int
	registrations map[string]*observerRegistration
	// ci holds the text of <testRoot>/<fixture>.ci for every fixture found.
	ci map[string]string
}

// parseObserverPackage reads every function and constant of the package in
// dir, then every registration call and every table a registration ranges
// over.
func parseObserverPackage(dir, testRoot string) (*observerPackage, error) {
	source := &observerPackage{
		testRoot:      testRoot,
		functions:     map[string]*ast.FuncDecl{},
		constants:     map[string]string{},
		variables:     map[string]ast.Expr{},
		structFields:  map[string][]string{},
		types:         map[string]bool{},
		waits:         map[string]observerWait{},
		exempt:        map[string]bool{},
		registrars:    map[string]int{},
		registrations: map[string]*observerRegistration{},
		ci:            map[string]string{},
	}
	for _, entry := range observerWaitFamilies {
		maps.Copy(source.waits, entry.waits)
		for _, name := range entry.exempt {
			source.exempt[name] = true
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, err
		}
		source.collect(file)
		files[name] = file
	}
	source.findRegistrars()
	for _, name := range slices.Sorted(maps.Keys(files)) {
		if err := source.readRegistrations(observerFamilyOf(name), files[name]); err != nil {
			return nil, err
		}
	}
	return source, nil
}

// collect records a file's package functions and constants.
func (source *observerPackage) collect(file *ast.File) {
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			if typed.Recv == nil {
				source.functions[typed.Name.Name] = typed
			}
		case *ast.GenDecl:
			if typed.Tok == token.VAR {
				source.collectVariables(typed)
				continue
			}
			if typed.Tok == token.TYPE {
				source.collectStructs(typed)
				continue
			}
			if typed.Tok != token.CONST {
				continue
			}
			for _, spec := range typed.Specs {
				values, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for index, name := range values.Names {
					if index >= len(values.Values) {
						break
					}
					literal, ok := values.Values[index].(*ast.BasicLit)
					if !ok {
						continue
					}
					if literal.Kind == token.INT {
						source.constants[name.Name] = literal.Value
						continue
					}
					if literal.Kind != token.STRING {
						continue
					}
					if text, err := strconv.Unquote(literal.Value); err == nil {
						source.constants[name.Name] = text
					}
				}
			}
		}
	}
}

// collectVariables records each package variable declared with one value.
func (source *observerPackage) collectVariables(decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		values, ok := spec.(*ast.ValueSpec)
		if !ok || len(values.Names) != len(values.Values) {
			continue
		}
		for index, name := range values.Names {
			source.variables[name.Name] = values.Values[index]
		}
	}
}

// collectStructs records each struct type's field names in order.
func (source *observerPackage) collectStructs(decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		source.types[typeSpec.Name.Name] = true
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}
		var names []string
		for _, field := range structType.Fields.List {
			for _, name := range field.Names {
				names = append(names, name.Name)
			}
		}
		source.structFields[typeSpec.Name.Name] = names
	}
}

// findRegistrars records Register and every package function that calls
// Register with one of its own parameters as the fixture name, with that
// parameter's index: registerPlugin08(name, plugin, scenario) is one.
func (source *observerPackage) findRegistrars() {
	source.registrars["Register"] = 0
	for name, decl := range source.functions {
		if name == "Register" || decl.Body == nil {
			continue
		}
		params := parameterNames(decl.Type)
		ast.Inspect(decl.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			if callee, ok := call.Fun.(*ast.Ident); !ok || callee.Name != "Register" {
				return true
			}
			arg, ok := call.Args[0].(*ast.Ident)
			if !ok {
				return true
			}
			if index := slices.Index(params, arg.Name); index >= 0 {
				source.registrars[name] = index
			}
			return true
		})
	}
}

// parameterNames answers a function's parameter names in order.
func parameterNames(funcType *ast.FuncType) []string {
	var names []string
	for _, field := range funcType.Params.List {
		if len(field.Names) == 0 {
			names = append(names, "_")
			continue
		}
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	return names
}

// readRegistrations reads one file's registration calls. A call to a
// registrar outside a registrar's own body registers a fixture: a constant
// fixture name is read from the call, and a name a range statement binds is
// read from each row of the table it ranges over.
func (source *observerPackage) readRegistrations(family string, file *ast.File) error {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil {
			continue
		}
		if _, isRegistrar := source.registrars[funcDecl.Name.Name]; isRegistrar {
			continue
		}
		var failure error
		var loops []*ast.RangeStmt
		var visit func(node ast.Node) bool
		visit = func(node ast.Node) bool {
			if failure != nil {
				return false
			}
			if loop, ok := node.(*ast.RangeStmt); ok {
				loops = append(loops, loop)
				ast.Inspect(loop.Body, visit)
				loops = loops[:len(loops)-1]
				return false
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			index, isRegistrar := source.registrars[callee.Name]
			if !isRegistrar || index >= len(call.Args) {
				return true
			}
			failure = source.register(family, funcDecl, loops, callee.Name, index, call)
			return true
		}
		ast.Inspect(funcDecl.Body, visit)
		if failure != nil {
			return failure
		}
	}
	return nil
}

// register records one registration call.
func (source *observerPackage) register(family string, enclosing *ast.FuncDecl, loops []*ast.RangeStmt, registrar string, index int, call *ast.CallExpr) error {
	var drivers []ast.Expr
	for other, arg := range call.Args {
		if other != index {
			drivers = append(drivers, arg)
		}
	}
	var registrars []string
	if registrar != "Register" {
		registrars = append(registrars, registrar)
	}
	if _, ok := source.stringValue(call.Args[index], nil); ok {
		fixture, isFixture := source.fixtureName(call.Args[index])
		if !isFixture {
			return nil // a trigger helper or a fixture another directory's .ci runs
		}
		for _, driver := range drivers {
			if err := source.resolvable(fixture, driver); err != nil {
				return err
			}
		}
		source.record(fixture, family, drivers, registrars)
		return nil
	}
	loop := bindingLoop(call.Args[index], loops)
	if loop == nil {
		if family == "12" {
			return errors.New("a family-12 Register call names its fixture with a non-constant that no range statement binds")
		}
		return nil // a fixture named at run time, outside any table
	}
	table := source.tableOf(loop.X, enclosing)
	if table == nil {
		return errors.New(registrar + " in " + enclosing.Name.Name + " ranges over a table this scan cannot read")
	}
	whole, fields := source.passedRowParts(loop, call)
	for _, row := range table.Elts {
		key, value := row, row
		if pair, ok := row.(*ast.KeyValueExpr); ok {
			key, value = pair.Key, pair.Value
		}
		fixture, ok := source.rowFixture(key, value)
		if !ok {
			continue
		}
		rowDrivers := append(slices.Clone(drivers), source.passedValues(table, value, whole, fields)...)
		source.record(fixture, family, rowDrivers, registrars)
	}
	return nil
}

// passedRowParts answers what of a table row the registration call runs:
// the whole row value (`scenario`), or the named fields it runs (`o.run`),
// through the loop's value variable or a copy of it taken in the loop body
// (`o := observer`, `n, s := name, scenario`). A field counts only where the
// registration runs it: as a registration argument, through a builder that
// runs or returns it, or called or handed on to a package function that runs
// it inside an inline driver. A whole row handed to a package function counts
// for the fields that function runs.
func (source *observerPackage) passedRowParts(loop *ast.RangeStmt, call *ast.CallExpr) (whole bool, fields map[string]bool) {
	fields = map[string]bool{}
	value, ok := loop.Value.(*ast.Ident)
	if !ok {
		return false, fields
	}
	names := map[string]bool{value.Name: true}
	ast.Inspect(loop.Body, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE || len(assign.Lhs) != len(assign.Rhs) {
			return true
		}
		for index, rhs := range assign.Rhs {
			if ident, ok := rhs.(*ast.Ident); ok && names[ident.Name] {
				if alias, ok := assign.Lhs[index].(*ast.Ident); ok {
					names[alias.Name] = true
				}
			}
		}
		return true
	})
	isRow := func(expr ast.Expr) bool {
		expr = source.unconvert(expr)
		if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
			expr = ast.Unparen(unary.X)
		}
		ident, ok := expr.(*ast.Ident)
		return ok && names[ident.Name]
	}
	rowField := func(expr ast.Expr) string {
		selector, ok := source.unconvert(expr).(*ast.SelectorExpr)
		if !ok {
			return ""
		}
		if root, ok := selector.X.(*ast.Ident); ok && names[root.Name] {
			return selector.Sel.Name
		}
		return ""
	}
	// handOn records what one argument of a package function call hands
	// over, when that function runs it.
	handOn := func(callee string, index int, arg ast.Expr, runs bool) {
		if isRow(arg) {
			for _, field := range source.runFields(callee, index) {
				fields[field] = true
			}
		}
		if !runs {
			return
		}
		if field := rowField(arg); field != "" {
			fields[field] = true
		}
		if ident, ok := source.unconvert(arg).(*ast.Ident); ok && names[ident.Name] {
			whole = true
		}
	}
	var consider func(expr ast.Expr)
	consider = func(expr ast.Expr) {
		switch typed := source.unconvert(expr).(type) {
		case *ast.SelectorExpr:
			if field := rowField(typed); field != "" {
				fields[field] = true
			}
		case *ast.Ident:
			if names[typed.Name] {
				whole = true
			}
		case *ast.CallExpr:
			callee, ok := typed.Fun.(*ast.Ident)
			if !ok || source.functions[callee.Name] == nil {
				return
			}
			for index, arg := range typed.Args {
				if source.buildsWith(callee.Name, index) {
					consider(arg)
					continue
				}
				handOn(callee.Name, index, arg, false)
			}
		case *ast.FuncLit:
			ast.Inspect(typed.Body, func(node ast.Node) bool {
				inner, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if field := rowField(inner.Fun); field != "" {
					fields[field] = true
				}
				if ident, ok := ast.Unparen(inner.Fun).(*ast.Ident); ok && names[ident.Name] {
					whole = true
				}
				callee, ok := inner.Fun.(*ast.Ident)
				if !ok || source.functions[callee.Name] == nil {
					return true
				}
				for index, arg := range inner.Args {
					handOn(callee.Name, index, arg, source.runsWith(callee.Name, index))
				}
				return true
			})
		}
	}
	for _, arg := range call.Args {
		consider(arg)
	}
	return whole, fields
}

// passedValues answers the expressions of one table row the registration
// runs: every element when the loop passes the whole row, else the elements
// of the fields it reads. A positional struct row is matched to its fields
// through the row type's declaration.
func (source *observerPackage) passedValues(table *ast.CompositeLit, value ast.Expr, whole bool, fields map[string]bool) []ast.Expr {
	if whole {
		return functionValues(value)
	}
	literal, ok := ast.Unparen(value).(*ast.CompositeLit)
	if !ok || len(fields) == 0 {
		return nil
	}
	order := source.rowFields(table, literal)
	var values []ast.Expr
	for index, element := range literal.Elts {
		name := ""
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			if key, ok := pair.Key.(*ast.Ident); ok {
				name = key.Name
			}
			element = pair.Value
		} else if index < len(order) {
			name = order[index]
		}
		if fields[name] {
			values = append(values, element)
		}
	}
	return values
}

// rowFields answers the field names of a table's rows in order: those of
// the row literal's own named type, or of the table's element or value type,
// named or written in place.
func (source *observerPackage) rowFields(table, row *ast.CompositeLit) []string {
	candidates := []ast.Expr{row.Type}
	switch typed := table.Type.(type) {
	case *ast.ArrayType:
		candidates = append(candidates, typed.Elt)
	case *ast.MapType:
		candidates = append(candidates, typed.Value)
	}
	for _, candidate := range candidates {
		switch typed := candidate.(type) {
		case *ast.Ident:
			if fields, ok := source.structFields[typed.Name]; ok {
				return fields
			}
		case *ast.StructType:
			var names []string
			for _, field := range typed.Fields.List {
				for _, name := range field.Names {
					names = append(names, name.Name)
				}
			}
			return names
		}
	}
	return nil
}

// bindingLoop answers the range statement that binds the identifier (or the
// root of the selector) a registration names its fixture with, or nil. A copy
// taken inside the loop (`o := observer`, `n, s := name, scenario`) belongs to
// the innermost loop.
func bindingLoop(expr ast.Expr, loops []*ast.RangeStmt) *ast.RangeStmt {
	root := ast.Unparen(expr)
	if selector, ok := root.(*ast.SelectorExpr); ok {
		root = selector.X
	}
	ident, ok := root.(*ast.Ident)
	if !ok {
		return nil
	}
	for _, loop := range slices.Backward(loops) {
		for _, bound := range []ast.Expr{loop.Key, loop.Value} {
			if name, ok := bound.(*ast.Ident); ok && name.Name == ident.Name {
				return loop
			}
		}
	}
	if len(loops) > 0 {
		return loops[len(loops)-1]
	}
	return nil
}

// tableOf answers the composite literal a range statement ranges over: one
// written in place, or a local of the enclosing function assigned one.
func (source *observerPackage) tableOf(expr ast.Expr, enclosing *ast.FuncDecl) *ast.CompositeLit {
	switch typed := ast.Unparen(expr).(type) {
	case *ast.CompositeLit:
		return typed
	case *ast.Ident:
		var found *ast.CompositeLit
		ast.Inspect(enclosing.Body, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok || len(assign.Lhs) != len(assign.Rhs) {
				return true
			}
			for index, lhs := range assign.Lhs {
				if name, ok := lhs.(*ast.Ident); ok && name.Name == typed.Name {
					if literal, ok := assign.Rhs[index].(*ast.CompositeLit); ok {
						found = literal
					}
				}
			}
			return true
		})
		return found
	}
	return nil
}

// rowFixture answers the fixture a table row names: its map key, or the first
// constant string element of a struct row for which a .ci exists.
func (source *observerPackage) rowFixture(key, value ast.Expr) (string, bool) {
	if key != value {
		return source.fixtureName(key)
	}
	literal, ok := value.(*ast.CompositeLit)
	if !ok {
		return "", false
	}
	for _, element := range literal.Elts {
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			element = pair.Value
		}
		if fixture, ok := source.fixtureName(element); ok {
			return fixture, true
		}
	}
	return "", false
}

// functionValues answers the expressions a table row hands over: the value
// itself, or a struct row's elements.
func functionValues(value ast.Expr) []ast.Expr {
	literal, ok := ast.Unparen(value).(*ast.CompositeLit)
	if !ok {
		return []ast.Expr{value}
	}
	var values []ast.Expr
	for _, element := range literal.Elts {
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			element = pair.Value
		}
		values = append(values, element)
	}
	return values
}

// resolvable refuses a driver expression the scan cannot follow, so no
// registration passes because its scenario was never read.
func (source *observerPackage) resolvable(fixture string, driver ast.Expr) error {
	switch typed := ast.Unparen(driver).(type) {
	case *ast.BasicLit, *ast.FuncLit, *ast.CompositeLit, *ast.UnaryExpr:
		return nil
	case *ast.Ident:
		if source.functions[typed.Name] != nil {
			return nil
		}
		if _, ok := source.constants[typed.Name]; ok {
			return nil
		}
		if value, ok := source.variables[typed.Name]; ok {
			return source.resolvable(fixture, value)
		}
		return errors.New(fixture + " registers " + typed.Name + ", which the scan did not find")
	case *ast.CallExpr:
		callee, ok := typed.Fun.(*ast.Ident)
		if !ok || source.functions[callee.Name] == nil {
			return errors.New(fixture + " registers a driver this scan cannot resolve: its call names no package function")
		}
		for _, arg := range typed.Args {
			call, isCall := ast.Unparen(arg).(*ast.CallExpr)
			if !isCall {
				continue
			}
			if inner, ok := call.Fun.(*ast.Ident); ok && source.functions[inner.Name] == nil {
				continue // a library call computing a value, not a driver
			}
			if err := source.resolvable(fixture, arg); err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New(fixture + " registers a scenario this scan cannot resolve")
}

func (source *observerPackage) record(fixture, family string, drivers []ast.Expr, registrars []string) {
	registration := source.registrations[fixture]
	if registration == nil {
		registration = &observerRegistration{}
		source.registrations[fixture] = registration
	}
	if !slices.Contains(registration.families, family) {
		registration.families = append(registration.families, family)
	}
	registration.drivers = append(registration.drivers, drivers...)
	for _, name := range registrars {
		if !slices.Contains(registration.registrars, name) {
			registration.registrars = append(registration.registrars, name)
		}
	}
}

// fixtureName answers the fixture an expression names: a constant string for
// which <testRoot>/<name>.ci exists. Its text is kept for the check.
func (source *observerPackage) fixtureName(expr ast.Expr) (string, bool) {
	name, ok := source.stringValue(expr, nil)
	if !ok || !strings.Contains(name, "/") {
		return "", false
	}
	if _, read := source.ci[name]; read {
		return name, true
	}
	data, err := os.ReadFile(filepath.Join(source.testRoot, name+".ci"))
	if err != nil {
		return "", false // a trigger helper or a fixture another directory's .ci runs
	}
	source.ci[name] = string(data)
	return name, true
}

// stringValue answers a string literal's text, a package constant's, or a
// parameter bound to a constant.
func (source *observerPackage) stringValue(expr ast.Expr, bound map[string]string) (string, bool) {
	switch typed := ast.Unparen(expr).(type) {
	case *ast.BasicLit:
		if typed.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(typed.Value)
		return text, err == nil
	case *ast.Ident:
		if text, ok := bound[typed.Name]; ok {
			return text, true
		}
		text, ok := source.constants[typed.Name]
		return text, ok
	}
	return "", false
}

// intValue answers an integer literal's value, a package constant's, or a
// parameter bound to one.
func (source *observerPackage) intValue(expr ast.Expr, bound map[string]string) (int, bool) {
	var text string
	switch typed := ast.Unparen(expr).(type) {
	case *ast.BasicLit:
		if typed.Kind != token.INT {
			return 0, false
		}
		text = typed.Value
	case *ast.Ident:
		value, ok := bound[typed.Name]
		if !ok {
			value = source.constants[typed.Name]
		}
		text = value
	default:
		return 0, false
	}
	value, err := strconv.Atoi(text)
	return value, err == nil
}

// observerWaited is what the code a registration runs waits on.
type observerWaited struct {
	selectors []string
	// count is the largest number of distinct peers a count wait asked for.
	count int
}

// observerWalk carries one registration's walk.
type observerWalk struct {
	source  *observerPackage
	visited map[string]bool
	answer  observerWaited
	exempt  bool
}

// waited walks the code a registration runs and answers what it waits on, and
// whether it runs an exempt driver.
func (source *observerPackage) waited(registration *observerRegistration) (observerWaited, bool) {
	walk := &observerWalk{source: source, visited: map[string]bool{}}
	for _, name := range registration.registrars {
		walk.invoke(name, nil)
	}
	for _, driver := range registration.drivers {
		walk.driver(driver, nil)
	}
	return walk.answer, walk.exempt
}

// driver follows an expression handed over to be run: a package function is
// called, a function literal runs, and a driver-building call runs its callee
// and each argument the callee runs (calls, returns, or hands to a package
// function that runs it). An argument the builder ignores is not followed.
func (walk *observerWalk) driver(expr ast.Expr, bound map[string]string) {
	switch typed := walk.source.unconvert(expr).(type) {
	case *ast.Ident:
		if walk.source.functions[typed.Name] != nil {
			walk.invoke(typed.Name, nil)
			return
		}
		if value, ok := walk.source.variables[typed.Name]; ok && !walk.visited["var "+typed.Name] {
			walk.visited["var "+typed.Name] = true
			walk.driver(value, bound)
		}
	case *ast.FuncLit:
		walk.body(typed.Body, bound)
		walk.handedOn(typed.Body)
	case *ast.CallExpr:
		callee, ok := typed.Fun.(*ast.Ident)
		if !ok || walk.source.functions[callee.Name] == nil {
			return
		}
		walk.call(typed, bound)
		for index, arg := range typed.Args {
			if walk.source.buildsWith(callee.Name, index) {
				walk.driver(arg, bound)
			}
		}
	}
}

// unconvert answers the value a conversion to a package type converts, or the
// expression itself.
func (source *observerPackage) unconvert(expr ast.Expr) ast.Expr {
	for {
		call, ok := ast.Unparen(expr).(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return ast.Unparen(expr)
		}
		callee, ok := call.Fun.(*ast.Ident)
		if !ok || !source.types[callee.Name] {
			return ast.Unparen(expr)
		}
		expr = call.Args[0]
	}
}

// paramUseDepth bounds how far paramUse follows a parameter handed from one
// package function to the next.
const paramUseDepth = 8

// paramUse reports how a package function uses its parameter at index:
// whether it runs it (calls it, hands it to a package function that runs it,
// or calls the result of one that returns it), and whether it returns it (as
// is, or through a package function that returns it). A parameter only
// returned runs when the caller runs the result: a registration runs the
// driver a builder returns, and a discarded result runs nothing.
func (source *observerPackage) paramUse(name string, index, depth int) (runs, returns bool) {
	decl := source.functions[name]
	if decl == nil || decl.Body == nil || depth > paramUseDepth {
		return false, false
	}
	params := parameterNames(decl.Type)
	if index >= len(params) || params[index] == "_" {
		return false, false
	}
	param := params[index]
	// handedReturn reports whether a call hands the parameter to a package
	// function that returns it, so the call's result is the parameter.
	handedReturn := func(expr ast.Expr) bool {
		call, ok := ast.Unparen(expr).(*ast.CallExpr)
		if !ok {
			return false
		}
		callee, ok := call.Fun.(*ast.Ident)
		if !ok || source.functions[callee.Name] == nil {
			return false
		}
		for argIndex, arg := range call.Args {
			if !isIdentNamed(arg, param) {
				continue
			}
			if _, returned := source.paramUse(callee.Name, argIndex, depth+1); returned {
				return true
			}
		}
		return false
	}
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.ReturnStmt:
			for _, result := range typed.Results {
				if isIdentNamed(result, param) || handedReturn(result) {
					returns = true
				}
			}
		case *ast.CallExpr:
			if isIdentNamed(typed.Fun, param) || handedReturn(typed.Fun) {
				runs = true
				return true
			}
			callee, ok := typed.Fun.(*ast.Ident)
			if !ok || source.functions[callee.Name] == nil {
				return true
			}
			for argIndex, arg := range typed.Args {
				if !isIdentNamed(arg, param) {
					continue
				}
				if called, _ := source.paramUse(callee.Name, argIndex, depth+1); called {
					runs = true
				}
			}
		}
		return true
	})
	return runs, returns
}

// buildsWith reports whether a driver-building call's argument at index ends
// up run: the builder runs it, or returns it as the driver the registration
// runs.
func (source *observerPackage) buildsWith(name string, index int) bool {
	runs, returns := source.paramUse(name, index, 0)
	return runs || returns
}

// runsWith reports whether a package function runs its argument at index.
func (source *observerPackage) runsWith(name string, index int) bool {
	runs, _ := source.paramUse(name, index, 0)
	return runs
}

// runFields answers the fields of a struct parameter a package function runs:
// those it calls (`driver.setup(plugin)`) or hands to a package function that
// runs them (`invokeScenario(ctx, plugin, driver.scenario)`).
func (source *observerPackage) runFields(name string, index int) []string {
	decl := source.functions[name]
	if decl == nil || decl.Body == nil {
		return nil
	}
	params := parameterNames(decl.Type)
	if index >= len(params) {
		return nil
	}
	param := params[index]
	fieldOf := func(expr ast.Expr) string {
		selector, ok := ast.Unparen(expr).(*ast.SelectorExpr)
		if !ok || !isIdentNamed(selector.X, param) {
			return ""
		}
		return selector.Sel.Name
	}
	var fields []string
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if field := fieldOf(call.Fun); field != "" {
			fields = append(fields, field)
		}
		callee, ok := call.Fun.(*ast.Ident)
		if !ok || source.functions[callee.Name] == nil {
			return true
		}
		for argIndex, arg := range call.Args {
			if field := fieldOf(arg); field != "" && source.runsWith(callee.Name, argIndex) {
				fields = append(fields, field)
			}
		}
		return true
	})
	return fields
}

// isIdentNamed reports whether an expression is the identifier name.
func isIdentNamed(expr ast.Expr, name string) bool {
	ident, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && ident.Name == name
}

// handedOn follows each package function a driver literal hands to a call
// of a package function in its own body that runs it, as
// `Observe(ctx, name, registration, scenario)` does: the literal is the
// driver, so that call is the driver call. A function named anywhere else in
// the literal, or handed to a conversion or a library call, is not followed.
func (walk *observerWalk) handedOn(body *ast.BlockStmt) {
	ast.Inspect(body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee, ok := call.Fun.(*ast.Ident)
		if !ok || walk.source.functions[callee.Name] == nil {
			return true
		}
		for index, arg := range call.Args {
			ident, ok := walk.source.unconvert(arg).(*ast.Ident)
			if !ok || walk.source.functions[ident.Name] == nil {
				continue
			}
			if walk.source.runsWith(callee.Name, index) {
				walk.invoke(ident.Name, nil)
			}
		}
		return true
	})
}

// call reads a call as a wait and follows its callee, with each constant
// argument bound to its parameter.
func (walk *observerWalk) call(call *ast.CallExpr, bound map[string]string) {
	callee, ok := call.Fun.(*ast.Ident)
	if !ok {
		return
	}
	walk.readWait(callee.Name, call, bound)
	decl := walk.source.functions[callee.Name]
	if decl == nil {
		return
	}
	params := parameterNames(decl.Type)
	binding := map[string]string{}
	for index, arg := range call.Args {
		if index >= len(params) {
			break
		}
		if text, ok := walk.source.stringValue(arg, bound); ok {
			binding[params[index]] = text
			continue
		}
		if value, ok := walk.source.intValue(arg, bound); ok {
			binding[params[index]] = strconv.Itoa(value)
		}
	}
	walk.invoke(callee.Name, binding)
}

// invoke walks a package function's body under a parameter binding, once per
// distinct binding.
func (walk *observerWalk) invoke(name string, binding map[string]string) {
	if walk.source.exempt[name] {
		walk.exempt = true
	}
	if wait, ok := walk.source.waits[name]; ok {
		if wait.kind == observerWaitFixed {
			walk.answer.selectors = append(walk.answer.selectors, wait.fixed...)
		}
		if wait.kind == observerWaitCount && wait.count > walk.answer.count && !wait.gated {
			walk.answer.count = wait.count
		}
	}
	decl := walk.source.functions[name]
	if decl == nil || decl.Body == nil {
		return
	}
	binding = stableBinding(decl, binding)
	key := name
	for _, param := range slices.Sorted(maps.Keys(binding)) {
		key += "\x00" + param + "=" + binding[param]
	}
	if walk.visited[key] {
		return
	}
	walk.visited[key] = true
	walk.body(decl.Body, binding)
}

// stableBinding drops each bound parameter the function assigns, or shadows
// with a `:=`, a range variable or a function literal's parameter: the
// constant the caller passed is then not what every use of the name reads.
func stableBinding(decl *ast.FuncDecl, binding map[string]string) map[string]string {
	if len(binding) == 0 {
		return binding
	}
	changed := map[string]bool{}
	mark := func(expr ast.Expr) {
		if ident, ok := ast.Unparen(expr).(*ast.Ident); ok {
			changed[ident.Name] = true
		}
	}
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.AssignStmt:
			for _, lhs := range typed.Lhs {
				mark(lhs)
			}
		case *ast.IncDecStmt:
			mark(typed.X)
		case *ast.UnaryExpr:
			if typed.Op == token.AND {
				mark(typed.X)
			}
		case *ast.RangeStmt:
			if typed.Key != nil {
				mark(typed.Key)
			}
			if typed.Value != nil {
				mark(typed.Value)
			}
		case *ast.ValueSpec:
			for _, name := range typed.Names {
				changed[name.Name] = true
			}
		case *ast.FuncLit:
			for _, name := range parameterNames(typed.Type) {
				changed[name] = true
			}
		}
		return true
	})
	stable := map[string]string{}
	for param, value := range binding {
		if !changed[param] {
			stable[param] = value
		}
	}
	return stable
}

// body follows every call in a body, function literals included. A function
// named without a call is not followed: it runs nothing here.
func (walk *observerWalk) body(body *ast.BlockStmt, bound map[string]string) {
	ast.Inspect(body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			walk.call(call, bound)
		}
		return true
	})
}

// showPeerCommand reads the selector of a `show bgp peer <selector> ...`.
var showPeerCommand = regexp.MustCompile(`^show bgp peer (\S+)`)

// readWait adds what one call waits on, when it calls a wait helper.
func (walk *observerWalk) readWait(name string, call *ast.CallExpr, bound map[string]string) {
	wait, ok := walk.source.waits[name]
	if !ok {
		return
	}
	switch wait.kind {
	case observerWaitSelector:
		last := wait.index
		if wait.variadic {
			last = len(call.Args) - 1
		}
		for index := wait.index; index <= last && index < len(call.Args); index++ {
			if selector, ok := walk.source.stringValue(call.Args[index], bound); ok {
				walk.answer.selectors = append(walk.answer.selectors, selector)
			}
		}
	case observerWaitFixed:
		// invoke adds the fixed peers when the helper is followed.
	case observerWaitCommand:
		if wait.index >= len(call.Args) {
			return
		}
		command, ok := walk.source.stringValue(call.Args[wait.index], bound)
		if !ok {
			return
		}
		if match := showPeerCommand.FindStringSubmatch(command); match != nil {
			walk.answer.selectors = append(walk.answer.selectors, match[1])
		}
	case observerWaitCount:
		if wait.gated {
			if wait.gate >= len(call.Args) || !isTrue(call.Args[wait.gate]) {
				return
			}
		}
		count := wait.count
		if count == 0 {
			if wait.index >= len(call.Args) {
				return
			}
			value, ok := walk.source.intValue(call.Args[wait.index], bound)
			if !ok {
				return
			}
			count = value
		}
		if count > walk.answer.count {
			walk.answer.count = count
		}
	case observerWaitPoll:
		for _, arg := range call.Args {
			predicate, ok := ast.Unparen(arg).(*ast.FuncLit)
			if !ok {
				continue
			}
			ast.Inspect(predicate.Body, func(node ast.Node) bool {
				inner, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				walk.pollRead(inner, bound)
				return true
			})
		}
	case observerWaitPollRead:
		// Read only inside a poll predicate, by pollRead.
	case observerWaitUnspecified:
	}
}

// pollRead adds the peer one call inside a poll predicate reads: the
// selector a poll-read helper is given, or the `show bgp peer <selector>`
// command the call sends. A string the predicate holds but sends nowhere
// reads nothing.
func (walk *observerWalk) pollRead(call *ast.CallExpr, bound map[string]string) {
	if callee, ok := call.Fun.(*ast.Ident); ok {
		if read, ok := walk.source.waits[callee.Name]; ok && read.kind == observerWaitPollRead && read.index < len(call.Args) {
			if selector, ok := walk.source.stringValue(call.Args[read.index], bound); ok {
				walk.answer.selectors = append(walk.answer.selectors, selector)
			}
		}
	}
	for _, arg := range call.Args {
		text, ok := walk.source.stringValue(arg, bound)
		if !ok {
			continue
		}
		if match := showPeerCommand.FindStringSubmatch(text); match != nil {
			walk.answer.selectors = append(walk.answer.selectors, match[1])
		}
	}
}

// isTrue reports whether an argument is the literal `true`.
func isTrue(expr ast.Expr) bool {
	ident, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && ident.Name == "true"
}

// anyPeer is the selector that sums every peer.
const anyPeer = "*"

// scriptedPeer is one ze peer a .ci scripts the far side of.
type scriptedPeer struct {
	name    string
	address string
}

// waitedOn reports whether waited names this peer, by name or by address. The
// "*" selector names it only when the .ci scripts no other peer, because a sum
// over several peers can reach its minimum from one of them.
func (peer scriptedPeer) waitedOn(waited []string, scripted int) bool {
	for _, entry := range waited {
		if entry == peer.name {
			return true
		}
		if entry == peer.address {
			return true
		}
		if entry == anyPeer && scripted == 1 {
			return true
		}
	}
	return false
}

// waitedOnBy reports whether a peer is among what a registration waits on: by
// name or address, by "*" when the .ci scripts no other peer, or by a count of
// distinct peers that covers every scripted one.
func (peer scriptedPeer) waitedOnBy(waited observerWaited, scripted int) bool {
	if waited.count >= scripted {
		return true
	}
	return peer.waitedOn(waited.selectors, scripted)
}

var (
	// peerCommand matches a `le test peer` command line, capturing its
	// arguments and the stdin block it runs.
	peerCommand = regexp.MustCompile(`(?m)^cmd=[a-z]+:.*exec=le test peer (.*):stdin=([A-Za-z0-9_-]+)\s*$`)
	bindFlag    = regexp.MustCompile(`--bind\s+(\S+)`)
	portFlag    = regexp.MustCompile(`--port\s+(\S+)`)
	dialFlag    = regexp.MustCompile(`--dial\s+(\S+):\$PORT`)
	peerBlock   = regexp.MustCompile(`(?m)^\s*peer\s+([A-Za-z0-9_.-]+)\s*\{`)
	remoteIP    = regexp.MustCompile(`remote\s*\{\s*ip\s+([^\s;}]+)`)
	remotePort  = regexp.MustCompile(`remote\s*\{[^}]*\bport\s+([^\s;}]+)`)
	localIP     = regexp.MustCompile(`local\s*\{\s*ip\s+([^\s;}]+)`)
	// stdinPeerBlock matches a .ci stdin block that scripts a BGP peer:
	// `stdin=peer:` or `stdin=peer<N>:`.
	stdinPeerBlock = regexp.MustCompile(`(?m)^stdin=(peer\d*):`)
)

// scriptedPeers answers the ze peers a .ci scripts, each once. A peer a
// strict family's .ci scripts must resolve to a configured peer; in any other
// family an unresolved one (a peer the scenario creates at run time) is named
// by the address it binds or dials.
func scriptedPeers(fixture, ci string, families []string) ([]scriptedPeer, error) {
	strict, stdinNames := observerFamilyFlags(families)
	var peers []scriptedPeer
	add := func(peer scriptedPeer) {
		for _, known := range peers {
			if known.name == peer.name {
				return
			}
		}
		peers = append(peers, peer)
	}
	for _, command := range peerCommand.FindAllStringSubmatch(ci, -1) {
		arguments := command[1]
		var peer scriptedPeer
		var found bool
		address := addrLoopback
		if dial := dialFlag.FindStringSubmatch(arguments); dial != nil {
			address = dial[1]
			peer, found = configuredPeer(ci, localIP, address)
		} else {
			if bind := bindFlag.FindStringSubmatch(arguments); bind != nil {
				address = bind[1]
			}
			peer, found = boundPeer(ci, arguments, address)
		}
		if !found {
			if strict {
				return nil, errors.New(fixture + " runs `le test peer " + arguments + "` for stdin=" + command[2] + ", and no configured peer matches its address")
			}
			peer = scriptedPeer{name: address, address: address}
		}
		add(peer)
	}
	if stdinNames {
		for _, match := range stdinPeerBlock.FindAllStringSubmatch(ci, -1) {
			name := match[1]
			if name == "peer" {
				name = "peer1"
			}
			add(scriptedPeer{name: name})
		}
	}
	return peers, nil
}

// boundPeer answers the configured peer a listening `le test peer` stands
// for: the one whose remote address is the bound address, or, for a wildcard
// bind that accepts whichever address ze dials, the one whose remote port is
// the command's port.
func boundPeer(ci, arguments, address string) (scriptedPeer, bool) {
	if address != "0.0.0.0" && address != "::" {
		return configuredPeer(ci, remoteIP, address)
	}
	port := portFlag.FindStringSubmatch(arguments)
	if port == nil {
		return scriptedPeer{}, false
	}
	return configuredPeer(ci, remotePort, port[1])
}

// configuredPeer answers the first configured peer whose value, as field
// reads it from the peer's block, is value.
func configuredPeer(ci string, field *regexp.Regexp, value string) (scriptedPeer, bool) {
	blocks := peerBlock.FindAllStringSubmatchIndex(ci, -1)
	for index, block := range blocks {
		end := len(ci)
		if index+1 < len(blocks) {
			end = blocks[index+1][0]
		}
		match := field.FindStringSubmatch(ci[block[1]:end])
		if len(match) < 2 || match[1] != value {
			continue
		}
		remote := remoteIP.FindStringSubmatch(ci[block[1]:end])
		if remote == nil {
			continue
		}
		return scriptedPeer{name: ci[block[2]:block[3]], address: remote[1]}, true
	}
	return scriptedPeer{}, false
}
