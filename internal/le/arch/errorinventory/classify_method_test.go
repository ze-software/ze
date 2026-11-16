// Design: docs/contributing/error-inventory.md -- concrete receiver error provenance.
package archerrorinventory

import "testing"

// TestErrorInventoryDoesNotChooseAmongMethodVariants keeps platform-dependent
// implementations unproved when their shared receiver name is ambiguous.
func TestErrorInventoryDoesNotChooseAmongMethodVariants(t *testing.T) {
	const path = "internal/le/sample/caller.go"
	const caller = `package sample
import leaction "github.com/ze-software/ze/internal/le/le/action"
type service struct{}
func call(s *service) {
	err := s.failure()
	leaction.ReportError(err) // case:ambiguous-method
}
`
	const typed = `//go:build linux
package sample
import "github.com/ze-software/ze/internal/core/opererr"
func (*service) failure() error {
	return opererr.New("Request refused", "The service is busy", "./le help")
}
`
	const plain = `//go:build darwin
package sample
import "errors"
func (*service) failure() error { return errors.New("plain platform failure") }
`
	rows := scanSources(t, map[string]string{
		path:                                   caller,
		"internal/le/sample/service_linux.go":  typed,
		"internal/le/sample/service_darwin.go": plain,
	})
	row := rowAt(t, rows, path, lineOf(t, caller, "case:ambiguous-method"))
	if row.Status != StatusPlain {
		t.Fatalf("ambiguous method status = %s; want %s", row.Status, StatusPlain)
	}
}

// TestErrorInventoryFollowsConcreteReceiverMethods proves only a receiver's
// declared concrete type selects a method; plain returns, dynamic interfaces
// and a shadow without a declared concrete type remain unproved.
func TestErrorInventoryFollowsConcreteReceiverMethods(t *testing.T) {
	const path = "internal/le/sample/methods.go"
	const source = `package sample
import (
	"errors"
	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)
type service struct{}
type dynamic interface { failure() error }
func (*service) failure() error {
	return opererr.New("Request refused", "The service is busy", "./le help")
}
func (*service) plain() error { return errors.New("plain method failure") }
func concrete(s *service) {
	err := s.failure()
	leaction.ReportError(err) // case:concrete-method
}
func plain(s *service) {
	err := s.plain()
	leaction.ReportError(err) // case:plain-method
}
func interfaceCall(s dynamic) {
	err := s.failure()
	leaction.ReportError(err) // case:dynamic-interface
}
func shadowed(s *service, unknown dynamic) {
	{
		s := unknown
		err := s.failure()
		leaction.ReportError(err) // case:shadowed-receiver
	}
}
func exitedShadow(s dynamic) {
	{
		var s *service
		_ = s
	}
	err := s.failure()
	leaction.ReportError(err) // case:exited-shadow
}
func unrelatedField(s dynamic) {
	var fields struct { s *service }
	_ = fields
	err := s.failure()
	leaction.ReportError(err) // case:unrelated-field
}
`
	rows := scanSources(t, map[string]string{path: source})
	for _, test := range []struct {
		marker string
		want   Status
	}{
		{"case:concrete-method", StatusTyped},
		{"case:plain-method", StatusPlain},
		{"case:dynamic-interface", StatusPlain},
		{"case:shadowed-receiver", StatusPlain},
		{"case:exited-shadow", StatusPlain},
		{"case:unrelated-field", StatusPlain},
	} {
		t.Run(test.marker, func(t *testing.T) {
			row := rowAt(t, rows, path, lineOf(t, source, test.marker))
			if row.Status != test.want {
				t.Fatalf("status = %s; want %s", row.Status, test.want)
			}
		})
	}
}
