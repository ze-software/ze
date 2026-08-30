// Design: docs/architecture/core-design.md -- the RFC area owns its writers
// Overview: inventory.go -- the source walk this writer records
// Related: artifact.go -- the validating reader every staged document must pass
//
// extraction_create.go is ze-rfc-extraction-create. It derives an unsigned
// skeleton, preserves authored decisions that still govern the same sentence,
// validates the staged bytes with the production parser, and writes the
// artifact only after that validation succeeds.
//
// Where it writes is decided by ONE property: an artifact reaches
// rfc/extraction/ only when every site and every section already carries a
// disposition. An unclassified skeleton goes to this session's scratch instead,
// because an unclassified artifact under rfc/extraction/ fails `./le rfc check`
// for the whole corpus, and a generator whose own output reds the gate invites
// exactly one accident: 44 skeletons written in a batch and left there.
package rfc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	extractionStagingPrefix = ".staging-"
	extractionStagingMaxAge = time.Hour
	// extractionScratchDir is the subdirectory of this session's scratch that
	// holds skeletons still being walked. One directory per session, so two
	// agents walking two RFCs never meet.
	extractionScratchDir = "rfc-extraction"
)

var extractionStemRE = regexp.MustCompile(`\A[a-z0-9][a-z0-9._-]*\z`)

// extractionCreateReport is the local-data answer for one skeleton write.
//
// Path is where the bytes landed and Destination is where a sign-off lives, so
// a caller reads whether the two agree rather than comparing strings.
type extractionCreateReport struct {
	Path                 string `json:"path"`
	Destination          string `json:"destination"`
	Placed               bool   `json:"placed"`
	Register             string `json:"register"`
	Sites                int    `json:"sites"`
	Sections             int    `json:"sections"`
	UnclassifiedSites    int    `json:"unclassified-sites"`
	UnclassifiedSections int    `json:"unclassified-sections"`
}

// Text answers what the author must do next. Pipe renderers consume the same
// fields through the JSON tags above.
func (r extractionCreateReport) Text() string {
	var out textbuf.Buffer
	out.Str("wrote ").Str(r.Path).Str(": register ").Str(r.Register).Str(", ").
		Int(int64(r.Sites)).Str(" site(s) in ").Int(int64(r.Sections)).Str(" section(s).\n")
	if r.Placed {
		return out.Str("Every site and section carried a classification forward from the ").
			Str("previous sign-off, so the refreshed artifact was written in place.\n").String()
	}
	return out.Int(int64(r.UnclassifiedSites)).Str(" site(s) and ").
		Int(int64(r.UnclassifiedSections)).Str(" section(s) are UNCLASSIFIED, so the ").
		Str("skeleton was NOT written to ").Str(r.Destination).Str(".\n").
		Str("Classify every one by hand in the file above, then move it in:\n  mv ").
		Str(r.Path).Byte(' ').Str(r.Destination).Byte('\n').
		Str("Generation cannot produce a sign-off; only a walk can, and an unclassified ").
		Str("artifact under rfc/extraction/ fails `./le rfc check` for the whole corpus.\n").
		String()
}

type extractionDocument struct {
	SchemaVersion  int                         `json:"schema-version"`
	Stem           string                      `json:"stem"`
	Register       string                      `json:"register"`
	SourcePath     string                      `json:"source-path"`
	SourceSHA      string                      `json:"source-sha"`
	SignedOff      string                      `json:"signed-off"`
	Reviewer       string                      `json:"reviewer"`
	RegisterReason string                      `json:"register-reason,omitempty"`
	ResignReason   string                      `json:"resign-reason,omitempty"`
	Sections       []extractionDocumentSection `json:"sections"`
	Sites          []extractionDocumentSite    `json:"sites"`
}

type extractionDocumentSection struct {
	ID           string   `json:"id"`
	Sites        int      `json:"sites"`
	Disposition  *string  `json:"disposition"`
	SkipKind     string   `json:"skip-kind,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	UnsourcedIDs []string `json:"unsourced-ids,omitempty"`
}

type extractionDocumentSite struct {
	ID           string  `json:"id"`
	Quote        string  `json:"quote"`
	Disposition  *string `json:"disposition"`
	ExcludedKind string  `json:"excluded-kind,omitempty"`
	Reason       string  `json:"reason,omitempty"`
	MappedTo     string  `json:"mapped-to,omitempty"`
	RelocatedTo  string  `json:"relocated-to,omitempty"`
	ReservedID   string  `json:"reserved-id,omitempty"`
}

// createExtraction derives and atomically writes one extraction skeleton.
func createExtraction(tree, stem string) (extractionCreateReport, error) {
	if err := validateExtractionStem(stem); err != nil {
		return extractionCreateReport{}, err
	}

	gated := summaryGatedCount(tree, stem)
	inventory, err := NewDeriver(tree).Inventory(stem, gated)
	if err != nil {
		return extractionCreateReport{}, err
	}
	if inventory == nil {
		var message textbuf.Buffer
		return extractionCreateReport{}, errors.New(message.Str(stem).
			Str(" has no source text at rfc/full/").Str(stem).Str(".txt or rfc/drafts/").
			Str(stem).Str(".txt. Fetch it (https://www.rfc-editor.org/rfc/").Str(stem).
			Str(".txt) before extracting: with no source there is no inventory to derive and no register to sign under").String())
	}

	path := treePath(tree, extractionRel+"/"+stem+".json")
	var previous *Extraction
	if _, statErr := os.Stat(path); statErr == nil {
		parsed, parseErr := ParseExtractionArtifact(tree, path)
		if parseErr != nil {
			return extractionCreateReport{}, parseErr
		}
		previous = &parsed
	} else if !os.IsNotExist(statErr) {
		var message textbuf.Buffer
		return extractionCreateReport{}, errors.New(message.Str(relTo(tree, path)).Str(": cannot read: ").Err(statErr).String())
	}

	document := newExtractionDocument(inventory, previous)
	report := extractionCreateReport{
		Destination:          relTo(tree, path),
		Register:             document.Register,
		Sites:                len(document.Sites),
		Sections:             len(document.Sections),
		UnclassifiedSites:    countUnclassifiedSites(document.Sites),
		UnclassifiedSections: countUnclassifiedSections(document.Sections),
	}

	// The whole guard, in one condition. A refresh whose every decision carried
	// forward is already a sign-off and belongs in the corpus; anything else is
	// a skeleton, and a skeleton in the corpus reds the gate.
	directory := treePath(tree, extractionRel)
	if report.UnclassifiedSites+report.UnclassifiedSections > 0 {
		directory, err = extractionScratch(tree)
		if err != nil {
			return extractionCreateReport{}, err
		}
	} else {
		report.Placed = true
	}

	if err := writeExtractionDocument(tree, directory, stem, document); err != nil {
		return extractionCreateReport{}, err
	}
	report.Path = relTo(tree, filepath.Join(directory, stem+".json"))
	return report, nil
}

// extractionScratch answers this session's skeleton directory, created.
//
// It is under tmp/session/, which is where every agent writes work in progress
// (ai/rules/commands.md), so two sessions walking two RFCs never collide and
// nothing an author has not finished can reach the tree the gate judges.
func extractionScratch(tree string) (string, error) {
	paths, err := lepath.ResolveSession(tree, true)
	if err != nil {
		var message textbuf.Buffer
		return "", errors.New(message.Str("cannot resolve this session's scratch directory, ").
			Str("which is where an unclassified skeleton is written: ").Err(err).String())
	}
	directory := treePath(tree, paths.Scratch, extractionScratchDir)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		var message textbuf.Buffer
		return "", errors.New(message.Str(relTo(tree, directory)).
			Str(": cannot create directory: ").Err(err).String())
	}
	return directory, nil
}

func validateExtractionStem(stem string) error {
	if extractionStemRE.MatchString(stem) && !strings.Contains(stem, "..") {
		return nil
	}
	var message textbuf.Buffer
	return errors.New(message.Str("stem ").Str(pyRepr(stem)).
		Str(" is not an RFC or draft stem (lowercase letters, digits, '.', '-', '_'; no path separator). ").
		Str("The stem names the source text and the artifact file, so it may never carry a path").String())
}

// summaryGatedCount preserves the legacy writer's failure direction: an absent
// or malformed summary supplies zero declared requirements. That can select a
// stronger derived register, so the public check still reports the malformed
// summary and governs whether the refreshed artifact can earn sign-off.
func summaryGatedCount(tree, stem string) int {
	path := treePath(tree, summaryRel+"/"+stem+".md")
	if _, err := os.Stat(path); err != nil {
		return 0
	}
	requirements, err := parseSummaryFile(tree, path)
	if err != nil {
		return 0
	}
	return gatedCounts(requirements)[stem]
}

func newExtractionDocument(inventory *Inventory, previous *Extraction) extractionDocument {
	previousSites := map[string]ExtractionSite{}
	previousSections := map[string]ExtractionSection{}
	if previous != nil {
		for _, site := range previous.Sites {
			previousSites[site.ID] = site
		}
		for _, section := range previous.Sections {
			previousSections[section.ID] = section
		}
	}

	sites := make([]extractionDocumentSite, 0, len(inventory.Sites))
	for _, site := range inventory.Sites {
		entry := extractionDocumentSite{ID: site.ID, Quote: site.Quote}
		if old, held := previousSites[site.ID]; held && old.Quote == site.Quote {
			entry.Disposition = stringPointer(old.Disposition)
			if old.Disposition == dispositionMapped {
				entry.MappedTo = old.MappedTo
			}
			if old.Disposition == dispositionExcluded {
				entry.ExcludedKind = old.ExcludedKind
				entry.Reason = old.Reason
			}
			if entry.MappedTo == "" {
				entry.MappedTo = old.MappedTo
			}
			entry.RelocatedTo = old.RelocatedTo
			entry.ReservedID = old.ReservedID
		}
		sites = append(sites, entry)
	}

	sections := make([]extractionDocumentSection, 0, len(inventory.Sections))
	for _, section := range inventory.Sections {
		entry := extractionDocumentSection{ID: section.ID, Sites: section.Sites}
		if old, held := previousSections[section.ID]; held {
			entry.Disposition = stringPointer(old.Disposition)
			if old.Disposition == dispositionSkipped {
				entry.SkipKind = old.SkipKind
				entry.Reason = old.Reason
			}
			entry.UnsourcedIDs = old.UnsourcedIDs
		}
		sections = append(sections, entry)
	}

	document := extractionDocument{
		SchemaVersion: extractionSchemaVersion,
		Stem:          inventory.Stem,
		Register:      inventory.Register,
		SourcePath:    inventory.SourcePath,
		SourceSHA:     inventory.SourceSHA,
		Sections:      sections,
		Sites:         sites,
	}
	if previous == nil {
		return document
	}
	if registerStrength[previous.Register] <= registerStrength[inventory.Register] {
		document.Register = previous.Register
	}
	document.SignedOff = previous.SignedOff
	document.Reviewer = previous.Reviewer
	document.RegisterReason = previous.RegisterReason
	document.ResignReason = previous.ResignReason
	return document
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func countUnclassifiedSites(sites []extractionDocumentSite) int {
	count := 0
	for _, site := range sites {
		if site.Disposition == nil {
			count++
		}
	}
	return count
}

func countUnclassifiedSections(sections []extractionDocumentSection) int {
	count := 0
	for _, section := range sections {
		if section.Disposition == nil {
			count++
		}
	}
	return count
}

func marshalExtractionDocument(document extractionDocument) ([]byte, error) {
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("encode extraction skeleton for %q: %w", document.Stem, err)
	}
	return []byte(escapeNonASCII(body.String())), nil
}

// escapeNonASCII reproduces json.dump's ensure_ascii default. RFC source text
// is normally ASCII, but a draft quote containing Unicode must still produce
// the same committed skeleton bytes as the legacy writer.
func escapeNonASCII(text string) string {
	var out textbuf.Buffer
	for _, char := range text {
		if char <= 0x7f {
			out.Byte(byte(char))
			continue
		}
		if char <= 0xffff {
			out.Str(fmt.Sprintf(`\u%04x`, char))
			continue
		}
		value := char - 0x10000
		high := 0xd800 + (value >> 10)
		low := 0xdc00 + (value & 0x3ff)
		out.Str(fmt.Sprintf(`\u%04x\u%04x`, high, low))
	}
	return out.String()
}

// writeExtractionDocument replaces one artifact in directory, atomically.
//
// directory is absolute, and is either rfc/extraction/ or this session's
// skeleton scratch. The staging tree is created inside it so the final rename
// never crosses a filesystem, which is what makes the replacement atomic and
// keeps a refused document from destroying the artifact it would have replaced.
func writeExtractionDocument(tree, directory, stem string, document extractionDocument) error {
	if err := os.MkdirAll(directory, 0o750); err != nil {
		var message textbuf.Buffer
		return errors.New(message.Str(relTo(tree, directory)).Str(": cannot create directory: ").Err(err).String())
	}
	sweepExtractionStaging(directory, time.Now())

	staging, err := os.MkdirTemp(directory, extractionStagingPrefix)
	if err != nil {
		var message textbuf.Buffer
		return errors.New(message.Str(relTo(tree, directory)).Str(": cannot create staging directory: ").Err(err).String())
	}
	defer os.RemoveAll(staging) //nolint:errcheck // best-effort cleanup after the atomic write or refusal

	body, err := marshalExtractionDocument(document)
	if err != nil {
		return err
	}
	staged := filepath.Join(staging, stem+".json")
	if err := os.WriteFile(staged, body, 0o644); err != nil { //nolint:gosec // authored evidence is world-readable by design
		var message textbuf.Buffer
		return errors.New(message.Str(relTo(tree, staged)).Str(": cannot write: ").Err(err).String())
	}

	path := filepath.Join(directory, stem+".json")
	if _, err := ParseExtractionArtifact(tree, staged); err != nil {
		reason := strings.ReplaceAll(err.Error(), relTo(tree, staged), relTo(tree, path))
		var message textbuf.Buffer
		return errors.New(message.Str("the skeleton derived for ").Str(stem).
			Str(" does not satisfy the artifact schema, so it was NOT written: ").Str(reason).
			Str("\nThis is a defect in the derivation, not in the source text. Nothing was changed on disk").String())
	}

	if err := os.Rename(staged, path); err != nil {
		var message textbuf.Buffer
		return errors.New(message.Str(relTo(tree, path)).Str(": cannot replace: ").Err(err).String())
	}
	return nil
}

func sweepExtractionStaging(directory string, now time.Time) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	cutoff := now.Add(-extractionStagingMaxAge)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), extractionStagingPrefix) {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		_ = os.RemoveAll(path) // best-effort hygiene must never stop the requested write
	}
}
