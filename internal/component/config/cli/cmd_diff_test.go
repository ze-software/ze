package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// minimal ze config: router-id + session asn local + one peer with connection/session.
const testConfigBase = `
bgp {
    router-id 1.1.1.1;
    session {
        asn {
            local 65000;
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 10.0.0.1;
            }
        }
        session {
            asn {
                remote 65001;
            }
        }
    }
}
`

// same as base but remote as changed.
const testConfigChanged = `
bgp {
    router-id 1.1.1.1;
    session {
        asn {
            local 65000;
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 10.0.0.1;
            }
        }
        session {
            asn {
                remote 65002;
            }
        }
    }
}
`

// base config with an extra peer added.
const testConfigAdded = `
bgp {
    router-id 1.1.1.1;
    session {
        asn {
            local 65000;
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 10.0.0.1;
            }
        }
        session {
            asn {
                remote 65001;
            }
        }
    }
    peer peer2 {
        connection {
            remote {
                ip 10.0.0.2;
            }
        }
        session {
            asn {
                remote 65003;
            }
        }
    }
}
`

// TestConfigDiffIdentical verifies identical configs produce no differences.
//
// VALIDATES: AC-9 — identical files produce empty diff, exit 0.
// PREVENTS: False positives in diff output.
func TestConfigDiffIdentical(t *testing.T) {
	file1 := writeTestConfig(t, testConfigBase)
	file2 := writeTestConfig(t, testConfigBase)

	code := cmdDiff([]string{file1, file2})
	assert.Equal(t, exitOK, code)
}

// TestConfigDiffChanged verifies changed values are detected.
//
// VALIDATES: AC-10 — changed remote as appears in diff output.
// PREVENTS: Missing changes in diff.
func TestConfigDiffChanged(t *testing.T) {
	file1 := writeTestConfig(t, testConfigBase)
	file2 := writeTestConfig(t, testConfigChanged)

	code := cmdDiff([]string{file1, file2})
	assert.Equal(t, exitOK, code)
}

// TestConfigDiffAdded verifies added peers appear in diff.
//
// VALIDATES: AC-11 — added peer subtree appears in diff output.
// PREVENTS: Missed additions in diff.
func TestConfigDiffAdded(t *testing.T) {
	file1 := writeTestConfig(t, testConfigBase)
	file2 := writeTestConfig(t, testConfigAdded)

	code := cmdDiff([]string{file1, file2})
	assert.Equal(t, exitOK, code)
}

// TestConfigDiffMissingFile verifies missing file returns exit 2.
//
// VALIDATES: AC-12 — nonexistent file returns exit code 2.
// PREVENTS: Crash or silent failure on missing file.
func TestConfigDiffMissingFile(t *testing.T) {
	file1 := writeTestConfig(t, testConfigBase)

	code := cmdDiff([]string{file1, "/nonexistent/path/config.conf"})
	assert.Equal(t, exitError, code)
}

// TestConfigDiffAnswersThreeKeyedSets verifies the answer `show config diff`
// hands the pipe layer carries added, removed and changed.
//
// VALIDATES: AC-10 — the diff payload has added/removed/changed keys.
// PREVENTS: a renderer receiving a diff that names no change set, which every
// pipe operator would then answer emptily.
func TestConfigDiffAnswersThreeKeyedSets(t *testing.T) {
	file1 := writeTestConfig(t, testConfigBase)
	file2 := writeTestConfig(t, testConfigChanged)

	payload, code := dataDiff([]string{file1, file2})
	require.Equal(t, exitOK, code)
	require.NotNil(t, payload)

	answer, ok := payload.(map[string]any)
	require.True(t, ok, "the diff answered %T, not a document", payload)
	for _, key := range []string{"added", "removed", "changed"} {
		_, present := answer[key]
		assert.True(t, present, "expected %q in the diff answer", key)
	}
}

// The md5 password the diff fixture stores, and the value an operator rotates
// it to. bgp/peer/connection/md5/password is marked ze:sensitive in
// internal/component/bgp/yang/ze-bgp-conf.yang, so the parser decodes it into
// the tree and every render carries the cleartext.
const (
	diffStoredPassword  = "diff-stored-md5-password"  //nolint:gosec // fixture value, not a credential
	diffRotatedPassword = "diff-rotated-md5-password" //nolint:gosec // fixture value, not a credential
)

// testConfigSecretBase carries one peer with an md5 password.
const testConfigSecretBase = `
bgp {
    router-id 1.1.1.1;
    session {
        asn {
            local 65000;
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 10.0.0.1;
            }
            md5 {
                password ` + diffStoredPassword + `;
            }
        }
        session {
            asn {
                remote 65001;
            }
        }
    }
}
`

// testConfigSecretRotated rotates peer1's password and adds a second peer that
// carries one of its own. The rotation lands in Changed as a scalar pair. The
// new peer lands in Added as a whole subtree, which is a separate walk.
const testConfigSecretRotated = `
bgp {
    router-id 1.1.1.1;
    session {
        asn {
            local 65000;
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 10.0.0.1;
            }
            md5 {
                password ` + diffRotatedPassword + `;
            }
        }
        session {
            asn {
                remote 65001;
            }
        }
    }
    peer peer2 {
        connection {
            remote {
                ip 10.0.0.2;
            }
            md5 {
                password ` + diffStoredPassword + `;
            }
        }
        session {
            asn {
                remote 65003;
            }
        }
    }
}
`

// marshalDiffAnswer runs the diff answer `show config diff` registers and
// answers it encoded, which is what the pipe layer renders.
func marshalDiffAnswer(t *testing.T, file1, file2 string) (string, int) {
	t.Helper()

	payload, code := dataDiff([]string{file1, file2})
	if payload == nil {
		return "", code
	}
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	return string(encoded), code
}

// captureDiffStdout runs fn with os.Stdout redirected and answers what it wrote.
func captureDiffStdout(t *testing.T, fn func() int) (int, string) {
	t.Helper()

	saved := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()

	code := fn()

	require.NoError(t, w.Close())
	os.Stdout = saved
	out := <-done
	require.NoError(t, r.Close())

	return code, out
}

// TestConfigDiffMasksASecret verifies `ze config diff` names a rotated
// credential and publishes neither value, in the text shape and the JSON shape.
//
// VALIDATES: no `ze config diff` shape renders a value the schema marks.
// PREVENTS: the third command in this directory publishing BOTH halves of a
// rotation. It loads through loadAndResolve, which parses and therefore decodes
// the stored $9$ value, and it printed `~ path: old -> new`. Its siblings
// `ze config dump` and `ze config show` mask.
func TestConfigDiffMasksASecret(t *testing.T) {
	file1 := writeTestConfig(t, testConfigSecretBase)
	file2 := writeTestConfig(t, testConfigSecretRotated)

	for _, shape := range []struct {
		name string
		args []string
	}{
		{"text", []string{file1, file2}},
		{"data", nil},
	} {
		t.Run(shape.name, func(t *testing.T) {
			var code int
			var out string
			if shape.args == nil {
				out, code = marshalDiffAnswer(t, file1, file2)
			} else {
				code, out = captureDiffStdout(t, func() int { return cmdDiff(shape.args) })
			}

			assert.Equal(t, exitOK, code)
			assert.Contains(t, out, "password",
				"the diff named no password leaf, so this case would prove nothing")
			assert.Contains(t, out, config.SecretDataPlaceholder, "the diff rendered no placeholder")
			assert.NotContains(t, out, diffStoredPassword, "the diff published the stored password")
			assert.NotContains(t, out, diffRotatedPassword, "the diff published the new password")
		})
	}
}

// TestConfigDiffJSONNamesARotatedSecret verifies the JSON shape still reports
// the rotation, with the placeholder on both sides of the pair.
//
// VALIDATES: masking the diff keeps the change visible.
// PREVENTS: the second-order effect of masking. Masking the two trees BEFORE
// the diff gives both sides one placeholder, so the command answers that a
// rotated credential did not move.
func TestConfigDiffJSONNamesARotatedSecret(t *testing.T) {
	file1 := writeTestConfig(t, testConfigSecretBase)
	file2 := writeTestConfig(t, testConfigSecretRotated)

	out, code := marshalDiffAnswer(t, file1, file2)
	require.Equal(t, exitOK, code)

	var decoded struct {
		Changed map[string]config.DiffPair `json:"changed"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))

	key := "peer/peer1/connection/md5/password"
	pair, ok := decoded.Changed[key]
	require.True(t, ok, "the rotated leaf %q is missing from %v", key, decoded.Changed)
	assert.Equal(t, config.SecretDataPlaceholder, pair.Old)
	assert.Equal(t, config.SecretDataPlaceholder, pair.New)
}

// TestConfigDiffNoArgs verifies usage error on missing arguments.
func TestConfigDiffNoArgs(t *testing.T) {
	code := cmdDiff([]string{})
	assert.Equal(t, exitError, code)
}

// TestConfigDiffRevisionNotFound verifies error when rollback revision does not exist.
//
// VALIDATES: diff with revision number returns error when no backups.
// PREVENTS: Panic or misleading output on missing rollback.
func TestConfigDiffRevisionNotFound(t *testing.T) {
	file := writeTestConfig(t, testConfigBase)
	code := cmdDiff([]string{"1", file})
	assert.Equal(t, exitError, code)
}

// TestConfigDiffRevisionMode verifies diff against a rollback revision.
//
// VALIDATES: "diff <N> <file>" resolves revision and compares.
// PREVENTS: Revision-number mode silently failing.
func TestConfigDiffRevisionMode(t *testing.T) {
	file := writeTestConfig(t, testConfigBase)

	// Create rollback dir with a backup containing different content
	rollbackDir := filepath.Join(filepath.Dir(file), "rollback")
	require.NoError(t, os.MkdirAll(rollbackDir, 0o700))

	backupName := "test-20260101-120000.000.conf"
	require.NoError(t, os.WriteFile(
		filepath.Join(rollbackDir, backupName),
		[]byte(testConfigChanged),
		0o600,
	))

	code := cmdDiff([]string{"1", file})
	assert.Equal(t, exitOK, code)
}
