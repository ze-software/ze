// VALIDATES: Sender keeps three states apart -- the operator, one named
// process, and nobody -- and no input to its constructors collapses one into
// another.
// PREVENTS: an unnamed issuer reading as the operator, which would hand the
// operator's authority to any dispatch path that forgot to name its sender
// (internal/component/bgp/reactor/send_permission.go).

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSenderZeroValueNamesNobody(t *testing.T) {
	var s Sender

	assert.False(t, s.IsSet(), "the zero Sender must report that nobody named the issuer")
	assert.False(t, s.IsOperator(), "the zero Sender must not read as the operator")
	name, ok := s.Process()
	assert.False(t, ok, "the zero Sender names no process")
	assert.Empty(t, name)
	assert.Equal(t, "unset", s.String(), "a log line and a metric label need a bounded name for it")
}

func TestSenderOperatorAndProcessAreDistinct(t *testing.T) {
	op := OperatorSender()
	require.True(t, op.IsSet())
	assert.True(t, op.IsOperator())
	_, ok := op.Process()
	assert.False(t, ok, "the operator is not a process, so no attach block matches it")
	assert.Equal(t, "operator", op.String())

	proc := ProcessSender("bgp-rib")
	require.True(t, proc.IsSet())
	assert.False(t, proc.IsOperator(), "a process must never read as the operator")
	name, ok := proc.Process()
	require.True(t, ok)
	assert.Equal(t, "bgp-rib", name, "the name is what a peer's attach block is matched against")
	assert.Equal(t, "bgp-rib", proc.String())
}

// TestProcessSenderRefusesANameItCannotTrust is the guard, and it is driven with
// the two inputs a caller can supply that would otherwise be granted something.
//
// An empty name is the omission the type exists to catch. The reserved operator
// name is the one string that would promote a process to the operator, so the
// constructor refuses it and the guarantee holds whatever a config parser
// accepts (ai/rules/evidence.md).
func TestProcessSenderRefusesANameItCannotTrust(t *testing.T) {
	empty := ProcessSender("")
	assert.False(t, empty.IsSet(), "a process nobody named must yield the zero Sender")
	assert.False(t, empty.IsOperator())

	forged := ProcessSender(operatorName)
	assert.False(t, forged.IsOperator(), "no process may present itself as the operator")
	assert.False(t, forged.IsSet(), "the refused name yields the zero Sender, which every guard denies")
}
