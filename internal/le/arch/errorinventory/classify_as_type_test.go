// Design: docs/contributing/error-inventory.md -- generic operator-error extraction.
package archerrorinventory

import "testing"

// TestErrorInventoryClassifiesAsTypeResult preserves the explicit target-type
// proof of errors.AsType without typing its boolean or an unrelated function.
func TestErrorInventoryClassifiesAsTypeResult(t *testing.T) {
	const path = "internal/component/config/cli/as_type.go"
	const source = `package sample
import (
	stderrors "errors"
	"fmt"
	"os"

	operator "github.com/ze-software/ze/internal/core/opererr"
	fakeerrors "github.com/ze-software/ze/internal/le/sample/fakeerrors"
)
type plainError struct{}
func (*plainError) Error() string { return "plain" }
func AsType[T error](err error) (T, bool) {
	var target T
	return target, false
}
func typed(err error) {
	if refusal, ok := stderrors.AsType[*operator.Error](err); ok {
		fmt.Fprintln(os.Stderr, "error:", refusal) // case:typed-if-result
	}
}
func assigned(err error) {
	refusal, ok := stderrors.AsType[*operator.Error](err)
	if ok {
		fmt.Fprintln(os.Stderr, refusal.Error()) // case:typed-assigned-result
	}
}
func unrelated(err error) {
	if refusal, ok := stderrors.AsType[*plainError](err); ok {
		fmt.Fprintln(os.Stderr, refusal) // case:unrelated-target
	}
}
func secondResult(err error) {
	_, matched := stderrors.AsType[*operator.Error](err)
	fmt.Fprintln(os.Stderr, matched) // case:boolean-result
}
func importedFake(err error) {
	if refusal, ok := fakeerrors.AsType[*operator.Error](err); ok {
		fmt.Fprintln(os.Stderr, refusal) // case:imported-fake
	}
}
func localFake(err error) {
	if refusal, ok := AsType[*operator.Error](err); ok {
		fmt.Fprintln(os.Stderr, refusal) // case:local-fake
	}
}
func emptyFix() {
	fmt.Fprintln(os.Stderr, operator.New("Refused", "The value is invalid", "")) // case:direct-empty-fix
}
func extractedEmptyFix() {
	if refusal, ok := stderrors.AsType[*operator.Error](operator.New("Refused", "The value is invalid", "")); ok {
		fmt.Fprintln(os.Stderr, refusal) // case:extracted-empty-fix
	}
}
func extractedEmptyWrap() {
	if refusal, ok := stderrors.AsType[*operator.Error](operator.Wrap(nil, "Refused", "The value is invalid", "")); ok {
		fmt.Fprintln(os.Stderr, refusal) // case:extracted-empty-wrap
	}
}
`
	const fake = `package fakeerrors
func AsType[T error](err error) (T, bool) {
	var target T
	return target, false
}
`
	rows := scanSources(t, map[string]string{
		path: source,
		"internal/le/sample/fakeerrors/as_type.go": fake,
	})
	for _, test := range []struct {
		marker string
		want   Status
	}{
		{"case:typed-if-result", StatusTyped},
		{"case:typed-assigned-result", StatusTyped},
		{"case:unrelated-target", StatusPlain},
		{"case:boolean-result", StatusPlain},
		{"case:imported-fake", StatusPlain},
		{"case:local-fake", StatusPlain},
		{"case:direct-empty-fix", StatusPlain},
		{"case:extracted-empty-fix", StatusPlain},
		{"case:extracted-empty-wrap", StatusPlain},
	} {
		t.Run(test.marker, func(t *testing.T) {
			row := rowAt(t, rows, path, lineOf(t, source, test.marker))
			if row.Status != test.want {
				t.Errorf("selected result status %s, want %s", row.Status, test.want)
			}
		})
	}
	if len(rows) != 9 {
		t.Errorf("writer count %d, want all nine positive and negative controls", len(rows))
	}
}
