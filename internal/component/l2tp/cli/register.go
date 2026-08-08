// Design: docs/architecture/api/commands.md — l2tp command ownership
//
// Register the `l2tp` root command with the importable command registry.
// This is the owner package: the offline L2TP CLI (packet decode, show) lives
// with internal/component/l2tp, not under cmd/ze.
package cli

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("l2tp", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "L2TP tools",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "decode, show [--user] <query>, tunnel [--user] {id <id> | all}, session [--user] {id <id> | all}",
	})

	// Flag inventory for shell completion (registration over hardcoding).
	// Mirrors the FlagSet clientFlags declares in show.go for the three
	// daemon-forwarding verbs. Every other token these verbs take belongs to
	// the daemon grammar and is forwarded unchanged, so --user is the whole
	// client-side flag surface.
	userFlag := []registry.FlagSpec{
		{Name: "--user", Description: "SSH login username (overrides zefs super-admin)", ValueHint: registry.FlagValueNone},
	}
	registry.RegisterCommandFlags("l2tp show", userFlag)
	registry.RegisterCommandFlags("l2tp tunnel", userFlag)
	registry.RegisterCommandFlags("l2tp session", userFlag)
}
