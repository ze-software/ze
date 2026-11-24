// Design: docs/architecture/api/architecture.md -- implied configuration report model, provider seams, and offline external endpoint query refusal.
// Related: implied_render.go -- text, set-command, and plain-display notice renderers for ImpliedReport.
// Related: graph.go -- config dependency graph consuming GraphBusEdgeProvider.
// Related: declared_schemas.go -- offline external program declaration query mode.
// Related: register_bus.go -- external bus declaration store.

package config

import (
	"cmp"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/plugin"
)

// Canonical origin values for compiled bus delivery rules and dependency graph edges.
const (
	ImpliedOriginWritten    = "written"
	ImpliedOriginInherited  = "inherited"
	ImpliedOriginRegistered = "registered"
	ImpliedOriginRequired   = "required"
	ImpliedOriginDormant    = "dormant"
)

// ImpliedEndpoint describes one endpoint Ze loads automatically without an
// explicit bus > endpoint block written in the configuration.
type ImpliedEndpoint struct {
	Name           string `json:"name" yaml:"name"`
	CompiledPlugin string `json:"compiled-plugin,omitempty" yaml:"compiled-plugin,omitempty"`
	ReasonKind     string `json:"reason-kind" yaml:"reason-kind"`
	ReasonSource   string `json:"reason-source,omitempty" yaml:"reason-source,omitempty"`
	ImpliedBy      string `json:"implied-by" yaml:"implied-by"`
}

// ImpliedDirectionRule describes one effective send or receive message rule on
// a bus participant, including its origin, producing line, replaced defaults,
// tag filters, and dormancy status.
type ImpliedDirectionRule struct {
	Message          string              `json:"message" yaml:"message"`
	Allowed          bool                `json:"allowed" yaml:"allowed"`
	Origin           string              `json:"origin" yaml:"origin"`
	Source           string              `json:"source,omitempty" yaml:"source,omitempty"`
	Targets          []string            `json:"targets,omitempty" yaml:"targets,omitempty"`
	ReplacedDefaults []string            `json:"replaced-defaults,omitempty" yaml:"replaced-defaults,omitempty"`
	Tags             map[string][]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Dormant          bool                `json:"dormant,omitempty" yaml:"dormant,omitempty"`
	DormantReason    string              `json:"dormant-reason,omitempty" yaml:"dormant-reason,omitempty"`
}

// ImpliedParticipant describes the effective bus allow-lists and per-message
// rules for one participant path.
type ImpliedParticipant struct {
	Path               string                 `json:"path" yaml:"path"`
	EndpointName       string                 `json:"endpoint-name,omitempty" yaml:"endpoint-name,omitempty"`
	ParentPath         string                 `json:"parent-path,omitempty" yaml:"parent-path,omitempty"`
	SendAllow          []string               `json:"send-allow,omitempty" yaml:"send-allow,omitempty"`
	SendAllowNone      bool                   `json:"send-allow-none,omitempty" yaml:"send-allow-none,omitempty"`
	SendAllowOrigin    string                 `json:"send-allow-origin,omitempty" yaml:"send-allow-origin,omitempty"`
	SendAllowSource    string                 `json:"send-allow-source,omitempty" yaml:"send-allow-source,omitempty"`
	ReceiveAllow       []string               `json:"receive-allow,omitempty" yaml:"receive-allow,omitempty"`
	ReceiveAllowNone   bool                   `json:"receive-allow-none,omitempty" yaml:"receive-allow-none,omitempty"`
	ReceiveAllowOrigin string                 `json:"receive-allow-origin,omitempty" yaml:"receive-allow-origin,omitempty"`
	ReceiveAllowSource string                 `json:"receive-allow-source,omitempty" yaml:"receive-allow-source,omitempty"`
	Receive            []ImpliedDirectionRule `json:"receive,omitempty" yaml:"receive,omitempty"`
	Send               []ImpliedDirectionRule `json:"send,omitempty" yaml:"send,omitempty"`
}

// ImpliedFilterMember describes one filter chain member inserted automatically
// alongside a target's written filter list.
type ImpliedFilterMember struct {
	TargetPath string `json:"target-path" yaml:"target-path"`
	Direction  string `json:"direction" yaml:"direction"`
	Position   string `json:"position" yaml:"position"`
	Ref        string `json:"ref" yaml:"ref"`
	ImpliedBy  string `json:"implied-by" yaml:"implied-by"`
	Required   bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Reason     string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// ImpliedReport collects every implied endpoint, effective bus participant
// wiring set, and inserted filter member active for a configuration.
type ImpliedReport struct {
	Endpoints    []ImpliedEndpoint     `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	Participants []ImpliedParticipant  `json:"participants,omitempty" yaml:"participants,omitempty"`
	Filters      []ImpliedFilterMember `json:"filters,omitempty" yaml:"filters,omitempty"`
}

// ImpliedEnvelope wraps a byte-pure written configuration beside its computed
// ImpliedReport for structured JSON and YAML outputs.
type ImpliedEnvelope struct {
	Config  any           `json:"config" yaml:"config"`
	Implied ImpliedReport `json:"implied" yaml:"implied"`
}

// ImpliedOptions configures how BuildImpliedReport gathers implied items.
type ImpliedOptions struct {
	Offline       bool
	ConfigFile    string
	LiveEndpoints []ImpliedEndpoint
	LiveCompiled  any
}

// ImpliedBusProvider computes implied endpoints and effective participant
// delivery rules from a parsed configuration tree and options.
type ImpliedBusProvider func(tree *Tree, opts ImpliedOptions) ([]ImpliedEndpoint, []ImpliedParticipant, error)

// ImpliedFilterProvider computes inserted filter chain members from a parsed
// configuration tree.
type ImpliedFilterProvider func(tree *Tree) ([]ImpliedFilterMember, error)

// GraphBusEdgeProvider computes origin-annotated bus dependency edges for
// BuildGraph from a parsed configuration tree.
type GraphBusEdgeProvider func(tree *Tree) []GraphEdge

// IsEmpty reports whether r holds zero implied items.
func (r ImpliedReport) IsEmpty() bool {
	return r.Count() == 0
}

// Count returns the number of implied items in r: implied endpoints,
// non-written participant rules, and implied filter members.
func (r ImpliedReport) Count() int {
	total := len(r.Endpoints) + len(r.Filters)
	for i := range r.Participants {
		p := &r.Participants[i]
		for j := range p.Receive {
			if p.Receive[j].Origin != ImpliedOriginWritten {
				total++
			}
		}
		for j := range p.Send {
			if p.Send[j].Origin != ImpliedOriginWritten {
				total++
			}
		}
	}
	return total
}

var (
	impliedProvidersMu     sync.RWMutex
	impliedBusProvider     ImpliedBusProvider
	impliedFilterProviders = map[string]ImpliedFilterProvider{}
	graphBusEdgeProvider   GraphBusEdgeProvider
)

// RegisterImpliedBusProvider registers the bus implied-report provider invoked
// by BuildImpliedReport. Callers MUST register from init(). Safe for concurrent
// use.
func RegisterImpliedBusProvider(fn ImpliedBusProvider) {
	if fn == nil {
		panic("BUG: config: RegisterImpliedBusProvider requires a non-nil function")
	}
	impliedProvidersMu.Lock()
	impliedBusProvider = fn
	impliedProvidersMu.Unlock()
}

// RegisterImpliedFilterProvider registers a named filter-chain provider invoked
// by BuildImpliedReport in sorted name order. Callers MUST register from
// init(). Safe for concurrent use.
func RegisterImpliedFilterProvider(name string, fn ImpliedFilterProvider) {
	if name == "" {
		panic("BUG: config: RegisterImpliedFilterProvider requires a non-empty name")
	}
	if fn == nil {
		panic("BUG: config: RegisterImpliedFilterProvider requires a non-nil function")
	}
	impliedProvidersMu.Lock()
	if impliedFilterProviders == nil {
		impliedFilterProviders = map[string]ImpliedFilterProvider{}
	}
	impliedFilterProviders[name] = fn
	impliedProvidersMu.Unlock()
}

// RegisterGraphBusEdgeProvider registers the compiled bus edge provider invoked
// by BuildGraph. Callers MUST register from init(). Safe for concurrent use.
func RegisterGraphBusEdgeProvider(fn GraphBusEdgeProvider) {
	if fn == nil {
		panic("BUG: config: RegisterGraphBusEdgeProvider requires a non-nil function")
	}
	impliedProvidersMu.Lock()
	graphBusEdgeProvider = fn
	impliedProvidersMu.Unlock()
}

// getGraphBusEdgeProvider returns the currently registered bus edge provider,
// or nil when none is registered. Safe for concurrent use.
func getGraphBusEdgeProvider() GraphBusEdgeProvider {
	impliedProvidersMu.RLock()
	fn := graphBusEdgeProvider
	impliedProvidersMu.RUnlock()
	return fn
}

// ResetImpliedProvidersForTest clears all registered implied-report and graph
// bus edge providers and returns a function that restores the prior state.
// MUST be paired with the returned restore function, typically via t.Cleanup.
// Safe for concurrent use.
func ResetImpliedProvidersForTest() (restore func()) {
	impliedProvidersMu.Lock()
	savedBus := impliedBusProvider
	savedGraph := graphBusEdgeProvider
	savedFilters := make(map[string]ImpliedFilterProvider, len(impliedFilterProviders))
	maps.Copy(savedFilters, impliedFilterProviders)
	impliedBusProvider = nil
	graphBusEdgeProvider = nil
	impliedFilterProviders = map[string]ImpliedFilterProvider{}
	impliedProvidersMu.Unlock()

	return func() {
		impliedProvidersMu.Lock()
		impliedBusProvider = savedBus
		graphBusEdgeProvider = savedGraph
		impliedFilterProviders = savedFilters
		impliedProvidersMu.Unlock()
	}
}

// BuildImpliedReport collects implied endpoints, effective bus participant
// rules, and inserted filter members from registered providers and returns a
// deterministically sorted ImpliedReport.
//
// When opts.Offline is true, any external endpoint under bus > endpoint whose
// declaration has not yet been recorded is queried in declare mode (never
// starting its run loop). If any query fails, BuildImpliedReport refuses with
// an *opererr.Error naming the endpoint, its run line, the underlying cause,
// and the conformance fix command, and returns an empty ImpliedReport.
func BuildImpliedReport(tree *Tree, opts ImpliedOptions) (ImpliedReport, error) {
	if opts.Offline && tree != nil {
		if err := queryUnrecordedExternalEndpoints(tree, opts.ConfigFile); err != nil {
			return ImpliedReport{}, err
		}
	}

	busProvider, filterProviders := snapshotImpliedProviders()

	var report ImpliedReport
	if busProvider != nil {
		eps, participants, err := busProvider(tree, opts)
		if err != nil {
			return ImpliedReport{}, err
		}
		report.Endpoints = slices.Clone(eps)
		report.Participants = slices.Clone(participants)
	} else if len(opts.LiveEndpoints) > 0 {
		report.Endpoints = slices.Clone(opts.LiveEndpoints)
	}

	for _, fp := range filterProviders {
		members, err := fp(tree)
		if err != nil {
			return ImpliedReport{}, err
		}
		report.Filters = append(report.Filters, members...)
	}

	sortImpliedReport(&report)
	return report, nil
}

// snapshotImpliedProviders reads the registered bus and filter providers under
// a single read lock, returning filter providers in sorted name order.
func snapshotImpliedProviders() (ImpliedBusProvider, []ImpliedFilterProvider) {
	impliedProvidersMu.RLock()
	defer impliedProvidersMu.RUnlock()

	busFn := impliedBusProvider
	if len(impliedFilterProviders) == 0 {
		return busFn, nil
	}
	names := slices.Sorted(maps.Keys(impliedFilterProviders))
	filters := make([]ImpliedFilterProvider, 0, len(names))
	for _, name := range names {
		filters = append(filters, impliedFilterProviders[name])
	}
	return busFn, filters
}

// queryUnrecordedExternalEndpoints checks every external endpoint declared
// under bus > endpoint in tree. Any endpoint not already recorded in the
// external bus declaration store is queried in offline declare mode; any
// failure refuses the whole report with an *opererr.Error.
func queryUnrecordedExternalEndpoints(tree *Tree, configFile string) error {
	busContainer := tree.GetContainer(busContainerName)
	if busContainer == nil {
		return nil
	}

	workDir := ""
	if namesFile(configFile) {
		workDir = filepath.Dir(configFile)
	}

	var unrecorded []plugin.PluginConfig
	for _, entry := range busContainer.GetListOrdered(busListEndpoint) {
		extContainer := entry.Value.GetContainer(busContainerExternal)
		if extContainer == nil {
			continue
		}
		if _, recorded := lookupExternalBusDecl(entry.Key); recorded {
			continue
		}
		runVal, _ := extContainer.Get("run")
		if strings.TrimSpace(runVal) == "" {
			return externalQueryRefusal(
				plugin.PluginConfig{Name: entry.Key, Run: runVal, WorkDir: workDir},
				"external endpoint has no run command",
			)
		}
		unrecorded = append(unrecorded, plugin.PluginConfig{
			Name:    entry.Key,
			Run:     runVal,
			WorkDir: workDir,
		})
	}
	if len(unrecorded) == 0 {
		return nil
	}

	_, busDecls, _, err := queryExternalDeclarations(unrecorded)
	if err != nil {
		return err
	}
	for _, bd := range busDecls {
		RecordExternalBusDecl(bd)
	}
	return nil
}

// sortImpliedReport sorts all slices inside report for deterministic output.
func sortImpliedReport(report *ImpliedReport) {
	slices.SortFunc(report.Endpoints, func(a, b ImpliedEndpoint) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		if c := cmp.Compare(a.CompiledPlugin, b.CompiledPlugin); c != 0 {
			return c
		}
		if c := cmp.Compare(a.ReasonKind, b.ReasonKind); c != 0 {
			return c
		}
		return cmp.Compare(a.ImpliedBy, b.ImpliedBy)
	})

	slices.SortFunc(report.Participants, func(a, b ImpliedParticipant) int {
		return cmp.Compare(a.Path, b.Path)
	})
	for i := range report.Participants {
		p := &report.Participants[i]
		sortImpliedDirectionRules(p.Receive)
		sortImpliedDirectionRules(p.Send)
	}

	slices.SortStableFunc(report.Filters, func(a, b ImpliedFilterMember) int {
		if c := cmp.Compare(a.TargetPath, b.TargetPath); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Direction, b.Direction); c != 0 {
			return c
		}
		return cmp.Compare(a.Position, b.Position)
	})
}

func sortImpliedDirectionRules(rules []ImpliedDirectionRule) {
	slices.SortFunc(rules, func(a, b ImpliedDirectionRule) int {
		if c := cmp.Compare(a.Message, b.Message); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Origin, b.Origin); c != 0 {
			return c
		}
		return cmp.Compare(a.Source, b.Source)
	})
}
