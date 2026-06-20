// Design: plan/spec-isis-2-wire.md -- register the offline `isis-decode` root verb
//
// Owner package: the offline IS-IS PDU decode CLI lives with
// internal/plugins/isis (the codec), not under cmd/ze. The root verb is
// "isis-decode" -- a dedicated OFFLINE verb, intentionally distinct from:
//   - the "isis" config component root (owned by isis-4), and
//   - the "show isis" / "clear isis" command tree (owned by isis-13).
// Keeping a separate verb avoids any collision with those siblings while still
// giving the wire codec a runnable end-to-end proof (test/isis-wire/).

package cli

import "codeberg.org/thomas-mangin/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("isis-decode", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Decode a hex IS-IS PDU from stdin to JSON (offline wire tool)",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "--pretty",
	})
}
