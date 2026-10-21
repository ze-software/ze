// Design: docs/architecture/api/update-syntax.md -- the `bgp-prefix-sid` keyword
// of update text and the attribute it writes.
// RFC: rfc/short/rfc8669.md - BGP Prefix-SID Label-Index and Originator SRGB TLVs
// Related: test/plugin/update-text-prefix-sid.ci -- the frame this announce must
// produce.
// Related: internal/component/bgp/plugins/cmd/update/update_text.go -- the
// KWPrefixSID case of parseCommonAttributeText that reads the value.
package fixture

import (
	"context"
	"fmt"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/update-text-prefix-sid", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "update-text-prefix-sid", &sdk.Registration{}, ObserverScenario(updateTextPrefixSID))
	})
}

// updateTextPrefixSID announces one labeled route carrying a label-index BGP
// Prefix-SID with two Originator SRGB ranges, through the daemon's own
// `update text` dispatcher.
//
// The value is spelled as ExaBGP writes it on the API, a comma between the two
// range groups included, so the .ci proves the spelling the bridge passes
// through reaches the wire. RFC 8669 Section 3.1 Label-Index TLV and Section
// 3.2 Originator SRGB TLV.
func updateTextPrefixSID(ctx context.Context, plugin *sdk.Plugin) error {
	const announce = "update text origin igp local-preference 100 " +
		"bgp-prefix-sid [ 300, [ ( 800000,4096 ) , ( 1000000,5000 ) ] ] " +
		"nhop 198.51.100.2 mp ipv4/mpls-label label 800002 add 198.51.100.101/32"
	if _, _, err := plugin.UpdateRoute(ctx, "*", announce); err != nil {
		return fmt.Errorf("update text with bgp-prefix-sid: %w", err)
	}
	if err := plugin01Quiesce(ctx, plugin); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: update text carrying bgp-prefix-sid dispatched")
	return nil
}
