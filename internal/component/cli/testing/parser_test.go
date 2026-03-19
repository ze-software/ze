package testing

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseEmptyFile verifies empty file produces empty test case.
//
// VALIDATES: Empty input doesn't crash parser.
// PREVENTS: Nil panic on empty input.
func TestParseEmptyFile(t *testing.T) {
	tc, err := ParseETFile("")
	require.NoError(t, err)
	assert.Empty(t, tc.Options)
	assert.Empty(t, tc.Inputs)
	assert.Empty(t, tc.Expects)
	assert.Empty(t, tc.Tmpfs)
}

// TestParseComments verifies comments are ignored.
//
// VALIDATES: Lines starting with # are skipped.
// PREVENTS: Comments interpreted as actions.
func TestParseComments(t *testing.T) {
	content := `# This is a comment
# Another comment
option=file:path=test.conf
# Comment between lines
input=type:text=hello`

	tc, err := ParseETFile(content)
	require.NoError(t, err)
	assert.Len(t, tc.Options, 1)
	assert.Len(t, tc.Inputs, 1)
}

// TestParseOption verifies option parsing.
//
// VALIDATES: option= lines parsed correctly with key=value pairs.
// PREVENTS: Options lost or malformed.
func TestParseOption(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantType string
		wantKV   map[string]string
	}{
		{
			name:     "file option",
			line:     "option=file:path=test.conf",
			wantType: "file",
			wantKV:   map[string]string{"path": "test.conf"},
		},
		{
			name:     "timeout option",
			line:     "option=timeout:value=30s",
			wantType: "timeout",
			wantKV:   map[string]string{"value": "30s"},
		},
		{
			name:     "width option",
			line:     "option=width:value=80",
			wantType: "width",
			wantKV:   map[string]string{"value": "80"},
		},
		{
			name:     "height option",
			line:     "option=height:value=24",
			wantType: "height",
			wantKV:   map[string]string{"value": "24"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Options, 1)
			opt := tc.Options[0]
			assert.Equal(t, tt.wantType, opt.Type)
			for k, v := range tt.wantKV {
				assert.Equal(t, v, opt.Values[k], "key %s", k)
			}
		})
	}
}

// TestParseInputType verifies input=type: parsing.
//
// VALIDATES: Text input converted to correct action.
// PREVENTS: User typing lost.
func TestParseInputType(t *testing.T) {
	tc, err := ParseETFile("input=type:text=edit bgp")
	require.NoError(t, err)
	require.Len(t, tc.Inputs, 1)

	inp := tc.Inputs[0]
	assert.Equal(t, "type", inp.Action)
	assert.Equal(t, "edit bgp", inp.Values["text"])
}

// TestParseInputKey verifies input=key: parsing.
//
// VALIDATES: Special key names parsed correctly.
// PREVENTS: Tab, Enter, etc. not recognized.
func TestParseInputKey(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey string
	}{
		{"tab key", "input=key:name=tab", "tab"},
		{"enter key", "input=key:name=enter", "enter"},
		{"up arrow", "input=key:name=up", "up"},
		{"down arrow", "input=key:name=down", "down"},
		{"escape", "input=key:name=esc", "esc"},
		{"backspace", "input=key:name=backspace", "backspace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Inputs, 1)
			assert.Equal(t, "key", tc.Inputs[0].Action)
			assert.Equal(t, tt.wantKey, tc.Inputs[0].Values["name"])
		})
	}
}

// TestParseInputShorthand verifies shorthand input syntax.
//
// VALIDATES: input=tab, input=enter, etc. work without key:name=.
// PREVENTS: Verbose syntax required for common keys.
func TestParseInputShorthand(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey string
	}{
		{"tab shorthand", "input=tab", "tab"},
		{"enter shorthand", "input=enter", "enter"},
		{"up shorthand", "input=up", "up"},
		{"down shorthand", "input=down", "down"},
		{"esc shorthand", "input=esc", "esc"},
		{"backspace shorthand", "input=backspace", "backspace"},
		{"delete shorthand", "input=delete", "delete"},
		{"home shorthand", "input=home", "home"},
		{"end shorthand", "input=end", "end"},
		{"pgup shorthand", "input=pgup", "pgup"},
		{"pgdn shorthand", "input=pgdn", "pgdn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Inputs, 1)
			assert.Equal(t, "key", tc.Inputs[0].Action)
			assert.Equal(t, tt.wantKey, tc.Inputs[0].Values["name"])
		})
	}
}

// TestParseInputCtrl verifies ctrl key combination parsing.
//
// VALIDATES: input=ctrl:key=c parses correctly.
// PREVENTS: Ctrl combinations not recognized.
func TestParseInputCtrl(t *testing.T) {
	tc, err := ParseETFile("input=ctrl:key=c")
	require.NoError(t, err)
	require.Len(t, tc.Inputs, 1)

	inp := tc.Inputs[0]
	assert.Equal(t, "ctrl", inp.Action)
	assert.Equal(t, "c", inp.Values["key"])
}

// TestParseExpectContext verifies context expectations.
//
// VALIDATES: expect=context: parsed correctly.
// PREVENTS: Context assertions not working.
func TestParseExpectContext(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantType  string
		wantValue string
	}{
		{
			name:      "context path",
			line:      "expect=context:path=bgp.peer.peer1",
			wantType:  "context",
			wantValue: "bgp.peer.peer1",
		},
		{
			name:      "context root",
			line:      "expect=context:root",
			wantType:  "context",
			wantValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Expects, 1)
			exp := tc.Expects[0]
			assert.Equal(t, tt.wantType, exp.Type)
			if tt.wantValue != "" {
				assert.Equal(t, tt.wantValue, exp.Values["path"])
			} else {
				_, hasRoot := exp.Values["root"]
				assert.True(t, hasRoot, "should have root flag")
			}
		})
	}
}

// TestParseExpectCompletion verifies completion expectations.
//
// VALIDATES: expect=completion: parsed correctly.
// PREVENTS: Completion assertions not working.
func TestParseExpectCompletion(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantType string
		wantKey  string
		wantVal  string
	}{
		{
			name:     "contains",
			line:     "expect=completion:contains=set,delete,edit",
			wantType: "completion",
			wantKey:  "contains",
			wantVal:  "set,delete,edit",
		},
		{
			name:     "exact",
			line:     "expect=completion:exact=true,false",
			wantType: "completion",
			wantKey:  "exact",
			wantVal:  "true,false",
		},
		{
			name:     "count",
			line:     "expect=completion:count=5",
			wantType: "completion",
			wantKey:  "count",
			wantVal:  "5",
		},
		{
			name:     "empty",
			line:     "expect=completion:empty",
			wantType: "completion",
			wantKey:  "empty",
			wantVal:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Expects, 1)
			exp := tc.Expects[0]
			assert.Equal(t, tt.wantType, exp.Type)
			if tt.wantVal != "" {
				assert.Equal(t, tt.wantVal, exp.Values[tt.wantKey])
			} else {
				_, hasKey := exp.Values[tt.wantKey]
				assert.True(t, hasKey, "should have %s flag", tt.wantKey)
			}
		})
	}
}

// TestParseExpectContent verifies content expectations.
//
// VALIDATES: expect=content: parsed correctly.
// PREVENTS: Content assertions not working.
func TestParseExpectContent(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey string
		wantVal string
	}{
		{
			name:    "contains",
			line:    "expect=content:contains=as 65001",
			wantKey: "contains",
			wantVal: "as 65001",
		},
		{
			name:    "not-contains",
			line:    "expect=content:not-contains=error",
			wantKey: "not-contains",
			wantVal: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Expects, 1)
			exp := tc.Expects[0]
			assert.Equal(t, "content", exp.Type)
			assert.Equal(t, tt.wantVal, exp.Values[tt.wantKey])
		})
	}
}

// TestParseExpectErrors verifies error expectations.
//
// VALIDATES: expect=errors: parsed correctly.
// PREVENTS: Error count assertions not working.
func TestParseExpectErrors(t *testing.T) {
	tc, err := ParseETFile("expect=errors:count=0")
	require.NoError(t, err)
	require.Len(t, tc.Expects, 1)

	exp := tc.Expects[0]
	assert.Equal(t, "errors", exp.Type)
	assert.Equal(t, "0", exp.Values["count"])
}

// TestParseExpectDirty verifies dirty flag expectations.
//
// VALIDATES: expect=dirty: parsed correctly.
// PREVENTS: Dirty state assertions not working.
func TestParseExpectDirty(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantVal string
	}{
		{"dirty true", "expect=dirty:true", "true"},
		{"dirty false", "expect=dirty:false", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Expects, 1)
			exp := tc.Expects[0]
			assert.Equal(t, "dirty", exp.Type)
			_, hasTrue := exp.Values["true"]
			_, hasFalse := exp.Values["false"]
			if tt.wantVal == "true" {
				assert.True(t, hasTrue)
			} else {
				assert.True(t, hasFalse)
			}
		})
	}
}

// TestParseExpectGhost verifies ghost text expectations.
//
// VALIDATES: expect=ghost: parsed correctly.
// PREVENTS: Ghost text assertions not working.
func TestParseExpectGhost(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey string
		wantVal string
	}{
		{
			name:    "ghost text",
			line:    "expect=ghost:text=-as",
			wantKey: "text",
			wantVal: "-as",
		},
		{
			name:    "ghost empty",
			line:    "expect=ghost:empty",
			wantKey: "empty",
			wantVal: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Expects, 1)
			exp := tc.Expects[0]
			assert.Equal(t, "ghost", exp.Type)
			if tt.wantVal != "" {
				assert.Equal(t, tt.wantVal, exp.Values[tt.wantKey])
			} else {
				_, hasKey := exp.Values[tt.wantKey]
				assert.True(t, hasKey)
			}
		})
	}
}

// TestParseTmpfs verifies tmpfs block parsing.
//
// VALIDATES: tmpfs= with terminator captures content.
// PREVENTS: Embedded files not extracted.
func TestParseTmpfs(t *testing.T) {
	content := `tmpfs=test.conf:terminator=EOF_CONF
bgp {
  local {
    as 65000
  }
  router-id 1.2.3.4
}
EOF_CONF`

	tc, err := ParseETFile(content)
	require.NoError(t, err)
	require.Len(t, tc.Tmpfs, 1)

	tf := tc.Tmpfs[0]
	assert.Equal(t, "test.conf", tf.Path)
	assert.Contains(t, tf.Content, "bgp {")
	assert.Contains(t, tf.Content, "as 65000")
	assert.Contains(t, tf.Content, "router-id 1.2.3.4")
}

// TestParseTmpfsMultiple verifies multiple tmpfs blocks.
//
// VALIDATES: Multiple embedded files parsed correctly.
// PREVENTS: Only first file captured.
func TestParseTmpfsMultiple(t *testing.T) {
	content := `tmpfs=original.conf:terminator=EOF_ORIG
bgp { local { as 65000; } }
EOF_ORIG

tmpfs=merge.conf:terminator=EOF_MERGE
bgp { peer peer1 { remote { ip 1.1.1.1; as 65001; } } }
EOF_MERGE`

	tc, err := ParseETFile(content)
	require.NoError(t, err)
	require.Len(t, tc.Tmpfs, 2)

	assert.Equal(t, "original.conf", tc.Tmpfs[0].Path)
	assert.Contains(t, tc.Tmpfs[0].Content, "as 65000")

	assert.Equal(t, "merge.conf", tc.Tmpfs[1].Path)
	assert.Contains(t, tc.Tmpfs[1].Content, "peer peer1")
}

// TestParseTmpfsWithMode verifies mode option parsing.
//
// VALIDATES: mode= option parsed correctly.
// PREVENTS: Executable files not marked correctly.
func TestParseTmpfsWithMode(t *testing.T) {
	content := `tmpfs=script.sh:mode=755:terminator=EOF_SH
#!/bin/bash
echo "hello"
EOF_SH`

	tc, err := ParseETFile(content)
	require.NoError(t, err)
	require.Len(t, tc.Tmpfs, 1)

	tf := tc.Tmpfs[0]
	assert.Equal(t, "script.sh", tf.Path)
	assert.Equal(t, "755", tf.Mode)
}

// TestParseWait verifies wait action parsing.
//
// VALIDATES: wait= actions parsed correctly.
// PREVENTS: Timing controls not working.
func TestParseWait(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey string
		wantVal string
	}{
		{
			name:    "wait ms",
			line:    "wait=ms:200",
			wantKey: "ms",
			wantVal: "200",
		},
		{
			name:    "wait validation",
			line:    "wait=validation",
			wantKey: "validation",
			wantVal: "",
		},
		{
			name:    "wait timer expire",
			line:    "wait=timer:expire",
			wantKey: "timer",
			wantVal: "expire",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Waits, 1)
			w := tc.Waits[0]
			if tt.wantVal != "" {
				assert.Equal(t, tt.wantVal, w.Values[tt.wantKey])
			} else {
				_, hasKey := w.Values[tt.wantKey]
				assert.True(t, hasKey)
			}
		})
	}
}

// TestParseCompleteExample verifies full test file parsing.
//
// VALIDATES: Complete test file with all elements parses correctly.
// PREVENTS: Integration issues between different line types.
func TestParseCompleteExample(t *testing.T) {
	content := `# Test: Edit navigation
# VALIDATES: Hierarchical navigation preserves context path

tmpfs=test.conf:terminator=EOF_CONF
bgp {
  local {
    as 65000
  }
  router-id 1.2.3.4
  peer peer1 {
    remote {
      ip 1.1.1.1
      as 65001
    }
  }
}
EOF_CONF

option=file:path=test.conf
option=timeout:value=10s

expect=context:root
expect=dirty:false

input=type:text=edit bgp
input=enter
expect=context:path=bgp
expect=error:none

input=type:text=set
expect=completion:contains=local,router-id,peer`

	tc, err := ParseETFile(content)
	require.NoError(t, err)

	// Verify all parts parsed
	assert.Len(t, tc.Tmpfs, 1, "should have 1 tmpfs")
	assert.Len(t, tc.Options, 2, "should have 2 options")
	assert.Len(t, tc.Inputs, 3, "should have 3 inputs")
	assert.Len(t, tc.Expects, 5, "should have 5 expects")

	// Verify order preserved
	assert.Equal(t, "type", tc.Inputs[0].Action)
	assert.Equal(t, "edit bgp", tc.Inputs[0].Values["text"])
}

// TestParseInvalidLine verifies unknown lines are handled.
//
// VALIDATES: Unknown action types produce clear error.
// PREVENTS: Silent failure on typos.
func TestParseInvalidLine(t *testing.T) {
	_, err := ParseETFile("unknown=foo:bar=baz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

// TestParseMissingTerminator verifies error on missing terminator.
//
// VALIDATES: Unclosed tmpfs block produces error.
// PREVENTS: Content bleeding into next section.
func TestParseMissingTerminator(t *testing.T) {
	content := `tmpfs=test.conf:terminator=EOF_CONF
bgp { local { as 65000; } }
# Missing EOF_CONF terminator`

	_, err := ParseETFile(content)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "terminator")
}

// TestParseBlankLines verifies blank lines are skipped.
//
// VALIDATES: Empty lines don't affect parsing.
// PREVENTS: Blank lines causing errors.
func TestParseBlankLines(t *testing.T) {
	content := `option=file:path=test.conf

input=type:text=hello

expect=dirty:false`

	tc, err := ParseETFile(content)
	require.NoError(t, err)
	assert.Len(t, tc.Options, 1)
	assert.Len(t, tc.Inputs, 1)
	assert.Len(t, tc.Expects, 1)
}

// TestParseExpectStatus verifies status message expectations.
//
// VALIDATES: expect=status: parsed correctly.
// PREVENTS: Status message assertions not working.
func TestParseExpectStatus(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey string
		wantVal string
	}{
		{
			name:    "status contains",
			line:    "expect=status:contains=committed",
			wantKey: "contains",
			wantVal: "committed",
		},
		{
			name:    "status empty",
			line:    "expect=status:empty",
			wantKey: "empty",
			wantVal: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Expects, 1)
			exp := tc.Expects[0]
			assert.Equal(t, "status", exp.Type)
			if tt.wantVal != "" {
				assert.Equal(t, tt.wantVal, exp.Values[tt.wantKey])
			} else {
				_, hasKey := exp.Values[tt.wantKey]
				assert.True(t, hasKey)
			}
		})
	}
}

// TestParseExpectError verifies command error expectations.
//
// VALIDATES: expect=error: parsed correctly.
// PREVENTS: Command error assertions not working.
func TestParseExpectError(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey string
		wantVal string
	}{
		{
			name:    "error contains",
			line:    "expect=error:contains=not found",
			wantKey: "contains",
			wantVal: "not found",
		},
		{
			name:    "error none",
			line:    "expect=error:none",
			wantKey: "none",
			wantVal: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Expects, 1)
			exp := tc.Expects[0]
			assert.Equal(t, "error", exp.Type)
			if tt.wantVal != "" {
				assert.Equal(t, tt.wantVal, exp.Values[tt.wantKey])
			} else {
				_, hasKey := exp.Values[tt.wantKey]
				assert.True(t, hasKey)
			}
		})
	}
}

// TestParseExpectTemplate verifies template mode expectations.
//
// VALIDATES: expect=template: parsed correctly.
// PREVENTS: Template mode assertions not working.
func TestParseExpectTemplate(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantVal string
	}{
		{"template true", "expect=template:true", "true"},
		{"template false", "expect=template:false", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Expects, 1)
			exp := tc.Expects[0]
			assert.Equal(t, "template", exp.Type)
			_, hasVal := exp.Values[tt.wantVal]
			assert.True(t, hasVal)
		})
	}
}

// TestParseExpectTimer verifies timer expectations.
//
// VALIDATES: expect=timer: parsed correctly.
// PREVENTS: Commit confirm timer assertions not working.
func TestParseExpectTimer(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey string
	}{
		{"timer active", "expect=timer:active", "active"},
		{"timer inactive", "expect=timer:inactive", "inactive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Expects, 1)
			exp := tc.Expects[0]
			assert.Equal(t, "timer", exp.Type)
			_, hasKey := exp.Values[tt.wantKey]
			assert.True(t, hasKey)
		})
	}
}

// TestParseExpectDropdown verifies dropdown expectations.
//
// VALIDATES: expect=dropdown: parsed correctly.
// PREVENTS: Dropdown visibility assertions not working.
func TestParseExpectDropdown(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey string
	}{
		{"dropdown visible", "expect=dropdown:visible", "visible"},
		{"dropdown hidden", "expect=dropdown:hidden", "hidden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := ParseETFile(tt.line)
			require.NoError(t, err)
			require.Len(t, tc.Expects, 1)
			exp := tc.Expects[0]
			assert.Equal(t, "dropdown", exp.Type)
			_, hasKey := exp.Values[tt.wantKey]
			assert.True(t, hasKey)
		})
	}
}
