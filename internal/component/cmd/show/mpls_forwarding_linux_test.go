//go:build linux

// Design: plan/spec-mpls-1-kernel.md -- MPLS operation classifier tests
package show

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMPLSOperationClassifier(t *testing.T) {
	assert.Equal(t, "pop", mplsOperation(nil), "no outgoing label is disposition")
	assert.Equal(t, "pop", mplsOperation([]int{mplsImplicitNull}), "implicit-null only is pop")
	assert.Equal(t, "swap", mplsOperation([]int{16001}), "single real label is swap")
	assert.Equal(t, "swap", mplsOperation([]int{16001, 16002}), "label stack is swap")
}
