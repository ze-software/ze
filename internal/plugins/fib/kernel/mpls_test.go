package fibkernel

import (
	"testing"

	"github.com/stretchr/testify/assert"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
)

func TestMPLSConstants(t *testing.T) {
	assert.Equal(t, uint32(1048575), uint32(maxMPLSLabel))
	assert.Equal(t, 16, mplsfibevents.MaxLabelStack)
}
