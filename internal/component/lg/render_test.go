package lg

import (
	"bytes"
	"testing"
)

func TestStateClassTemplateFunc(t *testing.T) {
	// VALIDATES: stateClass maps FSM states to CSS classes.
	tpl, err := parseLGTemplates()
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

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

	tmplStr := `{{stateClass .}}`
	for _, tt := range tests {
		t.Run("func_"+tt.state, func(t *testing.T) {
			tmpl, err := tpl.Clone()
			if err != nil {
				t.Fatal(err)
			}
			tmpl, err = tmpl.New("sc_test").Parse(tmplStr)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := tmpl.ExecuteTemplate(&buf, "sc_test", tt.state); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if buf.String() != tt.want {
				t.Errorf("stateClass(%q) = %q, want %q", tt.state, buf.String(), tt.want)
			}
		})
	}
}

func TestFormatASPathTemplateFunc(t *testing.T) {
	// VALIDATES: formatASPath renders AS path array as space-separated string.
	tpl, err := parseLGTemplates()
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	tests := []struct {
		name string
		in   any
		want string
	}{
		{"valid", []any{float64(65001), float64(65002)}, "65001 65002"},
		{"single", []any{float64(65001)}, "65001"},
		{"nil", nil, ""},
		{"non-array", "not-an-array", ""},
	}

	tmplStr := `{{formatASPath .}}`
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := tpl.Clone()
			if err != nil {
				t.Fatal(err)
			}
			tmpl, err = tmpl.New("ap_test").Parse(tmplStr)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := tmpl.ExecuteTemplate(&buf, "ap_test", tt.in); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if buf.String() != tt.want {
				t.Errorf("formatASPath(%v) = %q, want %q", tt.in, buf.String(), tt.want)
			}
		})
	}
}

func TestFormatCommunitiesTemplateFunc(t *testing.T) {
	// VALIDATES: formatCommunities renders community array as comma-separated string.
	tpl, err := parseLGTemplates()
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	tests := []struct {
		name string
		in   any
		want string
	}{
		{"valid", []any{"65000:100", "65000:200"}, "65000:100, 65000:200"},
		{"single", []any{"65000:100"}, "65000:100"},
		{"nil", nil, ""},
		{"non-array", 42, ""},
	}

	tmplStr := `{{formatCommunities .}}`
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := tpl.Clone()
			if err != nil {
				t.Fatal(err)
			}
			tmpl, err = tmpl.New("cm_test").Parse(tmplStr)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := tmpl.ExecuteTemplate(&buf, "cm_test", tt.in); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if buf.String() != tt.want {
				t.Errorf("formatCommunities(%v) = %q, want %q", tt.in, buf.String(), tt.want)
			}
		})
	}
}

func TestFormatNumCommas(t *testing.T) {
	// VALIDATES: formatNumCommas renders numbers with comma separators.
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"zero_float", float64(0), "0"},
		{"small", float64(42), "42"},
		{"thousands", float64(1234), "1,234"},
		{"millions", float64(1234567), "1,234,567"},
		{"string_num", "1234", "1,234"},
		{"nil", nil, ""},
		{"bool", true, "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatNumCommas(tt.in)
			if got != tt.want {
				t.Errorf("formatNumCommas(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
