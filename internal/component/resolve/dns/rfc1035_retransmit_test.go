package dns

import (
	"sync/atomic"
	"testing"

	mdns "github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dropFirstHandler sends no reply to the first query it receives, as if the
// datagram were lost, and answers every later A query with addr. It counts
// every query.
func dropFirstHandler(addr string, count *atomic.Int32) mdns.HandlerFunc {
	answer := countingHandler(addr, new(atomic.Int32))
	return func(w mdns.ResponseWriter, req *mdns.Msg) {
		if count.Add(1) == 1 {
			return
		}
		answer(w, req)
	}
}

// TestRFC1035StubRetransmitsALostQuery proves RFC 1035 Section 4.2.1,
// "Queries sent using UDP may be lost, and hence a retransmission strategy
// is required.", at the stub resolver Ze's own components ask.
//
// Method: one server, a 1 second timeout per attempt. The server drops the
// first query and answers the next, so the query is answered by the
// retransmission. The negative is the bound: a server that never answers is
// asked queryTries (2) times, no more, and the query fails with an error.
func TestRFC1035StubRetransmitsALostQuery(t *testing.T) {
	var lossyCount atomic.Int32
	lossy, stopLossy := testDNSServer(t, dropFirstHandler("192.0.2.40", &lossyCount))
	defer stopLossy()

	r := NewResolver(ResolverConfig{Servers: []string{lossy}, Timeout: 1})
	defer r.Close()
	records, err := r.ResolveA("lost.example.net")

	// RFC requirement: RFC1035-4.2.1-4 positive -- a single server that drops the first UDP query for lost.example.net is sent it again, and the answer is its A record 192.0.2.40 after two queries.
	require.NoError(t, err)
	assert.Equal(t, []string{"192.0.2.40"}, records, "the retransmitted query is answered")
	assert.Equal(t, int32(2), lossyCount.Load(), "the lost query and its retransmission")

	silent, silentCount, stopSilent := silentServer(t)
	defer stopSilent()
	r.SetServers([]string{silent})
	_, err = r.ResolveA("gone.example.net")

	// RFC requirement: RFC1035-4.2.1-4 negative -- retransmission is bounded: a server that never answers is asked exactly twice for gone.example.net, and the query then fails with an error.
	require.Error(t, err)
	assert.Equal(t, int32(2), silentCount.Load(), "a silent server is asked queryTries times, no more")
}
