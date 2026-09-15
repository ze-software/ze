// Design: docs/architecture/appliance/command-provider.md -- appliance command provider

// codegen:skip -- the ze_setup personality wires this, from cmd/ze/setup_features_setup.go
// under //go:build ze_setup. The universal composition root carries no tag, so naming it
// there would link the installer commands into every ze build.

package appliance

import (
	"fmt"
	"os"
	"slices"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
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
	slices.Sort(sorted)
	return textbuf.Join(sorted, ", ")
}

func init() {
	registry.MustRegisterRootHandler("appliance", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		ShortHelp: "Manage gokrazy-based Ze appliance images",
		Mode:      "offline",
		Section:   registry.SectionSystem,
		Subs:      subcommands(),
	})

	for _, check := range applianceDoctorChecks() {
		if err := diagnostic.RegisterDoctorCheck(check); err != nil {
			fmt.Fprintf(os.Stderr, "appliance doctor check registration: %v\n", err)
			os.Exit(2)
		}
	}
}
