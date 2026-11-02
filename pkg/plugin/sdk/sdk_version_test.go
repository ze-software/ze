package sdk

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAcceptProtocolVersion pins the Stage 1 answers this SDK accepts: exactly
// rpc.ProtocolVersion, and nothing else. The method: acceptProtocolVersion
// reads the answer naming rpc.ProtocolVersion, then an empty answer, an empty
// object, version 0 and a version other than rpc.ProtocolVersion.
//
// VALIDATES: the SDK speaks only the version it was built for.
// PREVENTS: a plugin speaking to an engine whose contract neither side
// checked, such as one from before the versioned contract or a later release.
func TestAcceptProtocolVersion(t *testing.T) {
	t.Parallel()

	accepted := []byte(`{"protocol-version":` + strconv.Itoa(rpc.ProtocolVersion) + `}`)
	assert.NoError(t, acceptProtocolVersion(accepted), "version %d", rpc.ProtocolVersion)

	other := `{"protocol-version":` + strconv.Itoa(rpc.ProtocolVersion+1) + `}`
	for _, answer := range []string{``, `{}`, `{"protocol-version":0}`, other} {
		assert.Error(t, acceptProtocolVersion([]byte(answer)), "answer %q", answer)
	}
}
