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
