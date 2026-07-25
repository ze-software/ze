// Design: ai/rules/pipe-completeness.md -- enrichAddr applies | resolve / | origin in | log render paths
// VALIDATES: enrichAddr adds PTR names (| resolve) and ASN data (| origin) to addresses,
// origin wins over resolve, and "*"/"" pass through unchanged.
// PREVENTS: | log render paths silently dropping data-transform pipes (pipe-completeness).

package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/command"
)

var errNoStubRecord = errors.New("no stub record")

type stubPTRResolver struct {
	names map[string][]string
}

func (s *stubPTRResolver) ResolvePTR(address string) ([]string, error) {
	if r, ok := s.names[address]; ok {
		return r, nil
	}
	return nil, errNoStubRecord
}

type stubOriginResolver struct {
	origins map[string]command.OriginResult
}

func (s *stubOriginResolver) LookupOrigin(_ context.Context, ip string) (command.OriginResult, error) {
	if o, ok := s.origins[ip]; ok {
		return o, nil
	}
	return command.OriginResult{}, errNoStubRecord
}

func setStubResolvers(t *testing.T) {
	t.Helper()
	command.SetPTRResolver(&stubPTRResolver{names: map[string][]string{
		"192.0.2.1": {"ping-target.test."},
	}})
	command.SetOriginResolver(&stubOriginResolver{origins: map[string]command.OriginResult{
		"192.0.2.1": {ASN: 64500, Prefix: "192.0.2.0/24", Name: "TEST-NET-AS"},
		"192.0.2.2": {ASN: 64501},
	}})
	t.Cleanup(func() {
		command.SetPTRResolver(nil)
		command.SetOriginResolver(nil)
	})
}

func TestEnrichAddr_NoFlags(t *testing.T) {
	setStubResolvers(t)
	assert.Equal(t, "192.0.2.1", enrichAddr("192.0.2.1", false, false))
}

func TestEnrichAddr_Resolve(t *testing.T) {
	setStubResolvers(t)
	assert.Equal(t, "192.0.2.1 ping-target.test", enrichAddr("192.0.2.1", true, false))
}

func TestEnrichAddr_ResolveNoRecord(t *testing.T) {
	setStubResolvers(t)
	assert.Equal(t, "198.51.100.9", enrichAddr("198.51.100.9", true, false))
}

func TestEnrichAddr_OriginWithName(t *testing.T) {
	setStubResolvers(t)
	assert.Equal(t, "192.0.2.1 TEST-NET-AS", enrichAddr("192.0.2.1", false, true))
}

func TestEnrichAddr_OriginASNOnly(t *testing.T) {
	setStubResolvers(t)
	assert.Equal(t, "192.0.2.2 AS64501", enrichAddr("192.0.2.2", false, true))
}

func TestEnrichAddr_OriginWinsOverResolve(t *testing.T) {
	setStubResolvers(t)
	assert.Equal(t, "192.0.2.1 TEST-NET-AS", enrichAddr("192.0.2.1", true, true))
}

func TestEnrichAddr_OriginMissFallsBackToResolve(t *testing.T) {
	setStubResolvers(t)
	command.SetOriginResolver(&stubOriginResolver{origins: map[string]command.OriginResult{}})
	assert.Equal(t, "192.0.2.1 ping-target.test", enrichAddr("192.0.2.1", true, true))
}

func TestEnrichAddr_StarAndEmptyUnchanged(t *testing.T) {
	setStubResolvers(t)
	assert.Equal(t, "*", enrichAddr("*", true, true))
	assert.Equal(t, "", enrichAddr("", true, true))
}
