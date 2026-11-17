//go:build ze_web

// Design: docs/architecture/web-interface.md -- authenticated candidate import wiring.

package hub

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config/storage"
	zeweb "github.com/ze-software/ze/internal/component/web"
)

type graphicalImportServer struct {
	base    string
	client  *http.Client
	store   storage.Storage
	editors *zeweb.EditorManager
	admin   *http.Cookie
	reader  *http.Cookie
}

// newGraphicalImportServer starts the production hub router and real local
// login. Cleanup joins its HTTP/history owners before closing the borrowed store.
func newGraphicalImportServer(t *testing.T) *graphicalImportServer {
	t.Helper()
	setAPIConfigDir(t, t.TempDir())
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile("config.conf", []byte("system { host import-original; }\n"), 0o600))
	users := []authz.UserConfig{
		bcryptUser(t, "import-admin", "synthetic", "import-admin"),
		bcryptUser(t, "import-reader", "synthetic", "import-reader"),
	}
	policy := authz.NewStore()
	policy.AddProfile(authz.Profile{Name: "import-admin", Run: authz.Section{Default: authz.Allow}, Edit: authz.Section{Default: authz.Allow}})
	policy.AddProfile(authz.Profile{Name: "import-reader", Run: authz.Section{Default: authz.Allow}, Edit: authz.Section{Default: authz.Deny}})
	policy.AssignProfiles("import-admin", []string{"import-admin"})
	policy.AssignProfiles("import-reader", []string{"import-reader"})
	live := func() ([]authz.UserConfig, error) { return users, nil }
	server, broker, editors := startWebServer(store, "config.conf", []string{"127.0.0.1:0"}, false, "", nil, nil, authz.StoreAuthorizer{Store: policy}, nil, nil, users[:1], live, nil, nil, nil)
	require.NotNil(t, server)
	service := newWebService(server, broker, editors)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		require.NoError(t, service.Shutdown(ctx))
	})
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	require.NoError(t, server.WaitReady(ctx))
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // test-owned self-signed loopback server
	t.Cleanup(transport.CloseIdleConnections)
	s := &graphicalImportServer{
		base: "https://" + server.Address(), store: store, editors: editors,
		client: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
	s.admin = s.login(t, "import-admin")
	s.reader = s.login(t, "import-reader")
	return s
}

func (s *graphicalImportServer) login(t *testing.T, username string) *http.Cookie {
	t.Helper()
	form := url.Values{"username": {username}, "password": {"synthetic"}}
	response, _ := s.request(t, http.MethodPost, "/login", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", nil, s.base)
	require.Equal(t, http.StatusSeeOther, response.StatusCode)
	for _, cookie := range response.Cookies() {
		if cookie.Name == "ze-session" {
			page, body := s.request(t, http.MethodGet, "/show/", http.NoBody, "", cookie, s.base)
			require.Equal(t, http.StatusOK, page.StatusCode, body)
			return cookie
		}
	}
	t.Fatal("successful real login returned no session cookie")
	return nil
}

func (s *graphicalImportServer) request(t *testing.T, method, path string, body io.Reader, contentType string, cookie *http.Cookie, origin string) (*http.Response, string) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, s.base+path, body)
	require.NoError(t, err)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Origin", origin)
	if path != "/login" {
		request.Header.Set("HX-Request", "true")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response, err := s.client.Do(request)
	require.NoError(t, err)
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	return response, string(data)
}

func (s *graphicalImportServer) paste(t *testing.T, fields url.Values) (*http.Response, string) {
	t.Helper()
	return s.request(t, http.MethodPost, "/config/import", strings.NewReader(fields.Encode()), "application/x-www-form-urlencoded", s.admin, s.base)
}

func (s *graphicalImportServer) committed(t *testing.T) []byte {
	t.Helper()
	data, err := s.store.ReadFile("config.conf")
	require.NoError(t, err)
	return data
}

// TestGraphicalImportPasteStagesUntilSeparateCommit drives login, import and
// explicit shared commit over the actual HTTPS routes, reading each owner back.
func TestGraphicalImportPasteStagesUntilSeparateCommit(t *testing.T) {
	s := newGraphicalImportServer(t)
	before := s.committed(t)
	response, body := s.paste(t, url.Values{"source": {"terminal"}, "location": {"absolute"}, "action": {"merge"}, "path": {""}, "content": {"system { host import-pasted; }"}})
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	assert.Contains(t, s.editors.ContentAtPath("import-admin", []string{"system"}), "import-pasted")
	assert.Contains(t, s.editors.ContentAtPath("import-reader", []string{"system"}), "import-original")
	assert.Equal(t, before, s.committed(t))
	assert.Contains(t, body, `id="commit-bar"`)
	assert.Contains(t, body, "import-pasted")
	assert.Equal(t, "/show/", response.Header.Get("HX-Push-Url"))
	form := url.Values{"action": {"now"}, "path": {""}}
	accepted, output := s.request(t, http.MethodPost, "/config/action", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", s.admin, s.base)
	require.Equal(t, http.StatusOK, accepted.StatusCode, output)
	assert.Contains(t, string(s.committed(t)), "import-pasted")
	assert.Zero(t, s.editors.ChangeCount("import-admin"))
}

// TestGraphicalImportFileUsesSharedScopedReplace checks multipart bytes against
// the shared editor load producer, preserving a slash key and unrelated edits.
func TestGraphicalImportFileUsesSharedScopedReplace(t *testing.T) {
	s := newGraphicalImportServer(t)
	before := s.committed(t)
	path := []string{"bgp", "group", "selected/key"}
	for _, owner := range []string{"import-admin", "import-oracle"} {
		require.NoError(t, s.editors.SetValue(owner, path, "description", "old-description"))
		require.NoError(t, s.editors.SetValue(owner, []string{"bgp", "group", "selected/key", "session", "asn"}, "remote", "65002"))
		require.NoError(t, s.editors.SetValue(owner, []string{"bgp", "group", "retained"}, "description", "unrelated-change"))
	}
	content := "description imported-file;"
	_, err := s.editors.ConfigCommand("import-oracle", contract.ConfigCommandRequest{Line: `load file relative replace "candidate one.conf"`, Path: path, Content: &content})
	require.NoError(t, err)
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	for name, value := range map[string]string{"source": "file", "location": "relative", "action": "replace", "path": "bgp/group/selected%2Fkey"} {
		require.NoError(t, writer.WriteField(name, value))
	}
	file, err := writer.CreateFormFile("file", "candidate one.conf")
	require.NoError(t, err)
	_, err = io.WriteString(file, content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	response, body := s.request(t, http.MethodPost, "/config/import", &form, writer.FormDataContentType(), s.admin, s.base)
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	assert.Equal(t, s.editors.ContentAtPath("import-oracle", path), s.editors.ContentAtPath("import-admin", path))
	assert.Contains(t, s.editors.ContentAtPath("import-admin", path), "imported-file")
	assert.NotContains(t, s.editors.ContentAtPath("import-admin", path), "65002")
	assert.Contains(t, s.editors.ContentAtPath("import-admin", []string{"bgp", "group", "retained"}), "unrelated-change")
	assert.Equal(t, before, s.committed(t))
	assert.Equal(t, "/show/bgp/group/selected%2Fkey/", response.Header.Get("HX-Push-Url"))
	assert.Contains(t, body, "imported-file")
}

// TestGraphicalImportAbsoluteReplaceReturnsSurvivingNavigation removes the
// currently selected draft entry without leaving its old edit forms behind.
func TestGraphicalImportAbsoluteReplaceReturnsSurvivingNavigation(t *testing.T) {
	s := newGraphicalImportServer(t)
	require.NoError(t, s.editors.SetValue("import-admin", []string{"bgp", "group", "removed/key"}, "description", "removed-draft"))
	before := s.committed(t)
	response, body := s.paste(t, url.Values{"source": {"terminal"}, "location": {"absolute"}, "action": {"replace"}, "path": {"bgp/group/removed%2Fkey"}, "content": {"system { host root-replacement; }"}})
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	assert.Contains(t, s.editors.ContentAtPath("import-admin", []string{"system"}), "root-replacement")
	assert.NotContains(t, s.editors.ContentAtPath("import-admin", nil), "removed-draft")
	assert.Equal(t, "/show/bgp/group/", response.Header.Get("HX-Push-Url"))
	assert.NotContains(t, body, `id="graphical-entry-target"`)
	assert.Contains(t, body, `id="commit-bar"`)
	assert.Equal(t, before, s.committed(t))
}

// TestGraphicalImportMalformedContentPreservesCandidate requires an atomic
// refusal even when an earlier part of the submitted text was valid.
func TestGraphicalImportMalformedContentPreservesCandidate(t *testing.T) {
	s := newGraphicalImportServer(t)
	require.NoError(t, s.editors.SetValue("import-admin", []string{"system"}, "host", "retained-draft"))
	before := s.editors.ContentAtPath("import-admin", nil)
	committed := s.committed(t)
	response, body := s.paste(t, url.Values{"source": {"terminal"}, "location": {"absolute"}, "action": {"merge"}, "content": {"system { host attempted-change; } bgp { router-id not-an-address; }"}})
	require.Equal(t, http.StatusBadRequest, response.StatusCode, body)
	assert.Contains(t, body, "router-id")
	assert.Contains(t, strings.ToLower(body), "fix:")
	assert.Equal(t, before, s.editors.ContentAtPath("import-admin", nil))
	assert.Equal(t, committed, s.committed(t))
}

// TestGraphicalImportRefusesForgedChoices keeps form transport from becoming
// an unchecked command or an alternative configuration path parser.
func TestGraphicalImportRefusesForgedChoices(t *testing.T) {
	s := newGraphicalImportServer(t)
	before := s.editors.ContentAtPath("import-admin", nil)
	for _, altered := range []url.Values{
		{"source": {"terminal | delete system"}},
		{"location": {"outside"}},
		{"action": {"merge | delete system"}},
		{"path": {"bgp/group/bad%ZZ"}},
		{"source": {"file"}},
		{"content": {""}},
	} {
		fields := url.Values{"source": {"terminal"}, "location": {"absolute"}, "action": {"merge"}, "path": {""}, "content": {"system { host forged; }"}}
		maps.Copy(fields, altered)
		response, body := s.paste(t, fields)
		require.Equal(t, http.StatusBadRequest, response.StatusCode, body)
		assert.Contains(t, strings.ToLower(body), "fix:")
		assert.Equal(t, before, s.editors.ContentAtPath("import-admin", nil))
	}
}

// TestGraphicalImportBoundsTheRequest refuses oversized content before it can
// stage configuration and gives a useful operator recovery instruction.
func TestGraphicalImportBoundsTheRequest(t *testing.T) {
	s := newGraphicalImportServer(t)
	before := s.editors.ContentAtPath("import-admin", nil)
	response, body := s.paste(t, url.Values{"source": {"terminal"}, "location": {"absolute"}, "action": {"merge"}, "content": {strings.Repeat("x", (4<<20)+1)}})
	require.Equal(t, http.StatusRequestEntityTooLarge, response.StatusCode, body)
	assert.Contains(t, strings.ToLower(body), "fix:")
	assert.Equal(t, before, s.editors.ContentAtPath("import-admin", nil))
}

// TestGraphicalImportRequiresLoginEditAndSameOrigin proves all three guards
// on the production route, including a reader's forged mutation request.
func TestGraphicalImportRequiresLoginEditAndSameOrigin(t *testing.T) {
	s := newGraphicalImportServer(t)
	before := s.committed(t)
	for _, attempt := range []struct {
		cookie *http.Cookie
		origin string
		status int
	}{
		{nil, s.base, http.StatusUnauthorized},
		{s.reader, s.base, http.StatusForbidden},
		{s.admin, "https://other.invalid", http.StatusForbidden},
	} {
		fields := url.Values{"source": {"terminal"}, "location": {"absolute"}, "action": {"merge"}, "content": {"system { host denied-import; }"}}
		response, body := s.request(t, http.MethodPost, "/config/import", strings.NewReader(fields.Encode()), "application/x-www-form-urlencoded", attempt.cookie, attempt.origin)
		require.Equal(t, attempt.status, response.StatusCode, body)
	}
	assert.Contains(t, s.editors.ContentAtPath("import-admin", []string{"system"}), "import-original")
	assert.Contains(t, s.editors.ContentAtPath("import-reader", []string{"system"}), "import-original")
	assert.Equal(t, before, s.committed(t))
}
