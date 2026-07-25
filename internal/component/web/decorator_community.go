// Design: docs/architecture/web-interface.md -- well-known BGP community name resolution
// Overview: render.go -- Template rendering resolves decorators at render time
// Related: decorator.go -- Decorator interface and registry

package web

import (
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// communityNameDecorator annotates a standard BGP community value in "ASN:value"
// form with its well-known name (RFC 1997 etc.), e.g. "65535:65281" -> "no-export".
// An ordinary (non-well-known) community renders as "ASN:value" already, so it
// gets no annotation.
type communityNameDecorator struct{}

// newCommunityNameDecorator creates a community-name decorator.
func newCommunityNameDecorator() *communityNameDecorator { return &communityNameDecorator{} }

func (d *communityNameDecorator) Name() string { return "community-name" }

// Decorate returns the well-known name for a standard community value in
// "ASN:value" form, or empty string when the community has no well-known name,
// is already a bare name, or does not parse -- graceful degradation.
func (d *communityNameDecorator) Decorate(value string) (string, error) {
	high, low, ok := strings.Cut(strings.TrimSpace(value), ":")
	if !ok {
		return "", nil // not an "ASN:value" community (e.g. already a bare name)
	}

	hi, err := strconv.ParseUint(high, 10, 16)
	if err != nil {
		return "", nil //nolint:nilerr // graceful degradation: non-numeric input is not an error
	}
	lo, err := strconv.ParseUint(low, 10, 16)
	if err != nil {
		return "", nil //nolint:nilerr // graceful degradation
	}

	c := attribute.Community(uint32(hi)<<16 | uint32(lo)) //nolint:gosec // G115: both halves bounded to 16 bits by ParseUint
	name := c.String()

	// A well-known community renders as a bare name; an ordinary one renders as
	// "ASN:value". Only the former is a useful annotation.
	if strings.Contains(name, ":") {
		return "", nil
	}
	return name, nil
}

// NewCommunityNameDecorator creates a community-name decorator. It needs no
// resolver -- the mapping is the in-process well-known community registry.
func NewCommunityNameDecorator() Decorator { return newCommunityNameDecorator() }
