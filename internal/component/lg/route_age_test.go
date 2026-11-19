// Related: handler_ui.go -- routeAgeFrom and routeRows, the readers under test
// Related: handler_api.go -- transformRoutes, which answers `age` and `age_available`
// Related: route_table.templ -- the Age cell

package lg

import (
	"encoding/json"
	"strings"
	"testing"
)

// routeAgeTestRows decodes rows written as bgp-rib writes them: JSON text, so
// a number reaches the reader as the decoder hands it over, not as a Go
// literal the test chose.
func routeAgeTestRows(t *testing.T, rows ...string) []any {
	t.Helper()
	routes := extractRoutes(parseJSON(`{"routes":[` + strings.Join(rows, ",") + `]}`))
	if len(routes) != len(rows) {
		t.Fatalf("the fixture decoded to %d rows, want %d", len(routes), len(rows))
	}
	return routes
}

// routeAgeTestTables renders the three places a route's age is shown.
func routeAgeTestTables(t *testing.T, row routeRow) map[string]string {
	t.Helper()
	rows := []routeRow{row}
	search, err := renderToString(routeResults(searchView{Routes: rows, Count: 1}))
	if err != nil {
		t.Fatalf("render the search table: %v", err)
	}
	peer, err := renderToString(peerRoutesContent(peerRoutesView{Address: "192.0.2.1", Routes: rows}))
	if err != nil {
		t.Fatalf("render the peer table: %v", err)
	}
	detail, err := renderToString(routeDetail(routeDetailView{Prefix: row.Prefix, Route: &rows[0]}))
	if err != nil {
		t.Fatalf("render the route detail: %v", err)
	}
	return map[string]string{"search table": search, "peer table": peer, "route detail": detail}
}

// TestRouteRowsReadTheAge feeds routeRows a received row with an `age` member
// and reads the row and the three renderings.
//
// VALIDATES: AC-7, page half. The seconds bgp-rib wrote reach routeRow.Age,
// the two route tables show them as an elapsed time with the exact seconds in
// data-seconds, and the route detail shows the same.
// PREVENTS: an age the engine answers and the page never shows, and a page
// that shows a rounded age with no exact value for a script.
func TestRouteRowsReadTheAge(t *testing.T) {
	rows := routeRows(routeAgeTestRows(t,
		`{"prefix":"10.50.1.0/24","next-hop":"192.0.2.1","peer-address":"192.0.2.1","age":192}`))
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if got := rows[0].Age; got != (routeAge{seconds: 192, known: true}) {
		t.Fatalf("Age = %+v, want 192 known seconds", got)
	}

	tables := routeAgeTestTables(t, rows[0])
	const cell = `<td class="route-age" data-seconds="192">3m 12s</td>`
	for _, where := range []string{"search table", "peer table"} {
		if !strings.Contains(tables[where], cell) {
			t.Errorf("%s does not hold %s: %s", where, cell, tables[where])
		}
		if !strings.Contains(tables[where], "<th>Age</th>") {
			t.Errorf("%s has no Age header: %s", where, tables[where])
		}
	}
	const entry = `<dt>Age</dt><dd class="route-age" data-seconds="192">3m 12s</dd>`
	if !strings.Contains(tables["route detail"], entry) {
		t.Errorf("route detail does not hold %s: %s", entry, tables["route detail"])
	}
}

// TestRouteRowsKeepAZeroAge feeds routeRows a row whose age is 0.
//
// VALIDATES: a route stored this second shows "0s" with data-seconds="0".
// PREVENTS: a reader that tells a known age from an unknown one by the value,
// which would empty the cell of the newest routes.
func TestRouteRowsKeepAZeroAge(t *testing.T) {
	rows := routeRows(routeAgeTestRows(t, `{"prefix":"10.50.1.0/24","age":0}`))
	if got := rows[0].Age; got != (routeAge{known: true}) {
		t.Fatalf("Age = %+v, want 0 known seconds", got)
	}
	const cell = `<td class="route-age" data-seconds="0">0s</td>`
	if search := routeAgeTestTables(t, rows[0])["search table"]; !strings.Contains(search, cell) {
		t.Errorf("search table does not hold %s: %s", cell, search)
	}
}

// TestRouteRowsLeaveAMissingAgeEmpty feeds routeRows rows with no reading: no
// `age` member, as bgp-rib writes a sent row and a best path row, and values
// that are not a whole number of seconds.
//
// VALIDATES: each answers the unknown age, the two tables render an empty Age
// cell with no data-seconds, and the route detail has no Age entry.
// PREVENTS: an absent age shown as 0, which reads as a route learned this
// second.
func TestRouteRowsLeaveAMissingAgeEmpty(t *testing.T) {
	tests := map[string]string{
		"no member":   `{"prefix":"10.50.1.0/24","next-hop":"192.0.2.1"}`,
		"a string":    `{"prefix":"10.50.1.0/24","age":"192"}`,
		"negative":    `{"prefix":"10.50.1.0/24","age":-1}`,
		"a fraction":  `{"prefix":"10.50.1.0/24","age":1.5}`,
		"null":        `{"prefix":"10.50.1.0/24","age":null}`,
		"beyond 2^53": `{"prefix":"10.50.1.0/24","age":1e300}`,
	}
	for name, row := range tests {
		t.Run(name, func(t *testing.T) {
			rows := routeRows(routeAgeTestRows(t, row))
			if got := rows[0].Age; got != (routeAge{}) {
				t.Fatalf("Age = %+v, want the unknown age", got)
			}
			tables := routeAgeTestTables(t, rows[0])
			for _, where := range []string{"search table", "peer table"} {
				if !strings.Contains(tables[where], `<td class="route-age"></td>`) {
					t.Errorf("%s does not hold an empty Age cell: %s", where, tables[where])
				}
			}
			for where, body := range tables {
				if strings.Contains(body, "data-seconds") {
					t.Errorf("%s states seconds for a route with no age: %s", where, body)
				}
			}
			if strings.Contains(tables["route detail"], "<dt>Age</dt>") {
				t.Errorf("route detail shows an Age entry for a route with no age: %s", tables["route detail"])
			}
		})
	}
}

// TestFormatElapsed calls the one function that words an elapsed time, on
// both sides of every unit boundary.
//
// VALIDATES: seconds below a minute, minutes and seconds below an hour, hours,
// minutes and seconds below a day, then days, hours and minutes.
// PREVENTS: an age of 60 seconds shown as "60s", or a day shown in hours.
func TestFormatElapsed(t *testing.T) {
	tests := []struct {
		seconds uint64
		want    string
	}{
		{0, "0s"},
		{1, "1s"},
		{59, "59s"},
		{60, "1m 0s"},
		{192, "3m 12s"},
		{3599, "59m 59s"},
		{3600, "1h 0m 0s"},
		{7503, "2h 5m 3s"},
		{86399, "23h 59m 59s"},
		{86400, "1d 0h 0m"},
		{356400, "4d 3h 0m"},
		{4294967295, "49710d 6h 28m"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatElapsed(tt.seconds); got != tt.want {
				t.Errorf("formatElapsed(%d) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

// TestTransformRoutesAgeAvailability feeds transformRoutes a received row with
// an age, one whose age is 0, and a row with none, then reads the JSON a
// client receives.
//
// VALIDATES: docs/architecture/api/birdwatcher-compat.md Section 6.2. `age` is
// present on every route; age_available is true only for a row that carried a
// readable age, a known 0 included, and false beside the placeholder 0 of a
// row that carried none.
// PREVENTS: a mandatory field answering a measured zero for an age Ze does not
// hold (plan/journal/constant-reported-as-measured-state.md, 2026-11-18).
func TestTransformRoutesAgeAvailability(t *testing.T) {
	ze := parseJSON(`{"routes":[
		{"prefix":"10.50.1.0/24","peer-address":"192.0.2.1","age":192},
		{"prefix":"10.50.2.0/24","peer-address":"192.0.2.1","age":0},
		{"prefix":"10.50.3.0/24","peer-address":"192.0.2.1"},
		{"prefix":"10.50.4.0/24","peer-address":"192.0.2.1","age":"192"}
	]}`)

	encoded, err := json.Marshal(transformRoutes(ze, ""))
	if err != nil {
		t.Fatalf("encode the answer: %v", err)
	}
	var answer struct {
		Routes []map[string]json.RawMessage `json:"routes"`
	}
	if err := json.Unmarshal(encoded, &answer); err != nil {
		t.Fatalf("decode the answer: %v", err)
	}

	want := []struct {
		network   string
		age       string
		available string
	}{
		{`"10.50.1.0/24"`, "192", "true"},
		{`"10.50.2.0/24"`, "0", "true"},
		{`"10.50.3.0/24"`, "0", "false"},
		{`"10.50.4.0/24"`, "0", "false"},
	}
	if len(answer.Routes) != len(want) {
		t.Fatalf("routes = %d, want %d: %s", len(answer.Routes), len(want), encoded)
	}
	for i, w := range want {
		route := answer.Routes[i]
		if got := string(route["network"]); got != w.network {
			t.Errorf("route %d network = %s, want %s", i, got, w.network)
		}
		if got := string(route["age"]); got != w.age {
			t.Errorf("%s age = %q, want %s", w.network, got, w.age)
		}
		if got := string(route["age_available"]); got != w.available {
			t.Errorf("%s age_available = %q, want %s", w.network, got, w.available)
		}
	}
}
