// Design: docs/contributing/error-inventory.md -- explicit local operator-error types.
package archerrorinventory

import "testing"

// TestErrorInventoryRetainsDeclaredTypeOnAssignment proves a receiver method
// need not be resolved when its receiving variable already proves the error
// type, while a shadowing short declaration does not inherit that proof.
func TestErrorInventoryRetainsDeclaredTypeOnAssignment(t *testing.T) {
	const path = "internal/le/sample/assignment.go"
	const source = `package sample
import (
	"errors"
	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)
type service struct{}
func (*service) failure() *opererr.Error {
	return opererr.New("Request refused", "The service is busy", "./le help")
}
func typedAssignment(s *service) {
	var err *opererr.Error
	err = s.failure()
	leaction.ReportError(err) // case:declared-type-assigned
}
func shadowedDeclaration(s *service) {
	var err *opererr.Error
	err = s.failure()
	{
		err := errors.New("plain shadow")
		err = errors.New("plain assigned shadow")
		leaction.ReportError(err) // case:plain-shadow
	}
}
`
	rows := scanSources(t, map[string]string{path: source})
	for _, test := range []struct {
		marker string
		want   Status
	}{
		{"case:declared-type-assigned", StatusTyped},
		{"case:plain-shadow", StatusPlain},
	} {
		t.Run(test.marker, func(t *testing.T) {
			row := rowAt(t, rows, path, lineOf(t, source, test.marker))
			if row.Status != test.want {
				t.Fatalf("status = %s; want %s", row.Status, test.want)
			}
		})
	}
}
