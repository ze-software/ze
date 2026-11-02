// Design: docs/architecture/testing/ci-format.md -- editor option refusals.

package testing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestETInvalidDimensionsRefuseBeforeExecution drives the file entry with no
// config: the option error must win over any attempt to create an editor.
func TestETInvalidDimensionsRefuseBeforeExecution(t *testing.T) {
	for _, name := range []string{"width", "height"} {
		for _, value := range []string{"", ":value=", ":value=0", ":value=-1", ":value=banana", ":value=999999999999999999999999"} {
			t.Run(name+value, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "invalid.et")
				require.NoError(t, os.WriteFile(path, []byte("option="+name+value+"\n"), 0o600))
				result := RunETFile(path, 1)
				require.False(t, result.Passed)
				require.Contains(t, result.Error, "option="+name)
				require.Empty(t, result.Steps)
			})
		}
	}
}

func TestETBudgetRejectsNonpositiveAndOverflow(t *testing.T) {
	for _, value := range []string{"0s", "-1s", "2562047h47m16.854775807s"} {
		t.Run(value, func(t *testing.T) {
			tc, err := parseETFile("option=timeout:value=" + value)
			require.NoError(t, err)
			_, err = testBudget(tc, 3)
			require.Error(t, err)
			require.Contains(t, err.Error(), "option=timeout")
		})
	}
}

func TestETOptionPositiveControls(t *testing.T) {
	tc, err := parseETFile(strings.Join([]string{
		"option=width:value=101", "option=height:value=37", "option=timeout:value=2s",
	}, "\n"))
	require.NoError(t, err)
	budget, err := testBudget(tc, 3)
	require.NoError(t, err)
	require.Equal(t, 6*time.Second, budget)
	tc, err = parseETFile("")
	require.NoError(t, err)
	budget, err = testBudget(tc, 1)
	require.NoError(t, err)
	require.Equal(t, 30*time.Second, budget)
}
