package cymru

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDNS returns a TXT resolver that returns deterministic Cymru-formatted responses.
func fakeDNS(responses map[string][]string) TXTResolver {
	return func(_ context.Context, name string) ([]string, error) {
		records, ok := responses[name]
		if !ok {
			return nil, nil
		}
		return records, nil
	}
}

// VALIDATES: AC-3 -- Cymru LookupASNName with valid ASN returns org name.
// PREVENTS: Cymru resolver returning empty for valid ASN.
func TestCymruASNName(t *testing.T) {
	dns := fakeDNS(map[string][]string{
		"AS65300.asn.cymru.com.": {"65300 | US | arin | 2000-01-01 | TESTNET - Test Network Inc, US"},
	})

	r := New(dns, nil)
	name, err := r.LookupASNName(context.Background(), 65300)

	require.NoError(t, err)
	assert.Equal(t, "Test Network Inc", name)
}

// VALIDATES: AC-3 -- org name extracted when no dash separator (e.g., "CLOUDFLARENET").
// PREVENTS: empty result for label-only format.
func TestCymruASNNameNoDash(t *testing.T) {
	dns := fakeDNS(map[string][]string{
		"AS13335.asn.cymru.com.": {"13335 | US | arin | 2010-07-14 | CLOUDFLARENET"},
	})

	r := New(dns, nil)
	name, err := r.LookupASNName(context.Background(), 13335)

	require.NoError(t, err)
	assert.Equal(t, "CLOUDFLARENET", name)
}

// VALIDATES: AC-4 -- DNS failure returns empty string (graceful degradation).
// PREVENTS: Cymru errors propagated to caller.
func TestCymruGracefulDegradation_DNSFailure(t *testing.T) {
	dns := func(_ context.Context, _ string) ([]string, error) {
		return nil, fmt.Errorf("network error")
	}

	r := New(dns, nil)
	name, err := r.LookupASNName(context.Background(), 65001)

	require.NoError(t, err)
	assert.Empty(t, name)
}

// VALIDATES: AC-4 -- empty DNS response returns empty string.
// PREVENTS: panic on empty TXT records.
func TestCymruGracefulDegradation_EmptyResponse(t *testing.T) {
	dns := fakeDNS(map[string][]string{})

	r := New(dns, nil)
	name, err := r.LookupASNName(context.Background(), 99999)

	require.NoError(t, err)
	assert.Empty(t, name)
}

// VALIDATES: AC-4 -- malformed Cymru TXT response returns empty string.
// PREVENTS: panic on unexpected response format.
func TestCymruGracefulDegradation_MalformedResponse(t *testing.T) {
	dns := fakeDNS(map[string][]string{
		"AS65001.asn.cymru.com.": {"this is not a valid cymru response"},
	})

	r := New(dns, nil)
	name, err := r.LookupASNName(context.Background(), 65001)

	require.NoError(t, err)
	assert.Empty(t, name)
}

// VALIDATES: AC-8 -- second call returns cached result, no DNS query.
// PREVENTS: redundant DNS queries for recently resolved ASNs.
func TestCymruCache(t *testing.T) {
	var queryCount atomic.Int32
	dns := func(_ context.Context, name string) ([]string, error) {
		queryCount.Add(1)
		if name == "AS65001.asn.cymru.com." {
			return []string{"65001 | US | arin | 2000-01-01 | TEST - Test Inc, US"}, nil
		}
		return nil, nil
	}

	r := New(dns, nil) // nil cache means use default (1h)
	name1, err1 := r.LookupASNName(context.Background(), 65001)
	require.NoError(t, err1)
	assert.Equal(t, "Test Inc", name1)
	assert.Equal(t, int32(1), queryCount.Load(), "first query should hit DNS")

	name2, err2 := r.LookupASNName(context.Background(), 65001)
	require.NoError(t, err2)
	assert.Equal(t, "Test Inc", name2)
	assert.Equal(t, int32(1), queryCount.Load(), "second query should come from cache")
}

// VALIDATES: boundary -- max ASN (4294967295) resolves correctly.
// PREVENTS: uint32 overflow in query formatting.
func TestCymruASNMax(t *testing.T) {
	dns := fakeDNS(map[string][]string{
		"AS4294967295.asn.cymru.com.": {"4294967295 | ZZ | iana | 2000-01-01 | MAX-ASN - Max ASN Test, ZZ"},
	})

	r := New(dns, nil)
	name, err := r.LookupASNName(context.Background(), 4294967295)

	require.NoError(t, err)
	assert.Equal(t, "Max ASN Test", name)
}

// VALIDATES: ASN 0 is a valid query (edge case).
// PREVENTS: zero-ASN rejection.
func TestCymruASNZero(t *testing.T) {
	dns := fakeDNS(map[string][]string{
		"AS0.asn.cymru.com.": {"0 | ZZ | iana | 2000-01-01 | RESERVED"},
	})

	r := New(dns, nil)
	name, err := r.LookupASNName(context.Background(), 0)

	require.NoError(t, err)
	assert.Equal(t, "RESERVED", name)
}

func TestBuildOrigin4Query(t *testing.T) {
	q := buildOrigin4Query("1.2.3.4")
	assert.Equal(t, "4.3.2.1.origin.asn.cymru.com.", q)
}

func TestBuildOrigin4QueryInvalid(t *testing.T) {
	assert.Empty(t, buildOrigin4Query("1.2.3"))
}

func TestBuildOrigin6Query(t *testing.T) {
	q := buildOrigin6Query("2001:db8::1")
	assert.Contains(t, q, ".origin6.asn.cymru.com.")
	assert.Contains(t, q, "1.0.0.0.0.0.0.0")
}

func TestBuildOrigin6QueryInvalid(t *testing.T) {
	assert.Empty(t, buildOrigin6Query("not-an-ip"))
}

func TestParseOriginResponse(t *testing.T) {
	o := parseOriginResponse("13335 | 1.1.1.0/24 | US | arin | 2014-03-28")
	assert.Equal(t, uint32(13335), o.ASN)
	assert.Equal(t, "1.1.1.0/24", o.Prefix)
}

func TestParseOriginResponseMalformed(t *testing.T) {
	o := parseOriginResponse("garbage")
	assert.Equal(t, uint32(0), o.ASN)
}

func TestLookupOrigin(t *testing.T) {
	dns := fakeDNS(map[string][]string{
		"4.3.2.1.origin.asn.cymru.com.": {"65001 | 1.2.3.0/24 | US | arin | 2000-01-01"},
		"AS65001.asn.cymru.com.":        {"65001 | US | arin | 2000-01-01 | EXAMPLE - Example Corp, US"},
	})

	r := New(dns, nil)
	o, err := r.LookupOrigin(context.Background(), "1.2.3.4")

	require.NoError(t, err)
	assert.Equal(t, uint32(65001), o.ASN)
	assert.Equal(t, "1.2.3.0/24", o.Prefix)
	assert.Equal(t, "Example Corp", o.Name)
}

func TestLookupOriginDNSFailure(t *testing.T) {
	dns := func(_ context.Context, _ string) ([]string, error) {
		return nil, fmt.Errorf("network error")
	}

	r := New(dns, nil)
	o, err := r.LookupOrigin(context.Background(), "1.2.3.4")

	require.NoError(t, err)
	assert.Equal(t, uint32(0), o.ASN)
}

func TestLookupOriginCached(t *testing.T) {
	var queryCount atomic.Int32
	dns := func(_ context.Context, name string) ([]string, error) {
		queryCount.Add(1)
		switch name {
		case "4.3.2.1.origin.asn.cymru.com.":
			return []string{"65001 | 1.2.3.0/24 | US | arin | 2000-01-01"}, nil
		case "AS65001.asn.cymru.com.":
			return []string{"65001 | US | arin | 2000-01-01 | TEST - Test Inc, US"}, nil
		}
		return nil, nil
	}

	r := New(dns, nil)
	o1, _ := r.LookupOrigin(context.Background(), "1.2.3.4")
	assert.Equal(t, uint32(65001), o1.ASN)
	first := queryCount.Load()

	o2, _ := r.LookupOrigin(context.Background(), "1.2.3.4")
	assert.Equal(t, uint32(65001), o2.ASN)
	assert.Equal(t, first, queryCount.Load(), "second call should use cache")
}
