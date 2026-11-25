// Design: docs/architecture/doctor-and-health-checks.md -- plugin registration and delivery checks
// Related: checks_delivery.go -- the row builders these checks run
//
// The registration and delivery checks register here, apart from
// register.go, so the checks of each problem class sit with their own
// registration. Each reads the running server's plugin snapshot, so each runs
// at the runtime phase, which only the daemon runs.

package doctor

import (
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// The registration and delivery check names.
const (
	pluginCommandsCheckName      = "plugin-commands"
	pluginSubscriptionsCheckName = "plugin-subscriptions"
	pluginDeliveryGrantCheckName = "plugin-delivery-agreement"
	pluginClaimsCheckName        = "plugin-claims"
	pluginPostStartupCheckName   = "plugin-poststartup"
	pluginPanicCheckName         = "plugin-panic"
	pluginDeliveryCheckName      = "plugin-delivery"
	pluginEnricherCheckName      = "plugin-enrichers"
	pluginConfigCommitCheckName  = "plugin-config-commit"
	pluginBusDormantCheckName    = "plugin-bus-dormant-receptions"
	pluginDeliveryOrderFirst     = 730
	pluginStateDependency        = "plugin-state"
)

func init() {
	checks := []diagnostic.DoctorCheck{
		{
			Name: pluginCommandsCheckName, Phase: diagnostic.DoctorPhaseRuntime, Order: pluginDeliveryOrderFirst,
			Component: componentPlugin, Dependencies: []string{pluginStateDependency},
			Platforms: []string{diagnostic.DoctorPlatformAny},
			Codes:     []string{server.CodeDoctorPluginCommandNotRegistered},
			Check:     snapshotCheck(server.CodeDoctorPluginCommandNotRegistered, commandRefusalRows),
		},
		{
			Name: pluginSubscriptionsCheckName, Phase: diagnostic.DoctorPhaseRuntime, Order: pluginDeliveryOrderFirst + 1,
			Component: componentPlugin, Dependencies: []string{pluginStateDependency},
			Platforms: []string{diagnostic.DoctorPlatformAny},
			Codes:     []string{server.CodeDoctorPluginSubscriptionDropped},
			Check:     snapshotCheck(server.CodeDoctorPluginSubscriptionDropped, subscriptionDropRows),
		},
		{
			Name: pluginDeliveryGrantCheckName, Phase: diagnostic.DoctorPhaseRuntime, Order: pluginDeliveryOrderFirst + 2,
			Component: componentPlugin, Dependencies: []string{pluginStateDependency},
			Platforms: []string{diagnostic.DoctorPlatformAny},
			Codes:     []string{server.CodeDoctorPluginDeliveryDisagreement},
			Check:     snapshotCheck(server.CodeDoctorPluginDeliveryDisagreement, deliveryDisagreementRows),
		},
		{
			Name: pluginClaimsCheckName, Phase: diagnostic.DoctorPhaseRuntime, Order: pluginDeliveryOrderFirst + 3,
			Component: componentPlugin, Dependencies: []string{pluginStateDependency},
			Platforms: []string{diagnostic.DoctorPlatformAny},
			Codes:     []string{server.CodeDoctorPluginClaimUnbacked},
			Check:     snapshotCheck(server.CodeDoctorPluginClaimUnbacked, unbackedClaimRows),
		},
		{
			Name: pluginPostStartupCheckName, Phase: diagnostic.DoctorPhaseRuntime, Order: pluginDeliveryOrderFirst + 4,
			Component: componentPlugin, Dependencies: []string{pluginStateDependency},
			Platforms: []string{diagnostic.DoctorPlatformAny},
			Codes:     []string{server.CodeDoctorPluginPostStartupUndelivered},
			Check:     snapshotCheck(server.CodeDoctorPluginPostStartupUndelivered, postStartupRows),
		},
		{
			Name: pluginPanicCheckName, Phase: diagnostic.DoctorPhaseRuntime, Order: pluginDeliveryOrderFirst + 5,
			Component: componentPlugin, Dependencies: []string{pluginStateDependency},
			Platforms: []string{diagnostic.DoctorPlatformAny},
			Codes:     []string{diagnostic.CodeDoctorPluginPanic},
			Check:     snapshotCheck(diagnostic.CodeDoctorPluginPanic, panicRows),
		},
		{
			Name: pluginDeliveryCheckName, Phase: diagnostic.DoctorPhaseRuntime, Order: pluginDeliveryOrderFirst + 6,
			Component: componentPlugin, Dependencies: []string{pluginStateDependency},
			Platforms: []string{diagnostic.DoctorPlatformAny},
			Codes:     []string{process.CodeDoctorPluginDeliveryFailed},
			Check:     snapshotCheck(process.CodeDoctorPluginDeliveryFailed, deliveryFailedRows),
		},
		{
			Name: pluginEnricherCheckName, Phase: diagnostic.DoctorPhaseRuntime, Order: pluginDeliveryOrderFirst + 7,
			Component: componentPlugin, Dependencies: []string{pluginStateDependency},
			Platforms: []string{diagnostic.DoctorPlatformAny},
			Codes:     []string{server.CodeDoctorPluginEnricherUnavailable},
			Check:     snapshotCheck(server.CodeDoctorPluginEnricherUnavailable, enricherRows),
		},
		{
			Name: pluginConfigCommitCheckName, Phase: diagnostic.DoctorPhaseRuntime, Order: pluginDeliveryOrderFirst + 8,
			Component: componentPlugin, Dependencies: []string{pluginStateDependency},
			Platforms: []string{diagnostic.DoctorPlatformAny},
			Codes:     []string{server.CodeDoctorPluginConfigUnapplied},
			Check:     snapshotCheck(server.CodeDoctorPluginConfigUnapplied, configUnappliedRows),
		},
		{
			Name: pluginBusDormantCheckName, Phase: diagnostic.DoctorPhaseRuntime, Order: pluginDeliveryOrderFirst + 9,
			Component: componentPlugin, Dependencies: []string{pluginStateDependency},
			Platforms: []string{diagnostic.DoctorPlatformAny},
			Codes:     []string{codeDoctorBusDormantReception},
			Check:     snapshotCheck(codeDoctorBusDormantReception, dormantReceptionRows),
		},
	}
	for i := range checks {
		if err := diagnostic.RegisterDoctorCheck(checks[i]); err != nil {
			panic("BUG: plugin: doctor check registration (" + checks[i].Name + "): " + err.Error())
		}
	}
}
