// Design: docs/contributing/gh-pages.md -- the looking glass replay: the tour
// Related: record.go -- record, which asks every step
// Related: recording.go -- TourStep, the step as the recording carries it

package sitelgreplay

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	chaosscenario "github.com/ze-software/ze/internal/chaos/scenario"
)

// Step is one question of the tour. Its text may name a subject of the
// network in braces, which subjects fills from the model: {peer}, {prefix},
// {address}, {announcer}, {origin} and {router}.
type Step struct {
	// Caption is what the player shows while the step plays.
	Caption string
	Method  string
	// Target is the path and query exactly as the looking glass markup writes
	// it, so the player finds the control that asks it.
	Target string
	// Form is the body of a POST.
	Form []Field
	// HTMX says the page asks through htmx and gets a fragment.
	HTMX bool
}

const (
	methodGet  = "GET"
	methodPost = "POST"
	// targetSearch is the search page, which the tour opens once and then
	// asks one question after another.
	targetSearch = "/lg/search"
	// traceRouters is how many routers of the AS the trace the tour shows
	// crosses at least: a start, a router in between, and the exit.
	traceRouters = 3
)

// tour is the declared questions, in order. Another question is one more
// entry: the recorder asks it, the recording carries its answer, and the
// player plays it, with no other edit.
//
// The first step is the document a replay opens on. A step the page asks
// through htmx is played by pressing the control that asks it, so each one
// follows the step that puts that control on the page.
var tour = []Step{
	{
		Caption: "The peers page: every simulated router and external neighbor, each holding a real BGP session to ze, with its uptime, its last state change and how often the session dropped.",
		Method:  methodGet, Target: "/lg/peers",
	},
	{
		Caption: "One peer's routes: what the external neighbor {peer} announced, and how long ago each route last changed.",
		Method:  methodGet, Target: "/lg/peer/{peer}", HTMX: true,
	},
	{
		Caption: "The search page.",
		Method:  methodGet, Target: targetSearch, HTMX: true,
	},
	{
		Caption: "Search by prefix: every speaker that announced {prefix}, with each route's next hop, AS path and age.",
		Method:  methodPost, Target: targetSearch, HTMX: true,
		Form: []Field{{Name: "prefix", Value: "{prefix}"}},
	},
	{
		Caption: "The AS path graph of {prefix}: the ASes its routes cross.",
		Method:  methodGet, Target: "/lg/graph?prefix={prefix}&mode=aspath", HTMX: true,
	},
	{
		Caption: "The next-hop graph of {prefix}: which router hands the traffic to which. Routers of the AS show by address, the neighbors outside by AS number only.",
		Method:  methodGet, Target: "/lg/graph?prefix={prefix}&mode=nexthop", HTMX: true,
	},
	{
		Caption: "The trace of {prefix}: each router's BGP next hop followed router to router to the exit, then the AS it is handed to and the AS path to the origin. From {router} it crosses three routers of the AS.",
		Method:  methodGet, Target: "/lg/graph?prefix={prefix}&mode=trace", HTMX: true,
	},
	{
		Caption: "The trace as text, the form a test reads.",
		Method:  methodGet, Target: "/lg/graph?prefix={prefix}&mode=trace&format=text",
	},
	{
		Caption: "The AS path graph as text.",
		Method:  methodGet, Target: "/lg/graph?prefix={prefix}&mode=aspath&format=text",
	},
	{
		Caption: "The next-hop graph as text.",
		Method:  methodGet, Target: "/lg/graph?prefix={prefix}&mode=nexthop&format=text",
	},
	{
		Caption: "Route detail: the route {announcer} sent for {prefix}, with every attribute and its age.",
		Method:  methodGet, Target: "/lg/route/detail?prefix={prefix}&peer={announcer}", HTMX: true,
	},
	{
		Caption: "Search by address: the routes that cover {address}.",
		Method:  methodPost, Target: targetSearch, HTMX: true,
		Form: []Field{{Name: "prefix", Value: "{address}"}},
	},
	{
		Caption: "Search by AS path with the end anchor: {origin}$ lists the routes AS {origin} originates.",
		Method:  methodPost, Target: targetSearch, HTMX: true,
		Form: []Field{{Name: "aspath", Value: "{origin}$"}},
	},
}

// subjects answers the values the tour's braces stand for, read from the
// model of the running network. The prefix is the first one whose trace from
// some router crosses traceRouters routers of the AS and that an external
// neighbor announces, so the trace the tour shows passes through the core
// and the peer the tour opens holds the route.
func subjects(topology *chaosscenario.Topology) (*strings.Replacer, error) {
	prefixes := topology.Prefixes()
	if len(prefixes) == 0 {
		return nil, errors.New("the simulated network announces no prefix, so the tour has nothing to ask about")
	}
	for _, prefix := range prefixes {
		router, crossed := tracedRouter(topology, prefix)
		if !crossed {
			continue
		}
		announcer, origin, hasAnnouncer := externalAnnouncer(topology, prefix)
		if !hasAnnouncer {
			continue
		}
		return strings.NewReplacer(
			"{peer}", announcer.String(),
			"{announcer}", announcer.String(),
			"{prefix}", prefix.String(),
			"{address}", prefix.Addr().Next().String(),
			"{origin}", strconv.FormatUint(uint64(origin), 10),
			"{router}", router.String(),
		), nil
	}
	return nil, fmt.Errorf("no prefix of the simulated network is announced by an external neighbor and traced across %d routers of the AS, so the tour has no trace to show", traceRouters)
}

// tracedRouter answers the first router of the AS whose trace of prefix
// crosses at least traceRouters routers.
func tracedRouter(topology *chaosscenario.Topology, prefix netip.Prefix) (netip.Addr, bool) {
	for _, speaker := range topology.Speakers() {
		if speaker.AS != topology.LocalAS() {
			continue
		}
		trace, err := topology.Trace(speaker.Address, prefix)
		if err != nil {
			// The model traces only a router that holds a route for the
			// prefix. This one does not, so it is not the tour's start.
			continue
		}
		if len(trace.Routers) >= traceRouters {
			return speaker.Address, true
		}
	}
	return netip.Addr{}, false
}

// externalAnnouncer answers the first external neighbor announcing prefix and
// the AS that originates the route it announces.
func externalAnnouncer(topology *chaosscenario.Topology, prefix netip.Prefix) (netip.Addr, uint32, bool) {
	for _, speaker := range topology.Speakers() {
		if speaker.AS == topology.LocalAS() {
			continue
		}
		for _, announcement := range speaker.Announcements {
			if announcement.Prefix != prefix {
				continue
			}
			if len(announcement.ASPath) == 0 {
				continue
			}
			return speaker.Address, announcement.ASPath[len(announcement.ASPath)-1], true
		}
	}
	return netip.Addr{}, 0, false
}

// resolve answers the tour with every subject filled in.
func resolve(steps []Step, values *strings.Replacer) []Step {
	resolved := make([]Step, 0, len(steps))
	for _, step := range steps {
		filled := Step{
			Caption: values.Replace(step.Caption), Method: step.Method,
			Target: values.Replace(step.Target), HTMX: step.HTMX,
		}
		for _, field := range step.Form {
			filled.Form = append(filled.Form, Field{Name: field.Name, Value: values.Replace(field.Value)})
		}
		resolved = append(resolved, filled)
	}
	return resolved
}
