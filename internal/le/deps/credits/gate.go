// Design: docs/architecture/third-party-credits.md -- the commit gate
// Design: docs/contributing/dependency-approval.md -- the commit gate on third-party imports
// Overview: credits.go -- the declaration and the record the gate reads
// Related: register.go -- where the gate registers as a change-set check

package credits

import (
	"path"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
	"github.com/ze-software/ze/internal/le/deps"
	repocompiles "github.com/ze-software/ze/internal/le/repo/compiles"
)

// toolingRoots are the trees whose Go files never ship: le itself, which
// TestNormalZeLinksNoInternalLe proves no shipped build links, and the test
// harness.
var toolingRoots = []string{"internal/le/", "internal/test/"}

// carryHint tells a refused commit what it must carry once update ran.
const carryHint = "run the command below, and carry " + DeclarationRel + " and " + DataRel + "/ in this commit:"

// ze point: repo-maintenance/directives/credit-every-module-the-shipped-binary-links
// judgeCommit is the `deps credits` change-set check. It judges only what the
// commit adds and runs no go command. It refuses a commit that adds, in a Go
// file that can ship, an import of a vendored module the generated data it
// leaves does not credit (unless the declaration it leaves names the module
// and the data was generated from that very declaration); that carries a
// licence or notice file of a credited work whose bytes the data it leaves
// does not hold; or that carries the declaration or the data while the data
// was generated from another declaration.
func judgeCommit(change changecheck.Change) (string, error) {
	view := commitView{change: change}
	var problems []string
	declarationProblem, err := view.declarationProblem()
	if err != nil {
		return "", err
	}
	if declarationProblem != "" {
		problems = append(problems, declarationProblem)
	}
	licenceProblems, err := view.licenceProblems()
	if err != nil {
		return "", err
	}
	problems = append(problems, licenceProblems...)
	importProblems, err := view.importProblems()
	if err != nil {
		return "", err
	}
	problems = append(problems, importProblems...)
	return strings.Join(problems, "\n\n"), nil
}

// commitView reads the tree as the commit leaves it. It is used by one
// judgeCommit call and is not safe for concurrent use.
type commitView struct {
	change changecheck.Change
	// loaded says data and declaration below were read.
	loaded          bool
	data            record
	dataPresent     bool
	dataProblem     string
	declaration     declaration
	declarationRaw  string
	declarationHeld bool
}

// leave reads one path as the commit leaves it: absent when it removes it,
// what it carries when it carries it, the parent's text otherwise.
func (v *commitView) leave(rel string) (string, bool, error) {
	if slices.Contains(v.change.Removed, rel) {
		return "", false, nil
	}
	if slices.Contains(v.change.Paths, rel) {
		return v.change.After(rel)
	}
	return v.change.Before(rel)
}

// carries answers whether the commit adds, changes or removes rel.
func (v *commitView) carries(rel string) bool {
	return slices.Contains(v.change.Paths, rel) || slices.Contains(v.change.Removed, rel)
}

// load reads the record and the declaration the commit leaves, once. A
// record that does not parse is kept as a problem, never as no record.
func (v *commitView) load() error {
	if v.loaded {
		return nil
	}
	v.loaded = true
	raw, present, err := v.leave(path.Join(DataRel, RecordName))
	if err != nil {
		return err
	}
	if present {
		rec, decodeErr := decodeRecord([]byte(raw))
		if decodeErr != nil {
			v.dataProblem = decodeErr.Error()
		} else {
			v.data, v.dataPresent = rec, true
		}
	}
	declarationRaw, held, err := v.leave(DeclarationRel)
	if err != nil {
		return err
	}
	if !held {
		return nil
	}
	// A declaration that does not parse is kept as a problem, the way a
	// record that does not parse is, never as no declaration.
	decl, parseErr := readDeclaration([]byte(declarationRaw))
	if parseErr != nil {
		v.dataProblem = parseErr.Error()
	} else {
		v.declaration, v.declarationRaw, v.declarationHeld = decl, declarationRaw, true
	}
	return nil
}

// generatedFromDeclaration answers whether the data the commit leaves was
// generated from the declaration it leaves.
func (v *commitView) generatedFromDeclaration() bool {
	if !v.dataPresent {
		return false
	}
	if !v.declarationHeld {
		return false
	}
	return v.data.Declaration == digest([]byte(v.declarationRaw))
}

// declarationProblem refuses a commit that carries the declaration or the
// data while the two do not match.
func (v *commitView) declarationProblem() (string, error) {
	touched := v.carries(DeclarationRel)
	for _, rel := range append(slices.Clone(v.change.Paths), v.change.Removed...) {
		if strings.HasPrefix(rel, DataRel+"/") {
			touched = true
		}
	}
	if !touched {
		return "", nil
	}
	if err := v.load(); err != nil {
		return "", err
	}
	if v.dataProblem != "" {
		return refusalText(DataRel+" or "+DeclarationRel+" as this commit leaves them cannot be read: "+v.dataProblem,
			"the binary embeds the data, so the commit gate judges it before it lands"), nil
	}
	if v.generatedFromDeclaration() {
		return "", nil
	}
	return refusalText("this commit carries "+DeclarationRel+" or "+DataRel+", and the data was not generated from that declaration",
		"the binary embeds the data, so a declaration edited without regenerating ships the credits it replaced"), nil
}

// licenceProblems refuses each carried licence or notice file of a credited
// work whose bytes the data the commit leaves does not hold, and each added
// licence file of a credited module the data does not copy.
func (v *commitView) licenceProblems() ([]string, error) {
	var problems []string
	for _, rel := range append(slices.Clone(v.change.Paths), v.change.Removed...) {
		if strings.HasPrefix(rel, DataRel+"/") {
			continue
		}
		if err := v.load(); err != nil {
			return nil, err
		}
		if !licenceNamed(rel) && len(v.recordedAs(rel)) == 0 {
			continue
		}
		problem, err := v.licenceProblem(rel)
		if err != nil {
			return nil, err
		}
		if problem != "" {
			problems = append(problems, problem)
		}
	}
	return problems, nil
}

// licenceNamed answers whether rel is named as a module's licence or notice
// file, or as a bundle's companion file. A work's file with another name
// (third_party/web/htmx/htmx.LICENSE) is found by the record instead.
func licenceNamed(rel string) bool {
	if strings.HasSuffix(rel, companionSuffix) {
		return true
	}
	_, licence := licenceRank(path.Base(rel))
	return licence
}

// licenceProblem judges one carried licence candidate against the data the
// commit leaves.
func (v *commitView) licenceProblem(rel string) (string, error) {
	origins := v.recordedAs(rel)
	text, present, err := v.leave(rel)
	if err != nil {
		return "", err
	}
	if !present {
		if len(origins) == 0 {
			return "", nil
		}
		return refusalText("this commit removes "+rel+", which the generated data still copies for "+origins[0].work,
			"the binary would print a licence text the tree no longer holds"), nil
	}
	if len(origins) == 0 {
		if !strings.HasPrefix(rel, vendorRoot+"/") {
			return "", nil
		}
		module := strings.TrimPrefix(path.Dir(rel), vendorRoot+"/")
		work, credited := v.data.work(module)
		if !credited {
			return "", nil
		}
		if work.Kind != kindModule {
			return "", nil
		}
		return refusalText("this commit adds "+rel+", a licence or notice file of the credited module "+module+", which the generated data does not copy",
			"the binary must carry every licence and notice file a shipped module carries"), nil
	}
	stored := textName([]byte(text))
	for _, origin := range origins {
		if origin.text == stored {
			continue
		}
		return refusalText("this commit changes "+rel+", a licence file of "+origin.work+", and the generated data holds other bytes for it",
			"the binary would ship the licence text the work no longer carries"), nil
	}
	return "", nil
}

// recordedOrigin is one place the data the commit leaves copies a file.
type recordedOrigin struct {
	work string
	text string
}

// recordedAs answers every work file the data the commit leaves copies from
// rel.
func (v *commitView) recordedAs(rel string) []recordedOrigin {
	var origins []recordedOrigin
	for index := range v.data.Works {
		work := &v.data.Works[index]
		for _, file := range work.Files {
			if file.Origin == rel {
				origins = append(origins, recordedOrigin{work: work.Name, text: file.Text})
			}
		}
	}
	return origins
}

// importProblems refuses each module a shippable carried Go file newly
// imports that the data the commit leaves does not credit.
func (v *commitView) importProblems() ([]string, error) {
	anchors := anchorDirs()
	byModule := make(map[string][]string)
	var modules []string
	for _, rel := range v.change.Paths {
		added, err := v.addedImports(rel, anchors)
		if err != nil {
			return nil, err
		}
		if len(added) == 0 {
			continue
		}
		vendorModules, _, err := v.leave(deps.VendorModulesRel)
		if err != nil {
			return nil, err
		}
		for _, pkg := range added {
			// A package no vendored module provides is the dependency
			// ledger's refusal (dependencyRefusal), not this one.
			module := deps.ModuleOf(vendorModules, pkg)
			if module == "" {
				continue
			}
			if !slices.Contains(modules, module) {
				modules = append(modules, module)
			}
			byModule[module] = append(byModule[module], rel+" imports "+pkg)
		}
	}
	if len(modules) == 0 {
		return nil, nil
	}
	if err := v.load(); err != nil {
		return nil, err
	}
	slices.Sort(modules)
	var problems []string
	for _, module := range modules {
		if v.credits(module) {
			continue
		}
		problems = append(problems, uncreditedModule(module, byModule[module]))
	}
	return problems, nil
}

// credits answers whether the commit leaves module credited: a record in
// the data, or an entry in the declaration the data was generated from, which
// update found no shipped flavor links.
func (v *commitView) credits(module string) bool {
	if _, held := v.data.work(module); held {
		return true
	}
	if !v.generatedFromDeclaration() {
		return false
	}
	all := v.declaration.entries()
	for index := range all {
		if all[index].kind == kindModule && all[index].name == module {
			return true
		}
	}
	return false
}

// addedImports answers the third-party imports a shippable carried Go file
// adds against its parent text.
func (v *commitView) addedImports(rel string, anchors []string) ([]string, error) {
	if !shippable(rel, anchors) {
		return nil, nil
	}
	judged, err := deps.JudgedWith(rel, func(moduleFile string) (bool, error) {
		_, present, err := v.leave(moduleFile)
		return present, err
	})
	if err != nil {
		return nil, err
	}
	if !judged {
		return nil, nil
	}
	after, present, err := v.change.After(rel)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	afterImports, parsed := parsedImports(rel, after)
	if !parsed {
		// A carried file that does not parse is refused by the dependency
		// gate (dependencyRefusal) before this check runs.
		return nil, nil
	}
	var beforeImports []string
	before, present, err := v.change.Before(rel)
	if err != nil {
		return nil, err
	}
	if present {
		// A parent that does not parse leaves beforeImports empty, so every
		// carried import is judged as added: the strict answer.
		if imports, parentParsed := parsedImports(rel, before); parentParsed {
			beforeImports = imports
		}
	}
	goMod, _, err := v.leave("go.mod")
	if err != nil {
		return nil, err
	}
	module, err := deps.ModulePath(goMod)
	if err != nil {
		return nil, err
	}
	return deps.Added(module, beforeImports, afterImports), nil
}

// parsedImports answers the imports of one Go file's text, and false when
// the text does not parse: the caller decides what an unparsable file means
// for its judgment.
func parsedImports(rel, text string) ([]string, bool) {
	imports, err := deps.Imports(rel, []byte(text))
	if err != nil {
		return nil, false
	}
	return imports, true
}

// anchorDirs answers the directories of the shipped flavors' anchor packages,
// from the build matrix.
func anchorDirs() []string {
	var dirs []string
	for _, flavor := range repocompiles.Flavors() {
		dir := strings.TrimPrefix(flavor.Anchor, "./")
		if !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// shippable answers whether a Go file can be linked into a shipped build: a
// non-test file under internal/ or pkg/, or under a shipped flavor's anchor
// directory in cmd/, outside the tooling roots. It approximates what a build
// links without running one: a file there that only a tooling tag compiles
// is judged as if it shipped, and the gate's repair still accepts it, because
// a declared module whose data update regenerated is credited whether or not
// it links.
func shippable(rel string, anchors []string) bool {
	if !strings.HasSuffix(rel, ".go") {
		return false
	}
	if strings.HasSuffix(rel, "_test.go") {
		return false
	}
	for _, tooling := range toolingRoots {
		if strings.HasPrefix(rel, tooling) {
			return false
		}
	}
	if strings.HasPrefix(rel, "internal/") {
		return true
	}
	if strings.HasPrefix(rel, "pkg/") {
		return true
	}
	for _, anchor := range anchors {
		if strings.HasPrefix(rel, anchor+"/") {
			return true
		}
	}
	return false
}

// uncreditedModule is the refusal of one newly imported module.
func uncreditedModule(module string, uses []string) string {
	var text textbuf.Buffer
	text.Str("deps credits: this commit makes the shipped binary import ").Str(module).Str(", which it does not credit\n")
	for _, use := range uses {
		text.Str("  ").Str(use).Byte('\n')
	}
	text.Str("  a module the binary links must carry its licence and holder, which ze thanks prints\n")
	text.Str("  add its entry to ").Str(DeclarationRel).Str(" (its curated category when Ze imports it directly, the linked list otherwise),\n")
	text.Str("  then ").Str(carryHint).Byte('\n')
	text.Str("    ").Str(updateCommand)
	return text.String()
}

// refusalText renders a gate refusal: the problem, why, and the command
// alone on the last line.
func refusalText(problem, why string) string {
	var text textbuf.Buffer
	text.Str("deps credits: ").Str(problem).Byte('\n')
	text.Str("  ").Str(why).Byte('\n')
	text.Str("  ").Str(carryHint).Byte('\n')
	text.Str("    ").Str(updateCommand)
	return text.String()
}
