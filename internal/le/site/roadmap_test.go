// Design: website/AI.md -- release pages publish one committed inventory on every build.
package site

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/spec/roadmap"
)

// roadmapGit runs Git against an isolated fixture, never the developer checkout.
func roadmapGit(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Roadmap Test", "GIT_AUTHOR_EMAIL=roadmap@example.test",
		"GIT_COMMITTER_NAME=Roadmap Test", "GIT_COMMITTER_EMAIL=roadmap@example.test")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}

// commitSiteFixture gives build fixtures the immutable plan tree a build needs.
func commitSiteFixture(t *testing.T, root string) {
	t.Helper()
	writeFixtureFile(t, filepath.Join(root, "plan", "README.md"), "# Plan\n")
	roadmapGit(t, root, "add", ".")
	roadmapGit(t, root, "-c", "commit.gpgsign=false", "commit", "-qm", "Site fixture")
}

// roadmapBuildPaths keeps the registered roadmap producer and derived search
// producer while replacing unrelated page families and live daemon inputs.
func roadmapBuildPaths(t *testing.T) (string, string) {
	t.Helper()
	var producer Producer
	for _, registered := range registeredProducers {
		if registered.Name == "roadmap" {
			producer = registered
		}
	}
	if producer.Render == nil {
		t.Fatal("the roadmap producer is not registered")
	}
	stubLiveInputs(t, `[{"path":"show test","short-help":"Show rows","mode":"read-only"}]`)
	stubProducers(t, producer)
	registerDerivedProducer(Producer{Name: "search", Render: renderSearch})
	root, output := siteFixture(t)
	writeFixtureFile(t, filepath.Join(root, "website", "data", "page-links.json"), `{}`)
	return root, output
}

// TestRoadmapBuildTracksPlanChanges exercises full and partial Build calls with
// only committed specs changed. Dirty and untracked specs must stay unpublished.
func TestRoadmapBuildTracksPlanChanges(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "full"
		if partial {
			name = "partial"
		}
		t.Run(name, func(t *testing.T) {
			root, output := roadmapBuildPaths(t)
			writeFixtureFile(t, filepath.Join(root, "plan", "immediate", "spec-router.md"),
				"# Spec: Router obligation\n\n| Field | Value |\n|---|---|\n| Status | blocked |\n| Updated | 2026-09-18 |\n")
			writeFixtureFile(t, filepath.Join(root, "plan", "spec-optional.md"),
				"# Spec: Optional capability\n\n| Field | Value |\n|---|---|\n| Status | deferred |\n")
			roadmapGit(t, root, "add", "plan")
			roadmapGit(t, root, "-c", "commit.gpgsign=false", "commit", "-qm", "First obligation")
			first, err := Build(BuildOptions{Repository: root, Output: output})
			if err != nil {
				t.Fatal(err)
			}
			before := readArtifact(t, output, roadmapDataFile)
			writeFixtureFile(t, filepath.Join(root, "plan", "pre-release", "spec-new.md"),
				"# Spec: New release obligation\n\n| Field | Value |\n|---|---|\n| Status | verification |\n")
			roadmapGit(t, root, "add", "plan/pre-release/spec-new.md")
			roadmapGit(t, root, "-c", "commit.gpgsign=false", "commit", "-qm", "Add release obligation")
			revision := roadmapGit(t, root, "rev-parse", "HEAD")
			writeFixtureFile(t, filepath.Join(root, "plan", "pre-release", "spec-new.md"), "# Spec: Dirty replacement\n")
			writeFixtureFile(t, filepath.Join(root, "plan", "spec-untracked.md"), "# Spec: Untracked obligation\n")
			writeFixtureFile(t, filepath.Join(root, "plan", "roadmap.md"), "# Stale generated inventory\n")
			second, err := Build(BuildOptions{Repository: root, Output: output, Partial: partial})
			if err != nil {
				t.Fatal(err)
			}
			if first.SourceDigest != second.SourceDigest {
				t.Fatal("a plan-only change altered the website source digest")
			}
			after := readArtifact(t, output, roadmapDataFile)
			if before == after {
				t.Fatal("a committed spec-only change left roadmap data unchanged")
			}
			snapshot, err := roadmap.Collect(context.Background(), root, revision)
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Counts.Required != 2 {
				t.Fatalf("required count = %d, want both required buckets", snapshot.Counts.Required)
			}
			if snapshot.Counts.NiceToHave != 1 {
				t.Fatalf("nice-to-have count = %d, want the optional item", snapshot.Counts.NiceToHave)
			}
			want, err := json.MarshalIndent(snapshot, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(after) != string(want) {
				t.Fatal("published JSON differs from the selected committed inventory")
			}
			for _, name := range []string{roadmapDestination, "project/roadmap/index.md", searchIndexFile} {
				content := readArtifact(t, output, name)
				if !strings.Contains(content, "New release obligation") {
					t.Errorf("%s lacks the committed obligation", name)
				}
				if !strings.Contains(content, "Optional capability") {
					t.Errorf("%s lost the nice-to-have item", name)
				}
				for _, excluded := range []string{"Dirty replacement", "Untracked obligation", "Stale generated inventory"} {
					if strings.Contains(content, excluded) {
						t.Errorf("%s publishes %q from the working tree", name, excluded)
					}
				}
			}
			mirror := readArtifact(t, output, "project/roadmap/index.md")
			if !strings.Contains(mirror, roadmap.Markdown(&snapshot, false)) {
				t.Fatal("the mirror differs from the snapshot's Markdown")
			}
			if !strings.Contains(mirror, "/blob/"+revision+"/plan/pre-release/spec-new.md") {
				t.Fatal("the source link does not pin the selected commit")
			}
			claims, err := readProducerRecord(Paths{Repository: root, Output: output})
			if err != nil {
				t.Fatal(err)
			}
			owner := slices.IndexFunc(claims, func(claim Claim) bool {
				return claim.Route == "/project/roadmap/"
			})
			if owner < 0 {
				t.Fatalf("roadmap route has no recorded owner: %v", claims)
			}
			if !slices.Equal(claims[owner].Producers, []string{"roadmap"}) {
				t.Fatalf("roadmap route has owners %v", claims[owner].Producers)
			}
			pages, err := docsProducerPages()
			if err != nil {
				t.Fatal(err)
			}
			for _, page := range pages {
				if page.Dest == roadmapDestination {
					t.Fatal("the docs producer still owns the roadmap route")
				}
			}
			if _, err := Build(BuildOptions{Repository: root, Output: output, Partial: true}); err != nil {
				t.Fatal(err)
			}
			if readArtifact(t, output, roadmapDataFile) != after {
				t.Fatal("identical committed input changed published JSON")
			}
			if readArtifact(t, output, "project/roadmap/index.md") != mirror {
				t.Fatal("identical committed input changed the Markdown mirror")
			}
		})
	}
}

// TestRoadmapBuildRefusesMissingPlan proves a failed discovery cannot publish
// an empty backlog, even when the working tree has an uncommitted plan file.
func TestRoadmapBuildRefusesMissingPlan(t *testing.T) {
	root, output := roadmapBuildPaths(t)
	roadmapGit(t, root, "rm", "-r", "plan")
	roadmapGit(t, root, "-c", "commit.gpgsign=false", "commit", "-qm", "Remove plan")
	writeFixtureFile(t, filepath.Join(root, "plan", "spec-local.md"), "# Spec: Local only\n")
	_, err := Build(BuildOptions{Repository: root, Output: output})
	if err == nil {
		t.Fatal("a committed tree without plan published an empty roadmap")
	}
	if !strings.Contains(err.Error(), "plan") {
		t.Fatalf("missing-plan error names no plan: %v", err)
	}
}

// TestRoadmapMetadataIsLiteralAndDeterministic renders hostile metadata through
// the production Markdown pipeline. Unknown states remain visible without
// creating links, markup, or live-count substitutions.
func TestRoadmapMetadataIsLiteralAndDeterministic(t *testing.T) {
	root, output := roadmapBuildPaths(t)
	title := `Literal <script>alert(1)</script> [link](https://invalid.test/) {{ze:unknown-count}}`
	writeFixtureFile(t, filepath.Join(root, "plan", "immediate", "spec-hostile.md"),
		"# Spec: "+title+"\n\n| Field | Value |\n|---|---|\n| Status | peculiar-state |\n"+
			"| Depends | <img src=x onerror=alert(1)> |\n")
	roadmapGit(t, root, "add", "plan/immediate/spec-hostile.md")
	roadmapGit(t, root, "-c", "commit.gpgsign=false", "commit", "-qm", "Literal metadata")
	paths := Paths{Repository: root, Source: filepath.Join(root, "website"), Output: output}
	if _, err := renderRoadmap(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, output, roadmapDestination)
	for _, injected := range []string{`<script>alert(1)</script>`, `<img src=x`, `href="https://invalid.test/"`} {
		if strings.Contains(page, injected) {
			t.Errorf("metadata created active markup: %s", injected)
		}
	}
	for _, literal := range []string{"peculiar-state", "{{ze:unknown-count}}", "<script>alert(1)</script>"} {
		if !strings.Contains(visibleText(mainContent(t, page)), literal) {
			t.Errorf("the page lost literal metadata %q", literal)
		}
	}
	outputs := []string{roadmapDestination, "project/roadmap/index.md", roadmapDataFile}
	before := make([]string, len(outputs))
	for index, name := range outputs {
		before[index] = readArtifact(t, output, name)
	}
	if _, err := renderRoadmap(paths); err != nil {
		t.Fatal(err)
	}
	for index, name := range outputs {
		if readArtifact(t, output, name) != before[index] {
			t.Errorf("identical revision changed %s", name)
		}
	}
}
