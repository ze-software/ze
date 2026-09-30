// Design: docs/architecture/dns/secure-transports.md -- the DoH GET "dns" variable
// RFC: rfc/short/rfc8484.md -- RFC8484-6-2, the base64url alphabet and the variable name

package dnsserver

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// alphabetQuery packs an A query whose first two octets (the message ID) are
// 0xfb 0xff. Their first twelve bits are 111110 111111, the two 6-bit values
// (62 and 63) on which the base64 alphabets differ: base64url writes them '-'
// and '_', standard base64 writes '+' and '/'. The encoded value therefore
// names which alphabet produced it.
func alphabetQuery(t *testing.T) []byte {
	t.Helper()
	wire := mustPack(t, "alphabet.test")
	wire[0], wire[1] = 0xfb, 0xff
	return wire
}

// dohGetVariable sends a DoH GET carrying value under the query variable name
// and returns the response status.
func dohGetVariable(t *testing.T, client *http.Client, port uint16, name, value string) int {
	t.Helper()
	u := url.URL{Scheme: "https", Host: hostPort(port), Path: DefaultDoHPath}
	q := u.Query()
	q.Set(name, value)
	u.RawQuery = q.Encode()
	resp := dohGet(t, client, u.String())
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

// VALIDATES: RFC 8484 section 6 over both of its clauses. The GET payload is
// read as base64url and only as base64url, and only from the variable named
// "dns". Method: one query whose encoding differs between the two alphabets
// (see alphabetQuery) is sent three ways through a real DoH listener.
// PREVENTS: a server that decodes standard base64, which the '*' value of
// TestDoHGetRejectsBadDNSParam cannot catch because '*' is in neither alphabet,
// and a server that reads the payload from a variable other than "dns".
// RFC requirement: RFC8484-6-2 positive -- a query whose base64url encoding holds '-' and '_' (octets 0xfb 0xff) sent under "dns" is decoded and answered (200, the A record for the queried name).
// RFC requirement: RFC8484-6-2 negative -- the same octets in the standard base64 alphabet ('+' and '/') under "dns" are refused (400), and the base64url value under a variable named "query" instead of "dns" is refused (400).
func TestRFC8484GetPayloadIsBase64URLInTheDNSVariable(t *testing.T) {
	port, client := startDoH(t, echoHandler("10.0.0.9"))
	query := alphabetQuery(t)

	urlSafe := base64.RawURLEncoding.EncodeToString(query)
	standard := base64.RawStdEncoding.EncodeToString(query)
	if !strings.HasPrefix(urlSafe, "-_") || !strings.HasPrefix(standard, "+/") {
		t.Fatalf("encodings %q and %q do not differ on the alphabet-specific characters", urlSafe, standard)
	}

	u := url.URL{Scheme: "https", Host: hostPort(port), Path: DefaultDoHPath}
	q := u.Query()
	q.Set("dns", urlSafe)
	u.RawQuery = q.Encode()
	resp := dohGet(t, client, u.String())
	defer func() { _ = resp.Body.Close() }()
	assertDoHAnswer(t, resp, "10.0.0.9")

	if status := dohGetVariable(t, client, port, "dns", standard); status != http.StatusBadRequest {
		t.Errorf("standard base64 under dns: status = %d, want 400", status)
	}
	if status := dohGetVariable(t, client, port, "query", urlSafe); status != http.StatusBadRequest {
		t.Errorf("base64url under query: status = %d, want 400", status)
	}
}
