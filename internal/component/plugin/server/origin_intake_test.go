// Design: docs/architecture/api/process-protocol.md -- original deciding connection.
package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestOriginIntakeDeclarationAndDelayedReplyCheckCurrentProcess(t *testing.T) {
	require.True(t, registrationFromRPC(&rpc.DeclareRegistrationInput{OriginIntake: true}).OriginIntake)
	host := &Server{ctx: t.Context()}
	proc := selectedCurrentProcess(t, host)
	proc.SetRegistration(&plugin.PluginRegistration{OriginIntake: true, Done: true})
	proc.SetStage(plugin.StageRunning)
	manager := process.NewProcessManager(nil)
	require.NoError(t, manager.AddProcess(proc.Name(), proc))
	host.procManager.Store(manager)
	owner, supported, err := host.OriginIntakeOwner(proc.Name())
	require.NoError(t, err)
	require.True(t, supported)
	input := rpc.OriginIntakeInput{Operation: rpc.OriginIntakeAnnounce, Destination: "192.0.2.7", PeerOwner: 17, OriginReceipt: 3,
		DeadlineMS: time.Now().Add(time.Second).UnixMilli(), Producer: rpc.OriginProducer{Kind: rpc.OriginProducerOperator},
		Source: rpc.OriginSource{Kind: rpc.OriginSourceLocalBGP}, Route: rpc.OriginBuild{Family: "ipv4/unicast", KeyNLRI: []byte{0}, NextHopPolicy: rpc.OriginNextHopSelf}}
	proc.Bridge().SetOriginIntake(func(_ context.Context, in rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
		require.NoError(t, manager.AddProcess(proc.Name(), process.NewProcess(proc.Config())))
		return &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{{Family: in.Route.Family, KeyNLRI: in.Route.KeyNLRI,
			Destination: in.Destination, PeerOwner: in.PeerOwner, Generation: in.OriginReceipt, Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}}}, nil
	})
	proc.Bridge().SetReady()
	result, err := host.OriginIntake(t.Context(), owner, input)
	require.ErrorContains(t, err, "incarnation")
	require.Nil(t, result)
}
