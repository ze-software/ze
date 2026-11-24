// Design: docs/architecture/api/process-protocol.md -- firewall table contribution
// Related: contribution.go -- the reservation checks, the replacement and the withdrawal
// Related: doctor_contribution.go -- the doctor check and the warning-clearing observer
// Related: ../../../pkg/plugin/rpc/firewall_tables.go -- the request values
//
// The table registry is process-local, so a producer that can run forked
// reaches it through this engine operation, never through RegisterTables. The
// owner of every contributed table is the authenticated calling process, and
// the tables of a process that ended are withdrawn by its generation cleanup.

package firewall

import (
	"context"
	"encoding/json"

	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func init() {
	// ConfigureSafe: an owner contributes its tables from its configure stage,
	// and the handler answers within ctx.
	operation := pluginserver.EngineOperation{
		Method:          rpc.MethodFirewallTablesReplace,
		Handler:         tablesReplaceHandler,
		Startup:         pluginserver.EngineOperationConfigureSafe,
		BusExemptReason: "governed by compile-time firewall.MustReserveTableNames grants keyed on compiled registration identity",
	}
	if err := pluginserver.RegisterEngineOp(operation); err != nil {
		panic("BUG: firewall table contribution registration failed: " + err.Error())
	}
	if err := pluginserver.RegisterProcessGenerationCleanup(func(_ *pluginserver.Server, proc *process.Process) {
		withdrawContributedTables(proc)
	}); err != nil {
		panic("BUG: firewall table contribution cleanup registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterDoctorCheck(contributionDoctorCheck); err != nil {
		panic("BUG: firewall contributed-tables doctor check registration failed: " + err.Error())
	}
	RegisterCommitObserver(clearContributionWarnings)
}

func tablesReplaceHandler(_ *pluginserver.Server, _ context.Context, proc *process.Process, params json.RawMessage) (any, error) {
	owner := proc.RegistrationName()
	endpoint := proc.Name()
	if err := refuseOwnerWithoutReservation(owner, endpoint); err != nil {
		return nil, err
	}
	var input rpc.FirewallTablesReplaceInput
	if err := pluginserver.DecodeEngineOperationInput(params, &input, "Firewall table contribution"); err != nil {
		return nil, err
	}
	if err := refuseSectionWithoutTableMap(endpoint, input.Section); err != nil {
		return nil, err
	}
	tables, err := ParseFirewallConfig(string(input.Section))
	if err != nil {
		return nil, opererr.Wrap(err,
			"Plugin "+endpoint+" contributed a firewall section that does not parse",
			"The section takes the firewall config form, and no table of a refused contribution is applied",
			"Send the section in the wrapped firewall config form, {\"firewall\":{\"table\":{...}}}, with each table as the firewall YANG module declares it",
		)
	}
	if err := refuseUnreservedContribution(owner, endpoint, tables); err != nil {
		return nil, err
	}
	output, err := replaceContributedTables(proc, tables)
	if err != nil {
		return nil, err
	}
	return output, nil
}

// refuseSectionWithoutTableMap refuses a section that holds no firewall table
// map. ParseFirewallConfig answers no table both for that and for an empty
// map, and a contribution with no table withdraws every table its owner
// contributed, so a malformed section must not read as a withdrawal. An
// explicit empty map, {"firewall":{"table":{}}}, is the withdrawal.
func refuseSectionWithoutTableMap(owner string, section json.RawMessage) error {
	var shape struct {
		Firewall *struct {
			Table map[string]json.RawMessage `json:"table"`
		} `json:"firewall"`
	}
	const why = "A contribution replaces the owner's complete set, so a section with no table map would withdraw every table"
	const fix = "Send the section as {\"firewall\":{\"table\":{...}}}, with an empty table map to withdraw every table"
	if err := json.Unmarshal(section, &shape); err != nil {
		return opererr.Wrap(err, "Plugin "+owner+" contributed a firewall section that is not a JSON object", why, fix)
	}
	if shape.Firewall == nil {
		return opererr.New("Plugin "+owner+" contributed a section with no firewall object", why, fix)
	}
	if shape.Firewall.Table == nil {
		return opererr.New("Plugin "+owner+" contributed a firewall section with no table map", why, fix)
	}
	return nil
}
