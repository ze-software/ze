package fleetui

import (
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/golden"
)

// goldenDeviceRequest builds the request of the device page at target over
// the device test source.
func goldenDeviceRequest(t *testing.T, target string) *PageRequest {
	t.Helper()

	request, err := NewPageRequest(context.Background(), newDeviceTestSource(), testAddress(t, target), testUser)
	require.NoError(t, err)

	return request
}

// goldenDeviceContent builds the content of the device page at target.
func goldenDeviceContent(t *testing.T, target string, build PageBuilder) templ.Component {
	t.Helper()

	content, err := build(context.Background(), goldenDeviceRequest(t, target))
	require.NoError(t, err)

	return content
}

// goldenDeviceView answers the device view of the request's device.
func goldenDeviceView(t *testing.T, request *PageRequest) *DeviceView {
	t.Helper()

	view, err := deviceOf(request)
	require.NoError(t, err)

	return view
}

// goldenSpecDevice is the capture plan of page_device.templ. Every case is
// built by the code a request runs, over the device test source of
// device_test.go.
func goldenSpecDevice(t *testing.T) golden.Spec {
	t.Helper()

	system := goldenDeviceRequest(t, deviceTarget(edgeLon01, tabDeviceSystem))
	offline := goldenDeviceRequest(t, deviceTarget(coreAms01, tabDeviceSystem))
	logs := goldenDeviceRequest(t, deviceTarget(edgeLon01, tabDeviceLogs))
	ctx := context.Background()

	return golden.Spec{
		"page_device.templ": {
			{Name: "deviceTab", Variants: []golden.Variant{
				{Name: "system", Data: goldenDeviceContent(t, deviceTarget(edgeLon01, tabDeviceSystem), systemContent)},
				{Name: "interfaces", Data: goldenDeviceContent(t, deviceTarget(edgeLon01, tabDeviceInterfaces), interfacesContent)},
				{Name: "routing", Data: goldenDeviceContent(t, deviceTarget(edgeLon01, tabDeviceRouting), routingContent)},
				{Name: "routing-unsupported", Data: goldenDeviceContent(t, deviceTarget(rsAms01, tabDeviceRouting)+"?sub="+subSummary, routingContent)},
				{Name: "logs-recent", Data: goldenDeviceContent(t, deviceTarget(edgeLon01, tabDeviceLogs), logsContent)},
				{Name: "interfaces-refused", Data: goldenDeviceContent(t, deviceTarget(cpe0113, tabDeviceInterfaces), interfacesContent)},
			}},
			{Name: "deviceSubTabs", Variants: []golden.Variant{
				{Data: deviceSubTabs(*subTabsView(logs.Address, "Log views", logSubKeys, logSubTitles, subWarnings))},
			}},
			{Name: "deviceSection", Variants: []golden.Variant{
				{Name: "timed", Data: deviceSection(DeviceSectionView{
					Title: titleResources, URL: sectionLink(system, fragmentDeviceResources), Every: everyResources, Wide: true,
					Body: resourcesSection(ctx, system, goldenDeviceView(t, system)),
				})},
				{Name: "untimed", Data: deviceSection(DeviceSectionView{
					Title: titleLabels, URL: sectionLink(system, fragmentDeviceLabels),
					Body: labelsSection(ctx, system, goldenDeviceView(t, system)),
				})},
			}},
			{Name: "deviceFacts", Variants: []golden.Variant{
				{Name: "management", Data: managementSection(ctx, system, goldenDeviceView(t, system))},
				{Name: "health", Data: healthSection(ctx, system, goldenDeviceView(t, system))},
				{Name: "offline-identity", Data: identitySection(ctx, offline, goldenDeviceView(t, offline))},
				{Name: "offline-hardware", Data: hardwareSection(ctx, offline, goldenDeviceView(t, offline))},
			}},
			{Name: "deviceConsole", Variants: []golden.Variant{
				{Name: "granted", Data: goldenDeviceContent(t, deviceTarget(edgeLon01, tabDeviceConsole), consoleContent)},
				{Name: "refused", Data: goldenDeviceContent(t, deviceTarget(cpe0113, tabDeviceConsole), consoleContent)},
			}},
		},
	}
}
