// Design: docs/architecture/api/process-protocol.md -- explicit portable encoding facts.
package rpc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// JSON construction keeps this regression compilable against original B, whose
// missing portable facts otherwise let an unknown context claim ASN4 silently.
func TestOriginIntakeRefusesUnknownFourOctetEncoding(t *testing.T) {
	now := time.Now()
	value := OriginIntakeInput{Operation: OriginIntakeAnnounce, Destination: "192.0.2.1", PeerOwner: 7, OriginReceipt: 13,
		DeadlineMS: now.Add(time.Second).UnixMilli(), Producer: OriginProducer{Kind: OriginProducerOperator},
		Source: OriginSource{Kind: OriginSourceLocalBGP}, Route: OriginBuild{Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 0, 0}, NextHopPolicy: OriginNextHopSelf}}
	data, err := json.Marshal(value)
	require.NoError(t, err)
	var input map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &input))
	var route map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(input["route"], &route))
	for _, facts := range []struct{ known, asn4, valid bool }{{false, false, true}, {true, false, true}, {true, true, true}, {false, true, false}} {
		route["as-path-known"], err = json.Marshal(facts.known)
		require.NoError(t, err)
		route["as-path-asn4"], err = json.Marshal(facts.asn4)
		require.NoError(t, err)
		input["route"], err = json.Marshal(route)
		require.NoError(t, err)
		data, err = json.Marshal(input)
		require.NoError(t, err)
		var decoded OriginIntakeInput
		require.NoError(t, json.Unmarshal(data, &decoded))
		if facts.valid {
			require.NoError(t, decoded.Validate(now))
		} else {
			require.ErrorContains(t, decoded.Validate(now), "unknown AS-path width")
		}
	}
}
