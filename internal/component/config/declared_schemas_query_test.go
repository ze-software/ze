// Design: docs/architecture/config/yang-config-design.md -- offline validation in query mode (S-3, AC-18, S16)
// Related: declared_schemas.go -- SchemaWithDeclaredPlugins, HasUnresolvedExternalPlugins

package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestOfflineValidateAsksProgramInQueryMode verifies S-3 / AC-18 / S16:
// offline validation queries declared external programs in declare mode
// (ZE_PLUGIN_MODE=declare) without starting their run loop, enforces their
// declared sends/receives, and fails closed with the conformance command when
// a program cannot be queried.
//
// VALIDATES: AC-18 and S16 (query mode succeeds and records Queried, undeclared
// message refused listing declared messages, missing binary fails closed with
// endpoint name, run line, cause, and conformance fix; run-loop marker never
// written; HasUnresolvedExternalPlugins does not fork or fix the module set).
// PREVENTS: Offline validation starting an external program's run loop,
// silently skipping external bus declarations, or omitting the conformance fix.
func TestOfflineValidateAsksProgramInQueryMode(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "run-loop-started.marker")
	scriptPath := filepath.Join(dir, "acme-declare.sh")

	// A tiny POSIX program that honors ZE_PLUGIN_MODE=declare by emitting one
	// valid Stage 1 declare-registration line and exiting 0 without touching
	// the run-loop marker file.
	script := `#!/bin/sh
if [ "$ZE_PLUGIN_MODE" = "declare" ]; then
  printf '#1 ze-plugin-engine:declare-registration {"protocol_version":1,"sends":[{"message":"acme:ping"}],"receives":[{"message":"acme:pong"}]}\n'
  exit 0
fi
echo "ran" > "` + marker + `"
exit 1
`
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755)) //nolint:gosec // test fixture script must be executable

	validCfg := `bus {
    endpoint acme {
        external {
            run "` + scriptPath + `";
        }
        receive {
            allow [ acme:pong ];
        }
    }
}
`
	// Step 0: HasUnresolvedExternalPlugins detects the external endpoint
	// WITHOUT executing the program or fixing the module set (AC-18 daemon-first).
	t.Run("has_unresolved_does_not_fork", func(t *testing.T) {
		restore := ResetDeclaredSchemasForTest()
		t.Cleanup(restore)

		assert.True(t, HasUnresolvedExternalPlugins(ConfigSource{Text: validCfg, Path: filepath.Join(dir, "site.conf")}))
		_, statErr := os.Stat(marker)
		assert.True(t, os.IsNotExist(statErr), "HasUnresolvedExternalPlugins must never run the program")
		assert.Empty(t, ExternalBusDeclarations(), "HasUnresolvedExternalPlugins must not query external programs")
	})

	// Step 1: Valid config passes query mode, records Queried, and validates.
	t.Run("valid_program_in_query_mode", func(t *testing.T) {
		restore := ResetDeclaredSchemasForTest()
		t.Cleanup(restore)

		schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{
			Text: validCfg,
			Path: filepath.Join(dir, "site.conf"),
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"acme"}, declared.Queried())
		assert.Empty(t, declared.WithoutSchema())

		tree, parseErr := NewParser(schema).Parse(validCfg)
		require.NoError(t, parseErr)
		require.NoError(t, ValidateBusConfig(tree))

		_, statErr := os.Stat(marker)
		assert.True(t, os.IsNotExist(statErr), "query mode must never start the program's run loop")
	})

	// Step 2: Undeclared message on the queried external endpoint is refused
	// with what the program declared (Refusal 4).
	t.Run("undeclared_message_refused_with_declared_list", func(t *testing.T) {
		restore := ResetDeclaredSchemasForTest()
		t.Cleanup(restore)

		badAllowCfg := `bus {
    endpoint acme {
        external {
            run "` + scriptPath + `";
        }
        receive {
            allow [ acme:nosuch ];
        }
    }
}
`
		schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{
			Text: badAllowCfg,
			Path: filepath.Join(dir, "site.conf"),
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"acme"}, declared.Queried())

		tree, parseErr := NewParser(schema).Parse(badAllowCfg)
		require.NoError(t, parseErr)
		valErr := ValidateBusConfig(tree)
		require.Error(t, valErr)
		assert.Contains(t, valErr.Error(), "acme:nosuch")
		assert.Contains(t, valErr.Error(), "acme:pong")

		_, statErr := os.Stat(marker)
		assert.True(t, os.IsNotExist(statErr), "run loop must never start")
	})

	// Step 3: Missing binary fails closed naming endpoint, run line, "not found",
	// and the exact conformance command fix.
	t.Run("missing_program_fails_closed", func(t *testing.T) {
		restore := ResetDeclaredSchemasForTest()
		t.Cleanup(restore)

		missingCfg := `bus {
    endpoint acme {
        external {
            run "./missing-acme-binary";
        }
    }
}
`
		_, _, err := SchemaWithDeclaredPlugins(ConfigSource{
			Text: missingCfg,
			Path: filepath.Join(dir, "site.conf"),
		})
		require.Error(t, err)
		opErr, ok := errors.AsType[*opererr.Error](err)
		require.True(t, ok, "query failure must return *opererr.Error")
		assert.Contains(t, opErr.Error(), "acme")
		assert.Contains(t, opErr.Error(), "./missing-acme-binary")
		assert.Contains(t, opErr.Error(), "not found")
		assert.Equal(t, `ze show plugin conformance run "./missing-acme-binary"`, opErr.Fix())
	})
}
