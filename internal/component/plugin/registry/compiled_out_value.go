// Design: docs/architecture/plugin/feature-gates.md -- naming a plugin this build was compiled without
// Related: gated_plugins.go -- GatedPluginTags, the generated table this file reads
// Related: config_need.go -- ConfigNeed, the declaration that ties a value to its provider

package registry

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
)

// CompiledOutValueError answers the refusal of value, a setting of the plugin
// owner (a backend name), when the build cannot act on it because every
// provider the owner's ConfigNeed names for that value was compiled out. It
// answers nil when owner declares no need on value, when a provider is
// registered, or when no provider is gated: then no build tag would add one,
// and naming a tag would send the operator the wrong way.
//
// The need is the one declaration of which plugin a value needs (the config
// component refuses the same config at validate, commit and boot from it,
// optional_provider.go); this answer serves a backend loader that meets the
// value after those doors, so both name the same tags.
func CompiledOutValueError(owner, value string) error {
	reg := Lookup(owner)
	if reg == nil {
		return nil
	}
	for _, need := range reg.ConfigNeeds {
		if need.Value != value {
			continue
		}
		requirements, gated := compiledOutRequirements(need)
		if !gated {
			continue
		}
		setting := strings.Join(need.Path, " ") + " " + value
		return compiledOutValueRefusal(owner, setting, need.Providers, requirements)
	}
	return nil
}

// compiledOutRequirements answers the tag sets that would carry a provider of
// need, and false when a provider is registered or none is gated.
func compiledOutRequirements(need ConfigNeed) (requirements [][]string, gated bool) {
	for _, provider := range need.Providers {
		if Lookup(provider) != nil {
			return nil, false
		}
		tags, ok := gatedPluginTags[provider]
		if !ok {
			continue
		}
		gated = true
		requirements = appendSmallestRequirement(requirements, tags)
	}
	slices.SortFunc(requirements, slices.Compare[[]string])
	return requirements, gated
}

// compiledOutValueRefusal writes the three parts: what the operator set, why
// the build cannot act on it, and the two fixes (a build with the tag, or the
// config without the setting).
func compiledOutValueRefusal(owner, setting string, providers []string, requirements [][]string) error {
	var problem strings.Builder
	problem.WriteString(owner)
	problem.WriteString(`: "`)
	problem.WriteString(setting)
	problem.WriteString(`" names a backend this build of ze was compiled without`)

	var why strings.Builder
	why.WriteString("the plugin that provides it (")
	why.WriteString(strings.Join(providers, " or "))
	why.WriteString(") is not in this build, so ")
	why.WriteString(owner)
	why.WriteString(" cannot load the backend")

	var fix strings.Builder
	fix.WriteString("rebuild ze with ")
	for i, tags := range requirements {
		if i > 0 {
			fix.WriteString(", or with ")
		}
		fix.WriteString(BuildTagsPhrase(tags))
	}
	fix.WriteString(` added to its -tags list, or remove "`)
	fix.WriteString(setting)
	fix.WriteString(`" from the config`)

	return opererr.New(problem.String(), why.String(), fix.String())
}

// BuildTagsPhrase writes "build tag ze_bgp" for one tag and "build tags ze_bgp
// and ze_bmp" for a nested gate, whose tags must all be set together. Every
// refusal that names a compiled-out plugin's tags uses it, so they read alike.
func BuildTagsPhrase(tags []string) string {
	if len(tags) == 1 {
		return "build tag " + tags[0]
	}
	return "build tags " + strings.Join(tags, " and ")
}
