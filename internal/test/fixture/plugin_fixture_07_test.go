package fixture

import (
	"strings"
	"testing"
)

// metricsAnswer07 is the shape "show metrics values" answers with: one object
// carrying the whole Prometheus exposition as a string field.
func metricsAnswer07(exposition string) any {
	return map[string]any{"metrics": exposition}
}

const fibExposition07 = `# HELP ze_fibkernel_route_installs_total Routes successfully added to kernel.
# TYPE ze_fibkernel_route_installs_total counter
ze_fibkernel_route_installs_total 0
# HELP ze_fibkernel_errors_total Backend operation failures.
# TYPE ze_fibkernel_errors_total counter
ze_fibkernel_errors_total{operation="add"} 1
`

// TestPluginFixture07MetricsTextReadsTheField checks that the exposition is
// read out of the field rather than JSON-encoded with the answer around it.
// The encoded form escapes every quote, so a label selector never matches it,
// which is how the fib-table scenario reported "no route processing" while the
// counter it wanted stood at 1.
func TestPluginFixture07MetricsTextReadsTheField(t *testing.T) {
	answer := metricsAnswer07(fibExposition07)
	if got := metricsText07(answer); got != fibExposition07 {
		t.Fatalf("metricsText07 = %q, want the exposition unchanged", got)
	}
	if encoded := text07(answer); strings.Contains(encoded, `ze_fibkernel_errors_total{operation="add"} 1`) {
		t.Fatal("text07 kept the quotes unescaped, so this test no longer pins the defect")
	}
}

func TestPluginFixture07Counter(t *testing.T) {
	cases := []struct {
		selector string
		want     float64
		found    bool
	}{
		{selector: `ze_fibkernel_errors_total{operation="add"}`, want: 1, found: true},
		{selector: "ze_fibkernel_route_installs_total", want: 0, found: true},
		{selector: `ze_fibkernel_errors_total{operation="del"}`, want: 0, found: false},
	}
	for _, c := range cases {
		got, found := counter07(fibExposition07, c.selector)
		if found != c.found {
			t.Fatalf("counter07(%s) found = %t, want %t", c.selector, found, c.found)
		}
		if got != c.want {
			t.Fatalf("counter07(%s) = %v, want %v", c.selector, got, c.want)
		}
	}
}

// TestPluginFixture07FibTableProcessed checks both platform answers and the
// negative: fib-kernel that never saw the change counts neither.
func TestPluginFixture07FibTableProcessed(t *testing.T) {
	if !fibTableProcessed07(fibExposition07) {
		t.Fatal("an add error means fib-kernel processed the change, want true")
	}
	installed := "ze_fibkernel_route_installs_total 1\n"
	if !fibTableProcessed07(installed) {
		t.Fatal("an install means fib-kernel processed the change, want true")
	}
	silent := "ze_fibkernel_route_installs_total 0\nze_fibkernel_errors_total{operation=\"add\"} 0\n"
	if fibTableProcessed07(silent) {
		t.Fatal("both counters at zero means the change never reached fib-kernel, want false")
	}
}
