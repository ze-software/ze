// Design: docs/architecture/plugin/feature-gates.md -- the coupling check
// Related: scan.go -- what the source holds
// Related: report.go -- what the check answers
// Related: actions.go -- the command surface
//
// Package coupling is `le plugin coupling check`: it finds a plugin that
// consumes something only a plugin under another build gate produces, with no
// declared edge between the two. A build without the producer's gate still
// runs the consumer, which then receives nothing and says nothing, so each such
// coupling owes an edge: an optional dependency with a config need when config
// makes the consumer depend on the producer, or StartAfter when it only listens.
//
// Any declared edge satisfies the check, in either direction: Dependencies,
// OptionalDependencies or StartAfter. A relationship the producer already
// declares toward the consumer states the coupling and orders the pair, and a
// second edge from the consumer back would be a cycle. There is no allowlist:
// a benign coupling declares StartAfter, which orders without loading.
package coupling

import (
	"slices"
	"strings"

	pluginimports "github.com/ze-software/ze/internal/le/plugin/imports"
	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

// Coupling is one consumer taking an item that no package under its own build
// gates produces, with no edge to any plugin that does.
type Coupling struct {
	// Consumer is the registered name of the plugin that takes the item.
	Consumer string `json:"consumer"`
	// Kind is event, registration or subscription.
	Kind string `json:"kind"`
	// Item is the handle or call as the consumer's source spells it.
	Item string `json:"item"`
	// Package is the repository-relative package that declares the item.
	Package string `json:"package"`
	// Providers are the plugins that produce the item, any one of which an
	// edge may name. Empty when the producing code belongs to no plugin.
	Providers []string `json:"providers"`
	// ConsumerTags are the build tags that carry the consumer.
	ConsumerTags []string `json:"consumer-tags,omitempty"`
	// ProviderTags are the build tags each producing site needs, joined.
	ProviderTags []string `json:"provider-tags"`
	// Site is the file and line where the coupling is first taken.
	Site string `json:"site"`
}

// model is what the judgement reads: the plugins and their edges, the gates,
// and the scanned uses and productions.
type model struct {
	plugins []repoinventory.Plugin
	gates   pluginimports.GateIndex
	scan    scanned
}

// Check scans the tree under root and answers every undeclared cross-gate
// coupling. The registrations come from the plugins this binary links, so le,
// built with every feature tag, sees them all.
func Check(root string) (CheckReport, error) {
	built, err := load(root)
	if err != nil {
		return CheckReport{}, err
	}
	return built.judge(), nil
}

// load reads the manifest, the registrations and the source.
func load(root string) (model, error) {
	if err := rootExists(root); err != nil {
		return model{}, err
	}
	plugins, err := repoinventory.Plugins(root)
	if err != nil {
		return model{}, err
	}
	return buildModel(root, plugins)
}

// buildModel reads the manifest and the source under root for one set of
// plugins, so a test can judge the tree against registrations it edited.
func buildModel(root string, plugins []repoinventory.Plugin) (model, error) {
	gates, err := pluginimports.LoadGateIndex(root)
	if err != nil {
		return model{}, err
	}
	files, fset, err := parseTree(root)
	if err != nil {
		return model{}, err
	}
	built := model{plugins: plugins, gates: gates}
	built.scan = scanTree(files, fset, func(dir string) bool { return built.owner(dir) != "" })
	return built, nil
}

// owner answers the plugin whose source directory holds dir, the deepest one
// when two nest, and "" when no plugin's directory holds it.
func (m *model) owner(dir string) string {
	name, depth := "", -1
	for i := range m.plugins {
		source := m.plugins[i].SourceDir
		if dir != source && !strings.HasPrefix(dir, source+"/") {
			continue
		}
		if len(source) > depth {
			name, depth = m.plugins[i].Name, len(source)
		}
	}
	return name
}

// providersOf answers the plugins a production stands for: the plugin whose
// directory holds it, or, for code outside every plugin's directory, each
// plugin built under exactly the same gates, because that code ships with them.
func (m *model) providersOf(made production) []string {
	if name := m.owner(made.dir); name != "" {
		return []string{name}
	}
	tags := m.gates.Requirement(made.dir)
	var names []string
	for i := range m.plugins {
		if slices.Equal(m.gates.Requirement(m.plugins[i].SourceDir), tags) {
			names = append(names, m.plugins[i].Name)
		}
	}
	return names
}

// linked reports whether either plugin declares any edge to the other.
func (m *model) linked(a, b string) bool {
	for i := range m.plugins {
		p := &m.plugins[i]
		var other string
		switch p.Name {
		case a:
			other = b
		case b:
			other = a
		default:
			continue
		}
		if slices.Contains(p.Dependencies, other) {
			return true
		}
		if slices.Contains(p.OptionalDependencies, other) {
			return true
		}
		if slices.Contains(p.StartAfter, other) {
			return true
		}
	}
	return false
}

// judge answers every undeclared cross-gate coupling, one per consumer and
// item, sorted. A use is cross-gate when no production of its item needs only
// tags the consumer's own build already carries.
func (m *model) judge() CheckReport {
	report := CheckReport{Plugins: len(m.plugins), Files: m.scan.files, Couplings: []Coupling{}}
	seen := map[string]bool{}
	for _, taken := range m.scan.uses {
		consumer := m.owner(taken.dir)
		if consumer == "" {
			report.Unjudged = append(report.Unjudged, taken.site)
			continue
		}
		found, owed := m.couplingOf(consumer, taken)
		if !owed {
			continue
		}
		if seen[consumer+" "+taken.key] {
			continue
		}
		seen[consumer+" "+taken.key] = true
		if len(found.Providers) == 0 {
			report.Undeclarable = append(report.Undeclarable, found)
			continue
		}
		report.Couplings = append(report.Couplings, found)
	}
	slices.SortFunc(report.Couplings, compareCouplings)
	slices.SortFunc(report.Undeclarable, compareCouplings)
	slices.Sort(report.Unjudged)
	return report
}

// couplingOf answers the coupling one use owes, and false when it owes none:
// no production at all, a production inside the consumer's own gates, or a
// declared edge to one of the providers.
func (m *model) couplingOf(consumer string, taken use) (Coupling, bool) {
	made := m.scan.productions[taken.key]
	if len(made) == 0 {
		return Coupling{}, false
	}
	consumerTags := m.gates.Requirement(taken.dir)
	var providers, providerTags []string
	attributed := false
	for _, production := range made {
		tags := m.gates.Requirement(production.dir)
		if subset(tags, consumerTags) {
			return Coupling{}, false
		}
		// A plugin that fills its own registry or feeds its own subscribers is
		// one unit, whatever gates its subpackages carry.
		if m.owner(production.dir) == consumer {
			return Coupling{}, false
		}
		if m.owner(production.dir) != "" {
			attributed = true
		}
		providerTags = appendMissing(providerTags, tags...)
		providers = appendMissing(providers, m.providersOf(production)...)
	}
	providers = slices.DeleteFunc(providers, func(name string) bool { return name == consumer })
	for _, provider := range providers {
		if m.linked(consumer, provider) {
			return Coupling{}, false
		}
	}
	// Code outside every plugin's directory (a subsystem such as L2TP) is no
	// name an edge can carry, so the coupling cannot be declared: it is
	// reported apart and does not fail the check. Same-gate plugins still
	// satisfied it above, because that code ships with them.
	if !attributed {
		providers = nil
	}
	slices.Sort(providers)
	slices.Sort(providerTags)
	return Coupling{
		Consumer:     consumer,
		Kind:         taken.kind,
		Item:         taken.label,
		Package:      packageOf(taken.key),
		Providers:    providers,
		ConsumerTags: consumerTags,
		ProviderTags: providerTags,
		Site:         taken.site,
	}, true
}

// compareCouplings orders couplings by consumer, then item.
func compareCouplings(a, b Coupling) int {
	if c := strings.Compare(a.Consumer, b.Consumer); c != 0 {
		return c
	}
	return strings.Compare(a.Item, b.Item)
}

// packageOf answers the package directory a key starts with.
func packageOf(key string) string {
	dot := strings.LastIndex(key[:strings.IndexByte(key+"(", '(')], ".")
	if dot < 0 {
		return key
	}
	return key[:dot]
}

// subset reports whether every tag in part is in whole.
func subset(part, whole []string) bool {
	for _, tag := range part {
		if !slices.Contains(whole, tag) {
			return false
		}
	}
	return true
}

// appendMissing appends each value not already in list.
func appendMissing(list []string, values ...string) []string {
	for _, value := range values {
		if !slices.Contains(list, value) {
			list = append(list, value)
		}
	}
	return list
}
