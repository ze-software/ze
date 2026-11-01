package registry

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCompileFilterDeclarations proves the Stage 1 shape check of a filter
// declaration: exactly one of name and name-pattern, no text-era "*", a
// pattern that compiles and matches the whole name, and raw read from Reads.
func TestCompileFilterDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filter rpc.FilterDecl
		want   string
	}{
		{"both", rpc.FilterDecl{Name: "a", NamePattern: "a"}, "both name"},
		{"neither", rpc.FilterDecl{}, "neither name nor name-pattern"},
		{"wildcard", rpc.FilterDecl{Name: "*"}, `"name-pattern": ".+"`},
		{"bad pattern", rpc.FilterDecl{NamePattern: "("}, "not an RE2 expression"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileFilterDeclarations("p", []rpc.FilterDecl{tc.filter})
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), `plugin "p"`) {
				t.Fatalf("want a refusal naming the plugin and %q, got %v", tc.want, err)
			}
		})
	}

	declared, err := CompileFilterDeclarations("p", []rpc.FilterDecl{
		{NamePattern: "[0-9]+", Reads: []string{rpc.FilterFieldRaw}},
		{Name: "65001", Reads: []string{"prefix"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !declared[0].Covers("65002") || declared[0].Covers("x65002") || declared[0].Covers("65002x") {
		t.Error("a name pattern must match the whole name and only it")
	}
	if !declared[0].ReadsRaw() || declared[1].ReadsRaw() {
		t.Error("raw is read from the declared reads")
	}
	d, ok := FindDeclaredFilter(declared, "65001")
	if !ok || d.Name != "65001" {
		t.Error("an exactly declared name wins over a pattern that also covers it")
	}
	if d, ok := FindDeclaredFilter(declared, "64512"); !ok || d.Pattern == nil {
		t.Error("a name only the pattern covers resolves to the pattern")
	}
	if _, ok := FindDeclaredFilter(declared, "as-set"); ok {
		t.Error("a name nothing covers must not resolve")
	}
}

// TestCheckFilterDeclarationsNeedsAJudge proves a build that links no
// subsystem running filters refuses a plugin declaring one, rather than
// accepting it unjudged.
func TestCheckFilterDeclarationsNeedsAJudge(t *testing.T) {
	filterDeclarations.mu.Lock()
	saved := filterDeclarations.check
	filterDeclarations.check = nil
	filterDeclarations.mu.Unlock()
	t.Cleanup(func() { SetFilterDeclarationCheck(saved) })

	if err := CheckFilterDeclarations("p", nil); err != nil {
		t.Fatalf("a plugin declaring no filter needs no judge: %v", err)
	}
	err := CheckFilterDeclarations("p", []rpc.FilterDecl{{Name: "f"}})
	if err == nil || !strings.Contains(err.Error(), "ze_bgp") {
		t.Fatalf("want a refusal naming the build tag, got %v", err)
	}
}
