// VPP backend MPLS input: the table creation, enables and disables a whole
// `mpls { enable }` set turns into.
package ifacevpp

import (
	"fmt"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/mpls"
)

// mplsChannel is an api.Channel fake for the MPLS messages. It answers
// MplsInterfaceDump with enabled and every reply with retval, and records each
// request in order.
type mplsChannel struct {
	enabled  []interface_types.InterfaceIndex
	retval   int32
	requests []api.Message
}

var _ api.Channel = (*mplsChannel)(nil)

func (c *mplsChannel) SendRequest(msg api.Message) api.RequestCtx {
	c.requests = append(c.requests, msg)
	return &mplsReqCtx{retval: c.retval}
}

func (c *mplsChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	c.requests = append(c.requests, msg)
	return &mplsMultiCtx{rows: c.enabled}
}

func (c *mplsChannel) SubscribeNotification(_ chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	return nil, fmt.Errorf("SubscribeNotification not implemented")
}

func (c *mplsChannel) SetReplyTimeout(time.Duration)          {}
func (c *mplsChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *mplsChannel) Close()                                 {}

type mplsReqCtx struct{ retval int32 }

func (r *mplsReqCtx) ReceiveReply(msg api.Message) error {
	switch reply := msg.(type) {
	case *mpls.MplsTableAddDelReply:
		reply.Retval = r.retval
	case *mpls.SwInterfaceSetMplsEnableReply:
		reply.Retval = r.retval
	}
	return nil
}

type mplsMultiCtx struct {
	rows []interface_types.InterfaceIndex
}

func (m *mplsMultiCtx) ReceiveReply(msg api.Message) (bool, error) {
	if len(m.rows) == 0 {
		return true, nil
	}
	if dst, ok := msg.(*mpls.MplsInterfaceDetails); ok {
		dst.SwIfIndex = m.rows[0]
	}
	m.rows = m.rows[1:]
	return false, nil
}

// VALIDATES: AC-27 -- table 0 is created before any enable; a named interface
// VPP does not report enabled is enabled once; one it reports and the set no
// longer names (enabled while ze was down) is disabled once; one already
// enabled and still named is left alone, because VPP counts enables.
// Method: the fake channel reports xe1 (2) and xe1.100 (3) enabled; the set
// names xe0 (1) and xe1.
func TestVPPReplaceMPLSInput(t *testing.T) {
	ch := &mplsChannel{enabled: []interface_types.InterfaceIndex{2, 3}}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 1, "xe0")
	b.names.Add("xe1", 2, "xe1")
	b.names.Add("xe1.100", 3, "xe1.100")
	// The backend's one-time name-map read (SwInterfaceDump in populateOnce)
	// is not part of the MPLS input set; mark it done so only the MPLS
	// requests reach the fake, as the sibling apply tests do.
	b.populate.Do(func() {})

	if err := b.ReplaceMPLSInput([]string{"xe0", "xe1"}); err != nil {
		t.Fatalf("ReplaceMPLSInput: %v", err)
	}
	if len(ch.requests) != 4 {
		t.Fatalf("requests = %d (%v), want table, dump, disable xe1.100, enable xe0", len(ch.requests), ch.requests)
	}
	table, ok := ch.requests[0].(*mpls.MplsTableAddDel)
	if !ok || !table.MtIsAdd || table.MtTable.MtTableID != 0 {
		t.Fatalf("first request = %#v, want MplsTableAddDel add table 0", ch.requests[0])
	}
	if _, ok := ch.requests[1].(*mpls.MplsInterfaceDump); !ok {
		t.Fatalf("second request = %T, want MplsInterfaceDump", ch.requests[1])
	}
	disable, ok := ch.requests[2].(*mpls.SwInterfaceSetMplsEnable)
	if !ok || disable.Enable || disable.SwIfIndex != 3 {
		t.Fatalf("third request = %#v, want disable index 3", ch.requests[2])
	}
	enable, ok := ch.requests[3].(*mpls.SwInterfaceSetMplsEnable)
	if !ok || !enable.Enable || enable.SwIfIndex != 1 {
		t.Fatalf("fourth request = %#v, want enable index 1", ch.requests[3])
	}

	// An empty set disables every enabled interface and creates no table.
	ch = &mplsChannel{enabled: []interface_types.InterfaceIndex{1}}
	b.ch = ch
	if err := b.ReplaceMPLSInput(nil); err != nil {
		t.Fatalf("ReplaceMPLSInput(nil): %v", err)
	}
	if len(ch.requests) != 2 {
		t.Fatalf("requests = %v, want dump then disable index 1", ch.requests)
	}

	// An unknown interface is refused before anything reaches VPP, and a VPP
	// refusal names the interface.
	ch = &mplsChannel{}
	b.ch = ch
	if err := b.ReplaceMPLSInput([]string{"nope0"}); err == nil {
		t.Fatal("unknown interface accepted")
	}
	if len(ch.requests) != 0 {
		t.Fatalf("requests = %v after a refused name, want none", ch.requests)
	}
	ch.retval = -6
	if err := b.ReplaceMPLSInput([]string{"xe0"}); err == nil {
		t.Fatal("a VPP refusal was not reported")
	}
}
