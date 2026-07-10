// VALIDATES: the action-first subcommand dispatcher routes known targets to their
// handler (forwarding the remaining args and returning the handler's exit code),
// and returns the right codes for empty input, help, and unknown targets.
// PREVENTS: a dispatch regression — wrong exit code, args not forwarded, or an
// unknown target being routed to a handler instead of the usage path.

package subdispatch

import "testing"

func TestDispatchKnownTarget(t *testing.T) {
	d := New("install", "install things")
	var gotArgs []string
	called := 0
	d.Register("foo", func(a []string) int {
		called++
		gotArgs = a
		return 7
	}, SubMeta{Desc: "foo target"})

	if rc := d.Dispatch([]string{"foo", "a", "b"}); rc != 7 {
		t.Errorf("Dispatch(foo) = %d, want 7 (handler's code)", rc)
	}
	if called != 1 {
		t.Errorf("handler called %d times, want 1", called)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "a" || gotArgs[1] != "b" {
		t.Errorf("handler args = %v, want [a b]", gotArgs)
	}
}

func TestDispatchControlPaths(t *testing.T) {
	d := New("install", "install things")
	d.Register("foo", func([]string) int { return 0 }, SubMeta{Desc: "foo"})

	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"empty", nil, 1},
		{"help", []string{"help"}, 0},
		{"-h", []string{"-h"}, 0},
		{"--help", []string{"--help"}, 0},
		{"unknown", []string{"nope"}, 1},
	} {
		if rc := d.Dispatch(tc.args); rc != tc.want {
			t.Errorf("Dispatch(%s) = %d, want %d", tc.name, rc, tc.want)
		}
	}
}

func TestSubcommandsSorted(t *testing.T) {
	d := New("install", "install things")
	d.Register("zeta", func([]string) int { return 0 }, SubMeta{})
	d.Register("alpha", func([]string) int { return 0 }, SubMeta{})
	if got := d.Subcommands(); got != "alpha, zeta" {
		t.Errorf("Subcommands() = %q, want %q", got, "alpha, zeta")
	}
}
