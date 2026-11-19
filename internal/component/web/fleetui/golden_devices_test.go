// Design: docs/architecture/fleet-interface.md -- the golden capture plan of the Devices tab

package fleetui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/golden"
)

// goldenDevicesTab answers the Devices tab of the page at target over the
// test estate of fixture_test.go, as the page builder fills it.
func goldenDevicesTab(t *testing.T, target string) DevicesTabView {
	t.Helper()

	view, err := devicesTabView(testRequest(t, target))
	require.NoError(t, err)

	return view
}

// goldenSpecDevices is the capture plan of page_devices.templ: the estate's
// table, a filtered and sorted one, a refused filter, a group node's table
// with a row open, and a filter no device matches.
func goldenSpecDevices(t *testing.T) golden.Spec {
	t.Helper()

	return golden.Spec{
		"page_devices.templ": {
			{Name: "DevicesTab", Variants: []golden.Variant{
				{Name: "estate", Data: DevicesTab(goldenDevicesTab(t, "/fleet/estate/devices/"))},
				{Name: "filtered", Data: DevicesTab(goldenDevicesTab(t, "/fleet/estate/devices/?q=role%3Aedge&sort=-cpu"))},
				{Name: "refused", Data: DevicesTab(goldenDevicesTab(t, "/fleet/estate/devices/?q=colour%3Ared&size=25"))},
				{Name: "group-row-open", Data: DevicesTab(goldenDevicesTab(t, "/fleet/group/site=lon/devices/?id="+coreLon01))},
				{Name: "no-match", Data: DevicesTab(goldenDevicesTab(t, "/fleet/estate/devices/?q=no-such-text"))},
			}},
		},
	}
}
