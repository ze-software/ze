// Design: docs/architecture/testing/ci-format.md -- decoded editor key expectations

package testing

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// Key absence must distinguish an absent key from a failed read; otherwise a
// permissions or corruption failure makes draft-cleanup assertions pass.
func TestKeyExpectationAbsentDoesNotHideReadFailure(t *testing.T) {
	expectation := Expectation{Type: "key", Values: map[string]string{"path": "file/active/router.conf.draft", "absent": ""}}
	for _, readErr := range []error{fs.ErrPermission, errors.New("CRC mismatch")} {
		state := &keyErrorState{readErr: readErr}
		if err := checkExpectation(expectation, state); !errors.Is(err, readErr) {
			t.Fatalf("absence hid read failure %v: %v", readErr, err)
		}
	}
	if err := checkExpectation(expectation, &keyErrorState{readErr: fs.ErrNotExist}); err != nil {
		t.Fatalf("missing key did not satisfy absence: %v", err)
	}
}

type keyErrorState struct {
	MockState
	readErr error
}

func (state *keyErrorState) ReadKey(string) ([]byte, error) { return nil, state.readErr }

// The expectation must read the model's current persistent value, including
// mutations after the model was constructed, rather than the loose fixture.
func TestKeyExpectationReadsCurrentStore(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	state := &headlessModel{store: store}
	const key = "file/active/router.conf"
	if err := store.WriteKey(key, []byte("old")); err != nil {
		t.Fatal(err)
	}
	testCase := &testCase{}
	if err := testCase.parseExpect("key:path=" + key + ":contains=new:not-contains=old"); err != nil {
		t.Fatal(err)
	}
	expectation := testCase.Expects[0]
	if err := checkExpectation(expectation, state); err == nil {
		t.Fatal("new value matched before it was written")
	}
	if err := store.WriteKey(key, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if err := checkExpectation(expectation, state); err != nil {
		t.Fatal(err)
	}
}
