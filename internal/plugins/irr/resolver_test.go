package irr

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// VALIDATES: the production resolver's AS-SET discovery for a bare AS number,
// through a real PeeringDB client over HTTP: the first AS-SET PeeringDB names,
// in the one spelling every key uses; the literal AS<asn> when PeeringDB holds
// the network and names no set; an error, deciding nothing, when PeeringDB
// does not answer. The owner's tests drive discovery through fakeResolver, so
// this is the test of storeResolver.discover itself (R1-2: it replaces the
// store's own discovery path and its tests).
// PREVENTS: a PeeringDB failure read as "no AS-SET", which would query the
// literal and flip the set an AS number is bound to on every outage; and a
// second AS-SET, or an unnormalised name, chosen over the first.
// METHOD: an httptest PeeringDB answering one ASN with two sets in lower case,
// one with an empty irr_as_set, and every other with a server error.
func TestStoreResolverDiscover(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body string
		switch r.URL.Query().Get("asn") {
		case "13335":
			body = `{"data":[{"irr_as_set":"as-cloudflare RIPE::AS-OTHER"}]}`
		case "65001":
			body = `{"data":[{"irr_as_set":""}]}`
		default:
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		if _, err := fmt.Fprint(w, body); err != nil {
			return
		}
	}))
	t.Cleanup(srv.Close)
	cfg := defaultOwnerConfig()
	cfg.peeringDBURL = srv.URL
	res := newStoreResolver(cfg)
	ctx := context.Background()

	if got, err := res.discover(ctx, 13335); err != nil || got != "AS-CLOUDFLARE" {
		t.Errorf("discover(13335) = %q, %v, want the first set PeeringDB names, AS-CLOUDFLARE", got, err)
	}
	if got, err := res.discover(ctx, 65001); err != nil || got != "AS65001" {
		t.Errorf("discover(65001) = %q, %v, want the literal AS65001 for a network naming no set", got, err)
	}
	if got, err := res.discover(ctx, 64999); err == nil {
		t.Errorf("discover(64999) = %q with no error, want PeeringDB's failure reported, never a guessed set", got)
	}
}
