// Design: docs/architecture/core-design.md -- generated data-plane feature reference
// Overview: drift.go -- the drift gate and writer that call this file
// Related: dataplane_page_test.go -- tests proving live declarations match the page
//
// dataplane_page.go renders docs/guide/dataplane-features.md directly from the
// live plugin registry declarations (DataPlaneSelector, ProgramsFIB,
// DataPlaneFeatures, StandaloneDataPlaneFeatures, HostSideDataPlane) and the
// loaded YANG schema's ze:backend annotations. The drift gate compares the
// published file against RenderDataPlanePage() byte for byte so the page cannot
// drift from the product.

package docyangcontract

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// dataPlaneFeatureReferencePath is the generated operator reference page
// listing every declared data-plane selector, FIB writer, feature declaration,
// and YANG ze:backend gate.
const dataPlaneFeatureReferencePath = "docs/guide/dataplane-features.md"

const (
	dataPlaneUpdateHint      = "run `./le doc yang-contract update`"
	dataPlaneTableSeparator  = " | "
	dataPlaneRowPrefix       = "| "
	dataPlaneRowSuffix       = " |\n"
	dataPlaneBacktick        = "`"
	dataPlaneBacktickJoinSep = "`, `"
)

var (
	errNoPrimaryDataPlaneSelector = errors.New("no primary DataPlaneSelector (SelectsDataPlane == true) is registered")
	errNoBackendAnnotationsFound  = errors.New("loaded YANG schema carries no ze:backend annotations")
)

// declaredSelectorRow holds one registered DataPlaneSelector together with its
// schema default and sorted registered backend names.
type declaredSelectorRow struct {
	owner      string
	leafPath   string
	display    string
	roots      []string
	primary    bool
	defaultVal string
	backends   []string
}

// declaredFIBRow holds one registered FIB writer (ProgramsFIB == true).
type declaredFIBRow struct {
	name          string
	roots         []string
	dataPlane     string
	hostSide      bool
	noPlaneChoice bool
}

// declaredFeatureRow holds one DataPlaneFeature collected from either a plugin
// Registration or StandaloneDataPlaneFeatures.
type declaredFeatureRow struct {
	owner           string
	configPath      string
	why             string
	planes          []string
	followsSelector string
	inUseKind       registry.DataPlaneInUseKind
	hostSide        bool
}

// backendAnnotationRow holds one schema node carrying a ze:backend annotation.
type backendAnnotationRow struct {
	path     string
	selector string
	backends []string
}

// RenderDataPlanePage renders docs/guide/dataplane-features.md from the live
// plugin registry and loaded YANG schema. Output is deterministic so exact byte
// comparison detects any drift.
func RenderDataPlanePage() (string, error) {
	schema, err := config.YANGSchema()
	if err != nil {
		return "", fmt.Errorf("load YANG schema: %w", err)
	}

	regs := registry.All()
	primary, selectors, selectorByOwner, err := collectDataPlaneSelectors(regs, schema)
	if err != nil {
		return "", err
	}
	fibWriters := collectFIBWriters(regs)
	followingFeatures, fixedFeatures := collectDataPlaneFeatures(regs, registry.StandaloneDataPlaneFeatures())
	annotations := collectBackendAnnotations(schema, primary, selectors)
	if len(annotations) == 0 {
		return "", errNoBackendAnnotationsFound
	}

	var tb textbuf.Buffer
	writeDataPlaneHeader(&tb, primary)
	writeSelectorsSection(&tb, selectors)
	writeFIBSection(&tb, primary, fibWriters)
	writeFollowingFeaturesSection(&tb, primary, followingFeatures, selectorByOwner)
	writeFixedFeaturesSection(&tb, primary, fixedFeatures)
	writeBackendAnnotationsSection(&tb, annotations)
	writeVerifierLimitsSection(&tb)
	return tb.String(), nil
}

// collectDataPlaneSelectors reads every registered DataPlaneSelector and its
// schema default from schema, returning the primary selector, all selectors
// (primary first, then sorted by leaf path), and a lookup by owner plugin name.
func collectDataPlaneSelectors(
	regs []*registry.Registration,
	schema *config.Schema,
) (declaredSelectorRow, []declaredSelectorRow, map[string]declaredSelectorRow, error) {
	var primary declaredSelectorRow
	foundPrimary := false
	var others []declaredSelectorRow

	for _, reg := range regs {
		if reg.DataPlaneSelector == nil {
			continue
		}
		sel := reg.DataPlaneSelector
		defVal, err := lookupSelectorDefault(schema, sel.LeafPath)
		if err != nil {
			return declaredSelectorRow{}, nil, nil, fmt.Errorf("selector %s (%s): %w", reg.Name, sel.LeafPath, err)
		}
		roots := make([]string, 0, len(sel.Roots))
		for _, root := range sel.Roots {
			roots = append(roots, displayConfigPath(root))
		}
		slices.Sort(roots)

		backends := sortedUnique(sel.RegisteredBackends())
		row := declaredSelectorRow{
			owner:      reg.Name,
			leafPath:   sel.LeafPath,
			display:    displayConfigPath(sel.LeafPath),
			roots:      roots,
			primary:    sel.SelectsDataPlane,
			defaultVal: defVal,
			backends:   backends,
		}
		if sel.SelectsDataPlane {
			primary = row
			foundPrimary = true
			continue
		}
		others = append(others, row)
	}
	if !foundPrimary {
		return declaredSelectorRow{}, nil, nil, errNoPrimaryDataPlaneSelector
	}

	slices.SortFunc(others, func(a, b declaredSelectorRow) int {
		return cmp.Compare(a.leafPath, b.leafPath)
	})
	ordered := make([]declaredSelectorRow, 0, 1+len(others))
	ordered = append(ordered, primary)
	ordered = append(ordered, others...)

	byOwner := make(map[string]declaredSelectorRow, len(ordered))
	for _, row := range ordered {
		byOwner[row.owner] = row
	}
	return primary, ordered, byOwner, nil
}

// lookupSelectorDefault resolves leafPath in schema and returns its YANG
// default value. It fails closed when the node is missing, is not a LeafNode,
// or declares no default.
func lookupSelectorDefault(schema *config.Schema, leafPath string) (string, error) {
	lookupPath := strings.TrimPrefix(leafPath, "/")
	node, err := schema.Lookup(lookupPath)
	if err != nil {
		return "", fmt.Errorf("lookup %q: %w", leafPath, err)
	}
	leaf, ok := node.(*config.LeafNode)
	if !ok {
		return "", fmt.Errorf("%q is %T, want *config.LeafNode", leafPath, node)
	}
	if leaf.Default == "" {
		return "", fmt.Errorf("%q declares no schema default", leafPath)
	}
	return leaf.Default, nil
}

// collectFIBWriters gathers all registrations that declare ProgramsFIB, sorted
// by plugin name.
func collectFIBWriters(regs []*registry.Registration) []declaredFIBRow {
	var out []declaredFIBRow
	for _, reg := range regs {
		if !reg.ProgramsFIB {
			continue
		}
		roots := make([]string, 0, len(reg.ConfigRoots))
		for _, root := range reg.ConfigRoots {
			roots = append(roots, displayConfigPath(root))
		}
		slices.Sort(roots)
		out = append(out, declaredFIBRow{
			name:          reg.Name,
			roots:         roots,
			dataPlane:     reg.DataPlane,
			hostSide:      reg.HostSideDataPlane,
			noPlaneChoice: reg.NoDataPlaneChoice,
		})
	}
	slices.SortFunc(out, func(a, b declaredFIBRow) int {
		return cmp.Compare(a.name, b.name)
	})
	return out
}

// collectDataPlaneFeatures gathers every DataPlaneFeature from registered
// plugins and standalone subsystems, split into selector-following features and
// fixed-plane features, each sorted deterministically by owner then config path.
func collectDataPlaneFeatures(
	regs []*registry.Registration,
	standalone []registry.DataPlaneFeature,
) (following, fixed []declaredFeatureRow) {
	for _, reg := range regs {
		for _, feat := range reg.DataPlaneFeatures {
			owner := feat.Name
			if owner == "" {
				owner = reg.Name
			}
			row := declaredFeatureRow{
				owner:           owner,
				configPath:      displayConfigPath(feat.ConfigPath),
				why:             feat.Why,
				planes:          sortedUnique(feat.Planes),
				followsSelector: feat.FollowsSelector,
				inUseKind:       feat.InUseKind,
				hostSide:        reg.HostSideDataPlane,
			}
			if row.followsSelector != "" {
				following = append(following, row)
				continue
			}
			fixed = append(fixed, row)
		}
	}
	for _, feat := range standalone {
		row := declaredFeatureRow{
			owner:           feat.Name,
			configPath:      displayConfigPath(feat.ConfigPath),
			why:             feat.Why,
			planes:          sortedUnique(feat.Planes),
			followsSelector: feat.FollowsSelector,
			inUseKind:       feat.InUseKind,
		}
		if row.followsSelector != "" {
			following = append(following, row)
			continue
		}
		fixed = append(fixed, row)
	}

	sortFeatures := func(a, b declaredFeatureRow) int {
		if c := cmp.Compare(a.owner, b.owner); c != 0 {
			return c
		}
		return cmp.Compare(a.configPath, b.configPath)
	}
	slices.SortFunc(following, sortFeatures)
	slices.SortFunc(fixed, sortFeatures)
	return following, fixed
}

// collectBackendAnnotations walks schema depth-first and returns every node
// carrying a non-empty ze:backend annotation, sorted by YANG path.
func collectBackendAnnotations(
	schema *config.Schema,
	primary declaredSelectorRow,
	selectors []declaredSelectorRow,
) []backendAnnotationRow {
	var rows []backendAnnotationRow
	walkSchemaBackendNodes(schema, "", func(nodePath string, backends []string) {
		governing := selectorForAnnotationPath(nodePath, primary, selectors)
		rows = append(rows, backendAnnotationRow{
			path:     "/" + nodePath,
			selector: governing.display,
			backends: sortedUnique(backends),
		})
	})
	slices.SortFunc(rows, func(a, b backendAnnotationRow) int {
		return cmp.Compare(a.path, b.path)
	})
	return rows
}

// walkSchemaBackendNodes visits every child node of cp depth-first and invokes
// visit for each node carrying a non-empty ze:backend annotation.
func walkSchemaBackendNodes(cp config.ChildProvider, prefix string, visit func(path string, backends []string)) {
	var pb textbuf.Buffer
	for _, name := range cp.Children() {
		child := cp.Get(name)
		if child == nil {
			continue
		}
		pb.Reset()
		if prefix != "" {
			pb.Str(prefix).Byte('/')
		}
		pb.Str(name)
		childPath := pb.String()

		if backends := schemaNodeBackends(child); len(backends) > 0 {
			visit(childPath, backends)
		}
		if next, ok := child.(config.ChildProvider); ok {
			walkSchemaBackendNodes(next, childPath, visit)
		}
	}
}

// schemaNodeBackends extracts the ze:backend slice from any schema node type
// that carries it.
func schemaNodeBackends(node config.Node) []string {
	switch n := node.(type) {
	case *config.LeafNode:
		return n.Backend
	case *config.ContainerNode:
		return n.Backend
	case *config.ListNode:
		return n.Backend
	case *config.InlineListNode:
		return n.Backend
	case *config.FlexNode:
		return n.Backend
	default:
		return nil
	}
}

// selectorForAnnotationPath returns the declared selector governing nodePath
// by longest matching root prefix, falling back to primary when no secondary
// selector root matches.
func selectorForAnnotationPath(
	nodePath string,
	primary declaredSelectorRow,
	selectors []declaredSelectorRow,
) declaredSelectorRow {
	best := primary
	bestLen := -1
	for _, sel := range selectors {
		if sel.primary {
			continue
		}
		for _, displayRoot := range sel.roots {
			slashRoot := strings.ReplaceAll(displayRoot, " ", "/")
			if !pathUnderRoot(nodePath, slashRoot) {
				continue
			}
			if len(slashRoot) > bestLen {
				best = sel
				bestLen = len(slashRoot)
			}
		}
	}
	return best
}

// pathUnderRoot reports whether nodePath equals root or is nested beneath root
// separated by '/'.
func pathUnderRoot(nodePath, root string) bool {
	if nodePath == root {
		return true
	}
	if len(nodePath) <= len(root) {
		return false
	}
	if !strings.HasPrefix(nodePath, root) {
		return false
	}
	return nodePath[len(root)] == '/'
}

func writeDataPlaneHeader(tb *textbuf.Buffer, primary declaredSelectorRow) {
	tb.Str("<!-- GENERATED by `./le doc yang-contract update` from live plugin\n").
		Str("     registrations and YANG ze:backend annotations. Do not edit by hand:\n").
		Str("     `./le doc check all` fails when this file and the declarations disagree. -->\n\n").
		Str("# Features by Data Plane: Linux Kernel and VPP\n\n").
		Str("Ze can forward packets through the Linux kernel (`kernel`) or through VPP (`vpp`).\n").
		Str("The primary selector (`").Str(primary.display).Str("`) chooses the deployment's\n").
		Str("forwarding data plane. Every backend, FIB writer, feature, and YANG `ze:backend`\n").
		Str("gate declares its plane requirement in code; `ze config validate`, commits,\n").
		Str("reloads, daemon startup, and `ze doctor` (`doctor-dataplane-mismatch`) all judge\n").
		Str("the configuration against the declarations listed below.\n\n").
		Str("For how Ze starts and supervises VPP, see the [VPP guide](vpp.md).\n\n")
}

func writeSelectorsSection(tb *textbuf.Buffer, selectors []declaredSelectorRow) {
	tb.Str("## Data-Plane Selectors\n\n").
		Str("Each component with pluggable forwarding backends declares a selector leaf and\n").
		Str("the configuration roots it governs. When a secondary selector is active (set\n").
		Str("explicitly, governing a present config root, or followed by an active table\n").
		Str("producer), its value must match the primary data plane (`interface backend`).\n").
		Str("An unused secondary selector at its default refuses nothing. Setting any\n").
		Str("selector leaf to a value that no backend in this build registers is refused\n").
		Str("with the schema default as the fix.\n\n").
		Str("| Selector | Role | Governed roots | Schema default | Registered backends |\n").
		Str("|----------|------|----------------|----------------|---------------------|\n")
	for _, sel := range selectors {
		role := "Secondary"
		if sel.primary {
			role = "Primary data plane"
		}
		tb.Str(dataPlaneRowPrefix).
			Str(codeSpan(sel.display)).Str(dataPlaneTableSeparator).
			Str(role).Str(dataPlaneTableSeparator).
			Str(codeSpanList(sel.roots)).Str(dataPlaneTableSeparator).
			Str(codeSpan(sel.defaultVal)).Str(dataPlaneTableSeparator).
			Str(codeSpanList(sel.backends)).Str(dataPlaneRowSuffix)
	}
	tb.Byte('\n')
}

func writeFIBSection(tb *textbuf.Buffer, primary declaredSelectorRow, writers []declaredFIBRow) {
	tb.Str("## FIB Writers\n\n").
		Str("Each FIB plugin declares `ProgramsFIB` and, when it targets a data plane, its\n").
		Str("`DataPlane`. Without an explicit `fib` block, Ze loads the writer matching\n").
		Str(codeSpan(primary.display)).Str(". An explicit `fib` block always wins. A host-side\n").
		Str("writer (`HostSideDataPlane: true`) programs the Linux host route table beside\n").
		Str("the selected data plane: it is accepted beside a selected-plane writer (for\n").
		Str("example `fib kernel` beside `fib vpp` under `interface backend vpp`), and\n").
		Str("refused alone under another data plane with the selected-plane `fib` block as\n").
		Str("the fix.\n\n").
		Str("| Plugin | Config block | Data plane | Host-side | Behavior |\n").
		Str("|--------|--------------|------------|-----------|----------|\n")
	const dataplaneYes = "yes"
	for _, w := range writers {
		planeCell := absentValueNone
		if w.dataPlane != "" {
			planeCell = codeSpan(w.dataPlane)
		}
		hostCell := "no"
		if w.hostSide {
			hostCell = dataplaneYes
		}
		behavior := fibWriterBehavior(w)
		tb.Str(dataPlaneRowPrefix).
			Str(codeSpan(w.name)).Str(dataPlaneTableSeparator).
			Str(codeSpanList(w.roots)).Str(dataPlaneTableSeparator).
			Str(planeCell).Str(dataPlaneTableSeparator).
			Str(hostCell).Str(dataPlaneTableSeparator).
			Str(behavior).Str(dataPlaneRowSuffix)
	}
	tb.Byte('\n')
}

func fibWriterBehavior(w declaredFIBRow) string {
	if w.noPlaneChoice {
		return "No-plane choice; reaches no data plane by design, accepted under any primary selector, and refused beside any other FIB plugin."
	}
	if w.dataPlane == "" {
		return "Explicit block only; declares no data plane and is not compared against the primary selector."
	}
	if w.hostSide {
		return "Host-side Linux writer; accepted beside a selected-plane FIB block and refused alone on another plane."
	}
	return "Programs its declared data plane; refused when the primary selector chooses another plane."
}

func writeFollowingFeaturesSection(
	tb *textbuf.Buffer,
	primary declaredSelectorRow,
	features []declaredFeatureRow,
	selectorByOwner map[string]declaredSelectorRow,
) {
	tb.Str("## Selector-Following Features\n\n").
		Str("These features program rules through another component's backend selector (for\n").
		Str("example firewall table producers). When any of them is active in the candidate\n").
		Str("configuration or command-line `--plugin` selection, its governing selector must\n").
		Str("match ").Str(codeSpan(primary.display)).Str(".\n\n").
		Str("| Owner | Config path | Follows selector | Active when | Why |\n").
		Str("|-------|-------------|------------------|-------------|-----|\n")
	for _, f := range features {
		selectorDisplay := f.followsSelector
		if sel, ok := selectorByOwner[f.followsSelector]; ok {
			selectorDisplay = sel.display
		}
		tb.Str(dataPlaneRowPrefix).
			Str(codeSpan(f.owner)).Str(dataPlaneTableSeparator).
			Str(codeSpan(f.configPath)).Str(dataPlaneTableSeparator).
			Str(codeSpan(selectorDisplay)).Str(dataPlaneTableSeparator).
			Str(inUseKindLabel(f.inUseKind)).Str(dataPlaneTableSeparator).
			Str(escapeTableCell(f.why)).Str(dataPlaneRowSuffix)
	}
	tb.Byte('\n')
}

func writeFixedFeaturesSection(
	tb *textbuf.Buffer,
	primary declaredSelectorRow,
	features []declaredFeatureRow,
) {
	tb.Str("## Fixed-Plane and Host-Side Features\n\n").
		Str("Features without the host-side mark are refused at commit, `ze config validate`,\n").
		Str("reload, and daemon start (and reported by `ze doctor` under\n").
		Str("`doctor-dataplane-mismatch`) whenever ").Str(codeSpan(primary.display)).Str(" selects\n").
		Str("a plane outside their supported planes.\n\n").
		Str("A feature marked host-side (`HostSideDataPlane: true`, such as `sysctl`) stays\n").
		Str("accepted on any data plane: keys controlling Linux kernel behavior apply to the\n").
		Str("host kernel on any data plane, while packet-forwarding keys such as `rp_filter`\n").
		Str("act only on packets punted to Linux, not on packets forwarded by VPP.\n\n").
		Str("| Owner | Config path | Supported planes | Host-side | Active when | Why |\n").
		Str("|-------|-------------|------------------|-----------|-------------|-----|\n")
	const dataplaneYes = "yes"
	for _, f := range features {
		hostCell := "no"
		if f.hostSide {
			hostCell = dataplaneYes
		}
		tb.Str(dataPlaneRowPrefix).
			Str(codeSpan(f.owner)).Str(dataPlaneTableSeparator).
			Str(codeSpan(f.configPath)).Str(dataPlaneTableSeparator).
			Str(codeSpanList(f.planes)).Str(dataPlaneTableSeparator).
			Str(hostCell).Str(dataPlaneTableSeparator).
			Str(inUseKindLabel(f.inUseKind)).Str(dataPlaneTableSeparator).
			Str(escapeTableCell(f.why)).Str(dataPlaneRowSuffix)
	}
	tb.Byte('\n')
}

func writeBackendAnnotationsSection(tb *textbuf.Buffer, annotations []backendAnnotationRow) {
	tb.Str("## YANG `ze:backend` Feature Gates\n\n").
		Str("The config engine walks the written configuration tree before schema defaults\n").
		Str("are materialized and refuses any node whose `ze:backend` annotation excludes the\n").
		Str("active backend of its governing selector. Narrowest annotation wins when a child\n").
		Str("node overrides an enclosing container or list. Nodes without a `ze:backend`\n").
		Str("annotation are not restricted by this walk.\n\n").
		Str("| YANG path | Selector | Supported backends |\n").
		Str("|-----------|----------|--------------------|\n")
	for _, ann := range annotations {
		tb.Str(dataPlaneRowPrefix).
			Str(codeSpan(ann.path)).Str(dataPlaneTableSeparator).
			Str(codeSpan(ann.selector)).Str(dataPlaneTableSeparator).
			Str(codeSpanList(ann.backends)).Str(dataPlaneRowSuffix)
	}
	tb.Byte('\n')
}

func writeVerifierLimitsSection(tb *textbuf.Buffer) {
	tb.Str("## Backend Verifier Limits\n\n").
		Str("Per-rule and per-qdisc compatibility limits are enforced in code by each VPP\n").
		Str("backend verifier and stay hand-written beside their source anchors:\n\n").
		Str("- [Firewall VPP compatibility and limits](firewall.md#vpp-backend-compatibility-and-limits): ").
		Str("ACL and NAT44 filter/action limits, SNAT/masquerade source-match refusal, and ").
		Str("commit-time verification of producer tables (`firewall-producer-tables`).\n").
		Str("- [Traffic control VPP compatibility matrix](traffic-control.md#vpp-backend-compatibility-matrix): ").
		Str("supported qdisc types, multi-class steering filters, root rate, mark filter, ").
		Str("and ingress policer refusals.\n").
		Str("- [VPP deployment guide](vpp.md): VPP daemon lifecycle, LCP pairing, WireGuard ").
		Str("plugin requirement (`doctor-vpp-wireguard`), and dual-FIB host-side routing.\n")
}

// inUseKindLabel renders a DataPlaneInUseKind as a short human-readable phrase.
func inUseKindLabel(kind registry.DataPlaneInUseKind) string {
	switch kind {
	case registry.DataPlaneInUseConfigRoot:
		return "config root present"
	case registry.DataPlaneInUsePredicate:
		return "predicate matches config"
	case registry.DataPlaneInUseSelectedPlugin:
		return "plugin selected in config or `--plugin`"
	case registry.DataPlaneInUseUnspecified:
		return "unspecified"
	default:
		// Open set: DataPlaneInUseKind is defined in package registry.
		return "unknown"
	}
}

// displayConfigPath converts a slash-separated config or YANG path into
// operator space-separated form.
func displayConfigPath(path string) string {
	trimmed := strings.TrimPrefix(path, "/")
	return strings.ReplaceAll(trimmed, "/", " ")
}

// sortedUnique returns a sorted, deduplicated copy of values.
func sortedUnique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := slices.Clone(values)
	slices.Sort(out)
	return slices.Compact(out)
}

// codeSpan wraps value in Markdown backticks.
func codeSpan(value string) string {
	var tb textbuf.Buffer
	return tb.Str(dataPlaneBacktick).Str(value).Str(dataPlaneBacktick).String()
}

// codeSpanList renders values as comma-separated Markdown code spans, or
// "none" when empty.
func codeSpanList(values []string) string {
	if len(values) == 0 {
		return absentValueNone
	}
	var tb textbuf.Buffer
	tb.Str(dataPlaneBacktick).Join(values, dataPlaneBacktickJoinSep).Str(dataPlaneBacktick)
	return tb.String()
}

// escapeTableCell escapes literal pipe characters inside Markdown table cells.
func escapeTableCell(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

// checkDataPlaneFeatureReference fails when docs/guide/dataplane-features.md
// disagrees with RenderDataPlanePage().
//
// Like checkPipeOperatorReference, it scopes itself to trees carrying
// docs/guide so minimal fixture roots used by drift tests owe no generated
// reference page.
func (c *checker) checkDataPlaneFeatureReference() []Issue {
	info, statErr := os.Stat(filepath.Join(c.root, "docs", "guide"))
	if statErr != nil {
		// refuse-by-default: guards checker.run -- minimal fixture roots without docs/guide owe no generated data-plane page
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		return []Issue{{
			File:    dataPlaneFeatureReferencePath,
			Message: "could not inspect docs/guide directory",
			Detail:  statErr.Error(),
		}}
	}
	if !info.IsDir() {
		return nil
	}

	path := filepath.Join(c.root, dataPlaneFeatureReferencePath)
	published, err := os.ReadFile(path) //nolint:gosec // repository-relative documentation path
	if err != nil {
		return []Issue{{
			File:    dataPlaneFeatureReferencePath,
			Message: "the generated data-plane feature reference is missing",
			Detail:  dataPlaneUpdateHint,
		}}
	}
	expected, err := RenderDataPlanePage()
	if err != nil {
		return []Issue{{
			File:    dataPlaneFeatureReferencePath,
			Message: "could not render data-plane feature reference from live declarations",
			Detail:  err.Error(),
		}}
	}
	if string(published) == expected {
		return nil
	}
	return []Issue{{
		File:    dataPlaneFeatureReferencePath,
		Message: "the published data-plane feature reference and the live declarations disagree",
		Detail:  dataPlaneUpdateHint,
	}}
}
