// Fixture: the ./le site, a refusal an action reports.
package tool

import (
	"errors"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// refuse is the fixture's one ./le site.
func refuse() {
	leaction.ReportError(errors.New("cache missing"))
}
