package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// RFC requirement: RFC7432-8.2.1-9 positive -- the received per-ES zero label is a complete three-octet field and survives ingress without requiring a bottom-of-stack bit.
func TestEVPNIngressPreservesPerESZeroLabel(t *testing.T) {
	registerEVPNRecognizer(t)
	route := []byte{1, 25, 0, 1, 192, 0, 2, 1, 0, 7, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 255, 255, 255, 255, 0, 0, 0}
	body := makeUpdateBody(nil, mpReachAttrs(evpnFam, route), nil)
	update, action, err := nlriTypeTestSession().enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionNone, action)
	got, found := mpReachNLRIOf(t, update.Payload())
	require.True(t, found)
	require.Equal(t, route, got)
}

// RFC requirement: RFC7432-8.2.1-9 negative -- missing or extra Ethernet A-D label octets cannot pass the ingress boundary into the RIB or propagation rails.
func TestEVPNIngressRejectsVariableLengthADLabels(t *testing.T) {
	registerEVPNRecognizer(t)
	for _, length := range []byte{22, 28} {
		for _, withdraw := range []bool{false, true} {
			route := append([]byte{1, length}, make([]byte, int(length))...)
			attrs := mpReachAttrs(evpnFam, route)
			if withdraw {
				attrs = mpUnreachAttrs(evpnFam, route)
			}
			body := makeUpdateBody(nil, attrs, nil)
			_, action, err := nlriTypeTestSession().enforceRFC7606(wireu.NewWireUpdate(body, 0))
			require.Error(t, err, "length %d, withdraw %t", length, withdraw)
			require.Equal(t, message.RFC7606ActionSessionReset, action)
		}
	}
}
