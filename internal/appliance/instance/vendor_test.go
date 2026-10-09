package instance

import (
	"slices"
	"strings"
	"testing"
)

// TestParseVendorModulesHeaderForms verifies every module header form that
// `go mod vendor` writes. A replaced module keeps its original import path and
// version, because its sources live under vendor/<original path>; a record that
// only documents a replacement provides no package and binds nothing.
//
// VALIDATES: the replace forms in a real modules.txt bind, and only the modules
// that provide a package bind.
func TestParseVendorModulesHeaderForms(t *testing.T) {
	cases := []struct {
		name, data string
		want       []vendorModule
	}{
		{
			name: "plain",
			data: "# example.com/a v1.2.3\n## explicit; go 1.26\nexample.com/a\n",
			want: []vendorModule{{path: "example.com/a", version: "v1.2.3", goVersion: "1.26"}},
		},
		{
			name: "module replacement",
			data: "# github.com/openconfig/goyang v1.6.3 => github.com/ze-software/goyang v1.6.4-0.20261009101552-a3cf525c4f98\n" +
				"## explicit; go 1.21\ngithub.com/openconfig/goyang/pkg/yang\n",
			want: []vendorModule{{path: "github.com/openconfig/goyang", version: "v1.6.3", goVersion: "1.21"}},
		},
		{
			name: "directory replacement",
			data: "# example.com/a v1.2.3 => ./third_party/a\n## explicit; go 1.22\nexample.com/a\n",
			want: []vendorModule{{path: "example.com/a", version: "v1.2.3", goVersion: "1.22"}},
		},
		{
			name: "replacement records bind nothing",
			data: "# example.com/a v1.2.3\n## explicit\nexample.com/a\n" +
				"# example.com/b => example.com/fork/b v1.0.1\n" +
				"# example.com/c => ../c\n" +
				"# example.com/d v0.1.0 => example.com/fork/d v0.1.1\n",
			want: []vendorModule{{path: "example.com/a", version: "v1.2.3"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseVendorModules(tc.data)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestParseVendorModulesRefusesUnknownHeaders verifies that a header the
// parser cannot read stops preparation instead of dropping a source binding.
//
// VALIDATES: malformed declarations are named in the error.
func TestParseVendorModulesRefusesUnknownHeaders(t *testing.T) {
	for _, header := range []string{
		"# example.com/a",
		"# example.com/a notaversion",
		"# example.com/a v1.2.3 extra",
		"# example.com/a v1.2.3 =>",
		"# example.com/a v1.2.3 => example.com/b",
		"# example.com/a v1.2.3 => example.com/b notaversion",
		"# example.com/a v1.2.3 => example.com/b v1.0.0 extra",
		"# example.com/a =>",
		"# example.com/a => ./a v1.0.0",
		"# example.com/a v1.2.3 -> example.com/b v1.0.0",
	} {
		t.Run(header, func(t *testing.T) {
			_, err := parseVendorModules(header + "\nexample.com/a\n")
			if err == nil {
				t.Fatalf("accepted %q", header)
			}
			if !strings.Contains(err.Error(), "bind vendor") {
				t.Fatalf("error %q does not name the binding", err)
			}
		})
	}
}
