// Design: docs/research/vpp-deployment-reference.md -- registered startup requirements
// Related: startupconf.go -- renders the selected immutable requirements

package vpp

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/naming"
	"github.com/ze-software/ze/internal/core/opererr"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// StartupFeature is a raw registration declaration. RegisterStartupFeature
// validates and copies it before publication. Enabled reads the candidate
// configuration only; it MUST NOT retain or mutate that tree, or inspect or
// change the running data plane. Predicates MUST be safe for concurrent calls.
type StartupFeature struct {
	Plugin      string
	Section     string
	Flags       []string
	ConfigReads []string
	Enabled     func(*config.Tree) (bool, error)
	Runtime     *StartupRuntime
}

type startupFeature struct {
	plugin  string
	section string
	flags   []string
	reads   []string
	enabled func(*config.Tree) (bool, error)
	runtime *startupRuntime
}

type startupFeatureRegistry struct {
	mu       sync.RWMutex
	features map[string]startupFeature
}

var startupFeatures startupFeatureRegistry

// RegisterStartupFeature registers a startup requirement from the package
// owning the feature. The VPP component never names its consumers. Call from
// init before the plugin registry is sealed; safe for concurrent registration.
func RegisterStartupFeature(name string, input StartupFeature) error {
	if err := startupFeatures.register(name, input, func(root string) error {
		return registry.AddConfigRead(componentVPP, root)
	}); err != nil {
		return err
	}
	if input.Runtime == nil {
		return nil
	}
	feature, err := newStartupFeature(name, input)
	if err != nil {
		return err
	}
	return registerStartupDoctor(name, feature)
}

func (r *startupFeatureRegistry) register(name string, input StartupFeature, addRead func(string) error) error {
	feature, err := newStartupFeature(name, input)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.features[name]; exists {
		return fmt.Errorf("VPP startup feature %q is already registered", name)
	}
	for _, root := range feature.reads {
		if err := addRead(root); err != nil {
			return err
		}
	}
	if r.features == nil {
		r.features = make(map[string]startupFeature)
	}
	r.features[name] = feature
	return nil
}

func newStartupFeature(name string, input StartupFeature) (startupFeature, error) {
	if err := naming.ValidateNodeName("VPP startup feature", name, 63); err != nil {
		return startupFeature{}, err
	}
	plugin, hasSuffix := strings.CutSuffix(input.Plugin, ".so")
	if !hasSuffix {
		return startupFeature{}, errors.New("VPP startup plugin must name a .so file")
	}
	if err := naming.ValidateNodeName("VPP startup plugin", plugin, 127); err != nil {
		return startupFeature{}, err
	}
	if err := naming.ValidateNodeName("VPP startup section", input.Section, 63); err != nil {
		return startupFeature{}, err
	}
	if input.Enabled == nil {
		return startupFeature{}, errors.New("VPP startup feature has no configuration predicate")
	}
	for _, flag := range input.Flags {
		if err := naming.ValidateNodeName("VPP startup flag", flag, 127); err != nil {
			return startupFeature{}, err
		}
	}
	for _, root := range input.ConfigReads {
		if err := naming.ValidateNodeName("VPP startup read root", root, 63); err != nil {
			return startupFeature{}, err
		}
		if root == componentVPP {
			return startupFeature{}, errors.New("a VPP startup feature must not mark VPP's owned root as read-only")
		}
	}
	runtime, err := newStartupRuntime(input.Runtime)
	if err != nil {
		return startupFeature{}, err
	}
	return startupFeature{plugin: input.Plugin, section: input.Section,
		flags: slices.Clone(input.Flags), reads: slices.Clone(input.ConfigReads), enabled: input.Enabled,
		runtime: runtime}, nil
}

func (r *startupFeatureRegistry) snapshot() []startupFeature {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.features))
	for name := range r.features {
		names = append(names, name)
	}
	slices.Sort(names)
	features := make([]startupFeature, 0, len(names))
	for _, name := range names {
		features = append(features, r.features[name])
	}
	return features
}

// startupConfigRoots derives the SDK delivery declaration from the same
// registrations that extend the in-process plugin's ConfigReads.
func startupConfigRoots() []string {
	roots := []string{componentVPP}
	for _, feature := range startupFeatures.snapshot() {
		for _, root := range feature.reads {
			if !slices.Contains(roots, root) {
				roots = append(roots, root)
			}
		}
	}
	return roots
}

// ParseConfigSections reads the delivered VPP root and every registered
// startup feature from one candidate. The returned settings own their selected
// declarations. Callers MUST validate them before starting the manager.
func ParseConfigSections(sections []sdk.ConfigSection) (*VPPSettings, error) {
	settings, err := parseStartupSections(sections, startupFeatures.snapshot())
	if err != nil {
		if _, ok := errors.AsType[*opererr.Error](err); ok {
			return nil, err
		}
		return nil, opererr.Wrap(err, "VPP startup requirements cannot be configured",
			"The selected forwarding features must have complete startup settings before VPP starts.", "ze doctor")
	}
	return settings, nil
}

// StartupRuntime declares the loaded plugin's binary APIs. RegisterStartupFeature
// captures message names, CRCs and types immediately; it never retains the
// caller's message objects. Generated typed-nil messages are valid declarations.
// Code MUST have diagnostic metadata registered by the feature's own package.
type StartupRuntime struct {
	Messages []api.Message
	Code     string
	Fix      string
}

func parseStartupSections(sections []sdk.ConfigSection, features []startupFeature) (*VPPSettings, error) {
	settings := &VPPSettings{Enabled: false}
	roots := make(map[string]any, len(sections))
	for _, section := range sections {
		if section.Root == componentVPP {
			parsed, err := ParseConfigSection(section.Data)
			if err != nil {
				return nil, err
			}
			settings = parsed
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(section.Data), &data); err != nil {
			return nil, err
		}
		for name, value := range data {
			if _, exists := roots[name]; exists {
				return nil, fmt.Errorf("VPP received duplicate configuration root %q", name)
			}
			roots[name] = value
		}
	}
	tree, err := config.TreeFromPluginMap(roots)
	if err != nil {
		return nil, err
	}
	for _, feature := range features {
		enabled, err := feature.enabled(tree)
		if err != nil {
			return nil, err
		}
		if enabled {
			// Selection callbacks belong to the registry. Installed settings
			// retain only immutable facts so equal candidates compare equal.
			feature.enabled = nil
			settings.startup = append(settings.startup, feature)
		}
	}
	return settings, nil
}
