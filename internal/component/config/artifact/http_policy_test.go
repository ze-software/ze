// Design: docs/architecture/web-interface.md -- native HTTP policy over one server-owned artifact owner.
package artifact_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	editor "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/artifact"
	configcli "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/component/doctor" // Registers the actual runtime recent-events check.
	"github.com/ze-software/ze/internal/component/plugin"
	_ "github.com/ze-software/ze/internal/component/plugin/all" // Match the shipped command and schema composition.
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/zefs"
)

const policyConfig = "bgp {\n\trouter-id 1.2.3.4\n\tsession { asn { local 65000; } }\n}\n"
const policyWorkspace = "/backups/"

type policyGrant struct{ edit bool }

func (grant policyGrant) Authorize(_, _, _ string, readOnly bool) bool {
	if readOnly {
		return true
	}
	return grant.edit
}

type policyAuthentication struct{ grant policyGrant }

func (authentication policyAuthentication) Authenticate(authz.AuthRequest) (authz.AuthResult, error) {
	return authz.AuthResult{Authenticated: true, Authorizer: authentication.grant}, nil
}

type policyHTTP struct {
	owner    *artifact.Owner
	server   *pluginserver.Server
	handler  http.Handler
	dispatch web.CommandDispatcher
	source   []byte
}

// newPolicyHTTP retains the original artifact inputs and binds the faultable
// sole owner to the real registration, dispatcher and authentication adapters.
func newPolicyHTTP(t *testing.T, owner *artifact.Owner, readOnly bool) policyHTTP {
	t.Helper()
	if owner == nil {
		owner = artifact.New()
		owner.FixtureParent(t.TempDir())
		t.Cleanup(func() { require.NoError(t, owner.Close()) })
	}
	resolved, err := configyang.DefaultLoader()
	require.NoError(t, err)
	dispatcher := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(dispatcher, configyang.WireMethodToPath(resolved), configyang.PathToDescription(resolved), configyang.PathToHelp(resolved), configyang.PathToArgDefs(resolved), configyang.BuildCommandTree(resolved)))
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	require.NoError(t, configcli.AttachArtifactOwner(server, owner))
	dispatch := func(ctx context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
		return dispatcher.Dispatch(&pluginserver.CommandContext{Server: server, Username: caller.Username,
			Authorizer: caller.Authorizer, RequestContext: ctx, Surface: "web", Sender: plugin.OperatorSender()}, line)
	}
	grant := policyGrant{edit: !readOnly}
	handler := web.HandleBackupWorkspace(web.RouteDeps{Dispatch: dispatch, Authorizer: grant})
	authenticated := web.AuthMiddlewareWithAudit(web.NewSessionStore(nil), policyAuthentication{grant: grant},
		func(http.ResponseWriter, *http.Request) {}, handler, nil)
	path := filepath.Join(t.TempDir(), "original.zefs")
	store, err := storage.CreateBlobPopulated(path, func(seed storage.Storage) error {
		if err := seed.WriteFile("router.conf", []byte(policyConfig), 0o600); err != nil {
			return err
		}
		if err := seed.WriteFile("other.conf", []byte(policyConfig), 0o600); err != nil {
			return err
		}
		if err := seed.WriteKey("file/draft/router.conf", []byte("preserved original draft")); err != nil {
			return err
		}
		if err := seed.WriteFile(editor.DraftPath("router.conf"), []byte("set bgp router-id 192.0.2.99\n"), 0o600); err != nil {
			return err
		}
		if err := seed.WriteFile(editor.ChangePath("router.conf", "alice"), []byte("set bgp router-id 198.51.100.80\n"), 0o600); err != nil {
			return err
		}
		if err := seed.WriteKey(zefs.KeyLocalAdminPassword.Pattern, []byte("credential-to-preserve")); err != nil {
			return err
		}
		return seed.WriteKey("unknown/binary", []byte{0, 255, 128, 0, 1})
	}, false, zefs.Spare(0))
	require.NoError(t, err)
	require.NoError(t, store.Close())
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return policyHTTP{owner: owner, server: server, handler: authenticated, dispatch: dispatch, source: content}
}

func (fixture policyHTTP) request(t *testing.T, path, user string, body io.Reader, contentType, origin string) *httptest.ResponseRecorder {
	t.Helper()
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	} else {
		body = http.NoBody
	}
	request := httptest.NewRequestWithContext(t.Context(), method, "https://example.com"+path, body)
	request.Header.Set("Origin", origin)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if user != "" {
		request.SetBasicAuth(user, "fixture-password")
	}
	response := httptest.NewRecorder()
	fixture.handler.ServeHTTP(response, request)
	return response
}

func (fixture policyHTTP) upload(t *testing.T, content []byte, user, origin string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("backup", "original.zefs")
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, form.Close())
	return fixture.request(t, policyWorkspace, user, &body, form.FormDataContentType(), origin)
}

func (fixture policyHTTP) open(t *testing.T, path, user string) *httptest.ResponseRecorder {
	t.Helper()
	return fixture.request(t, path, user, nil, "", "https://example.com")
}

func policyID(path string) string {
	return strings.Trim(strings.TrimPrefix(path, policyWorkspace), "/")
}

// TestBackupHTTPRemainingBudgetBoundsUpload retains the exact one-byte budget
// boundary through native multipart upload and its HTTP 413 answer.
func TestBackupHTTPRemainingBudgetBoundsUpload(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	fixture.owner.FixtureRetainedBytes(artifact.OwnerBytesMax - int64(len(fixture.source)) + 1)
	response := fixture.upload(t, fixture.source, "alice", "https://example.com")
	assert.Equal(t, http.StatusRequestEntityTooLarge, response.Code, response.Body.String())
	assert.Empty(t, fixture.owner.FixtureState().Workspaces)
}

// TestBackupHTTPFailedUploadCleanupSealsAdmission preserves the bounded debt,
// HTTP 500/503 outcomes, usable committed form and eventual admission recovery.
func TestBackupHTTPFailedUploadCleanupSealsAdmission(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	upload := fixture.upload(t, fixture.source, "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, upload.Code)
	validURL := upload.Header().Get("Location")
	validDirectory := filepath.Dir(fixture.owner.FixtureState().Workspaces[policyID(validURL)])
	blocked := ""
	fixture.owner.FixtureRemoval(func(path string) error {
		if path == validDirectory {
			return os.RemoveAll(path)
		}
		if blocked == "" {
			blocked = path
		}
		if path == blocked {
			return errors.New("injected partial-upload removal failure")
		}
		return os.RemoveAll(path)
	})
	failed := fixture.upload(t, []byte("corrupt artifact"), "alice", "https://example.com")
	require.Equal(t, http.StatusInternalServerError, failed.Code, failed.Body.String())
	state := fixture.owner.FixtureState()
	require.Equal(t, blocked, state.Debt)
	require.Positive(t, state.DebtBytes)
	entries, err := os.ReadDir(state.Root)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	refused := fixture.upload(t, fixture.source, "alice", "https://example.com")
	assert.Equal(t, http.StatusServiceUnavailable, refused.Code, refused.Body.String())
	state = fixture.owner.FixtureState()
	assert.Len(t, state.Workspaces, 1)
	entries, err = os.ReadDir(state.Root)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "repeated refusals must not allocate private directories")
	opened := fixture.open(t, validURL, "alice")
	assert.Equal(t, http.StatusOK, opened.Code, opened.Body.String())
	assert.Contains(t, opened.Body.String(), "1.2.3.4", "the committed configuration form remains usable during debt")
	fixture.owner.FixtureRemoval(os.RemoveAll)
	retry := fixture.upload(t, fixture.source, "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, retry.Code, retry.Body.String())
	state = fixture.owner.FixtureState()
	assert.Empty(t, state.Debt)
	assert.Zero(t, state.DebtBytes)
	_, err = os.Stat(blocked)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// TestBackupHTTPFailedSnapshotCleanupBoundsAllOwners retains a complete first
// download, exact other-account refusal, and inspection/recovery during debt.
func TestBackupHTTPFailedSnapshotCleanupBoundsAllOwners(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	first := fixture.upload(t, fixture.source, "alice", "https://example.com")
	second := fixture.upload(t, fixture.source, "bob", "https://example.com")
	require.Equal(t, http.StatusSeeOther, first.Code)
	require.Equal(t, http.StatusSeeOther, second.Code)
	firstURL, secondURL := first.Header().Get("Location"), second.Header().Get("Location")
	fixture.owner.FixtureRemoval(func(path string) error {
		if _, err := os.Stat(filepath.Join(path, "snapshot")); err == nil {
			return errors.New("injected snapshot cleanup failure")
		}
		return os.RemoveAll(path)
	})
	download := fixture.open(t, firstURL+"download", "alice")
	assert.Equal(t, http.StatusOK, download.Code, download.Body.String())
	assert.NotEmpty(t, download.Body.Bytes())
	state := fixture.owner.FixtureState()
	require.NotEmpty(t, state.Debt)
	require.Positive(t, state.DebtBytes)
	before, err := os.ReadDir(state.Root)
	require.NoError(t, err)
	require.Len(t, before, 3)
	refused := fixture.open(t, secondURL+"download", "bob")
	assert.Equal(t, http.StatusInternalServerError, refused.Code, refused.Body.String())
	after, err := os.ReadDir(state.Root)
	require.NoError(t, err)
	assert.Equal(t, policyEntryNames(before), policyEntryNames(after), "debt must refuse before allocating any other snapshot directory")
	opened := fixture.open(t, secondURL, "bob")
	assert.Equal(t, http.StatusOK, opened.Code, opened.Body.String())
	assert.Contains(t, opened.Body.String(), "1.2.3.4")
	fixture.owner.FixtureRemoval(os.RemoveAll)
	assert.Equal(t, http.StatusOK, fixture.open(t, secondURL+"download", "bob").Code)
	state = fixture.owner.FixtureState()
	assert.Empty(t, state.Debt)
	assert.Zero(t, state.DebtBytes)
}

func policyEntryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for index, entry := range entries {
		names[index] = entry.Name()
	}
	return names
}

// TestBackupHTTPStorageFailureReachesDoctor drives a real failed directory
// creation through HTTP and retains initialized, private-safe doctor metadata.
func TestBackupHTTPStorageFailureReachesDoctor(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	diagnostic.RegisterBuiltinCodes()
	require.NotNil(t, diagnostic.Lookup(diagnostic.CodeDoctorBackupWorkspaceStorage))
	fixture := newPolicyHTTP(t, nil, false)
	fixture.owner.FixtureParent(filepath.Join(t.TempDir(), "missing-parent"))
	response := fixture.upload(t, fixture.source, "private-account", "https://example.com")
	assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	require.Contains(t, diagnostic.EventCodes(), diagnostic.CodeDoctorBackupWorkspaceStorage)
	events := report.Errors(0)
	require.Len(t, events, 1)
	assert.Equal(t, diagnostic.CodeDoctorBackupWorkspaceStorage, events[0].Code)
	assert.NotContains(t, events[0].Message, "private-account")
	assert.NotContains(t, events[0].Message, "original.zefs")
	assert.NotEmpty(t, events[0].Fix)
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name != "recent-events" {
			continue
		}
		require.Contains(t, check.Codes, diagnostic.CodeDoctorBackupWorkspaceStorage)
		rows := check.Check(diagnostic.DoctorCheckContext{})
		require.Len(t, rows, 1)
		assert.Equal(t, diagnostic.CodeDoctorBackupWorkspaceStorage, rows[0].Code)
		assert.Equal(t, events[0].Fix, rows[0].Fix)
		assert.Equal(t, diagnostic.SeverityWarning, rows[0].Severity)
		assert.Contains(t, rows[0].Message, events[0].Message)
		assert.Contains(t, rows[0].Message, events[0].Source)
		assert.Contains(t, rows[0].Message, events[0].Subject)
		assert.Contains(t, rows[0].Message, "1 time, last at "+events[0].Updated.UTC().Format(time.RFC3339))
		assert.NotContains(t, rows[0].Message, "private-account")
		assert.NotContains(t, rows[0].Message, "original.zefs")
		return
	}
	t.Fatal("the runtime recent-events check must report the actual HTTP storage failure")
}

// TestBackupHTTPExpiryAndClose retains exact expired/closed HTTP answers and
// proves only the private owner's files are removed.
func TestBackupHTTPExpiryAndClose(t *testing.T) {
	fixture := newPolicyHTTP(t, nil, false)
	now := time.Now()
	fixture.owner.FixtureClock(now)
	upload := fixture.upload(t, fixture.source, "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, upload.Code)
	state := fixture.owner.FixtureState()
	sentinel := filepath.Join(state.Parent, "unrelated")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0o600))
	fixture.owner.FixtureClock(now.Add(artifact.Idle + time.Second))
	assert.Equal(t, http.StatusNotFound, fixture.open(t, upload.Header().Get("Location"), "alice").Code)
	state = fixture.owner.FixtureState()
	assert.Empty(t, state.Workspaces)
	assert.Zero(t, state.Bytes)
	require.NoError(t, fixture.owner.Close())
	_, err := os.Stat(state.Root)
	require.ErrorIs(t, err, os.ErrNotExist)
	kept, err := os.ReadFile(sentinel)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(kept))
	assert.Equal(t, http.StatusGone, fixture.open(t, policyWorkspace, "alice").Code)
}

// TestBackupHTTPCloseFailureReportsPossibleRetention retains the actual cause,
// private bytes, repeated failure and exact HTTP closed-admission response.
func TestBackupHTTPCloseFailureReportsPossibleRetention(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	owner := artifact.New()
	owner.FixtureParent(t.TempDir())
	root, err := owner.FixtureRoot()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(root)) })
	path := filepath.Join(root, "retained.zefs")
	require.NoError(t, os.WriteFile(path, []byte("private task-owned bytes"), 0o600))
	fixture := newPolicyHTTP(t, owner, false)
	failure := errors.New("injected private directory cleanup failure")
	owner.FixtureRemoval(func(string) error { return failure })
	err = owner.Close()
	require.ErrorIs(t, err, failure)
	assert.Contains(t, err.Error(), "files may remain")
	assert.Contains(t, err.Error(), "Run ze show doctor")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "private task-owned bytes", string(content))
	assert.Equal(t, http.StatusGone, fixture.open(t, policyWorkspace, "alice").Code)
	require.ErrorIs(t, owner.Close(), failure)
	events := report.Errors(0)
	require.Len(t, events, 1)
	assert.Equal(t, diagnostic.CodeDoctorBackupWorkspaceStorage, events[0].Code)
}

// TestBackupHTTPRefusals proves every original authorization/origin/integrity
// refusal leaves no published workspace, retained bytes, transfer or partial file.
func TestBackupHTTPRefusals(t *testing.T) {
	for _, test := range []struct {
		name, user, origin string
		corrupt, readOnly  bool
		status             int
	}{
		{"anonymous", "", "https://example.com", false, false, http.StatusUnauthorized},
		{"read-only", "alice", "https://example.com", false, true, http.StatusForbidden},
		{"cross-origin", "alice", "https://other.example", false, false, http.StatusForbidden},
		{"corrupt", "alice", "https://example.com", true, false, http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPolicyHTTP(t, nil, test.readOnly)
			source := fixture.source
			if test.corrupt {
				source = []byte("this is not a Ze backup")
			}
			response := fixture.upload(t, source, test.user, test.origin)
			assert.Equal(t, test.status, response.Code, response.Body.String())
			state := fixture.owner.FixtureState()
			assert.Empty(t, state.Workspaces)
			assert.Zero(t, state.Bytes)
			assert.Zero(t, state.Uploads)
			assert.Zero(t, state.Sources)
			assert.Zero(t, state.Views)
			if state.Root != "" {
				entries, err := os.ReadDir(state.Root)
				require.NoError(t, err)
				assert.Empty(t, entries)
			}
		})
	}
}
