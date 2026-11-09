// VALIDATES: `system dns resolv-conf-path` refuses a relative path and a path
//            that is not in clean form, names the path refused and the fix,
//            and keeps "" (write no file) and an absolute path valid.
// PREVENTS:  the old silent sanitising, where such a path committed, Ze wrote
//            no resolv.conf, and no line anywhere said why.

package config

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

func TestValidateResolvConfPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want []string // fragments the refusal must carry; nil means accepted
	}{
		{name: "empty disables the file", path: ""},
		{name: "absolute is accepted", path: "/etc/resolv.conf"},
		{name: "proc cwd form is accepted", path: "/proc/self/cwd/test.resolv.conf"},
		{name: "bare name is refused", path: "resolv.conf",
			want: []string{`"resolv.conf"`, "not an absolute path", "/etc/resolv.conf", `""`}},
		{name: "dot-dot relative is refused", path: "../etc/resolv.conf",
			want: []string{`"../etc/resolv.conf"`, "not an absolute path"}},
		{name: "dot-dot element is refused", path: "/var/../etc/resolv.conf",
			want: []string{"not in clean form", `"/etc/resolv.conf"`}},
		{name: "repeated slash is refused", path: "/etc//resolv.conf",
			want: []string{"not in clean form", `"/etc/resolv.conf"`}},
		{name: "trailing slash is refused", path: "/etc/",
			want: []string{"not in clean form", `"/etc"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateResolvConfPath(tc.path)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("path %q was refused: %v", tc.path, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("path %q was accepted, want a refusal", tc.path)
			}
			for _, fragment := range tc.want {
				if !strings.Contains(err.Error(), fragment) {
					t.Errorf("refusal %q does not carry %q", err, fragment)
				}
			}
		})
	}
}

func TestResolvConfPathValidatorIsRegistered(t *testing.T) {
	// The YANG leaf binds ze:validate "resolv-conf-path". A validator that is
	// not registered under that name never runs, and the leaf accepts any path.
	reg := yang.NewValidatorRegistry()
	RegisterValidators(reg)
	cv := reg.Get("resolv-conf-path")
	if cv == nil {
		t.Fatal("resolv-conf-path is not in the validator registry")
	}
	if cv.ValidateFn == nil {
		t.Fatal("resolv-conf-path has a nil ValidateFn, so it refuses nothing")
	}
}

// TestLoadConfigRefusesRelativeResolvConfPath drives the refusal through the
// door an operator uses. The tests above prove the function; this proves the
// leaf binds it and the section walk reaches it. Drop the ze:validate line
// from ze-system-conf.yang and this goes red while the others stay green.
func TestLoadConfigRefusesRelativeResolvConfPath(t *testing.T) {
	const refused = `system {
	dns {
		resolv-conf-path resolv.conf
	}
}
`
	_, err := LoadConfig(refused, "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted a relative resolv-conf-path")
	}
	for _, want := range []string{"resolv-conf-path", `"resolv.conf"`, "not an absolute path"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %s", want, err)
		}
	}

	for _, accepted := range []string{`""`, "/etc/resolv.conf"} {
		src := "system {\n\tdns {\n\t\tresolv-conf-path " + accepted + "\n\t}\n}\n"
		if _, err := LoadConfig(src, "test.conf", nil); err != nil {
			t.Errorf("LoadConfig refused resolv-conf-path %s: %v", accepted, err)
		}
	}
}
