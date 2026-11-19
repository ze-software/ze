package fixture

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chaosscenario "github.com/ze-software/ze/internal/chaos/scenario"
)

// lgSimulatedPeersMarkup is a peers page in the shape peersContent renders
// (internal/component/lg/peers.templ): a header of plain cells, a body whose
// state cell carries a class and whose AS cell may carry the AS name in markup.
const lgSimulatedPeersMarkup = `<h1>BGP Peers</h1>
<table class="peers-table"><thead><tr>
<th>Peer</th><th>Remote AS</th><th>State</th><th>Uptime</th><th>Received</th><th>Description</th>
</tr></thead>
<tbody hx-sse:connect="/lg/events">
<tr class="state-up" hx-get="/lg/peer/198.51.100.1"><td>198.51.100.1</td><td>
 65000<span class="asn-name">Example &amp; Co</span></td><td class="state">established</td><td>3s</td><td>2</td><td></td></tr>
<tr class="state-up"><td>203.0.113.1</td><td>65101</td><td class="state">established</td><td>3s</td><td>1</td><td></td></tr>
</tbody></table>`

func lgSimulatedTestSpeakers() []chaosscenario.Speaker {
	prefix := netip.MustParsePrefix("198.18.0.0/24")
	inside := netip.MustParsePrefix("198.19.0.0/24")
	return []chaosscenario.Speaker{
		{
			Address: netip.MustParseAddr("198.51.100.1"), AS: 65000, Session: chaosscenario.SessionKindInternal,
			Announcements: []chaosscenario.Announcement{{Prefix: prefix}, {Prefix: inside}},
		},
		{
			Address: netip.MustParseAddr("203.0.113.1"), AS: 65101, Session: chaosscenario.SessionKindExternal,
			Announcements: []chaosscenario.Announcement{{Prefix: prefix}},
		},
	}
}

// TestLGSimulatedReadTableReadsByColumn reads a peers page by column name.
//
// VALIDATES: a cell is found by the name of its column, a cell with a class is
// read, and the text of a cell stops at the markup that follows it.
// PREVENTS: the fixture reading the wrong cell when the page gains a column,
// or reading an AS number with the AS name glued to it.
func TestLGSimulatedReadTableReadsByColumn(t *testing.T) {
	table, err := lgSimulatedReadTable(lgSimulatedPeersMarkup)
	require.NoError(t, err)
	require.Len(t, table.rows, 2)

	for column, want := range map[string]string{
		"Peer": "198.51.100.1", "Remote AS": "65000", "State": "established", "Received": "2", "Description": "",
	} {
		got, cellErr := table.cell(table.rows[0], column)
		require.NoError(t, cellErr, column)
		assert.Equal(t, want, got, column)
	}

	_, err = table.cell(table.rows[0], "Drops")
	require.Error(t, err, "a column the page does not have")
	_, err = table.cell([]string{"198.51.100.1"}, "State")
	require.Error(t, err, "a row shorter than the header")

	_, err = lgSimulatedReadTable(`<p class="no-results">No routes found.</p>`)
	require.Error(t, err, "a page with no table")
}

// TestLGSimulatedPeersProblemNamesTheDifference compares a peers page with a
// model that matches it, then with models that differ in one value each.
//
// VALIDATES: the page of the model's own speakers has no problem, and a wrong
// route count, a wrong AS, a session that is not established, a missing speaker
// and an extra peer are each named.
// PREVENTS: the fixture printing its OK line for a page that does not show what
// the model computed.
func TestLGSimulatedPeersProblemNamesTheDifference(t *testing.T) {
	assert.Empty(t, lgSimulatedPeersProblem(lgSimulatedPeersMarkup, lgSimulatedTestSpeakers()))

	speakers := lgSimulatedTestSpeakers()
	speakers[0].Announcements = speakers[0].Announcements[:1]
	assert.Contains(t, lgSimulatedPeersProblem(lgSimulatedPeersMarkup, speakers), `Received "2" for the speaker 198.51.100.1`)

	speakers = lgSimulatedTestSpeakers()
	speakers[1].AS = 65102
	assert.Contains(t, lgSimulatedPeersProblem(lgSimulatedPeersMarkup, speakers), `Remote AS "65101" for the speaker 203.0.113.1`)

	speakers = lgSimulatedTestSpeakers()
	speakers[1].Address = netip.MustParseAddr("203.0.113.2")
	assert.Contains(t, lgSimulatedPeersProblem(lgSimulatedPeersMarkup, speakers), "does not list the speaker 203.0.113.2")

	assert.Contains(t, lgSimulatedPeersProblem(lgSimulatedPeersMarkup, lgSimulatedTestSpeakers()[:1]), "lists 2 peers, and the model has 1 speakers")

	const idle = `<table><thead><tr><th>Peer</th><th>Remote AS</th><th>State</th><th>Received</th></tr></thead>
<tbody><tr><td>198.51.100.1</td><td>65000</td><td class="state">idle</td><td></td></tr></tbody></table>`
	speakers = lgSimulatedTestSpeakers()[:1]
	assert.Contains(t, lgSimulatedPeersProblem(idle, speakers), `State "idle" for the speaker 198.51.100.1`)
}

// TestLGSimulatedHandCheckedRefusesAWrongNetwork gives the literal check rows
// that break what the generator states it builds.
//
// VALIDATES: rows in the stated shape pass, and a second exit missing, a
// neighbour announcing another next hop than its own address, and a router
// announcing a neighbour's address as next hop are each refused.
// PREVENTS: the literal check agreeing with any set of rows.
func TestLGSimulatedHandCheckedRefusesAWrongNetwork(t *testing.T) {
	routers := func(nextHop, asPath string) []lgSimulatedRoute {
		rows := make([]lgSimulatedRoute, 0, lgSimulatedNetworkParams.Routers)
		for _, peer := range []string{"198.51.100.1", "198.51.100.2", "198.51.100.3", "198.51.100.4", "198.51.100.5"} {
			rows = append(rows, lgSimulatedRoute{peer: peer, nextHop: nextHop, asPath: asPath})
		}
		return rows
	}
	network := func() map[netip.Prefix][]lgSimulatedRoute {
		return map[netip.Prefix][]lgSimulatedRoute{
			lgSimulatedPrefixTwoExits: append(routers("198.51.100.1", "65101 64500"),
				lgSimulatedRoute{peer: "203.0.113.1", nextHop: "203.0.113.1", asPath: "65101 64500"},
				lgSimulatedRoute{peer: "203.0.113.2", nextHop: "203.0.113.2", asPath: "65102 64500"}),
			lgSimulatedPrefixOneExit: append(routers("198.51.100.2", "65103"),
				lgSimulatedRoute{peer: "203.0.113.3", nextHop: "203.0.113.3", asPath: "65103"}),
			lgSimulatedPrefixInside: routers("198.51.100.3", ""),
		}
	}
	require.NoError(t, lgSimulatedHandChecked(network()))

	oneExitOnly := network()
	oneExitOnly[lgSimulatedPrefixTwoExits] = oneExitOnly[lgSimulatedPrefixTwoExits][:6]
	require.ErrorContains(t, lgSimulatedHandChecked(oneExitOnly), "the prefix has two exits")

	foreignNextHop := network()
	foreignNextHop[lgSimulatedPrefixTwoExits][5].nextHop = "203.0.113.9"
	require.ErrorContains(t, lgSimulatedHandChecked(foreignNextHop), "it announces its own address")

	noRewrite := network()
	noRewrite[lgSimulatedPrefixTwoExits][0].nextHop = "203.0.113.1"
	require.ErrorContains(t, lgSimulatedHandChecked(noRewrite), "which is no router's loopback")

	pathInside := network()
	pathInside[lgSimulatedPrefixInside][2].asPath = "65101"
	require.ErrorContains(t, lgSimulatedHandChecked(pathInside), "originated inside the AS has none")

	missing := network()
	delete(missing, lgSimulatedPrefixOneExit)
	require.ErrorContains(t, lgSimulatedHandChecked(missing), "which every generated network has")
}
