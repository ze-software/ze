// Design: docs/architecture/api/commands.md -- cmdutil tests

package cmdutil

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/cmd/ze/cli"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

// VALIDATES: ExtractOutputFormat removes trailing format keyword.
// PREVENTS: format extraction breaking command dispatch.
func TestExtractOutputFormat(t *testing.T) {
	tests := []struct {
		name       string
		words      []string
		wantWords  []string
		wantFormat string
	}{
		{"json suffix", []string{"peer", "list", "json"}, []string{"peer", "list"}, "json"},
		{"yaml suffix", []string{"peer", "list", "yaml"}, []string{"peer", "list"}, "yaml"},
		{"table suffix", []string{"peer", "list", "table"}, []string{"peer", "list"}, "table"},
		{"no format", []string{"peer", "list"}, []string{"peer", "list"}, ""},
		{"format in middle", []string{"peer", "json", "list"}, []string{"peer", "json", "list"}, ""},
		{"only format", []string{"json"}, nil, "json"},
		{"empty", nil, nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			words, format := ExtractOutputFormat(tt.words)
			if format != tt.wantFormat {
				t.Errorf("format = %q, want %q", format, tt.wantFormat)
			}
			if len(words) != len(tt.wantWords) {
				t.Errorf("words = %v, want %v", words, tt.wantWords)
				return
			}
			for i, w := range words {
				if w != tt.wantWords[i] {
					t.Errorf("words[%d] = %q, want %q", i, w, tt.wantWords[i])
				}
			}
		})
	}
}

// VALIDATES: looksLikeSelector matches IPv4, IPv6, and glob patterns.
// PREVENTS: IPv6 peer selectors not being recognized.
func TestLooksLikeSelector(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"127.0.0.1", true},
		{"192.168.*.*", true},
		{"*", true},
		{"::1", true},
		{"2001:db8::1", true},
		{"fe80::1%eth0", true},
		{"peer-name", false},
		{"show", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := looksLikeSelector(tt.input)
			if got != tt.want {
				t.Errorf("looksLikeSelector(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// VALIDATES: DescribeCommand returns description or summarizes children.
// PREVENTS: garbled help output for group commands.
func TestDescribeCommand(t *testing.T) {
	// Leaf with description.
	leaf := &cli.Command{Description: "Show BGP peers"}
	if got := DescribeCommand(leaf); got != "Show BGP peers" {
		t.Errorf("leaf desc = %q, want %q", got, "Show BGP peers")
	}

	// Group with children.
	group := &cli.Command{
		Children: map[string]*cli.Command{
			"bgp":  {},
			"peer": {},
		},
	}
	got := DescribeCommand(group)
	want := "subcommands: bgp, peer"
	if got != want {
		t.Errorf("group desc = %q, want %q", got, want)
	}

	// Empty node.
	empty := &cli.Command{}
	if got := DescribeCommand(empty); got != "" {
		t.Errorf("empty desc = %q, want empty", got)
	}
}

// VALIDATES: SuggestFromTree returns suggestions for typos.
// PREVENTS: missing "did you mean?" hints for unknown commands.
func TestSuggestFromTree(t *testing.T) {
	tree := &cli.Command{
		Children: map[string]*cli.Command{
			"peer":    {Description: "Peer commands"},
			"summary": {Description: "Summary"},
		},
	}

	// Close match should suggest.
	got := SuggestFromTree("pear", tree)
	if got != "peer" {
		t.Errorf("SuggestFromTree(pear) = %q, want %q", got, "peer")
	}

	// Nil tree children.
	got = SuggestFromTree("anything", &cli.Command{})
	if got != "" {
		t.Errorf("SuggestFromTree(nil children) = %q, want empty", got)
	}
}

// VALIDATES: RegisterLocalCommand stores handler and RunCommand dispatches it.
// PREVENTS: local handler registration silently failing.
func TestRegisterLocalCommandAndDispatch(t *testing.T) {
	// Clean up after test.
	defer cmdregistry.ResetForTest()

	called := false
	err := RegisterLocalCommand("test cmd", func(_ []string) int {
		called = true
		return 42
	})
	if err != nil {
		t.Fatalf("RegisterLocalCommand returned error: %v", err)
	}

	if !cmdregistry.HasLocal("test cmd") {
		t.Fatal("handler not found in cmdregistry")
	}
	handler, _ := cmdregistry.LookupLocal([]string{"test", "cmd"})
	if handler == nil {
		t.Fatal("LookupLocal returned nil")
	}
	code := handler(nil)
	if !called {
		t.Error("handler was not called")
	}
	if code != 42 {
		t.Errorf("handler returned %d, want 42", code)
	}
}

// VALIDATES: RegisterLocalCommand rejects empty path.
// PREVENTS: empty key in localHandlers map causing silent misdispatch.
func TestRegisterLocalCommandEmptyPath(t *testing.T) {
	err := RegisterLocalCommand("", func(_ []string) int { return 0 })
	if err == nil {
		t.Error("expected error for empty path, got nil")
		cmdregistry.ResetForTest() // cleanup
	}
}

// VALIDATES: RegisterLocalCommand rejects nil handler.
// PREVENTS: nil function call panic at dispatch time.
func TestRegisterLocalCommandNilHandler(t *testing.T) {
	err := RegisterLocalCommand("test nil", nil)
	if err == nil {
		t.Error("expected error for nil handler, got nil")
		cmdregistry.ResetForTest() // cleanup
	}
}

// VALIDATES: RegisterLocalCommand overwrites existing entry.
// PREVENTS: stale handlers persisting after re-registration.
func TestRegisterLocalCommandOverwrite(t *testing.T) {
	defer cmdregistry.ResetForTest()

	first := false
	second := false

	if err := RegisterLocalCommand("overwrite", func(_ []string) int {
		first = true
		return 1
	}); err != nil {
		t.Fatal(err)
	}

	if err := RegisterLocalCommand("overwrite", func(_ []string) int {
		second = true
		return 2
	}); err != nil {
		t.Fatal(err)
	}

	handler, _ := cmdregistry.LookupLocal([]string{"overwrite"})
	if handler == nil {
		t.Fatal("LookupLocal returned nil after overwrite")
	}
	code := handler(nil)
	if first {
		t.Error("first handler was called after overwrite")
	}
	if !second {
		t.Error("second handler was not called")
	}
	if code != 2 {
		t.Errorf("handler returned %d, want 2", code)
	}
}

// VALIDATES: matchLocalHandler finds longest prefix and passes remaining args.
// PREVENTS: wrong prefix matching or lost arguments.
func TestMatchLocalHandler(t *testing.T) {
	defer cmdregistry.ResetForTest()

	// Register handlers for testing.
	short := func(_ []string) int { return 1 }
	long := func(_ []string) int { return 2 }
	if err := RegisterLocalCommand("show bgp", short); err != nil {
		t.Fatal(err)
	}
	if err := RegisterLocalCommand("show bgp decode", long); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		words    []string
		selector string
		wantCode int // -1 means no match (nil handler)
		wantArgs []string
	}{
		{"exact match", []string{"show", "bgp", "decode"}, "", 2, nil},
		{"prefix with remaining args", []string{"show", "bgp", "decode", "--update", "FF"}, "", 2, []string{"--update", "FF"}},
		{"shorter prefix", []string{"show", "bgp", "foo"}, "", 1, []string{"foo"}},
		{"longest wins over shorter", []string{"show", "bgp", "decode", "x"}, "", 2, []string{"x"}},
		{"with selector", []string{"show", "bgp", "decode"}, "1.2.3.4", 2, []string{"1.2.3.4"}},
		{"args and selector", []string{"show", "bgp", "decode", "x"}, "1.2.3.4", 2, []string{"x", "1.2.3.4"}},
		{"no match", []string{"unknown", "cmd"}, "", -1, nil},
		{"empty words", nil, "", -1, nil},
		{"single word no match", []string{"version"}, "", -1, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, args := matchLocalHandler(tt.words, tt.selector)
			if tt.wantCode == -1 {
				if handler != nil {
					t.Error("expected nil handler, got non-nil")
				}
				return
			}
			if handler == nil {
				t.Fatal("expected handler, got nil")
			}
			code := handler(nil)
			if code != tt.wantCode {
				t.Errorf("handler returned %d, want %d", code, tt.wantCode)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
				return
			}
			for i, a := range args {
				if a != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, a, tt.wantArgs[i])
				}
			}
		})
	}
}
