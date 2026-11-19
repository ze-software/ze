package fleetui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// The words of the device test source's relay.
const (
	testRelayProfile = "operator"
	testRefusal      = "relayed commands are refused: this device grants them no profile"
	testGrantLine    = "set fleet relay profile operator"
	testVersionLine  = "ze 1.4.2 (test build)"
)

// deviceTestSource answers the device views over the test estate of
// fixture_test.go: edge-lon-01 runs BGP and OSPF, rs-ams-01 runs BGP on a
// release that does not answer the BGP summary, core-ams-01 is offline,
// cpe-0113 grants no relay profile, and the others run no routing protocol.
type deviceTestSource struct {
	*testSource
}

// newDeviceTestSource answers a device test source.
func newDeviceTestSource() *deviceTestSource {
	return &deviceTestSource{testSource: newTestSource()}
}

// Devices answers the test estate, with the relay granted by every device
// but cpe-0113 and the identity of edge-lon-01 filled in.
func (source *deviceTestSource) Devices(context.Context) ([]Device, error) {
	devices := testDevices()
	for index := range devices {
		device := &devices[index]
		if device.Name != cpe0113 {
			device.Connection.RelayGranted = true
			device.Connection.RelayProfile = testRelayProfile
		}
		if device.Name == edgeLon01 {
			device.Inventory.Serial = "ZE1000-0001"
			device.Inventory.ManagementAddr = netip.MustParseAddr("192.0.2.1")
		}
	}

	return devices, nil
}

// reach answers the failure every question about the device named name
// meets, or nil.
func (source *deviceTestSource) reach(name string) error {
	switch name {
	case coreAms01:
		return opererr.Wrap(ErrDeviceOffline, coreAms01+" has been offline since 2026-11-12 12:00:00 UTC.",
			"The device holds no managed connection, so the hub cannot ask it for its live state.",
			"open /fleet/device/core-ams-01/tasks/")
	case cpe0113:
		return opererr.Wrap(ErrRelayRefused, cpe0113+" refused the command: "+testRefusal+".",
			"The device decides which profile a relayed command runs under, and its configuration names none.",
			testGrantLine)
	}

	devices, _ := source.Devices(context.Background()) //nolint:errcheck // the test estate always answers
	for index := range devices {
		if devices[index].Name == name {
			return nil
		}
	}

	return DeviceUnknownError(name)
}

func (source *deviceTestSource) DeviceSystem(_ context.Context, name string) (DeviceSystem, error) {
	if err := source.reach(name); err != nil {
		return DeviceSystem{}, err
	}

	return DeviceSystem{
		Time:     testNow.Add(2 * time.Second),
		Hardware: []HardwareItem{{Kind: "processor", Description: "4 cores", Detail: "healthy"}},
		Resources: Resources{
			CPUPercent: 71, Load: 1.5,
			MemoryUsedBytes: 4 << 30, MemoryTotalBytes: 8 << 30,
			StorageUsedBytes: 10 << 30, StorageTotalBytes: 100 << 30,
		},
	}, nil
}

func (source *deviceTestSource) DeviceInterfaces(_ context.Context, name string) ([]Interface, error) {
	if err := source.reach(name); err != nil {
		return nil, err
	}

	return []Interface{
		{
			Name: "eth0", Kind: "ethernet", AdminUp: true, LinkUp: true,
			Addresses: []netip.Prefix{netip.MustParsePrefix("192.0.2.1/24")},
			SpeedBps:  1_000_000_000, MTU: 1500, InBps: 12_000_000, OutBps: 3_000_000,
		},
		{Name: "eth1", Kind: "ethernet", AdminUp: true, MTU: 1500, Errors: 7, Drops: 2},
		{Name: "eth2", Kind: "ethernet", MTU: 1500},
	}, nil
}

func (source *deviceTestSource) DevicePeers(_ context.Context, name string) ([]Peer, error) {
	if err := source.reach(name); err != nil {
		return nil, err
	}

	switch name {
	case edgeLon01:
		return []Peer{
			{
				Name: "transit-a", Address: netip.MustParseAddr("198.51.100.1"), Group: "transit", RemoteAS: 64500,
				State: "established", Established: true, Since: testNow.Add(-3 * time.Hour),
				PrefixesReceived: 900, PrefixesSent: 12,
			},
			{
				Name: "transit-b", Address: netip.MustParseAddr("198.51.100.2"), Group: "transit", RemoteAS: 64501,
				State: "idle", Since: testNow.Add(-time.Minute), LastError: "hold timer expired",
			},
		}, nil
	case rsAms01:
		return []Peer{{
			Name: "member-1", Address: netip.MustParseAddr("203.0.113.1"), Group: "members", RemoteAS: 64510,
			State: "established", Established: true, Since: testNow.Add(-time.Hour),
		}}, nil
	}

	return []Peer{}, nil
}

func (source *deviceTestSource) DeviceFamilies(_ context.Context, name string) ([]FamilySummary, error) {
	if err := source.reach(name); err != nil {
		return nil, err
	}

	switch name {
	case edgeLon01:
		return []FamilySummary{{Family: "ipv4/unicast", PeersEstablished: 1, Routes: 900, BestRoutes: 880}}, nil
	case rsAms01:
		return nil, opererr.Wrap(ErrViewUnsupported, rsAms01+" runs version 1.3.9, which does not answer this view.",
			"The command behind this view was added in a later release, so the device has nothing to answer.",
			"open /fleet/device/rs-ams-01/system/")
	}

	return []FamilySummary{}, nil
}

func (source *deviceTestSource) DeviceAdjacencies(_ context.Context, name string) ([]ProtocolAdjacencies, error) {
	if err := source.reach(name); err != nil {
		return nil, err
	}

	if name != edgeLon01 {
		return []ProtocolAdjacencies{}, nil
	}

	return []ProtocolAdjacencies{{Protocol: "OSPF", Rows: []Adjacency{
		{Neighbor: "10.0.0.2", Interface: "eth0", State: "full", Up: true, Since: testNow.Add(-2 * time.Hour)},
	}}}, nil
}

func (source *deviceTestSource) DeviceLogs(_ context.Context, name string) ([]LogLine, error) {
	if err := source.reach(name); err != nil {
		return nil, err
	}

	// Oldest first, so a test sees the tab put the newest first.
	return []LogLine{
		{At: testNow.Add(-3 * time.Hour), Level: "error", Source: "bgp", Message: "session to transit-b reset"},
		{At: testNow.Add(-2 * time.Hour), Level: "warn", Source: "system", Message: "log rotation took 2.1 s"},
		{At: testNow.Add(-time.Hour), Level: "info", Source: "managed", Message: "health report sent to the hub"},
	}, nil
}

func (source *deviceTestSource) Console(_ context.Context, name, _, command string) (ConsoleAnswer, error) {
	if err := source.reach(name); err != nil {
		return ConsoleAnswer{}, err
	}

	if command == "show version" {
		return ConsoleAnswer{Output: testVersionLine + "\n"}, nil
	}

	return ConsoleAnswer{}, opererr.New(`The test console does not answer "`+command+`".`,
		"It answers: help, show version.", "help")
}

// deviceTarget answers the address of a tab of the device named name.
func deviceTarget(name, tab string) string {
	return "/fleet/device/" + name + "/" + tab + "/"
}

// getDevicePage asks the fleet handler over the device test source for
// target and answers the body, after checking the status.
func getDevicePage(t *testing.T, target string) string {
	t.Helper()

	recorder := getFleet(t, newFleetHandler(newDeviceTestSource()), target)
	require.Equal(t, http.StatusOK, recorder.Code, "status of %s: %s", target, recorder.Body.String())

	return recorder.Body.String()
}

// getDeviceFragment asks the fragment handler over the device test source
// for target and answers the body, after checking the status.
func getDeviceFragment(t *testing.T, target string) string {
	t.Helper()

	handler := http.HandlerFunc(newFleetHandler(newDeviceTestSource()).serveFragment)
	recorder := getFleet(t, handler, target)
	require.Equal(t, http.StatusOK, recorder.Code, "status of %s: %s", target, recorder.Body.String())

	return recorder.Body.String()
}

// postConsole posts command to the console of the device named name and
// answers the recorded response.
func postConsole(t *testing.T, name, command string) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{consoleField: {command}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, consoleURL(name), strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()
	newConsoleHandler(newDeviceTestSource()).ServeHTTP(recorder, request)

	return recorder
}

// requireAll asserts that body holds every one of wanted.
func requireAll(t *testing.T, body string, wanted ...string) {
	t.Helper()

	for _, text := range wanted {
		require.Contains(t, body, text)
	}
}

func TestDeviceTabsOfAHealthyDeviceAnswerTheirSections(t *testing.T) {
	for _, tc := range []struct {
		tab    string
		wanted []string
	}{
		{tabDeviceSystem, []string{
			titleIdentity, titleHardware, titleResources, titleHealth, titleLabels, titleManagement,
			"Open console", "ZE1000-0001", "4 cores", "2s ahead of the hub", "https://192.0.2.1:443/", testRelayProfile,
		}},
		{tabDeviceInterfaces, []string{titleInterfaces, "eth0", "eth1", "eth2", "3 of 3 interfaces", deviceToolFormID}},
		{tabDeviceRouting, []string{titlePeers, titleSummary, "OSPF", "transit-a", "hold timer expired"}},
		{tabDeviceLogs, []string{"Recent", "Warnings", "Errors", "Following", "health report sent to the hub"}},
		{tabDeviceConsole, []string{"data-terminal-url=\"" + consoleURL(edgeLon01) + "\"", testRelayProfile, "show bgp peer list"}},
	} {
		t.Run(tc.tab, func(t *testing.T) {
			requireAll(t, getDevicePage(t, deviceTarget(edgeLon01, tc.tab)), tc.wanted...)
		})
	}
}

func TestDeviceTimedSectionsRefreshOnTheirInterval(t *testing.T) {
	system := getDevicePage(t, deviceTarget(edgeLon01, tabDeviceSystem))
	assert.Contains(t, system, `hx-trigger="every 10s"`)
	assert.Contains(t, system, FragmentLink(fragmentDeviceResources, testAddress(t, deviceTarget(edgeLon01, tabDeviceSystem)), ""))

	interfaces := getDevicePage(t, deviceTarget(edgeLon01, tabDeviceInterfaces))
	assert.Contains(t, interfaces, `hx-trigger="every 5s"`)
}

func TestDeviceSectionFragmentAnswersAlone(t *testing.T) {
	page := testAddress(t, deviceTarget(edgeLon01, tabDeviceSystem))
	body := getDeviceFragment(t, FragmentLink(fragmentDeviceResources, page, ""))

	requireAll(t, body, titleResources, "CPU", "4.0 GiB")
	assert.NotContains(t, body, titleIdentity, "a section's fragment answers that section only")
}

func TestDeviceSystemOfAnOfflineDeviceKeepsItsInventory(t *testing.T) {
	body := getDevicePage(t, deviceTarget(coreAms01, tabDeviceSystem))

	requireAll(t, body, "ze-1000", "These values were reported before it was last seen, 6 days ago",
		"fleet-hero--offline", coreAms01+" has been offline since")
}

func TestDeviceRelayedSectionOfAnOfflineDeviceShowsOffline(t *testing.T) {
	body := getDevicePage(t, deviceTarget(coreAms01, tabDeviceInterfaces))

	requireAll(t, body, "fleet-hero--offline", coreAms01+" has been offline since")
	assert.NotContains(t, body, "eth0")
}

func TestDeviceConsoleOfAnOfflineDeviceHasNoTerminal(t *testing.T) {
	body := getDevicePage(t, deviceTarget(coreAms01, tabDeviceConsole))

	requireAll(t, body, coreAms01+" is offline, so it has no console.", "6 days ago")
	assert.NotContains(t, body, "data-terminal-url")
}

func TestDeviceRelayRefusedShowsTheRefusalOnInterfaces(t *testing.T) {
	body := getDevicePage(t, deviceTarget(cpe0113, tabDeviceInterfaces))

	requireAll(t, body, "fleet-hero--relay-refused", testRefusal, testGrantLine)
}

func TestDeviceRelayRefusedShowsTheRefusalOnConsole(t *testing.T) {
	body := getDevicePage(t, deviceTarget(cpe0113, tabDeviceConsole))

	requireAll(t, body, testRefusal, testGrantLine, `data-terminal-refused="true"`, "data-fleet-copy")
	assert.NotContains(t, body, "data-terminal-fill", "a refused console suggests no command")
}

func TestDeviceUnsupportedViewKeepsTheTab(t *testing.T) {
	body := getDevicePage(t, deviceTarget(rsAms01, tabDeviceRouting)+"?sub="+subSummary)

	requireAll(t, body, "fleet-hero--not-supported", "runs version 1.3.9", titlePeers)
}

func TestDeviceRoutingShowsOnlySubTabsTheSourceAnswers(t *testing.T) {
	edge := getDevicePage(t, deviceTarget(edgeLon01, tabDeviceRouting))
	requireAll(t, edge, "sub="+subPeers, "sub="+subSummary, "sub=ospf")

	server := getDevicePage(t, deviceTarget(rsAms01, tabDeviceRouting))
	requireAll(t, server, "sub="+subPeers, "sub="+subSummary)
	assert.NotContains(t, server, "sub=ospf", "a protocol the device does not run has no sub-tab")

	core := getDevicePage(t, deviceTarget(coreLon01, tabDeviceRouting))
	requireAll(t, core, "runs no routing protocol the hub can show")
	assert.NotContains(t, core, "sub="+subPeers)
}

func TestDeviceInterfacesFilterByState(t *testing.T) {
	body := getDevicePage(t, deviceTarget(edgeLon01, tabDeviceInterfaces)+"?q=state:down")

	requireAll(t, body, "counters/eth1", "1 of 3 interfaces")
	assert.NotContains(t, body, "counters/eth0")
}

func TestDeviceInterfacePanelShowsCounters(t *testing.T) {
	body := getDevicePage(t, deviceTarget(edgeLon01, tabDeviceInterfaces)+"?id=eth1")

	requireAll(t, body, "Counters", "Drops", "Link state")
}

func TestDeviceToolAnswersThePrototypeToast(t *testing.T) {
	page := testAddress(t, deviceTarget(edgeLon01, tabDeviceInterfaces))
	target := FragmentLink(fragmentDeviceTool, page, toolCounters+"/eth0")

	requireAll(t, getDeviceFragment(t, target), "The prototype changed nothing", "Counters detail of eth0 on "+edgeLon01)
}

func TestDeviceLogsErrorsKeepErrorsNewestFirst(t *testing.T) {
	body := getDevicePage(t, deviceTarget(edgeLon01, tabDeviceLogs)+"?sub="+subErrors)

	requireAll(t, body, "session to transit-b reset")
	assert.NotContains(t, body, "health report sent to the hub")
	assert.NotContains(t, body, "Following", "only Recent follows the log")
}

func TestDeviceLogsRecentPutsTheNewestFirst(t *testing.T) {
	body := getDevicePage(t, deviceTarget(edgeLon01, tabDeviceLogs))

	newest := strings.Index(body, "health report sent to the hub")
	oldest := strings.Index(body, "session to transit-b reset")
	require.True(t, newest >= 0 && oldest >= 0)
	assert.Less(t, newest, oldest)
}

func TestDeviceConsoleRunsAKnownCommand(t *testing.T) {
	recorder := postConsole(t, edgeLon01, "show   version")

	require.Equal(t, http.StatusOK, recorder.Code)
	requireAll(t, recorder.Body.String(), "show version", testVersionLine)
}

func TestDeviceConsoleRefusesAnUnknownCommandWithTheList(t *testing.T) {
	recorder := postConsole(t, edgeLon01, "reboot")

	require.Equal(t, http.StatusOK, recorder.Code)
	requireAll(t, recorder.Body.String(), "The test console does not answer", "It answers: help, show version.")
	assert.Contains(t, recorder.Body.String(), `data-terminal-refused="false"`)
}

func TestDeviceConsoleRefusesConfigurationMode(t *testing.T) {
	recorder := postConsole(t, edgeLon01, "configure")

	requireAll(t, recorder.Body.String(), "Configuration mode is not offered", deviceTarget(edgeLon01, "config"))
}

func TestDeviceConsoleOfARefusingDeviceClosesTheTerminal(t *testing.T) {
	recorder := postConsole(t, cpe0113, "show version")

	requireAll(t, recorder.Body.String(), testRefusal, `data-terminal-refused="true"`)
}

// TestDeviceConsoleRefusesAnAddressWithNoDevice posts to a console address
// that names no device.
//
// VALIDATES: the refusal is a terminal entry under a success status, with
// the problem and the fix, as every refusal of the console route is.
// PREVENTS: a plain text refusal the terminal cannot append, which the
// operator sees as an answer with no output.
func TestDeviceConsoleRefusesAnAddressWithNoDevice(t *testing.T) {
	recorder := postConsole(t, "", "show version")

	require.Equal(t, http.StatusOK, recorder.Code)
	requireAll(t, recorder.Body.String(), "fleet-terminal-entry", "names no device", "Try: open the device&#39;s Console tab")
}

// TestDeviceConsoleRefusesAnOverlongFormAsAnEntry posts a form longer than
// the route reads.
//
// VALIDATES: the refusal is a terminal entry under a success status, with
// the bound and the fix.
// PREVENTS: a 400 in plain text the terminal cannot append.
func TestDeviceConsoleRefusesAnOverlongFormAsAnEntry(t *testing.T) {
	recorder := postConsole(t, edgeLon01, strings.Repeat("a", consoleFormBytesMax+1))

	require.Equal(t, http.StatusOK, recorder.Code)
	requireAll(t, recorder.Body.String(), "fleet-terminal-entry", "The command could not be read.", "Try: type a shorter command")
}

// TestDeviceConsoleCutsAnOverlongCommandOnACharacter posts a command of
// two-byte characters longer than the console sends on.
//
// VALIDATES: the echoed command is cut after consoleCommandMax characters,
// never inside one.
// PREVENTS: an echo that ends in half a character.
func TestDeviceConsoleCutsAnOverlongCommandOnACharacter(t *testing.T) {
	recorder := postConsole(t, edgeLon01, strings.Repeat("é", consoleCommandMax+1))

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.True(t, utf8.ValidString(body), "the echo holds whole characters only")
	assert.Contains(t, body, strings.Repeat("é", consoleCommandMax))
	assert.NotContains(t, body, strings.Repeat("é", consoleCommandMax+1))
	assert.Contains(t, body, "The command is too long.")
}

// TestSortHeaderMarksTheDefaultColumn draws the header of a device table's
// default column on an address that names no sort column.
//
// VALIDATES: the default column is marked sorted, ascending or descending
// as the address says, and its first click turns the order around.
// PREVENTS: a table sorted by a column no header marks, whose first click
// sorts it the same way again.
func TestSortHeaderMarksTheDefaultColumn(t *testing.T) {
	address := testAddress(t, deviceTarget(edgeLon01, tabDeviceInterfaces))
	address.View.Sort = SortSpec{}
	first := interfaceColumns.resolved(SortSpec{}).Column
	require.NotEmpty(t, first)

	header := sortHeader(interfaceColumns, address, "First", ColumnKindText, first, false)
	assert.Equal(t, SortMarkAscending, header.Sort)
	want := address
	want.View.Sort = SortSpec{Column: first, Descending: true}
	want.View.Page = 1
	want.View.Row = ""
	assert.Equal(t, want.String(), header.SortHref, "the first click sorts the default column descending")

	address.View.Sort = SortSpec{Descending: true}
	descending := sortHeader(interfaceColumns, address, "First", ColumnKindText, first, false)
	assert.Equal(t, SortMarkDescending, descending.Sort)

	for _, key := range interfaceColumns.Keys() {
		if key != first {
			other := sortHeader(interfaceColumns, address, "Other", ColumnKindText, key, false)
			assert.Zero(t, other.Sort, "only the default column is marked")
		}
	}
}

// fixCount counts the fix lines of the refusals markup shows.
func fixCount(markup string) int {
	return strings.Count(markup, "Fix: ")
}

// TestDeviceTablesShowEveryRefusalWithItsFix asks the interfaces, peers and
// logs tables for a filter and a sort they refuse.
//
// VALIDATES: each refusal is shown with its fix, and a refused filter and a
// refused sort are both shown.
// PREVENTS: a refusal that names the problem and not the fix, and a sort
// refusal that hides why the rows are unfiltered.
func TestDeviceTablesShowEveryRefusalWithItsFix(t *testing.T) {
	badFilter := url.Values{paramFilter: {"nosuchfield:x"}}.Encode()
	both := url.Values{paramFilter: {"nosuchfield:x"}, paramSort: {"nosuchcolumn"}}.Encode()

	for _, tab := range []string{tabDeviceInterfaces, tabDeviceRouting} {
		t.Run(tab, func(t *testing.T) {
			base := fixCount(getDevicePage(t, deviceTarget(edgeLon01, tab)))
			filtered := fixCount(getDevicePage(t, deviceTarget(edgeLon01, tab)+"?"+badFilter))
			sorted := fixCount(getDevicePage(t, deviceTarget(edgeLon01, tab)+"?"+both))

			assert.Equal(t, base+1, filtered, "the filter refusal names its fix")
			assert.Equal(t, base+2, sorted, "the filter and the sort refusals are both shown")
		})
	}

	logs := deviceTarget(edgeLon01, tabDeviceLogs)
	assert.Equal(t, fixCount(getDevicePage(t, logs))+1, fixCount(getDevicePage(t, logs+"?"+badFilter)))
}

// TestDeviceErrorsNameTheirFix asks for an unknown interface tool and for
// the clock of a device that refuses relayed commands.
//
// VALIDATES: the toast and the clock row carry the refusal's fix.
// PREVENTS: an operator told what failed and not what to do.
func TestDeviceErrorsNameTheirFix(t *testing.T) {
	page := testAddress(t, deviceTarget(edgeLon01, tabDeviceInterfaces))
	toast := getDeviceFragment(t, FragmentLink(fragmentDeviceTool, page, "nosuchtool/eth0"))
	assert.Contains(t, toast, "Fix: choose a tool from the row")

	request := &PageRequest{Source: newDeviceTestSource()}
	rows := clockRows(t.Context(), request, cpe0113)
	require.Len(t, rows, 1)
	assert.Contains(t, rows[0].Text, "Fix: "+testGrantLine)
}
