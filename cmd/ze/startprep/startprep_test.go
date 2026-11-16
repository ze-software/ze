package startprep

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunRunsStepsInRegistrationOrder proves Run runs every registered step
// once, in the order Register saw them, and that the unregister function takes
// a step out of the next pass.
//
// Method: register two recording steps, run, unregister one, run again.
// MUTATION: a Run that ranges over a sorted or reversed copy reorders the
// first pass; an unregister that removes nothing leaves "first" in the second.
func TestRunRunsStepsInRegistrationOrder(t *testing.T) {
	var ran []string
	removeSecond := Register("test-second", func() { ran = append(ran, "second") })
	t.Cleanup(removeSecond)
	removeFirst := Register("test-first", func() { ran = append(ran, "first") })

	Run()
	assert.Equal(t, []string{"second", "first"}, ran)
	assert.Equal(t, []string{"test-second", "test-first"}, testNames())

	removeFirst()
	ran = nil
	Run()
	assert.Equal(t, []string{"second"}, ran)
}

// TestRegisterRefusesADefect proves the three registrations only a Ze defect
// makes panic, rather than leave a step that never runs or runs twice.
//
// MUTATION: dropping the duplicate guard lets the second Register return,
// and the assertion that it panics fails.
func TestRegisterRefusesADefect(t *testing.T) {
	remove := Register("test-duplicate", func() {})
	t.Cleanup(remove)

	require.Panics(t, func() { Register("test-duplicate", func() {}) })
	require.Panics(t, func() { Register("", func() {}) })
	require.Panics(t, func() { Register("test-nil", nil) })
	assert.Equal(t, []string{"test-duplicate"}, testNames())
}

// testNames answers the names this file's tests registered, so a step that
// another package's init() registered in the same test binary does not
// change them.
func testNames() []string {
	var names []string
	for _, name := range Names() {
		if strings.HasPrefix(name, "test-") {
			names = append(names, name)
		}
	}
	return names
}
