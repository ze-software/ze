// Design: docs/architecture/wire/nlri-bgpls.md -- BGP-LS export, withdrawal of a changed NLRI
// RFC: rfc/short/rfc9552.md -- Section 5.2, the old NLRI is withdrawn in MP_UNREACH_NLRI
// Overview: reactor_api_batch.go -- buildBatchWithdrawUpdate, where a withdrawal becomes wire
// Related: ../plugins/ls_export/export_rfc9552_polarity_test.go -- the producer sends the withdrawal

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// TestRFC9552LinkStateWithdrawalLeavesInMPUnreach drives the reactor step a BGP-LS
// withdrawal reaches after ls_export sends "nlri bgp-ls/bgp-ls del <hex>" and the update
// handler turns it into a withdraw batch.
//
// VALIDATES: RFC 9552 Section 5.2 -- "the BGP-LS Producer MUST withdraw the old NLRI by
// including it in the MP_UNREACH_NLRI". The withdrawn Link-State NLRI leaves in an
// MP_UNREACH_NLRI attribute for AFI 16388 / SAFI 71 carrying its bytes unchanged, and
// never in the IPv4 Withdrawn Routes field or the NLRI field.
// PREVENTS: a BGP-LS withdrawal framed like an IPv4 one, which a receiver cannot read as
// a Link-State withdrawal and so leaves the old NLRI standing beside the new.
//
// RFC requirement: RFC9552-5.2-6 positive -- the wire half: a withdraw batch holding one Link-State Node NLRI builds an UPDATE whose MP_UNREACH_NLRI (code 15) is AFI 16388, SAFI 71 followed by that NLRI's exact bytes, with empty Withdrawn Routes and NLRI fields.
func TestRFC9552LinkStateWithdrawalLeavesInMPUnreach(t *testing.T) {
	old := lsNodeNLRI(65001)
	wn, err := nlri.NewWireNLRI(lsFam, old, false)
	require.NoError(t, err)
	batch := bgptypes.NLRIBatch{Family: lsFam, NLRIs: []nlri.NLRI{wn}}

	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update := adapter.buildBatchWithdrawUpdate(attrBuf, nlriBuf, batch, announceFacts{})
	require.NotNil(t, update)

	assert.Empty(t, update.WithdrawnRoutes, "a Link-State NLRI is never an IPv4 withdrawn route")
	assert.Empty(t, update.NLRI)
	_, value, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrMPUnreachNLRI))
	require.True(t, ok, "the withdrawal must carry MP_UNREACH_NLRI")
	want := append([]byte{0x40, 0x04, 0x47}, old...) // AFI 16388, SAFI 71, then the NLRI
	assert.Equal(t, want, value, "MP_UNREACH_NLRI must name the BGP-LS family and the old NLRI unchanged")
}
