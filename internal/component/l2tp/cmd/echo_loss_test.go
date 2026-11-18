// Design: docs/guide/l2tp.md -- echo quality availability through the registered command

package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	l2tp "github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// This read-only DTO fixture constructs no captured session or native owner.
// Explicit and absent JSON loss values decode before and after optionality.
type cqmEchoReadService struct {
	fakeService
	echo l2tp.LoginEchoState
}

func (service *cqmEchoReadService) EchoState(login string) *l2tp.LoginEchoState {
	if login != service.echo.Login {
		return nil
	}
	return &service.echo
}

// TestEchoCommandKeepsMeasuredZeroAndOmitsUnknownLoss reaches the registered RPC
// with known nonzero, known zero and unavailable read-only quality observations.
// MUTATION: interval presence alone emits a fabricated healthy loss for 101s.
func TestEchoCommandKeepsMeasuredZeroAndOmitsUnknownLoss(t *testing.T) {
	registrations := pluginserver.AllBuiltinRPCs()
	var echo *pluginserver.RPCRegistration
	for index := range registrations {
		if registrations[index].WireMethod == "ze-l2tp:echo" {
			require.Nil(t, echo, "one registered command owns the wire method")
			echo = &registrations[index]
		}
	}
	require.NotNil(t, echo, "the real echo command must be registered")
	require.NotNil(t, echo.Handler)
	for _, test := range []struct {
		name      string
		stateJSON string
		interval  string
		lossKnown bool
		loss      float64
	}{
		{
			"1s", `{"Login":"alice","LastRTT":2000000,"BucketState":"negotiating",
			"EchoInterval":1000000000,"LossRatio":0.99}`, "1s", true, 0.99,
		},
		{
			"100s", `{"Login":"alice","LastRTT":2000000,"BucketState":"negotiating",
			"EchoInterval":100000000000,"LossRatio":0}`, "1m40s", true, 0,
		},
		{
			"101s", `{"Login":"alice","LastRTT":2000000,"BucketState":"negotiating",
			"EchoInterval":101000000000}`, "1m41s", false, 0,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var service cqmEchoReadService
			require.NoError(t, json.Unmarshal([]byte(test.stateJSON), &service.echo))
			publishFake(t, &service)
			response, err := echo.Handler(nil, commandtest.Args("alice"))
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, plugin.StatusDone, response.Status)
			var data map[string]any
			require.NoError(t, json.Unmarshal([]byte(responseString(t, response)), &data))
			assert.Equal(t, "alice", data["login"])
			assert.Equal(t, test.interval, data["echo-interval"])
			assert.Equal(t, 2.0, data["last-rtt-ms"])
			assert.Equal(t, "negotiating", data["bucket-state"])
			if test.lossKnown {
				require.Contains(t, data, "loss-ratio", "computed zero must remain present")
				assert.Equal(t, test.loss, data["loss-ratio"])
			} else {
				assert.NotContains(t, data, "loss-ratio", "unavailable loss must remain absent")
			}
		})
	}
}
