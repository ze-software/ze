// Design: plan/spec-prefix-data.md -- PeeringDB client for prefix update
//
// Package peeringdb provides a client for querying PeeringDB-compatible APIs
// for per-ASN prefix counts. Used by the prefix update command to suggest
// prefix maximum values.
package peeringdb

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"time"
)

const defaultTimeout = 10 * time.Second

// PrefixCounts holds per-family prefix counts returned by PeeringDB.
type PrefixCounts struct {
	IPv4 uint32
	IPv6 uint32
}

// Suspicious reports whether both counts are zero, which typically means
// the ASN has no data in PeeringDB rather than genuinely zero prefixes.
func (p PrefixCounts) Suspicious() bool {
	return p.IPv4 == 0 && p.IPv6 == 0
}

// apiResponse and netRecord use PeeringDB's snake_case field names
// (info_prefixes4, info_prefixes6). These are external API fields,
// not ze's own JSON format which uses kebab-case.

// PeeringDB queries a PeeringDB-compatible API for prefix counts.
type PeeringDB struct {
	baseURL string
	http    *http.Client
}

// NewPeeringDB creates a PeeringDB client for the given base URL.
// For localhost (127.0.0.1) URLs, TLS certificate validation is skipped
// to support functional tests with fake servers.
func NewPeeringDB(baseURL string) *PeeringDB {
	transport := &http.Transport{}
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = dt.Clone()
	}

	// Skip TLS verification for localhost only.
	parsed, err := url.Parse(baseURL)
	if err == nil {
		host, _, _ := net.SplitHostPort(parsed.Host)
		if host == "" {
			host = parsed.Host
		}
		if host == "127.0.0.1" {
			transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // localhost only, for testing with fake PeeringDB
			}
		}
	}

	return &PeeringDB{
		baseURL: baseURL,
		http: &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
		},
	}
}

// LookupASN queries PeeringDB for prefix counts of the given ASN.
// Returns an error if the ASN is not found in PeeringDB.
func (c *PeeringDB) LookupASN(ctx context.Context, asn uint32) (PrefixCounts, error) {
	reqURL := fmt.Sprintf("%s/api/net?asn=%d", c.baseURL, asn)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return PrefixCounts{}, fmt.Errorf("peeringdb: create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return PrefixCounts{}, fmt.Errorf("peeringdb: query ASN %d: %w", asn, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return PrefixCounts{}, fmt.Errorf("peeringdb: ASN %d: HTTP %d", asn, resp.StatusCode)
	}

	return decodePeeringDBResponse(resp, asn)
}

// maxResponseSize limits PeeringDB response bodies to 1 MB.
const maxResponseSize = 1 << 20

// decodePeeringDBResponse extracts prefix counts from a PeeringDB API response.
// Uses raw JSON decoding to avoid struct tags with PeeringDB's snake_case fields.
func decodePeeringDBResponse(resp *http.Response, asn uint32) (PrefixCounts, error) {
	var raw struct {
		Data []json.RawMessage `json:"data"`
	}
	limited := io.LimitReader(resp.Body, maxResponseSize)
	if err := json.NewDecoder(limited).Decode(&raw); err != nil {
		return PrefixCounts{}, fmt.Errorf("peeringdb: ASN %d: decode: %w", asn, err)
	}

	if len(raw.Data) == 0 {
		return PrefixCounts{}, fmt.Errorf("peeringdb: ASN %d: not found", asn)
	}

	// Decode first record as generic map to handle PeeringDB's snake_case fields.
	var fields map[string]any
	if err := json.Unmarshal(raw.Data[0], &fields); err != nil {
		return PrefixCounts{}, fmt.Errorf("peeringdb: ASN %d: decode record: %w", asn, err)
	}

	return PrefixCounts{
		IPv4: jsonUint32(fields, "info_prefixes4"),
		IPv6: jsonUint32(fields, "info_prefixes6"),
	}, nil
}

// jsonUint32 extracts a uint32 from a JSON map field. Returns 0 if missing or not a number.
func jsonUint32(m map[string]any, key string) uint32 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	if f < 0 {
		return 0
	}
	return uint32(f)
}

// ApplyMargin returns count increased by the given percentage margin.
// For example, ApplyMargin(1000, 10) returns 1100.
// Uses uint64 intermediate to avoid overflow for large prefix counts.
// Result capped at math.MaxUint32.
func ApplyMargin(count uint32, margin uint8) uint32 {
	result := uint64(count) + uint64(count)*uint64(margin)/100
	if result > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(result)
}
