// VALIDATES: a managed client tells its owner, after each fetch the hub
// answered, whether it applied nothing because the device's configuration
// differs from its baseline. `show fleet status` reports that answer.
// PREVENTS: a daemon whose status says "not-holding" for a client that holds.

package managed

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// answerOffer is a hub that holds holdHubConfig: it returns that configuration
// to a fetch and accepts every other request, the acknowledgement among them.
func answerOffer(req *rpc.Request) any {
	if req.Method != fleet.VerbConfigFetch {
		return struct{}{}
	}
	return fleet.ConfigFetchResponse{
		Version: fleet.VersionHash([]byte(holdHubConfig)),
		Config:  base64.StdEncoding.EncodeToString([]byte(holdHubConfig)),
	}
}

// TestClientReportsHoldToItsOwner proves the OnHold callback: a client over a
// store whose configuration differs from its baseline fetches once from a hub
// that offers another configuration, and once from a hub that answers
// "current", and the test reads what each fetch reported.
//
// VALIDATES: AC-15, "whether a fetch is being held", at its producer
// (fetchAgainstBaseline).
// PREVENTS: a hold that is logged and acknowledged and never reaches the
// daemon's status, and a hold that stays reported after the hub agreed.
func TestClientReportsHoldToItsOwner(t *testing.T) {
	for name, want := range map[string]struct {
		answer  func(*rpc.Request) any
		holding bool
	}{
		"the hub offers another configuration": {answerOffer, true},
		"the hub holds the same configuration": {answerCurrent, false},
	} {
		t.Run(name, func(t *testing.T) {
			store := holdDevice(t, holdDeviceConfig, holdEarlierConfig)
			cfg, _, applied := holdClient("192.0.2.1:1791", NewBaselineAccess(store))
			reported := make(chan bool, 4)
			cfg.OnHold = func(holding bool) { reported <- holding }
			mc, _ := holdPipe(t, want.answer)

			require.NoError(t, fetchAndProcess(context.Background(), mc, &cfg))

			require.Len(t, reported, 1, "one fetch reports once")
			assert.Equal(t, want.holding, <-reported)
			assert.Empty(t, applied, "a device that differs from its baseline applied a configuration")
		})
	}
}
