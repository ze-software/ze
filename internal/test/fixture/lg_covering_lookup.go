// Design: docs/architecture/api/birdwatcher-compat.md -- the covering lookup through a real bgp-rib
// Related: plugin_fixture_09.go -- the registration, observe09, injectRoute09 and the other looking glass drivers

package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The routes lgCoveringLookup injects, by the pseudo-peer that holds them.
// injectRoute09 sets each route's next hop to its peer, so a result row names
// its peer in the next-hop column on the page and in `gateway` in the API.
//
// 10.0.1.1 also holds 192.0.0.0/2 and 192.0.0.0/24, a pair whose text starts
// the same way: the text filter `prefix 192.0.0.0/2` lists both, and the
// exact lookup the looking glass sends lists the /2 alone.
var lgCoveringRoutes = map[string][]string{
	addrLGPeerFirst:  {prefixTen8, prefixTen1Of16, prefixDefaultIPv4, prefixTestNet1, prefix192Short, "192.0.0.0/24"},
	addrLGPeerSecond: {prefixTen8, prefixTestNet1},
}

// What each question answers, per peer, in the order the rows must come: the
// most specific prefix first within each peer. The order of the peers
// themselves is the plugin's and is not asserted.
var (
	lgCoveringAddressWant = map[string][]string{
		addrLGPeerFirst:  {prefixTen1Of16, prefixTen8, prefixDefaultIPv4},
		addrLGPeerSecond: {prefixTen8},
	}
	lgCoveringPrefixWant = map[string][]string{
		addrLGPeerFirst:  {prefixTen8},
		addrLGPeerSecond: {prefixTen8},
	}
	lgCoveringShortPrefixWant = map[string][]string{
		addrLGPeerFirst: {prefix192Short},
	}
)

// lgCoveringLookup injects nested prefixes from two peers, then asks the
// looking glass for an address and for prefixes with their length, through
// the search page and through both API endpoints, and checks the rows each one
// lists and their order. 192.0.0.0/2 must list that prefix and not
// 192.0.0.0/24, and 192.0.0.1/2 the same, its host bits cleared by the
// looking glass before bgp-rib, which refuses them, is asked.
func lgCoveringLookup(ctx context.Context, _ []string) error {
	return observe09(ctx, "lg-covering", func(ctx context.Context, p *sdk.Plugin) error {
		if err := p12WaitPeersEOR(ctx, p, "peer1"); err != nil {
			return err
		}
		for _, peer := range []string{addrLGPeerFirst, addrLGPeerSecond} {
			for _, prefix := range lgCoveringRoutes[peer] {
				if err := injectRoute09(ctx, p, peer, prefix); err != nil {
					return err
				}
			}
		}
		port := os.Getenv("ZE_LG_PORT")
		if port == "" {
			return fmt.Errorf("covering: ZE_LG_PORT not set in plugin environment")
		}
		base := "http://127.0.0.1:" + port

		questions := []struct {
			value string
			want  map[string][]string
		}{
			{"10.1.2.3", lgCoveringAddressWant},
			{prefixTen8, lgCoveringPrefixWant},
			{prefix192Short, lgCoveringShortPrefixWant},
			{"192.0.0.1/2", lgCoveringShortPrefixWant},
		}
		for _, question := range questions {
			page, err := lgCoveringSearchRows(ctx, base, question.value)
			if err != nil {
				return err
			}
			if err := lgCoveringCompare("POST /lg/search prefix="+question.value, page, question.want); err != nil {
				return err
			}
			for _, endpoint := range []string{"routes/prefix", "routes/search"} {
				api, err := lgCoveringAPIRows(ctx, base, endpoint, question.value)
				if err != nil {
					return err
				}
				if err := lgCoveringCompare(endpoint+"?prefix="+question.value, api, question.want); err != nil {
					return err
				}
			}
		}
		fmt.Fprintln(os.Stderr, "OK: covering and exact lookups listed the expected rows in order on the page and the API")
		return nil
	})
}

// lgCoveringCompare checks the rows one question listed against want, peer by
// peer, order included.
func lgCoveringCompare(question string, got, want map[string][]string) error {
	for peer, prefixes := range want {
		if !slices.Equal(got[peer], prefixes) {
			return fmt.Errorf("covering: %s listed %v for peer %s, want %v (all rows %v)", question, got[peer], peer, prefixes, got)
		}
	}
	for peer := range got {
		if _, expected := want[peer]; !expected {
			return fmt.Errorf("covering: %s listed rows for peer %s, which holds no matching route: %v", question, peer, got)
		}
	}
	return nil
}

// lgCoveringCell is one cell of the search page's route table
// (internal/component/lg/route_table.templ). A row holds eight: the best
// marker, the prefix, the next hop, the AS path, the origin, the local
// preference, the MED and the peer.
var lgCoveringCell = regexp.MustCompile(`<td>([^<]*)</td>`)

// lgCoveringSearchRows posts one search and answers the prefixes it listed,
// grouped by next hop in the order the page listed them.
func lgCoveringSearchRows(ctx context.Context, base, value string) (map[string][]string, error) {
	form := url.Values{columnPrefix: {value}}.Encode()
	body, err := lgCoveringRequest(ctx, http.MethodPost, base+"/lg/search", form)
	if err != nil {
		return nil, err
	}
	start := strings.Index(body, "<tbody>")
	if start < 0 {
		return nil, fmt.Errorf("covering: search for %s rendered no route table: %.400q", value, body)
	}
	end := strings.Index(body[start:], "</tbody>")
	if end < 0 {
		return nil, fmt.Errorf("covering: search for %s rendered an unclosed route table: %.400q", value, body)
	}
	rows := map[string][]string{}
	for _, row := range strings.Split(body[start:start+end], "<tr")[1:] {
		cells := lgCoveringCell.FindAllStringSubmatch(row, -1)
		if len(cells) != 8 {
			return nil, fmt.Errorf("covering: search for %s rendered a row with %d cells, want 8: %.300q", value, len(cells), row)
		}
		rows[cells[2][1]] = append(rows[cells[2][1]], cells[1][1])
	}
	return rows, nil
}

// lgCoveringAPIRows asks one birdwatcher endpoint and answers the networks it
// listed, grouped by gateway in the order the API listed them.
func lgCoveringAPIRows(ctx context.Context, base, endpoint, value string) (map[string][]string, error) {
	target := base + "/api/looking-glass/" + endpoint + "?" + url.Values{"prefix": {value}}.Encode()
	body, err := lgCoveringRequest(ctx, http.MethodGet, target, "")
	if err != nil {
		return nil, err
	}
	var answer struct {
		Routes []struct {
			Network string `json:"network"`
			Gateway string `json:"gateway"`
		} `json:"routes"`
		Count int `json:"routes_count"`
	}
	if err := json.Unmarshal([]byte(body), &answer); err != nil {
		return nil, fmt.Errorf("covering: %s?prefix=%s is not JSON: %w: %.300q", endpoint, value, err, body)
	}
	if answer.Count != len(answer.Routes) {
		return nil, fmt.Errorf("covering: %s?prefix=%s routes_count %d over %d routes", endpoint, value, answer.Count, len(answer.Routes))
	}
	rows := map[string][]string{}
	for _, route := range answer.Routes {
		rows[route.Gateway] = append(rows[route.Gateway], route.Network)
	}
	return rows, nil
}

// lgCoveringRequest sends one request to the looking glass and answers its
// body, retrying only while the listener is not yet up. Any answer that is
// not 200 is an error naming it.
func lgCoveringRequest(ctx context.Context, method, target, form string) (string, error) {
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	// The answer is read and its body closed inside the attempt that got it,
	// so no path leaves a body open. A read failure ends the polling: only a
	// listener that is not yet up is retried.
	var status int
	var data []byte
	var readErr error
	var lastErr error
	if !Poll(ctx, 75, 200*time.Millisecond, func() bool {
		var body io.Reader = http.NoBody
		if form != "" {
			body = strings.NewReader(form)
		}
		request, err := http.NewRequestWithContext(ctx, method, target, body)
		if err != nil {
			lastErr = err
			return false
		}
		if form != "" {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		resp, err := client.Do(request)
		if err != nil {
			lastErr = err
			return false
		}
		defer resp.Body.Close() //nolint:errcheck // the body is read
		status = resp.StatusCode
		data, readErr = io.ReadAll(resp.Body)
		return true
	}) {
		return "", fmt.Errorf("covering: looking glass never answered %s %s: %w", method, target, lastErr)
	}
	if readErr != nil {
		return "", fmt.Errorf("covering: read %s %s: %w", method, target, readErr)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("covering: %s %s answered %d: %.300q", method, target, status, data)
	}
	return string(data), nil
}
