// VALIDATES: the one reader of a rule set's parts answers the exact 1-based
// line range of each part, refuses a heading it cannot place, and reads a
// file whatever its line endings.
// PREVENTS: the subagent start hook and the payload report acting on a
// guessed range. Both read their parts here, so a wrong cut sends an agent
// the wrong rules, or names lines that hold something else, with every caller
// still green.

package ruleset

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fixtureTree writes a fixture checkout and answers its root. Paths are
// repository-relative.
func fixtureTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
	return root
}

// wantSpan fails the test unless part spans first to last and holds text.
func wantSpan(t *testing.T, what string, part Part, first, last int64, text string) {
	t.Helper()

	if part.First != first || part.Last != last {
		t.Errorf("%s spans lines %d-%d, want %d-%d", what, part.First, part.Last, first, last)
	}
	if part.Text != text {
		t.Errorf("%s holds %q, want %q", what, part.Text, text)
	}
}

func TestTheAgentRuleSetReaderIgnoresHeadingsInACodeFence(t *testing.T) {
	// Goal: the one reader the start hook and the payload report share answers
	// the exact line range of each part, at any heading level, and a heading
	// inside a code fence neither repeats a part heading nor ends a part.
	// Method: each part holds a fenced heading that, read as a heading, would
	// repeat the main-session heading or end the hard bans early.
	decisions := "# Owner Meta Decisions\n" + // line 1
		"\n" + // 2
		"## " + DecisionsAgentPartHeading + "\n" + // 3
		"```\n" + // 4
		"## " + DecisionsMainPartHeading + "\n" + // 5
		"```\n" + // 6
		"\n" + // 7
		"## " + DecisionsMainPartHeading + "\n" + // 8
		"- MAIN-ONLY\n" // 9
	instructions := "# " + HardBansHeading + "\n" + // line 1
		"```\n# a shell comment\n```\n" + // 2-4
		"- Never push.\n" + // 5
		"# Finding the Rule\n" // 6
	tree := fixtureTree(t, map[string]string{DecisionsFile: decisions, InstructionsFile: instructions})

	part, err := AgentDecisions(tree)
	if err != nil {
		t.Fatalf("AgentDecisions: %v", err)
	}
	wantSpan(t, "the every-agent part", part, 3, 7,
		"## "+DecisionsAgentPartHeading+"\n```\n## "+DecisionsMainPartHeading+"\n```\n\n")

	bans, err := HardBans(tree)
	if err != nil {
		t.Fatalf("HardBans: %v", err)
	}
	wantSpan(t, "the hard bans", bans, 1, 5, "# "+HardBansHeading+"\n```\n# a shell comment\n```\n- Never push.\n")
}

func TestTheReaderKnowsTheFenceRuleTheLintKnows(t *testing.T) {
	// Goal: the reader follows the one fence rule the rules lint uses: a tilde
	// fence hides a heading as a backtick fence does, and a block closes only
	// on its own marker character. Method: a tilde fence holding the
	// main-session heading, and a hard bans section whose tilde fence holds a
	// backtick line and a heading; a reader that knew only backticks would
	// count the heading twice, or end the bans inside the block.
	decisions := "## " + DecisionsAgentPartHeading + "\n" + // line 1
		"~~~\n" + // 2
		"## " + DecisionsMainPartHeading + "\n" + // 3
		"~~~\n" + // 4
		"## " + DecisionsMainPartHeading + "\n" // 5
	instructions := "# " + HardBansHeading + "\n" + // line 1
		"~~~~\n```\n# a shell comment\n~~~~\n" + // 2-5
		"- Never push.\n" + // 6
		"# Finding the Rule\n" // 7
	tree := fixtureTree(t, map[string]string{DecisionsFile: decisions, InstructionsFile: instructions})

	part, err := AgentDecisions(tree)
	if err != nil {
		t.Fatalf("AgentDecisions: %v", err)
	}
	wantSpan(t, "the every-agent part", part, 1, 4, "## "+DecisionsAgentPartHeading+"\n~~~\n## "+DecisionsMainPartHeading+"\n~~~\n")
	bans, err := HardBans(tree)
	if err != nil {
		t.Fatalf("HardBans: %v", err)
	}
	wantSpan(t, "the hard bans", bans, 1, 6, "# "+HardBansHeading+"\n~~~~\n```\n# a shell comment\n~~~~\n- Never push.\n")
}

func TestTheEveryAgentPartSpansItsExactLines(t *testing.T) {
	// Goal: the every-agent part runs from its heading to the line before the
	// main-session heading, counted from 1, with no off-by-one at either end.
	// Method: the heading on line 1, where a 0-based count answers 0, and the
	// two headings adjacent, where the part is its heading line alone.
	agent := "# " + DecisionsAgentPartHeading + "\n"
	session := "# " + DecisionsMainPartHeading + "\n"
	for _, tc := range []struct {
		name, decisions string
		first, last     int64
		text            string
	}{
		{"heading on line 1", agent + "- every agent\n" + session + "- main only\n", 1, 2, agent + "- every agent\n"},
		{"adjacent headings", "# Owner Meta Decisions\n" + agent + session + "- main only\n", 2, 2, agent},
		{"adjacent headings on line 1", agent + session, 1, 1, agent},
	} {
		part, err := AgentDecisions(fixtureTree(t, map[string]string{DecisionsFile: tc.decisions}))
		if err != nil {
			t.Errorf("%s: AgentDecisions: %v", tc.name, err)
			continue
		}
		wantSpan(t, tc.name, part, tc.first, tc.last, tc.text)
	}
}

func TestAPartHeadingTheReaderCannotPlaceIsAnError(t *testing.T) {
	// Goal: a heading that is absent, repeated or out of order is answered as
	// an error naming what is wrong, never as a range a caller could act on.
	// Method: each fault on its own; the error must carry the reader's own
	// words for that fault, and the part no text and no line.
	agent := "# " + DecisionsAgentPartHeading + "\n- every agent\n"
	session := "# " + DecisionsMainPartHeading + "\n- main only\n"
	bans := "# " + HardBansHeading + "\n- Never push.\n"
	for _, tc := range []struct {
		name  string
		read  Reader
		files map[string]string
		want  string
	}{
		{"no every-agent heading", AgentDecisions, map[string]string{DecisionsFile: session},
			DecisionsFile + ` holds 0 headings "` + DecisionsAgentPartHeading + `", want exactly one`},
		{"no main-session heading", AgentDecisions, map[string]string{DecisionsFile: agent},
			DecisionsFile + ` holds 0 headings "` + DecisionsMainPartHeading + `", want exactly one`},
		{"a repeated every-agent heading", AgentDecisions, map[string]string{DecisionsFile: agent + agent + session},
			DecisionsFile + ` holds 2 headings "` + DecisionsAgentPartHeading + `", want exactly one`},
		{"the headings in the wrong order", AgentDecisions, map[string]string{DecisionsFile: session + agent},
			DecisionsFile + ` puts the heading "` + DecisionsMainPartHeading + `" before "` + DecisionsAgentPartHeading + `"`},
		{"no hard bans heading", HardBans, map[string]string{InstructionsFile: "# Ze\n- intro\n"},
			InstructionsFile + ` holds 0 headings "` + HardBansHeading + `", want exactly one`},
		{"a repeated hard bans heading", HardBans, map[string]string{InstructionsFile: bans + bans},
			InstructionsFile + ` holds 2 headings "` + HardBansHeading + `", want exactly one`},
	} {
		part, err := tc.read(fixtureTree(t, tc.files))
		if err == nil {
			t.Errorf("%s: answered lines %d-%d instead of an error", tc.name, part.First, part.Last)
			continue
		}
		if err.Error() != tc.want {
			t.Errorf("%s: the error is %q, want %q", tc.name, err, tc.want)
		}
		if part.Text != "" || part.First != 0 || part.Last != 0 {
			t.Errorf("%s: the error came with a range: lines %d-%d, %q", tc.name, part.First, part.Last, part.Text)
		}
	}
}

func TestTheHardBansEndAtTheNextHeadingOfTheSameOrAHigherLevel(t *testing.T) {
	// Goal: the hard bans section keeps its deeper subheadings and ends at the
	// first heading of its own level or a higher one. Method: a level-2
	// section with a level-3 subheading inside, ended once by a sibling and
	// once by a level-1 heading; a reader that ends at any heading stops at the
	// subheading, and one that ends only at its own level runs past the
	// level-1 heading.
	section := "## " + HardBansHeading + "\n" + // line 2
		"### A ban\n" + // 3
		"- Never push.\n" // 4
	for _, tc := range []struct{ name, next string }{
		{"a sibling heading", "## Owner Meta Decisions\n"},
		{"a higher heading", "# Finding the Rule\n"},
	} {
		instructions := "# Ze\n" + section + tc.next + "- not a ban\n"
		bans, err := HardBans(fixtureTree(t, map[string]string{InstructionsFile: instructions}))
		if err != nil {
			t.Errorf("%s: HardBans: %v", tc.name, err)
			continue
		}
		wantSpan(t, "the hard bans ended by "+tc.name, bans, 2, 4, section)
	}
}

func TestTheReaderReadsCRLFAndAnUnterminatedLastLine(t *testing.T) {
	// Goal: a file saved with CRLF line endings, or without a final newline,
	// is cut at the same lines as its LF twin, and the part keeps its bytes.
	// Method: each reader over both shapes; a reader that matches a heading
	// with its carriage return finds none, and one that drops an unterminated
	// line answers one line short.
	crlf := fixtureTree(t, map[string]string{
		DecisionsFile:    "# Owner\r\n## " + DecisionsAgentPartHeading + "\r\n- every agent\r\n## " + DecisionsMainPartHeading + "\r\n",
		InstructionsFile: "# " + HardBansHeading + "\r\n- Never push.\r\n# Finding the Rule\r\n",
	})
	part, err := AgentDecisions(crlf)
	if err != nil {
		t.Fatalf("CRLF AgentDecisions: %v", err)
	}
	wantSpan(t, "the CRLF every-agent part", part, 2, 3, "## "+DecisionsAgentPartHeading+"\r\n- every agent\r\n")
	bans, err := HardBans(crlf)
	if err != nil {
		t.Fatalf("CRLF HardBans: %v", err)
	}
	wantSpan(t, "the CRLF hard bans", bans, 1, 2, "# "+HardBansHeading+"\r\n- Never push.\r\n")

	unterminated := fixtureTree(t, map[string]string{
		DecisionsFile:    "## " + DecisionsAgentPartHeading + "\n- every agent\n## " + DecisionsMainPartHeading,
		InstructionsFile: "# " + HardBansHeading + "\n- Never push.",
		TriggersFile:     "triggers\nlast",
	})
	part, err = AgentDecisions(unterminated)
	if err != nil {
		t.Fatalf("unterminated AgentDecisions: %v", err)
	}
	wantSpan(t, "the unterminated every-agent part", part, 1, 2, "## "+DecisionsAgentPartHeading+"\n- every agent\n")
	bans, err = HardBans(unterminated)
	if err != nil {
		t.Fatalf("unterminated HardBans: %v", err)
	}
	wantSpan(t, "the unterminated hard bans", bans, 1, 2, "# "+HardBansHeading+"\n- Never push.")
	whole, err := WholeFile(TriggersFile)(unterminated)
	if err != nil {
		t.Fatalf("unterminated WholeFile: %v", err)
	}
	wantSpan(t, "the unterminated whole file", whole, 1, 2, "triggers\nlast")
}

func TestReadAnswersAnAbsentPartAsMissing(t *testing.T) {
	// Goal: Read answers an absent file or heading as Missing with its reason,
	// so a caller that joins the texts cannot take an absent part for a
	// shorter one, and keeps reading the parts that are there. Method: one
	// present file, one absent file, one file without its heading.
	tree := fixtureTree(t, map[string]string{
		CoreFile:      "core\n",
		DecisionsFile: "# Owner Meta Decisions\n- main only\n",
	})
	parts, err := Read(tree, []Reader{WholeFile(CoreFile), WholeFile(TriggersFile), AgentDecisions})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(parts) != 3 {
		t.Fatalf("Read answered %d parts, want 3", len(parts))
	}
	if parts[0].Missing || parts[0].Text != "core\n" {
		t.Errorf("the present file: %+v", parts[0])
	}
	if !parts[1].Missing || parts[1].Reason != TriggersFile || parts[1].Text != "" {
		t.Errorf("the absent file is not Missing with its path: %+v", parts[1])
	}
	if !parts[2].Missing || !strings.Contains(parts[2].Reason, `0 headings "`+DecisionsAgentPartHeading+`"`) {
		t.Errorf("the absent heading is not Missing with its reason: %+v", parts[2])
	}
	if _, err := WholeFile(TriggersFile)(tree); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an absent file is not answered as fs.ErrNotExist: %v", err)
	}
}

// ownerApprovedSituationalSections names the sections of the checkout's
// decisions file an agent reads only before the work each one names, in file
// order. The owner approved exactly these four; the reader finds them by
// heading level and never by this list, which only the test holds.
var ownerApprovedSituationalSections = []string{
	"ExaBGP compatibility",
	"Test systems and labs",
	"Developer tooling",
	"Website design",
}

func TestTheSituationalSectionsAreTheFourTheOwnerApproved(t *testing.T) {
	// Goal: AC-15, on the checkout's own ai/META-DECISIONS.md. Exactly the
	// four approved sections are situational, each states a situation, each
	// range holds its own heading and lies past the every-agent range, and no
	// approved heading is left in the every-agent text.
	// Method: SituationalDecisions and AgentDecisions over the repository root.
	root := filepath.Clean("../../../../..")
	sections, err := SituationalDecisions(root)
	if err != nil {
		t.Fatalf("SituationalDecisions: %v", err)
	}
	agent, err := AgentDecisions(root)
	if err != nil {
		t.Fatalf("AgentDecisions: %v", err)
	}
	headings := make([]string, 0, len(sections))
	for _, section := range sections {
		headings = append(headings, section.Heading)
		if section.Situation == "" {
			t.Errorf("section %q names no situation", section.Heading)
		}
		if !strings.HasPrefix(section.Text, "## "+section.Heading+"\n") {
			t.Errorf("section %q at lines %d-%d does not open with its heading", section.Heading, section.First, section.Last)
		}
		if section.First <= agent.Last {
			t.Errorf("section %q starts at line %d, inside the every-agent range %d-%d", section.Heading, section.First, agent.First, agent.Last)
		}
		if strings.Contains(agent.Text, "\n## "+section.Heading+"\n") {
			t.Errorf("the every-agent part still holds a section %q", section.Heading)
		}
	}
	if !slices.Equal(headings, ownerApprovedSituationalSections) {
		t.Errorf("the situational sections are %q, want %q", headings, ownerApprovedSituationalSections)
	}
}

func TestTheSituationalReaderAnswersEachSectionAndItsSituation(t *testing.T) {
	// Goal: each section of the third part is answered with its exact range,
	// up to the next section or the end of the part, its deeper headings kept,
	// and its situation joined from a wrapped first paragraph, which ends at a
	// bullet. A heading in a code fence neither opens nor ends a section.
	// Method: a file whose third part holds a wrapped situation, a level-3
	// subheading, a fenced level-2 heading, and a situation followed at once by
	// a bullet.
	decisions := "# " + DecisionsAgentPartHeading + "\n" + // line 1
		"- every agent\n" + // 2
		"# " + DecisionsMainPartHeading + "\n" + // 3
		"- main only\n" + // 4
		"# " + DecisionsSituationalPartHeading + "\n" + // 5
		"Opening.\n" + // 6
		"## Lab work\n" + // 7
		"\n" + // 8
		SituationPrefix + "running a lab\n" + // 9
		"  or a VM.\n" + // 10
		"\n" + // 11
		"- Use Ze's kernel.\n" + // 12
		"### Detail\n" + // 13
		"```\n## Not a section\n```\n" + // 14-16
		"## Website design\n" + // 17
		SituationPrefix + "editing the website.\n" + // 18
		"- Render into gh-pages.\n" // 19
	sections, err := SituationalDecisions(fixtureTree(t, map[string]string{DecisionsFile: decisions}))
	if err != nil {
		t.Fatalf("SituationalDecisions: %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("answered %d sections, want 2: %+v", len(sections), sections)
	}
	wantSpan(t, "the lab section", sections[0].Part, 7, 16,
		"## Lab work\n\n"+SituationPrefix+"running a lab\n  or a VM.\n\n- Use Ze's kernel.\n### Detail\n```\n## Not a section\n```\n")
	wantSpan(t, "the website section", sections[1].Part, 17, 19,
		"## Website design\n"+SituationPrefix+"editing the website.\n- Render into gh-pages.\n")
	for index, want := range []struct{ heading, situation string }{
		{"Lab work", "running a lab or a VM."},
		{"Website design", "editing the website."},
	} {
		if sections[index].Heading != want.heading || sections[index].Situation != want.situation || sections[index].File != DecisionsFile {
			t.Errorf("section %d is %q in %s, situation %q; want %q, situation %q",
				index+1, sections[index].Heading, sections[index].File, sections[index].Situation, want.heading, want.situation)
		}
	}
}

func TestASituationWrappedOntoTheNextLineIsRead(t *testing.T) {
	// Goal: a situation paragraph whose marker stands alone on its line, the
	// work wrapped onto the next, names that work, as one that starts on the
	// marker's line does. Method: the marker line alone, then the work.
	decisions := "# " + DecisionsAgentPartHeading + "\n- every agent\n# " + DecisionsMainPartHeading + "\n- main only\n# " +
		DecisionsSituationalPartHeading + "\n## Lab work\n" + strings.TrimSpace(SituationPrefix) + "\n  running a lab.\n\n- Use Ze's kernel.\n"
	sections, err := SituationalDecisions(fixtureTree(t, map[string]string{DecisionsFile: decisions}))
	if err != nil {
		t.Fatalf("SituationalDecisions: %v", err)
	}
	if len(sections) != 1 || sections[0].Situation != "running a lab." {
		t.Errorf("answered %+v, want one section whose situation is %q", sections, "running a lab.")
	}
}

func TestASituationalPartTheReaderCannotPlaceIsAnError(t *testing.T) {
	// Goal: a third part that cannot tell an agent what to read is an error
	// naming what is wrong, never an empty list that reads as "nothing to
	// read", and Read's absence test classifies it as missing.
	// Method: each fault on its own: the heading absent, repeated, before the
	// main-session heading, a part with no section, a bullet or a deeper
	// heading before the first section, a later heading of the part's level
	// that ends the part before the end of the file, a section that opens with
	// a bullet, with another paragraph, or with the prefix and no situation.
	agent := "# " + DecisionsAgentPartHeading + "\n- every agent\n"
	session := "# " + DecisionsMainPartHeading + "\n- main only\n"
	situational := "# " + DecisionsSituationalPartHeading + "\nOpening.\n"
	section := "## Lab work\n" + SituationPrefix + "running a lab.\n- Use Ze's kernel.\n"
	noSituation := `section "Lab work" names no work: its first paragraph must be "` + SituationPrefix + `<the work it binds>", so no agent can tell when to read it`
	stray := func(line string) string {
		return DecisionsFile + ` holds "` + line + `" under the heading "` + DecisionsSituationalPartHeading + `" before its first section, so no agent is told to read it`
	}
	for _, tc := range []struct{ name, decisions, want string }{
		{"no third part", agent + session,
			DecisionsFile + ` holds 0 headings "` + DecisionsSituationalPartHeading + `", want exactly one`},
		{"a repeated third part", agent + session + situational + section + situational + section,
			DecisionsFile + ` holds 2 headings "` + DecisionsSituationalPartHeading + `", want exactly one`},
		{"the third part before the main-session part", agent + situational + section + session,
			DecisionsFile + ` puts the heading "` + DecisionsSituationalPartHeading + `" before "` + DecisionsMainPartHeading + `"`},
		{"no section", agent + session + situational,
			DecisionsFile + ` holds no section under the heading "` + DecisionsSituationalPartHeading + `"`},
		{"a bullet before the first section", agent + session + situational + "- An unread rule.\n" + section,
			stray("- An unread rule.")},
		{"a deeper heading before the first section", agent + session + situational + "### Stray\n- An unread rule.\n" + section,
			stray("### Stray")},
		{"a heading of the part's level after it", agent + session + situational + section + "# Later\n- An unread rule.\n",
			DecisionsFile + ` ends the part "` + DecisionsSituationalPartHeading + `" at the heading "Later", so the rules after it sit in no section an agent is told to read`},
		{"a section that opens with a bullet", agent + session + situational + "## Lab work\n\n- Use Ze's kernel.\n",
			DecisionsFile + " " + noSituation},
		{"a section that opens with another paragraph", agent + session + situational + "## Lab work\nAbout labs.\n" + SituationPrefix + "running a lab.\n",
			DecisionsFile + " " + noSituation},
		{"a prefix that names nothing", agent + session + situational + "## Lab work\n" + strings.TrimSpace(SituationPrefix) + "\n\n- Use Ze's kernel.\n",
			DecisionsFile + " " + noSituation},
	} {
		sections, err := SituationalDecisions(fixtureTree(t, map[string]string{DecisionsFile: tc.decisions}))
		if err == nil {
			t.Errorf("%s: answered %d sections instead of an error", tc.name, len(sections))
			continue
		}
		if sections != nil {
			t.Errorf("%s: the error came with sections: %+v", tc.name, sections)
		}
		if err.Error() != tc.want {
			t.Errorf("%s: the error is %q, want %q", tc.name, err, tc.want)
		}
		if reason, absent := MissingReason(err, DecisionsFile); !absent || reason != tc.want {
			t.Errorf("%s: MissingReason = %q, %v; want the error as an absence", tc.name, reason, absent)
		}
	}
	if _, err := SituationalDecisions(fixtureTree(t, map[string]string{CoreFile: "core\n"})); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an absent decisions file is not answered as fs.ErrNotExist: %v", err)
	}
}
