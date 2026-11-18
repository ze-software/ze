// Design: docs/architecture/web-interface.md -- the AS path search through a real bgp-rib
// Related: lg_covering_lookup.go -- lgCoveringRequest and lgCoveringCell, the page reader this file shares
// Related: plugin_fixture_09.go -- the registration, observe09 and the other looking glass drivers

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// lgASPathSearchPeer holds every route lgASPathSearch injects. Its next hop
// is the peer, as injectRoute09 sets it.
const lgASPathSearchPeer = "10.0.1.1"

// The routes lgASPathSearch injects, each with the AS path it carries.
// 10.30.3.0/24 holds 65002 then 65001, the reverse of the run the search
// asks for, and 10.30.2.0/24 holds 65001 after another AS, so the anchor has
// a route to leave out.
var lgASPathSearchRoutes = []struct {
	prefix, path string
}{
	{"10.30.1.0/24", "65001,65002,65003"},
	{"10.30.2.0/24", "64999,65001,65002"},
	{"10.30.3.0/24", "65002,65001"},
	{"10.30.4.0/24", "65001"},
}

// What each AS path typed in the search page lists, as a sorted set: the
// order of the rows is the plugin's and is not asserted.
var lgASPathSearchWant = []struct {
	aspath   string
	prefixes []string
}{
	{"65001 65002", []string{"10.30.1.0/24", "10.30.2.0/24"}},
	{"^65001", []string{"10.30.1.0/24", "10.30.4.0/24"}},
	{"^65001 65002", []string{"10.30.1.0/24"}},
}

// lgASPathSearchRefused is the AS path the page refuses, the end anchor the
// engine has no grammar for, and the lines its banner must carry.
const lgASPathSearchRefused = "65001$"

var lgASPathSearchRefusal = []string{
	`class="error-banner">AS path 65001$ holds 65001$, which is not an AS number`,
	"separated by spaces",
	"a leading ^ anchors the run at the start of the path",
	"fix: 65001 65002",
}

// lgASPathSearch injects routes with chosen AS paths, then asks the search
// page for AS paths typed with spaces and with an anchor, checks the routes
// each one lists, and checks that an end anchor is refused with the form the
// field takes.
func lgASPathSearch(ctx context.Context, _ []string) error {
	return observe09(ctx, "lg-aspath", func(ctx context.Context, p *sdk.Plugin) error {
		if err := p12WaitPeersEOR(ctx, p, "peer1"); err != nil {
			return err
		}
		for _, route := range lgASPathSearchRoutes {
			if err := lgASPathSearchInject(ctx, p, route.prefix, route.path); err != nil {
				return err
			}
		}
		port := os.Getenv("ZE_LG_PORT")
		if port == "" {
			return errors.New("aspath: ZE_LG_PORT not set in plugin environment")
		}
		base := "http://127.0.0.1:" + port

		for _, question := range lgASPathSearchWant {
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
				return fmt.Errorf("aspath: search for %q listed %v, want %v", question.aspath, got, question.prefixes)
			}
		}

		body, err := lgCoveringRequest(ctx, http.MethodPost, base+"/lg/search",
			url.Values{"aspath": {lgASPathSearchRefused}}.Encode())
		if err != nil {
			return err
		}
		for _, line := range lgASPathSearchRefusal {
			if !strings.Contains(body, line) {
				return fmt.Errorf("aspath: search for %q does not answer %q: %.600q", lgASPathSearchRefused, line, body)
			}
		}
		fmt.Fprintln(os.Stderr, "OK: AS path searches listed the routes of the run and refused the end anchor")
		return nil
	})
}

// lgASPathSearchInject injects one route from lgASPathSearchPeer with the AS
// path given in the inject command's comma form.
func lgASPathSearchInject(ctx context.Context, p *sdk.Plugin, prefix, path string) error {
	command := "request bgp rib inject " + lgASPathSearchPeer + " ipv4/unicast " + prefix +
		" origin igp nhop " + lgASPathSearchPeer + " aspath " + path
	r := dispatch09(ctx, p, command)
	if !done09(r) {
		return fmt.Errorf("%s: status=%s data=%s", command, r.status, r.text)
	}
	return nil
}

// lgASPathSearchRows answers the prefixes one search page listed, sorted. A
// page with an error banner is an error naming it, because every question
// here is one the page accepts.
func lgASPathSearchRows(aspath, body string) ([]string, error) {
	if strings.Contains(body, `class="error-banner"`) {
		return nil, fmt.Errorf("aspath: search for %q answered an error: %.600q", aspath, body)
	}
	start := strings.Index(body, "<tbody>")
	if start < 0 {
		return nil, fmt.Errorf("aspath: search for %q rendered no route table: %.400q", aspath, body)
	}
	end := strings.Index(body[start:], "</tbody>")
	if end < 0 {
		return nil, fmt.Errorf("aspath: search for %q rendered an unclosed route table: %.400q", aspath, body)
	}
	prefixes := []string{}
	for _, row := range strings.Split(body[start:start+end], "<tr")[1:] {
		cells := lgCoveringCell.FindAllStringSubmatch(row, -1)
		if len(cells) != 8 {
			return nil, fmt.Errorf("aspath: search for %q rendered a row with %d cells, want 8: %.300q", aspath, len(cells), row)
		}
		prefixes = append(prefixes, cells[1][1])
	}
	slices.Sort(prefixes)
	return prefixes, nil
}
