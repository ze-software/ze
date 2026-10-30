package policy

import sdk "github.com/ze-software/ze/pkg/plugin/sdk"

// ApplyRIBImportsForTest runs the policy plugin's config transaction path
// (parse at verify, then install at apply) over sections, for the external
// config-level tests that need the real YANG modules of the attachment owners.
func ApplyRIBImportsForTest(sections []sdk.ConfigSection) error {
	cfg, _, err := parseRIBImports(sections)
	if err != nil {
		return err
	}
	return installRIBImports(cfg)
}
