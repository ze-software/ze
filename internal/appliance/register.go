// Design: plan/spec-appliance-command-plugin.md — appliance command provider
package appliance

import (
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func subcommands() string {
	seen := make(map[string]struct{})
	for _, cmd := range applianceCommands() {
		seen[cmd.Key] = struct{}{}
	}
	sorted := make([]string, 0, len(seen))
	for k := range seen {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}

func init() {
	registry.MustRegisterRootHandler("appliance", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Manage gokrazy-based Ze appliance images",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        subcommands(),
	})
}
