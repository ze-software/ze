// Design: docs/contributing/error-inventory.md -- a writer's selected error result.
// Related: classify.go -- tuple bindings and result-specific return tracing.

package archerrorinventory

import "testing"

// TestErrorInventoryClassifiesSelectedCallResult scans the real writer graph,
// retaining the selected result index through assignments and nested helpers.
// Plain alternatives and a typed error in another result remain plain.
func TestErrorInventoryClassifiesSelectedCallResult(t *testing.T) {
	const path = "internal/le/sample/results.go"
	const source = `package sample

import (
	"errors"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

func refusal() error {
	return opererr.New("No selected test", "One test is required", "./le test qemu all")
}

func number(valid bool) (int, error) {
	if !valid { return 0, refusal() }
	return 1, nil
}

func request(valid bool) (*int, error) {
	value, err := number(valid)
	if err != nil { return nil, err }
	return &value, nil
}

func typedAssignment() {
	_, err := request(false)
	leaction.ReportError(err) // case:typed-assignment
}

func typedDeclaration() {
	var value, err = number(false)
	_ = value
	leaction.ReportError(err) // case:typed-declaration
}

func mixed(valid bool) (int, error) {
	if valid { return 0, refusal() }
	return 0, errors.New("plain alternative")
}

func plainAlternative() {
	_, err := mixed(false)
	leaction.ReportError(err) // case:plain-alternative
}

func twoErrors() (error, error) {
	return refusal(), errors.New("plain selected result")
}

func otherResult() {
	_, err := twoErrors()
	leaction.ReportError(err) // case:plain-other-result
}

func firstResult() {
	err, _ := twoErrors()
	leaction.ReportError(err) // case:typed-first-result
}
`
	rows := scanSources(t, map[string]string{path: source})
	for _, test := range []struct {
		marker string
		want   Status
	}{
		{"case:typed-assignment", StatusTyped},
		{"case:typed-declaration", StatusTyped},
		{"case:plain-alternative", StatusPlain},
		{"case:plain-other-result", StatusPlain},
		{"case:typed-first-result", StatusTyped},
	} {
		t.Run(test.marker, func(t *testing.T) {
			row := rowAt(t, rows, path, lineOf(t, source, test.marker))
			if row.Status != test.want {
				t.Errorf("selected result status %s, want %s", row.Status, test.want)
			}
		})
	}
	if len(rows) != 5 {
		t.Errorf("writer count %d, want all five positive and negative controls", len(rows))
	}
}
