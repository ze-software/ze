//go:build ze_l2tp

package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	_ "github.com/ze-software/ze/internal/component/l2tp/pppoe/yang"
)

// TestPPPoEAuthPolicyRepairCommands executes each repair alternative from the
// actual configuration error through the editor, then re-extracts the policy.
func TestPPPoEAuthPolicyRepairCommands(t *testing.T) {
	const seed = `pppoe {
    auth-method chap-md5;
    authentication-allow [ pap ];
}`
	_, policyErr := pppoe.ExtractParameters(map[string]any{
		"pppoe": map[string]any{"authentication-allow": []string{"pap"}},
	})
	if policyErr == nil {
		t.Fatal("invalid explicit policy produced no error")
	}
	parts := strings.Split(policyErr.Error(), "`")
	var commands []string
	for i := 1; i < len(parts); i += 2 {
		commands = append(commands, parts[i])
	}
	if len(commands) != 4 {
		t.Fatalf("error does not name both executable repair alternatives: %s", policyErr)
	}
	for name, repair := range map[string][]string{
		"restore default":  commands[:1],
		"select CHAP only": commands[1:],
	} {
		t.Run(name, func(t *testing.T) {
			configPath := writeTestConfig(t, seed)
			ed, err := NewLooseFileEditor(nil, configPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = ed.Close() })
			model, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
			if err != nil {
				t.Fatal(err)
			}
			for _, command := range repair {
				if _, err := model.dispatchCommand(command); err != nil {
					t.Fatalf("printed command %q failed: %v", command, err)
				}
			}
			parameters, err := pppoe.ExtractParameters(ed.Tree().ToMap())
			if err != nil {
				t.Fatalf("printed repair left invalid policy: %v", err)
			}
			if parameters.AuthMethod != ppp.AuthMethodCHAPMD5 ||
				!slices.Equal(parameters.AuthAllow, []ppp.AuthMethod{ppp.AuthMethodCHAPMD5}) ||
				parameters.AllowNoAuth {
				t.Fatalf("printed repair produced unintended auth policy: %+v", parameters)
			}
		})
	}
}
