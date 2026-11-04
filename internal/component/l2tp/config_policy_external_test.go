package l2tp_test

import (
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
		"mru 64;", "mru 1492;", "mru 1500;",
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
