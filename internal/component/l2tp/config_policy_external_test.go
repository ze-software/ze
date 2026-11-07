package l2tp_test

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp"
)

// TestConfigPPPPolicy validates the actual original-tree registration and YANG
// constraints before defaults or extraction can conceal an invalid policy.
func TestConfigPPPPolicy(t *testing.T) {
	for _, body := range []string{
		"mru 63;", "mru 1501;",
		"authentication { allow [ pap ]; }",
		"authentication { allow [ none ]; }",
		"auth-method none; allow-no-auth true; authentication { allow [ chap-md5 ]; }",
	} {
		if _, err := config.LoadConfig("l2tp { "+body+" }", "policy.conf", nil); err == nil {
			t.Errorf("accepted invalid policy %s", body)
		}
	}
	for _, body := range []string{
		"mru 68; ncp { enable-ipv6cp false; }", "mru 1492;", "mru 1500;",
		"auth-method pap; authentication { allow [ pap chap-md5 ]; }",
		"auth-method none; allow-no-auth true;",
	} {
		tree, err := config.LoadConfig("l2tp { "+body+" }", "policy.conf", nil)
		if err != nil {
			t.Fatalf("valid policy %s: %v", body, err)
		}
		if _, err := l2tp.ExtractParameters(tree.Tree); err != nil {
			t.Fatalf("extract valid policy %s: %v", body, err)
		}
	}
}

// TestValidateRefusesRelayRealmBinding proves `ze config validate` reaches the
// relay-realm checks of ExtractParameters. It parses each config as the
// validate command does and runs ValidateCustomSections, the walk that command
// and LoadConfig share, then asserts the refusal names the fix. Before the
// l2tp-parameters tree validator was registered, both bad configs validated.
func TestValidateRefusesRelayRealmBinding(t *testing.T) {
	const listener = " environment { l2tp { server main { ip 0.0.0.0; port 1701; } } }"
	cases := []struct {
		name, body, want string
	}{
		{"undeclared", "remote lns-a { address 203.0.113.10; } relay-realm example.net { remote lns-missing; }",
			"`l2tp remote lns-missing { address <ip> }`"},
		{"case-duplicate", "remote lns-a { address 203.0.113.10; } relay-realm example.net { remote lns-a; } " +
			"relay-realm Example.NET { remote lns-a; }", "appears twice once case is ignored"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := config.ParseTreeForValidation("l2tp { enabled true; " + tc.body + " }" + listener)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, err = config.ValidateCustomSections(tree)
			if err == nil {
				t.Fatalf("validate accepted %s", tc.body)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("refusal %q does not name the fix %q", err, tc.want)
			}
		})
	}
	tree, err := config.ParseTreeForValidation("l2tp { enabled true; remote lns-a { address 203.0.113.10; } " +
		"relay-realm example.net { remote lns-a; } }" + listener)
	if err != nil {
		t.Fatalf("parse valid binding: %v", err)
	}
	if _, err := config.ValidateCustomSections(tree); err != nil {
		t.Fatalf("validate refused a declared binding: %v", err)
	}
}
