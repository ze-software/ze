// Design: website/AI.md -- deployment choices lead to published user guides.
package site

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestDeploymentJourneysReachPublishedGuides follows the actual audience data
// through the homepage renderer and joins each deployment link to its guide.
func TestDeploymentJourneysReachPublishedGuides(t *testing.T) {
	paths := homeFixture(t)
	copyFixture(t, filepath.Join(repositoryRoot(t), "website", "data", audienceDataFile),
		filepath.Join(paths.Source, "data", audienceDataFile))
	if _, err := renderHome(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, homeDest)
	for _, source := range []string{
		"guide/ubuntu-build-install.md", "guide/docker.md", "guide/netlab.md", "guide/appliance.md",
	} {
		t.Run(source, func(t *testing.T) {
			published := false
			for _, row := range docsManifest {
				if row.Source == source {
					published = true
				}
			}
			if !published {
				t.Fatalf("deployment guide %s is not published", source)
			}
			destination, err := docsDestination(source)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(page, `href="`+destination+`/"`) {
				t.Errorf("homepage deployment choices do not reach %s", destination)
			}
		})
	}
}

// TestHomepageStartingCommandsUseSuppliedConfiguration verifies the visible
// command example reaches an existing tutorial rather than unexplained input.
func TestHomepageStartingCommandsUseSuppliedConfiguration(t *testing.T) {
	page, mirror := renderHomeFixture(t)
	for _, body := range []string{page, mirror} {
		if strings.Contains(body, "make</span> build") || strings.Contains(body, "make build") {
			t.Error("homepage asks for the nonexistent Makefile")
		}
		if !strings.Contains(body, "examples/quickstart/r1.conf") {
			t.Error("homepage starting commands do not name the supplied first peer config")
		}
	}
}
