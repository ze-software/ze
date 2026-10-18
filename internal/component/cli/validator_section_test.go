// VALIDATES: the editor commit adds every section failure but the bgp ones the
// editor's own YANG pass reports, and adds a bgp reference failure.
// PREVENTS: a leafref in the bgp section naming nothing passing `commit`.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestEditorReportsSectionFailure drives the filter sectionErrors applies:
// a bgp type failure is dropped, a bgp reference failure and any failure of
// another section are kept.
func TestEditorReportsSectionFailure(t *testing.T) {
	bgp := string(config.ConfigTypeBGP)
	assert.False(t, editorReportsSectionFailure(config.SectionValidationError{Section: bgp, Err: yang.ValidationError{Type: yang.ErrTypeType}}))
	assert.True(t, editorReportsSectionFailure(config.SectionValidationError{Section: bgp, Err: yang.ValidationError{Type: yang.ErrTypeReference}}))
	assert.True(t, editorReportsSectionFailure(config.SectionValidationError{Section: "traffic", Err: yang.ValidationError{Type: yang.ErrTypeType}}))
	assert.True(t, editorReportsSectionFailure(config.SectionValidationError{Section: "traffic", Err: yang.ValidationError{Type: yang.ErrTypeReference}}))
}
