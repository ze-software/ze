// Related: parity.go -- every typed dispatcher row is judged in Go
// Related: fixtures.go -- typed fixtures and actual-hook drift contracts
//
// VALIDATES: the native hook selftest owns all 208 dispatcher rows, all 456
// Results.check sites, and their 607 concrete runtime fixture identities from
// the typed native catalog.
// PREVENTS: missing categories, fixtures, messages, exits, or hook source drift.
package aihooks

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/hookruntime"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func hookCheckout(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	return root
}

func TestParityMapsEveryTypedDispatcherRow(t *testing.T) {
	results, population, golden := runParity(hookCheckout(t))
	want := Population{
		Bash:          bashRowsExpected,
		WriteEdit:     writeEditRowsExpected,
		Weakening:     weakeningRowsExpected,
		PostWriteEdit: postWriteEditRowsExpected,
	}
	if !reflect.DeepEqual(population, want) {
		t.Fatalf("population = %+v, want %+v", population, want)
	}
	if len(golden) != 209 {
		t.Fatalf("dispatcher fixtures = %d, want 209", len(golden))
	}
	seen := make(map[string]struct{}, len(golden))
	exitCounts := [3]int{}
	for _, fixture := range golden {
		if !fixture.Passed {
			t.Errorf("%s %q: native code %d, expected %d", fixture.Table, fixture.Name,
				fixture.NativeCode, fixture.ExpectedCode)
		}
		key := fixture.Table + "\x00" + fixture.Name
		if _, exists := seen[key]; exists {
			t.Errorf("duplicate dispatcher identity %q", key)
		}
		seen[key] = struct{}{}
		exitCounts[fixture.ExpectedCode]++
	}
	if exitCounts != [3]int{76, 9, 124} {
		t.Fatalf("dispatcher exit populations = %v, want [76 9 124]", exitCounts)
	}
	for _, result := range results {
		if !result.Passed {
			t.Errorf("%s: %s", result.Name, result.Message)
		}
	}
}

func TestBashNativeStatementBoundariesMatchDispatcher(t *testing.T) {
	cases := []struct {
		name    string
		command string
		code    int
	}{
		{
			name: "later unbounded loop is still refused",
			command: "timeout 60 bash -c 'until [ -f a ]; do sleep 1; done'; " +
				"while true; do sleep 5; done",
			code: 2,
		},
		{
			name:    "loop words in a grep pattern are text",
			command: "grep -rn 'until ! pgrep' ai/rules",
			code:    0,
		},
		{
			name:    "run-shaped quoted scratch redirect is refused",
			command: `bash -c "./le doc check all > tmp/out.log"`,
			code:    2,
		},
	}
	tree := bashGoldenProbeTree(t)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, output, err := bashHookVerdict(tree, test.command)
			if err != nil {
				t.Fatalf("the hook could not judge %q: %v", test.command, err)
			}
			if got != test.code {
				t.Errorf("hook verdict on %q = %d, want %d\n%s", test.command, got, test.code, output)
			}
		})
	}
}

// bashGoldenProbeTree builds the tree the bash rows are judged over, removed
// when the test ends.
func bashGoldenProbeTree(t *testing.T) string {
	t.Helper()
	tree, err := probeTree("bash-golden", bashGoldenTree)
	if tree != "" {
		t.Cleanup(func() { _ = os.RemoveAll(tree) })
	}
	if err != nil {
		t.Fatalf("build the bash golden tree: %v", err)
	}
	return tree
}

// TestBashGoldenIsJudgedByTheHook asks the BASH_GOLDEN judge for every row and
// asks the hook's own pretool-bash action (hookruntime.ProbeBash) for the
// same row over the same tree, from a shell environment set to mislead it.
//
// VALIDATES: each row's verdict and output are the hook's; each row's expected
// code is that verdict; every row the hook refuses carries the hook's own
// refusal text, which no hand model writes; the environment the probe pins
// (GOFLAGS, CDPATH, CLAUDE_CODE_FORK_SUBAGENT, CLAUDE_PROJECT_DIR) is put back.
// PREVENTS: a golden judged by a model of the hook, which stays green when the
// hook's verdict moves (the `go build ./...` row stayed at 0 after the build
// guard began refusing it), and a verdict that depends on the caller's shell.
func TestBashGoldenIsJudgedByTheHook(t *testing.T) {
	tree := bashGoldenProbeTree(t)
	// A root the probe did not pin would leave `@PROJECT@/tmp/notes.txt`
	// outside the tree the hook judges, and the scratch row would pass.
	misleading := map[string]string{
		"GOFLAGS":                   "-toolexec=probe",
		"CDPATH":                    t.TempDir(),
		"CLAUDE_CODE_FORK_SUBAGENT": "1",
		"CLAUDE_PROJECT_DIR":        t.TempDir(),
	}
	for name, value := range misleading {
		t.Setenv(name, value)
	}
	table := parityCatalog[0]
	if table.key != "BASH_GOLDEN" {
		t.Fatalf("parityCatalog[0] is %s, want BASH_GOLDEN", table.key)
	}
	for _, fixture := range table.fixtures {
		got, output, err := table.judge(tree, fixture.name)
		if err != nil {
			t.Errorf("%q: the judge could not answer: %v", fixture.name, err)
			continue
		}
		command := strings.ReplaceAll(fixture.name, bashGoldenProject, tree)
		hookCode, hookOutput, err := hookruntime.ProbeBash(command, tree)
		if err != nil {
			t.Errorf("%q: the hook could not answer: %v", fixture.name, err)
			continue
		}
		if got != hookCode || output != hookOutput {
			t.Errorf("%q: judge answered %d %q, the hook %d %q", fixture.name, got, output, hookCode, hookOutput)
		}
		if got != fixture.expectedCode {
			t.Errorf("%q: the hook answers %d, the row expects %d\n%s", fixture.name, got, fixture.expectedCode, output)
		}
		if got == 2 && strings.TrimSpace(output) == "" {
			t.Errorf("%q: refused with no text, so the verdict did not come from the hook", fixture.name)
		}
	}
	for name, value := range misleading {
		if current := os.Getenv(name); current != value {
			t.Errorf("%s = %q after the probes, want %q put back", name, current, value)
		}
	}
}

func TestEveryBehavioralCategoryDiscriminates(t *testing.T) {
	for _, category := range fixtureCategories {
		t.Run(category.name, func(t *testing.T) {
			allowed, err := categoryVerdict(category.name, category.allow)
			switch {
			case err != nil:
				t.Errorf("the allow probe could not run: %v", err)
			case !allowed:
				t.Errorf("allow probe was refused: %q", category.allow)
			}
			refused, err := categoryVerdict(category.name, category.refuse)
			switch {
			case err != nil:
				t.Errorf("the refusal probe could not run: %v", err)
			case refused:
				t.Errorf("refusal probe was allowed: %q", category.refuse)
			}
		})
	}
}

func TestEveryBehavioralCategoryHasTypedProducerMapping(t *testing.T) {
	counts := make(map[string]int, len(fixtureCategories))
	for _, fixture := range fixtureCatalog {
		counts[fixture.category]++
	}
	for _, category := range fixtureCategories {
		if category.runner == "" {
			t.Errorf("%s has no typed producer mapping", category.name)
		}
		if counts[category.name] == 0 {
			t.Errorf("%s has no typed fixtures", category.name)
		}
	}
	if len(counts) != len(fixtureCategories) {
		t.Fatalf("fixture categories = %d, want %d", len(counts), len(fixtureCategories))
	}
}

func TestNativeWeakeningProducerBindings(t *testing.T) {
	wantCategories := map[string]struct{}{
		"rfc-test-guard": {},
		"weakened-hatch": {},
		"rfc-approval":   {},
	}
	for _, category := range fixtureCategories {
		if _, governed := wantCategories[category.name]; !governed {
			continue
		}
		if category.runner != "test-weakened proposed" ||
			category.owner != "internal/le/hookruntime/writeedit.go" ||
			category.evidence != "func writeWeakening(" {
			t.Errorf("%s producer binding = %+v", category.name, category)
		}
		delete(wantCategories, category.name)
	}
	if len(wantCategories) != 0 {
		t.Fatalf("missing native producer categories: %v", wantCategories)
	}
	if len(fixtureProducerBoundaries) != 3 {
		t.Fatalf("native producer boundaries = %d, want 3", len(fixtureProducerBoundaries))
	}
	paths, err := hookSourceFiles(hookCheckout(t))
	if err != nil {
		t.Fatal(err)
	}
	sourcePaths := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		sourcePaths[path] = struct{}{}
	}
	for _, boundary := range fixtureProducerBoundaries {
		if boundary.actionOwner != "internal/le/test/weakened/actions.go" ||
			boundary.actionEvidence != `Verb:   "proposed"` ||
			boundary.nativeOwner != "internal/le/test/weakened/proposed.go" ||
			boundary.nativeEvidence == "" {
			t.Errorf("%s native boundary = %+v", boundary.category, boundary)
		}
		if _, exists := sourcePaths[boundary.actionOwner]; !exists {
			t.Errorf("%s action owner is absent from drift sources", boundary.category)
		}
		if _, exists := sourcePaths[boundary.nativeOwner]; !exists {
			t.Errorf("%s native owner is absent from drift sources", boundary.category)
		}
	}
}

func TestFixtureCatalogExactPopulationAndContent(t *testing.T) {
	result := checkFixturePopulation(
		fixtureCategories[:], fixtureSites[:], fixtureCatalog,
	)
	if !result.Passed {
		t.Fatalf("fixture catalog: %s", result.Message)
	}
	if len(fixtureSites) != fixtureSitesExpected {
		t.Fatalf("Results.check sites = %d, want %d", len(fixtureSites), fixtureSitesExpected)
	}
	if len(fixtureCatalog) != fixtureChecksExpected {
		t.Fatalf("expanded fixtures = %d, want %d", len(fixtureCatalog), fixtureChecksExpected)
	}
	public := fixtureCases()
	if len(public) != len(fixtureCatalog) {
		t.Fatalf("reported fixtures = %d, want %d", len(public), len(fixtureCatalog))
	}
	for index, fixture := range fixtureCatalog {
		row := public[index]
		if row.Category != fixture.category || row.Name != fixture.name ||
			row.ExpectedExit != fixture.expectedExit || row.Site != fixture.site ||
			row.Variant != fixture.variant || row.Producer == "" {
			t.Fatalf("reported fixture %d = %+v", index, row)
		}
		if len(row.Messages) != len(fixture.messages) {
			t.Fatalf("reported fixture %d messages = %d, want %d",
				index, len(row.Messages), len(fixture.messages))
		}
		for messageIndex, message := range fixture.messages {
			if row.Messages[messageIndex].Match != message.match ||
				row.Messages[messageIndex].Text != message.text {
				t.Fatalf("reported fixture %d message %d drifted", index, messageIndex)
			}
		}
	}
	exitCounts := map[int]int{}
	messageCounts := map[string]int{}
	identities := make(map[[2]int]struct{}, len(fixtureCatalog))
	names := make(map[string]int, len(fixtureCatalog))
	for _, fixture := range fixtureCatalog {
		exitCounts[fixture.expectedExit]++
		for _, message := range fixture.messages {
			messageCounts[message.match]++
		}
		identity := [2]int{fixture.site, fixture.variant}
		if _, exists := identities[identity]; exists {
			t.Fatalf("duplicate fixture identity %v", identity)
		}
		identities[identity] = struct{}{}
		names[fixture.name]++
	}
	if !reflect.DeepEqual(exitCounts, map[int]int{-1: 286, 0: 166, 1: 12, 2: 77}) {
		t.Fatalf("fixture exit populations = %v", exitCounts)
	}
	wantMessages := map[string]int{"contains": 83, "not-contains": 11, "equals": 3, "suffix": 3}
	if !reflect.DeepEqual(messageCounts, wantMessages) {
		t.Fatalf("fixture message populations = %v, want %v", messageCounts, wantMessages)
	}
	if len(names) != fixtureUniqueNamesExpected {
		t.Fatalf("unique fixture names = %d, want %d", len(names), fixtureUniqueNamesExpected)
	}
	for name, count := range names {
		if count != 1 {
			t.Fatalf("fixture name %q occurs %d times, want 1", name, count)
		}
	}
}

func TestFixtureCatalogMutationsFailClosed(t *testing.T) {
	check := checkFixturePopulation
	t.Run("missing category", func(t *testing.T) {
		categories := append([]fixtureCategory(nil), fixtureCategories[1:]...)
		if result := check(categories, fixtureSites[:], fixtureCatalog); result.Passed {
			t.Fatal("catalog passed after a category was removed")
		}
	})
	t.Run("missing static site", func(t *testing.T) {
		sites := append([]fixtureSite(nil), fixtureSites[:len(fixtureSites)-1]...)
		if result := check(fixtureCategories[:], sites, fixtureCatalog); result.Passed {
			t.Fatal("catalog passed after a Results.check site was removed")
		}
	})
	t.Run("generator drift", func(t *testing.T) {
		sites := append([]fixtureSite(nil), fixtureSites[:]...)
		for index := range sites {
			if len(sites[index].generator.labels) == 0 {
				continue
			}
			sites[index].generator.labels = append(
				[]string(nil), sites[index].generator.labels...)
			sites[index].generator.labels[0] += "-drift"
			break
		}
		if result := check(fixtureCategories[:], sites, fixtureCatalog); result.Passed {
			t.Fatal("catalog passed after a generator label changed")
		}
	})
	t.Run("missing expanded fixture", func(t *testing.T) {
		fixtures := append([]fixtureSpec(nil), fixtureCatalog[:len(fixtureCatalog)-1]...)
		if result := check(fixtureCategories[:], fixtureSites[:], fixtures); result.Passed {
			t.Fatal("catalog passed after an expanded fixture was removed")
		}
	})
	t.Run("exit drift", func(t *testing.T) {
		fixtures := append([]fixtureSpec(nil), fixtureCatalog...)
		for index := range fixtures {
			if fixtures[index].expectedExit < 0 {
				continue
			}
			fixtures[index].expectedExit = (fixtures[index].expectedExit + 1) % 3
			break
		}
		if result := check(fixtureCategories[:], fixtureSites[:], fixtures); result.Passed {
			t.Fatal("catalog passed after an expected exit changed")
		}
	})
	t.Run("message drift", func(t *testing.T) {
		fixtures := append([]fixtureSpec(nil), fixtureCatalog...)
		for index := range fixtures {
			if len(fixtures[index].messages) == 0 {
				continue
			}
			fixtures[index].messages = append(
				[]fixtureMessage(nil), fixtures[index].messages...)
			fixtures[index].messages[0].text += " drift"
			break
		}
		if result := check(fixtureCategories[:], fixtureSites[:], fixtures); result.Passed {
			t.Fatal("catalog passed after an expected message changed")
		}
	})
	t.Run("identity drift", func(t *testing.T) {
		fixtures := append([]fixtureSpec(nil), fixtureCatalog...)
		fixtures[1].site = fixtures[0].site
		fixtures[1].variant = fixtures[0].variant
		if result := check(fixtureCategories[:], fixtureSites[:], fixtures); result.Passed {
			t.Fatal("catalog passed after an expanded identity was duplicated")
		}
	})
	t.Run("native producer boundary drift", func(t *testing.T) {
		boundaries := append(
			[]fixtureProducerBoundary(nil), fixtureProducerBoundaries[:]...)
		boundaries[0].nativeEvidence += " drift"
		if producerBoundaryDigest(boundaries) == fixtureBoundaryDigest {
			t.Fatal("producer digest accepted a changed native boundary")
		}
	})
}

func TestParityCatalogMutationsFailClosed(t *testing.T) {
	tables := append([]parityTable(nil), parityCatalog[:]...)
	tables[0].fixtures = append([]parityFixture(nil), tables[0].fixtures...)
	tables[0].fixtures[0].expectedCode = 0
	if parityDigest(tables) == parityCatalogDigest {
		t.Fatal("dispatcher catalog digest accepted changed expected exit")
	}
	tables[0].fixtures[0] = parityCatalog[0].fixtures[0]
	tables[0].fixtures[0].name += " drift"
	if parityDigest(tables) == parityCatalogDigest {
		t.Fatal("dispatcher catalog digest accepted changed input identity")
	}
	tables[0].fixtures[0] = parityCatalog[0].fixtures[0]
	tables[0].fixtures = tables[0].fixtures[1:]
	if parityDigest(tables) == parityCatalogDigest {
		t.Fatal("dispatcher catalog digest accepted a missing row")
	}
}

func TestActualHookMessageAndExitMutationsFailClosed(t *testing.T) {
	root := copyHookSources(t)
	bashHook := filepath.Join(root, filepath.FromSlash("internal/le/hookruntime/bash.go"))
	body, err := os.ReadFile(bashHook)
	if err != nil {
		t.Fatalf("read copied hook: %v", err)
	}
	original := string(body)
	mutations := map[string]string{
		"message": strings.Replace(original,
			"✘ BLOCKED: go build without -o bin/",
			"✘ BLOCKED: changed message", 1),
		"exit": strings.Replace(original,
			"return &verdict{2, red + bold + \"✘ BLOCKED: go build",
			"return &verdict{1, red + bold + \"✘ BLOCKED: go build", 1),
	}
	for name, mutation := range mutations {
		t.Run(name, func(t *testing.T) {
			if mutation == original {
				t.Fatalf("%s mutation did not match hook source", name)
			}
			if err := os.WriteFile(bashHook, []byte(mutation), 0o600); err != nil {
				t.Fatalf("write mutated hook: %v", err)
			}
			if result := checkHookSources(root); result.Passed || result.Code != 2 {
				t.Fatalf("mutated hook result = %+v, want code-2 failure", result)
			}
			if err := os.WriteFile(bashHook, body, 0o600); err != nil {
				t.Fatalf("restore copied hook: %v", err)
			}
		})
	}
}

func TestRunFailsClosedWhenHookPopulationIsMissing(t *testing.T) {
	report, code := Run(t.TempDir())
	if code != 2 {
		t.Fatalf("Run code = %d, want 2", code)
	}
	found := false
	for _, result := range report.Results {
		if result.Name == "hook-source-drift" && !result.Passed {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing hook source report = %+v", report)
	}
}

func TestActionOwnsUnitAndEveryRuntimeHook(t *testing.T) {
	listing := Actions()
	if listing.Area != area || len(listing.Actions) != 18 {
		t.Fatalf("listing = %+v", listing)
	}
	verbs := make(map[string]bool, len(listing.Actions))
	for _, row := range listing.Actions {
		if row.Writes {
			t.Errorf("hook action unexpectedly declares tree writes: %+v", row)
		}
		verbs[row.Verb] = true
	}
	for _, required := range []string{"unit", "pretool-bash", "pretool-writeedit",
		"posttool-writeedit", "posttool-bash", "session-start", "validate-spec"} {
		if !verbs[required] {
			t.Errorf("missing hook action %q", required)
		}
	}
	if !command.HasLocal(leroot.CommandPath(area)) {
		t.Fatalf("importing hookcheck did not register %q", area)
	}
}

func TestEveryConfiguredHookUsesRegisteredNativeAction(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(hookCheckout(t), ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	registered := make(map[string]bool)
	for _, row := range Actions().Actions {
		registered[row.Verb] = true
	}
	configured := 0
	const prefix = `"command": "$CLAUDE_PROJECT_DIR/le ai hooks `
	for line := range strings.SplitSeq(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, `"command":`) {
			continue
		}
		if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, `",`) {
			t.Errorf("non-native hook command: %s", line)
			continue
		}
		verb := strings.TrimSuffix(strings.TrimPrefix(line, prefix), `",`)
		if !registered[verb] {
			t.Errorf("configured hook action %q is not registered", verb)
		}
		configured++
	}
	if configured != 16 {
		t.Fatalf("configured native hooks = %d, want 16", configured)
	}
}

func copyHookSources(t *testing.T) string {
	t.Helper()
	root, _, _ := hookAuthorityFixture(t)
	return root
}

// TestEveryStoredDigestMatchesTheTreeItRatchets recomputes each stored digest
// from the data it ratchets and compares it with the constant in the package.
// A stale constant is what `./le ai hooks unit` reports, and nothing in the
// unit gate saw it: three commits landed a renamed fixture, a moved producer
// package and a changed hook while every test here stayed green, because each
// one only asserts that a MUTATED input differs from the constant, which a
// stale constant satisfies too.
//
// The failure prints the replacement literal, so re-baselining a ratchet is a
// paste rather than a hand-written hex table.
func TestEveryStoredDigestMatchesTheTreeItRatchets(t *testing.T) {
	sources, err := hookSourceContentDigest(hookCheckout(t))
	if err != nil {
		t.Fatalf("read hook sources: %v", err)
	}
	ratchets := []struct {
		name    string
		current [32]byte
		stored  [32]byte
	}{
		{"parityCatalogDigest", parityDigest(parityCatalog[:]), parityCatalogDigest},
		{"fixtureSiteDigest", fixtureSiteContentDigest(fixtureSites[:]), fixtureSiteDigest},
		{"fixtureCatalogDigest", fixtureDigest(fixtureCatalog), fixtureCatalogDigest},
		{"fixtureCategoryDigest", categoryDigest(fixtureCategories[:]), fixtureCategoryDigest},
		{
			"fixtureBoundaryDigest",
			producerBoundaryDigest(fixtureProducerBoundaries[:]),
			fixtureBoundaryDigest,
		},
		{"hookSourcesDigest", sources, hookSourcesDigest},
	}
	for _, ratchet := range ratchets {
		if ratchet.current == ratchet.stored {
			continue
		}
		t.Errorf("%s is stale: the data it ratchets changed and the constant did not.\n"+
			"Review the change, then replace the constant with:\n%s",
			ratchet.name, goByteLiteral(ratchet.current))
	}
}

// goByteLiteral renders a digest as the body of the Go array literal the
// package stores, eight bytes to a line and indented with two tabs.
func goByteLiteral(digest [32]byte) string {
	var out strings.Builder
	for index, value := range digest {
		if index%8 == 0 {
			out.WriteString("\t\t")
		}
		fmt.Fprintf(&out, "0x%02x,", value)
		if index%8 == 7 {
			out.WriteByte('\n')
			continue
		}
		out.WriteByte(' ')
	}
	return out.String()
}
