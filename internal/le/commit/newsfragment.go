// Design: docs/contributing/committing.md -- the gates ./le commit create runs
// Related: prepare.go -- Create, which calls this gate after specMechanismsRefusal
// Related: review.go -- closedSpecStem and specWorkPopulation, which this gate reads
// Related: snapshot.go -- prospectiveContent, the tree a fragment is read from
// Related: ../site/topics/topics.go -- Parse, the one reader of the topic vocabulary

package commit

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/commit/hunks"
	"github.com/ze-software/ze/internal/le/site/topics"
)

// newsFragmentDirectory holds one news fragment per change: the operator facts
// the weekly update and the release notes start from. The site does not
// publish it (website/changes/fragments/README.md).
const newsFragmentDirectory = "website/changes/fragments/"

// newsFragmentReadme is the format page beside the fragments. It is not a
// fragment, so the gate never judges it as one.
const newsFragmentReadme = newsFragmentDirectory + "README.md"

// newsTopicsPath is the vocabulary a fragment's topic is judged against. The
// gate reads it from the tree the commit produces, so a family the same
// commit adds is already known.
const newsTopicsPath = "website/" + topics.File

// newsFrontMatterFence opens and closes a fragment's front matter.
const newsFrontMatterFence = "---"

// newsHeadingPrefix opens a heading line of a fragment's body.
const newsHeadingPrefix = "## "

// The front matter keys of a fragment. A fragment names the spec it closes, or
// the journal row a fix outside a spec answers; its topic is a tag family of
// website/data/topics.json; feature and found-by are optional.
const (
	newsKeySpec    = "spec"
	newsKeyJournal = "journal"
	newsKeyTopic   = "topic"
	newsKeyFeature = "feature"
	newsKeyFoundBy = "found-by"
)

// newsNone is the body a heading may carry when the fact is truly absent.
// newsHeadings says which headings refuse it.
const newsNone = "none"

// newsHeading is one fixed section of a fragment.
type newsHeading struct {
	// Name is the heading text, written `## <Name>`.
	Name string
	// NoneAllowed reports whether the section may read only "none". Before
	// and After never may: they are the operator-visible change itself.
	NoneAllowed bool
}

// newsHeadings is the one declaration of a fragment's six headings, in the
// order a fragment writes them. website/changes/fragments/README.md cites it.
var newsHeadings = []newsHeading{
	{Name: "Before", NoneAllowed: false},
	{Name: "After", NoneAllowed: false},
	{Name: "Corrects", NoneAllowed: true},
	{Name: "Exposure", NoneAllowed: true},
	{Name: "Upstream", NoneAllowed: true},
	{Name: "Asked by", NoneAllowed: true},
}

// newsFoundBy is the closed list a fix fragment's found-by value is drawn
// from: what surfaced the defect it describes. spec-weekly-event-report lists
// fix fragments by this value.
var newsFoundBy = []string{"rfc-check", "interop", "test-judge", "review", "operator-report", "upstream"}

// newsFragmentPath answers the one path a spec's fragment is written at. The
// path is fixed by the stem, never by a front matter value.
func newsFragmentPath(stem string) string {
	return newsFragmentDirectory + stem + ".md"
}

// newsFragmentOwed is the refusal a closure meets when it carries neither a
// fragment nor a no-news reason. It names the file to write, its headings and
// the keyword that admits the closure instead.
func newsFragmentOwed(stem string) error {
	names := make([]string, 0, len(newsHeadings))
	for _, heading := range newsHeadings {
		names = append(names, "## "+heading.Name)
	}
	return errors.New("closing spec " + stem + " owes a news fragment, because the weekly update and the release notes " +
		"are written from the fragments closures leave: write " + newsFragmentPath(stem) +
		" with front matter (" + newsKeySpec + ": " + stem + ", " + newsKeyTopic + ": <tag family from website/data/topics.json>, optional " +
		newsKeyFeature + " and " + newsKeyFoundBy + ") and the six headings " + strings.Join(names, ", ") +
		" (see " + newsFragmentReadme + "), carry it in commit A, and prepare this closure again; " +
		"or, when operators see no change, add " + keywordNoNews + ` "<reason>" to this closure`)
}

// newsFragmentRefusal is the news-fragment gate Create runs over a commit in
// the Ze checkout, after specMechanismsRefusal. It answers the No-news:
// trailer the commit carries, or "" when it carries none, and an error that
// refuses the commit; couldNotRun when a fragment or git history cannot be
// read. It reads fragments from content, the prospective tree, never from the
// working tree.
//
// Every fragment the commit adds or edits is judged by newsFragmentsCarried,
// whether or not the commit closes a spec: a fix outside a spec carries one
// too, and its shape is owed the same way.
//
// A closure (closedSpecStem) then owes exactly one of two statements: the
// spec's fragment at newsFragmentPath, naming the spec and written by its
// work (newsFragmentWrittenBySpec), or the no-news reason, which becomes the
// No-news: trailer. Both together contradict each other, a same-named
// fragment no work of the spec touched is stale, and no-news on a commit
// that closes nothing states nothing; each is refused.
func newsFragmentRefusal(root string, options *Options, paths, removed []string, selection hunks.Selection,
	content prospectiveContent,
) (string, error) {
	if err := newsFragmentsCarried(root, paths, content.tree); err != nil {
		return "", err
	}
	// The closure is read as the review gate reads it: from HEAD and the
	// selected working-tree blobs, so a relocation or a rename is no closure
	// here either (closedSpecStem).
	stem, err := closedSpecStem(paths, removed, workingTreeSides(root, content.before, selection))
	if err != nil {
		return "", err
	}
	noNews := strings.TrimSpace(options.NoNews) != ""
	if stem == "" {
		if noNews {
			return "", errors.New(keywordNoNews + " was given to a commit that closes no spec, so it states nothing " +
				"(no spec is removed, or the removal is a relocation or a rename): drop " + keywordNoNews +
				" and prepare the commit again")
		}
		return "", nil
	}
	path := newsFragmentPath(stem)
	text, present, problem := committedText(root, content.tree, path)
	if problem != "" {
		return "", couldNotRun(errors.New("read news fragment " + path + ": " + problem))
	}
	if present {
		owned, err := newsFragmentWrittenBySpec(root, stem, path, paths, options, content)
		if err != nil {
			return "", err
		}
		// A same-named fragment no work of this spec touched belongs to an
		// earlier spec that used the stem. It is not this closure's news, so
		// no-news leaves it alone, and without no-news the closure is refused
		// rather than satisfied by another spec's text.
		if owned {
			if noNews {
				return "", errors.New("closing spec " + stem + " carries both the news fragment " + path + " and " +
					keywordNoNews + ", which say opposite things about what operators see: drop " + keywordNoNews +
					" to publish the fragment, or remove the fragment in commit A to keep " + keywordNoNews)
			}
			return "", newsFragmentClosureProblem(root, stem, path, text, content.tree)
		}
		if !noNews {
			return "", errors.New("closing spec " + stem + " finds " + path + ", but no commit attributed to the spec " +
				"(Spec-work: " + stem + "), no earlier block of this script and not this commit touched it, so it is " +
				"a stale fragment from an earlier spec of the same name: rewrite " + path + " for this spec in commit A, " +
				"or add " + keywordNoNews + ` "<reason>" to this closure when operators see no change`)
		}
	}
	if !noNews {
		return "", newsFragmentOwed(stem)
	}
	return admissionTrailer(trailerNoNews, "", options.NoNews)
}

// newsFragmentWrittenBySpec answers whether this spec's work wrote the
// fragment at path: a commit attributed to the spec through Spec-work:, an
// earlier block of the script this commit appends to (commit A, not yet
// landed), or this commit itself. It reads the population the closure review
// covers (checkVerificationGates), so the two gates never judge different
// work.
func newsFragmentWrittenBySpec(root, stem, path string, paths []string, options *Options,
	content prospectiveContent,
) (bool, error) {
	if slices.Contains(paths, path) {
		return true, nil
	}
	population, err := specWorkPopulation(root, stem, content.head)
	if err != nil {
		return false, couldNotRun(err)
	}
	if slices.Contains(population.Paths, path) {
		return true, nil
	}
	if !options.Append {
		return false, nil
	}
	earlier, err := scriptPaths(content.scriptPath, content.script)
	if err != nil {
		return false, couldNotRun(err)
	}
	return slices.Contains(earlier, path), nil
}

// newsFragmentClosureProblem judges the closing spec's own fragment: it must
// parse, and its spec key must name the spec it closes, because the path
// alone does not say which spec wrote the text.
func newsFragmentClosureProblem(root, stem, path, text, tree string) error {
	vocabulary, err := newsTopicVocabulary(root, tree)
	if err != nil {
		return err
	}
	fragment, err := parseNewsFragment(path, text, vocabulary)
	if err != nil {
		return err
	}
	if fragment.Spec() != stem {
		return errors.New(path + " is the news fragment of spec " + stem + ", but its front matter names " +
			newsFragmentOwner(fragment) + ": set " + newsKeySpec + ": " + stem + " in its front matter")
	}
	return nil
}

// newsFragmentOwner names what a fragment's front matter says it answers.
func newsFragmentOwner(fragment newsFragment) string {
	if fragment.Spec() != "" {
		return newsKeySpec + " " + fragment.Spec()
	}
	return newsKeyJournal + " " + fragment.Journal()
}

// newsFragmentsCarried judges every news fragment among paths, read from
// tree, with parseNewsFragment. Every malformed fragment is named in one
// refusal, so the author fixes them all in one pass.
func newsFragmentsCarried(root string, paths []string, tree string) error {
	var vocabulary map[string]string
	var problems []string
	for _, path := range paths {
		if !isNewsFragmentPath(path) {
			continue
		}
		text, present, problem := committedText(root, tree, path)
		if problem != "" {
			return couldNotRun(errors.New("judge news fragment " + path + ": " + problem))
		}
		// A path the commit deletes leaves no fragment to judge, whatever its
		// name, so deleting a misnamed file is never refused.
		if !present {
			continue
		}
		if problem := newsFragmentNameProblem(path); problem != "" {
			problems = append(problems, problem)
			continue
		}
		// The vocabulary is read once, and only by a commit that carries a
		// fragment: every other commit owes nothing to topics.json.
		if vocabulary == nil {
			loaded, err := newsTopicVocabulary(root, tree)
			if err != nil {
				return err
			}
			vocabulary = loaded
		}
		if _, err := parseNewsFragment(path, text, vocabulary); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) != 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

// newsTopicVocabulary reads the topic vocabulary from tree, the tree the
// commit produces.
func newsTopicVocabulary(root, tree string) (map[string]string, error) {
	text, present, problem := committedText(root, tree, newsTopicsPath)
	if problem != "" {
		return nil, couldNotRun(errors.New("read " + newsTopicsPath + " to judge news fragment topics: " + problem))
	}
	if !present {
		return nil, errors.New(newsTopicsPath + " is absent from the tree this commit produces, and a news fragment's " +
			"topic is judged against it: restore it, or carry it in this commit")
	}
	vocabulary, err := topics.Parse(newsTopicsPath, []byte(text))
	if err != nil {
		return nil, fmt.Errorf("%w; news fragment topics are judged against it, so fix it in this commit", err)
	}
	return vocabulary, nil
}

// isNewsFragmentPath reports whether path, slash-separated from the checkout
// root, lies in the news fragment directory and is not its README. Every
// commit that carries such a path has it judged as a fragment.
func isNewsFragmentPath(path string) bool {
	if !strings.HasPrefix(path, newsFragmentDirectory) {
		return false
	}
	return path != newsFragmentReadme
}

// newsFragmentNameProblem answers why a path in the fragment directory cannot
// be a fragment, or "" when it can: a fragment is one Markdown file directly
// in the directory.
func newsFragmentNameProblem(path string) string {
	name := strings.TrimPrefix(path, newsFragmentDirectory)
	stem, markdown := strings.CutSuffix(name, ".md")
	if markdown && stem != "" && !strings.Contains(stem, "/") {
		return ""
	}
	return "news fragment " + path + " is refused: a fragment is one <name>.md file directly in " +
		newsFragmentDirectory + ", so move it there with that name, or leave it out of this commit"
}

// newsFragment is one news fragment whose front matter and headings passed
// every check parseNewsFragment makes. Only parseNewsFragment builds one, so a
// value in hand is a valid fragment; the zero value is no fragment. The weekly
// event report reads it from inside this package (weekly_source.go); a reader
// outside it (the release notes) exports what it needs when it lands.
type newsFragment struct {
	path     string
	spec     string
	journal  string
	topic    string
	feature  string
	foundBy  string
	sections map[string]string
}

// Spec answers the spec stem the fragment closes, or "" for a fix outside a
// spec, which names a journal row instead.
func (fragment newsFragment) Spec() string { return fragment.spec }

// Journal answers the journal row a fix outside a spec closes, or "" for a
// spec closure, which names a spec instead.
func (fragment newsFragment) Journal() string { return fragment.journal }

// Section answers the trimmed text under one of the six headings, and false
// for a name that is not one of them.
func (fragment newsFragment) Section(name string) (string, bool) {
	text, known := fragment.sections[name]
	return text, known
}

// parseNewsFragment reads one fragment and judges its shape: the front matter
// names a spec or a journal row, a topic that vocabulary lists, an optional
// feature and an optional found-by from the closed list; the body carries the
// six headings of newsHeadings, in order, each with text, and Before and
// After never read only "none". vocabulary maps each tag family to its
// category, as topics.Parse answers it.
//
// The refusal names the file, every problem found, and the fix for each.
func parseNewsFragment(path, text string, vocabulary map[string]string) (newsFragment, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	metadata, body, problems := newsFrontMatter(text)
	if len(problems) == 0 {
		problems = newsMetadataProblems(metadata, vocabulary)
	}
	var sections map[string]string
	// A front matter that never closes leaves no body to find, and judging
	// none would refuse six headings the file may well carry.
	if metadata != nil || body != "" {
		var sectionProblems []string
		sections, sectionProblems = newsSections(body)
		problems = append(problems, sectionProblems...)
	}
	if len(problems) != 0 {
		return newsFragment{}, errors.New("news fragment " + path + " is refused: " + strings.Join(problems, "; ") +
			" (the format: " + newsFragmentReadme + ")")
	}
	return newsFragment{
		path:     path,
		spec:     metadata[newsKeySpec],
		journal:  metadata[newsKeyJournal],
		topic:    metadata[newsKeyTopic],
		feature:  metadata[newsKeyFeature],
		foundBy:  metadata[newsKeyFoundBy],
		sections: sections,
	}, nil
}

// newsKeys is every front matter key a fragment may write, in the order the
// refusals list them.
var newsKeys = []string{newsKeySpec, newsKeyJournal, newsKeyTopic, newsKeyFeature, newsKeyFoundBy}

// newsFrontMatter splits a fragment into its front matter and its body. It
// answers the problems that make the front matter unreadable. A fragment with
// no front matter answers its whole text as the body, so its headings are
// still judged in the same refusal; one whose front matter never closes
// answers a nil map and no body.
func newsFrontMatter(text string) (map[string]string, string, []string) {
	rest, opened := strings.CutPrefix(text, newsFrontMatterFence+"\n")
	if !opened {
		return nil, text, []string{"it opens with no front matter: start the file with a " + newsFrontMatterFence +
			" line, one `key: value` line for each of " + strings.Join(newsKeys, ", ") + " it uses, and a closing " +
			newsFrontMatterFence + " line"}
	}
	block, body, closed := strings.Cut(rest+"\n", "\n"+newsFrontMatterFence+"\n")
	if !closed {
		return nil, "", []string{"its front matter never closes: end it with a " + newsFrontMatterFence + " line"}
	}
	metadata := map[string]string{}
	var problems []string
	for offset, line := range strings.Split(block, "\n") {
		number := strconv.Itoa(offset + 2)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		key, value, separated := strings.Cut(trimmed, ":")
		if !separated {
			problems = append(problems, "front matter line "+number+" is not `key: value`: write it that way or remove it")
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if !slices.Contains(newsKeys, key) {
			problems = append(problems, "front matter key "+strconv.Quote(key)+" on line "+number+" is not one of "+
				strings.Join(newsKeys, ", ")+": correct its spelling or remove it")
			continue
		}
		if _, repeated := metadata[key]; repeated {
			problems = append(problems, "front matter key "+key+" appears twice: keep one")
			continue
		}
		if value == "" {
			problems = append(problems, "front matter key "+key+" has no value: give it one or remove the line")
			continue
		}
		metadata[key] = value
	}
	return metadata, body, problems
}

// newsMetadataProblems judges the front matter values: one of spec and
// journal, a topic the vocabulary lists, and a found-by from the closed list
// when one is given. A fragment with no found-by is accepted, because most
// changes are not defects a check surfaced.
func newsMetadataProblems(metadata, vocabulary map[string]string) []string {
	var problems []string
	_, spec := metadata[newsKeySpec]
	_, journal := metadata[newsKeyJournal]
	if spec && journal {
		problems = append(problems, "it names both "+newsKeySpec+" and "+newsKeyJournal+
			": a fragment answers one spec closure or one journal row, so keep the one it answers")
	}
	if !spec && !journal {
		problems = append(problems, "it names neither "+newsKeySpec+" nor "+newsKeyJournal+": add `"+newsKeySpec+
			": <spec stem>` for a spec closure, or `"+newsKeyJournal+": <journal class file and date>` for a fix outside a spec")
	}
	if problem := newsTopicProblem(metadata, vocabulary); problem != "" {
		problems = append(problems, problem)
	}
	foundBy, named := metadata[newsKeyFoundBy]
	if named && !slices.Contains(newsFoundBy, foundBy) {
		problems = append(problems, newsKeyFoundBy+" "+strconv.Quote(foundBy)+" is not one of "+
			strings.Join(newsFoundBy, ", ")+": use one of them, or remove the line when no check surfaced the change")
	}
	return problems
}

// newsTopicProblem answers why the fragment's topic is not a tag family of the
// vocabulary, or "" when it is one. A family that differs only in case is
// named, because the vocabulary's casing is canonical.
func newsTopicProblem(metadata, vocabulary map[string]string) string {
	topic, named := metadata[newsKeyTopic]
	if !named {
		return "it names no " + newsKeyTopic + ": add `" + newsKeyTopic + ": <tag family>` from " + newsTopicsPath
	}
	if _, known := vocabulary[topic]; known {
		return ""
	}
	problem := newsKeyTopic + " " + strconv.Quote(topic) + " is not a tag family in " + newsTopicsPath
	for family := range vocabulary {
		if strings.EqualFold(family, topic) {
			return problem + ": write it " + strconv.Quote(family) + ", the vocabulary's spelling"
		}
	}
	return problem + ": use a family it lists, or add " + strconv.Quote(topic) + " to it in this commit"
}

// newsSections reads the body's headings in order and the text under each. A
// `## ` line inside a fenced code block is text, not a heading. It answers
// the trimmed text under each declared heading and every problem with the
// headings: text before the first one, an unknown or repeated one, a missing
// or empty one, a "none" where newsHeadings refuses it, and the order.
func newsSections(body string) (map[string]string, []string) {
	var order []string
	texts := map[string]*strings.Builder{}
	var problems []string
	var current *strings.Builder
	preamble := false
	fenced := false
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
		}
		heading, isHeading := strings.CutPrefix(line, newsHeadingPrefix)
		if isHeading && !fenced {
			name := strings.TrimSpace(heading)
			if _, repeated := texts[name]; repeated {
				problems = append(problems, "heading "+newsHeadingPrefix+name+" appears twice: merge the two sections")
			}
			order = append(order, name)
			current = &strings.Builder{}
			texts[name] = current
			continue
		}
		if current == nil {
			preamble = preamble || trimmed != ""
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
	}
	if preamble {
		problems = append(problems, "it carries text before the first heading: move it under the heading it belongs to")
	}
	return newsSectionTexts(order, texts, problems)
}

// newsSectionTexts judges the headings newsSections read against newsHeadings
// and answers the declared sections' trimmed text with every problem found.
func newsSectionTexts(order []string, texts map[string]*strings.Builder, problems []string) (map[string]string, []string) {
	names := make([]string, 0, len(newsHeadings))
	sections := make(map[string]string, len(newsHeadings))
	for _, heading := range newsHeadings {
		names = append(names, newsHeadingPrefix+heading.Name)
		builder, present := texts[heading.Name]
		if !present {
			problems = append(problems, "heading "+newsHeadingPrefix+heading.Name+" is missing: add it"+newsHeadingFix(heading))
			continue
		}
		text := strings.TrimSpace(builder.String())
		sections[heading.Name] = text
		if text == "" {
			problems = append(problems, "heading "+newsHeadingPrefix+heading.Name+" is empty: write under it"+newsHeadingFix(heading))
			continue
		}
		if !heading.NoneAllowed && newsReadsNone(text) {
			problems = append(problems, "heading "+newsHeadingPrefix+heading.Name+" reads only "+newsNone+
				": say what an operator sees, because a fragment exists to state that change")
		}
	}
	var declared []string
	for _, name := range order {
		if newsHeadingIndex(name) < 0 {
			problems = append(problems, "heading "+newsHeadingPrefix+name+" is not one of "+strings.Join(names, ", ")+
				": rename it to one of them or remove it")
			continue
		}
		if !slices.Contains(declared, name) {
			declared = append(declared, name)
		}
	}
	if !slices.IsSortedFunc(declared, func(left, right string) int {
		return newsHeadingIndex(left) - newsHeadingIndex(right)
	}) {
		problems = append(problems, "its headings are out of order: write "+strings.Join(names, ", ")+", in that order")
	}
	return sections, problems
}

// newsHeadingIndex answers a heading name's place in newsHeadings, and -1 for
// a name that is not one of the six.
func newsHeadingIndex(name string) int {
	return slices.IndexFunc(newsHeadings, func(heading newsHeading) bool { return heading.Name == name })
}

// newsHeadingFix completes a missing or empty heading's refusal: what to write
// under it.
func newsHeadingFix(heading newsHeading) string {
	if heading.NoneAllowed {
		return ", with " + newsNone + " when the fact is truly absent"
	}
	return ", saying what an operator sees; " + newsNone + " is refused here"
}

// newsReadsNone reports whether a section's whole text is "none", whatever
// its case, and with any closing full stop or code or emphasis marks.
func newsReadsNone(text string) bool {
	bare := strings.Trim(strings.TrimSuffix(text, "."), "`*_ ")
	return strings.EqualFold(bare, newsNone)
}
