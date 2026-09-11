// Design: docs/architecture/core-design.md -- le's native development gates
// Related: anchors_report.go -- structured document-owner findings

package speccitation

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/docstocode"
)

const designHeaderLines = 25

var (
	specBulletPattern = regexp.MustCompile(`^\s*-\s+\x60([^\x60]+)\x60`)
	designPattern     = regexp.MustCompile(`^//\s*Design:\s*(\S+\.md)\b`)
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
	body, err := os.ReadFile(specPath) //nolint:gosec // specPath is a spec under the checkout root, joined from it above
	if err != nil {
		return AnchorReport{}, fmt.Errorf("read spec %s: %w", spec, err)
	}
	index, err := loadDocumentIndex(root)
	if err != nil {
		return AnchorReport{}, err
	}
	// An empty index is refused rather than reported. Every mention below is
	// read out of it, so an index that answered nothing would produce a clean
	// audit that checked nothing, and a caller cannot tell that from a spec
	// with no findings (ai/rules/principles.md).
	if len(index) == 0 {
		return AnchorReport{}, fmt.Errorf(
			"no documentation under docs/ carries a source anchor, so no document edge could " +
				"be derived and this audit would pass without checking anything")
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

// loadDocumentIndex answers which documents anchor each code path.
//
// It calls the generator rather than parsing ai/CODE-TO-DOCS.md, which is that
// map RENDERED for a person. The rendering groups by package and drops a path
// into a package heading plus a basename, so the parse that read it back saw
// only the rows rendered as a table: a package of three files or fewer renders
// as bullets, and 674 of the tree's 2,537 code paths were invisible to this
// audit. A reader that reconstructs what a renderer dropped is a second
// declaration of the fact, and it disagreed with the first (ai/rules/principles.md).
func loadDocumentIndex(root string) (map[string][]string, error) {
	index, err := docstocode.DocumentsByPath(root)
	if err != nil {
		return nil, fmt.Errorf("derive the document index for %s: %w", root, err)
	}
	return index, nil
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
	slices.Sort(documents)
	return documents
}
