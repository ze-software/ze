// Design: docs/contributing/gh-pages.md -- recorded RFC graph and roadmap inputs
// Related: rfcledger.go, roadmap.go -- the existing published data this projection reads
package site

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/rfc"
	"github.com/ze-software/ze/internal/le/spec/roadmap"
	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

const rfcMapLayersFile = "data/rfc-release-layers.json"

// rfcMapData is a page-only DTO. Its zero value is not a loaded projection;
// loadRFCMapData returns one only after validating the recorded input identities.
// Proof measurements stay in the existing per-RFC files, not this model.
type rfcMapData struct {
	SchemaVersion   int                            `json:"schema-version"`
	Inputs          []rfcMapInput                  `json:"inputs"`
	Nodes           []rfcMapNode                   `json:"nodes"`
	Edges           []rfcLedgerEdge                `json:"edges"`
	Requirements    map[string][]rfcMapRequirement `json:"requirements"`
	Relations       []rfcMapRelation               `json:"relations"`
	Diagnostics     []rfcMapDiagnostic             `json:"diagnostics"`
	Layers          []rfcMapLayer                  `json:"layers"`
	Profiles        []rfcMapProfile                `json:"profiles"`
	DefaultLayer    string                         `json:"default-layer"`
	RoadmapRevision string                         `json:"roadmap-revision"`
}

type rfcMapInput struct {
	Label    string `json:"label"`
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Revision string `json:"revision,omitempty"`
}

type rfcMapNode struct {
	Stem      string   `json:"stem"`
	Title     string   `json:"title"`
	Aggregate string   `json:"aggregate"`
	Summary   bool     `json:"summary"`
	Walked    bool     `json:"walked"`
	Component []string `json:"component"`
}

type rfcMapRequirement struct {
	RID        string               `json:"rid"`
	Level      string               `json:"level"`
	Gated      bool                 `json:"gated"`
	Text       string               `json:"text"`
	Section    string               `json:"section"`
	Annotation *rfcLedgerAnnotation `json:"annotation,omitempty"`
}

type rfcMapRelation struct {
	Spec   string `json:"spec"`
	Stem   string `json:"stem"`
	RID    string `json:"rid,omitempty"`
	Kind   string `json:"kind"`
	Source string `json:"source,omitempty"`
	Note   string `json:"note,omitempty"`
}

type rfcMapDiagnostic struct {
	Source    string `json:"source"`
	Message   string `json:"message"`
	Spec      string `json:"spec,omitempty"`
	RID       string `json:"rid,omitempty"`
	Reference string `json:"reference,omitempty"`
}

type rfcMapLayer struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Stems []string `json:"stems"`
}

type rfcMapProfile struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Kind     string   `json:"kind"`
	Release  bool     `json:"release"`
	Required []string `json:"required"`
	Optional []string `json:"optional"`
}

type rfcMapLayerManifest struct {
	SchemaVersion int           `json:"schema-version"`
	Proposed      bool          `json:"proposed"`
	DefaultLayer  string        `json:"default-layer"`
	Layers        []rfcMapLayer `json:"layers"`
}

// These match authored identities and citations, never RFC requirement semantics.
var (
	rfcMapLayerID       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	rfcMapSpecReference = regexp.MustCompile(`/?(?:[[:alnum:]_.-]+/)*spec-[[:alnum:]_-]+\.md`)
)

// loadRFCMapData reads each recorded artifact once. A missing ledger revision
// or evidence date stays absent: neither HEAD nor file time can supply it.
func loadRFCMapData(paths Paths) (rfcMapData, error) {
	var ledger rfcLedger
	ledgerInput, err := readRFCMapInput(paths.Output, rfcLedgerFile, "RFC requirement ledger", &ledger)
	if err != nil {
		return rfcMapData{}, err
	}
	var snapshot roadmap.Snapshot
	roadmapInput, err := readRFCMapInput(paths.Output, roadmapDataFile, "Committed spec roadmap", &snapshot)
	if err != nil {
		return rfcMapData{}, err
	}
	if snapshot.SchemaVersion != 2 {
		return rfcMapData{}, fmt.Errorf("%s uses roadmap schema %d; supply a recorded schema-version 2 snapshot", roadmapDataFile, snapshot.SchemaVersion)
	}
	if snapshot.Revision == "" {
		return rfcMapData{}, fmt.Errorf("%s has no recorded revision; supply a committed roadmap snapshot before rendering the RFC map", roadmapDataFile)
	}
	if snapshot.Items == nil {
		return rfcMapData{}, fmt.Errorf("%s has no spec inventory; supply its recorded items before rendering the RFC map", roadmapDataFile)
	}
	roadmapInput.Revision = snapshot.Revision
	manifest, layerInput, err := loadRFCMapLayers(paths.Source)
	if err != nil {
		return rfcMapData{}, err
	}
	data := rfcMapData{
		SchemaVersion: 1,
		Inputs:        []rfcMapInput{ledgerInput, roadmapInput, layerInput},
		Nodes:         []rfcMapNode{}, Edges: []rfcLedgerEdge{},
		Requirements: map[string][]rfcMapRequirement{},
		Relations:    []rfcMapRelation{}, Diagnostics: []rfcMapDiagnostic{},
		Profiles: []rfcMapProfile{}, Layers: manifest.Layers,
		DefaultLayer: manifest.DefaultLayer, RoadmapRevision: snapshot.Revision,
	}
	known, err := projectRFCMapGraph(&data, &ledger)
	if err != nil {
		return rfcMapData{}, err
	}
	if err := validateRFCMapLayers(&manifest, known); err != nil {
		return rfcMapData{}, err
	}
	if err := projectRFCMapProfiles(&data, ledger.Profiles, known); err != nil {
		return rfcMapData{}, err
	}
	if err := projectRFCMapRelations(&data, &ledger, &snapshot, known); err != nil {
		return rfcMapData{}, err
	}
	return data, nil
}

func readRFCMapInput(root, name, label string, value any) (rfcMapInput, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name))) //nolint:gosec // the site action supplies the artifact root
	if err != nil {
		return rfcMapInput{}, fmt.Errorf("read %s for the RFC map; supply the recorded input before rendering: %w", name, err)
	}
	if err := json.Unmarshal(content, value); err != nil {
		return rfcMapInput{}, fmt.Errorf("decode %s for the RFC map; repair the recorded JSON input: %w", name, err)
	}
	return rfcMapInput{Label: label, Path: name, SHA256: fmt.Sprintf("%x", sha256.Sum256(content))}, nil
}

func loadRFCMapLayers(source string) (rfcMapLayerManifest, rfcMapInput, error) {
	var manifest rfcMapLayerManifest
	content, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(rfcMapLayersFile))) //nolint:gosec // the site action supplies the source root
	if err != nil {
		return manifest, rfcMapInput{}, fmt.Errorf("read %s; add the proposed RFC layers before rendering: %w", rfcMapLayersFile, err)
	}
	if !json.Valid(content) {
		return manifest, rfcMapInput{}, fmt.Errorf("%s is not one valid JSON document; repair the layer declaration before rendering", rfcMapLayersFile)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, rfcMapInput{}, fmt.Errorf("decode %s; repair its layer declaration: %w", rfcMapLayersFile, err)
	}
	input := rfcMapInput{Label: "Proposed release layers", Path: "website/" + rfcMapLayersFile,
		SHA256: fmt.Sprintf("%x", sha256.Sum256(content))}
	return manifest, input, nil
}

func projectRFCMapGraph(data *rfcMapData, ledger *rfcLedger) (map[string]bool, error) {
	if len(ledger.Graph.Nodes) == 0 {
		return nil, fmt.Errorf("%s has no graph nodes; supply a recorded RFC graph before rendering the map", rfcLedgerFile)
	}
	known := make(map[string]bool, len(ledger.Graph.Nodes))
	for _, row := range ledger.Graph.Nodes {
		if row == nil {
			return nil, fmt.Errorf("%s contains a null graph node; repair the recorded graph", rfcLedgerFile)
		}
		if row.Stem == "" {
			return nil, fmt.Errorf("%s contains a graph node with no stem; repair its identity", rfcLedgerFile)
		}
		if known[row.Stem] {
			return nil, fmt.Errorf("%s repeats graph node %s; keep one node per RFC identity", rfcLedgerFile, row.Stem)
		}
		known[row.Stem] = true
		data.Nodes = append(data.Nodes, rfcMapNode{Stem: row.Stem, Title: row.Title,
			Aggregate: row.Aggregate, Summary: row.Summary, Walked: row.Walked,
			Component: append([]string{}, row.Component...)})
	}
	for _, node := range data.Nodes {
		if err := validateRFCMapMembers("cycle of "+node.Stem, node.Component, known); err != nil {
			return nil, err
		}
	}
	for _, edge := range ledger.Graph.Edges {
		if !known[edge.From] {
			return nil, fmt.Errorf("%s edge starts at unknown %s; repair its graph node", rfcLedgerFile, edge.From)
		}
		if !known[edge.To] {
			return nil, fmt.Errorf("%s edge ends at unknown %s; repair its graph node", rfcLedgerFile, edge.To)
		}
		if edge.Kind == "" {
			return nil, fmt.Errorf("%s edge %s to %s has no kind; retain the recorded edge kind", rfcLedgerFile, edge.From, edge.To)
		}
		data.Edges = append(data.Edges, edge)
	}
	return known, nil
}

func validateRFCMapLayers(manifest *rfcMapLayerManifest, known map[string]bool) error {
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("%s uses schema %d; write schema-version 1", rfcMapLayersFile, manifest.SchemaVersion)
	}
	if !manifest.Proposed {
		return fmt.Errorf("%s must declare proposed true; these groups are planning proposals", rfcMapLayersFile)
	}
	if len(manifest.Layers) == 0 {
		return fmt.Errorf("%s declares no layers; add a proposed group and its default-layer", rfcMapLayersFile)
	}
	ids := map[string]bool{}
	for _, layer := range manifest.Layers {
		if !rfcMapLayerID.MatchString(layer.ID) {
			return fmt.Errorf("%s layer id %q is invalid; use a lowercase hyphenated identity", rfcMapLayersFile, layer.ID)
		}
		if ids[layer.ID] {
			return fmt.Errorf("%s repeats layer %s; keep each group identity once", rfcMapLayersFile, layer.ID)
		}
		ids[layer.ID] = true
		if strings.TrimSpace(layer.Title) == "" {
			return fmt.Errorf("%s layer %s has no title; name the proposed capability", rfcMapLayersFile, layer.ID)
		}
		if len(layer.Stems) == 0 {
			return fmt.Errorf("%s layer %s has no members; list its recorded RFC stems", rfcMapLayersFile, layer.ID)
		}
		if err := validateRFCMapMembers("layer "+layer.ID, layer.Stems, known); err != nil {
			return err
		}
	}
	if !ids[manifest.DefaultLayer] {
		return fmt.Errorf("%s default-layer %q names no group; select a declared layer id", rfcMapLayersFile, manifest.DefaultLayer)
	}
	return nil
}

func validateRFCMapMembers(owner string, members []string, known map[string]bool) error {
	seen := map[string]bool{}
	for _, stem := range members {
		if !known[stem] {
			return fmt.Errorf("RFC map %s names unknown %q; use a stem in the recorded graph", owner, stem)
		}
		if seen[stem] {
			return fmt.Errorf("RFC map %s repeats %s; retain each member once", owner, stem)
		}
		seen[stem] = true
	}
	return nil
}

func projectRFCMapProfiles(data *rfcMapData, reports []*rfc.TreeReport, known map[string]bool) error {
	ids := map[string]bool{}
	for _, report := range reports {
		if report == nil {
			return fmt.Errorf("%s contains a null profile; repair its recorded profile list", rfcLedgerFile)
		}
		if report.Profile == "" {
			return fmt.Errorf("%s contains a profile without an identity; restore its profile name", rfcLedgerFile)
		}
		if ids[report.Profile] {
			return fmt.Errorf("%s repeats profile %s; retain each profile once", rfcLedgerFile, report.Profile)
		}
		ids[report.Profile] = true
		profile := rfcMapProfile{ID: report.Profile, Title: report.Title, Kind: report.Kind,
			Release: report.Release, Required: []string{}, Optional: []string{}}
		for _, row := range report.Rows {
			if row == nil {
				return fmt.Errorf("RFC map profile %s contains a null required member; repair its recorded rows", report.Profile)
			}
			profile.Required = append(profile.Required, row.Stem)
		}
		if report.Optional != nil {
			for _, row := range report.Optional.Rows {
				if row == nil {
					return fmt.Errorf("RFC map profile %s contains a null optional member; repair its recorded rows", report.Profile)
				}
				profile.Optional = append(profile.Optional, row.Stem)
			}
		}
		members := append(slices.Clone(profile.Required), profile.Optional...)
		if err := validateRFCMapMembers("profile "+profile.ID, members, known); err != nil {
			return err
		}
		data.Profiles = append(data.Profiles, profile)
	}
	return nil
}

func projectRFCMapRelations(data *rfcMapData, ledger *rfcLedger, snapshot *roadmap.Snapshot, known map[string]bool) error {
	requirementStems := map[string]string{}
	summaryStems := map[string]bool{}
	for index := range ledger.Stems {
		stem := &ledger.Stems[index]
		if !known[stem.Stem] {
			return fmt.Errorf("%s summary %s has no graph node; supply the complete recorded graph", rfcLedgerFile, stem.Stem)
		}
		if summaryStems[stem.Stem] {
			return fmt.Errorf("%s repeats summary %s; retain one summary per identity", rfcLedgerFile, stem.Stem)
		}
		summaryStems[stem.Stem] = true
		data.Requirements[stem.Stem] = []rfcMapRequirement{}
		for index := range stem.Requirements {
			requirement := &stem.Requirements[index]
			if requirement.RID == "" {
				return fmt.Errorf("%s summary %s has a statement without an RID; repair its recorded requirement", rfcLedgerFile, stem.Stem)
			}
			if _, exists := requirementStems[requirement.RID]; exists {
				return fmt.Errorf("%s repeats RID %s; retain one statement per identity", rfcLedgerFile, requirement.RID)
			}
			requirementStems[requirement.RID] = stem.Stem
			data.Requirements[stem.Stem] = append(data.Requirements[stem.Stem], rfcMapRequirement{
				RID: requirement.RID, Level: requirement.Level, Gated: requirement.Gated,
				Text: requirement.Subject(), Section: requirement.Section, Annotation: requirement.Annotation})
		}
	}
	basenames := map[string][]string{}
	names := map[string]bool{}
	seen := map[rfcMapRelation]bool{}
	for index := range snapshot.Items {
		item := &snapshot.Items[index]
		if item.Name == "" {
			return fmt.Errorf("%s contains an unnamed spec; restore its recorded identity", roadmapDataFile)
		}
		if names[item.Name] {
			return fmt.Errorf("%s repeats spec %s; retain one item per identity", roadmapDataFile, item.Name)
		}
		names[item.Name] = true
		basenames[path.Base(item.Path)] = append(basenames[path.Base(item.Path)], item.Name)
		for _, message := range item.Diagnostics {
			data.Diagnostics = append(data.Diagnostics, rfcMapDiagnostic{Source: item.Path, Spec: item.Name, Message: message})
		}
		for _, token := range item.RFCGaps {
			stems, unknown := specstatus.ResolveRFCGaps([]string{token}, requirementStems, summaryStems)
			if len(unknown) > 0 {
				data.Diagnostics = append(data.Diagnostics, rfcMapDiagnostic{Source: item.Path,
					Spec: item.Name, Reference: token, Message: "RFC gaps token names no recorded RID or summary; review the spec header against the recorded RFC ledger."})
				continue
			}
			relation := rfcMapRelation{Spec: item.Name, Stem: stems[0], Kind: "declared-rfc", Source: item.Path}
			if _, exact := requirementStems[token]; exact {
				relation.RID = token
				relation.Kind = "declared-requirement"
			}
			appendRFCMapRelation(data, relation, seen)
		}
	}
	for index := range ledger.Stems {
		stem := &ledger.Stems[index]
		for index := range stem.Requirements {
			requirement := &stem.Requirements[index]
			if requirement.Annotation == nil {
				continue
			}
			var kind string
			switch requirement.Annotation.Kind {
			case rfc.AnnotationGap:
				kind = "summary-gap-citation"
			case rfc.AnnotationPartial:
				kind = "summary-partial-citation"
			default:
				continue
			}
			for _, reference := range rfcMapSpecReference.FindAllString(requirement.Annotation.Reason, -1) {
				matches := basenames[path.Base(reference)]
				if len(matches) != 1 {
					data.Diagnostics = append(data.Diagnostics, rfcMapDiagnostic{Source: stem.SummaryPath,
						RID: requirement.RID, Reference: reference,
						Message: fmt.Sprintf("Requirement citation resolves to %d recorded specs; review its historical filename against the roadmap before assigning ownership.", len(matches))})
					continue
				}
				appendRFCMapRelation(data, rfcMapRelation{Spec: matches[0], Stem: stem.Stem,
					RID: requirement.RID, Kind: kind, Source: stem.SummaryPath,
					Note: "Named as " + reference + "; this citation does not establish sole implementation ownership."}, seen)
			}
		}
	}
	return nil
}

func appendRFCMapRelation(data *rfcMapData, relation rfcMapRelation, seen map[rfcMapRelation]bool) {
	if seen[relation] {
		return
	}
	seen[relation] = true
	data.Relations = append(data.Relations, relation)
}
