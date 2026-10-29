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

	// Flag inventory for shell completion and the flag-register gate. It
	// mirrors the FlagSet runKernel (cmd_kernel.go) parses: a flag added there
	// and not here is invisible to completion, and `./le cli grammar` refuses it.
	registry.RegisterCommandFlags("appliance kernel", []registry.FlagSpec{
		{Name: "--arch", Description: "target architecture (default: from appliance config or host)", ValueHint: registry.FlagValueNone},
		{Name: "--builder", Description: "build backend: docker or qemu", ValueHint: registry.FlagValueNone},
		{Name: "--evict-cache", Description: "bound this kernel's cache namespace and exit, without building", ValueHint: registry.FlagValueNone},
		{Name: "--jobs", Description: "positive parallel kernel build job count (default: builder CPU count)", ValueHint: registry.FlagValueNone},
		{Name: "--print-cache-dir", Description: "print this kernel's durable cache path and exit, without building", ValueHint: registry.FlagValueNone},
		{Name: "--profile", Description: "kernel profile token", ValueHint: registry.FlagValueNone},
		{Name: "--target", Description: "kernel target: installer (default) or runtime", ValueHint: registry.FlagValueNone},
		{Name: "--version", Description: "Linux kernel version", ValueHint: registry.FlagValueNone},
	})

	for _, check := range applianceDoctorChecks() {
		if err := diagnostic.RegisterDoctorCheck(check); err != nil {
			fmt.Fprintf(os.Stderr, "appliance doctor check registration: %v\n", err)
			os.Exit(2)
		}
	}
}
