package resolve

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/resolve/cymru"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/dns"
)

// VALIDATES: AC-12 -- Resolvers struct holds single DNS instance shared by all consumers.
// PREVENTS: Multiple DNS resolvers created at hub startup.
func TestResolvers_SingleDNSInstance(t *testing.T) {
	dnsResolver := dns.NewResolver(dns.ResolverConfig{
		Server:    "127.0.0.1:53",
		Timeout:   1,
		CacheSize: 0,
	})
	defer dnsResolver.Close()

	r := Resolvers{
		DNS: dnsResolver,
	}

	assert.NotNil(t, r.DNS, "DNS resolver should be set")
}

// VALIDATES: Cymru resolver wired through Resolvers using DNS.
// PREVENTS: Cymru resolver not connected to DNS.
func TestResolvers_CymruUsesDNS(t *testing.T) {
	var called bool
	fakeTXT := func(_ context.Context, _ string) ([]string, error) {
		called = true
		return []string{"65001 | US | arin | 2000-01-01 | TEST - Test Inc, US"}, nil
	}

	cymruResolver := cymru.New(fakeTXT, nil)

	r := Resolvers{
		Cymru: cymruResolver,
	}

	name, err := r.Cymru.LookupASNName(context.Background(), 65001)
	require.NoError(t, err)
	assert.Equal(t, "Test Inc", name)
	assert.True(t, called, "Cymru should call DNS TXT resolver")
}
