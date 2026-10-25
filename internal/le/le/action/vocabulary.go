// Design: docs/contributing/running-commands.md -- le's verb vocabulary
// Related: leaction.go -- Action.Does and Action.Frozen, which the name rules read
//
// le's verbs form one closed set (D-Q9 and D-P4-vocab of spec-le-cli-usability).
// Every action word is either a VERB of this table, performing that verb's one
// meaning, or a NAME: a noun or adjective naming what the action works on (a
// suite, a scenario, a platform, a shell, an artifact, a selection), which
// declares the vocabulary verb it performs in Action.Does. The tests over the
// registered tree hold every area to it; this file is the one declaration they
// and `le help ai verbs` read.

package leaction

import "slices"

// VerbKind says what a verb does to the world, so a reader knows from the word
// alone whether it may change files and what its exit code means. The zero
// value is no kind, so an entry that names none is visibly incomplete.
type VerbKind uint8

const (
	VerbKindUnspecified VerbKind = iota
	// VerbReport reads and prints, and exits 0 whenever it answered.
	VerbReport
	// VerbGate reads and exits 1 on a defect.
	VerbGate
	// VerbRegenerate rewrites a committed artifact from its source.
	VerbRegenerate
	// VerbRecord appends to a ledger.
	VerbRecord
	// VerbRender produces a published artifact this repository does not commit.
	VerbRender
	// VerbRun has side effects outside the tree's committed files.
	VerbRun
)

// String names the kind for a page and for `le help ai verbs`.
func (k VerbKind) String() string {
	switch k {
	case VerbReport:
		return "report"
	case VerbGate:
		return "gate"
	case VerbRegenerate:
		return "regenerate"
	case VerbRecord:
		return "record"
	case VerbRender:
		return "render"
	case VerbRun:
		return "run"
	case VerbKindUnspecified:
		return "unspecified"
	default:
		panic("BUG: leaction.VerbKind: a verb kind outside the declared set")
	}
}

// Verb is one entry of the closed vocabulary: the word, its one meaning, and
// its kind.
type Verb struct {
	Word    string
	Meaning string
	Kind    VerbKind
}

// vocabulary is the closed set, in the order the page lists it.
var vocabulary = []Verb{
	{"report", "print what the area finds in the tree, read-only", VerbReport},
	{"list", "print the members of a set, one per line", VerbReport},
	{"show", "print one recorded item: a claim, a certificate, a path, a model, a script", VerbReport},
	{"preview", "print what another action would do or write, doing and writing nothing", VerbReport},
	{"compare", "print the difference between two or more inputs", VerbReport},
	{"track", "print the trend of a history ledger over time; exit 1 on a regression only when asked", VerbReport},
	{"suggest", "say whether a run is owed and why, never failing", VerbReport},
	{"discriminate", "print which breaks of the producing code prove a tagged requirement, and what is still unproven", VerbReport},
	{"combine", "merge several result files into one result file outside the committed tree", VerbReport},
	{"check", "run the area's gate over the tree; exit 1 on a defect", VerbGate},
	{"selftest", "run the gate against its seeded fixtures, proving it catches what it claims", VerbGate},
	{"judge", "decide one proposed edit read on stdin; exit 1 when refused", VerbGate},
	{"update", "rewrite a committed artifact from its canonical source", VerbRegenerate},
	{"reseal", "re-pin the recorded fingerprints a mechanical edit moved, keeping every verdict", VerbRegenerate},
	{"record", "append one result, verdict or measurement to its ledger", VerbRecord},
	{"approve", "record the owner's approval of one named change, in his words", VerbRecord},
	{"render", "produce a page, deck or recording for the published site", VerbRender},
	{"run", "start the area's job, with its side effects; exit is the job's", VerbRun},
	{"build", "produce a binary or image that is not committed", VerbRun},
	{"send", "send traffic at a target", VerbRun},
	{"install", "install tools, a kernel or a profile on this host", VerbRun},
	{"start", "start a long-lived service", VerbRun},
	{"stop", "stop a long-lived service", VerbRun},
	{"clean", "remove what earlier runs left behind, only what this checkout or run provably owns", VerbRun},
	{"trim", "bound a store's size by evicting its oldest entries", VerbRun},
	{"ensure", "create the item when absent and leave it when present, then print it", VerbRun},
	{"create", "create one new item, such as a commit script, each time", VerbRun},
	{"migrate", "convert files from a retired layout to the current one", VerbRun},
	{"rewrite", "apply one mechanical edit to tracked files, previewing unless told to apply", VerbRun},
	{"move", "relocate code or files to another path, carrying what refers to them", VerbRun},
	{"rename", "change a name everywhere it is spelled", VerbRun},
	{"rebase", "rebase a worktree branch onto its upstream", VerbRun},
	{"push", "prepare the owner-authorized push of a verified commit", VerbRun},
	{"publish", "post an announcement to an outside channel", VerbRun},
	{"claim", "claim a spec for this session", VerbRun},
	{"rollback", "set the newest le build aside so the previous one runs", VerbRun},
}

// retiredVerbs are the words the owner ruled out of the vocabulary: `write`
// becomes `update`, and `generate` with it. A name that holds one as a hyphen
// part still names that retired verb, so the name rules refuse it too.
var retiredVerbs = []string{"write", "generate"}

// testWord is the word no name may hold as a hyphen part: a `*-test` word is a
// suite named for the act of testing it, which is the area's job, not a name
// (the owner's ruling over `ipsec-mobike-test` and `condensed-update`).
const testWord = "test"

// SharedName is the one name every area may use: every member of the area,
// in table order.
const SharedName = "all"

// Vocabulary answers a copy of the closed verb set, in its declared order.
func Vocabulary() []Verb {
	return slices.Clone(vocabulary)
}

// VocabularyVerb answers the vocabulary entry spelled word, and whether one is.
func VocabularyVerb(word string) (Verb, bool) {
	index := slices.IndexFunc(vocabulary, func(verb Verb) bool { return verb.Word == word })
	if index < 0 {
		return Verb{}, false
	}
	return vocabulary[index], true
}

// ForbiddenNamePart reports whether a hyphen part of a name may not be part:
// a vocabulary verb, a retired verb, or `test`. A name holding one hides a
// second verb inside one word, which the vocabulary exists to prevent.
func ForbiddenNamePart(part string) bool {
	if _, ok := VocabularyVerb(part); ok {
		return true
	}
	if slices.Contains(retiredVerbs, part) {
		return true
	}
	return part == testWord
}
