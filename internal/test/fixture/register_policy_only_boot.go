// Design: docs/guide/plugins.md -- the BGP filter plugins own bgp and read policy.
// Related: test/plugin/policy-only-starts-without-bgp.ci -- the config this observes.
package fixture

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// policyOnlyBootProbe is a command only the BGP subsystem serves.
const policyOnlyBootProbe = "show bgp peer"

func init() {
	Register("plugin/policy-only-boot", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "plugin/policy-only-boot", &sdk.Registration{}, policyOnlyBoot)
	})
}

// policyOnlyBoot runs once every plugin the policy-only config selected is
// ready, so reaching it at all proves the startup tier completed. It then
// asks the daemon for a BGP command: with no bgp block, no BGP plugin may
// have loaded, so the command MUST be refused rather than answered.
func policyOnlyBoot(ctx context.Context, p *sdk.Plugin) error {
	status, _, err := p.DispatchCommand(ctx, policyOnlyBootProbe)
	if err == nil {
		var tb textbuf.Buffer
		tb.Str("the daemon answered ").Str(policyOnlyBootProbe).Str(" with status ").Str(status)
		tb.Str(": a policy-only config loaded BGP")
		return errors.New(tb.String())
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var ok textbuf.Buffer
	ok.Str("OK: every plugin is ready and the daemon refused ").Str(policyOnlyBootProbe)
	ok.Str(": ").Str(err.Error()).Byte('\n')
	return ok.StdErr()
}
