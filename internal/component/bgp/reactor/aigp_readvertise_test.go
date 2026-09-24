package reactor

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type aigpReplayFixture struct {
	r                   *Reactor
	source, destination *Peer
	conn                *recordingConn
	ctxID               bgpctx.ContextID
	completed           chan struct{}
	ids                 []uint64
}

func newAIGPReplayFixture(t *testing.T, before func([]fwdItem)) *aigpReplayFixture {
	t.Helper()
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	source, _ := newAnnouncePeer(t, "198.18.231.1")
	destination, conn := newAnnouncePeer(t, "198.18.232.2")
	for _, peer := range []*Peer{source, destination} {
		peer.settings.AIGPSession = new(true)
		peer.settings.GlobalLocalAS = 65000
		peer.settings.LocalAddress = netip.MustParseAddr("10.0.0.254")
		peer.settings.NextHopMode = NextHopSelf
		peer.settings.ProcessBindings = []ProcessBinding{{PluginName: "aigp-forwarder", SendAll: true}}
		caps := &NegotiatedCapabilities{ASN4: true, families: map[family.Family]bool{family.IPv4Unicast: true}}
		peer.negotiated.Store(caps)
		peer.sendCtx.Store(ctx)
		peer.sendCtxID, peer.recvCtxID = ctxID, ctxID
		peer.session.sendCtxID = ctxID
		peer.session.negotiated = &capability.Negotiated{ASN4: true}
		peer.refreshForwardFacts()
	}
	cache := newRecentUpdateCache(100)
	cache.RegisterConsumer("aigp-forwarder")
	f := &aigpReplayFixture{source: source, destination: destination, conn: conn, ctxID: ctxID, completed: make(chan struct{}, 16)}
	f.r = &Reactor{
		clock:           source.clock,
		config:          &Config{LocalAS: 65000},
		peers:           map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source, destination.Settings().PeerKey(): destination},
		recentUpdates:   cache,
		messageReceiver: &testDeliveryReceiver{consumerCount: 1},
		attrModHandlers: attrModHandlersWithDefaults(),
	}
	destination.session.aigpPeer = destination
	destination.session.aigpReactor = f.r
	f.r.fwdPool = newFwdPool(func(key fwdKey, items []fwdItem) {
		if before != nil {
			before(items)
		}
		fwdBatchHandler(key, items)
		for i := range items {
			if items[i].peer != nil {
				f.completed <- struct{}{}
				break
			}
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	f.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: destination.Settings().PeerKey()}, 4096)
	t.Cleanup(func() {
		f.r.fwdPool.Stop()
		for _, id := range f.ids {
			cache.Delete(id)
		}
	})
	igpcost.Set(nil)
	t.Cleanup(func() { igpcost.Set(nil) })
	loc := locrib.Default()
	require.NotNil(t, loc)
	protocol := redistevents.RegisterProtocol("aigp-readvertise-test")
	t.Cleanup(func() { loc.Remove(family.IPv4Unicast, netip.PrefixFrom(source.Settings().Address, 32), protocol, 0) })
	return f
}

func (f *aigpReplayFixture) metric(cost uint32) {
	locrib.Default().Insert(family.IPv4Unicast, netip.PrefixFrom(f.source.Settings().Address, 32), locrib.Path{Source: redistevents.RegisterProtocol("aigp-readvertise-test"), Metric: cost})
}

func (f *aigpReplayFixture) body(t *testing.T, metric uint64) []byte {
	t.Helper()
	body := aigpTestBody(metric)
	sections, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	_, _, nh, found := attribute.AttrFind(sections.Attrs(body), attribute.AttrNextHop)
	require.True(t, found)
	copy(nh, f.source.Settings().Address.AsSlice())
	return body
}

func (f *aigpReplayFixture) receive(t *testing.T, body []byte) uint64 {
	t.Helper()
	wu := wireu.NewWireUpdate(body, f.ctxID)
	wu.SetSourceID(f.source.SourceID())
	require.True(t, f.r.notifyMessageReceiver(f.source.Settings().Address, msgtype.TypeUPDATE, body, wu, f.ctxID, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0))
	f.ids = append(f.ids, wu.MessageID())
	return wu.MessageID()
}

func (f *aigpReplayFixture) forward(t *testing.T, id uint64) {
	t.Helper()
	sel, err := selector.Parse(f.destination.Settings().Address.String())
	require.NoError(t, err)
	require.NoError(t, (&reactorAPIAdapter{r: f.r}).ForwardUpdate(sel, id, "aigp-forwarder", plugin.ProcessSender("aigp-forwarder")))
}

func (f *aigpReplayFixture) drain(t *testing.T) {
	t.Helper()
	done := make(chan struct{})
	require.True(t, f.r.fwdPool.Dispatch(fwdKey{peerAddr: f.destination.Settings().PeerKey()}, fwdItem{done: func() { close(done) }}))
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("forward queue did not drain")
	}
}

func (f *aigpReplayFixture) waitBatch(t *testing.T) {
	t.Helper()
	select {
	case <-f.completed:
	case <-time.After(5 * time.Second):
		t.Fatal("no UPDATE batch reached the socket worker")
	}
	f.drain(t)
}

func aigpSocketBodies(t *testing.T, conn *recordingConn) [][]byte {
	t.Helper()
	data := conn.written()
	var bodies [][]byte
	for len(data) != 0 {
		require.GreaterOrEqual(t, len(data), message.HeaderLen)
		n := int(binary.BigEndian.Uint16(data[16:18]))
		require.GreaterOrEqual(t, n, message.HeaderLen)
		require.LessOrEqual(t, n, len(data))
		require.Equal(t, byte(msgtype.TypeUPDATE), data[18])
		bodies = append(bodies, data[message.HeaderLen:n])
		data = data[n:]
	}
	return bodies
}

// RFC requirement: RFC7311-3.4.3-7 positive -- a real Loc-RIB change re-advertises a previously sent path without a new received UPDATE, after cache eviction.
// RFC requirement: RFC7311-3.4.3-7 negative -- recomputation starts from received AIGP, never the metric in the prior advertisement.
func TestAIGPReadvertisesFromReceivedGeneration(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.metric(7)
	id := f.receive(t, f.body(t, 100))
	f.forward(t, id)
	f.waitBatch(t)
	require.False(t, f.r.recentUpdates.Contains(id), "the metric replay must outlive the transient UPDATE cache")
	initial := aigpSocketBodies(t, f.conn)
	require.Len(t, initial, 1)
	metric, present := aigpReceivedMetric(t, initial[0])
	require.True(t, present)
	require.Equal(t, uint64(107), metric)

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { defer close(stopped); f.r.runAIGPAdvertisements(ctx) }()
	t.Cleanup(func() { cancel(); <-stopped })
	f.metric(11)
	f.waitBatch(t)
	bodies := aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, 2)
	metric, present = aigpReceivedMetric(t, bodies[1])
	require.True(t, present)
	require.Equal(t, uint64(111), metric)
}

func TestAIGPMetricChangeDoesNotInventRecipients(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.metric(7)
	f.receive(t, f.body(t, 100))
	f.metric(11)
	f.r.readvertiseAIGP()
	f.drain(t)
	require.Empty(t, f.conn.written(), "a received route in a role-less collector does not authorize forwarding")
}

func TestAIGPQueuedReplayCannotResurrectOldGeneration(t *testing.T) {
	for _, change := range []string{"withdrawal", "replacement", "source-disconnect", "destination-disconnect"} {
		t.Run(change, func(t *testing.T) {
			entered, release := make(chan struct{}, 1), make(chan struct{})
			f := newAIGPReplayFixture(t, func(items []fwdItem) {
				for i := range items {
					if items[i].aigpReplay != nil {
						entered <- struct{}{}
						<-release
						return
					}
				}
			})
			t.Cleanup(func() {
				select {
				case <-release:
				default:
					close(release)
				}
			})
			f.metric(7)
			f.forward(t, f.receive(t, f.body(t, 100)))
			f.waitBatch(t)
			before := f.conn.written()
			f.metric(11)
			f.r.readvertiseAIGP()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				close(release)
				t.Fatal("metric replay was not queued")
			}
			var next uint64
			switch change {
			case "withdrawal":
				next = f.receive(t, makeUpdateBody([]byte{24, 10, 20, 0}, nil, nil))
			case "replacement":
				next = f.receive(t, f.body(t, 200))
			case "source-disconnect":
				f.r.notifyPeerClosed(f.source, "test disconnect")
			case "destination-disconnect":
				f.r.notifyPeerClosed(f.destination, "test disconnect")
			}
			close(release)
			f.waitBatch(t)
			require.Equal(t, before, f.conn.written(), "queued replay must not publish an obsolete source generation")
			if next != 0 {
				f.forward(t, next)
				f.waitBatch(t)
				bodies := aigpSocketBodies(t, f.conn)
				require.Len(t, bodies, 2)
				if change == "replacement" {
					metric, present := aigpReceivedMetric(t, bodies[1])
					require.True(t, present)
					require.Equal(t, uint64(211), metric)
				} else {
					sections, err := wire.ParseUpdateSections(bodies[1])
					require.NoError(t, err)
					require.Equal(t, []byte{24, 10, 20, 0}, sections.Withdrawn(bodies[1]))
				}
			}
		})
	}
}

func TestAIGPReadvertisementRechecksSenderPermission(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.metric(7)
	f.forward(t, f.receive(t, f.body(t, 100)))
	f.waitBatch(t)
	before := f.conn.written()
	f.r.mu.Lock()
	f.destination.settings.ProcessBindings = nil
	f.r.mu.Unlock()
	f.metric(11)
	f.r.readvertiseAIGP()
	f.drain(t)
	require.Equal(t, before, f.conn.written(), "a retained receipt does not grant permanent authority to the original process")
}

type aigpFailedConn struct{ *recordingConn }

func (c aigpFailedConn) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestAIGPFailedFlushDoesNotCreateRecipient(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.metric(7)
	s := f.destination.session
	failed := aigpFailedConn{f.conn}
	s.conn = failed
	s.bufWriter = bufio.NewWriterSize(failed, 4096)
	f.forward(t, f.receive(t, f.body(t, 100)))
	f.waitBatch(t)
	s.mu.Lock()
	s.writeMu.Lock()
	s.conn = f.conn
	s.bufWriter = bufio.NewWriterSize(f.conn, 4096)
	s.writeMu.Unlock()
	s.mu.Unlock()
	f.metric(11)
	f.r.readvertiseAIGP()
	f.drain(t)
	require.Empty(t, f.conn.written(), "a failed buffered socket flush is not an advertisement")
}

func TestAIGPReadvertisementPreservesAddPathIdentity(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	f.ctxID, f.source.recvCtxID = ctxID, ctxID
	f.destination.sendCtx.Store(ctx)
	f.destination.sendCtxID = ctxID
	f.destination.session.sendCtxID = ctxID
	f.destination.refreshForwardFacts()
	pathBody := func(metric uint64, pathID uint32) []byte {
		body := f.body(t, metric)
		sections, err := wire.ParseUpdateSections(body)
		require.NoError(t, err)
		nlri := binary.BigEndian.AppendUint32(nil, pathID)
		nlri = append(nlri, sections.NLRI(body)...)
		return makeUpdateBody(nil, sections.Attrs(body), nlri)
	}
	f.metric(7)
	f.forward(t, f.receive(t, pathBody(100, 31)))
	f.waitBatch(t)
	f.forward(t, f.receive(t, pathBody(200, 32)))
	f.waitBatch(t)
	initial := aigpSocketBodies(t, f.conn)
	require.Len(t, initial, 2)
	ids := make(map[uint64]uint32)
	for _, body := range initial {
		sections, err := wire.ParseUpdateSections(body)
		require.NoError(t, err)
		metric, present := aigpReceivedMetric(t, body)
		require.True(t, present)
		ids[metric] = binary.BigEndian.Uint32(sections.NLRI(body)[:4])
	}
	require.NotEqual(t, ids[107], ids[207], "two received paths to the same prefix must remain separate at the recipient")
	f.metric(11)
	f.r.readvertiseAIGP()
	f.drain(t)
	bodies := aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, 4)
	for _, body := range bodies[2:] {
		sections, err := wire.ParseUpdateSections(body)
		require.NoError(t, err)
		metric, present := aigpReceivedMetric(t, body)
		require.True(t, present)
		wantID, exists := ids[metric-4]
		require.True(t, exists, "metric replay must use one of the original received metrics")
		require.Equal(t, wantID, binary.BigEndian.Uint32(sections.NLRI(body)[:4]))
	}
	withdraw := binary.BigEndian.AppendUint32(nil, 31)
	withdraw = append(withdraw, 24, 10, 20, 0)
	f.forward(t, f.receive(t, makeUpdateBody(withdraw, nil, nil)))
	f.drain(t)
	before := len(aigpSocketBodies(t, f.conn))
	f.metric(13)
	f.r.readvertiseAIGP()
	f.drain(t)
	bodies = aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, before+1, "withdrawing one received path must leave exactly the other recipient path eligible for metric replay")
	metric, present := aigpReceivedMetric(t, bodies[before])
	require.True(t, present)
	require.Equal(t, uint64(213), metric)
	sections, err := wire.ParseUpdateSections(bodies[before])
	require.NoError(t, err)
	require.Equal(t, ids[207], binary.BigEndian.Uint32(sections.NLRI(bodies[before])[:4]))
}

func TestAIGPUnchangedMetricDoesNotReadvertise(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.metric(7)
	f.forward(t, f.receive(t, f.body(t, 100)))
	f.waitBatch(t)
	before := f.conn.written()
	loc := locrib.Default()
	protocol := redistevents.RegisterProtocol("aigp-readvertise-test")
	unrelated := netip.MustParsePrefix("198.18.233.1/32")
	t.Cleanup(func() { loc.Remove(family.IPv4Unicast, unrelated, protocol, 0) })
	loc.Insert(family.IPv4Unicast, unrelated, locrib.Path{Source: protocol, Metric: 400})
	f.r.readvertiseAIGP()
	f.drain(t)
	require.Equal(t, before, f.conn.written(), "a routing revision with no change to this path's accumulated metric must not re-advertise it")
}
