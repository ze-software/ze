// VPP backend VLAN QoS: a QoS step VPP refuses fails the call that asked for
// it, on a VLAN being created and on one that already exists.
//
// VALIDATES: a VLAN whose QoS programming VPP refuses is deleted and the
// create fails; UpdateVLANQoSMap returns a refused map or mark.
// PREVENTS: config-apply reading the half-created VLAN as one that already
// existed and committing green with its QoS maps not applied.
package ifacevpp

import (
	"strings"
	"testing"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/qos"

	"github.com/ze-software/ze/internal/component/iface"
)

// qosStep names one QoS request VPP can refuse.
type qosStep int

const (
	qosStepUnspecified qosStep = iota
	qosStepRecord
	qosStepEgressMap
	qosStepMarkEnable
	qosStepMarkDisable
)

// qosRefusingChannel is a parentChannel that refuses one QoS request: refused names
// it, and refusedRetval chooses whether VPP answers with a nonzero retval or
// the transport fails the request. Every other request gets parentChannel's
// answer, so CreateVLAN, its MTU and the cleanup DeleteSubif behave as in
// mtu_parent_test.go. Not safe for concurrent use.
type qosRefusingChannel struct {
	*parentChannel
	refused       qosStep
	refusedRetval bool
}

var _ api.Channel = (*qosRefusingChannel)(nil)

func (c *qosRefusingChannel) SendRequest(msg api.Message) api.RequestCtx {
	return &qosRefusingReqCtx{ch: c, req: msg, inner: c.parentChannel.SendRequest(msg)}
}

type qosRefusingReqCtx struct {
	ch    *qosRefusingChannel
	req   api.Message
	inner api.RequestCtx
}

func (r *qosRefusingReqCtx) ReceiveReply(msg api.Message) error {
	step := r.step()
	if step == qosStepUnspecified {
		return r.inner.ReceiveReply(msg)
	}
	if step != r.ch.refused {
		return nil
	}
	if !r.ch.refusedRetval {
		return api.INVALID_VALUE
	}
	switch reply := msg.(type) {
	case *qos.QosRecordEnableDisableReply:
		reply.Retval = int32(api.INVALID_VALUE)
	case *qos.QosEgressMapUpdateReply:
		reply.Retval = int32(api.INVALID_VALUE)
	case *qos.QosMarkEnableDisableReply:
		reply.Retval = int32(api.INVALID_VALUE)
	}
	return nil
}

// step names the QoS request this context carries, or qosStepUnspecified for
// any other request.
func (r *qosRefusingReqCtx) step() qosStep {
	switch req := r.req.(type) {
	case *qos.QosRecordEnableDisable:
		return qosStepRecord
	case *qos.QosEgressMapUpdate:
		return qosStepEgressMap
	case *qos.QosMarkEnableDisable:
		if req.Enable {
			return qosStepMarkEnable
		}
		return qosStepMarkDisable
	}
	return qosStepUnspecified
}

func newQoSRefusingChannel(refused qosStep, refusedRetval bool) *qosRefusingChannel {
	return &qosRefusingChannel{
		parentChannel: &parentChannel{
			details: map[interface_types.InterfaceIndex]interfaces.SwInterfaceDetails{3: parentDetails(1500, 1500)},
			subIdx:  7,
		},
		refused:       refused,
		refusedRetval: refusedRetval,
	}
}

// TestVPPCreateVLANRemovesTheVLANWhenItsQoSFails proves a VLAN whose QoS maps
// VPP refuses is deleted and the create fails. config-apply reads a failed
// create whose interface exists as an interface that already existed, so a
// VLAN left behind would commit green with its maps not applied and no undo
// recorded to remove it. Method: create xe0.100 (new index 7) with an egress
// map while the stub refuses each QoS request in turn, by error and by retval,
// and read the error and the DeleteSubif sent.
func TestVPPCreateVLANRemovesTheVLANWhenItsQoSFails(t *testing.T) {
	steps := []struct {
		name string
		step qosStep
	}{
		{"qos record", qosStepRecord},
		{"egress map", qosStepEgressMap},
		{"egress mark", qosStepMarkEnable},
	}
	for _, s := range steps {
		for _, retval := range []bool{false, true} {
			name := s.name + " refused by error"
			if retval {
				name = s.name + " refused by retval"
			}
			t.Run(name, func(t *testing.T) {
				ch := newQoSRefusingChannel(s.step, retval)
				b := newParentBackend(ch.parentChannel)
				b.ch = ch
				err := b.CreateVLAN(iface.VLANSpec{
					Parent:        "xe0",
					VLANID:        100,
					IngressQoSMap: map[uint32]uint32{3: 3},
					EgressQoSMap:  map[uint32]uint32{0: 1, 5: 5},
				})
				if err == nil {
					t.Fatal("CreateVLAN with a refused QoS step returned nil")
				}
				if !strings.Contains(err.Error(), "xe0.100 qos") {
					t.Fatalf("CreateVLAN err = %v, want it to name xe0.100 qos", err)
				}
				del := sent[*interfaces.DeleteSubif](&mtuChannel{requests: ch.requests})
				if len(del) != 1 || del[0].SwIfIndex != 7 {
					t.Fatalf("DeleteSubif requests = %+v, want one for index 7", del)
				}
				if _, known := b.names.lookupName(7); known {
					t.Fatal("the deleted VLAN is still in the name map")
				}
			})
		}
	}
}

// TestVPPCreateVLANKeepsTheVLANWhenItsQoSApplies is the control for the test
// above: with every QoS request accepted the create succeeds and nothing is
// deleted, so the cleanup runs on a refusal only.
func TestVPPCreateVLANKeepsTheVLANWhenItsQoSApplies(t *testing.T) {
	ch := newQoSRefusingChannel(qosStepUnspecified, false)
	b := newParentBackend(ch.parentChannel)
	b.ch = ch
	if err := b.CreateVLAN(iface.VLANSpec{
		Parent:       "xe0",
		VLANID:       100,
		EgressQoSMap: map[uint32]uint32{0: 1},
	}); err != nil {
		t.Fatalf("CreateVLAN: %v", err)
	}
	if del := sent[*interfaces.DeleteSubif](&mtuChannel{requests: ch.requests}); len(del) != 0 {
		t.Fatalf("DeleteSubif requests = %+v, want none", del)
	}
	if _, known := b.names.lookupIndex("xe0.100"); !known {
		t.Fatal("the created VLAN is missing from the name map")
	}
}

// TestVPPUpdateVLANQoSMapReturnsARefusal proves the existing-VLAN path, which
// config-apply takes when the VLAN already exists, fails when VPP refuses a
// QoS request, so a reload that changed the maps cannot commit green with the
// old ones installed. Method: update xe0.100 (index 7) with an egress map
// while the stub refuses the map or the mark, and with no egress map while it
// refuses the mark disable, by error and by retval.
func TestVPPUpdateVLANQoSMapReturnsARefusal(t *testing.T) {
	steps := []struct {
		name   string
		step   qosStep
		egress map[uint32]uint32
	}{
		{"egress map", qosStepEgressMap, map[uint32]uint32{0: 1}},
		{"egress mark", qosStepMarkEnable, map[uint32]uint32{0: 1}},
		{"mark disable", qosStepMarkDisable, nil},
	}
	for _, s := range steps {
		for _, retval := range []bool{false, true} {
			name := s.name + " refused by error"
			if retval {
				name = s.name + " refused by retval"
			}
			t.Run(name, func(t *testing.T) {
				ch := newQoSRefusingChannel(s.step, retval)
				b := newParentBackend(ch.parentChannel)
				b.ch = ch
				b.names.Add("xe0.100", 7, "xe0.100")
				err := b.UpdateVLANQoSMap("xe0.100", nil, s.egress)
				if err == nil {
					t.Fatal("UpdateVLANQoSMap with a refused QoS step returned nil")
				}
				if !strings.Contains(err.Error(), "xe0.100") {
					t.Fatalf("UpdateVLANQoSMap err = %v, want it to name xe0.100", err)
				}
			})
		}
	}
}
