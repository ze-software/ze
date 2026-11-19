// Design: docs/architecture/fleet-interface.md -- a dashboard count and the rows its link opens

package fleetui

import (
	"bytes"
	"context"
	"html"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"
)

// The links of this file go from a dashboard card into another family's
// table, carrying a filter that table must read. Each test takes the count
// the card shows and the address its link opens, and asks the target page's
// own query how many rows that address lists. The donut links into the
// devices table are checked by TestDashboardDonutCountsEqualTheirRows.

// linksSource is the test estate with edge-lon-01 answering peers and
// interfaces whose states agree with the counts of its last sample, as a hub
// whose sample and live answer were taken together does.
type linksSource struct {
	*testSource
}

// newLinksSource answers the source of the link tests.
func newLinksSource() *linksSource {
	return &linksSource{testSource: newTestSource()}
}

// Devices sets the interface counts of edge-lon-01's last sample. Its peer
// counts are the test estate's: ten established, one down, one admin down.
func (source *linksSource) Devices(ctx context.Context) ([]Device, error) {
	devices, err := source.testSource.Devices(ctx)
	if err != nil {
		return nil, err
	}

	for index := range devices {
		if devices[index].Name == edgeLon01 {
			devices[index].Latest.InterfacesUp = 2
			devices[index].Latest.InterfacesDown = 1
			devices[index].Latest.InterfacesAdminDown = 1
		}
	}

	return devices, nil
}

// DevicePeers answers ten established peers, one in the state the device
// names (active), and one administratively down.
func (source *linksSource) DevicePeers(_ context.Context, name string) ([]Peer, error) {
	if name != edgeLon01 {
		return nil, DeviceUnknownError(name)
	}

	peers := make([]Peer, 0, 12)
	address := netip.MustParseAddr("198.51.100.1")
	for index := range 10 {
		peers = append(peers, Peer{
			Name: "peer-" + strconv.Itoa(index), Address: address, State: "established", Established: true,
		})
		address = address.Next()
	}
	peers = append(peers,
		Peer{Name: "peer-active", Address: netip.MustParseAddr("198.51.100.20"), State: "active"},
		Peer{Name: "peer-off", Address: netip.MustParseAddr("198.51.100.21"), State: "admin-down"},
	)

	return peers, nil
}

// DeviceInterfaces answers two interfaces up, one down and one
// administratively down.
func (source *linksSource) DeviceInterfaces(_ context.Context, name string) ([]Interface, error) {
	if name != edgeLon01 {
		return nil, DeviceUnknownError(name)
	}

	return []Interface{
		{Name: "eth0", AdminUp: true, LinkUp: true},
		{Name: "eth1", AdminUp: true, LinkUp: true},
		{Name: "eth2", AdminUp: true},
		{Name: "eth3"},
	}, nil
}

// cardLinks renders a card and answers, in the order drawn and once each,
// the addresses it links that carry a filter into the tab path names.
func cardLinks(t *testing.T, card templ.Component, path string) []string {
	t.Helper()

	var rendered bytes.Buffer
	require.NoError(t, card.Render(context.Background(), &rendered))

	pattern := regexp.MustCompile(`href="(` + regexp.QuoteMeta(path) + `\?[^"]*)"`)

	links := []string{}
	for _, match := range pattern.FindAllStringSubmatch(rendered.String(), -1) {
		link := html.UnescapeString(match[1])
		if !slices.Contains(links, link) {
			links = append(links, link)
		}
	}

	return links
}

// TestDashboardPeerLinksCountTheirRows opens the Peers card of a device.
//
// VALIDATES: each segment of the Peers card (established, not established,
// administratively down) counts exactly the peers the Routing tab's filter
// keeps for the address its link opens.
// PREVENTS: a link whose filter names a state no peer reports, so a count
// of one opens an empty table.
func TestDashboardPeerLinksCountTheirRows(t *testing.T) {
	source := newLinksSource()
	request := dashboardRequest(t, source, "/fleet/device/"+edgeLon01+"/dashboard/")
	scope := dashboardScopeFor(t, request)

	card, err := dashboardPeersCard(context.Background(), request, scope, CardView{Title: "Peers"})
	require.NoError(t, err)

	latest := &scope.device.Device.Latest
	counts := []int{latest.PeersEstablished, latest.PeersDown, latest.PeersAdminDown}
	links := cardLinks(t, card, "/fleet/device/"+edgeLon01+"/"+dashTabRouting+"/")
	require.Len(t, links, len(counts))

	peers, err := source.DevicePeers(context.Background(), edgeLon01)
	require.NoError(t, err)

	for index, link := range links {
		filter, parseErr := peerFilters.Parse(testAddress(t, link).View.Filter, nil)
		require.NoError(t, parseErr, link)
		require.Len(t, filter.Apply(peers), counts[index], link)
	}
}

// TestDashboardInterfaceLinksCountTheirRows opens the Interfaces card of a
// device.
//
// VALIDATES: each segment of the Interfaces card (up, down, administratively
// down) counts exactly the interfaces the Interfaces tab's filter keeps for
// the address its link opens.
// PREVENTS: an interface counted down on the card and listed under another
// state by the tab.
func TestDashboardInterfaceLinksCountTheirRows(t *testing.T) {
	source := newLinksSource()
	request := dashboardRequest(t, source, "/fleet/device/"+edgeLon01+"/dashboard/")
	scope := dashboardScopeFor(t, request)

	card, err := dashboardInterfacesCard(context.Background(), request, scope, CardView{Title: titleInterfaces})
	require.NoError(t, err)

	latest := &scope.device.Device.Latest
	counts := []int{latest.InterfacesUp, latest.InterfacesDown, latest.InterfacesAdminDown}
	links := cardLinks(t, card, "/fleet/device/"+edgeLon01+"/"+dashTabInterfaces+"/")
	require.Len(t, links, len(counts))

	interfaces, err := source.DeviceInterfaces(context.Background(), edgeLon01)
	require.NoError(t, err)

	for index, link := range links {
		filter, parseErr := interfaceFilters.Parse(testAddress(t, link).View.Filter, nil)
		require.NoError(t, parseErr, link)
		require.Len(t, filter.Apply(interfaces), counts[index], link)
	}
}

// TestDashboardAlarmLinksCountTheirRows opens the Alarms card of the estate
// and of a device.
//
// VALIDATES: the count beside the Alarms card's title equals the rows the
// Alarms tab lists for the address of its link: the estate's tab, and for a
// device the estate's tab filtered by device:<name>.
// PREVENTS: a device's alarm count that opens the alarms of every device.
func TestDashboardAlarmLinksCountTheirRows(t *testing.T) {
	source := newTestSource()

	for _, target := range []string{pathEstate, "/fleet/device/" + coreLon01 + "/dashboard/"} {
		request := dashboardRequest(t, source, target)
		view := alarmsCardView(request, dashboardScopeFor(t, request), CardView{Title: titleAlarms})
		require.NotNil(t, view.Card.Link, target)

		opened := dashboardRequest(t, source, view.Card.Link.Href)
		rows, devices, err := scopedAlarms(opened)
		require.NoError(t, err)

		filter, err := alarmFilters.Parse(opened.Address.View.Filter, LabelKeys(devices))
		require.NoError(t, err, view.Card.Link.Href)
		require.Len(t, filter.Apply(rows), view.Card.Count, view.Card.Link.Href)
	}
}

// TestDashboardPausedRolloutLinkCountsItsRows opens the paused rollout line
// of the Needs attention card.
//
// VALIDATES: the line counts exactly the rollouts the Rollouts table keeps
// for the address its link opens, state:paused.
// PREVENTS: a line that opens the rollouts table with a filter it refuses or
// that keeps another set.
func TestDashboardPausedRolloutLinkCountsItsRows(t *testing.T) {
	source := newDashboardSource()
	request := dashboardRequest(t, source, pathEstate)
	scope := dashboardScopeFor(t, request)

	lines, err := NeedsAttention(scope.views, source.rollouts)
	require.NoError(t, err)
	view := attentionCardView(scope, lines, CardView{Title: titleAttention})

	found := false
	for index, line := range view.Lines {
		if lines[index].Kind != AttentionRolloutPaused {
			continue
		}
		found = true

		address := testAddress(t, line.Href)
		require.Equal(t, dashTabRollouts, address.Tab)

		filter, parseErr := rolloutFilters.Parse(address.View.Filter, nil)
		require.NoError(t, parseErr, line.Href)
		require.Len(t, filter.Apply(source.rollouts), line.Count, line.Href)
	}
	require.True(t, found, "the test estate has a paused rollout")
}
