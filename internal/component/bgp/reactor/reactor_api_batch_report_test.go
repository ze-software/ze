package reactor

import (
	"bufio"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// These tests cover what SendRoutes REPORTS, not what it puts on the wire. The
// wire behavior is unchanged and its own tests still hold it; what is new is
// that a peer which took less than the commit queued says so on its own row,
// with a reason, instead of being skipped by a bare `continue`.

// errCommitConnRefused is what a test connection answers once it has been told
// to stop accepting writes.
var errCommitConnRefused = errors.New("test connection refused the write")

// failAfterConn accepts writesLeft writes and refuses every write after them, so
// a test can put a peer's session into partial failure at a chosen point. The
// session's bufio.Writer caches the first error, so the refusal is sticky, which
// is what a broken socket does.
type failAfterConn struct {
	recordingConn
	writesLeft int
}

func (c *failAfterConn) Write(b []byte) (int, error) {
	if c.writesLeft <= 0 {
		return 0, errCommitConnRefused
	}
	c.writesLeft--
	return c.recordingConn.Write(b)
}

// commitPeerOption configures one peer of a commit fixture.
type commitPeerOption struct {
	address string
	name    string

	// established says the peer holds an encoding context. A peer built
	// without one is what a configured-but-down session looks like to
	// SendRoutes (A-3).
	established bool

	// writesLeft bounds how many writes the peer's connection accepts. Zero
	// with established set is a peer whose socket refuses everything.
	writesLeft int
}

// newCommitFixture builds an adapter holding one peer per option, and returns
// each peer's connection in the same order. A peer marked established is driven
// to fsm.StateEstablished and backed by a failAfterConn, so a test decides
// exactly how many UPDATEs it accepts, and reads back what reached the wire.
func newCommitFixture(t *testing.T, options ...commitPeerOption) (*reactorAPIAdapter, []*failAfterConn) {
	t.Helper()

	conns := make([]*failAfterConn, 0, len(options))
	byKey := make(map[netip.AddrPort]*Peer, len(options))

	for _, option := range options {
		settings := &PeerSettings{
			Connection: ConnectionBoth,
			Address:    netip.MustParseAddr(option.address),
			Name:       option.name,
			LocalAS:    65000,
			PeerAS:     65001,
			RouterID:   0x01020301,
		}
		peer := NewPeer(settings)
		peer.negotiated.Store(&NegotiatedCapabilities{
			families: map[family.Family]bool{
				family.IPv4Unicast: true,
				family.IPv6Unicast: true,
			},
		})

		var conn *failAfterConn
		if option.established {
			peer.state.Store(int32(PeerStateEstablished))

			ctx := bgpctx.EncodingContextForASN4(true)
			ctxID, registerErr := bgpctx.Registry.Register(ctx)
			require.NoError(t, registerErr)
			peer.sendCtx.Store(ctx)
			peer.sendCtxID = ctxID

			session := NewSession(settings)
			require.NoError(t, session.fsm.Event(fsm.EventManualStart))
			require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
			require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
			require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
			require.Equal(t, fsm.StateEstablished, session.fsm.State())

			conn = &failAfterConn{writesLeft: option.writesLeft}
			session.mu.Lock()
			session.conn = conn
			session.bufWriter = bufio.NewWriterSize(conn, 4096)
			session.mu.Unlock()

			peer.mu.Lock()
			peer.session = session
			peer.mu.Unlock()
		}

		byKey[settings.PeerKey()] = peer
		conns = append(conns, conn)
	}

	adapter := &reactorAPIAdapter{r: &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  byKey,
	}}
	return adapter, conns
}

// rowFor finds the row a result carries for one peer address. Every matched
// peer owes a row, so a missing one is a failure rather than a zero value.
func rowFor(t *testing.T, result bgptypes.TransactionResult, address string) bgptypes.PeerCommitResult {
	t.Helper()
	for i := range result.Peers {
		if result.Peers[i].Address == address {
			return result.Peers[i]
		}
	}
	t.Fatalf("no row for peer %s; rows: %+v", address, result.Peers)
	return bgptypes.PeerCommitResult{}
}

// commitWithdrawals builds count IPv4 /32 withdrawals starting at 10.0.0.0.
func commitWithdrawals(count int) []nlri.NLRI {
	out := make([]nlri.NLRI, 0, count)
	for i := range count {
		addr := netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)})
		out = append(out, nlri.NewINET(family.IPv4Unicast, netip.PrefixFrom(addr, 32), 0))
	}
	return out
}

// TestSendRoutesNamesTheNotEstablishedPeer covers the first of the three
// outcomes that used to leave the answer wrong: a matched peer with no send
// context was skipped with a bare `continue`, while the queue length had already
// been reported as delivered for every peer.
//
// VALIDATES: AC-1, AC-2 -- one row per matched peer; the down peer's row states
// not-established with zero counts, and the totals count only what left.
// PREVENTS: a commit reporting one withdrawal delivered when it reached one of
// two matched peers.
func TestSendRoutesNamesTheNotEstablishedPeer(t *testing.T) {
	adapter, conns := newCommitFixture(t,
		commitPeerOption{address: "10.0.0.2", name: "up", established: true, writesLeft: 8},
		commitPeerOption{address: "10.0.0.3", name: "down"},
	)

	withdrawals := commitWithdrawals(1)
	result, err := adapter.SendRoutes(selector.All(), nil, withdrawals, false, plugin.OperatorSender())
	require.NoError(t, err, "a down peer is a row, never a failure of the command")

	require.Len(t, result.Peers, 2, "one row per matched peer")

	up := rowFor(t, result, "10.0.0.2")
	assert.Equal(t, "up", up.Name)
	assert.Equal(t, "established", up.State)
	assert.Equal(t, 1, up.RoutesWithdrawn)
	assert.Equal(t, 1, up.UpdatesSent)
	assert.Empty(t, up.Reasons, "this peer took everything queued")

	down := rowFor(t, result, "10.0.0.3")
	assert.Equal(t, "down", down.Name)
	assert.Equal(t, 0, down.RoutesWithdrawn)
	assert.Equal(t, 0, down.UpdatesSent)
	assert.Equal(t, []string{bgptypes.CommitReasonNotEstablished}, down.Reasons)

	// AC-2: the queue size is its own field, and the delivered total is the sum
	// of the rows rather than the queue length.
	assert.Equal(t, 1, result.WithdrawalsQueued)
	assert.Equal(t, 1, result.RoutesWithdrawn, "one of two matched peers took it")
	assert.Equal(t, 1, result.UpdatesSent)

	// The wire is unchanged: the established peer still received its UPDATE.
	assert.NotEmpty(t, conns[0].written(), "the established peer must still be sent the withdrawal")
}

// TestSendRoutesKeepsThePartialUpdatesOfARefusedCommit covers the second
// outcome: (*CommitService).Commit returns partial stats BESIDE its error, and
// the caller used to discard both with a bare `continue`. The UPDATEs that had
// already left were then counted nowhere.
//
// VALIDATES: AC-5 -- the row reports the UPDATEs that left and the routes they
// carried, states announce-refused, and the top-level total includes them.
// PREVENTS: a commit that half-succeeded reporting zero work done.
func TestSendRoutesKeepsThePartialUpdatesOfARefusedCommit(t *testing.T) {
	// One write accepted, every later write refused. Two routes with different
	// next hops are two attribute groups, and GroupByAttributesTwoLevel sorts
	// them by key, so the first UPDATE leaves and the second does not.
	adapter, _ := newCommitFixture(t,
		commitPeerOption{address: "10.0.0.2", established: true, writesLeft: 1},
	)

	routes := []*rib.Route{
		rib.NewRouteWithASPath(
			nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.1.0.0/24"), 0),
			netip.MustParseAddr("192.0.2.1"), nil, nil),
		rib.NewRouteWithASPath(
			nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.2.0.0/24"), 0),
			netip.MustParseAddr("192.0.2.2"), nil, nil),
	}

	result, err := adapter.SendRoutes(selector.All(), routes, nil, false, plugin.OperatorSender())
	require.NoError(t, err)

	row := rowFor(t, result, "10.0.0.2")
	assert.Equal(t, 1, row.UpdatesSent, "the UPDATE that left before the refusal is counted")
	assert.Equal(t, 1, row.RoutesAnnounced, "the route that UPDATE carried is counted")
	assert.Equal(t, []string{bgptypes.CommitReasonAnnounceRefused}, row.Reasons)

	assert.Equal(t, 2, result.RoutesQueued)
	assert.Equal(t, 1, result.RoutesAnnounced, "half the commit reached the wire")
	assert.Equal(t, 1, result.UpdatesSent)
}

// TestSendRoutesCountsOnlyTheWithdrawalsThatLeft covers the third outcome: a
// family whose NLRIs do not fit the build buffer is refused and the loop moves
// on. That refusal reached the log and nothing else, while the answer reported
// every queued withdrawal as delivered.
//
// VALIDATES: AC-2 -- a refused family is not counted as withdrawn, and its
// reason reaches the row.
// PREVENTS: an operator being told a thousand prefixes were withdrawn when the
// build buffer refused all of them.
func TestSendRoutesCountsOnlyTheWithdrawalsThatLeft(t *testing.T) {
	adapter, _ := newCommitFixture(t,
		commitPeerOption{address: "10.0.0.2", established: true, writesLeft: 8},
	)

	// The build buffer is 4K (bufMuxStd). An IPv4 /32 NLRI is five octets, so
	// two thousand of them cannot fit and writeBatchNLRI refuses the family.
	// One IPv6 prefix in a second family still fits and still leaves.
	withdrawals := commitWithdrawals(2000)
	withdrawals = append(withdrawals,
		nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::/32"), 0))

	result, err := adapter.SendRoutes(selector.All(), nil, withdrawals, false, plugin.OperatorSender())
	require.NoError(t, err)

	row := rowFor(t, result, "10.0.0.2")
	assert.Equal(t, 1, row.RoutesWithdrawn, "only the IPv6 family fitted and left")
	assert.Equal(t, 1, row.UpdatesSent)
	assert.Equal(t, []string{bgptypes.CommitReasonWithdrawRefused}, row.Reasons)

	assert.Equal(t, 2001, result.WithdrawalsQueued)
	assert.Equal(t, 1, result.RoutesWithdrawn, "the refused family is not delivered work")
}

// TestSendRoutesReportsTheEORThatLeft covers the End-of-RIB half, which used to
// be echoed from the operator's request rather than observed.
//
// VALIDATES: AC-6 -- a peer whose End-of-RIB send failed reports none, and a
// peer that accepted one reports it.
// PREVENTS: `request commit eor` answering that a marker was sent to a peer
// whose socket refused it.
func TestSendRoutesReportsTheEORThatLeft(t *testing.T) {
	// The first peer accepts the withdrawal and the End-of-RIB. The second
	// accepts the withdrawal and refuses the End-of-RIB after it.
	adapter, _ := newCommitFixture(t,
		commitPeerOption{address: "10.0.0.2", established: true, writesLeft: 8},
		commitPeerOption{address: "10.0.0.3", established: true, writesLeft: 1},
	)

	result, err := adapter.SendRoutes(selector.All(), nil, commitWithdrawals(1), true, plugin.OperatorSender())
	require.NoError(t, err)

	accepted := rowFor(t, result, "10.0.0.2")
	assert.Equal(t, 1, accepted.EORSent)
	assert.Empty(t, accepted.Reasons)

	refused := rowFor(t, result, "10.0.0.3")
	assert.Equal(t, 1, refused.RoutesWithdrawn, "the withdrawal left before the socket refused")
	assert.Equal(t, 0, refused.EORSent, "the marker did not leave, so it is not reported as sent")
	assert.Equal(t, []string{bgptypes.CommitReasonEORRefused}, refused.Reasons)

	assert.True(t, result.EORRequested, "the operator asked for one")
	assert.Equal(t, 1, result.EORSent, "one of the two markers left")
}

// TestSendRoutesReasonsAndShortfallAgree holds the invariant the whole answer
// rests on: a row carries a reason if and only if that peer took less than the
// commit offered it. A reason without a shortfall would fail a command that
// succeeded; a shortfall without a reason would answer `done` over dropped work.
//
// VALIDATES: AC-3, AC-4 across every peer shape this rail produces.
// PREVENTS: a future outcome added to the loop that leaves one side unset.
func TestSendRoutesReasonsAndShortfallAgree(t *testing.T) {
	adapter, _ := newCommitFixture(t,
		commitPeerOption{address: "10.0.0.2", established: true, writesLeft: 8},
		commitPeerOption{address: "10.0.0.3", established: true, writesLeft: 0},
		commitPeerOption{address: "10.0.0.4"},
	)

	withdrawals := commitWithdrawals(2)
	result, err := adapter.SendRoutes(selector.All(), nil, withdrawals, true, plugin.OperatorSender())
	require.NoError(t, err)
	require.Len(t, result.Peers, 3)

	for _, row := range result.Peers {
		short := row.RoutesWithdrawn < result.WithdrawalsQueued ||
			row.RoutesAnnounced < result.RoutesQueued ||
			(result.EORRequested && row.EORSent < len(result.Families))
		assert.Equal(t, short, len(row.Reasons) > 0,
			"peer %s: shortfall=%v reasons=%v", row.Address, short, row.Reasons)
	}

	// The peer that accepted everything is the `done` case, and the two that
	// did not are what turns the command into an error.
	assert.Empty(t, rowFor(t, result, "10.0.0.2").Reasons)
	assert.NotEmpty(t, rowFor(t, result, "10.0.0.3").Reasons)
	assert.Equal(t, []string{bgptypes.CommitReasonNotEstablished}, rowFor(t, result, "10.0.0.4").Reasons)
}

// Compile-time proof that failAfterConn is still a net.Conn after the embedded
// recordingConn's Write is shadowed.
var _ net.Conn = (*failAfterConn)(nil)
