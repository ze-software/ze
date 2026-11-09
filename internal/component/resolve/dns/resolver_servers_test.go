package dns

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	mdns "github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// silentServer answers a UDP address that receives queries and never replies,
// counting what it received. Caller MUST call the returned cleanup.
func silentServer(t *testing.T) (string, *atomic.Int32, func()) {
	t.Helper()
	pc, err := (&net.ListenConfig{}).ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	require.NoError(t, err)
	var received atomic.Int32
	done := make(chan struct{})
	go readUntilClosed(pc, &received, done)
	return pc.LocalAddr().String(), &received, func() {
		_ = pc.Close()
		<-done
	}
}

// readUntilClosed counts datagrams on pc until it is closed, then closes done.
func readUntilClosed(pc net.PacketConn, received *atomic.Int32, done chan<- struct{}) {
	defer close(done)
	buf := make([]byte, 512)
	for {
		if _, _, err := pc.ReadFrom(buf); err != nil {
			return
		}
		received.Add(1)
	}
}

// countingHandler answers every A query with addr and counts the queries.
func countingHandler(addr string, count *atomic.Int32) mdns.HandlerFunc {
	return func(w mdns.ResponseWriter, req *mdns.Msg) {
		count.Add(1)
		m := new(mdns.Msg)
		m.SetReply(req)
		rr, err := mdns.NewRR(req.Question[0].Name + " 300 IN A " + addr)
		if err == nil {
			m.Answer = append(m.Answer, rr)
		}
		_ = w.WriteMsg(m)
	}
}

// TestResolverWithNoServersFailsClosed: a resolver built with no servers is a
// usable value, and every query fails with "no DNS server configured" without
// sending anything: it reads no resolv.conf of its own and has no built-in
// server to fall back to.
//
// VALIDATES: spec-home-cpe-23 D-14 condition 1, replacing
// TestResolveDefaultServer, TestResolveSystemDNSNoPublicFallback and
// TestResolveSystemDNSFailClosed (ze-test-judge approval, 2026-11-08).
// PREVENTS: a hardcoded public resolver, or a resolv.conf read behind the
// registry's back, answering when nothing named a server.
func TestResolverWithNoServersFailsClosed(t *testing.T) {
	r := NewResolver(ResolverConfig{Timeout: 1})
	require.NotNil(t, r)
	defer r.Close()

	assert.Empty(t, r.Servers(), "a resolver given no servers holds none")
	_, err := r.ResolveA("example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no DNS server configured")
	_, _, status, err := r.ResolveWithTTL("example.com", mdns.TypeA)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no DNS server configured")
	assert.Equal(t, StatusUnspecified, status)
}

// TestInternalResolverFollowsSelection: the resolver asks its servers in
// order and moves to the next when one does not answer; SetServers replaces
// the list for the next query without a new resolver.
//
// VALIDATES: spec-home-cpe-23 AC-1 (queries 192.0.2.53, then 192.0.2.54 when
// the first does not answer) and AC-2 (a new list applies without a restart),
// at the resolver the hub hands the router selection to.
// PREVENTS: the single-server resolver that ignored every server after the
// first and every change after startup.
func TestInternalResolverFollowsSelection(t *testing.T) {
	silent, silentCount, stopSilent := silentServer(t)
	defer stopSilent()
	var firstCount, secondCount atomic.Int32
	first, stopFirst := testDNSServer(t, countingHandler("192.0.2.10", &firstCount))
	defer stopFirst()
	second, stopSecond := testDNSServer(t, countingHandler("192.0.2.20", &secondCount))
	defer stopSecond()

	r := NewResolver(ResolverConfig{Servers: []string{silent, first}, Timeout: 1})
	defer r.Close()

	records, err := r.ResolveA("one.example.net")
	require.NoError(t, err)
	assert.Equal(t, []string{"192.0.2.10"}, records, "the second server answers when the first does not")
	assert.Equal(t, int32(1), silentCount.Load(), "the first server was asked first")
	assert.Equal(t, int32(1), firstCount.Load())

	r.SetServers([]string{second})
	assert.Equal(t, []string{second}, r.Servers())
	records, err = r.ResolveA("two.example.net")
	require.NoError(t, err)
	assert.Equal(t, []string{"192.0.2.20"}, records, "the replaced list applies to the next query")
	assert.Equal(t, int32(1), firstCount.Load(), "the old server is no longer asked")

	r.SetServers(nil)
	_, err = r.ResolveA("three.example.net")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no DNS server configured")

	r.SetServers([]string{"127.0.0.1"})
	assert.Equal(t, []string{"127.0.0.1:53"}, r.Servers(), "a server with no port is asked on 53")
}

// failingHandler answers every query with rcode and no record, counting the
// queries.
func failingHandler(rcode int, count *atomic.Int32) mdns.HandlerFunc {
	return func(w mdns.ResponseWriter, req *mdns.Msg) {
		count.Add(1)
		m := new(mdns.Msg)
		m.SetRcode(req, rcode)
		_ = w.WriteMsg(m)
	}
}

// TestResolverPassesOverFailingServer: a server that answers SERVFAIL or
// REFUSED is passed over, and the query takes the next server's answer.
//
// VALIDATES: RFC 1034 Section 5.3.3, "if the response shows a servers failure
// or other bizarre contents, delete the server from the SLIST and go back to
// step 3.", at the resolver the hub hands the router selection to
// (spec-home-cpe-23a review ISSUE 1).
// PREVENTS: an upstream that answers SERVFAIL or REFUSED ending the query
// with an empty result while a later server would have answered.
func TestResolverPassesOverFailingServer(t *testing.T) {
	cases := []struct {
		name  string
		rcode int
	}{
		{name: "servfail", rcode: mdns.RcodeServerFailure},
		{name: "refused", rcode: mdns.RcodeRefused},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var failingCount, answeringCount atomic.Int32
			failing, stopFailing := testDNSServer(t, failingHandler(tc.rcode, &failingCount))
			defer stopFailing()
			answering, stopAnswering := testDNSServer(t, countingHandler("192.0.2.30", &answeringCount))
			defer stopAnswering()

			r := NewResolver(ResolverConfig{Servers: []string{failing, answering}, Timeout: 1})
			defer r.Close()

			records, _, status, err := r.ResolveWithTTL("next.example.net", mdns.TypeA)
			require.NoError(t, err)
			assert.Equal(t, StatusSuccess, status, "the second server's NOERROR is the answer")
			assert.Equal(t, []string{"192.0.2.30"}, records, "the second server answers when the first answers %s", tc.name)
			assert.Equal(t, int32(1), failingCount.Load(), "the failing server was asked first")
			assert.Equal(t, int32(1), answeringCount.Load(), "the next server was asked")
		})
	}
}

// TestResolverEveryServerFailing: when every server answers a server
// failure, each is asked once and the caller gets the last failure as a
// status no caller acts on, never a NOERROR with no record.
//
// VALIDATES: RFC 1034 Section 5.3.3 (every server deleted from the SLIST).
// PREVENTS: an outage of every upstream read as a name that holds no record.
func TestResolverEveryServerFailing(t *testing.T) {
	var servfailCount, refusedCount atomic.Int32
	servfail, stopServfail := testDNSServer(t, failingHandler(mdns.RcodeServerFailure, &servfailCount))
	defer stopServfail()
	refused, stopRefused := testDNSServer(t, failingHandler(mdns.RcodeRefused, &refusedCount))
	defer stopRefused()

	r := NewResolver(ResolverConfig{Servers: []string{servfail, refused}, Timeout: 1})
	defer r.Close()

	records, ttl, status, err := r.ResolveWithTTL("down.example.net", mdns.TypeA)
	require.NoError(t, err, "a server failure is an answer, not a transport error")
	assert.Equal(t, StatusRefused, status, "the last server's failure is the status")
	assert.False(t, status.Authoritative(), "a server failure says nothing about the name")
	assert.Empty(t, records)
	assert.Zero(t, ttl)
	assert.Equal(t, int32(1), servfailCount.Load(), "the first server was asked")
	assert.Equal(t, int32(1), refusedCount.Load(), "the second server was asked after the first failed")
}
