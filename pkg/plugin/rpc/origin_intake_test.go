// Design: docs/architecture/api/process-protocol.md -- origin intake value validation.
package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOriginIntakeRefusesMissingScopeAndGuessedReceipt(t *testing.T) {
	input := OriginIntakeInput{Operation: OriginIntakeAnnounce, Destination: "192.0.2.1", PeerOwner: 7, OriginReceipt: 13,
		DeadlineMS: time.Now().Add(time.Second).UnixMilli(), Producer: OriginProducer{Kind: OriginProducerOperator},
		Source: OriginSource{Kind: OriginSourceLocalBGP}, Route: OriginBuild{Family: "ipv4/unicast", KeyNLRI: []byte{0}, NextHopPolicy: OriginNextHopSelf}}
	require.NoError(t, input.Validate(time.Now()))
	for _, change := range []func(*OriginIntakeInput){
		func(in *OriginIntakeInput) { in.Operation = OriginIntakeUnspecified },
		func(in *OriginIntakeInput) { in.OriginReceipt = 0 },
		func(in *OriginIntakeInput) { in.PeerOwner = 0 },
		func(in *OriginIntakeInput) { in.Route.SourcePathID = 9 },
		func(in *OriginIntakeInput) { in.Source.Kind = OriginSourceRedistributed },
	} {
		bad := input
		change(&bad)
		require.Error(t, bad.Validate(time.Now()))
	}
	result := &UpdateRouteOutput{Admissions: []RouteAdmission{{Family: input.Route.Family, KeyNLRI: input.Route.KeyNLRI,
		Destination: input.Destination, PeerOwner: input.PeerOwner, Generation: input.OriginReceipt,
		Outcome: RouteAdmissionRetained, Disposition: RouteAdmissionSuppressedSelection}}}
	require.NoError(t, ValidateOriginIntakeOutput(input, result))
	result.Admissions[0].Generation++
	require.Error(t, ValidateOriginIntakeOutput(input, result))
	require.Error(t, ValidateOriginIntakeOutput(input, nil))
}

func TestOriginIntakeTypedSlotRejectsMissingOutput(t *testing.T) {
	bridge := NewDirectBridge()
	bridge.SetOriginIntake(func(context.Context, OriginIntakeInput) (*UpdateRouteOutput, error) {
		return nil, nil //nolint:nilnil // Deliberate bad-success callback output must not become retention.
	})
	bridge.SetReady()
	input := OriginIntakeInput{Operation: OriginIntakeAnnounce, Destination: "192.0.2.1", PeerOwner: 7, OriginReceipt: 13,
		Producer: OriginProducer{Kind: OriginProducerOperator}, Source: OriginSource{Kind: OriginSourceLocalBGP},
		Route: OriginBuild{Family: "ipv4/unicast", KeyNLRI: []byte{0}, NextHopPolicy: OriginNextHopSelf}}
	output, err := bridge.OriginIntake(t.Context(), input)
	require.Error(t, err)
	require.Nil(t, output)
}
