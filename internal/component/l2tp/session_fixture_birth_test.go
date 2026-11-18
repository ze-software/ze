// Design: docs/architecture/l2tp/subscriber-session-model.md -- explicit identities in protocol fixtures

package l2tp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/subscriberid"
)

// fixedRouteFixtureBirth supplies an explicit packed input to isolated route
// unit tests. The ordinal names one logical fixture, not a wire coordinate.
// A test MUST use another ordinal for another logical admission; only events
// of the same admission may reuse it. Actual admission tests mint real births.
func fixedRouteFixtureBirth(t *testing.T, ordinal uint64) subscriberid.L2TPBirth {
	t.Helper()
	birth, err := subscriberid.ParseL2TPBirth(fmt.Sprintf("l2tp-%032x", ordinal))
	require.NoError(t, err)
	return birth
}

// newFixtureBirth creates an explicit original identity for a protocol fixture.
// The fixture MUST retain the value for every fact of that same admission.
func newFixtureBirth(t *testing.T) subscriberid.L2TPBirth {
	t.Helper()
	birth, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	return birth
}

// newFixtureL2TPSession constructs one new raw actor fixture. The caller MUST
// pass an unpublished row and publish only the returned value; this helper
// MUST run before the row enters its tunnel or its Driver receives an attempt.
// Every call mints a distinct logical identity, including label replacements.
func newFixtureL2TPSession(t *testing.T, session *L2TPSession) *L2TPSession {
	t.Helper()
	require.NotNil(t, session)
	require.True(t, session.birth.IsZero(), "a fixture cannot replace an existing admission's birth")
	require.Empty(t, session.subscriberID)
	require.Zero(t, session.pppAttempt)
	birth := newFixtureBirth(t)
	session.birth = birth
	session.subscriberID = birth.String()
	return session
}
