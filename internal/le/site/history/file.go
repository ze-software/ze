// Design: docs/architecture/site-facts.md -- the committed-facts pattern, its second instance
// Related: sections.go -- the section table whose list the file carries
// Related: days.go -- the Date type a recorded day is keyed by
//
// The History File Contract of
// plan/post-release/engineering/spec-activity-codebase-growth.md: the one
// writer and the one strict reader of website/data/repo-history.json. A
// History exists only validated: NewHistory and ReadHistory are its two
// producers, and both refuse a history whose days are out of order, name an
// unknown section or break the line arithmetic.

package sitehistory

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
)

// HistoryFile is the committed size history, relative to the repository root.
// The site build publishes it as data/repo-history.json.
const HistoryFile = "website/data/repo-history.json"

// SchemaVersion is the version of the file's layout this tool writes and reads.
const SchemaVersion = 1

// historyBytesMax bounds the file the reader accepts. A day is one line of
// about two kilobytes, so the bound holds tens of thousands of days and still
// refuses a file nobody wrote with this tool before it is held in memory.
const historyBytesMax = 64 << 20

// valueMax bounds every count the file holds. No repository holds 2^48 lines,
// and the bound keeps every sum the arithmetic check forms far below overflow.
const valueMax = 1 << 48

// ErrNoHistory is the cause ReadHistory reports when the file does not exist,
// so a caller tells "no history measured yet" apart from a malformed file with
// errors.Is.
var ErrNoHistory = errors.New("no size history file")

// GoLines is the Go split of a Go section's lines.
type GoLines struct {
	// Code counts the lines that hold code.
	Code uint64
	// Blank counts the lines that hold only white space.
	Blank uint64
	// Comment counts the lines that hold only a comment.
	Comment uint64
}

// SectionValues is what one section held on one recorded day.
type SectionValues struct {
	// Files counts the section's regular files.
	Files uint64
	// Lines counts the section's lines.
	Lines uint64
	// Gained counts the lines the section's files gained since the previous
	// recorded day.
	Gained uint64
	// Lost counts the lines the section's files lost since the previous
	// recorded day.
	Lost uint64
	// Go is the split of Lines, held by a section whose GoSplit is set and
	// zero in every other section.
	Go GoLines
}

// DayRecord is one recorded day of the file.
type DayRecord struct {
	// Date is the UTC day.
	Date Date
	// Commit is the full ID of the commit whose tree was measured.
	Commit string
	// Modules counts the modules that tree's vendor/modules.txt lists.
	Modules uint64
	// Sections holds each section's values, keyed by section ID. A section
	// absent from a day held nothing that day.
	Sections map[string]SectionValues
}

// History is a validated size history. Its zero value holds no day, and
// WriteHistory refuses it.
type History struct {
	// rules is the identifier of the rules every day was measured under.
	rules string
	// sections is the file's section list, in its order.
	sections []Section
	// days is every recorded day, ascending.
	days []DayRecord
}

// NewHistory validates a history and answers it. It refuses an empty rules
// identifier, an empty or repeated section, no day, days out of order or
// repeated, a commit that is not a full object ID, a day naming a section the
// list lacks, a count above the bound, a Go split on a section without one or
// not summing to its lines, and any section whose lines on a day differ from
// its lines on the previous day plus gained minus lost.
//
// The history keeps its own copies of the slices and maps it is given.
func NewHistory(rules string, sectionList []Section, days []DayRecord) (History, error) {
	if rules == "" {
		return History{}, errors.New("the size history names no rules identifier")
	}
	split, err := sectionSplits(sectionList)
	if err != nil {
		return History{}, err
	}
	if len(days) == 0 {
		return History{}, errors.New("the size history holds no day")
	}
	for index := range days {
		if err := checkDay(days, index, split); err != nil {
			return History{}, err
		}
	}
	if err := checkArithmetic(sectionList, days); err != nil {
		return History{}, err
	}
	return History{rules: rules, sections: append([]Section(nil), sectionList...), days: copyDays(days)}, nil
}

// Rules answers the identifier of the rules every day was measured under.
func (h History) Rules() string {
	return h.rules
}

// Sections answers a copy of the file's section list, in its order.
func (h History) Sections() []Section {
	return append([]Section(nil), h.sections...)
}

// Days answers a copy of every recorded day, ascending.
func (h History) Days() []DayRecord {
	return copyDays(h.days)
}

// sectionSplits answers whether each listed section carries the Go split,
// keyed by its ID, and refuses an empty list, an empty ID or label, and a
// repeated ID.
func sectionSplits(sectionList []Section) (map[string]bool, error) {
	if len(sectionList) == 0 {
		return nil, errors.New("the size history lists no section")
	}
	split := make(map[string]bool, len(sectionList))
	for position, section := range sectionList {
		if section.ID == "" {
			return nil, fmt.Errorf("section %d of the size history has no identifier", position+1)
		}
		if section.Label == "" {
			return nil, fmt.Errorf("section %s of the size history has no label", section.ID)
		}
		if _, repeated := split[section.ID]; repeated {
			return nil, fmt.Errorf("section %s is listed twice in the size history", section.ID)
		}
		split[section.ID] = section.GoSplit
	}
	return split, nil
}

// checkDay validates one day on its own and against the day before it.
func checkDay(days []DayRecord, index int, split map[string]bool) error {
	day := days[index]
	if day.Date.IsZero() {
		return fmt.Errorf("day %d of the size history holds no date", index+1)
	}
	if index > 0 {
		order := day.Date.Compare(days[index-1].Date)
		if order == 0 {
			return fmt.Errorf("day %s appears twice in the size history", day.Date)
		}
		if order < 0 {
			return fmt.Errorf("day %s follows day %s in the size history, but its days must ascend", day.Date, days[index-1].Date)
		}
	}
	if !objectID(day.Commit) {
		return fmt.Errorf("day %s records %q, which is not a full commit ID", day.Date, day.Commit)
	}
	if day.Modules > valueMax {
		return fmt.Errorf("day %s records %d modules, above the bound of %d", day.Date, day.Modules, uint64(valueMax))
	}
	for id, values := range day.Sections {
		goSplit, known := split[id]
		if !known {
			return unknownSection(day.Date, id)
		}
		if err := checkValues(day.Date, id, goSplit, values); err != nil {
			return err
		}
	}
	return nil
}

// unknownSection is the refusal of a day naming a section the list lacks.
func unknownSection(day Date, id string) error {
	return fmt.Errorf("day %s names section %s, which the size history's section list does not hold", day, id)
}

// checkValues validates one section's values on one day: every count within
// the bound, and the Go split present only where the section carries it and
// summing to the section's lines there.
func checkValues(day Date, id string, goSplit bool, values SectionValues) error {
	for _, count := range []uint64{values.Files, values.Lines, values.Gained, values.Lost, values.Go.Code, values.Go.Blank, values.Go.Comment} {
		if count > valueMax {
			return fmt.Errorf("day %s section %s holds the count %d, above the bound of %d", day, id, count, uint64(valueMax))
		}
	}
	split := values.Go.Code + values.Go.Blank + values.Go.Comment
	if goSplit {
		if split != values.Lines {
			return fmt.Errorf("day %s section %s holds %d lines, but its %d code, %d blank and %d comment lines make %d",
				day, id, values.Lines, values.Go.Code, values.Go.Blank, values.Go.Comment, split)
		}
		return nil
	}
	if split != 0 {
		return fmt.Errorf("day %s section %s holds a Go split, which only a Go section carries", day, id)
	}
	return nil
}

// checkArithmetic refuses any section whose lines on a recorded day differ
// from its lines on the previous recorded day plus the lines it gained minus
// the lines it lost. A section absent from a day counts as zero there.
//
// The no-gap rule needs the history the file describes, so it belongs to the
// callers that hold it, not to this check.
func checkArithmetic(sectionList []Section, days []DayRecord) error {
	for index := 1; index < len(days); index++ {
		previous, current := days[index-1], days[index]
		for _, section := range sectionList {
			before := previous.Sections[section.ID].Lines
			now := current.Sections[section.ID]
			if before+now.Gained == now.Lines+now.Lost {
				continue
			}
			expected := int64(before) + int64(now.Gained) - int64(now.Lost) //nolint:gosec // every count is at most valueMax, 2^48
			return fmt.Errorf("day %s section %s holds %d lines, but its %d lines on %s plus %d gained minus %d lost make %d",
				current.Date, section.ID, now.Lines, before, previous.Date, now.Gained, now.Lost, expected)
		}
	}
	return nil
}

// objectID reports whether id is a full SHA-1 or SHA-256 object ID in lower
// case hex, the only form git answers a commit in.
func objectID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	for _, character := range id {
		digit := '0' <= character && character <= '9'
		letter := 'a' <= character && character <= 'f'
		if !digit && !letter {
			return false
		}
	}
	return true
}

// copyDays answers a copy of days whose maps the caller does not share.
func copyDays(days []DayRecord) []DayRecord {
	out := make([]DayRecord, len(days))
	for index, day := range days {
		out[index] = day
		out[index].Sections = maps.Clone(day.Sections)
	}
	return out
}

// historyJSON is the file as it is decoded. Every field a valid file must hold
// is a pointer, so a missing field is refused rather than read as zero.
type historyJSON struct {
	// Schema is the layout version.
	Schema *int `json:"schema"`
	// Rules is the rules identifier.
	Rules *string `json:"rules"`
	// Sections is the section list.
	Sections []sectionJSON `json:"sections"`
	// Days is every recorded day.
	Days []dayJSON `json:"days"`
}

// sectionJSON is one entry of the file's section list.
type sectionJSON struct {
	// ID is the section's identifier.
	ID *string `json:"id"`
	// Label is the section's label.
	Label *string `json:"label"`
	// GoSplit says the section carries the Go split.
	GoSplit *bool `json:"go-split"`
}

// dayJSON is one recorded day of the file.
type dayJSON struct {
	// Date is the UTC day.
	Date *Date `json:"date"`
	// Commit is the measured commit.
	Commit *string `json:"commit"`
	// Modules is the module count.
	Modules *uint64 `json:"modules"`
	// Sections holds each section's values, keyed by ID.
	Sections map[string]valuesJSON `json:"sections"`
}

// valuesJSON is one section's values on one day.
type valuesJSON struct {
	// Files counts the regular files.
	Files *uint64 `json:"files"`
	// Lines counts the lines.
	Lines *uint64 `json:"lines"`
	// Gained counts the lines gained.
	Gained *uint64 `json:"gained"`
	// Lost counts the lines lost.
	Lost *uint64 `json:"lost"`
	// Go is the Go split, written only for a section that carries it.
	Go *goJSON `json:"go,omitempty"`
}

// goJSON is the Go split of one section's lines.
type goJSON struct {
	// Code counts the code lines.
	Code *uint64 `json:"code"`
	// Blank counts the blank lines.
	Blank *uint64 `json:"blank"`
	// Comment counts the comment lines.
	Comment *uint64 `json:"comment"`
}

// ReadHistory reads and validates the size history under root. A missing file
// is refused with ErrNoHistory as its cause; any other refusal names what is
// wrong with the file.
func ReadHistory(root string) (History, error) {
	path := filepath.Join(root, filepath.FromSlash(HistoryFile))
	raw, err := readBounded(path)
	if errors.Is(err, fs.ErrNotExist) {
		return History{}, opererr.Wrap(ErrNoHistory, HistoryFile+" does not exist",
			"no size history has been measured into this checkout yet",
			"./le site history update all")
	}
	if err != nil {
		return History{}, opererr.Wrap(err, "cannot read "+HistoryFile,
			"the size history is the only source of the sizes the activity page publishes",
			"make "+path+" readable, then run the command again")
	}
	history, err := decodeHistory(raw)
	if err != nil {
		return History{}, opererr.Wrap(err, HistoryFile+" is not a valid size history",
			"every size the activity page publishes comes from this file, so a file that breaks its contract publishes wrong sizes",
			"./le site history update all")
	}
	return history, nil
}

// readBounded reads path whole, refusing a file larger than historyBytesMax.
func readBounded(path string) ([]byte, error) {
	file, err := os.Open(path) //nolint:gosec // the path is the checkout's own history file
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // a read-only file holds nothing a failed close could lose

	raw, err := io.ReadAll(io.LimitReader(file, historyBytesMax+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > historyBytesMax {
		return nil, fmt.Errorf("the file is larger than the bound of %d bytes", historyBytesMax)
	}
	return raw, nil
}

// decodeHistory decodes the file strictly, refusing an unknown field, a
// missing one, and anything after the one JSON object, then validates it.
func decodeHistory(raw []byte) (History, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var file historyJSON
	if err := decoder.Decode(&file); err != nil {
		return History{}, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return History{}, errors.New("the file holds more than its one JSON object")
	}
	if file.Schema == nil {
		return History{}, errors.New("the file names no schema")
	}
	if *file.Schema != SchemaVersion {
		return History{}, fmt.Errorf("the file is schema %d, and this tool reads schema %d", *file.Schema, SchemaVersion)
	}
	if file.Rules == nil {
		return History{}, errors.New("the file names no rules identifier")
	}
	sectionList, err := decodeSections(file.Sections)
	if err != nil {
		return History{}, err
	}
	days, err := decodeDays(file.Days, sectionList)
	if err != nil {
		return History{}, err
	}
	return NewHistory(*file.Rules, sectionList, days)
}

// decodeSections converts the file's section list, refusing a missing field.
func decodeSections(entries []sectionJSON) ([]Section, error) {
	out := make([]Section, 0, len(entries))
	for position, entry := range entries {
		if entry.ID == nil {
			return nil, fmt.Errorf("section %d of the file has no id", position+1)
		}
		if entry.Label == nil {
			return nil, fmt.Errorf("section %s of the file has no label", *entry.ID)
		}
		if entry.GoSplit == nil {
			return nil, fmt.Errorf("section %s of the file does not say whether it carries the Go split", *entry.ID)
		}
		out = append(out, Section{ID: *entry.ID, Label: *entry.Label, GoSplit: *entry.GoSplit})
	}
	return out, nil
}

// decodeDays converts the file's days, refusing a missing field, a section the
// list lacks, and a Go split present where the section carries none or absent
// where it does.
func decodeDays(entries []dayJSON, sectionList []Section) ([]DayRecord, error) {
	split := make(map[string]bool, len(sectionList))
	for _, section := range sectionList {
		split[section.ID] = section.GoSplit
	}
	out := make([]DayRecord, 0, len(entries))
	for position, entry := range entries {
		if entry.Date == nil {
			return nil, fmt.Errorf("day %d of the file has no date", position+1)
		}
		if entry.Commit == nil {
			return nil, fmt.Errorf("day %s of the file has no commit", *entry.Date)
		}
		if entry.Modules == nil {
			return nil, fmt.Errorf("day %s of the file has no module count", *entry.Date)
		}
		if entry.Sections == nil {
			return nil, fmt.Errorf("day %s of the file has no sections", *entry.Date)
		}
		day := DayRecord{Date: *entry.Date, Commit: *entry.Commit, Modules: *entry.Modules, Sections: make(map[string]SectionValues, len(entry.Sections))}
		for id, encoded := range entry.Sections {
			goSplit, known := split[id]
			if !known {
				return nil, unknownSection(day.Date, id)
			}
			values, err := decodeValues(day.Date, id, goSplit, encoded)
			if err != nil {
				return nil, err
			}
			day.Sections[id] = values
		}
		out = append(out, day)
	}
	return out, nil
}

// decodeValues converts one section's values on one day.
func decodeValues(day Date, id string, goSplit bool, encoded valuesJSON) (SectionValues, error) {
	fields := []struct {
		name  string
		value *uint64
	}{{"files", encoded.Files}, {"lines", encoded.Lines}, {"gained", encoded.Gained}, {"lost", encoded.Lost}}
	for _, field := range fields {
		if field.value == nil {
			return SectionValues{}, fmt.Errorf("day %s section %s has no %s", day, id, field.name)
		}
	}
	values := SectionValues{Files: *encoded.Files, Lines: *encoded.Lines, Gained: *encoded.Gained, Lost: *encoded.Lost}
	if !goSplit {
		if encoded.Go != nil {
			return SectionValues{}, fmt.Errorf("day %s section %s holds a Go split, which only a Go section carries", day, id)
		}
		return values, nil
	}
	if encoded.Go == nil {
		return SectionValues{}, fmt.Errorf("day %s section %s carries the Go split and holds none", day, id)
	}
	if encoded.Go.Code == nil {
		return SectionValues{}, fmt.Errorf("day %s section %s has no code line count", day, id)
	}
	if encoded.Go.Blank == nil {
		return SectionValues{}, fmt.Errorf("day %s section %s has no blank line count", day, id)
	}
	if encoded.Go.Comment == nil {
		return SectionValues{}, fmt.Errorf("day %s section %s has no comment line count", day, id)
	}
	values.Go = GoLines{Code: *encoded.Go.Code, Blank: *encoded.Go.Blank, Comment: *encoded.Go.Comment}
	return values, nil
}

// WriteHistory writes history to the size history file under root and answers
// the path it wrote. The bytes go to a temporary file in the same directory,
// which is then renamed over the file, so a refusal or a crash leaves the old
// file whole. The same history writes the same bytes.
func WriteHistory(root string, history History) (string, error) {
	if history.rules == "" {
		return "", errors.New("the size history to write holds no day; build it with NewHistory")
	}
	content, err := encodeHistory(history)
	if err != nil {
		return "", fmt.Errorf("encode %s: %w", HistoryFile, err)
	}

	path := filepath.Join(root, filepath.FromSlash(HistoryFile))
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return "", fmt.Errorf("create %s: %w", directory, err)
	}
	if err := replaceFile(path, content); err != nil {
		return "", fmt.Errorf("replace %s: %w", path, err)
	}
	return path, nil
}

// replaceFile writes content to a temporary file beside path, then renames it
// over path. The temporary file is removed on every path that does not rename.
func replaceFile(path string, content []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".repo-history-*.json")
	if err != nil {
		return err
	}
	temporary := file.Name()
	// After the rename the temporary name no longer exists, so this removal
	// fails then by design and its error says nothing.
	defer os.Remove(temporary) //nolint:errcheck // see the comment above

	if _, err := file.Write(content); err != nil {
		file.Close() //nolint:errcheck // the write error is the one reported
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close() //nolint:errcheck // the sync error is the one reported
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

// encodeHistory renders the file: a fixed key order, one section and one day
// per line, two-space indentation and a trailing newline. A ranged update
// therefore changes only the lines of the days it measured.
func encodeHistory(history History) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("{\n  \"schema\": ")
	out.WriteString(strconv.Itoa(SchemaVersion))
	out.WriteString(",\n  \"rules\": ")
	rules, err := json.Marshal(history.rules)
	if err != nil {
		return nil, err
	}
	out.Write(rules)

	split := make(map[string]bool, len(history.sections))
	lines := make([][]byte, 0, len(history.days))
	for _, section := range history.sections {
		split[section.ID] = section.GoSplit
		line, err := json.Marshal(sectionJSON{ID: &section.ID, Label: &section.Label, GoSplit: &section.GoSplit})
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	out.WriteString(",\n  \"sections\": [\n")
	writeLines(&out, lines)

	lines = lines[:0]
	for _, day := range history.days {
		line, err := json.Marshal(encodeDay(day, split))
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	out.WriteString("  ],\n  \"days\": [\n")
	writeLines(&out, lines)
	out.WriteString("  ]\n}\n")
	return out.Bytes(), nil
}

// writeLines writes one array element per line, comma-separated.
func writeLines(out *bytes.Buffer, lines [][]byte) {
	for index, line := range lines {
		out.WriteString("    ")
		out.Write(line)
		if index < len(lines)-1 {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
}

// encodeDay converts one day for the writer. The encoder writes the section
// map's keys sorted, so the order is fixed.
func encodeDay(day DayRecord, split map[string]bool) dayJSON {
	date, commit, modules := day.Date, day.Commit, day.Modules
	encoded := dayJSON{Date: &date, Commit: &commit, Modules: &modules, Sections: make(map[string]valuesJSON, len(day.Sections))}
	for id, values := range day.Sections {
		entry := valuesJSON{Files: &values.Files, Lines: &values.Lines, Gained: &values.Gained, Lost: &values.Lost}
		if split[id] {
			entry.Go = &goJSON{Code: &values.Go.Code, Blank: &values.Go.Blank, Comment: &values.Go.Comment}
		}
		encoded.Sections[id] = entry
	}
	return encoded
}
