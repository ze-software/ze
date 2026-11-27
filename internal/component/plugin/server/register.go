// Design: docs/architecture/api/process-protocol.md — RPC registration
// Detail: system.go — system and daemon command handlers
// Detail: session.go — session protocol handlers
// Detail: plugin_rpc.go — plugin introspection handlers
//
// RPC registration uses init()-based self-registration. Each handler file
// registers its own RPCs via init() + RegisterRPCs(). External packages
// (e.g., handler, editor) call RegisterRPCs() from their own init().

package server

import (
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// serverEventCodes are the report-bus codes the plugin server raises as
// events, which the recent-events doctor check reports (see
// docs/architecture/doctor-and-health-checks.md, "Event codes"): a payload or
// state only a defect produces, a refused plugin request, an answer or a
// queue that never reached its plugin, a config parent left behind, a
// deprecated command name, a managed client never told of a change, a
// managed configuration hash chain that did not converge after resync, a
// configuration reload the server refused, a restarted external plugin whose
// YANG modules are not the ones ze started with (stage1_schema.go), an external
// plugin that gave boot no config schema and a module boot refused at the join
// (external_schema_report.go), and an external plugin's connect-back refused by
// the acceptor the server starts (plugin/ipc tls.go; ipc cannot import
// diagnostic without a cycle).
var serverEventCodes = []diagnostic.EventCode{
	{Code: ipc.ConnectBackRefusedCode},
	{Code: diagnostic.CodeDoctorInternalDefect},
	{Code: diagnostic.CodeDoctorPluginRequestRefused},
	{Code: CodeDoctorPluginAnswerUndelivered},
	{Code: CodeDoctorPluginDrainIncomplete},
	{Code: CodeDoctorCommandEnsureRollbackFailed},
	{Code: CodeDoctorCommandDeprecatedUsed},
	{Code: CodeDoctorManagedClientNotifyDropped},
	{Code: CodeDoctorManagedChainDrift},
	{Code: CodeDoctorConfigReloadRefused},
	{Code: CodeDoctorPluginSchemaChanged},
	{Code: CodeDoctorPluginSchemaUndeclared},
	{Code: CodeDoctorPluginSchemaRefused},
	{Code: diagnostic.CodeDoctorBuildRegistrationUnheld},
}

func init() {
	registerDoctorCodes()
	registerManagedReportReceivers()
	for _, event := range serverEventCodes {
		if err := diagnostic.RegisterEventCode(event); err != nil {
			panic("BUG: plugin server event code registration refused: " + err.Error())
		}
	}
	// A plugin's proxy enrichers and their doctor failures go with its process.
	RegisterProcessCleanup(unregisterProxyEnrichers)
	// The config roots an external plugin declared stop counting as delivered
	// when its process ends (external_roots.go).
	RegisterProcessCleanup(forgetExternalConfigRoots)
}
