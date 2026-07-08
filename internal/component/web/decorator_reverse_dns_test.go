// Design: (none -- test for reverse-dns decorator)

package web

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReverseDNSDecorator verifies an IP address resolves to its PTR hostname,
// with the trailing dot stripped for display.
//
// VALIDATES: L119 decorators-v2 -- reverse-dns decorator annotates an IP leaf
// with its reverse-DNS name.
// PREVENTS: the decorator returning empty when PTR resolution succeeds, or
// leaking the FQDN trailing dot into rendered output.
func TestReverseDNSDecorator(t *testing.T) {
	d := newReverseDNSDecorator(func(addr string) ([]string, error) {
		if addr == "192.0.2.1" {
			return []string{"host.example.com."}, nil
		}
		return nil, nil
	})

	got, err := d.Decorate("192.0.2.1")
	require.NoError(t, err)
	assert.Equal(t, "host.example.com", got, "trailing dot must be stripped")
	assert.Equal(t, "reverse-dns", d.Name())
}

// TestReverseDNSDecoratorGraceful verifies graceful degradation: DNS failures,
// empty results, and non-IP inputs produce no annotation and no error, and a
// non-IP or empty value never hits the resolver.
//
// VALIDATES: L119 -- reverse-dns fails soft (display enrichment must never error).
// PREVENTS: DNS errors propagating to output, or a lookup on non-address leaves.
func TestReverseDNSDecoratorGraceful(t *testing.T) {
	tests := []struct {
		name    string
		resolve ptrResolver
		input   string
		want    string
	}{
		{
			name:    "DNS error",
			resolve: func(string) ([]string, error) { return nil, errors.New("timeout") },
			input:   "192.0.2.1",
			want:    "",
		},
		{
			name:    "no records",
			resolve: func(string) ([]string, error) { return nil, nil },
			input:   "192.0.2.1",
			want:    "",
		},
		{
			name: "non-IP input skips lookup",
			resolve: func(string) ([]string, error) {
				t.Error("resolver must not be called for non-IP input")
				return nil, nil
			},
			input: "not-an-ip",
			want:  "",
		},
		{
			name: "empty value skips lookup",
			resolve: func(string) ([]string, error) {
				t.Error("resolver must not be called for empty value")
				return nil, nil
			},
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newReverseDNSDecorator(tt.resolve)
			got, err := d.Decorate(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
