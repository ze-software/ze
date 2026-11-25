// Design: docs/architecture/core-design.md -- tests for the generated data-plane feature reference
// Related: dataplane_page.go -- RenderDataPlanePage and checkDataPlaneFeatureReference

package docyangcontract

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// VALIDATES: AC-12 -- RenderDataPlanePage renders every declared DataPlaneSelector,
// ProgramsFIB writer, DataPlaneFeature (registered and standalone), and YANG
// ze:backend annotation deterministically, links the hand-written verifier guides,
// and the drift gate refuses missing or stale copies while accepting what
// writeGenerated produces.
// PREVENTS: docs/guide/dataplane-features.md drifting from live plugin
// declarations or schema annotations, or missing newly declared features.
func TestDataPlanePageMatchesDeclarations(t *testing.T) {
	page, err := RenderDataPlanePage()
	if err != nil {
		t.Fatalf("RenderDataPlanePage: %v", err)
	}
	second, err := RenderDataPlanePage()
	if err != nil {
		t.Fatalf("second RenderDataPlanePage: %v", err)
	}
	if page != second {
		t.Fatal("RenderDataPlanePage output is not deterministic across consecutive calls")
	}
	if !strings.HasPrefix(page, "<!-- GENERATED ") {
		t.Error("rendered page must start with a GENERATED banner")
	}
	if !strings.Contains(page, "# Features by Data Plane: Linux Kernel and VPP\n") {
		t.Error("rendered page is missing its top-level heading")
	}

	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("config.YANGSchema: %v", err)
	}
	regs := registry.All()

	selectorCount := 0
	fibCount := 0
	featureCount := 0
	for _, reg := range regs {
		if reg.DataPlaneSelector != nil {
			selectorCount++
			displayLeaf := displayConfigPath(reg.DataPlaneSelector.LeafPath)
			if !strings.Contains(page, codeSpan(displayLeaf)) {
				t.Errorf("rendered page omits declared selector %q", displayLeaf)
			}
			defVal, defErr := lookupSelectorDefault(schema, reg.DataPlaneSelector.LeafPath)
			if defErr != nil {
				t.Fatalf("lookupSelectorDefault(%q): %v", reg.DataPlaneSelector.LeafPath, defErr)
			}
			if !strings.Contains(page, codeSpan(defVal)) {
				t.Errorf("rendered page omits schema default %q for selector %q", defVal, displayLeaf)
			}
			for _, backend := range reg.DataPlaneSelector.RegisteredBackends() {
				if !strings.Contains(page, codeSpan(backend)) {
					t.Errorf("rendered page omits registered backend %q of selector %q", backend, displayLeaf)
				}
			}
		}

		if reg.ProgramsFIB {
			fibCount++
			if !strings.Contains(page, codeSpan(reg.Name)) {
				t.Errorf("rendered page omits FIB writer %q", reg.Name)
			}
			for _, root := range reg.ConfigRoots {
				displayRoot := displayConfigPath(root)
				if !strings.Contains(page, codeSpan(displayRoot)) {
					t.Errorf("rendered page omits FIB writer %q config root %q", reg.Name, displayRoot)
				}
			}
		}

		for _, feat := range reg.DataPlaneFeatures {
			featureCount++
			owner := feat.Name
			if owner == "" {
				owner = reg.Name
			}
			assertFeatureMentioned(t, page, owner, feat)
		}
	}

	standalone := registry.StandaloneDataPlaneFeatures()
	for _, feat := range standalone {
		featureCount++
		assertFeatureMentioned(t, page, feat.Name, feat)
	}

	if selectorCount == 0 {
		t.Fatal("composition root declared no DataPlaneSelector")
	}
	if fibCount == 0 {
		t.Fatal("composition root declared no ProgramsFIB plugin")
	}
	if featureCount == 0 {
		t.Fatal("composition root declared no DataPlaneFeature")
	}

	annotationCount := 0
	walkSchemaBackendNodes(schema, "", func(nodePath string, backends []string) {
		annotationCount++
		fullPath := "/" + nodePath
		if !strings.Contains(page, codeSpan(fullPath)) {
			t.Errorf("rendered page omits ze:backend annotated path %q", fullPath)
		}
		for _, backend := range backends {
			if !strings.Contains(page, codeSpan(backend)) {
				t.Errorf("rendered page omits backend %q for annotated path %q", backend, fullPath)
			}
		}
	})
	if annotationCount == 0 {
		t.Fatal("loaded YANG schema carried no ze:backend annotations")
	}

	for _, guideLink := range []string{
		"(firewall.md#vpp-backend-compatibility-and-limits)",
		"(traffic-control.md#vpp-backend-compatibility-matrix)",
		"(vpp.md)",
	} {
		if !strings.Contains(page, guideLink) {
			t.Errorf("rendered page is missing link %s", guideLink)
		}
	}

	t.Run("fixture root without docs/guide is not flagged", func(t *testing.T) {
		bare := t.TempDir()
		writeDoc(t, bare, "docs/architecture/api/text-parser.md", "# Text Parser\n\nUses textparse.NewScanner.\n")
		if report := Drift(t.Context(), bare); len(report.Issues) != 0 {
			t.Fatalf("bare fixture root drew findings: %v", messages(report))
		}
	})

	t.Run("missing and stale copies are refused, and writeGenerated clears drift", func(t *testing.T) {
		root := t.TempDir()
		writeDoc(t, root, "docs/guide/vpp.md", "# VPP\n")

		if report := Drift(t.Context(), root); !reports(report, "the generated data-plane feature reference is missing") {
			t.Fatalf("missing data-plane page was not reported: %v", messages(report))
		}

		writeDoc(t, root, dataPlaneFeatureReferencePath, "# Stale hand-written copy\n")
		if report := Drift(t.Context(), root); !reports(report, "the published data-plane feature reference and the live declarations disagree") {
			t.Fatalf("stale data-plane page was not reported: %v", messages(report))
		}

		written, writeErr := writeGenerated(root)
		if writeErr != nil {
			t.Fatalf("writeGenerated: %v", writeErr)
		}
		if !slices.Contains(written.Paths, dataPlaneFeatureReferencePath) {
			t.Fatalf("writeGenerated.Paths %v omits %q", written.Paths, dataPlaneFeatureReferencePath)
		}
		if !strings.Contains(written.Text(), "wrote "+dataPlaneFeatureReferencePath+"\n") {
			t.Errorf("WriteReport.Text() %q omits data-plane path", written.Text())
		}
		diskBytes, readErr := os.ReadFile(filepath.Join(root, dataPlaneFeatureReferencePath))
		if readErr != nil {
			t.Fatalf("read written data-plane page: %v", readErr)
		}
		if string(diskBytes) != page {
			t.Error("written file bytes differ from RenderDataPlanePage()")
		}
		if report := Drift(t.Context(), root); len(report.Issues) != 0 {
			t.Fatalf("freshly generated references still reported drift: %v", messages(report))
		}
	})
}

// assertFeatureMentioned verifies that page names feat's owner, config path,
// explanation, and supported planes.
func assertFeatureMentioned(t *testing.T, page, owner string, feat registry.DataPlaneFeature) {
	t.Helper()
	if !strings.Contains(page, codeSpan(owner)) {
		t.Errorf("rendered page omits feature owner %q", owner)
	}
	displayPath := displayConfigPath(feat.ConfigPath)
	if !strings.Contains(page, codeSpan(displayPath)) {
		t.Errorf("rendered page omits feature config path %q (owner %q)", displayPath, owner)
	}
	if feat.Why != "" && !strings.Contains(page, escapeTableCell(feat.Why)) {
		t.Errorf("rendered page omits feature explanation %q (owner %q)", feat.Why, owner)
	}
	for _, plane := range feat.Planes {
		if !strings.Contains(page, codeSpan(plane)) {
			t.Errorf("rendered page omits plane %q for feature %q", plane, owner)
		}
	}
}
