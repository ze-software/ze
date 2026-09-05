// The bootstrap-distance check. Every protocol that stamps an administrative
// distance on a locrib.Path carries its own constant for the window before
// sysrib publishes the declaration, and that constant has to hold the same
// number as the YANG leaf it stands in for.
//
// The two sets agree today, and nothing in the tree could notice if they
// stopped. parseAdminDistanceConfig("{}") publishes the schema defaults at
// process start, so every producer reads the declared value from the seam and
// its own constant is never read. Delete the publish and the suite stays green,
// because the numbers happen to match. The publish earns its keep only when
// they DISAGREE, and a disagreement is exactly what no test could see.
//
// The population is DERIVED, never listed: a package is a producer because it
// calls OrDefault or Of on the seam (internal/core/rib/distance), and the
// protocols it stamps are the declared names its seam-reading files write as
// string literals. A hand-written table of the four producers would be a third
// declaration of the same fact, and the failure it invites is the one this
// repository has already paid for twice: a gate that excludes part of its own
// population and prints green (plan/journal/gate-excludes-part-of-its-population.md).
//
// VALIDATES: every producer's bootstrap constant equals the YANG default of the
// protocol it stamps; every protocol a producer stamps is a leaf the schema
// declares; and every bootstrap constant in the tree belongs to a package that
// reads the seam.
// PREVENTS: the silent disagreement above, and the fail-open shape where a new
// producer, an unreadable file, an unresolvable constant or a broken walk
// leaves the check with nothing to compare and reports a clean tree.

package sysrib

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	// seamImportPath is the package every producer reads its distance from.
	// Importing it and calling OrDefault or Of is what MAKES a package a
	// producer here, so the population follows the code rather than a list.
	seamImportPath = "github.com/ze-software/ze/internal/core/rib/distance"

	// bootstrapPrefix is the name every bootstrap constant starts with:
	// DefaultAdminDistance in a package that stamps one protocol, and
	// DefaultAdminDistance<PROTOCOL> where it stamps more than one.
	bootstrapPrefix = "DefaultAdminDistance"
)

// producer is one package that stamps an administrative distance: whether it
// reads the seam, the declared protocol names its seam-reading files write, and
// the bootstrap constants it declares with the value of each.
type producer struct {
	readsSeam bool
	protocols map[string]bool
	constants map[string]int
}

func newProducer() *producer {
	return &producer{protocols: map[string]bool{}, constants: map[string]int{}}
}

// TestBootstrapDistancesMatchTheDeclaration runs the check over this tree.
func TestBootstrapDistancesMatchTheDeclaration(t *testing.T) {
	// The reference is the product's own resolution of the YANG defaults, so
	// the check cannot disagree with what sysrib publishes at process start.
	declared, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatalf("resolving the declared distances: %v", err)
	}
	if len(declared) == 0 {
		t.Fatal("the schema declared no distance at all, so there was nothing to compare")
	}

	for _, problem := range checkBootstraps(repoRootForBootstraps(t), declared) {
		t.Error(problem)
	}
}

// checkBootstraps reads every non-test Go file under root and answers one line
// for each disagreement it finds. The answer is empty when the tree agrees.
//
// Anything it cannot read, parse or resolve is a line too. A check that cannot
// see a producer must say so rather than pass, because an empty answer and a
// clean tree would otherwise be the same result (ai/rules/principles.md).
func checkBootstraps(root string, declared map[string]int) []string {
	producers := map[string]*producer{}
	problems := scanForProducers(root, declared, producers)

	seamReaders := 0
	for _, prod := range producers {
		if prod.readsSeam {
			seamReaders++
		}
	}
	if seamReaders == 0 {
		return append(problems, fmt.Sprintf(
			"no package under %s reads the distance seam (%s): the walk found nothing, "+
				"so the check compared nothing", root, seamImportPath))
	}

	for _, dir := range sortedNames(producers) {
		problems = append(problems, checkProducer(dir, producers[dir], declared)...)
	}
	return problems
}

// scanForProducers walks the tree and fills producers, one entry per directory.
func scanForProducers(root string, declared map[string]int, producers map[string]*producer) []string {
	var problems []string

	for _, top := range []string{"internal", "pkg", "cmd"} {
		dir := filepath.Join(root, top)
		if _, err := os.Stat(dir); err != nil {
			// A tree without one of the three is a fixture, not a defect. A
			// tree without any producer at all is caught by the seam-reader
			// count in checkBootstraps.
			continue
		}

		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: walk stopped: %v", path, err))
				return nil
			}
			if entry.IsDir() {
				if skipDirForBootstraps(entry.Name()) {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			// A test's fallback is a fixture rather than a bootstrap, and this
			// file's own fixtures name the seam.
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			problems = append(problems, scanFile(path, declared, producers)...)
			return nil
		})
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: walk failed: %v", dir, err))
		}
	}
	return problems
}

// scanFile records what one file says about its package: whether it reads the
// seam, which declared protocols it names, and which bootstrap constants it
// declares.
func scanFile(path string, declared map[string]int, producers map[string]*producer) []string {
	source, err := os.ReadFile(path)
	if err != nil {
		return []string{fmt.Sprintf("%s: unreadable, so its producer could not be checked: %v", path, err)}
	}

	// Only two things make a file interesting, and both are visible in its
	// text: the seam's import path and the constant name. Reading the bytes and
	// parsing the few that match keeps the walk cheap.
	text := string(source)
	if !strings.Contains(text, seamImportPath) && !strings.Contains(text, bootstrapPrefix) {
		return nil
	}

	file, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
	if err != nil {
		return []string{fmt.Sprintf("%s: unparseable, so its producer could not be checked: %v", path, err)}
	}

	dir := filepath.Dir(path)
	prod, ok := producers[dir]
	if !ok {
		prod = newProducer()
		producers[dir] = prod
	}

	problems := collectConstants(path, file, prod)

	alias := seamAlias(file)
	if alias == "" {
		return problems
	}
	if !readsSeam(file, alias) {
		// sysrib itself imports the seam to PUBLISH on it. A publisher stamps
		// nothing, so it owes no bootstrap constant.
		return problems
	}

	prod.readsSeam = true
	for _, name := range stringLiterals(file) {
		if _, ok := declared[name]; ok {
			prod.protocols[name] = true
		}
	}
	return problems
}

// collectConstants records every bootstrap constant the file declares. A
// constant whose value is not a plain integer is reported rather than skipped:
// the check exists to compare that number.
func collectConstants(path string, file *ast.File, prod *producer) []string {
	var problems []string

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range value.Names {
				if !strings.HasPrefix(name.Name, bootstrapPrefix) {
					continue
				}
				number, err := constantValue(value, i)
				if err != nil {
					problems = append(problems, fmt.Sprintf("%s: %s %v", path, name.Name, err))
					continue
				}
				prod.constants[name.Name] = number
			}
		}
	}
	return problems
}

// constantValue reads the integer a bootstrap constant holds.
func constantValue(spec *ast.ValueSpec, index int) (int, error) {
	if index >= len(spec.Values) {
		return 0, errors.New("has no value of its own, so its distance could not be read")
	}
	literal, ok := spec.Values[index].(*ast.BasicLit)
	if !ok || literal.Kind != token.INT {
		return 0, errors.New("is not a plain integer, so its distance could not be read")
	}
	number, err := strconv.Atoi(literal.Value)
	if err != nil {
		return 0, fmt.Errorf("holds %s, which is not a number: %w", literal.Value, err)
	}
	return number, nil
}

// seamAlias returns the name the file uses for the seam package, or "" when it
// does not import it.
func seamAlias(file *ast.File) string {
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != seamImportPath {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return "distance"
	}
	return ""
}

// readsSeam reports whether the file asks the seam for a distance. Set is the
// publisher's call and does not count.
func readsSeam(file *ast.File, alias string) bool {
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || pkg.Name != alias {
			return true
		}
		if selector.Sel.Name == "OrDefault" || selector.Sel.Name == "Of" {
			found = true
		}
		return true
	})
	return found
}

// stringLiterals returns every string literal in the file.
func stringLiterals(file *ast.File) []string {
	var values []string
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		values = append(values, value)
		return true
	})
	return values
}

// checkProducer compares one package's bootstrap constants against the
// declaration, and reports anything it could not pair in either direction.
func checkProducer(dir string, prod *producer, declared map[string]int) []string {
	if !prod.readsSeam {
		if len(prod.constants) == 0 {
			return nil
		}
		return []string{fmt.Sprintf(
			"%s: declares %s but never reads the distance seam (%s), so the constant "+
				"is the only distance this package can stamp and the declaration cannot reach it",
			dir, strings.Join(sortedNames(prod.constants), ", "), seamImportPath)}
	}

	if len(prod.protocols) == 0 {
		return []string{fmt.Sprintf(
			"%s: reads the distance seam but names no declared protocol, so its bootstrap "+
				"value could not be checked against any leaf of rib { distance { } }", dir)}
	}

	var problems []string
	paired := map[string]bool{}

	for _, protocol := range sortedNames(prod.protocols) {
		name := bootstrapNameFor(protocol, prod)
		if name == "" {
			problems = append(problems, fmt.Sprintf(
				"%s: stamps %q but declares no %s constant for it, so nothing pins its "+
					"bootstrap value to the %s default of %d",
				dir, protocol, bootstrapPrefix, protocol, declared[protocol]))
			continue
		}
		paired[name] = true

		if prod.constants[name] != declared[protocol] {
			problems = append(problems, fmt.Sprintf(
				"%s: %s is %d but rib { distance { %s } } defaults to %d; the bootstrap value "+
					"a producer stamps before the declaration is published MUST be the declared one",
				dir, name, prod.constants[name], protocol, declared[protocol]))
		}
	}

	for _, name := range sortedNames(prod.constants) {
		if paired[name] {
			continue
		}
		problems = append(problems, fmt.Sprintf(
			"%s: %s pairs with no protocol this package stamps (%s), so no leaf of "+
				"rib { distance { } } could be found to check it against",
			dir, name, strings.Join(sortedNames(prod.protocols), ", ")))
	}
	return problems
}

// bootstrapNameFor returns the constant that carries protocol's bootstrap
// value, or "" when none does. A constant named DefaultAdminDistance<PROTOCOL>
// carries the protocol it names. A package that stamps one protocol and
// declares one constant pairs them whatever the constant is called, which is
// how the IS-IS and OSPF producers spell it.
func bootstrapNameFor(protocol string, prod *producer) string {
	for name := range prod.constants {
		suffix := strings.TrimPrefix(name, bootstrapPrefix)
		if strings.EqualFold(suffix, protocol) {
			return name
		}
	}
	if len(prod.protocols) == 1 && len(prod.constants) == 1 {
		for name := range prod.constants {
			return name
		}
	}
	return ""
}

// sortedNames returns a map's keys in a fixed order, so the check answers the
// same lines in the same order on every run.
func sortedNames[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

// skipDirForBootstraps reports whether a directory holds no first-party
// producer.
func skipDirForBootstraps(name string) bool {
	return name == "vendor" || name == "testdata" || name == "node_modules" || name == ".git"
}

// repoRootForBootstraps walks up from the working directory to the tree root.
// go test runs in the package directory, so the walk starts inside the tree
// whatever the caller's working directory was.
func repoRootForBootstraps(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod in any parent of the working directory, so the tree root is unknown")
		}
		dir = parent
	}
}

// fixtureDeclared is the declaration the fixture cases below are checked
// against. It is the shape parseAdminDistanceConfig answers, with the numbers
// fixed so a case can disagree with them on purpose.
var fixtureDeclared = map[string]int{"connected": 0, "static": 10, "ebgp": 20, "ospf": 110, "isis": 115, "ibgp": 200}

// The fixture producer, spelled the way the real ones are: an import of the
// seam under an alias, a call to OrDefault, and a constant beside it.
const fixtureProducer = `package fake

import ribdistance "` + seamImportPath + `"

const DefaultAdminDistance uint8 = %d

func stamp(bootstrap uint8) uint8 { return ribdistance.OrDefault(%q, bootstrap) }
`

// TestCheckBootstrapsFindsEachDisagreement drives the check over fixture trees,
// one per way the two sets of numbers can come apart. Every case names what it
// expects to be reported, and the agreeing case proves the check is quiet when
// the tree is right, which is what makes the red cases mean anything.
func TestCheckBootstrapsFindsEachDisagreement(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		expect string // a substring of the one problem, or "" for no problem at all
	}{
		{
			name:   "a producer that agrees with the declaration is quiet",
			files:  map[string]string{"internal/plugins/fake/install.go": fmt.Sprintf(fixtureProducer, 110, "ospf")},
			expect: "",
		},
		{
			name:   "a bootstrap constant that disagrees is reported",
			files:  map[string]string{"internal/plugins/fake/install.go": fmt.Sprintf(fixtureProducer, 111, "ospf")},
			expect: "DefaultAdminDistance is 111 but rib { distance { ospf } } defaults to 110",
		},
		{
			name:   "a protocol the schema does not declare is reported",
			files:  map[string]string{"internal/plugins/fake/install.go": fmt.Sprintf(fixtureProducer, 110, "rip")},
			expect: "names no declared protocol",
		},
		{
			name: "a bootstrap constant in a package that never reads the seam is reported",
			files: map[string]string{
				"internal/plugins/fake/install.go":   fmt.Sprintf(fixtureProducer, 110, "ospf"),
				"internal/plugins/other/distance.go": "package other\n\nconst DefaultAdminDistance uint8 = 115\n",
			},
			expect: "never reads the distance seam",
		},
		{
			name: "a producer that stamps two protocols is checked on each constant",
			files: map[string]string{"internal/component/fake/rib.go": `package fake

import ribdistance "` + seamImportPath + `"

const (
	DefaultAdminDistanceEBGP uint8 = 20
	DefaultAdminDistanceIBGP uint8 = 201
)

func stamp(external bool) uint8 {
	protocol, bootstrap := "ibgp", DefaultAdminDistanceIBGP
	if external {
		protocol, bootstrap = "ebgp", DefaultAdminDistanceEBGP
	}
	return ribdistance.OrDefault(protocol, bootstrap)
}
`},
			expect: "DefaultAdminDistanceIBGP is 201 but rib { distance { ibgp } } defaults to 200",
		},
		{
			name:   "a tree with no producer at all is reported rather than passed",
			files:  map[string]string{"internal/plugins/fake/install.go": "package fake\n"},
			expect: "the walk found nothing",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := writeFixtureTree(t, testCase.files)

			problems := checkBootstraps(root, fixtureDeclared)

			if testCase.expect == "" {
				if len(problems) > 0 {
					t.Fatalf("expected a quiet tree, got %v", problems)
				}
				return
			}
			if len(problems) != 1 {
				t.Fatalf("expected one problem naming %q, got %v", testCase.expect, problems)
			}
			if !strings.Contains(problems[0], testCase.expect) {
				t.Fatalf("expected a problem naming %q, got %q", testCase.expect, problems[0])
			}
		})
	}
}

// TestCheckBootstrapsReportsAnUnparseableFile proves the check fails closed on
// a file it cannot read as Go. A producer hiding behind a parse error must not
// look like a tree with no producer.
func TestCheckBootstrapsReportsAnUnparseableFile(t *testing.T) {
	root := writeFixtureTree(t, map[string]string{
		"internal/plugins/fake/install.go":   fmt.Sprintf(fixtureProducer, 110, "ospf"),
		"internal/plugins/broken/install.go": "package broken\n\nconst DefaultAdminDistance uint8 = (\n",
	})

	problems := checkBootstraps(root, fixtureDeclared)

	if len(problems) != 1 || !strings.Contains(problems[0], "unparseable") {
		t.Fatalf("expected the unparseable file to be reported, got %v", problems)
	}
}

// writeFixtureTree writes files, keyed by their path under the tree root, into
// a fresh directory and returns that root.
func writeFixtureTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("creating %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	return root
}
