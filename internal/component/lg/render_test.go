package lg

import (
	"testing"
)

// every test here used to reach its function through a cloned
// html/template and an ExecuteTemplate call. The function was reachable only
// as a lgFuncMap entry. templ calls plain Go functions, so the funcMap is gone
// and each function is called directly. The assertion count per test drops
// with the template plumbing. The case count rises. formatNumCommas(any) is
// replaced by formatNum(string) plus formatCount(int), because the view models
// are typed.

func TestStateClass(t *testing.T) {
	// VALIDATES: stateClass maps FSM states to CSS classes.
	// PREVENTS: an unknown state rendering as up or down.
	tests := []struct {
		state string
		want  string
	}{
		{"established", "state-up"},
		{"idle", "state-down"},
		{"active", "state-down"},
		{"connect", "state-down"},
		{"opensent", "state-down"},
		{"openconfirm", "state-down"},
		{"unknown", "state-unknown"},
		{"", "state-unknown"},
	}

	for _, tt := range tests {
		t.Run("state_"+tt.state, func(t *testing.T) {
			if got := stateClass(tt.state); got != tt.want {
				t.Errorf("stateClass(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestBMPStateClass(t *testing.T) {
	// VALIDATES: a BMP peer is up or down and nothing else.
	// PREVENTS: an unexpected BMP state rendering with no class at all.
	tests := []struct {
		state string
		want  string
	}{
		{"up", "state-up"},
		{"down", "state-down"},
		{"", "state-down"},
		{"established", "state-down"},
	}

	for _, tt := range tests {
		t.Run("state_"+tt.state, func(t *testing.T) {
			if got := bmpStateClass(tt.state); got != tt.want {
				t.Errorf("bmpStateClass(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestFormatASPath(t *testing.T) {
	// VALIDATES: formatASPath renders an AS path as space-separated ASNs.
	// PREVENTS: a one-entry path or an absent path rendering a stray separator.
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"pair", []string{"65001", "65002"}, "65001 65002"},
		{"single", []string{"65001"}, "65001"},
		{"none", nil, ""},
		{"empty", []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatASPath(tt.in); got != tt.want {
				t.Errorf("formatASPath(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatCommunities(t *testing.T) {
	// VALIDATES: formatCommunities renders a community list comma-separated.
	// PREVENTS: a trailing comma on a one-entry list.
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"pair", []string{"65000:100", "65000:200"}, "65000:100, 65000:200"},
		{"single", []string{"65000:100"}, "65000:100"},
		{"none", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatCommunities(tt.in); got != tt.want {
				t.Errorf("formatCommunities(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatNum(t *testing.T) {
	// VALIDATES: formatNum groups an engine-reported number with commas, and
	// distinguishes an absent count from a zero one.
	// PREVENTS: an omitted route count rendering as a zero Ze never sent
	// (handler_api.go, routeCountsAvailable).
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"absent", "", ""},
		{"zero", "0", "0"},
		{"one_below_group", "999", "999"},
		{"first_group", "1000", "1,000"},
		{"one_above_group", "1001", "1,001"},
		{"one_below_second_group", "999999", "999,999"},
		{"second_group", "1000000", "1,000,000"},
		{"float_input", "1234.9", "1,234"},
		{"negative", "-1234", "-1,234"},
		{"not_a_number", "n/a", "n/a"},
		{"four_byte_asn_count", "4200000000", "4,200,000,000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatNum(tt.in); got != tt.want {
				t.Errorf("formatNum(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatCount(t *testing.T) {
	// VALIDATES: formatCount groups a count the looking glass computed itself.
	// PREVENTS: a zero result count rendering as an empty cell. formatNum
	// reserves the empty cell for a count the engine cannot produce.
	tests := []struct {
		name string
		in   int
		want string
	}{
		{"zero", 0, "0"},
		{"one", 1, "1"},
		{"one_below_group", 999, "999"},
		{"first_group", 1000, "1,000"},
		{"one_above_group", 1001, "1,001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatCount(tt.in); got != tt.want {
				t.Errorf("formatCount(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatUptime(t *testing.T) {
	// VALIDATES: formatUptime shortens a Go duration for the peer table.
	// PREVENTS: a value the engine did not produce as a duration being lost.
	tests := []struct {
		in   string
		want string
	}{
		{"0s", "0s"},
		{"59s", "59s"},
		{"6m10.766415s", "6m 10s"},
		{"3h12m4s", "3h 12m 4s"},
		{"49h1m2s", "2d 1h 1m"},
		{"not-a-duration", "not-a-duration"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := formatUptime(tt.in); got != tt.want {
				t.Errorf("formatUptime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRouteDetailURL(t *testing.T) {
	// VALIDATES: the HTMX target that expands a route row carries both keys.
	// PREVENTS: a row expanding some other peer's copy of the same prefix.
	got := routeDetailURL(routeRow{Prefix: "203.0.113.0/24", PeerAddress: "2001:db8::1"})
	want := "/lg/route/detail?prefix=203.0.113.0/24&peer=2001:db8::1"

	if got != want {
		t.Errorf("routeDetailURL = %q, want %q", got, want)
	}
}
