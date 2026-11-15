package aisync

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// skillRationales is where a skill's reasons live once they leave its body:
// ai/rationale/<skill>.md, named after the skill it serves.
const skillRationales = "ai/rationale"

// rationaleCitation matches a citation of a rationale section in a skill body,
// after whitespace is collapsed: `ai/rationale/<page>.md`, "<heading>".
var rationaleCitation = regexp.MustCompile("`(" + skillRationales + `/[^` + "`" + `]+\.md)` + "`" + `, "([^"]+)"`)

// rationaleProblems answers what is wrong between the skills under root and
// the rationale pages they point at, one line per problem. Both directions are
// judged: every section of a skill's own page is named by that skill, and every
// rationale citation in any skill names a heading its page holds. pages counts
// the skills that have a page of their own.
func rationaleProblems(t *testing.T, root string) (problems []string, pages int) {
	t.Helper()
	skills, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(skillSources), "*"+markdown))
	if err != nil {
		t.Fatal(err)
	}
	headings := map[string]map[string]bool{}
	headingsOf := func(pagePath string) (map[string]bool, bool) {
		if known, seen := headings[pagePath]; seen {
			return known, known != nil
		}
		page, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(pagePath)))
		if errors.Is(err, fs.ErrNotExist) {
			headings[pagePath] = nil
			return nil, false
		}
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for line := range strings.SplitSeq(string(page), "\n") {
			if heading, isSection := strings.CutPrefix(line, "## "); isSection {
				found[strings.Join(strings.Fields(heading), " ")] = true
			}
		}
		headings[pagePath] = found
		return found, true
	}
	for _, skill := range skills {
		body, err := os.ReadFile(skill)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(filepath.Base(skill), markdown)
		skillPath := skillSources + "/" + name + markdown
		// Collapse every run of whitespace, so a citation wrapped across two
		// lines of the body still matches.
		flat := strings.Join(strings.Fields(string(body)), " ")
		for _, citation := range rationaleCitation.FindAllStringSubmatch(flat, -1) {
			known, exists := headingsOf(citation[1])
			if !exists {
				problems = append(problems, skillPath+" cites "+citation[1]+", which does not exist")
				continue
			}
			if !known[citation[2]] {
				problems = append(problems, skillPath+" cites "+citation[1]+`, "`+citation[2]+`", which is no heading of that page`)
			}
		}
		pagePath := skillRationales + "/" + name + markdown
		own, exists := headingsOf(pagePath)
		if !exists {
			continue
		}
		pages++
		for heading := range own {
			if !strings.Contains(flat, `"`+heading+`"`) {
				problems = append(problems, pagePath+`: section "`+heading+`" is named by no step of `+skillPath)
			}
		}
	}
	return problems, pages
}

// VALIDATES: in the real checkout, every section of a skill's rationale page,
// ai/rationale/<skill>.md, is named by that skill's body, and every rationale
// citation in a skill body names a heading its page holds, a citation of
// another skill's page or of a rule's rationale page included. At least one
// skill must have a page, so the test cannot pass empty.
// PREVENTS: a sentence moved out of a skill body to a page no step points at,
// which an agent never reads, and a pointer to a section that does not exist.
func TestEverySkillRationaleSectionIsNamedByItsSkill(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	problems, pages := rationaleProblems(t, repository)
	for _, problem := range problems {
		t.Error(problem)
	}
	if pages == 0 {
		t.Fatalf("no skill under %s has a page under %s, so nothing was checked", skillSources, skillRationales)
	}
}

// VALIDATES: a skill citation of a heading its page does not hold is refused,
// in both forms: its own page, and another page.
// PREVENTS: the reverse check passing on the real tree only because nothing
// in it is broken, while a citation of a renamed heading would pass too.
func TestASkillCitationOfAMissingRationaleHeadingIsRefused(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ai/rationale/ze-demo.md", "# Rationale\n\n## Kept\n\nwhy\n")
	write(t, root, "ai/rationale/other.md", "# Other\n\n## There\n\nwhy\n")
	write(t, root, "ai/skills/ze-demo.md", "---\nname: ze-demo\ndescription: d\n---\n"+
		"Step one (why: `ai/rationale/ze-demo.md`, \"Kept\").\n"+ // <!-- doc-links: ignore (citation of a kept heading in the temporary checkout rationale fixture) -->
		"Step two (why: `ai/rationale/ze-demo.md`,\n\"Gone\").\n"+ // <!-- doc-links: ignore (citation of an intentionally missing heading in the temporary checkout rationale fixture) -->
		"Step three (why: `ai/rationale/other.md`, \"Absent\").\n"+ // <!-- doc-links: ignore (citation of an intentionally missing heading on the temporary checkout's other rationale page) -->
		"Step four (why: `ai/rationale/other.md`, \"There\").\n") // <!-- doc-links: ignore (citation of a kept heading on the temporary checkout's other rationale page) -->
	problems, pages := rationaleProblems(t, root)
	if pages != 1 {
		t.Fatalf("pages = %d, want 1", pages)
	}
	want := []string{
		`ai/skills/ze-demo.md cites ai/rationale/ze-demo.md, "Gone", which is no heading of that page`, // <!-- doc-links: ignore (exact diagnostic expected for the temporary checkout's missing own-page heading) -->
		`ai/skills/ze-demo.md cites ai/rationale/other.md, "Absent", which is no heading of that page`, // <!-- doc-links: ignore (exact diagnostic expected for the temporary checkout's missing other-page heading) -->
	}
	if strings.Join(problems, "\n") != strings.Join(want, "\n") {
		t.Fatalf("problems:\n%s\nwant:\n%s", strings.Join(problems, "\n"), strings.Join(want, "\n"))
	}
}
