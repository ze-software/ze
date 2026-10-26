// Design: docs/contributing/rfc-conformance-gates.md -- profiles and technology aggregates
// Related: meta.go -- the summary's `Support` and `Aggregate` cells, which name an aggregate declared here
// Related: profilecoverage.go -- the mandatory and optional figures a profile rolls up
//
// profile.go reads rfc/profiles/<name>.md. A profile is one of two kinds:
//
//   - a use-case profile names the root RFCs a deployment needs, and the RFCs
//     it needs that the corpus does not hold yet;
//   - a technology aggregate is the group a summary names in its own Meta
//     table, and it carries the heading, intro and table shape the public
//     status page renders that group under.
//
// Membership is declared by the RFC, never by the aggregate: a new summary
// registers itself by naming its aggregate, and no file here lists it.
package rfc

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// profileRel is the directory every profile file lives in.
const profileRel = "rfc/profiles"

// The fields of a profile's `| Field | Value |` table.
const (
	profileKind    = "Kind"
	profileTitle   = "Title"
	profileRelease = "Release"
	profileRoots   = "Roots"
	profileAbsent  = "Absent"
	profileHeading = "Heading"
	profileIntro   = "Intro"
	profileShape   = "Shape"
)

// The two kinds a profile declares.
const (
	profileKindUseCase    = "use-case"
	profileKindTechnology = "technology"
)

// Shape is the public table an aggregate's rows render as.
type Shape uint8

const (
	// shapeUnspecified is the zero value, which no parsed aggregate holds.
	shapeUnspecified Shape = iota
	// ShapeWide is the six-column table: RFC, Area, Status, Proof,
	// Implemented coverage, Remaining.
	ShapeWide
	// ShapeBrief is the four-column table for standards that are not RFCs:
	// Standard, Area, Status, Note. Its `Support remaining` is held at `-`.
	ShapeBrief
)

// Aggregate is one technology aggregate.
type Aggregate struct {
	Name    string
	Title   string
	Heading string
	// Intro is the prose between the heading and the table, empty for an
	// aggregate whose file writes `-`.
	Intro string
	Shape Shape
}

// UseCase is one use-case profile.
type UseCase struct {
	Name    string
	Title   string
	Release bool
	// Roots are summary stems, each one present in rfc/short.
	Roots []string
	// Absent are RFC numbers the use case needs and the corpus does not hold.
	// Each renders at 0%, never as "not walked" (owner, 2026-10-25).
	Absent []int
}

// Profiles is every parsed profile file, by name.
type Profiles struct {
	UseCases   map[string]UseCase
	Aggregates map[string]Aggregate
}

// profileNameRE is the file name shape, which is also the name a summary writes.
var profileNameRE = regexp.MustCompile(`\A[a-z0-9]+(-[a-z0-9]+)*\z`)

// absentRE is one Absent item: `RFC <number>`, at most five digits.
var absentRE = regexp.MustCompile(`\ARFC ([1-9]\d{0,4})\z`)

// LoadProfiles reads every rfc/profiles/*.md against the summary stems, which
// it reads from the tree when stems is nil.
//
// A profile that does not parse is COLLECTED as a problem rather than
// returned, the same way a summary is: one file midway through an edit must
// not stop `./le rfc check` for every session. The bad profile is absent from
// the answer, so every summary naming it is refused as naming an unknown
// aggregate, which is loud. The error return is for a directory that cannot
// be read at all.
func LoadProfiles(tree string, stems map[string]bool) (Profiles, []string, error) {
	out := Profiles{UseCases: map[string]UseCase{}, Aggregates: map[string]Aggregate{}}
	if stems == nil {
		var err error
		if stems, err = summaryStems(tree); err != nil {
			return Profiles{}, nil, err
		}
	}
	entries, err := os.ReadDir(treePath(tree, profileRel))
	if err != nil {
		if os.IsNotExist(err) {
			var tb textbuf.Buffer
			return out, []string{tb.Str(profileRel).Str(": the directory is absent, so no ").
				Str("aggregate is declared and every summary's `Support` names an unknown one. ").
				Str("`git status ").Str(profileRel).Str("` names what removed it").String()}, nil
		}
		return Profiles{}, nil, err
	}
	var problems []string
	for _, entry := range entries {
		name, isProfile := strings.CutSuffix(entry.Name(), ".md")
		if entry.IsDir() || !isProfile {
			continue
		}
		rel := profileRel + "/" + entry.Name()
		text, err := readFile(filepath.Join(treePath(tree, profileRel), entry.Name()), rel)
		if err != nil {
			return Profiles{}, nil, err
		}
		if err := parseProfile(&out, text, name, rel, stems); err != nil {
			problems = append(problems, err.Error())
		}
	}
	return out, problems, nil
}

// parseProfile reads one profile into out, refusing an unknown or missing
// field, an unknown kind, a root that is not a summary, and an absent RFC the
// corpus already holds.
func parseProfile(out *Profiles, text, name, rel string, stems map[string]bool) error {
	if !profileNameRE.MatchString(name) {
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": the file name is not a lower-case hyphenated name, ").
			Str("and a summary writes that name in its `Support` cell. Rename the file"))
	}
	header := "# Profile: " + name
	first, _, _ := strings.Cut(text, "\n")
	if strings.TrimSpace(first) != header {
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": the first line is ").Str(pyRepr(first)).
			Str(", not ").Str(pyRepr(header)).Str(". Write that heading"))
	}
	values, err := profileFields(text, rel)
	if err != nil {
		return err
	}
	switch values[profileKind] {
	case profileKindUseCase:
		useCase, err := parseUseCase(values, name, rel, stems)
		if err != nil {
			return err
		}
		out.UseCases[name] = useCase
		return nil
	case profileKindTechnology:
		aggregate, err := parseAggregate(values, name, rel)
		if err != nil {
			return err
		}
		out.Aggregates[name] = aggregate
		return nil
	}
	var tb textbuf.Buffer
	return parseErr(tb.Str(rel).Str(": `").Str(profileKind).Str("` is ").
		Str(pyRepr(values[profileKind])).Str(", which is neither `").Str(profileKindUseCase).
		Str("` nor `").Str(profileKindTechnology).Str("`"))
}

// profileFields answers the `| Field | Value |` rows, refusing a repeated label.
func profileFields(text, rel string) (map[string]string, error) {
	values := map[string]string{}
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		if len(cells) != 2 {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(rel).Str(": the row ").Str(pyRepr(truncateRunes(trimmed, 60))).
				Str(" has ").Int(int64(len(cells))).Str(" cells, not 2"))
		}
		label, value := strings.TrimSpace(cells[0]), strings.TrimSpace(cells[1])
		if label == "Field" || strings.Trim(label, "-") == "" {
			continue
		}
		if _, held := values[label]; held {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(rel).Str(": `").Str(label).Str("` is declared twice"))
		}
		values[label] = value
	}
	return values, nil
}

// requireFields refuses a missing field, an empty one, and any field the kind
// does not read, so a misspelled label is never skipped in silence.
func requireFields(values map[string]string, want []string, rel string) error {
	for _, label := range want {
		if values[label] != "" {
			continue
		}
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": `").Str(label).Str("` is absent or empty. A `").
			Str(values[profileKind]).Str("` profile declares ").Str(pyRepr(want)))
	}
	for _, label := range sortedKeysOf(values) {
		if slices.Contains(want, label) {
			continue
		}
		var tb textbuf.Buffer
		return parseErr(tb.Str(rel).Str(": `").Str(label).Str("` is not a field of a `").
			Str(values[profileKind]).Str("` profile, which declares ").Str(pyRepr(want)).
			Str(". Delete the row or correct its label"))
	}
	return nil
}

// parseUseCase reads a use-case profile.
func parseUseCase(values map[string]string, name, rel string, stems map[string]bool) (UseCase, error) {
	want := []string{profileKind, profileTitle, profileRelease, profileRoots, profileAbsent}
	if err := requireFields(values, want, rel); err != nil {
		return UseCase{}, err
	}
	out := UseCase{Name: name, Title: values[profileTitle]}
	switch values[profileRelease] {
	case "yes":
		out.Release = true
	case "no":
		out.Release = false
	default:
		var tb textbuf.Buffer
		return UseCase{}, parseErr(tb.Str(rel).Str(": `").Str(profileRelease).Str("` is ").
			Str(pyRepr(values[profileRelease])).Str(", which is neither `yes` nor `no`"))
	}
	for root := range strings.SplitSeq(values[profileRoots], ",") {
		root = strings.TrimSpace(root)
		if !stems[root] {
			// AC-9: a root that is not a summary would drop out of the
			// closure in silence. It is either summarized or listed Absent.
			var tb textbuf.Buffer
			return UseCase{}, parseErr(tb.Str(rel).Str(": profile ").Str(pyRepr(name)).
				Str(" names root ").Str(pyRepr(root)).Str(", which has no ").Str(summaryRel).
				Byte('/').Str(root).Str(".md. Correct the stem, or move it to `").
				Str(profileAbsent).Str("` as `RFC <number>`"))
		}
		out.Roots = append(out.Roots, root)
	}
	if values[profileAbsent] == supportNone {
		return out, nil
	}
	for item := range strings.SplitSeq(values[profileAbsent], ",") {
		number, err := absentNumber(strings.TrimSpace(item), name, rel, stems)
		if err != nil {
			return UseCase{}, err
		}
		out.Absent = append(out.Absent, number)
	}
	return out, nil
}

// absentNumber reads one Absent item and refuses one the corpus holds: an RFC
// with a summary is a root or reached, and listing it absent would show it at
// 0% beside its own figure.
func absentNumber(item, name, rel string, stems map[string]bool) (int, error) {
	match := absentRE.FindStringSubmatch(item)
	if match == nil {
		var tb textbuf.Buffer
		return 0, parseErr(tb.Str(rel).Str(": `").Str(profileAbsent).Str("` item ").Str(pyRepr(item)).
			Str(" is not `RFC <number>`"))
	}
	number, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, err
	}
	if stems["rfc"+match[1]] {
		var tb textbuf.Buffer
		return 0, parseErr(tb.Str(rel).Str(": profile ").Str(pyRepr(name)).Str(" lists ").
			Str(item).Str(" as absent, but ").Str(summaryRel).Str("/rfc").Str(match[1]).
			Str(".md exists. Move it to `").Str(profileRoots).Str("` or delete the item"))
	}
	return number, nil
}

// parseAggregate reads a technology aggregate.
func parseAggregate(values map[string]string, name, rel string) (Aggregate, error) {
	want := []string{profileKind, profileTitle, profileHeading, profileIntro, profileShape, profileRelease}
	if err := requireFields(values, want, rel); err != nil {
		return Aggregate{}, err
	}
	if values[profileRelease] != "n/a" {
		var tb textbuf.Buffer
		return Aggregate{}, parseErr(tb.Str(rel).Str(": `").Str(profileRelease).Str("` is ").
			Str(pyRepr(values[profileRelease])).Str(", but only a use-case profile is a release ").
			Str("profile. Write `n/a`"))
	}
	out := Aggregate{Name: name, Title: values[profileTitle], Heading: values[profileHeading]}
	if values[profileIntro] != supportNone {
		out.Intro = values[profileIntro]
	}
	switch values[profileShape] {
	case "wide":
		out.Shape = ShapeWide
	case "brief":
		out.Shape = ShapeBrief
	default:
		var tb textbuf.Buffer
		return Aggregate{}, parseErr(tb.Str(rel).Str(": `").Str(profileShape).Str("` is ").
			Str(pyRepr(values[profileShape])).Str(", which is neither `wide` nor `brief`"))
	}
	return out, nil
}
