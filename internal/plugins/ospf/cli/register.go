// Design: plan/spec-ospf-2-wire.md -- register the offline `ospf-decode` root verb

package cli

import "codeberg.org/thomas-mangin/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("ospf-decode", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Decode a hex OSPFv2 packet from stdin to JSON (offline wire tool)",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "--pretty",
	})
}
