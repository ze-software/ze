package web

import "testing"

// TestPendingChangeLabelCounts verifies the commit counter's text at each
// boundary the golden fixtures pin and at the one they cannot.
// VALIDATES: the singular, the plural and the empty form of the counter.
// PREVENTS: "1 pending changes", and a "0 pending changes" on a clean tree.
func TestPendingChangeLabelCounts(t *testing.T) {
	cases := []struct {
		changes int
		want    string
	}{
		{changes: -1, want: ""},
		{changes: 0, want: ""},
		{changes: 1, want: "1 pending change"},
		{changes: 2, want: "2 pending changes"},
	}

	for _, tc := range cases {
		if got := pendingChangeLabel(tc.changes); got != tc.want {
			t.Errorf("pendingChangeLabel(%d) = %q, want %q", tc.changes, got, tc.want)
		}
	}
}

// TestTristateDefaultClassLeansWithTheDefault verifies the unset boolean track
// carries the direction of the schema default.
// VALIDATES: a leaf with no configuration shows how the daemon behaves.
// PREVENTS: an unset toggle reading the same whichever way the default points.
func TestTristateDefaultClassLeansWithTheDefault(t *testing.T) {
	cases := []struct {
		def  string
		want string
	}{
		{def: "true", want: "ze-tristate-track ze-tristate-default ze-tristate-default-yes"},
		{def: "false", want: "ze-tristate-track ze-tristate-default ze-tristate-default-no"},
		{def: "", want: "ze-tristate-track ze-tristate-default"},
		{def: "yes", want: "ze-tristate-track ze-tristate-default"},
	}

	for _, tc := range cases {
		if got := tristateDefaultClass(tc.def); got != tc.want {
			t.Errorf("tristateDefaultClass(%q) = %q, want %q", tc.def, got, tc.want)
		}
	}
}

// TestFieldPlaceholderPrefersTheDefault verifies which text an unset editor
// shows, and that a configured leaf shows none.
// VALIDATES: default before description, and neither once a value is set.
// PREVENTS: a placeholder printed behind a value the operator already entered.
func TestFieldPlaceholderPrefersTheDefault(t *testing.T) {
	cases := []struct {
		name  string
		field FieldMeta
		want  string
	}{
		{name: "set", field: FieldMeta{Value: "180", Default: "90", Description: "hold timer"}, want: ""},
		{name: "default", field: FieldMeta{Default: "90", Description: "hold timer"}, want: "90"},
		{name: "description", field: FieldMeta{Description: "hold timer"}, want: "hold timer"},
		{name: "bare", field: FieldMeta{}, want: ""},
	}

	for _, tc := range cases {
		if got := fieldPlaceholder(tc.field); got != tc.want {
			t.Errorf("%s: fieldPlaceholder = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestFieldHxValsQuotesTheLeaf verifies the hx-vals payload is JSON HTMX can
// parse.
// VALIDATES: both payload shapes, with and without a value.
// PREVENTS: an unquoted leaf name breaking every editor POST.
func TestFieldHxValsQuotesTheLeaf(t *testing.T) {
	if got, want := fieldHxVals("hold-time"), `{"leaf":"hold-time"}`; got != want {
		t.Errorf("fieldHxVals = %q, want %q", got, want)
	}

	if got, want := fieldHxValsWith("enabled", "false"), `{"leaf":"enabled","value":"false"}`; got != want {
		t.Errorf("fieldHxValsWith = %q, want %q", got, want)
	}
}

// TestSplitFieldOptionsEmptyYieldsNone verifies an enum with no options renders
// the blank option alone.
// VALIDATES: the empty list is nil, not a one-element list holding "".
// PREVENTS: a stray blank <option> under the placeholder one.
func TestSplitFieldOptionsEmptyYieldsNone(t *testing.T) {
	if got := splitFieldOptions(""); got != nil {
		t.Errorf("splitFieldOptions(\"\") = %#v, want nil", got)
	}

	got := splitFieldOptions("igp,egp")
	if len(got) != 2 || got[0] != "igp" || got[1] != "egp" {
		t.Errorf("splitFieldOptions = %#v, want [igp egp]", got)
	}
}
