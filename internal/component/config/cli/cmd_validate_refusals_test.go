// Design: docs/architecture/config/yang-config-design.md -- Value refusals
// Related: cmd_validate.go -- ValidateContent keeps every refusal typed

package cli

import (
	"errors"
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestValidateContentTwoValueRefusals exercises the session commit boundary
// with two malformed leaves in different sections (AC-8, AC-9, AC-21).
func TestValidateContentTwoValueRefusals(t *testing.T) {
	inputs := map[string]string{
		"hierarchical": `bgp {
    peer peer1 {
        session { capability { route-refresh requre; } }
    }
}
ospf {
    interfaces { interface eth0 { cost 0; } }
}
`,
		"set": `set bgp peer peer1 session capability route-refresh requre
set ospf interfaces interface eth0 cost 0
`,
	}
	for name, input := range inputs {
		t.Run(name, func(t *testing.T) {
			err := ValidateContent(input, "candidate")
			if err == nil {
				t.Fatal("two invalid values were accepted")
			}
			text := err.Error()
			for _, want := range []string{
				"config validation failed:\n",
				`  bgp peer peer1 session capability route-refresh: "requre" is not an accepted value (did you mean require?)`,
				"\n  fix: set bgp peer peer1 session capability route-refresh require",
				`  ospf interfaces interface eth0 cost: "0" is outside the accepted range`,
				"\n  cost accepts a value in 1..65535",
				"\n  fix: set ospf interfaces interface eth0 cost <1..65535>",
			} {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q in:\n%s", want, text)
				}
			}
			if count := strings.Count(text, "\n  fix: "); count != 2 {
				t.Errorf("got %d refusal fixes, want 2:\n%s", count, text)
			}
			if strings.Contains(text, "line ") {
				t.Errorf("session refusal carries a file line:\n%s", text)
			}
			for _, line := range strings.Split(text, "\n")[1:] {
				if !strings.HasPrefix(line, "  ") {
					t.Errorf("continuation is not indented: %q", line)
				}
			}
			var refusal *configyang.ValidationError
			if !errors.As(err, &refusal) {
				t.Fatal("the first refusal's typed facts were lost")
			}
			if refusal.Got != "requre" {
				t.Errorf("first refusal Got = %q, want requre", refusal.Got)
			}
			var operator *opererr.Error
			if !errors.As(err, &operator) {
				t.Fatal("the first refusal's operator error was lost")
			}
			if operator.Fix() != "set bgp peer peer1 session capability route-refresh require" {
				t.Errorf("first fix = %q", operator.Fix())
			}
		})
	}
}
