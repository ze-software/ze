package firewall

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDomainGroupLeafProducesAProvidedSetMatch proves a rule naming a domain
// group becomes a match against the set the firewall-domain plugin registers.
//
// VALIDATES: AC-1 -- a rule matching a group filters on the group's addresses.
// PREVENTS: the shape that makes the plain `source-address "@name"` route
// unusable here. validateMatch refuses a match against a set this table does
// not declare UNLESS ProvidedType is set, because the two owners meet only at
// ApplyAll. Without ProvidedType every domain-group rule would be refused at
// verify with "match references unknown set".
func TestDomainGroupLeafProducesAProvidedSetMatch(t *testing.T) {
	matches, err := parseFromBlock(map[string]any{"source-domain-group": "cdn"})
	require.NoError(t, err)
	require.Len(t, matches, 1)

	inSet, ok := matches[0].(MatchInSet)
	require.True(t, ok)

	wantV4, _ := DomainGroupSetNames("cdn")
	assert.Equal(t, wantV4, inSet.SetName)
	assert.Equal(t, SetFieldSourceAddr, inSet.MatchField)
	assert.Equal(t, SetTypeIPv4, inSet.ProvidedType,
		"without ProvidedType the rule is refused for naming a set this table does not declare")
}

// TestDomainGroupDestinationLeafMatchesTheDestinationField proves the two
// directions produce different match fields. A destination leaf that matched
// the source address would filter the wrong half of every packet.
func TestDomainGroupDestinationLeafMatchesTheDestinationField(t *testing.T) {
	matches, err := parseFromBlock(map[string]any{"destination-domain-group": "cdn"})
	require.NoError(t, err)
	require.Len(t, matches, 1)
	inSet, ok := matches[0].(MatchInSet)
	require.True(t, ok)
	assert.Equal(t, SetFieldDestAddr, inSet.MatchField)
}

// TestExpandProvidedTermV6TwinsADomainGroupTerm proves an operator writing one
// domain-group rule gets both families.
//
// The plugin declares both sets whenever a group holds any address, so the
// twin always has a set to name. A parser that twinned IRR and not domain
// groups would leave every domain-group rule IPv4-only, and the IPv6 traffic
// the operator meant to filter would pass.
func TestExpandProvidedTermV6TwinsADomainGroupTerm(t *testing.T) {
	matches, err := parseFromBlock(map[string]any{"source-domain-group": "cdn"})
	require.NoError(t, err)

	twin := expandProvidedTermV6(Term{Name: "permit-cdn", Matches: matches, Actions: []Action{Accept{}}})
	require.NotNil(t, twin, "a domain-group term owes an IPv6 twin")
	assert.Equal(t, "permit-cdn_v6", twin.Name)

	require.Len(t, twin.Matches, 1)
	inSet, ok := twin.Matches[0].(MatchInSet)
	require.True(t, ok)
	_, wantV6 := DomainGroupSetNames("cdn")
	assert.Equal(t, wantV6, inSet.SetName)
	assert.Equal(t, SetTypeIPv6, inSet.ProvidedType)
}

// TestExpandProvidedTermV6StillTwinsAnIRRTerm proves generalizing the twin
// expansion did not drop the owner it was written for.
func TestExpandProvidedTermV6StillTwinsAnIRRTerm(t *testing.T) {
	matches, err := parseFromBlock(map[string]any{"source-asn": "65001"})
	require.NoError(t, err)

	twin := expandProvidedTermV6(Term{Name: "permit-asn", Matches: matches, Actions: []Action{Accept{}}})
	require.NotNil(t, twin)
	require.Len(t, twin.Matches, 1)
	inSet, ok := twin.Matches[0].(MatchInSet)
	require.True(t, ok)
	assert.Equal(t, irrV6Prefix+"AS65001", inSet.SetName)
}

// TestExpandProvidedTermV6LeavesAHandWrittenSetAlone proves a set name an
// operator typed is not twinned.
//
// ProvidedType gates the expansion, not the name. An operator can write
// `source-address "@domain_v4_x"` by hand, and that names a set their own table
// must declare; twinning it would invent a reference to an IPv6 set nobody
// declares, which validateMatch would then have to accept on the term's own
// say-so.
func TestExpandProvidedTermV6LeavesAHandWrittenSetAlone(t *testing.T) {
	matches, err := parseFromBlock(map[string]any{"source-address": "@domain_v4_cdn"})
	require.NoError(t, err)
	require.Len(t, matches, 1)
	inSet, ok := matches[0].(MatchInSet)
	require.True(t, ok)
	assert.Equal(t, SetType(0), inSet.ProvidedType, "a typed name carries no ProvidedType")

	assert.Nil(t, expandProvidedTermV6(Term{Name: "hand", Matches: matches}),
		"a hand-written set name must not be twinned")
}

// TestV6TwinNameSeparatesNoTwinFromAnEmptyName proves the bool return is what
// tells a caller there is no twin. A caller reading the name alone could not
// tell "no twin" from "the twin is the empty string", and would emit a match
// against a set with no name.
func TestV6TwinNameSeparatesNoTwinFromAnEmptyName(t *testing.T) {
	name, ok := v6TwinName(domainV4Prefix + "cdn")
	assert.True(t, ok)
	assert.Equal(t, domainV6Prefix+"cdn", name)

	name, ok = v6TwinName(irrV4Prefix + "AS1")
	assert.True(t, ok)
	assert.Equal(t, irrV6Prefix+"AS1", name)

	_, ok = v6TwinName("ports")
	assert.False(t, ok, "a set with no provided prefix has no twin")

	_, ok = v6TwinName(domainV4Prefix)
	assert.False(t, ok, "the bare prefix names no group, so it has no twin")
}
