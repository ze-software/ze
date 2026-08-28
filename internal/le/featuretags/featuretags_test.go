// Related: featuretags.go -- the generator these tests call as a function

package featuretags

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const manifest = "# a comment\nze_bgp internal/component/bgp\nze_bgp internal/component/bgp/plugins/rib\nze_web internal/component/web\n\nze_lg internal/component/lg\n"

// fixture builds a checkout holding the manifest and the four files derived
// from it, each already carrying a tag list that is one gate short.
func fixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	write(t, root, "feature-gates.txt", manifest)
	write(t, root, ".golangci.yml", "linters:\n  build-tags:\n    - ze_core\n    - ze_bgp\n  enable:\n    - errcheck\n")
	write(t, root, filepath.Join("gokrazy", "ze", "config.json"),
		"{\n  \"Hostname\": \"ze\",\n  \"GoBuildTags\": [\"ze_core\", \"ze_appliance\"],\n  \"Packages\": []\n}\n")
	write(t, root, filepath.Join("docs", "guide", "quickstart.md"),
		"Install it:\n\n    CGO_ENABLED=0 go install -tags 'ze_core ze_distro' example.test/m/cmd/ze@latest\n\nThen run it.\n")
	write(t, root, filepath.Join(".github", "workflows", "codeql.yml"),
		"      - run: CGO_ENABLED=0 go build -tags 'ze_core ze_distro' ./...\n"+
			"      - run: CGO_ENABLED=0 go build -tags 'ze_core ze_appliance' ./...\n"+
			"      - run: CGO_ENABLED=0 go build -tags 'ze_setup' ./cmd/ze\n")

	return root
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create the fixture directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write the fixture %s: %v", rel, err)
	}
}

func read(t *testing.T, root, rel string) string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}

// VALIDATES: the manifest is read once into two orders, each unique: the order
// the file declares, and the alphabet.
// PREVENTS: a duplicate tag reaching a build-tag list, and the two orders being
// derived twice from two readings that can disagree.
func TestManifestReadsIntoTwoOrders(t *testing.T) {
	root := fixture(t)

	declared, sorted, err := readTags(root)
	if err != nil {
		t.Fatalf("readTags: %v", err)
	}
	if strings.Join(declared, " ") != "ze_bgp ze_web ze_lg" {
		t.Errorf("the declared order is %v, want [ze_bgp ze_web ze_lg]", declared)
	}
	if strings.Join(sorted, " ") != "ze_bgp ze_lg ze_web" {
		t.Errorf("the sorted order is %v, want [ze_bgp ze_lg ze_web]", sorted)
	}
}

// VALIDATES: a manifest with no gate in it is an error rather than an empty tag
// list.
// PREVENTS: the fail-open every generator invites. An empty read and a manifest
// that genuinely declares nothing look the same, and writing the empty answer
// would strip every gate tag out of four files at once.
func TestAnEmptyManifestIsAnError(t *testing.T) {
	root := t.TempDir()
	write(t, root, "feature-gates.txt", "# every line is a comment\n\n")

	if _, _, err := readTags(root); err == nil {
		t.Fatal("readTags accepted a manifest declaring no gate")
	}
}

// VALIDATES: check names every file whose tag list disagrees with the manifest,
// relative to the tree, and answers 1.
// PREVENTS: a hand-edited tag list surviving a gate, which is the whole reason
// these four files are generated rather than maintained.
func TestCheckNamesEveryStaleFile(t *testing.T) {
	root := fixture(t)

	report, code := runCheck(root)
	if code != 1 {
		t.Fatalf("check over a stale tree answered %d, want 1", code)
	}
	want := []string{".golangci.yml", "gokrazy/ze/config.json", "docs/guide/quickstart.md", ".github/workflows/codeql.yml"}
	if strings.Join(report.Stale, ",") != strings.Join(want, ",") {
		t.Fatalf("check named %v, want %v", report.Stale, want)
	}
}

// VALIDATES: check writes nothing.
// PREVENTS: a verification gate that repairs the files it judges.
func TestCheckWritesNothing(t *testing.T) {
	root := fixture(t)
	before := read(t, root, ".golangci.yml")

	if _, code := runCheck(root); code != 1 {
		t.Fatal("check answered 0 over a stale tree")
	}
	if after := read(t, root, ".golangci.yml"); after != before {
		t.Errorf("check rewrote .golangci.yml:\n%s", after)
	}
}

// VALIDATES: each rewrite is surgical: only the tag list moves, and the file's
// own shape, indentation and neighbors survive.
// PREVENTS: a generator that reformats a hand-maintained file, which turns a
// one-tag change into a diff nobody can review.
func TestWriteEditsOnlyTheTagList(t *testing.T) {
	root := fixture(t)

	if _, code := runWrite(root); code != 0 {
		t.Fatal("write failed")
	}

	golangci := read(t, root, ".golangci.yml")
	if !strings.Contains(golangci, "linters:\n  build-tags:\n    - ze_core\n    - ze_bgp\n    - ze_web\n    - ze_lg\n  enable:\n    - errcheck\n") {
		t.Errorf(".golangci.yml is not the manifest order with its neighbors kept:\n%s", golangci)
	}

	gokrazy := read(t, root, filepath.Join("gokrazy", "ze", "config.json"))
	if !strings.Contains(gokrazy, "  \"GoBuildTags\": [\"ze_core\", \"ze_appliance\", \"ze_bgp\", \"ze_lg\", \"ze_web\"],\n") {
		t.Errorf("gokrazy config.json is not the sorted order behind the two base tags:\n%s", gokrazy)
	}
	if !strings.Contains(gokrazy, "\"Hostname\": \"ze\"") {
		t.Errorf("gokrazy config.json lost a neighboring key:\n%s", gokrazy)
	}

	quickstart := read(t, root, filepath.Join("docs", "guide", "quickstart.md"))
	if !strings.Contains(quickstart, "CGO_ENABLED=0 go install -tags 'ze_core ze_distro ze_bgp ze_lg ze_web' example.test/m/cmd/ze@latest\n") {
		t.Errorf("quickstart.md did not keep the rest of the command line:\n%s", quickstart)
	}

	codeql := read(t, root, filepath.Join(".github", "workflows", "codeql.yml"))
	if !strings.Contains(codeql, "go build -tags 'ze_core ze_distro ze_bgp ze_lg ze_web' ./...") {
		t.Errorf("codeql.yml did not take the distro combo first:\n%s", codeql)
	}
	if !strings.Contains(codeql, "go build -tags 'ze_core ze_appliance ze_bgp ze_lg ze_web' ./...") {
		t.Errorf("codeql.yml did not take the appliance combo second:\n%s", codeql)
	}
	if !strings.Contains(codeql, "go build -tags 'ze_setup' ./cmd/ze") {
		t.Errorf("codeql.yml rewrote the setup combo, which carries no gate tag:\n%s", codeql)
	}
}

// VALIDATES: write then check agree, and a second write reports nothing.
// PREVENTS: a generator whose own check rejects its output, and one that
// rewrites a file whose bytes already agree.
func TestWriteThenCheckAgreeAndWriteIsIdempotent(t *testing.T) {
	root := fixture(t)

	first, code := runWrite(root)
	if code != 0 {
		t.Fatalf("write answered %d, want 0", code)
	}
	if len(first.Updated) != 4 {
		t.Errorf("write updated %v, want all four", first.Updated)
	}

	if report, code := runCheck(root); code != 0 {
		t.Fatalf("check after write answered %d; stale: %v", code, report.Stale)
	}

	again, code := runWrite(root)
	if code != 0 {
		t.Fatalf("the second write answered %d, want 0", code)
	}
	if len(again.Updated) != 0 {
		t.Errorf("the second write rewrote %v, want nothing", again.Updated)
	}
}

// VALIDATES: a target file whose anchor is gone stops the run rather than being
// left alone.
// PREVENTS: a workflow restructure silently dropping CodeQL coverage of the
// gated surface. The generator cannot find its anchor, and a check that passed
// anyway would say the gated build is still analyzed when it is not.
func TestAMissingAnchorStopsTheRun(t *testing.T) {
	cases := map[string]struct {
		file    string
		content string
	}{
		"golangci without the key":   {".golangci.yml", "linters:\n  enable:\n    - errcheck\n"},
		"golangci with no items":     {".golangci.yml", "linters:\n  build-tags:\n  enable:\n    - errcheck\n"},
		"gokrazy without the key":    {filepath.Join("gokrazy", "ze", "config.json"), "{\n  \"Hostname\": \"ze\"\n}\n"},
		"quickstart without the run": {filepath.Join("docs", "guide", "quickstart.md"), "Install it somehow.\n"},
		"codeql with one combo":      {filepath.Join(".github", "workflows", "codeql.yml"), "      - run: CGO_ENABLED=0 go build -tags 'ze_core ze_distro' ./...\n"},
		"codeql with three combos": {filepath.Join(".github", "workflows", "codeql.yml"),
			"      - run: CGO_ENABLED=0 go build -tags 'ze_core ze_distro' ./...\n" +
				"      - run: CGO_ENABLED=0 go build -tags 'ze_core ze_appliance' ./...\n" +
				"      - run: CGO_ENABLED=0 go build -tags 'ze_core ze_extra' ./...\n"},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			root := fixture(t)
			write(t, root, one.file, one.content)

			if _, err := Check(root); err == nil {
				t.Fatalf("check accepted %s with no anchor in it", one.file)
			}
			if _, code := runCheck(root); code != 1 {
				t.Error("check answered 0 for a file it could not edit")
			}
		})
	}
}

// VALIDATES: both answers render themselves as actionable native verdicts.
// PREVENTS: a report a developer cannot read at a terminal.
func TestBothReportsRenderNativeWording(t *testing.T) {
	current := CheckReport{}
	want := "feature-tag lists are current (.golangci.yml, gokrazy/ze/config.json, docs/guide/quickstart.md, .github/workflows/codeql.yml)\n"
	if got := current.Text(); got != want {
		t.Errorf("a current check renders\n%q\nwant\n%q", got, want)
	}

	stale := CheckReport{Stale: []string{".golangci.yml"}}
	if got := stale.Text(); got != ".golangci.yml is stale; run ./le feature-tags write\n" {
		t.Errorf("a stale check renders %q", got)
	}

	updated := WriteReport{Updated: []string{".golangci.yml", "gokrazy/ze/config.json"}}
	if got := updated.Text(); got != "updated .golangci.yml\nupdated gokrazy/ze/config.json\n" {
		t.Errorf("a write renders %q", got)
	}

	quiet := WriteReport{}
	wantQuiet := "feature-tag lists already current (.golangci.yml, gokrazy/ze/config.json, docs/guide/quickstart.md, .github/workflows/codeql.yml)\n"
	if got := quiet.Text(); got != wantQuiet {
		t.Errorf("a write that changed nothing renders\n%q\nwant\n%q", got, wantQuiet)
	}
}
