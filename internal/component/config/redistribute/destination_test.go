package redistribute

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterDestinationDeclaresTheRoot proves a declared destination is
// found with the root it declared, and that declaring it again with the same
// root changes nothing.
//
// VALIDATES: RegisterDestination stores the declaration LookupDestination
// returns, and a repeated identical declaration is accepted.
// PREVENTS: a destination whose root the config doors cannot read.
func TestRegisterDestinationDeclaresTheRoot(t *testing.T) {
	d := Destination{Name: "desttest-declared", ConfigRoot: "desttestroot"}
	require.NoError(t, RegisterDestination(d))
	require.NoError(t, RegisterDestination(d), "an identical declaration is a no-op")

	got, ok := LookupDestination("desttest-declared")
	require.True(t, ok)
	assert.Equal(t, "desttestroot", got.ConfigRoot)

	_, ok = LookupDestination("desttest-never-declared")
	assert.False(t, ok, "a name nobody declared is not found")
}

// TestDestinationNamesListsEveryDeclarationSorted proves the name list a
// refusal prints is the registry's, in a stable order.
//
// VALIDATES: DestinationNames answers every declared name, sorted, once.
// PREVENTS: a refusal whose list of valid destinations omits one or varies
// from run to run.
func TestDestinationNamesListsEveryDeclarationSorted(t *testing.T) {
	require.NoError(t, RegisterDestination(Destination{Name: "desttest-names-b"}))
	require.NoError(t, RegisterDestination(Destination{Name: "desttest-names-a"}))
	require.NoError(t, RegisterDestination(Destination{Name: "desttest-names-a"}))

	names := DestinationNames()
	assert.True(t, slices.IsSorted(names), "names come back sorted: %v", names)
	assert.Equal(t, 1, countOf(names, "desttest-names-a"), "a repeated declaration is listed once")
	assert.Equal(t, 1, countOf(names, "desttest-names-b"))
}

func countOf(names []string, name string) int {
	n := 0
	for _, candidate := range names {
		if candidate == name {
			n++
		}
	}
	return n
}

// TestRegisterDestinationRefusesAConflict proves a second declaration under
// the same name with another root is refused and leaves the first in place,
// and that a declaration with no name is refused.
//
// VALIDATES: ErrDestinationConflict for a changed root and for an empty name.
// PREVENTS: two packages disagreeing on the root one destination needs, with
// the later init() silently winning.
func TestRegisterDestinationRefusesAConflict(t *testing.T) {
	require.NoError(t, RegisterDestination(Destination{Name: "desttest-conflict", ConfigRoot: "first"}))

	err := RegisterDestination(Destination{Name: "desttest-conflict", ConfigRoot: "second"})
	require.ErrorIs(t, err, ErrDestinationConflict)

	got, ok := LookupDestination("desttest-conflict")
	require.True(t, ok)
	assert.Equal(t, "first", got.ConfigRoot, "the refused declaration does not replace the first")

	assert.ErrorIs(t, RegisterDestination(Destination{ConfigRoot: "x"}), ErrDestinationConflict)
}

// TestRegisterConsumerFactoryNeedsADeclaredDestination proves a factory is
// refused until its destination is declared, and accepted after.
//
// VALIDATES: RegisterConsumerFactory refuses a name RegisterDestination never
// declared.
// PREVENTS: a factory-built destination whose missing config block the config
// doors cannot refuse, because they read only the declaration.
func TestRegisterConsumerFactoryNeedsADeclaredDestination(t *testing.T) {
	factory := ConsumerFactory{
		Name: "desttest-factory",
		New:  func(RouteUpdater) RedistConsumer { return nil },
	}

	err := RegisterConsumerFactory(factory)
	require.ErrorIs(t, err, ErrConsumerFactoryConflict)
	assert.Contains(t, err.Error(), "no destination declared")
	_, ok := LookupConsumerFactory("desttest-factory")
	assert.False(t, ok, "the refused factory is not registered")

	require.NoError(t, RegisterDestination(Destination{Name: "desttest-factory", ConfigRoot: "desttestroot"}))
	require.NoError(t, RegisterConsumerFactory(factory))
	_, ok = LookupConsumerFactory("desttest-factory")
	assert.True(t, ok)
}
