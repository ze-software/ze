package commit

import (
	"slices"
	"strings"
	"testing"
)

// prepareNewsClosure prepares commit A (a.go) and appends commit B, the
// removal of spec x, to its script, the way /ze-close does, with a clean
// review covering the spec's whole change set so the review gate admits it.
// noNews is the no-news reason commit B gives, or "" for none.
func prepareNewsClosure(t *testing.T, root, noNews string) (Prepared, error) {
	t.Helper()
	specWorkHistory(t, root)
	writeCommitFixture(t, root, "internal/a.go", "package internal\n")
	if _, err := Create(root, &Options{Subject: "x: commit A", Files: []string{"internal/a.go"},
		NoTest: "the fixture carries no test", SpecWork: "x"}); err != nil {
		t.Fatalf("Create refused commit A: %v", err)
	}
	writeReviewArtifact(t, root, "x", "internal/a.go", "internal/b.go")
	return Create(root, &Options{Subject: "plan: close x", Remove: []string{specWorkSpecPath},
		Append: true, SpecWork: "x", NoNews: noNews})
}

// TestCreateRefusesClosureWithoutNewsFragment drives the gate through Create,
// the entry point `./le commit create ... remove <spec>` reaches.
//
// VALIDATES: a closure carrying no fragment and no no-news reason is refused,
// and the refusal names the fragment path to write, each of the six headings,
// and the no-news keyword with its reason (AC-2, wiring).
// PREVENTS: a closure landing with no statement of whether operators see a
// change, and a refusal that leaves the author guessing what fixes it.
func TestCreateRefusesClosureWithoutNewsFragment(t *testing.T) {
	root := newCommitRepository(t)
	claimAs(t, root, "news-fragment-owed", "", "")
	_, err := prepareNewsClosure(t, root, "")
	if err == nil {
		t.Fatal("a closure with no news fragment and no no-news reason was prepared")
	}
	wants := []string{"website/changes/fragments/x.md", `no-news "<reason>"`}
	for _, heading := range newsHeadings {
		wants = append(wants, "## "+heading.Name)
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}
}

// TestCreateNoNewsWritesTrailer drives the no-news admission through Create.
//
// VALIDATES: a closure carrying no-news "<reason>" and no fragment is
// prepared, and its message ends with `No-news: <reason>` directly before
// `Spec-work: x`; the trailer is not listed among the person acts the push
// owes (AC-3, wiring).
// PREVENTS: a no-news reason dropped in silence, written after the
// attribution, or recorded as a debt that holds a push.
func TestCreateNoNewsWritesTrailer(t *testing.T) {
	root := newCommitRepository(t)
	claimAs(t, root, "news-fragment-none", "", "")
	const reason = "tooling: no operator-visible change"
	prepared, err := prepareNewsClosure(t, root, reason)
	if err != nil {
		t.Fatalf("a closure with a no-news reason was refused: %v", err)
	}
	message := preparedMessage(t, root, &prepared)
	lines := strings.Split(strings.TrimRight(message, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("the message holds fewer than two lines:\n%s", message)
	}
	tail := lines[len(lines)-2:]
	if want := []string{trailerNoNews + reason, trailerSpecWork + "x"}; !slices.Equal(tail, want) {
		t.Fatalf("the message ends with %q, want %q:\n%s", tail, want, message)
	}
	for _, trailer := range prepared.Trailers {
		if strings.HasPrefix(trailer, trailerNoNews) {
			t.Fatalf("No-news: is listed as a person act the push owes: %q", prepared.Trailers)
		}
	}
}

// newsVocabulary is the topic vocabulary the fragment fixtures commit at
// website/data/topics.json: one family, so a second one is visibly new.
const newsVocabulary = `{"tags":{"BGP":"routing"}}`

// newsSection is one heading of a fixture fragment and the text under it.
type newsSection struct {
	name string
	body string
}

// newsValidSections answers the six headings of a valid fragment, in order:
// operator-visible text under Before and After, "none" under the rest.
func newsValidSections() []newsSection {
	return []newsSection{
		{name: "Before", body: "A peer reset withdrew every route the peer had sent."},
		{name: "After", body: "A peer reset keeps the routes for the restart time."},
		{name: "Corrects", body: "none"},
		{name: "Exposure", body: "none"},
		{name: "Upstream", body: "none"},
		{name: "Asked by", body: "none"},
	}
}

// newsFragmentText writes a fragment from its front matter lines and its
// sections.
func newsFragmentText(front string, sections []newsSection) string {
	var text strings.Builder
	text.WriteString("---\n" + front + "---\n")
	for _, section := range sections {
		text.WriteString("\n## " + section.name + "\n\n" + section.body + "\n")
	}
	return text.String()
}

// newsFragmentRepository is a Ze checkout fixture whose HEAD carries the
// topic vocabulary, under the identity session.
func newsFragmentRepository(t *testing.T, session string) string {
	t.Helper()
	root := newCommitRepository(t)
	claimAs(t, root, session, "", "")
	commitFixture(t, root, "website: the topic vocabulary", map[string]string{newsTopicsPath: newsVocabulary}, nil)
	return root
}

// createCarrying writes files into root and prepares one ordinary commit
// carrying them, the way `./le commit create ... file <path>` does.
func createCarrying(t *testing.T, root string, files map[string]string) (Prepared, error) {
	t.Helper()
	paths := make([]string, 0, len(files))
	for path, content := range files {
		writeCommitFixture(t, root, path, content)
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return Create(root, &Options{Subject: "website: a news fragment", Files: paths})
}

// TestNewsFragmentUnknownTopicRefused drives the gate through Create.
//
// VALIDATES: a carried fragment whose topic is not a tag family of
// website/data/topics.json is refused, and the refusal names the fragment,
// the topic and the vocabulary file; the same fragment with a listed topic is
// prepared (AC-4).
// PREVENTS: a fragment filed under a topic the update report cannot chip.
func TestNewsFragmentUnknownTopicRefused(t *testing.T) {
	const fragment = newsFragmentDirectory + "x.md"
	root := newsFragmentRepository(t, "news-topic-unknown")
	_, err := createCarrying(t, root, map[string]string{
		fragment: newsFragmentText("spec: x\ntopic: Telepathy\n", newsValidSections()),
	})
	if err == nil {
		t.Fatal("a fragment whose topic the vocabulary does not list was prepared")
	}
	for _, want := range []string{fragment, `"Telepathy"`, newsTopicsPath} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}

	listed := newsFragmentRepository(t, "news-topic-listed")
	if _, err := createCarrying(t, listed, map[string]string{
		fragment: newsFragmentText("spec: x\ntopic: BGP\n", newsValidSections()),
	}); err != nil {
		t.Fatalf("a fragment with a listed topic was refused: %v", err)
	}
}

// TestNewsFragmentTopicAddedInSameCommitAccepted drives the gate through
// Create.
//
// VALIDATES: a fragment whose topic the commit itself adds to
// website/data/topics.json is prepared, because the gate reads the
// vocabulary from the tree the commit produces; the same fragment without
// the vocabulary edit is refused, so the acceptance comes from the carried
// vocabulary and not from a gate that judges nothing (AC-4 survivor).
// PREVENTS: a gate reading HEAD's or the working tree's vocabulary, which
// would refuse a new family until a second commit, or admit one the commit
// does not carry.
func TestNewsFragmentTopicAddedInSameCommitAccepted(t *testing.T) {
	const fragment = newsFragmentDirectory + "x.md"
	text := newsFragmentText("spec: x\ntopic: OSPF\n", newsValidSections())

	root := newsFragmentRepository(t, "news-topic-added")
	if _, err := createCarrying(t, root, map[string]string{
		fragment:       text,
		newsTopicsPath: `{"tags":{"BGP":"routing","OSPF":"routing"}}`,
	}); err != nil {
		t.Fatalf("a fragment whose topic the same commit adds was refused: %v", err)
	}

	// The working tree carries the new family, the commit does not.
	without := newsFragmentRepository(t, "news-topic-not-carried")
	writeCommitFixture(t, without, newsTopicsPath, `{"tags":{"BGP":"routing","OSPF":"routing"}}`)
	_, err := createCarrying(t, without, map[string]string{fragment: text})
	if err == nil {
		t.Fatal("a fragment whose topic only the uncommitted working tree lists was prepared")
	}
	if !strings.Contains(err.Error(), `"OSPF"`) {
		t.Fatalf("the refusal does not name the topic:\n%v", err)
	}
}

// TestNewsFragmentFoundByJudged judges the found-by value through
// parseNewsFragment, then once through Create.
//
// VALIDATES: each declared found-by value is accepted and answered by
// the parsed foundBy, an unknown one is refused with every accepted value listed, and a
// fragment with no found-by is accepted (AC-4a).
// PREVENTS: a value outside the closed list, which the event report would
// group under a name nobody declared.
func TestNewsFragmentFoundByJudged(t *testing.T) {
	vocabulary := map[string]string{"BGP": "routing"}
	for _, value := range newsFoundBy {
		text := newsFragmentText("journal: bgp-reset 2026-11-09\ntopic: BGP\nfound-by: "+value+"\n", newsValidSections())
		fragment, err := parseNewsFragment("website/changes/fragments/bgp-reset.md", text, vocabulary)
		if err != nil {
			t.Errorf("found-by %s was refused: %v", value, err)
			continue
		}
		if fragment.foundBy != value {
			t.Errorf("foundBy = %q, want %q", fragment.foundBy, value)
		}
	}

	absent, err := parseNewsFragment("website/changes/fragments/x.md",
		newsFragmentText("spec: x\ntopic: BGP\n", newsValidSections()), vocabulary)
	if err != nil {
		t.Fatalf("a fragment with no found-by was refused: %v", err)
	}
	if absent.foundBy != "" {
		t.Fatalf("foundBy = %q for a fragment that names none", absent.foundBy)
	}

	const fragment = newsFragmentDirectory + "bgp-reset.md"
	root := newsFragmentRepository(t, "news-found-by")
	_, err = createCarrying(t, root, map[string]string{
		fragment: newsFragmentText("journal: bgp-reset 2026-11-09\ntopic: BGP\nfound-by: hunch\n", newsValidSections()),
	})
	if err == nil {
		t.Fatal("a fragment whose found-by is not in the closed list was prepared")
	}
	wants := append([]string{fragment, `"hunch"`}, newsFoundBy...)
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}
}

// TestNewsFragmentHeadingsJudged judges the six headings through
// parseNewsFragment, then a missing one once through Create.
//
// VALIDATES: each heading missing is refused naming it; each heading empty is
// refused naming it; "none" under Before or After is refused naming it;
// "none" under Corrects, Exposure, Upstream and Asked by is accepted; an
// unknown heading and headings out of order are refused (AC-5).
// PREVENTS: a fragment with no operator-visible change in it, and a refusal
// that does not say which heading to fix.
func TestNewsFragmentHeadingsJudged(t *testing.T) {
	vocabulary := map[string]string{"BGP": "routing"}
	const front = "spec: x\ntopic: BGP\n"
	type headingCase struct {
		name     string
		sections []newsSection
		refused  string
	}
	var cases []headingCase
	for index, heading := range newsHeadings {
		missing := slices.Delete(newsValidSections(), index, index+1)
		cases = append(cases, headingCase{name: heading.Name + " missing", sections: missing,
			refused: "## " + heading.Name + " is missing"})
		empty := newsValidSections()
		empty[index].body = ""
		cases = append(cases, headingCase{name: heading.Name + " empty", sections: empty,
			refused: "## " + heading.Name + " is empty"})
		none := newsValidSections()
		none[index].body = "None."
		refused := ""
		if !heading.NoneAllowed {
			refused = "## " + heading.Name + " reads only none"
		}
		cases = append(cases, headingCase{name: heading.Name + " none", sections: none, refused: refused})
	}
	unknown := append(newsValidSections(), newsSection{name: "Lessons", body: "the agent learned a thing"})
	cases = append(cases, headingCase{name: "unknown heading", sections: unknown, refused: "## Lessons is not one of"})
	swapped := newsValidSections()
	swapped[0], swapped[1] = swapped[1], swapped[0]
	cases = append(cases, headingCase{name: "out of order", sections: swapped, refused: "out of order"})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fragment, err := parseNewsFragment("website/changes/fragments/x.md", newsFragmentText(front, tc.sections), vocabulary)
			if tc.refused == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				if _, known := fragment.Section("Before"); !known {
					t.Fatal("an accepted fragment answers no Before section")
				}
				return
			}
			if err == nil {
				t.Fatalf("accepted; want a refusal naming %q", tc.refused)
			}
			if !strings.Contains(err.Error(), tc.refused) {
				t.Fatalf("the refusal does not name %q:\n%v", tc.refused, err)
			}
		})
	}

	const path = newsFragmentDirectory + "x.md"
	root := newsFragmentRepository(t, "news-headings")
	_, err := createCarrying(t, root, map[string]string{
		path: newsFragmentText(front, slices.Delete(newsValidSections(), 1, 2)),
	})
	if err == nil {
		t.Fatal("a fragment with no ## After was prepared")
	}
	for _, want := range []string{path, "## After is missing"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}
}

// TestNewsFragmentValidatedOnNonClosure drives commits that close no spec
// through Create.
//
// VALIDATES: a fix outside a spec that carries a malformed fragment is
// refused, naming the fragment and the heading; a valid journal fragment is
// prepared with no No-news: trailer; the README beside the fragments is never
// judged as one; a file in the directory that is not <name>.md is refused
// (AC-7).
// PREVENTS: a gate that judges fragments only at a closure, so a fix's
// fragment reaches the weekly update unread.
func TestNewsFragmentValidatedOnNonClosure(t *testing.T) {
	const fragment = newsFragmentDirectory + "bgp-reset.md"
	const front = "journal: bgp-reset 2026-11-09\ntopic: BGP\nfound-by: interop\n"

	malformed := newsFragmentRepository(t, "news-non-closure-malformed")
	sections := newsValidSections()
	sections[0].body = "none"
	_, err := createCarrying(t, malformed, map[string]string{fragment: newsFragmentText(front, sections)})
	if err == nil {
		t.Fatal("a non-closure commit carrying a malformed fragment was prepared")
	}
	for _, want := range []string{fragment, "## Before reads only none"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}

	valid := newsFragmentRepository(t, "news-non-closure-valid")
	prepared, err := createCarrying(t, valid, map[string]string{fragment: newsFragmentText(front, newsValidSections())})
	if err != nil {
		t.Fatalf("a non-closure commit carrying a valid fragment was refused: %v", err)
	}
	if message := preparedMessage(t, valid, &prepared); strings.Contains(message, trailerNoNews) {
		t.Fatalf("a non-closure commit carries a No-news: trailer:\n%s", message)
	}

	readme := newsFragmentRepository(t, "news-non-closure-readme")
	if _, err := createCarrying(t, readme, map[string]string{newsFragmentReadme: "# News fragments\n\nThe format.\n"}); err != nil {
		t.Fatalf("the fragment README was judged as a fragment: %v", err)
	}

	misnamed := newsFragmentRepository(t, "news-non-closure-misnamed")
	const nested = newsFragmentDirectory + "old/bgp-reset.txt"
	_, err = createCarrying(t, misnamed, map[string]string{nested: newsFragmentText(front, newsValidSections())})
	if err == nil {
		t.Fatal("a file in the fragment directory that is not <name>.md was prepared")
	}
	if !strings.Contains(err.Error(), nested) {
		t.Fatalf("the refusal does not name %s:\n%v", nested, err)
	}
}

// TestNewsFragmentSkippedOutsideCheckout drives Create in a git tree that is
// not the Ze checkout, the published site or the wiki.
//
// VALIDATES: a malformed file at a fragment path is prepared there, with no
// No-news: trailer, while the same file in a Ze checkout is refused, so the
// skip comes from lepath.IsCheckout and not from a gate that judges nothing
// (AC-9).
// PREVENTS: the site and wiki commits refused for a vocabulary and a format
// their trees do not hold.
func TestNewsFragmentSkippedOutsideCheckout(t *testing.T) {
	const fragment = newsFragmentDirectory + "x.md"
	const text = "no front matter and no headings\n"

	site := newSiteRepository(t)
	claimAs(t, site, "news-outside-checkout", "", "")
	prepared, err := createCarrying(t, site, map[string]string{fragment: text})
	if err != nil {
		t.Fatalf("a commit outside the Ze checkout was refused: %v", err)
	}
	if message := preparedMessage(t, site, &prepared); strings.Contains(message, trailerNoNews) {
		t.Fatalf("a commit outside the Ze checkout carries a No-news: trailer:\n%s", message)
	}

	checkout := newsFragmentRepository(t, "news-inside-checkout")
	if _, err := createCarrying(t, checkout, map[string]string{fragment: text}); err == nil {
		t.Fatal("the same malformed fragment was prepared in a Ze checkout")
	}
}

// TestNewsFragmentUnreadableCouldNotRun calls the gate over a tree git cannot
// resolve.
//
// VALIDATES: a fragment the gate cannot read answers couldNotRun, exit 3,
// naming the fragment, never a pass and never a refusal of the commit (R-6).
// PREVENTS: an unreadable tree read as a fragment that is absent, which would
// admit a commit nobody judged.
func TestNewsFragmentUnreadableCouldNotRun(t *testing.T) {
	const fragment = newsFragmentDirectory + "x.md"
	root := newsFragmentRepository(t, "news-unreadable")
	_, err := newsFragmentRefusal(root, &Options{}, []string{fragment}, nil, nil,
		prospectiveContent{tree: "0123456789abcdef0123456789abcdef01234567"})
	if err == nil {
		t.Fatal("a fragment in a tree git cannot read was admitted")
	}
	if code := exitCodeOf(err); code != 3 {
		t.Fatalf("exit code %d, want 3 (could not run): %v", code, err)
	}
	if !strings.Contains(err.Error(), fragment) {
		t.Fatalf("the error does not name %s: %v", fragment, err)
	}
}

// newsClosureFragment is a valid fragment for spec x, the stem the closure
// fixtures remove.
func newsClosureFragment() string {
	return newsFragmentText("spec: x\ntopic: BGP\n", newsValidSections())
}

// newsClosureFragmentPath is where spec x's fragment is written.
const newsClosureFragmentPath = newsFragmentDirectory + "x.md"

// prepareFragmentClosure prepares commit A (a.go, and every file in carryA)
// and appends commit B, the removal of spec x, to its script, the way
// /ze-close does. The review pins a.go, b.go (the spec's earlier commit) and
// pinned, so the review gate admits the closure and the news-fragment gate is
// what decides. noNews is the no-news reason commit B gives, or "".
func prepareFragmentClosure(t *testing.T, root string, carryA map[string]string, pinned []string, noNews string) (Prepared, error) {
	t.Helper()
	files := []string{"internal/a.go"}
	writeCommitFixture(t, root, "internal/a.go", "package internal\n")
	for path, content := range carryA {
		writeCommitFixture(t, root, path, content)
		files = append(files, path)
	}
	slices.Sort(files)
	if _, err := Create(root, &Options{Subject: "x: commit A", Files: files,
		NoTest: "the fixture carries no test", SpecWork: "x"}); err != nil {
		t.Fatalf("Create refused commit A: %v", err)
	}
	writeReviewArtifact(t, root, "x", append([]string{"internal/a.go", "internal/b.go"}, pinned...)...)
	return Create(root, &Options{Subject: "plan: close x", Remove: []string{specWorkSpecPath},
		Append: true, SpecWork: "x", NoNews: noNews})
}

// assertNoNoNewsLine fails when a prepared message carries a No-news: line.
func assertNoNoNewsLine(t *testing.T, root string, prepared *Prepared) {
	t.Helper()
	message := preparedMessage(t, root, prepared)
	for line := range strings.SplitSeq(message, "\n") {
		if strings.HasPrefix(line, trailerNoNews) {
			t.Fatalf("a closure that carries a fragment writes %q:\n%s", line, message)
		}
	}
}

// TestNewsFragmentClosureAccepted drives a closure whose own commit carries
// the spec's fragment through Create.
//
// VALIDATES: a closure that is not appended to a script, and whose commit
// carries a valid website/changes/fragments/x.md naming spec x, is prepared
// with no No-news: trailer (AC-1, the fragment written by this commit).
// PREVENTS: the gate counting only earlier work, so the one-commit closure
// owes a fragment it already carries.
func TestNewsFragmentClosureAccepted(t *testing.T) {
	root := newsFragmentRepository(t, "news-closure-accepted")
	specWorkHistory(t, root)
	writeCommitFixture(t, root, newsClosureFragmentPath, newsClosureFragment())
	writeReviewArtifact(t, root, "x", "internal/b.go", newsClosureFragmentPath)
	prepared, err := Create(root, &Options{Subject: "plan: close x", Files: []string{newsClosureFragmentPath},
		Remove: []string{specWorkSpecPath}, SpecWork: "x"})
	if err != nil {
		t.Fatalf("a closure carrying its own fragment was refused: %v", err)
	}
	assertNoNoNewsLine(t, root, &prepared)
}

// TestNewsFragmentClosureNamesItsSpecRefused drives a closure whose fragment
// sits at the spec's path but names another spec.
//
// VALIDATES: the fragment's spec front matter must name the stem it closes;
// a mismatch is refused, naming the path and the key to set (AC-1).
// PREVENTS: a fragment copied from another spec satisfying this closure
// because of its file name alone.
func TestNewsFragmentClosureNamesItsSpecRefused(t *testing.T) {
	root := newsFragmentRepository(t, "news-closure-wrong-spec")
	specWorkHistory(t, root)
	wrong := newsFragmentText("spec: y\ntopic: BGP\n", newsValidSections())
	_, err := prepareFragmentClosure(t, root, map[string]string{newsClosureFragmentPath: wrong},
		[]string{newsClosureFragmentPath}, "")
	if err == nil {
		t.Fatal("a closure whose fragment names spec y was prepared for spec x")
	}
	for _, want := range []string{newsClosureFragmentPath, "spec: x"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}

// TestNewsFragmentClosureFindsFragmentFromEarlierSpecCommit drives a closure
// whose fragment landed in an earlier commit attributed to the spec.
//
// VALIDATES: a fragment written by a landed commit carrying `Spec-work: x`
// satisfies commit B, which neither carries nor appends to it (AC-1, A-1:
// judging at commit B over the spec's population is enough).
// PREVENTS: a closure refused because its fragment landed before commit A.
func TestNewsFragmentClosureFindsFragmentFromEarlierSpecCommit(t *testing.T) {
	root := newsFragmentRepository(t, "news-closure-earlier")
	specWorkHistory(t, root)
	commitFixture(t, root, "x: the news fragment\n\n"+trailerSpecWork+"x",
		map[string]string{newsClosureFragmentPath: newsClosureFragment()}, nil)
	prepared, err := prepareFragmentClosure(t, root, nil, []string{newsClosureFragmentPath}, "")
	if err != nil {
		t.Fatalf("a closure whose fragment an earlier spec commit wrote was refused: %v", err)
	}
	assertNoNoNewsLine(t, root, &prepared)
}

// TestNewsFragmentCarriedEarlierInSameScript drives the /ze-close shape:
// commit A carries the fragment and commit B is appended to its script.
//
// VALIDATES: a fragment carried by an earlier block of the script commit B
// appends to, not yet landed and so in no Spec-work: commit, satisfies the
// closure (AC-1, R-5).
// PREVENTS: the two-commit closure refused because commit A has not landed
// when commit B is prepared.
func TestNewsFragmentCarriedEarlierInSameScript(t *testing.T) {
	root := newsFragmentRepository(t, "news-closure-script")
	specWorkHistory(t, root)
	prepared, err := prepareFragmentClosure(t, root, map[string]string{newsClosureFragmentPath: newsClosureFragment()},
		[]string{newsClosureFragmentPath}, "")
	if err != nil {
		t.Fatalf("a closure whose commit A carries the fragment was refused: %v", err)
	}
	assertNoNoNewsLine(t, root, &prepared)
}

// TestNewsFragmentStaleSameNameRefused drives a closure that finds a
// same-named fragment no work of the spec touched.
//
// VALIDATES: website/changes/fragments/x.md present at HEAD, landed by a
// commit with no `Spec-work: x`, untouched by commit A and commit B, refuses
// the closure as stale and names the path (AC-6, R-4). The same closure with
// no-news is prepared: the stale text is another spec's, not this closure's
// news, so it does not contradict the statement.
// PREVENTS: an earlier spec's fragment published as this spec's news.
func TestNewsFragmentStaleSameNameRefused(t *testing.T) {
	root := newsFragmentRepository(t, "news-closure-stale")
	specWorkHistory(t, root)
	commitFixture(t, root, "an earlier spec x: the news fragment",
		map[string]string{newsClosureFragmentPath: newsClosureFragment()}, nil)
	_, err := prepareFragmentClosure(t, root, nil, nil, "")
	if err == nil {
		t.Fatal("a closure was satisfied by a fragment no work of the spec touched")
	}
	for _, want := range []string{newsClosureFragmentPath, "stale"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}

	root = newsFragmentRepository(t, "news-closure-stale-none")
	specWorkHistory(t, root)
	commitFixture(t, root, "an earlier spec x: the news fragment",
		map[string]string{newsClosureFragmentPath: newsClosureFragment()}, nil)
	if _, err := prepareFragmentClosure(t, root, nil, nil, "tooling: no operator-visible change"); err != nil {
		t.Fatalf("a no-news closure was refused over another spec's stale fragment: %v", err)
	}
}

// TestNewsFragmentAndNoNewsTogetherRefused drives a closure that carries the
// spec's fragment and also says no-news.
//
// VALIDATES: the two statements contradict each other and the closure is
// refused, naming the fragment and the keyword (AC-3, its survivor).
// PREVENTS: a closure that publishes news while recording that there is none.
func TestNewsFragmentAndNoNewsTogetherRefused(t *testing.T) {
	root := newsFragmentRepository(t, "news-closure-both")
	specWorkHistory(t, root)
	_, err := prepareFragmentClosure(t, root, map[string]string{newsClosureFragmentPath: newsClosureFragment()},
		[]string{newsClosureFragmentPath}, "tooling: no operator-visible change")
	if err == nil {
		t.Fatal("a closure carrying both a fragment and no-news was prepared")
	}
	for _, want := range []string{newsClosureFragmentPath, keywordNoNews} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}

// TestNoNewsOnNonClosureRefused drives no-news on a commit that removes no
// spec.
//
// VALIDATES: no-news given to an ordinary commit is refused, naming the
// keyword to drop, because it states nothing there.
// PREVENTS: a no-news reason dropped in silence, which the author would read
// as recorded.
func TestNoNewsOnNonClosureRefused(t *testing.T) {
	root := newsFragmentRepository(t, "news-non-closure")
	writeCommitFixture(t, root, "internal/a.go", "package internal\n")
	_, err := Create(root, &Options{Subject: "internal: a change", Files: []string{"internal/a.go"},
		NoTest: "the fixture carries no test", NoNews: "tooling: no operator-visible change"})
	if err == nil {
		t.Fatal("no-news on a commit that closes no spec was prepared")
	}
	if !strings.Contains(err.Error(), "drop "+keywordNoNews) {
		t.Fatalf("the refusal does not name the keyword to drop: %v", err)
	}
}
