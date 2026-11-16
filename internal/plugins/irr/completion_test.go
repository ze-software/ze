package irr

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"

	// The firewall-irr leaves carry `ze:validate "irr-as-set"`; linking their
	// YANG puts them in the schema the editor completes over.
	_ "github.com/ze-software/ze/internal/component/firewall/plugins/irr/yang"
	_ "github.com/ze-software/ze/internal/component/firewall/yang"
)

// testPointer is a pointer value the owner could have written: a server, a
// generation and a lower-case hex SHA-256.
var testPointer = store.Pointer{Server: "whois.radb.net", Generation: 1, Digest: strings.Repeat("ab", 32)}

// seedASSetState writes what a running daemon holds after two consumers
// published their interest and the owner stored two datasets: AS-STORED is
// stored and referenced, AS-FETCHED is stored and referenced by nobody (an
// `update irr as-set` of an unreferenced set), AS-WANTED is referenced and not
// stored yet. AS65001 is a bare AS number, which is no AS-SET name.
func seedASSetState(t *testing.T, put func(key string, value []byte)) {
	t.Helper()
	interest := map[string]store.Interest{
		"bgp-filter-irr": {ASSets: []string{"as-stored", "AS-WANTED"}, ASNs: []uint32{65001}},
		"firewall-irr":   {ASSets: []string{"AS-STORED"}},
	}
	for consumer, in := range interest {
		value, err := store.EncodeInterest(in)
		require.NoError(t, err)
		put(zefs.KeyIRRInterest.Key(consumer), value)
	}
	pointer, err := store.EncodePointer(testPointer)
	require.NoError(t, err)
	put(store.PointerKey("AS-STORED"), pointer)
	put(store.PointerKey("AS-FETCHED"), pointer)
}

// wantKnown is what seedASSetState must complete to, sorted by name.
var wantKnown = []knownASSet{
	{name: "AS-FETCHED", stored: true},
	{name: "AS-STORED", stored: true},
	{name: "AS-WANTED", stored: false},
}

// TestKnownASSetsReadsInterestAndPointers proves the completion source reads
// both key families the owner's store holds, and only AS-SET names.
//
// VALIDATES: spec-fetched-data-survives-restart B-5, the source half: every
// AS-SET a consumer's interest names and every AS-SET a pointer names is
// offered once, upper-cased as store.ASSetName keys it, marked stored when a
// pointer exists, and a bare AS number is never offered as an AS-SET.
// Method: the fake state the owner's own tests use, seeded as a running
// daemon's store would be.
func TestKnownASSetsReadsInterestAndPointers(t *testing.T) {
	state := newFakeState()
	seedASSetState(t, func(key string, value []byte) { state.values[key] = value })

	known, err := knownASSets(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, wantKnown, known)
}

// TestKnownASSetsKeepsTheReadableBesideAMalformedInterest proves one bad
// interest value costs only its own names.
//
// VALIDATES: an interest value that does not decode is left out, its key is
// named in the error, and every other name is still answered.
// PREVENTS: one consumer's corrupt key emptying the dropdown for every leaf.
func TestKnownASSetsKeepsTheReadableBesideAMalformedInterest(t *testing.T) {
	state := newFakeState()
	seedASSetState(t, func(key string, value []byte) { state.values[key] = value })
	state.values[zefs.KeyIRRInterest.Key("broken")] = []byte("{not json")

	known, err := knownASSets(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), zefs.KeyIRRInterest.Key("broken"))
	assert.Equal(t, wantKnown, known)
}

// failingList is a store whose listing fails, as a store the daemon could not
// open does.
type failingList struct{ err error }

func (f failingList) StateList(context.Context, string) ([]string, error) { return nil, f.err }

func (failingList) StateGet(context.Context, string) ([]byte, bool, error) {
	return nil, false, errors.New("unreachable: nothing was listed")
}

// TestKnownASSetsRefusesAFailedListing proves a listing that fails answers an
// error and no names, never an empty list read as "nothing is stored".
func TestKnownASSetsRefusesAFailedListing(t *testing.T) {
	listErr := errors.New("store closed")
	known, err := knownASSets(context.Background(), failingList{err: listErr})
	require.ErrorIs(t, err, listErr)
	assert.Nil(t, known)
}

// useDaemonStore installs a private store as the daemon's, the store the
// editor and the dispatcher read in the daemon, and returns a writer for it.
func useDaemonStore(t *testing.T) func(key string, value []byte) {
	t.Helper()
	owned, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	statestore.SetStore(owned)
	t.Cleanup(func() {
		statestore.SetStore(nil)
		if err := owned.Close(); err != nil {
			t.Errorf("close the test store: %v", err)
		}
	})
	return func(key string, value []byte) {
		require.NoError(t, statestore.Write(key, value))
	}
}

// The firewall-irr leaves the editor completes in these tests.
const (
	interfaceASSetInput  = "set firewall irr interface eth1 source-as-set "
	termSourceASSetInput = "set firewall table T chain C term X from source-as-set "
	termTargetASSetInput = "set firewall table T chain C term X from destination-as-set "
)

// TestEditorOffersStoredASSets proves an operator typing an AS-SET leaf in the
// editor is offered the AS-SETs the daemon's store holds, with what each one
// is.
//
// VALIDATES: spec-fetched-data-survives-restart B-5, the config-leaf half,
// through Completer.Complete, the call every editor keystroke makes (SSH, web
// and `ze config edit`, which all run the editor in the daemon). Each of the
// three firewall-irr AS-SET leaves offers exactly the stored and referenced
// names, each saying whether a dataset is stored.
// PREVENTS: a ze:validate line dropped from one leaf, and a CompleteFn that
// reads the owner's memory, which a forked owner holds in another process.
func TestEditorOffersStoredASSets(t *testing.T) {
	seedASSetState(t, useDaemonStore(t))
	completer, err := cli.NewCompleter()
	require.NoError(t, err)

	for _, input := range []string{interfaceASSetInput, termSourceASSetInput, termTargetASSetInput} {
		t.Run(input, func(t *testing.T) {
			offered := make(map[string]string)
			for _, completion := range completer.Complete(nil, input, nil) {
				offered[completion.Text] = completion.ShortHelp
			}
			want := make(map[string]string, len(wantKnown))
			for _, set := range wantKnown {
				want[set.name] = set.help()
			}
			assert.Equal(t, want, offered)
		})
	}
}

// TestEditorASSetCompletionIsNeverAConstraint proves the suggestion decides
// nothing: an AS-SET the store has never seen is still a value the leaf
// takes, because writing it is how an operator asks for it.
func TestEditorASSetCompletionIsNeverAConstraint(t *testing.T) {
	seedASSetState(t, useDaemonStore(t))
	completer, err := cli.NewCompleter()
	require.NoError(t, err)

	path := []string{"firewall", "irr", "interface", "eth1", "source-as-set"}
	require.NoError(t, completer.ValidateValueAtPath(path, "AS-NEVER-SEEN"))
}

// TestEditorWithEmptyStoreOffersTheTypeHint proves a store holding no AS-SET
// offers no name: the editor prints the leaf's type hint, which says what to
// type, instead of a candidate list.
func TestEditorWithEmptyStoreOffersTheTypeHint(t *testing.T) {
	useDaemonStore(t)
	completer, err := cli.NewCompleter()
	require.NoError(t, err)

	completions := completer.Complete(nil, interfaceASSetInput, nil)
	require.NotEmpty(t, completions, "the leaf must still answer its type hint")
	for _, completion := range completions {
		assert.True(t, strings.HasPrefix(completion.Text, "<"),
			"an empty store offered %q, which is no type hint", completion.Text)
	}
}

// TestCommandArgumentOffersStoredASSets proves `update irr as-set` and `clear
// irr as-set` complete from the same names, read in the daemon.
//
// VALIDATES: spec-fetched-data-survives-restart B-5, the command half: the
// ArgCompleter both registrations declare answers every stored and referenced
// AS-SET with its help, reading the daemon's store and no plugin process.
func TestCommandArgumentOffersStoredASSets(t *testing.T) {
	seedASSetState(t, useDaemonStore(t))

	candidates := completeASSetArgument(nil, nil)
	got := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		got[candidate.Value] = candidate.ShortHelp
	}
	assert.Equal(t, map[string]string{
		"AS-FETCHED": asSetHelpStored,
		"AS-STORED":  asSetHelpStored,
		"AS-WANTED":  asSetHelpReferenced,
	}, got)
}

// TestCommandArgumentWithNoStoreOffersNothing proves a process with no store
// offers no AS-SET rather than failing the completion.
func TestCommandArgumentWithNoStoreOffersNothing(t *testing.T) {
	statestore.SetStore(nil)
	assert.Empty(t, completeASSetArgument(nil, nil))
}
