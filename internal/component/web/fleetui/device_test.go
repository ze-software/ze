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
	request := httptest.NewRequest(http.MethodPost, consoleURL(name), strings.NewReader(form.Encode()))
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

func TestDeviceConsoleRefusesAnAddressWithNoDevice(t *testing.T) {
	recorder := postConsole(t, "", "show version")

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}
