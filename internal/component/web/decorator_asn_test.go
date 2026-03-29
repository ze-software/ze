// Design: (none -- new component, predates documentation)

package web

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestASNDecoratorParseCymru verifies Team Cymru TXT response parsing.
// VALIDATES: AC-6 -- organization name extracted from pipe-delimited TXT record.
// PREVENTS: Wrong field extracted from Cymru response.
func TestASNDecoratorParseCymru(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{
			name:   "standard response",
			input:  "64500 | US | arin | 2005-06-01 | CLOUDFLARE - Cloudflare, Inc., US",
			want:   "Cloudflare, Inc.",
			wantOK: true,
		},
		{
			name:   "short name without comma",
			input:  "13335 | US | arin | 2010-07-14 | CLOUDFLARENET",
			want:   "CLOUDFLARENET",
			wantOK: true,
		},
		{
			name:   "name with dash prefix",
			input:  "15169 | US | arin | 2000-03-30 | GOOGLE - Google LLC, US",
			want:   "Google LLC",
			wantOK: true,
		},
		{
			name:   "empty response",
			input:  "",
			wantOK: false,
		},
		{
			name:   "too few fields",
			input:  "64500 | US",
			wantOK: false,
		},
		{
			name:   "name field empty",
			input:  "64500 | US | arin | 2005-06-01 | ",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseASNName(tt.input)
			assert.Equal(t, tt.wantOK, ok, "parseASNName ok")
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestASNDecorator verifies that the ASN decorator resolves names via DNS.
// VALIDATES: AC-1 -- YANG leaf with ze:decorate "asn-name" rendered with org name.
// PREVENTS: Decorator returns empty when DNS succeeds.
func TestASNDecorator(t *testing.T) {
	// Use a fake resolver function to avoid real DNS.
	d := newASNNameDecorator(func(name string) ([]string, error) {
		if name == "AS64500.asn.cymru.com." {
			return []string{"64500 | US | arin | 2005-06-01 | CLOUDFLARE - Cloudflare, Inc., US"}, nil
		}
		return nil, nil
	})

	result, err := d.Decorate("64500")
	require.NoError(t, err)
	assert.Equal(t, "Cloudflare, Inc.", result)
	assert.Equal(t, "asn-name", d.Name())
}

// TestASNDecoratorFailure verifies graceful degradation on DNS failure.
// VALIDATES: AC-4 -- DNS timeout/NXDOMAIN results in no annotation, not error.
// PREVENTS: DNS errors propagating to rendered output.
func TestASNDecoratorFailure(t *testing.T) {
	tests := []struct {
		name       string
		resolveFn  func(string) ([]string, error)
		input      string
		wantResult string
	}{
		{
			name: "DNS error",
			resolveFn: func(string) ([]string, error) {
				return nil, errors.New("timeout")
			},
			input:      "64500",
			wantResult: "",
		},
		{
			name: "NXDOMAIN (empty result)",
			resolveFn: func(string) ([]string, error) {
				return nil, nil
			},
			input:      "64500",
			wantResult: "",
		},
		{
			name: "non-numeric ASN",
			resolveFn: func(string) ([]string, error) {
				t.Error("resolver should not be called for non-numeric ASN")
				return nil, nil
			},
			input:      "not-a-number",
			wantResult: "",
		},
		{
			name: "empty value",
			resolveFn: func(string) ([]string, error) {
				t.Error("resolver should not be called for empty value")
				return nil, nil
			},
			input:      "",
			wantResult: "",
		},
		{
			name: "ASN too large",
			resolveFn: func(string) ([]string, error) {
				t.Error("resolver should not be called for out-of-range ASN")
				return nil, nil
			},
			input:      "4294967296",
			wantResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newASNNameDecorator(tt.resolveFn)
			result, err := d.Decorate(tt.input)
			require.NoError(t, err, "decorator should not return errors")
			assert.Equal(t, tt.wantResult, result)
		})
	}
}

// TestASNDecoratorBoundary verifies boundary ASN values.
// VALIDATES: Boundary testing for ASN range 0-4294967295.
// PREVENTS: Off-by-one in ASN validation.
func TestASNDecoratorBoundary(t *testing.T) {
	queriedASN := ""
	d := newASNNameDecorator(func(name string) ([]string, error) {
		queriedASN = name
		return []string{"0 | ZZ | iana | 2000-01-01 | RESERVED"}, nil
	})

	tests := []struct {
		name      string
		input     string
		wantQuery bool
	}{
		{name: "zero (valid)", input: "0", wantQuery: true},
		{name: "max uint32 (valid)", input: "4294967295", wantQuery: true},
		{name: "max+1 (invalid)", input: "4294967296", wantQuery: false},
		{name: "negative (invalid)", input: "-1", wantQuery: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queriedASN = ""
			result, err := d.Decorate(tt.input)
			require.NoError(t, err)
			if tt.wantQuery {
				assert.NotEmpty(t, queriedASN, "should have queried DNS")
			} else {
				assert.Empty(t, queriedASN, "should not have queried DNS")
				assert.Empty(t, result, "invalid ASN should return empty result")
			}
		})
	}
}

// TestNewASNNameDecoratorFromResolver verifies the public constructor works
// with a resolver interface.
// VALIDATES: AC-1 -- public API creates working ASN decorator.
// PREVENTS: Signature mismatch in public constructor going unnoticed.
func TestNewASNNameDecoratorFromResolver(t *testing.T) {
	resolver := &fakeResolver{
		fn: func(name string) ([]string, error) {
			if name == "AS64500.asn.cymru.com." {
				return []string{"64500 | US | arin | 2005-06-01 | EXAMPLE - Example Corp, US"}, nil
			}
			return nil, nil
		},
	}

	d := NewASNNameDecoratorFromResolver(resolver)
	assert.Equal(t, "asn-name", d.Name())

	result, err := d.Decorate("64500")
	require.NoError(t, err)
	assert.Equal(t, "Example Corp", result)
}

// fakeResolver implements the interface expected by NewASNNameDecoratorFromResolver.
type fakeResolver struct {
	fn func(string) ([]string, error)
}

func (f *fakeResolver) ResolveTXT(name string) ([]string, error) { return f.fn(name) }
