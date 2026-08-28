// Design: docs/architecture/core-design.md -- le's native development gates
// Related: anchors_report.go -- structured document-owner findings

package speccitation

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	documentIndexPath = "ai/CODE-TO-DOCS.md"
	designHeaderLines = 25
)

var (
	indexSectionPattern = regexp.MustCompile(`^##\s+\x60([^\x60]+)\x60\s*$`)
	indexRowPattern     = regexp.MustCompile(`^\|\s*\x60([^\x60]*)\x60\s*\|\s*(.+?)\s*\|\s*$`)
	indexDocPattern     = regexp.MustCompile(`\x60([^\x60]+\.md)\x60`)
	specBulletPattern   = regexp.MustCompile(`^\s*-\s+\x60([^\x60]+)\x60`)
	designPattern       = regexp.MustCompile(`^//\s*Design:\s*(\S+\.md)\b`)
)

var sourcePrefixes = [...]string{
	"internal/", "cmd/", "pkg/", "test/", "rfc/", "tools/", "demos/",
	"website/", "gokrazy/", "contrib/", "etc/", "examples/", ".github/",
}
var sourceSuffixes = [...]string{
	".go", ".ci", ".et", ".wb", ".yang", ".templ", ".json", ".yaml", ".yml", ".toml",
}

// AuditAnchors reads one spec and reports design documents that its named source
// files declare or mention. A declared document changes the verdict. A mention
// is advisory.
func AuditAnchors(root, spec string) (AnchorReport, error) {
	specPath := filepath.FromSlash(spec)
	if !filepath.IsAbs(specPath) {
		specPath = filepath.Join(root, specPath)
	}
	body, err := os.ReadFile(specPath)
	if err != nil {
		return AnchorReport{}, fmt.Errorf("read spec %s: %w", spec, err)
	}
	index, err := loadDocumentIndex(root)
	if err != nil {
		return AnchorReport{}, err
	}
	if len(index) == 0 {
		return AnchorReport{}, fmt.Errorf("%s is absent or empty, so no anchor could be derived. Run: ./le docs-to-code index-update", documentIndexPath)
	}

	text := string(body)
	files := specSourceFiles(text)
	owners := make(map[string][]string)
	mentions := make(map[string][]string)
	for _, source := range files {
		owner := declaredDesignDocument(root, source)
		if owner != "" {
			if !strings.Contains(text, owner) {
				owners[owner] = append(owners[owner], source)
			}
		}
		for _, document := range index[source] {
			if document == owner {
				continue
			}
			if strings.Contains(text, document) {
				continue
			}
			mentions[document] = append(mentions[document], source)
		}
	}
	return anchorReport(spec, files, owners, mentions), nil
}

func declaredDesignDocument(root, relative string) string {
	file, err := os.Open(filepath.Join(root, filepath.FromSlash(relative))) //nolint:gosec // relative came from the spec under root
	if err != nil {
		return ""
	}
	defer file.Close() //nolint:errcheck // read-only

	scanner := bufio.NewScanner(file)
	for range designHeaderLines {
		if !scanner.Scan() {
			break
		}
		match := designPattern.FindStringSubmatch(scanner.Text())
		if match != nil {
			return match[1]
		}
	}
	return ""
}

func loadDocumentIndex(root string) (map[string][]string, error) {
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(documentIndexPath)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string][]string{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", documentIndexPath, err)
	}

	mapping := make(map[string][]string)
	section := ""
	for line := range strings.SplitSeq(string(body), "\n") {
		if match := indexSectionPattern.FindStringSubmatch(line); match != nil {
			section = match[1]
			continue
		}
		match := indexRowPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if section == "" {
			continue
		}
		name := match[1]
		if name == "" {
			continue
		}
		if name == "File" {
			continue
		}
		documents := indexDocPattern.FindAllStringSubmatch(match[2], -1)
		if len(documents) == 0 {
			continue
		}
		path := strings.TrimSuffix(section, "/") + "/" + name
		for _, document := range documents {
			mapping[path] = append(mapping[path], document[1])
		}
	}
	return mapping, nil
}

func specSourceFiles(text string) []string {
	var files []string
	inFiles := false
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			inFiles = isSpecFilesHeading(line)
			continue
		}
		if strings.HasPrefix(line, "### ") {
			inFiles = false
			continue
		}
		if !inFiles {
			continue
		}
		match := specBulletPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if !isSourcePath(match[1]) {
			continue
		}
		files = append(files, match[1])
	}
	return files
}

func isSpecFilesHeading(line string) bool {
	if strings.HasPrefix(line, "## Files to Modify") {
		return true
	}
	return strings.HasPrefix(line, "## Files to Create")
}

func isSourcePath(path string) bool {
	prefix := false
	for _, candidate := range sourcePrefixes {
		if strings.HasPrefix(path, candidate) {
			prefix = true
			break
		}
	}
	if !prefix {
		return false
	}
	for _, suffix := range sourceSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func anchorReport(spec string, files []string, owners, mentions map[string][]string) AnchorReport {
	report := AnchorReport{Spec: spec, Files: files}
	for _, document := range sortedDocuments(owners) {
		report.Owners = append(report.Owners, AnchorFinding{Document: document, Sources: owners[document]})
	}
	for _, document := range sortedDocuments(mentions) {
		report.Mentions = append(report.Mentions, AnchorFinding{Document: document, Sources: mentions[document]})
	}
	return report
}

func sortedDocuments(found map[string][]string) []string {
	documents := make([]string, 0, len(found))
	for document := range found {
		documents = append(documents, document)
	}
	sort.Strings(documents)
	return documents
}
