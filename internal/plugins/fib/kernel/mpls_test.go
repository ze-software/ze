package fibkernel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMPLSConstants(t *testing.T) {
	assert.Equal(t, uint32(1048575), uint32(maxMPLSLabel))
	assert.Equal(t, 16, maxLabelStack)
}
