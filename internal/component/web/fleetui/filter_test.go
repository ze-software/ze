package fleetui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// filterNames applies a devices filter to the test estate and answers the
// names it keeps.
func filterNames(t *testing.T, text string) []string {
	t.Helper()

	views := testViews()
	filter, err := DeviceFilters().Parse(text, LabelKeys(views))
	require.NoError(t, err, text)

	return namesOf(filter.Apply(views))
}

// TestFilterParse parses one filter per form of the grammar and applies it to
// the test estate.
//
// VALIDATES: a bare word, field:value, field:a,b, field:!value,
// label:key=value, the label key shorthand, the numeric and duration
// comparisons, has:alarm, has:task and several terms together each keep
// exactly the devices they describe.
// PREVENTS: a filter that shows rows it does not describe, which would make
// every count that links to it wrong.
func TestFilterParse(t *testing.T) {
	cases := []struct {
		name   string
		filter string
		want   []string
	}{
		{"the empty filter keeps every device", "", []string{edgeLon01, edgeLon02, coreLon01, coreAms01, rsAms01, cpe0113, cpe0200}},
		{"a bare word matches a name or a label value, ignoring case", "LON", []string{edgeLon01, edgeLon02, coreLon01}},
		{"a bare word matches a label value alone", "route-server", []string{rsAms01}},
		{"field equals value", "state:offline", []string{coreAms01}},
		{"field equals value, ignoring case", "state:OFFLINE", []string{coreAms01}},
		{"field equals any of the values", "state:degraded,down", []string{coreLon01, cpe0113}},
		{"field does not equal the value", "version:!1.4.2", []string{coreLon01, coreAms01, rsAms01, cpe0200}},
		{"field equals none of the values", "state:!healthy,unknown", []string{coreLon01, coreAms01, rsAms01, cpe0113}},
		{"the device carries the label", "label:site=ams", []string{coreAms01, rsAms01}},
		{"a label key in use is a field", "site:lon", []string{edgeLon01, edgeLon02, coreLon01}},
		{"a label key in use takes a list", "role:core,cpe", []string{coreLon01, coreAms01, cpe0113, cpe0200}},
		{"a number above", "cpu:>70", []string{edgeLon01, coreLon01}},
		{"a number below leaves out a device with no sample", "cpu:<10", []string{rsAms01}},
		{"a number equal", "cpu:12", []string{edgeLon02}},
		{"a duration above", "uptime:>30d", []string{edgeLon01}},
		{"a duration since the device was seen", "last-seen:>1d", []string{coreAms01}},
		{"the configuration state", "config:rejected", []string{cpe0113}},
		{"the group", "group:edge", []string{edgeLon01, edgeLon02}},
		{"the model", "model:ze-1000", []string{edgeLon01, edgeLon02, coreLon01, coreAms01, rsAms01, cpe0113}},
		{"has an open alarm", "has:alarm", []string{coreLon01, coreAms01}},
		{"has an active task", "has:task", []string{edgeLon01, edgeLon02}},
		{"every term must hold", "site:lon state:!healthy", []string{coreLon01}},
		{"spaces between terms do not matter", "  site:lon   has:task ", []string{edgeLon01, edgeLon02}},
	}

	for index := range cases {
		tc := &cases[index]
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, filterNames(t, tc.filter))
		})
	}
}

// TestFilterRefusesMalformedTerm parses filters the grammar does not accept.
//
// VALIDATES: AC "a malformed filter names the term and the valid fields" --
// each refusal is an operator error wrapping ErrFilterRefused whose problem
// names the term and whose fix lists the fields.
// PREVENTS: a mistyped filter answered with an empty table, which reads as
// "no device matches".
func TestFilterRefusesMalformedTerm(t *testing.T) {
	cases := []struct {
		name   string
		filter string
		// names is the text the problem line must hold.
		names string
	}{
		{"an unknown field", "stat:offline", `"stat:offline"`},
		{"a field with no value", "state:", `"state:"`},
		{"a value with no field", ":offline", `":offline"`},
		{"a negation with no value", "state:!", `"state:!"`},
		{"an empty value in a list", "state:down,,offline", `"state:down,,offline"`},
		{"a comparison on a text field", "version:>1.4", `"version:>1.4"`},
		{"a number field given a word", "cpu:>high", `"cpu:>high"`},
		{"a number field given nothing", "cpu:>", `"cpu:>"`},
		{"a duration with no unit", "uptime:>30", `"uptime:>30"`},
		{"a label with no value", "label:site", `"label:site"`},
		{"a label with an empty key", "label:=lon", `"label:=lon"`},
		{"the bad term among good ones", "site:lon stat:offline has:alarm", `"stat:offline"`},
	}

	views := testViews()
	labelKeys := LabelKeys(views)

	for index := range cases {
		tc := &cases[index]
		t.Run(tc.name, func(t *testing.T) {
			_, err := DeviceFilters().Parse(tc.filter, labelKeys)

			refusal := requireRefusalOf(t, err, ErrFilterRefused)
			assert.Contains(t, refusal.Problem(), tc.names)
			assert.Contains(t, refusal.Fix(), "name, state, version, config, group, cpu, memory, uptime, last-seen, model")
			assert.Contains(t, refusal.Why(), "previous rows")
		})
	}
}

// TestFilterRefusesValueOutsideAClosedField gives the state field a value it
// does not take.
//
// VALIDATES: the refusal names the term and the value, and its fix lists the
// values the field takes.
// PREVENTS: state:helthy read as "no device is in that state".
func TestFilterRefusesValueOutsideAClosedField(t *testing.T) {
	_, err := DeviceFilters().Parse("state:helthy", nil)

	refusal := requireRefusalOf(t, err, ErrFilterRefused)
	assert.Contains(t, refusal.Problem(), `"state:helthy"`)
	assert.Contains(t, refusal.Problem(), `"helthy"`)
	assert.Contains(t, refusal.Fix(), "offline, diverged, down, degraded, healthy, unknown")
}

// TestFilterRefusesUnknownFlag asks for a flag the devices table does not
// have.
//
// VALIDATES: the refusal names the term and its fix lists the flags.
// PREVENTS: has:alarms read as a filter that matches nothing.
func TestFilterRefusesUnknownFlag(t *testing.T) {
	_, err := DeviceFilters().Parse("has:fire", nil)

	refusal := requireRefusalOf(t, err, ErrFilterRefused)
	assert.Contains(t, refusal.Problem(), `"has:fire"`)
	assert.Contains(t, refusal.Fix(), "alarm, task")
}

// TestFilterLabelShorthandNeedsAKeyInUse names a label key no device of the
// rows carries.
//
// VALIDATES: a key that is in use is a field, and the same term is refused
// as an unknown field when no row carries the key.
// PREVENTS: a mistyped label key read as a filter that matches nothing.
func TestFilterLabelShorthandNeedsAKeyInUse(t *testing.T) {
	_, err := DeviceFilters().Parse("site:lon", []string{"role", "site"})
	require.NoError(t, err)

	_, err = DeviceFilters().Parse("site:lon", []string{"role"})
	refusal := requireRefusalOf(t, err, ErrFilterRefused)
	assert.Contains(t, refusal.Problem(), "not a label key in use")
}

// TestFilterLengthBoundary parses a filter of exactly the longest length and
// one character more.
//
// VALIDATES: 512 characters are accepted and 513 are refused with a message
// that names the limit.
// PREVENTS: an unbounded filter text reaching the term parser.
func TestFilterLengthBoundary(t *testing.T) {
	longest := strings.Repeat("a", FilterLengthMax)

	_, err := DeviceFilters().Parse(longest, nil)
	require.NoError(t, err)

	_, err = DeviceFilters().Parse(longest+"a", nil)
	refusal := requireRefusalOf(t, err, ErrFilterRefused)
	assert.Contains(t, refusal.Problem(), "512")
}

// TestFilterLengthCountsCharacters parses a filter of the longest length in
// characters that are each more than one byte.
//
// VALIDATES: the limit counts characters, not bytes.
// PREVENTS: a filter in another script refused at half its allowed length.
func TestFilterLengthCountsCharacters(t *testing.T) {
	_, err := DeviceFilters().Parse(strings.Repeat("é", FilterLengthMax), nil)

	require.NoError(t, err)
}

// TestFilterTermCountBoundary parses a filter of exactly the most terms and
// one more.
//
// VALIDATES: FilterTermsMax terms are accepted and one more is refused.
// PREVENTS: a filter of hundreds of terms applied to every row.
func TestFilterTermCountBoundary(t *testing.T) {
	most := strings.TrimSpace(strings.Repeat("a ", FilterTermsMax))

	_, err := DeviceFilters().Parse(most, nil)
	require.NoError(t, err)

	_, err = DeviceFilters().Parse(most+" a", nil)
	requireRefusal(t, err, ErrFilterRefused)
}

// TestFilterTextIsNormalized reads back the text of a parsed filter.
//
// VALIDATES: the text an address carries holds the terms separated by one
// space.
// PREVENTS: two addresses for one filter.
func TestFilterTextIsNormalized(t *testing.T) {
	filter, err := DeviceFilters().Parse("  site:lon    has:task ", []string{"site"})
	require.NoError(t, err)

	assert.Equal(t, "site:lon has:task", filter.Text())
}

// TestFilterSchemaWithoutLabels parses label forms against a grammar whose
// rows carry no labels.
//
// VALIDATES: a table registers its own fields and shares the grammar, and
// the label forms are refused where rows have no labels.
// PREVENTS: the grammar being tied to the devices table.
func TestFilterSchemaWithoutLabels(t *testing.T) {
	type row struct {
		actor string
		count float64
	}

	schema := NewFilterSchema[row]("audit entries",
		func(entry *row, needle string) bool { return strings.Contains(entry.actor, needle) }, nil)
	schema.MustRegisterField(TextField("actor", nil, func(entry *row) string { return entry.actor }))
	schema.MustRegisterField(NumberField("devices", func(entry *row) (float64, bool) { return entry.count, true }))

	rows := []row{{actor: "thomas", count: 4}, {actor: "hub", count: 1}}

	filter, err := schema.Parse("actor:thomas devices:>2", nil)
	require.NoError(t, err)
	assert.Equal(t, []row{{actor: "thomas", count: 4}}, filter.Apply(rows))

	_, err = schema.Parse("label:site=lon", nil)
	refusal := requireRefusalOf(t, err, ErrFilterRefused)
	assert.Contains(t, refusal.Fix(), "actor, devices")
	assert.NotContains(t, refusal.Fix(), "label:key=value")

	_, err = schema.Parse("has:alarm", nil)
	requireRefusal(t, err, ErrFilterRefused)
}

// TestFilterFieldValuesFeedTheAssistedMenus asks the devices grammar for what
// an assisted menu lists.
//
// VALIDATES: the fields come in registration order, the state field answers
// its closed set of values, and a field that takes any value answers none.
// PREVENTS: a menu that lists values by hand beside the registry.
func TestFilterFieldValuesFeedTheAssistedMenus(t *testing.T) {
	assert.Equal(t, []string{
		"name", "state", "version", "config", "group", "cpu", "memory", "uptime", "last-seen", "model",
	}, DeviceFilters().Fields())
	assert.Equal(t, []string{"alarm", "task"}, DeviceFilters().Flags())

	values, closed := DeviceFilters().FieldValues("state")
	require.True(t, closed)
	assert.Equal(t, []string{"offline", "diverged", "down", "degraded", "healthy", "unknown"}, values)

	_, closed = DeviceFilters().FieldValues("version")
	assert.False(t, closed)

	_, closed = DeviceFilters().FieldValues("no-such-field")
	assert.False(t, closed)
}

// TestSetFilterTerm writes the choice of an assisted menu into a filter.
//
// VALIDATES: a filter with no term of the field gains one at its end, the
// first term of the field is replaced in place, a second one is dropped, and
// the other terms keep their order.
// PREVENTS: an assisted menu that stacks contradicting terms in the box.
func TestSetFilterTerm(t *testing.T) {
	cases := []struct {
		name   string
		filter string
		field  string
		value  string
		want   string
	}{
		{"into an empty filter", "", "state", "offline", "state:offline"},
		{"appended", "site:lon", "state", "offline", "site:lon state:offline"},
		{"replaced in place", "state:down site:lon", "state", "offline", "state:offline site:lon"},
		{"a second term of the field is dropped", "state:down lon state:healthy", "state", "offline", "state:offline lon"},
		{"a field whose name starts another is left alone", "statement:x", "state", "offline", "statement:x state:offline"},
		{"a list is a value", "site:lon", "state", "degraded,down", "site:lon state:degraded,down"},
	}

	for index := range cases {
		tc := &cases[index]
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SetFilterTerm(tc.filter, tc.field, tc.value))
		})
	}
}

// TestRemoveFilterTerm clears the choice of an assisted menu.
//
// VALIDATES: every term of the field leaves and the others stay in order.
// PREVENTS: a cleared menu that leaves its term in the box.
func TestRemoveFilterTerm(t *testing.T) {
	assert.Equal(t, "lon site:lon", RemoveFilterTerm("state:down lon state:offline site:lon", "state"))
	assert.Equal(t, "site:lon", RemoveFilterTerm("site:lon", "state"))
	assert.Empty(t, RemoveFilterTerm("state:down", "state"))
}
