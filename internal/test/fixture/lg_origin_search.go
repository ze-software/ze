// Design: docs/architecture/web-interface.md -- the end-anchored AS path search through a real bgp-rib
// Related: lg_aspath_search.go -- lgASPathSearchInject and lgASPathSearchRows, the injector and the page reader this file shares
// Related: plugin_fixture_09.go -- the registration and observe09

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The routes lgOriginSearch injects, all under addrLGPeerFirst, each with the
// AS path it carries. 65001 originates three of them: behind one AS, behind
// two, and prepended three times. 10.40.2.0/24 crosses 65001 and is originated
// by 65002, so an end anchor has a route to leave out.
var lgOriginSearchRoutes = []struct {
	prefix, path string
}{
	{"10.40.1.0/24", "65010,65001"},
	{"10.40.2.0/24", "65001,65002"},
	{"10.40.3.0/24", "64999,65010,65001"},
	{"10.40.4.0/24", "65010,65001,65001,65001"},
}

// What each end-anchored AS path lists, as a sorted set: the order of the rows
// is the plugin's and is not asserted. The path anchored at both ends lists
// the one route whose path is those two AS numbers and nothing else, so the
// longer path and the prepended path that both hold the pair stay out.
var lgOriginSearchWant = []struct {
	aspath   string
	prefixes []string
}{
	{"65001$", []string{"10.40.1.0/24", "10.40.3.0/24", "10.40.4.0/24"}},
	{"^65010 65001$", []string{"10.40.1.0/24"}},
}

// lgOriginSearch injects routes that 65001 originates and one that only
// crosses it, then asks the search page for AS paths anchored at the end and
// checks the routes each one lists, absences included.
func lgOriginSearch(ctx context.Context, _ []string) error {
	return observe09(ctx, "lg-origin-search", func(ctx context.Context, p *sdk.Plugin) error {
		if err := p12WaitPeersEOR(ctx, p, "peer1"); err != nil {
			return err
		}
		for _, route := range lgOriginSearchRoutes {
			if err := lgASPathSearchInject(ctx, p, route.prefix, route.path); err != nil {
				return err
			}
		}
		port := os.Getenv("ZE_LG_PORT")
		if port == "" {
			return errors.New("origin search: ZE_LG_PORT not set in plugin environment")
		}
		base := "http://127.0.0.1:" + port

		for _, question := range lgOriginSearchWant {
			body, err := lgCoveringRequest(ctx, http.MethodPost, base+"/lg/search",
				url.Values{"aspath": {question.aspath}}.Encode())
			if err != nil {
				return err
			}
			got, err := lgASPathSearchRows(question.aspath, body)
			if err != nil {
				return err
			}
			if !slices.Equal(got, question.prefixes) {
				return fmt.Errorf("origin search: AS path %q listed %v, want %v", question.aspath, got, question.prefixes)
			}
		}
		fmt.Fprintln(os.Stderr, "OK: end-anchored AS path searches listed the routes 65001 originates and left out the route that crosses it")
		return nil
	})
}
