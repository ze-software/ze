// Design: docs/architecture/api/process-protocol.md -- the filter text protocol
// Related: plugin_fixture_filter_subject.go -- the same observer shape over the whole attribute set
// Related: register_filter_origin.go -- the registration

package fixture

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The policy this fixture implements: a route whose ORIGIN is INCOMPLETE is
// refused, and a route whose ORIGIN is IGP is let through. RFC 4271 Section
// 5.1.1 gives INCOMPLETE to a route learned by another means, which is the
// distinction an operator writes this policy to act on.
const (
	filterOriginRejected = "incomplete"
	filterOriginAccepted = "igp"

	filterOriginRejectedPrefix = "10.0.0.0/24"
	filterOriginAcceptedPrefix = "10.1.0.0/24"

	// filterOriginUnnamed is what the fixture records when the subject names
	// no origin at all. That is the defect state: the policy then has nothing
	// to judge, and every route takes the same verdict.
	filterOriginUnnamed = "<unnamed>"
)

// filterOriginVerdict is what the policy answered for one route, beside the
// ORIGIN it read to answer it.
type filterOriginVerdict struct {
	origin string
	action sdk.FilterAction
}

// filterOriginState collects one verdict for each prefix. The handler runs on
// the plugin's callback goroutine and the scenario reads it on its own, so the
// mutex is load-bearing.
type filterOriginState struct {
	mu       sync.Mutex
	verdicts map[string]filterOriginVerdict
}

// filterMatchOnOrigin registers one import filter that decides on the ORIGIN
// the subject names, then checks that each route got the verdict the policy
// wrote.
//
// The check runs in compiled code because the subject reaches the plugin over
// the RPC and never appears in the daemon's stderr. The plugin prints what it
// read, so a failure carries the text that produced it.
func filterMatchOnOrigin(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("filter-match-on-origin takes no arguments")
	}

	state := &filterOriginState{verdicts: make(map[string]filterOriginVerdict)}
	registration := sdk.Registration{Filters: []sdk.FilterDecl{{
		Name:      "reject-incomplete",
		Direction: sdk.FilterImport,
		OnError:   sdk.OnErrorReject,
	}}}

	setup := func(plugin *sdk.Plugin) error {
		plugin.OnFilterUpdate(func(input *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
			prefix, announced := filterOriginField(input.Update, "add")
			if !announced {
				prefix = filterOriginUnnamed
			}
			origin, named := filterOriginField(input.Update, "origin")
			if !named {
				origin = filterOriginUnnamed
			}

			action := sdk.FilterAccept
			if origin == filterOriginRejected {
				action = sdk.FilterReject
			}

			state.mu.Lock()
			state.verdicts[prefix] = filterOriginVerdict{origin: origin, action: action}
			state.mu.Unlock()

			filterOriginSaid(prefix, origin, action, input.Update)
			return &sdk.FilterUpdateOutput{Action: action}, nil
		})
		return nil
	}

	return observeConfigured(ctx, "origin-policy", registration, setup,
		func(ctx context.Context, plugin *sdk.Plugin) error {
			return filterOriginCheck(ctx, plugin, state)
		})
}

// filterOriginSaid prints one verdict with the subject it was taken on, so a
// failure carries the text the daemon really handed the policy.
func filterOriginSaid(prefix, origin string, action sdk.FilterAction, subject string) {
	var line textbuf.Buffer
	line.Str("origin-policy: prefix=").Str(prefix)
	line.Str(" origin=").Str(origin)
	line.Str(" action=").Str(action.String())
	line.Str(" subject=").Quoted(subject)
	line.Byte('\n')
	_ = line.StdErr() //nolint:errcheck // a failed write to stderr has nowhere left to report itself
}

// filterOriginField returns the value the subject gives one attribute name, and
// whether the subject names it at all.
//
// It compares whole tokens. "originator-id" starts with the letters of
// "origin" and is a different attribute, and one subject can carry both.
func filterOriginField(subject, name string) (string, bool) {
	fields := strings.Fields(subject)
	for i, field := range fields {
		if field != name || i+1 >= len(fields) {
			continue
		}
		return fields[i+1], true
	}
	return "", false
}

// filterOriginCheck waits for both routes to be judged, compares each verdict
// with the policy the operator wrote, and then reads the routes the daemon
// stored.
//
// The stored routes are the half that says the verdict REACHED the route. A
// plugin that answers reject over a subject the chain then ignores would pass
// every assertion above that one.
func filterOriginCheck(ctx context.Context, plugin *sdk.Plugin, state *filterOriginState) error {
	Poll(ctx, 100, 100*time.Millisecond, func() bool {
		state.mu.Lock()
		defer state.mu.Unlock()
		return len(state.verdicts) == 2
	})

	state.mu.Lock()
	verdicts := make(map[string]filterOriginVerdict, len(state.verdicts))
	for prefix, verdict := range state.verdicts {
		verdicts[prefix] = verdict
	}
	state.mu.Unlock()

	wanted := map[string]filterOriginVerdict{
		filterOriginRejectedPrefix: {origin: filterOriginRejected, action: sdk.FilterReject},
		filterOriginAcceptedPrefix: {origin: filterOriginAccepted, action: sdk.FilterAccept},
	}
	for prefix, want := range wanted {
		got, judged := verdicts[prefix]
		if !judged {
			return fmt.Errorf("the policy was never called for %s, it judged %v", prefix, verdicts)
		}
		if got.origin != want.origin {
			return fmt.Errorf("%s: the subject named origin %q, want %q", prefix, got.origin, want.origin)
		}
		if got.action != want.action {
			return fmt.Errorf("%s: the policy answered %s, want %s", prefix, got.action, want.action)
		}
	}

	if err := fixture10Quiesce(ctx, plugin); err != nil {
		return err
	}
	stored, err := filterOriginStored(ctx, plugin)
	if err != nil {
		return err
	}
	if len(stored) != 1 || !strings.Contains(stored[0], filterOriginAcceptedPrefix) {
		return fmt.Errorf("the daemon stored %v, want %s alone", stored, filterOriginAcceptedPrefix)
	}

	var ok textbuf.Buffer
	ok.Str("OK: ").Str(filterOriginRejectedPrefix)
	ok.Str(" was rejected on its origin and ").Str(filterOriginAcceptedPrefix)
	ok.Str(" was accepted\n")
	_ = ok.StdErr() //nolint:errcheck // a failed write to stderr has nowhere left to report itself
	return nil
}

// filterOriginStored returns the routes the adj-rib-in plugin holds, one key
// for each, across every peer.
func filterOriginStored(ctx context.Context, plugin *sdk.Plugin) ([]string, error) {
	data, err := fixture10Object(fixture10Call(ctx, plugin, "show bgp adj-rib-in"), "adj-rib-in")
	if err != nil {
		return nil, err
	}

	var keys []string
	for _, routes := range map09(data["adj-rib-in"]) {
		for _, route := range list09(routes) {
			keys = append(keys, fmt.Sprint(map09(route)["key"]))
		}
	}
	return keys, nil
}
