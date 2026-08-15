// Related: secret.go -- the masking rule these tests hold

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/test/golden"
)

// storedSecret is the value an operator configured. No response body carries
// it. It is spelled so a substring search cannot match anything else.
const storedSecret = "api-bearer-token-98f3a1"

// storedEphemeralSecret is the value of a leaf that is BOTH ze:ephemeral and
// ze:sensitive, which is what plaintext-password is. ze:ephemeral answers "is
// this value persisted". Only ze:sensitive answers "is this value a secret". A
// display path that read ze:ephemeral would mask a future ephemeral leaf that
// holds no secret. Its paired write guard would then drop what the operator
// typed there.
const storedEphemeralSecret = "operator-typed-passphrase-5b21ce"

// replacementSecret is what the operator sets on top of storedSecret, so the
// commit review carries a previous value and a new one.
const replacementSecret = "api-bearer-token-77de20"

// secretValues is every value the fixture stores. No response body carries any
// of them.
var secretValues = []string{storedSecret, storedEphemeralSecret, replacementSecret}

// secretUser owns the editor session every render path below reads.
const secretUser = "alice"

// secretPath is where the fixture keeps that value. The path is real:
// ze-api-conf.yang marks environment/api-server/token ze:sensitive, the
// editor accepts it, and the API service page of the workbench renders it.
var secretPath = []string{"environment", "api-server"}

// secretConfigFile is the fixture as an operator writes it. The editor parses
// this file, so the tree the handlers read is the one a running daemon holds.
//
// It carries no ephemeral leaf. The editor factory builds its own editor on the
// shipped YANG schema, so that schema must accept the file. The fixture schema
// below adds the ephemeral leaf, for the render paths that take a schema as an
// argument.
const secretConfigFile = "environment {\n" +
	"    api-server {\n" +
	"        token \"" + storedSecret + "\";\n" +
	"    }\n" +
	"}\n"

// secretSchemaAndTree builds the fixture every path below renders.
//
// A container holds a ze:sensitive leaf, and a list under it holds one of its
// own. The list marks the secret leaf ze:required and ze:unique, which is what
// carries the value into the add-entry overlay and into the list table.
//
// sensitive is a parameter so a caller can unmark the leaf and watch the same
// path publish the value. That is the experiment this whole file exists for.
// The mask must follow the schema's marking and nothing else. A real YANG
// module cannot be unmarked from a test. The fixture schema stands in for one,
// and every path below is given the fixture.
func secretSchemaAndTree(sensitive bool) (*config.Schema, *config.Tree) {
	containerSecret := config.Leaf(config.TypeString)
	containerSecret.Sensitive = sensitive

	entrySecret := config.Leaf(config.TypeString)
	entrySecret.Sensitive = sensitive

	// The shape of plaintext-password: written by the operator, hashed into a
	// sibling at commit, never persisted. It stays ze:ephemeral in both runs, so
	// unmarking ze:sensitive leaves an ephemeral leaf that must publish again.
	ephemeralSecret := config.Leaf(config.TypeString)
	ephemeralSecret.Ephemeral = true
	ephemeralSecret.Sensitive = sensitive

	clients := config.List(config.TypeString,
		config.Field("token", entrySecret),
		config.Field("ip", config.Leaf(config.TypeIP)),
	)
	clients.KeyName = "name"
	clients.Required = [][]string{{"token"}}
	clients.Unique = [][]string{{"token"}, {"ip"}}

	schema := config.NewSchema()
	schema.Define("environment", config.Container(
		config.Field("api-server", config.Container(
			config.Field("token", containerSecret),
			config.Field("plaintext-token", ephemeralSecret),
			config.Field("enabled", config.Leaf(config.TypeBool)),
			config.Field("client", clients),
		)),
	))

	entry := config.NewTree()
	entry.Set("token", storedSecret)
	entry.Set("ip", "192.0.2.1")

	apiServer := config.NewTree()
	apiServer.Set("token", storedSecret)
	apiServer.Set("plaintext-token", storedEphemeralSecret)
	apiServer.Set("enabled", "true")
	apiServer.AddListEntry("client", "fleet", entry)

	environment := config.NewTree()
	environment.SetContainer("api-server", apiServer)

	tree := config.NewTree()
	tree.SetContainer("environment", environment)

	return schema, tree
}

// secretRenders is every path in this package that copies a stored leaf value
// into a response body. Each one renders the fixture above and answers the
// markup an operator receives.
//
// One component is not a population. TestWorkbenchFormNeverRendersAStoredSecret
// held this property over workbenchForm alone, and the leaf view and the
// fragment view published every ze:sensitive leaf underneath it.
func secretRenders(t *testing.T, schema *config.Schema, tree *config.Tree) map[string]string {
	t.Helper()

	renderer, err := NewRenderer()
	require.NoError(t, err)

	out := make(map[string]string)

	leafPath := []string{"environment", "api-server", "token"}
	listPath := []string{"environment", "api-server", "client"}

	leafView, err := buildConfigViewData(schema, tree, secretPath)
	require.NoError(t, err)
	out["config-view-container"] = string(renderConfigContent(renderer, leafView))

	oneLeaf, err := buildConfigViewData(schema, tree, leafPath)
	require.NoError(t, err)
	out["config-view-leaf"] = string(renderConfigContent(renderer, oneLeaf))

	out["fragment-container"] = renderComponentString(t, fullContent(buildFragmentData(schema, tree, secretPath)))
	out["fragment-list-table"] = renderComponentString(t, fullContent(buildFragmentData(schema, tree, listPath)))
	out["add-entry-form"] = addFormMarkup(t, schema, renderer)
	out["workbench-form"] = workbenchFormMarkup(t, renderer, schema, tree)
	out["cli-terminal-show"] = terminalShowOutput(t, schema, tree)
	out["cli-terminal-compare"] = terminalCompareOutput(t, schema, tree)
	out["cli-bar-show"] = cliBarShowMarkup(t, schema, renderer)
	out["commit-review"] = commitReviewMarkup(t, schema, renderer)

	return out
}

// secretEditorManager answers an editor manager whose working tree is the
// fixture config file, with a session already open for secretUser.
func secretEditorManager(t *testing.T, schema *config.Schema) *EditorManager {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(secretConfigFile), 0o600))

	mgr := NewEditorManager(storage.NewFilesystem(), configPath, schema,
		testEditorFactory(), testEditSessionFactory())
	_, err := mgr.GetOrCreate(secretUser)
	require.NoError(t, err)
	require.Equal(t, storedSecret, getConfigValue(mgr.Tree(secretUser), "environment/api-server/token"),
		"the fixture config must reach the editor tree, or every case built on it proves nothing")

	return mgr
}

// terminalShowOutput answers the whole response of the web CLI terminal's show
// verb.
//
// That verb needs no config authorization (terminalAuthCommand maps no case for
// it), so the text it returns reaches any authenticated session. It serializes
// a whole subtree, and the serializer writes what the tree holds. The parser
// decodes a $9$ value into the tree, so the tree holds cleartext.
func terminalShowOutput(t *testing.T, schema *config.Schema, tree *config.Tree) string {
	t.Helper()

	mgr := secretEditorManager(t, schema)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, nil, nil, nil)

	req := postConfigRequest(t, "/cli/terminal", url.Values{"command": {"show"}, "mode": {"config"}}, secretUser)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "terminal show: %s", rec.Body.String())

	var resp terminalResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Output, "the terminal rendered no configuration, so this case would prove nothing")

	return rec.Body.String()
}

// rotatedSecretTree answers the fixture tree with the container secret replaced,
// which is what an operator does when they rotate a credential.
func rotatedSecretTree(tree *config.Tree) *config.Tree {
	rotated := tree.Clone()
	rotated.GetContainer("environment").GetContainer("api-server").Set("token", replacementSecret)
	return rotated
}

// terminalCompareOutput answers what the compare verb prints after a rotation.
//
// Both sides are masked, so the text diff reads one placeholder against the same
// placeholder and sees nothing. The verb needs no config authorization, so it
// must name the leaf and publish neither value.
func terminalCompareOutput(t *testing.T, schema *config.Schema, tree *config.Tree) string {
	t.Helper()

	out := compareTreesAtPath(tree, rotatedSecretTree(tree), schema, nil, "")
	require.NotEqual(t, terminalOutputNoChanges, out,
		"compare reported no change at all, so this case would prove nothing")

	return out
}

// cliBarShowMarkup answers the content area the CLI bar's show verb writes.
// It is the second web surface over the same configuration text, and it
// serialized through the editor rather than through the terminal's helper.
func cliBarShowMarkup(t *testing.T, schema *config.Schema, renderer *Renderer) string {
	t.Helper()

	mgr := secretEditorManager(t, schema)
	handler := HandleCLICommandWithAuthorizer(mgr, schema, renderer, nil)

	req := postConfigRequest(t, "/cli", url.Values{"command": {"show"}}, secretUser)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "cli bar show: %s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "api-server",
		"the CLI bar rendered no configuration, so this case would prove nothing")

	return rec.Body.String()
}

// commitReviewMarkup answers the commit page for a session that rotated the
// secret.
//
// The page shows what the change replaced. A pending change carries the previous
// value beside the new one. The review therefore discloses the secret the
// operator rotated, and the one they typed.
func commitReviewMarkup(t *testing.T, schema *config.Schema, renderer *Renderer) string {
	t.Helper()

	mgr := secretEditorManager(t, schema)
	// test-relax: the ephemeral leaf is not set here. The editor parses against
	// the shipped YANG schema, which holds no ephemeral leaf under this path, so
	// the set would fail on the fixture rather than prove anything. The ephemeral
	// leaf is covered by the render paths that take the fixture schema.
	require.NoError(t, mgr.SetValue(secretUser, secretPath, "token", replacementSecret))

	handler := HandleConfigCommitWithAuthorizerAndAudit(mgr, renderer, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/config/commit/", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), ctxKeyUsername, secretUser))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "commit page: %s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "environment api-server token",
		"the commit page listed no change, so this case would prove nothing")

	return rec.Body.String()
}

// renderComponentString renders one component and answers its markup.
func renderComponentString(t *testing.T, c templ.Component) string {
	t.Helper()

	var buf strings.Builder
	require.NoError(t, c.Render(context.Background(), &buf))

	return buf.String()
}

// addFormMarkup answers the add-entry overlay for the client list.
//
// The overlay prefills a ze:required field with the value the parent container
// holds, so the operator does not retype an inherited setting. For a secret
// that prefill is the leak: resolveInheritedValue reads the stored value.
//
// The editor manager parses a real config file, so the tree the handler reads
// is the one a running daemon holds. The fixture schema walks the path, which
// is how a list marked ze:required reaches this handler at all.
func addFormMarkup(t *testing.T, schema *config.Schema, renderer *Renderer) string {
	t.Helper()

	// test-relax: the file write and the two assertions moved into
	// secretEditorManager unchanged, because three render paths now need the same
	// editor session. Nothing was dropped. The helper asserts it for all of them.
	mgr := secretEditorManager(t, schema)

	req := httptest.NewRequest(http.MethodGet, "/config/add-form/environment/api-server/client/", http.NoBody)
	req = req.WithContext(context.WithValue(req.Context(), ctxKeyUsername, secretUser))
	rec := httptest.NewRecorder()
	HandleConfigAddForm(mgr, schema, renderer).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "add form: %s", rec.Body.String())

	return rec.Body.String()
}

// workbenchFormMarkup answers the API configuration form the workbench serves.
//
// The purpose-built pages build their own view data out of tree reads. The
// leaf node is gone by the time a value reaches a field. renderPageContent
// masks the tree they read instead. buildAPIFormData (page_services.go) reads
// environment/api-server/token out of that tree.
func workbenchFormMarkup(t *testing.T, renderer *Renderer, schema *config.Schema, tree *config.Tree) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/show/api/", http.NoBody)
	content, handled := renderPageContent(renderer, req, []string{segAPI}, tree, schema, nil, nil, nil)
	require.True(t, handled, "the workbench must serve /show/api/ as a purpose-built page")
	require.NotEmpty(t, content, "the API page rendered nothing, so this case would prove nothing")

	return string(content)
}

// TestNoRenderPathEmitsAStoredSecret verifies no path in this package writes a
// stored secret into a response body.
//
// VALIDATES: every render path masks a ze:sensitive value.
// PREVENTS: a secret in the page source, and from there in view-source, the
// browser cache and any proxy that reads the document. The mask read
// LeafNode.Bcrypt alone, so environment/api-server/token, l2tp/shared-secret
// and the IPsec pre-shared secret rendered in plaintext. A read-only session
// reaches those views.
func TestNoRenderPathEmitsAStoredSecret(t *testing.T) {
	schema, tree := secretSchemaAndTree(true)

	for name, markup := range secretRenders(t, schema, tree) {
		t.Run(name, func(t *testing.T) {
			for _, value := range secretValues {
				assert.NotContains(t, markup, value, "a stored secret reached the response body")
			}
			assert.Contains(t, markup, config.SecretDataPlaceholder,
				"the field must render the display placeholder, or this case rendered no value at all")
		})
	}
}

// TestSecretMaskingFollowsTheSchemaMarking verifies the mask reads the schema
// and nothing else.
//
// VALIDATES: an unmarked leaf renders its value, and every path above owes its
// silence to ze:sensitive.
// PREVENTS: a guard that looks green because the value was absent, a name
// matched, or a component happened to type the field as a password. Unmark the
// leaf and every case in the test above must publish the value again.
func TestSecretMaskingFollowsTheSchemaMarking(t *testing.T) {
	schema, tree := secretSchemaAndTree(false)

	renders := secretRenders(t, schema, tree)

	for name, markup := range renders {
		t.Run(name, func(t *testing.T) {
			assert.Contains(t, markup, storedSecret,
				"an unmarked leaf must render its value, or this path masks something other than ze:sensitive")
		})
	}

	var all strings.Builder
	for _, markup := range renders {
		all.WriteString(markup)
	}

	assert.Contains(t, all.String(), storedEphemeralSecret,
		"an ephemeral leaf that is not ze:sensitive must render its value, or the mask reads ze:ephemeral")
}

// TestCompareNamesARotatedSecretAndPublishesNeitherValue verifies the compare
// verb reports a changed secret leaf by path.
//
// VALIDATES: compare answers a change when a secret moved, at the root and at a
// context path, and carries neither the old value nor the new one.
// PREVENTS: the mask making compare state something false. compareTreesAtPath
// diffs two masked texts, so a rotated ze:sensitive leaf reads as the same
// placeholder on each side and the verb answered "(no changes)". Before the mask
// the change was visible, and leaking.
func TestCompareNamesARotatedSecretAndPublishesNeitherValue(t *testing.T) {
	schema, tree := secretSchemaAndTree(true)
	rotated := rotatedSecretTree(tree)

	require.Equal(t, serializeTreeAtPath(tree, schema, nil), serializeTreeAtPath(rotated, schema, nil),
		"the mask must hide the rotation from the text, or this test proves nothing")

	out := compareTreesAtPath(tree, rotated, schema, nil, "")
	assert.Contains(t, out, "environment.api-server.token", "compare must name the leaf that changed")
	assert.Contains(t, out, config.SecretDataPlaceholder, "the value must read as the display placeholder")
	for _, value := range secretValues {
		assert.NotContains(t, out, value, "compare published a stored secret")
	}

	sub := compareTreesAtPath(tree, rotated, schema, []string{"environment", "api-server"}, "")
	assert.Contains(t, sub, "~ token ", "a compare at a context path names the leaf under that path")
	assert.NotContains(t, sub, storedSecret)
	assert.NotContains(t, sub, replacementSecret)

	assert.Equal(t, terminalOutputNoChanges, compareTreesAtPath(tree, tree.Clone(), schema, nil, ""),
		"an unchanged tree must still answer no changes")
}

// TestContentAtPathAnswersNothingWhenItCannotMask verifies the CLI-bar show
// path fails closed.
//
// VALIDATES: a manager with no schema answers nothing rather than raw text.
// PREVENTS: a fail-open fallback. EditorManager.ContentAtPath called
// Editor.DisplayContentAtPath here, and that function's first branch returns
// e.workingContent with no mask at all (internal/component/cli/editor_mask.go).
// It was unreachable only because newEditor substitutes an empty tree on a parse
// failure, which is an invariant three files away.
func TestContentAtPathAnswersNothingWhenItCannotMask(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(secretConfigFile), 0o600))

	mgr := NewEditorManager(storage.NewFilesystem(), configPath, nil,
		testEditorFactory(), testEditSessionFactory())
	_, err := mgr.GetOrCreate(secretUser)
	require.NoError(t, err)

	out := mgr.ContentAtPath(secretUser, nil)
	assert.NotContains(t, out, storedSecret, "a display path with no schema published a stored secret")
	assert.Empty(t, out, "no schema means no mask, so this path must answer nothing")
}

// TestAnUntouchedSecretIsNeitherRewrittenNorDeleted verifies the write half of
// the placeholder pair over a ze:sensitive leaf.
//
// VALIDATES: posting the placeholder leaves the stored value alone, an empty
// field deletes the leaf, and a new value is written.
// PREVENTS: the mask destroying what it protects. An empty form value is a
// DELETE on this path, and a posted placeholder would otherwise be stored as
// the secret itself.
func TestAnUntouchedSecretIsNeitherRewrittenNorDeleted(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0o600))

	mgr := NewEditorManager(storage.NewFilesystem(), configPath, schema,
		testEditorFactory(), testEditSessionFactory())
	renderer, err := NewRenderer()
	require.NoError(t, err)

	handler := HandleConfigSetWithAuthorizer(mgr, schema, renderer, nil)

	set := func(t *testing.T, value string) {
		t.Helper()

		req := postConfigRequest(t, "/config/set/environment/api-server/", map[string][]string{
			"leaf":  {"token"},
			"value": {value},
		}, "alice")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusSeeOther, rec.Code, "set %q: %s", value, rec.Body.String())
	}

	set(t, storedSecret)
	require.Equal(t, storedSecret, getConfigValue(mgr.Tree("alice"), "environment/api-server/token"),
		"the first save must store the value")

	set(t, config.SecretDataPlaceholder)
	assert.Equal(t, storedSecret, getConfigValue(mgr.Tree("alice"), "environment/api-server/token"),
		"posting the placeholder must leave the stored secret alone")

	set(t, "a-new-secret-4471bc")
	assert.Equal(t, "a-new-secret-4471bc", getConfigValue(mgr.Tree("alice"), "environment/api-server/token"),
		"a new value must be written")

	set(t, "")
	assert.Empty(t, getConfigValue(mgr.Tree("alice"), "environment/api-server/token"),
		"clearing the field must delete the leaf")
}

// plaintextLeafPattern finds the head of a write-only credential leaf in a YANG
// module.
var plaintextLeafPattern = regexp.MustCompile(`leaf\s+(plaintext-[a-z0-9-]+)\s*\{`)

// TestAWriteOnlyPasswordLeafIsMarkedSensitive verifies every plaintext-<name>
// leaf in the shipped YANG carries ze:sensitive.
//
// VALIDATES: the schema says the value is a secret, so config.LeafHoldsSecret
// answers true and every display path masks it.
// PREVENTS: a cleartext password in a response body. plaintext-password carried
// ze:ephemeral alone. That extension says the value is not written to the config
// file. It says nothing about secrecy, so the telemetry form and the generic
// leaf view published what the operator typed. plaintext- is this repository's
// name for an operator-typed credential already. internal/core/redact/redact.go
// redacts the prefix from a command line, and the editor commit path hashes the
// leaf into its canonical sibling.
//
// It reads the YANG sources rather than config.YANGSchema(). A schema built in
// this test binary holds the modules this binary links. The two modules that own
// these leaves are not among them, so a registry walk is silently empty.
//
// It lives in this package because this package owns the render paths the
// marking protects.
func TestAWriteOnlyPasswordLeafIsMarkedSensitive(t *testing.T) {
	root := golden.RepoFile(t, "internal")

	modules, seen := 0, 0

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".yang") {
			return nil
		}

		body, readErr := os.ReadFile(path) //nolint:gosec // a test reading the repository's own modules
		if readErr != nil {
			return readErr
		}

		modules++
		source := string(body)

		for _, match := range plaintextLeafPattern.FindAllStringSubmatchIndex(source, -1) {
			seen++

			// The description is dropped before the check. A description that
			// names the extension would otherwise satisfy the check for it.
			block := withoutQuoted(balancedBlockAt(source, match[0]))
			assert.Contains(t, block, "ze:sensitive;",
				"%s: leaf %s holds an operator's cleartext credential and is not marked ze:sensitive",
				path, source[match[2]:match[3]])
		}

		return nil
	})
	require.NoError(t, err)
	require.Positive(t, modules, "no YANG module was read; the walk root is wrong")
	require.Positive(t, seen, "no plaintext- leaf was checked; the pattern has stopped matching")
}

// TestUploadedConfigWithAMaskedSecretIsRefused verifies the write half of the
// placeholder pair over a ze:sensitive leaf, at the upload entry point.
//
// VALIDATES: a config whose ze:sensitive leaf holds the display placeholder is
// refused, and the stored configuration is left alone.
// PREVENTS: a dump-then-upload round trip destroying a secret. `ze config dump
// --strip` writes the placeholder for every ze:sensitive leaf. The guard keyed
// on ze:bcrypt, so uploading that dump stored the literal placeholder as the
// API bearer token, and the daemon then ran with it.
func TestUploadedConfigWithAMaskedSecretIsRefused(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")
	original := "environment {\n    api-server {\n        token \"" + storedSecret + "\";\n    }\n}\n"
	require.NoError(t, os.WriteFile(configPath, []byte(original), 0o600))

	mgr := NewEditorManager(storage.NewFilesystem(), configPath, schema,
		testEditorFactory(), testEditSessionFactory())
	handler := HandleConfigUpload(mgr, webGoldenValidate, configPath, adminWebAuthorizer(), nil)

	masked := "environment {\n    api-server {\n        token \"" + config.SecretDataPlaceholder + "\";\n    }\n}\n"
	req := postConfigRequest(t, "/config/upload", url.Values{"config": {masked}}, secretUser)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "the upload must be refused")
	assert.Contains(t, rec.Body.String(), config.SecretDataPlaceholder,
		"the error must name the placeholder it refused")

	stored, err := os.ReadFile(configPath) //nolint:gosec // the test wrote this path
	require.NoError(t, err)
	assert.Equal(t, original, string(stored), "a refused upload must leave the stored configuration alone")
}
