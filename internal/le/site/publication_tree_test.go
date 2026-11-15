// Design: docs/contributing/gh-pages.md -- commit identity is raw regular-file content.
package site

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestSitePublicationTreeModes permits ordinary and executable regular files,
// but never treats a symlink target or gitlink as publishable file content.
func TestSitePublicationTreeModes(t *testing.T) {
	for _, scenario := range []string{"regular", "executable", "symlink", "gitlink"} {
		t.Run(scenario, func(t *testing.T) {
			paths, source := publicationFixture(t)
			receipt := publicationRecordFixture(t, paths, source)
			filename := filepath.Join(paths.Output, "assets", "site.css")
			switch scenario {
			case "executable":
				if err := os.Chmod(filename, 0o755); err != nil { //nolint:gosec // executable regular-file mode is the fixture input
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Remove(filename); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("site.js", filename); err != nil {
					t.Fatal(err)
				}
			}
			roadmapGit(t, paths.Output, "add", "-f", ".")
			if scenario == "gitlink" {
				roadmapGit(t, paths.Output, "update-index", "--add", "--cacheinfo", "160000,"+source+",linked")
			}
			roadmapGit(t, paths.Output, "-c", "commit.gpgsign=false", "commit", "-qm", "Artifact mode fixture")
			revision := roadmapGit(t, paths.Output, "rev-parse", "HEAD")
			files, err := publicationCommittedFiles(t.Context(), paths.Output, revision)
			if scenario == "symlink" || scenario == "gitlink" {
				if err == nil || !strings.Contains(err.Error(), "nonregular mode") {
					t.Fatalf("mode %s was not refused: %v", scenario, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(files, receipt.Files) {
				t.Errorf("regular mode %s changed the committed byte identity", scenario)
			}
			if err := publicationCommit(t.Context(), paths, receipt, revision); err != nil {
				t.Errorf("regular mode %s could not bind the receipt: %v", scenario, err)
			}
		})
	}
}

// TestSitePublicationTreeIgnoresReplacementObjects proves local replacement
// refs cannot redefine the bytes identified by an otherwise valid commit SHA.
func TestSitePublicationTreeIgnoresReplacementObjects(t *testing.T) {
	paths, source := publicationFixture(t)
	receipt := publicationRecordFixture(t, paths, source)
	roadmapGit(t, paths.Output, "add", "-f", ".")
	roadmapGit(t, paths.Output, "-c", "commit.gpgsign=false", "commit", "-qm", "Original reviewed artifact")
	original := roadmapGit(t, paths.Output, "rev-parse", "HEAD")
	writeFixtureFile(t, filepath.Join(paths.Output, "assets", "site.css"), "different committed bytes\n")
	roadmapGit(t, paths.Output, "add", "assets/site.css")
	roadmapGit(t, paths.Output, "-c", "commit.gpgsign=false", "commit", "-qm", "Replacement artifact")
	replacement := roadmapGit(t, paths.Output, "rev-parse", "HEAD")
	roadmapGit(t, paths.Output, "replace", original, replacement)
	files, err := publicationCommittedFiles(t.Context(), paths.Output, original)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(files, receipt.Files) {
		t.Error("replacement ref changed the original commit's raw byte identity")
	}
}
