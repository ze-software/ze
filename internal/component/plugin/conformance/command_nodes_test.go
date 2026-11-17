package conformance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/externalyang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// commandModulesDeclaration is the declaration of the external-yang test
// plugin's `commands` variant (internal/test/fixture): the config module, the
// command module and its rpc module, with the commands names.
func commandModulesDeclaration(names ...string) *rpc.DeclareRegistrationInput {
	declaration := &rpc.DeclareRegistrationInput{ProtocolVersion: rpc.ProtocolVersion}
	for _, module := range []externalyang.Module{externalyang.Valid(), externalyang.Commands(), externalyang.API()} {
		declaration.Schemas = append(declaration.Schemas, rpc.SchemaDecl{
			Module: module.Name, Namespace: module.Namespace, YANGText: module.Text,
		})
	}
	for _, name := range names {
		declaration.Commands = append(declaration.Commands, rpc.CommandDecl{Name: name})
	}
	return declaration
}

// VALIDATES: the checker judges command-nodes-served for a program in
// isolation: it joins the modules the declaration carries and passes the
// fixture's `commands` declaration, and fails, with the daemon's refusal and a
// fix, a declaration whose module holds a command node no declared command
// reaches.
// PREVENTS: a plugin whose module brings a command it never serves passing
// `ze plugin conformance`, and the check reported as needing a daemon.
func TestRunJudgesTheCommandNodesOfTheDeclaredModules(t *testing.T) {
	const check = "command-nodes-served"

	served := runTarget(t, writes(string(declarationBytes(t,
		commandModulesDeclaration(externalyang.CommandStatus, externalyang.CommandTarget)))), testBudget)
	row := rowNamed(t, served, check)
	assert.Equal(t, VerdictPass, row.Verdict, row.Reason)
	assert.Equal(t, stageOne, row.Stage)

	unserved := runTarget(t, writes(string(declarationBytes(t,
		commandModulesDeclaration(externalyang.CommandStatus)))), testBudget)
	row = rowNamed(t, unserved, check)
	require.Equal(t, VerdictFail, row.Verdict, "reason %q", row.Reason)
	assert.Equal(t, stageOne, row.Stage)
	assert.Contains(t, row.Reason, `plugin "conformance-subject" brings command "`+externalyang.CommandTarget+
		`" in module "acme-monitor-cmd", and declares no command at Stage 1 that reaches it`)
	assert.NotEmpty(t, row.Fix, "a failed row names the change that fixes it")
	assert.Equal(t, ExitViolation, unserved.ExitCode())
}
