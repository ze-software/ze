package cli

import (
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestYangErrorCodeIsRegistered drives yangErrorCode over every validation
// error type the YANG validator declares and looks each answer up.
//
// VALIDATES: the code `validate config` puts in a diagnostic is one
// `ze explain <code>` resolves.
// PREVENTS: a code spelled here and nowhere else, which an operator cannot look
// up and no other surface agrees with.
func TestYangErrorCodeIsRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	// The range is the enumeration's own extent: ErrTypeUnknown is the zero
	// value and ErrTypeCardinality the last member (validator.go), so an error
	// type added to that block is covered here without an edit.
	for errType := configyang.ErrTypeUnknown; errType <= configyang.ErrTypeCardinality; errType++ {
		code := yangErrorCode(errType)
		if code == "" {
			t.Errorf("error type %s answers no code", errType)
			continue
		}
		if diagnostic.Lookup(code) == nil {
			t.Errorf("error type %s answers %q, which the code registry does not hold", errType, code)
		}
	}
}
