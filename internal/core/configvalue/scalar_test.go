// Detail: configvalue.go -- Bool, Int

package configvalue

import "testing"

// TestBoolReadsTheStringThePluginReceives checks the shape that made this
// function necessary. Tree.values is a map[string]string, so a plugin's config
// arrives with every leaf as text, and a .(bool) assertion on it never
// succeeds.
func TestBoolReadsTheStringThePluginReceives(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    any
		value bool
		ok    bool
	}{
		{"json delivery, true", "true", true, true},
		{"json delivery, false", "false", false, true},
		{"in process", true, true, true},
		{"in process, false", false, false, true},
		{"absent", nil, false, false},
		{"not a boolean", "yes please", false, false},
		{"a number", 1.0, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, ok := Bool(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && value != tc.value {
				t.Errorf("value = %v, want %v", value, tc.value)
			}
		})
	}
}

// TestIntReadsEveryDeliveryShape checks the three shapes an integer leaf
// reaches a reader in, and refuses the two that would answer a question the
// operator never asked: a fractional number and text that is not a number.
func TestIntReadsEveryDeliveryShape(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    any
		value int64
		ok    bool
	}{
		{"json delivery", "56", 56, true},
		{"in process, int", 56, 56, true},
		{"in process, int64", int64(56), 56, true},
		{"through encoding/json", 56.0, 56, true},
		{"negative", "-1", -1, true},
		{"zero is a value", "0", 0, true},
		{"absent", nil, 0, false},
		{"fractional", 56.5, 0, false},
		{"not a number", "fifty-six", 0, false},
		{"a boolean", true, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, ok := Int(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && value != tc.value {
				t.Errorf("value = %d, want %d", value, tc.value)
			}
		})
	}
}
