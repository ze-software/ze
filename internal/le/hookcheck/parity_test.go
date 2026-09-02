// Related: parity.go -- every typed dispatcher row is judged in Go
// Related: fixtures.go -- typed fixtures and actual-hook drift contracts
//
// VALIDATES: the native hook selftest owns all 208 dispatcher rows, all 456
// Results.check sites, and their 607 concrete runtime fixture identities from
// the typed native catalog.
// PREVENTS: missing categories, fixtures, messages, exits, or hook source drift.
package hookcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
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
	if len(golden) != 208 {
		t.Fatalf("dispatcher fixtures = %d, want 208", len(golden))
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
	if exitCounts != [3]int{73, 9, 126} {
		t.Fatalf("dispatcher exit populations = %v, want [73 9 126]", exitCounts)
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
			command: `bash -c "./le doc check verify > tmp/out.log"`,
			code:    2,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := bashCode(test.command); got != test.code {
				t.Errorf("bashCode(%q) = %d, want %d", test.command, got, test.code)
			}
		})
	}
}

func TestEveryBehavioralCategoryDiscriminates(t *testing.T) {
	for _, category := range fixtureCategories {
		t.Run(category.name, func(t *testing.T) {
			if !categoryVerdict(category.name, category.allow) {
				t.Errorf("allow probe was refused: %q", category.allow)
			}
			if categoryVerdict(category.name, category.refuse) {
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
		"rfc-test-guard":     {},
		"weakened-hatch":     {},
		"rfc-changed-ledger": {},
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
	sourcePaths := make(map[string]struct{}, len(hookSourcePaths))
	for _, path := range hookSourcePaths {
		sourcePaths[path] = struct{}{}
	}
	for _, boundary := range fixtureProducerBoundaries {
		if boundary.actionOwner != "internal/le/testweakened/actions.go" ||
			boundary.actionEvidence != `Verb:   "proposed"` ||
			boundary.nativeOwner != "internal/le/testweakened/proposed.go" ||
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
	if !reflect.DeepEqual(exitCounts, map[int]int{-1: 314, 0: 186, 1: 13, 2: 94}) {
		t.Fatalf("fixture exit populations = %v", exitCounts)
	}
	wantMessages := map[string]int{"contains": 102, "not-contains": 14, "equals": 6, "suffix": 3}
	if !reflect.DeepEqual(messageCounts, wantMessages) {
		t.Fatalf("fixture message populations = %v, want %v", messageCounts, wantMessages)
	}
	if len(names) != fixtureUniqueNamesExpected {
		t.Fatalf("unique fixture names = %d, want %d", len(names), fixtureUniqueNamesExpected)
	}
	for name, count := range names {
		want := 1
		if name == "review-model-verb-implementation" {
			want = 2
		}
		if count != want {
			t.Fatalf("fixture name %q occurs %d times, want %d", name, count, want)
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
	if listing.Area != area || len(listing.Actions) != 21 {
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
		"posttool-writeedit", "pretool-agent-skill", "session-start", "validate-spec"} {
		if !verbs[required] {
			t.Errorf("missing hook action %q", required)
		}
	}
	if !registry.HasLocal(leroot.CommandPath(area)) {
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
	const prefix = `"command": "$CLAUDE_PROJECT_DIR/le hook-check `
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
	if configured != 19 {
		t.Fatalf("configured native hooks = %d, want 19", configured)
	}
}

func copyHookSources(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	checkout := hookCheckout(t)
	for _, relative := range hookSourcePaths {
		body, err := os.ReadFile(filepath.Join(checkout, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		target := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatalf("create %s parent: %v", relative, err)
		}
		if err := os.WriteFile(target, body, 0o600); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
	return root
}

// TestEveryStoredDigestMatchesTheTreeItRatchets recomputes each stored digest
// from the data it ratchets and compares it with the constant in the package.
// A stale constant is what `./le hook-check unit` reports, and nothing in the
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
