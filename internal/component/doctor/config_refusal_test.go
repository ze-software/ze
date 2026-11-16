// Design: docs/features/ai-first.md — doctor command tests

package doctor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestConfigRefusalPartsStateTheFixOnce: the doctor finding of a refused
// config carries its fix in Fix and never repeats it in the message.
// Method: split an operator error, the same error held inside a joined error, and a plain
// error.
//
// VALIDATES: an operator error gives its text without the fix line and its
// own fix; a plain error gives its whole text and `ze config validate <name>`.
// PREVENTS: a doctor-config-parse finding with no fix, or one that prints the
// fix twice.
func TestConfigRefusalPartsStateTheFixOnce(t *testing.T) {
	held := opererr.New("plugin acme gave no declaration", "ze holds no schema of it", "ze show plugin conformance run acme")
	message, fix := configRefusalParts(held, "ze.conf")
	assert.Equal(t, "plugin acme gave no declaration\nze holds no schema of it", message)
	assert.Equal(t, "ze show plugin conformance run acme", fix)

	message, fix = configRefusalParts(errors.Join(held), "ze.conf")
	assert.Equal(t, "plugin acme gave no declaration\nze holds no schema of it", message)
	assert.Equal(t, "ze show plugin conformance run acme", fix)

	message, fix = configRefusalParts(errors.New("line 3: unknown top-level keyword: acme"), "ze.conf")
	assert.Equal(t, "line 3: unknown top-level keyword: acme", message)
	assert.Equal(t, "ze config validate ze.conf", fix)
}
