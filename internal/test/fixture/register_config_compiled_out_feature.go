// Design: docs/architecture/plugin/feature-gates.md -- naming a plugin this build was compiled without
// Related: ui_fixture_ze_stripped_no_bgp.go -- validateZEStrippedConfig, the validate door this fixture shares

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func init() {
	Register("ui/config-compiled-out-feature-names-tag", uiDriver(runConfigCompiledOutFeatureNamesTag))
}

// runConfigCompiledOutFeatureNamesTag proves a nested block that a
// compiled-out plugin owns is refused naming the build tag and both fixes,
// and a typo at the same place is not. ze-stripped carries fib (ungated) and
// no fib-vpp (ze_vpp), so `fib { vpp { } }` is unknown there only because of
// the missing tag (config.KeywordHint over registry.GatedConfigRootRequirements).
func runConfigCompiledOutFeatureNamesTag(ctx context.Context) error {
	gated, err := validateZEStrippedConfig(ctx, "fib-vpp.conf")
	if err != nil {
		return err
	}
	if gated.exitCode != 1 {
		return fmt.Errorf("fib vpp on a build without ze_vpp: rc=%d, want 1: out=%q", gated.exitCode, gated.output)
	}
	const refusal = "  line 2: unknown field in fib: vpp\n" +
		"  vpp is configured by a plugin this build of ze was compiled without, so the keyword is unknown here\n" +
		"  fix: rebuild ze with build tag ze_vpp added to its -tags list, or remove the vpp block from the config\n"
	if !strings.Contains(gated.output, refusal) {
		return fmt.Errorf("fib vpp refusal does not name ze_vpp and both fixes:\nwant\n%s\ngot\n%s", refusal, gated.output)
	}

	typo, err := validateZEStrippedConfig(ctx, "fib-typo.conf")
	if err != nil {
		return err
	}
	if typo.exitCode != 1 {
		return fmt.Errorf("fib vppqx: rc=%d, want 1: out=%q", typo.exitCode, typo.output)
	}
	if !strings.Contains(typo.output, "line 2: unknown field in fib: vppqx") {
		return fmt.Errorf("fib vppqx refusal does not name the field: out=%q", typo.output)
	}
	if strings.Contains(typo.output, "build tag") {
		return fmt.Errorf("a typo was answered as a compiled-out plugin: out=%q", typo.output)
	}

	_, err = fmt.Fprintln(os.Stdout, "compiled-out nested block names ze_vpp; typo names no tag")
	return err
}
