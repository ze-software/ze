// Design: docs/architecture/cli/error-surface.md -- a fix is the exact command to type.

package cli

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestPrintedEnumFixPreservesListKey replays the refusal's exact correction
// through dispatch, so quoting must preserve the actual schema list key.
func TestPrintedEnumFixPreservesListKey(t *testing.T) {
	for _, key := range []string{"edge routers", `edge"routers`, `edge\routers`, "edge[routers]", "<edge routers>", "|", "edge#3", "edge#3#4"} {
		for _, underTop := range []bool{false, true} {
			t.Run(key+"/top="+strconv.FormatBool(underTop), func(t *testing.T) {
				m, _ := topTestModel(t)
				if underTop {
					m = enterLine(t, m, "edit bgp peer peer1")
					require.NoError(t, m.err)
				}
				position := append([]string(nil), m.contextPath...)
				args := []string{"set", "bgp", "group", key, "session", "capability", "route-refresh", "requre"}
				if underTop {
					args = append([]string{"top"}, args...)
				}
				keyAt := 3
				if underTop {
					keyAt++
				}
				line := config.FormatEditPath(args[:keyAt]) + " " + strconv.Quote(key) + " " + config.FormatEditPath(args[keyAt+1:])
				m = enterLine(t, m, line)
				err := m.err
				require.Error(t, err)
				var refusal *configyang.ValidationError
				require.True(t, errors.As(err, &refusal), "the entered command must reach the enum refusal: %v", err)
				printed := strings.TrimPrefix(lastLine(err.Error()), "fix: ")
				m = enterLine(t, m, printed)
				require.NoError(t, m.err, "printed fix %q must execute", printed)
				assert.Equal(t, position, m.contextPath)
				group := m.editor.tree.GetContainer("bgp").GetList("group")[key]
				require.NotNil(t, group, "the original list key survives the printed fix")
				value, ok := group.GetContainer("session").GetContainer("capability").Get("route-refresh")
				assert.True(t, ok)
				assert.Equal(t, "require", value)
				assert.Nil(t, m.editor.tree.GetContainer("bgp").GetList("group")["edge"])
			})
		}
	}
}

// TestFrameFixPreservesLiteralTokens checks the rendering boundary itself,
// including literal angle brackets that are data rather than placeholders.
func TestFrameFixPreservesLiteralTokens(t *testing.T) {
	for _, value := range []string{"", "two words", "with\ttab", `say "hello"`, `slash\value`, "[one two]", "<one two>", "/* SECRET-DATA */", "|"} {
		t.Run(value, func(t *testing.T) {
			args := []string{"bgp", "description", value}
			line := FrameFix(FixCommand{Verb: cmdSet, Args: args}, nil)
			tokens, err := config.ParseEditPath(line)
			require.NoError(t, err)
			assert.Equal(t, append([]string{cmdSet}, args...), tokens)
		})
	}
}

// TestLengthFixLeavesTheCorrectionPlaceholderRaw distinguishes human-readable
// correction syntax from literal strings that happen to contain angle brackets.
func TestLengthFixLeavesTheCorrectionPlaceholderRaw(t *testing.T) {
	err := configyang.LengthRefusal(configyang.RefusedValue{
		Path: []string{"probe", "label"}, Leaf: "label", Value: "long",
	}, "1..3", "")
	framed := FramedSetRefusal(err, nil, []string{"probe", "label", "long"}, nil)
	assert.Equal(t, "fix: set probe label <1..3 characters>", lastLine(framed.Error()))
}

// TestPrintedKeyFixPreservesFollowingValue exercises the direct editor caller:
// correcting a near enum key must preserve the complete final leaf-list member.
func TestPrintedKeyFixPreservesFollowingValue(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-key-activation-probe-conf.yang", keyActivationProbe))
	for _, value := range []string{"two words", `say "hello"`, "[one two]", "<one two>", "|"} {
		t.Run(value, func(t *testing.T) {
			m, _ := topTestModel(t)
			err := m.editor.SetValue([]string{"key-probe", "lane", "blu"}, "member", value)
			require.Error(t, err)
			printed := strings.TrimPrefix(lastLine(err.Error()), "fix: ")
			_, err = m.dispatchCommand(printed)
			require.NoError(t, err, "printed fix %q must execute", printed)
			entry := m.editor.tree.GetContainer("key-probe").GetList("lane")["blue"]
			require.NotNil(t, entry)
			assert.Equal(t, []config.MemberState{{Value: value}}, entry.GetMultiValuesState("member"))
		})
	}
}
