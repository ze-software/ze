// Design: docs/architecture/config/yang-config-design.md — the SSH CLI never
// echoes a secret back to the terminal

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"

	// The OSPFv3 IPsec integrity key is the one ze:sensitive leaf whose YANG
	// type the completer enforces, so it is what makes the refusal reachable.
	_ "github.com/ze-software/ze/internal/plugins/ospf/yang"
)

// The values these cases type at the CLI bar. Each carries a distinctive TAIL,
// asserted separately from the whole: a message can escape the value and then
// contain no copy of it while publishing every character.
const (
	typedCLISecret     = "hunter2-a4471bc-tail"
	typedCLISecretTail = "a4471bc-tail"
)

// editModelOverRealSchema answers a Model whose editor reads the shipped YANG,
// which is where the ze:sensitive marking lives. The synthetic fixture in
// secret_test.go cannot serve these cases: cmdSet validates through the
// completer, and the completer reads the module registry rather than an
// editor's schema.
func editModelOverRealSchema(t *testing.T) *Model {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600))

	editor, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { editor.Close() }) //nolint:errcheck // test cleanup

	model, err := NewModel(editor, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	return &model
}

// TestSSHCLISetNeverEchoesASecret drives the line the operator types.
//
// VALIDATES: the status line `set` writes for a ze:sensitive leaf carries
// config.SecretDataPlaceholder and no part of the value, and so does the
// refusal when the value fails its YANG type.
// PREVENTS: the config editor printing the credential the operator just typed.
// The status line sits in the scrollback of a shared terminal and in any
// session recording, and Model.cmdSet built it with the raw value.
func TestSSHCLISetNeverEchoesASecret(t *testing.T) {
	t.Run("the status line", func(t *testing.T) {
		model := editModelOverRealSchema(t)

		result, err := model.dispatchCommand("set system authentication user alice plaintext-password " + typedCLISecret)
		require.NoError(t, err)

		require.Contains(t, result.statusMessage, "system authentication user alice plaintext-password",
			"the status line must name the leaf, or this case proves nothing")
		assert.NotContains(t, result.statusMessage, typedCLISecret, "the status line published the value")
		assert.NotContains(t, result.statusMessage, typedCLISecretTail, "the status line published the tail of the value")
		assert.Contains(t, result.statusMessage, config.SecretDataPlaceholder, "the status line wrote no placeholder")
	})

	t.Run("the refusal", func(t *testing.T) {
		model := editModelOverRealSchema(t)

		_, err := model.dispatchCommand(
			"set ospf address-family ipv6 interfaces interface eth0 ipsec key " + typedCLISecret)
		require.Error(t, err, "an OSPFv3 integrity key is hex, so this value must be refused")
		require.Contains(t, err.Error(), "invalid value",
			"the refusal must be the value validator's, or this case proves nothing")
		assert.NotContains(t, err.Error(), typedCLISecretTail, "the refusal published the tail of the value")
		assert.Contains(t, err.Error(), config.SecretDataPlaceholder, "the refusal wrote no placeholder")
	})
}

// TestSSHCLISetStillEchoesAValueTheSchemaDoesNotMark is the other polarity.
//
// VALIDATES: an unmarked leaf still echoes what the operator typed.
// PREVENTS: a vacuous pass above. config.DisplayValueAtPath fails closed on a
// path it cannot resolve, so a resolver that cannot walk an ordinary token path
// would mask every status line the editor writes and the test above would still
// pass.
func TestSSHCLISetStillEchoesAValueTheSchemaDoesNotMark(t *testing.T) {
	model := editModelOverRealSchema(t)

	result, err := model.dispatchCommand("set bgp router-id 10.9.8.7")
	require.NoError(t, err)

	assert.Contains(t, result.statusMessage, "10.9.8.7",
		"the editor masked a leaf the schema does not mark, so its mask reads something other than the schema")
	assert.NotContains(t, result.statusMessage, config.SecretDataPlaceholder)
}

// TestCommitConflictNeverEchoesASecret drives `commit` when two operators
// changed the same credential.
//
// VALIDATES: the LIVE conflict line names the contested path and neither value.
// PREVENTS: the conflict report publishing a credential, including the one
// ANOTHER operator typed. Conflict.MyValue and Conflict.OtherValue were the raw
// leaf values, and six renderers write them: the SSH CLI commit and load
// reports, the web CLI bar, the web CLI terminal and the web commit handler.
//
// The path stays in the clear, so the operator still learns which leaf is
// contested and can re-set it.
func TestCommitConflictNeverEchoesASecret(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600))

	mine := editSessionModel(t, configPath, "alice", typedCLISecret)
	_ = editSessionModel(t, configPath, "bob", "bob-"+typedCLISecretTail)

	result, err := mine.dispatchCommand("commit")
	require.NoError(t, err)

	require.Contains(t, result.output, "LIVE",
		"the commit must report a live conflict, or this case proves nothing")
	require.Contains(t, result.output, "plaintext-password",
		"the conflict must name the contested leaf")
	assert.NotContains(t, result.output, typedCLISecretTail,
		"the conflict report published the tail of a value")
	assert.Contains(t, result.output, config.SecretDataPlaceholder,
		"the conflict report wrote no placeholder")
}

// editSessionModel answers a Model editing configPath as username, with the
// secret already set on the shared credential leaf.
func editSessionModel(t *testing.T, configPath, username, secret string) *Model {
	t.Helper()

	editor, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { editor.Close() }) //nolint:errcheck // test cleanup
	editor.SetSession(NewEditSession(username, "ssh"))

	model, err := NewModel(editor, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)

	_, err = model.dispatchCommand("set system authentication user carol plaintext-password " + secret)
	require.NoError(t, err)

	return &model
}

// TestPendingChangeSummaryNeverEchoesASecret covers the line `ze config edit`
// writes when it offers to adopt an orphaned session.
//
// VALIDATES: the summary of a pending set on a ze:sensitive leaf names the path
// and carries config.SecretDataPlaceholder in place of the value.
// PREVENTS: the adoption prompt printing every credential the previous session
// typed, one line per change. config.PendingChange.Summary wrote `set <path>
// <value>` with the raw value.
//
// It drives Editor.PendingChangeSummary rather than cmdEdit, which is one
// fmt.Fprintf from it: the prompt reads stdin and then launches the terminal
// UI, so the command itself needs a terminal this test has not got.
func TestPendingChangeSummaryNeverEchoesASecret(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfig), 0o600))

	model := editSessionModel(t, configPath, "alice", typedCLISecret)
	_, err := model.dispatchCommand("set bgp router-id 10.9.8.7")
	require.NoError(t, err)

	changes := model.editor.PendingChanges(model.editor.session.ID)
	require.NotEmpty(t, changes, "the session recorded no pending change, so this case proves nothing")

	var secret, plain string
	for _, change := range changes {
		summary := model.editor.PendingChangeSummary(change)
		if strings.Contains(summary, "plaintext-password") {
			secret = summary
		}
		if strings.Contains(summary, "router-id") {
			plain = summary
		}
	}

	require.NotEmpty(t, secret, "the credential change is missing, so this case proves nothing")
	assert.NotContains(t, secret, typedCLISecretTail, "the summary published the tail of the value")
	assert.Contains(t, secret, config.SecretDataPlaceholder, "the summary wrote no placeholder")

	require.NotEmpty(t, plain, "the ordinary change is missing, so the other polarity proves nothing")
	assert.Contains(t, plain, "10.9.8.7",
		"the summary masked a leaf the schema does not mark, so its mask reads something other than the schema")
}
