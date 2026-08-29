// Design: website/AI.md -- the docs producer publishes the repository's Markdown
package site

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// TestDocsDestinationPrefersAnExactMappingOverAPrefix pins the ORDER the two
// destination tables are consulted in.
//
// Both tables match architecture/config/deprecated-options.md: the exact table
// sends it to reference/deprecations, and the architecture/ prefix would send
// it to architecture/config/deprecated-options. The published site carries the
// first, so a prefix consulted first would move a live route.
func TestDocsDestinationPrefersAnExactMappingOverAPrefix(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{"architecture/config/deprecated-options.md", "reference/deprecations"},
		{"architecture/route-selection.md", "architecture/route-selection"},
		{"contributing/testing.md", "contribute/testing"},
		{"contributing/documentation-testing.md", "contribute/documentation-testing"},
		{"guide/netlab.md", "labs/netlab"},
		{"guide/anomaly.md", "guides/anomaly"},
		{"plugin-development.md", "developers/plugins"},
		{"plugin-development/commands.md", "developers/plugins/commands"},
		{"architecture.md", "architecture"},
	}
	for _, want := range cases {
		t.Run(want.source, func(t *testing.T) {
			got, err := docsDestination(want.source)
			if err != nil {
				t.Fatalf("destination: %v", err)
			}
			if got != want.want {
				t.Errorf("%s publishes to %q, want %q", want.source, got, want.want)
			}
		})
	}
}

// TestDocsDestinationRefusesAnUnmappedSource pins the absence of a default.
//
// The retired registry raised rather than falling back to docs/<stem>, so a new
// source family had to state its public URL before it could publish. A fallback
// would publish a route nobody chose, under a URL nobody reviewed.
func TestDocsDestinationRefusesAnUnmappedSource(t *testing.T) {
	for _, source := range []string{"nowhere.md", "labs/l2tp-interop.md", "rfc/short/rfc4271.md"} {
		if got, err := docsDestination(source); err == nil {
			t.Errorf("%s has no public destination and must be refused, not sent to %q", source, got)
		}
	}
}

// TestDocsManifestNamesEachSourceOnce covers the recovered manifest itself.
//
// The retired registry named architecture/config/deprecated-options.md twice
// and Python deduped it silently. Every row must also name a source that
// exists, or the producer publishes a page from nothing.
func TestDocsManifestNamesEachSourceOnce(t *testing.T) {
	seen := map[string]bool{}
	for _, row := range docsManifest {
		if seen[row.Source] {
			t.Errorf("the manifest names %s twice", row.Source)
		}
		seen[row.Source] = true
	}
	if len(docsManifest) != 115 {
		t.Errorf("the recovered manifest holds %d rows, want the 115 the retired registry named once", len(docsManifest))
	}
}

// TestEveryDocsProducerSourceExists checks the whole page set against the tree.
// A row naming a moved or deleted file is a published route with no producer,
// which is the failure this spec exists to remove.
func TestEveryDocsProducerSourceExists(t *testing.T) {
	root := repositoryRoot(t)
	pages, err := docsProducerPages()
	if err != nil {
		t.Fatalf("pages: %v", err)
	}
	if len(pages) != 148 {
		t.Errorf("the docs producer publishes %d pages, want the 148 the recovered registry names", len(pages))
	}
	destinations := map[string]string{}
	for _, page := range pages {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(page.Source))); err != nil {
			t.Errorf("%s publishes %s, which is not in the tree: %v", page.Dest, page.Source, err)
		}
		if !strings.HasSuffix(page.Dest, "/"+pageIndexFile) {
			t.Errorf("%s must publish a directory index, not %q", page.Source, page.Dest)
		}
		if other, taken := destinations[page.Dest]; taken {
			t.Errorf("%s and %s both publish %s", other, page.Source, page.Dest)
		}
		destinations[page.Dest] = page.Source
	}
}

// TestDocsProducerClaimsOnlyPublishedRoutes is the arithmetic AC-1 counts.
//
// Every route this producer answers must be a route the site already
// publishes: a new one would be a page nobody asked for, and a missing one
// leaves a published route unclaimed. The published set is read from the
// sibling gh-pages worktree, which is the artifact ./le site check reads.
func TestDocsProducerClaimsOnlyPublishedRoutes(t *testing.T) {
	published := publishedArtifactRoutes(t)
	pages, err := docsProducerPages()
	if err != nil {
		t.Fatalf("pages: %v", err)
	}
	claimed := 0
	for _, page := range pages {
		route := "/" + strings.TrimSuffix(page.Dest, pageIndexFile)
		if !slices.Contains(published, route) {
			t.Errorf("%s claims %s, which the site does not publish", page.Source, route)
			continue
		}
		claimed++
	}
	t.Logf("the docs producer claims %d of the %d published routes, leaving %d unclaimed",
		claimed, len(published), len(published)-claimed)
}

// repositoryRoot answers the checkout these tests read their sources from.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

// publishedArtifactRoutes answers every route the site published at the last
// commit the retired Python renderers made, gh-pages 2fa8fa2ad.
//
// The list is committed rather than read from the sibling worktree for two
// reasons: a worktree is not part of this repository, so a machine without one
// would run a different test, and the parity target is that COMMIT rather than
// whatever the worktree holds now. Its 712 entries are the count `./le site
// check` reported when phase 1 armed the coverage arithmetic.
func publishedArtifactRoutes(t *testing.T) []string {
	t.Helper()
	content := readTestdata(t, "published-routes.txt")
	routes := strings.Split(strings.TrimSpace(content), "\n")
	if len(routes) != 712 {
		t.Fatalf("the published route fixture holds %d routes, want 712", len(routes))
	}
	return routes
}
