// Design: docs/architecture/api/process-protocol.md -- event types and direction in config
// Related: events.go -- the registry a token is resolved against
// Related: ids.go -- Direction, the typed enum a token resolves to

package events

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// TokenWildcard names every event type, in both directions. It is the
// character the subscription grammar and the peer selector already use, and no
// registered type can spell it, so it never collides with a type name.
const TokenWildcard = "*"

// Direction suffixes. Each one is the separator plus the matching Direction
// constant above, written out because a config token is text an operator
// types.
const (
	suffixReceived = "-received"
	suffixSent     = "-sent"
)

// SplitTypeToken resolves one config token into an event type and the
// direction that token grants it in.
//
// Resolution is registry first. A token that names a registered type keeps its
// whole name and gets DirBoth, whatever it ends with, so "update-rpki" stays
// one type and a plugin that registers a type ending in "-sent" keeps its
// name. Only a token the registry does not know is split on a trailing
// "-received" or "-sent", and the name that remains must itself be registered.
//
// The last return is false when neither the whole token nor the name under a
// direction suffix is a registered type. The caller reports that: this
// function knows the registry, not the config leaf the token came from.
func SplitTypeToken(namespace, token string) (string, Direction, bool) {
	if IsValidEvent(namespace, token) {
		return token, DirBoth, true
	}
	if base, found := strings.CutSuffix(token, suffixReceived); found && IsValidEvent(namespace, base) {
		return base, DirReceived, true
	}
	if base, found := strings.CutSuffix(token, suffixSent); found && IsValidEvent(namespace, base) {
		return base, DirSent, true
	}
	return "", DirUnspecified, false
}

// DirectionToken returns the config token that names an event type in one
// direction. It is the inverse of SplitTypeToken, and it is what a completion
// list offers. A direction of both, or none, needs no suffix.
func DirectionToken(eventType string, dir Direction) string {
	var tb textbuf.Buffer
	switch dir {
	case DirReceived:
		return tb.Str(eventType).Str(suffixReceived).String()
	case DirSent:
		return tb.Str(eventType).Str(suffixSent).String()
	case DirBoth, DirUnspecified:
		return eventType
	}
	return eventType
}

// DirectionWordHint explains a token that is a bare direction word, and
// returns "" for every other token. A direction belongs to the type it applies
// to, so "sent" on its own names nothing. It was a receive token in its own
// right until the type carried the direction, which is why the word an
// operator typed then must say what to type now.
func DirectionWordHint(token string) string {
	var tb textbuf.Buffer
	switch token {
	case DirectionReceived, DirectionSent:
		return tb.Quoted(token).
			Str(" is a direction, not an event type: name the type and its direction, as in ").
			Quoted(DirectionToken("update", ParseDirection(token))).String()
	case DirectionBoth:
		return tb.Quoted(token).
			Str(" is a direction, not an event type: a type with no direction means both, as in ").
			Quoted("update").String()
	}
	return ""
}
