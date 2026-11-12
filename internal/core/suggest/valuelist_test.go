package suggest

import "testing"

// TestValueListRender pins the value lines a refusal prints. Goal: every help
// starts two columns after the longest name, a value with no help is its name
// alone with no trailing space, and the alternation keeps the given order.
// Method: render fixed lists and compare whole strings, so a change of indent,
// padding or separator is red.
func TestValueListRender(t *testing.T) {
	values := []Value{
		{Name: "enable", Help: "Advertise the capability"},
		{Name: "disable", Help: "Do not advertise it"},
		{Name: "require", Help: "Refuse a peer without it"},
		{Name: "refuse", Help: "Refuse a peer with it"},
	}
	const want = "  enable   Advertise the capability\n" +
		"  disable  Do not advertise it\n" +
		"  require  Refuse a peer without it\n" +
		"  refuse   Refuse a peer with it"
	if got := ValueLines(values, "  "); got != want {
		t.Fatalf("ValueLines:\n%s\nwant:\n%s", got, want)
	}

	// AC-4: a value with no help is two spaces and the name, and the others
	// stay aligned on the longest name, the bare one included.
	mixed := []Value{{Name: "hex", Help: "Hexadecimal"}, {Name: "base64"}, {Name: "b", Help: "Short"}}
	const wantMixed = "  hex     Hexadecimal\n" +
		"  base64\n" +
		"  b       Short"
	if got := ValueLines(mixed, "  "); got != wantMixed {
		t.Fatalf("ValueLines with a bare value:\n%q\nwant:\n%q", got, wantMixed)
	}

	// The indent is the caller's: a nested list writes four spaces.
	if got := ValueLines([]Value{{Name: "a", Help: "x"}}, "    "); got != "    a  x" {
		t.Fatalf("ValueLines with a four-space indent: %q", got)
	}

	if got := Alternation(values); got != "<enable|disable|require|refuse>" {
		t.Fatalf("Alternation: %q", got)
	}
	if got := Alternation([]Value{{Name: "monitor"}}); got != "<monitor>" {
		t.Fatalf("Alternation of one value: %q", got)
	}
	if got := Names(values); len(got) != 4 || got[0] != "enable" || got[3] != "refuse" {
		t.Fatalf("Names: %v", got)
	}
}
