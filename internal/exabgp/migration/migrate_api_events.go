// Design: docs/architecture/exabgp-bridge.md -- which events a neighbor grants a script
// Overview: migrate.go -- neighbor and process migration
//
// An ExaBGP neighbor's `api` block names the processes it feeds AND the message
// kinds it feeds them. The relation is therefore per NEIGHBOR and per PROCESS,
// and this file reads one api block's half of it.

package migration

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// apiFlagEvents are the four events ExaBGP grants with a bare word rather than
// inside a direction block. Each is its own event name.
//
// Source: src/exabgp/configuration/neighbor/api.py, ParseAPI.flatten, the
// `for command in ('neighbor-changes', 'negotiated', 'fsm', 'signal')` loop.
var apiFlagEvents = []string{"neighbor-changes", "negotiated", "fsm", "signal"}

// apiMessageEvents are the BGP messages a direction block grants, spelled as
// ExaBGP's own Message.CODE.SHORT names (src/exabgp/bgp/message/message.py,
// short_names). Ze spells its event kinds the same way, which is what lets the
// grant be compared against bridge.Event.APIKey with no second table.
//
// `parsed`, `packets` and `consolidate` are absent on purpose: they say how a
// message is RENDERED and name no message, so ExaBGP's dispatcher never looks a
// process up under one of them.
var apiMessageEvents = []string{
	"open", "update", "notification", "keepalive", "refresh", "operational",
}

// apiDirections are the two blocks a grant can sit in. The event name carries
// the direction, so `receive { update; }` grants `receive-update` alone.
var apiDirections = []string{"receive", "send"}

// apiBlockEvents answers the events one api block grants, in the names
// bridge.Event.APIKey answers.
//
// ExaBGP reads an ABSENT leaf as a refusal rather than as a default: flatten
// takes `data.get(action, False)`, so `api { processes [ p ]; }` with no
// direction block grants p no event at all, and its script is fed nothing. That
// is why the answer can legitimately be empty, and why an empty answer must not
// be read as "the block stated no selection".
func apiBlockEvents(api *config.Tree) []string {
	events := make([]string, 0, len(apiFlagEvents)+len(apiDirections)*len(apiMessageEvents))

	for _, flag := range apiFlagEvents {
		if apiGranted(api, flag) {
			events = append(events, flag)
		}
	}

	for _, direction := range apiDirections {
		block := api.GetContainer(direction)
		if block == nil {
			continue
		}
		for _, message := range apiMessageEvents {
			if !apiGranted(block, message) {
				continue
			}
			var tb textbuf.Buffer
			events = append(events, tb.Str(direction).Byte('-').Str(message).String())
		}
	}

	slices.Sort(events)
	return events
}

// apiGranted reports whether one bare word in an api block is a grant.
//
// ExaBGP writes the word alone, `neighbor-changes;`, and the ze parser records
// it as the string "true". A word written with an explicit `false` is a
// refusal, which ExaBGP's own boolean parser accepts.
func apiGranted(tree *config.Tree, name string) bool {
	value, ok := tree.Get(name)
	if !ok {
		return false
	}
	return exabgpBoolean(value)
}
