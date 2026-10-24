// Fixture for RuleSwitchNilError (AC-1). A trailing `want:` comment names the
// rule a finding must carry on that line; a function with none answers none.
package fixture

import "errors"

var errUnknown = errors.New("unknown kind")

// defaultNil answers a nil error for a kind no case recognized.
func defaultNil(kind int) (string, error) {
	switch kind {
	case 1:
		return "one", nil
	default: // want: switch-unmatched-nil-error
		return "", nil
	}
}

// fallOffNil reaches its last return only for a kind no case recognized.
func fallOffNil(kind int) (string, error) {
	switch kind { // want: switch-unmatched-nil-error
	case 1:
		return "one", nil
	case 2:
		return "two", nil
	}
	return "", nil
}

// typeFallOff is the same shape over a type switch.
func typeFallOff(value any) (int, error) {
	switch typed := value.(type) { // want: switch-unmatched-nil-error
	case int:
		return typed, nil
	}
	return 0, nil
}

// refuses names the unknown kind: no finding.
func refuses(kind int) (string, error) {
	switch kind {
	case 1:
		return "one", nil
	default:
		return "", errUnknown
	}
}

// setsThenReturns has a case that does not leave, so the return after the
// switch serves the matched kind too: no finding.
func setsThenReturns(kind int) (string, error) {
	name := "zero"
	switch kind {
	case 1:
		name = "one"
	}
	return name, nil
}

type receiver struct{ failure error }

// recordsDefault records the unknown kind before it answers: no finding.
func (r *receiver) recordsDefault(kind int) error {
	switch kind {
	case 1:
		return nil
	default:
		r.failure = errUnknown
		return nil
	}
}
