// Design: docs/architecture/api/birdwatcher-compat.md -- the `age` of a route through a real bgp-rib
// Related: lg_aspath_search.go -- lgASPathSearchInject, the injector this file shares
// Related: lg_covering_lookup.go -- lgCoveringRequest, the looking glass reader this file shares
// Related: plugin_fixture_09.go -- the registration and observe09
// Related: ../../component/lg/route_table.templ -- the Age cell lgRouteAgePage reads

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// lgRouteAgePrefix is the one route whose age the scenario follows.
	lgRouteAgePrefix = "10.50.1.0/24"
	// lgRouteAgePath is the AS path the route is learned and re-announced
	// with, and lgRouteAgePathChanged the one that changes its attributes.
	lgRouteAgePath        = "65001,65002"
	lgRouteAgePathChanged = "65001,65003"
	// lgRouteAgeSettled is the age, in seconds, the route reaches before the
	// re-announcements. It is the margin of both verdicts: an age that was
	// reset reads at most the seconds since the reset, so it is told from an
	// age that was kept only while those seconds stay under this value.
	lgRouteAgeSettled = 4
	// The wait for lgRouteAgeSettled: 40 reads, half a second apart, is
	// twenty seconds for an age that needs four.
	lgRouteAgeAttempts = 40
	lgRouteAgeDelay    = 500 * time.Millisecond
)

// lgRouteAgeCell is the Age cell of a route row of the search page
// (internal/component/lg/route_table.templ): the exact seconds in
// data-seconds, and the age as a reader takes it as the cell's text.
var lgRouteAgeCell = regexp.MustCompile(`<td class="route-age" data-seconds="(\d+)">([^<]*)</td>`)

// lgRouteAge learns one route, waits for the API to answer a growing age for
// it, checks that the route table shows an age between two reads of the API,
// announces the route again with equal attributes and checks that the age
// kept growing, then announces it with another AS path and checks that the
// age started again.
func lgRouteAge(ctx context.Context, _ []string) error {
	return observe09(ctx, "lg-route-age", func(ctx context.Context, p *sdk.Plugin) error {
		if err := p12WaitPeersEOR(ctx, p, "peer1"); err != nil {
			return err
		}
		port := os.Getenv("ZE_LG_PORT")
		if port == "" {
			return errors.New("route age: ZE_LG_PORT not set in plugin environment")
		}
		base := "http://127.0.0.1:" + port

		if err := lgASPathSearchInject(ctx, p, lgRouteAgePrefix, lgRouteAgePath); err != nil {
			return err
		}
		settled, err := lgRouteAgeWaitSettled(ctx, base)
		if err != nil {
			return err
		}

		// AC-7: the page shows the age the API answers. The route does not
		// change between the three reads, so its age only grows: the page's
		// seconds lie between the API's answer before and after it.
		shown, err := lgRouteAgePage(ctx, base)
		if err != nil {
			return err
		}
		after, err := lgRouteAgeRead(ctx, base)
		if err != nil {
			return err
		}
		if shown < settled {
			return fmt.Errorf("route age: the route table shows %s as %d seconds old, younger than the %d the API answered before that read",
				lgRouteAgePrefix, shown, settled)
		}
		if shown > after {
			return fmt.Errorf("route age: the route table shows %s as %d seconds old, older than the %d the API answered after that read",
				lgRouteAgePrefix, shown, after)
		}

		// AC-8: an announcement that changes nothing leaves the age alone.
		if err := lgASPathSearchInject(ctx, p, lgRouteAgePrefix, lgRouteAgePath); err != nil {
			return err
		}
		kept, err := lgRouteAgeRead(ctx, base)
		if err != nil {
			return err
		}
		if kept < settled {
			return fmt.Errorf("route age: %s was %d seconds old, and %d after it was announced again with equal attributes: an equal announcement reset the age",
				lgRouteAgePrefix, settled, kept)
		}

		// AC-8: an announcement that changes an attribute starts the age again.
		changeStarted := time.Now()
		if err := lgASPathSearchInject(ctx, p, lgRouteAgePrefix, lgRouteAgePathChanged); err != nil {
			return err
		}
		restarted, err := lgRouteAgeRead(ctx, base)
		if err != nil {
			return err
		}
		// A restarted age is at most the whole seconds since the change began,
		// plus one for the clock's one-second resolution.
		ceiling := int(time.Since(changeStarted).Seconds()) + 1
		if ceiling >= kept {
			return fmt.Errorf("route age: the change and the read after it took %d seconds, as long as the age of %d the route had: this run cannot tell a restarted age from a kept one",
				ceiling, kept)
		}
		if restarted > ceiling {
			return fmt.Errorf("route age: %s was %d seconds old, and %d after its AS path changed, where a restarted age is at most %d: a changed route kept its age",
				lgRouteAgePrefix, kept, restarted, ceiling)
		}
		fmt.Fprintf(os.Stderr, "OK: the API age of %s grew to %d, was %d after an equal announcement and %d after a changed one\n",
			lgRouteAgePrefix, settled, kept, restarted)
		fmt.Fprintf(os.Stderr, "OK: the route table showed %s as %d seconds old between API ages %d and %d\n",
			lgRouteAgePrefix, shown, settled, after)
		return nil
	})
}

// lgRouteAgePage searches the looking glass page for lgRouteAgePrefix and
// answers the seconds its one row carries in the Age cell. A page that does
// not list the prefix once, a row whose Age cell holds no seconds and a cell
// that shows a reader nothing are each an error.
func lgRouteAgePage(ctx context.Context, base string) (int, error) {
	form := url.Values{columnPrefix: {lgRouteAgePrefix}}.Encode()
	body, err := lgCoveringRequest(ctx, http.MethodPost, base+"/lg/search", form)
	if err != nil {
		return 0, err
	}
	if listed := strings.Count(body, "<td>"+lgRouteAgePrefix+"</td>"); listed != 1 {
		return 0, fmt.Errorf("route age: the search page lists %s %d times, want 1: %.600q", lgRouteAgePrefix, listed, body)
	}
	cells := lgRouteAgeCell.FindAllStringSubmatch(body, -1)
	if len(cells) != 1 {
		return 0, fmt.Errorf("route age: the search page for %s holds %d Age cells with seconds, want 1: %.600q", lgRouteAgePrefix, len(cells), body)
	}
	seconds, err := strconv.Atoi(cells[0][1])
	if err != nil {
		return 0, fmt.Errorf("route age: the Age cell of %s holds seconds %q: %w", lgRouteAgePrefix, cells[0][1], err)
	}
	if strings.TrimSpace(cells[0][2]) == "" {
		return 0, fmt.Errorf("route age: the Age cell of %s holds %d seconds and shows a reader nothing", lgRouteAgePrefix, seconds)
	}
	return seconds, nil
}

// lgRouteAgeWaitSettled reads the route's age until it reaches
// lgRouteAgeSettled and answers the age it read. An age that never grows is
// an error naming the last value read.
func lgRouteAgeWaitSettled(ctx context.Context, base string) (int, error) {
	age := 0
	var readErr error
	if !Poll(ctx, lgRouteAgeAttempts, lgRouteAgeDelay, func() bool {
		age, readErr = lgRouteAgeRead(ctx, base)
		if readErr != nil {
			return true
		}
		return age >= lgRouteAgeSettled
	}) {
		return 0, fmt.Errorf("route age: the API answers age %d for %s after %d reads over %s, want at least %d: the route's age does not grow",
			age, lgRouteAgePrefix, lgRouteAgeAttempts, time.Duration(lgRouteAgeAttempts)*lgRouteAgeDelay, lgRouteAgeSettled)
	}
	if readErr != nil {
		return 0, readErr
	}
	return age, nil
}

// lgRouteAgeRead asks the birdwatcher prefix endpoint for lgRouteAgePrefix and
// answers the `age` of its one route, in seconds. A missing route, a second
// route, a missing `age` member and a negative age are each an error.
func lgRouteAgeRead(ctx context.Context, base string) (int, error) {
	target := base + "/api/looking-glass/routes/prefix?" + url.Values{columnPrefix: {lgRouteAgePrefix}}.Encode()
	body, err := lgCoveringRequest(ctx, http.MethodGet, target, "")
	if err != nil {
		return 0, err
	}
	var answer struct {
		Routes []struct {
			Network string   `json:"network"`
			Age     *float64 `json:"age"`
		} `json:"routes"`
	}
	if err := json.Unmarshal([]byte(body), &answer); err != nil {
		return 0, fmt.Errorf("route age: routes/prefix?prefix=%s is not JSON: %w: %.300q", lgRouteAgePrefix, err, body)
	}
	if len(answer.Routes) != 1 {
		return 0, fmt.Errorf("route age: routes/prefix?prefix=%s lists %d routes, want 1: %.300q", lgRouteAgePrefix, len(answer.Routes), body)
	}
	route := answer.Routes[0]
	if route.Network != lgRouteAgePrefix {
		return 0, fmt.Errorf("route age: routes/prefix?prefix=%s lists %s", lgRouteAgePrefix, route.Network)
	}
	if route.Age == nil {
		return 0, fmt.Errorf("route age: the route of %s carries no age member: %.300q", lgRouteAgePrefix, body)
	}
	if *route.Age < 0 {
		return 0, fmt.Errorf("route age: the route of %s carries a negative age %v", lgRouteAgePrefix, *route.Age)
	}
	return int(*route.Age), nil
}
