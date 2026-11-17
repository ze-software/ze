//go:build ze_web

// Design: docs/architecture/web-interface.md -- caller-held graphical inspection.
package hub

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zeweb "github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/env"
)

const suppliedWebSource = `bgp {
    router-id 203.0.113.253;
    peer peer1 {
        description supplied-reader-sentinel;
        connection {
            remote { ip 127.0.0.1; }
            local { ip 127.0.0.1; }
        }
        session {
            asn { remote 65533; local 65533; }
        }
    }
}
system { host supplied-reader-sentinel; }
`
const suppliedWebAfter = `bgp {
    router-id 203.0.113.253;
    peer peer1 {
        description supplied-comparison-sentinel;
        connection {
            remote { ip 127.0.0.1; }
            local { ip 127.0.0.1; }
        }
        session {
            asn { remote 65533; local 65533; }
        }
    }
}
system { host supplied-comparison-sentinel; }
`

// TestWebSuppliedInspectionFinderOffersReadOnlyInputs enters the real primary
// Finder with a genuine reader. Choices come from the same registered owners
// as dispatch; candidate mutation remains forbidden.
func TestWebSuppliedInspectionFinderOffersReadOnlyInputs(t *testing.T) {
	web := newSuppliedWeb(t)
	response, body := web.get("/show/")
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	require.Contains(t, body, "id=\"graphical-supplied-toggle\"")
	require.Contains(t, body, "id=\"graphical-supplied-command\"")
	require.NotContains(t, body, "id=\"graphical-edit-toggle\"")
	count := 0
	for _, entry := range command.ListLocal() {
		if entry.Meta.SuppliedInput == nil {
			continue
		}
		count++
		require.Contains(t, body, "value=\""+entry.Path+"\"")
	}
	require.Positive(t, count, "the real supplied-input registry must be linked")
	response, body = web.get("/config/inspect/form?command=" + url.QueryEscape("show config supplied diff"))
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	for _, id := range []string{"graphical-supplied-before", "graphical-supplied-after", "graphical-supplied-before-file", "graphical-supplied-after-file", "graphical-supplied-submit"} {
		require.Contains(t, body, "id=\""+id+"\"")
	}
	require.Contains(t, body, "hx-post=\"/config/inspect\"")
	require.Contains(t, body, "hx-encoding=\"multipart/form-data\"")
	require.NotContains(t, body, "name=\"write\"")
}

// TestWebSuppliedInspectionAnswersMatchTerminal follows the actual authenticated
// GUI route through the real daemon dispatcher, then compares its displayed
// answer with the operational terminal using the same owner and bytes.
func TestWebSuppliedInspectionAnswersMatchTerminal(t *testing.T) {
	web := newSuppliedWeb(t)
	require.NoError(t, web.editors.SetValue("reader", []string{"system"}, "host", "keep-reader-candidate"))
	before := web.editors.ContentAtPath("reader", nil)
	committed, err := web.store.ReadFile("supplied-web.conf")
	require.NoError(t, err)
	logBefore := env.Get("ze.log")
	for _, operation := range []string{"dump", "validation", "fmt", "graph", "diff"} {
		t.Run(operation, func(t *testing.T) {
			path := "show config supplied " + operation
			values := url.Values{"command": {path}}
			source := []byte(suppliedWebSource)
			if operation == "diff" {
				values.Set("before", suppliedWebSource)
				values.Set("after", suppliedWebAfter)
				source, err = json.Marshal(map[string]string{"before": suppliedWebSource, "after": suppliedWebAfter})
				require.NoError(t, err)
			} else {
				values.Set("content", suppliedWebSource)
			}
			start := web.callCount()
			response, body := web.post("reader", "/config/inspect", values, nil)
			require.NoError(t, response.Body.Close())
			require.Equal(t, http.StatusOK, response.StatusCode, body)
			result := suppliedResultText(t, body)
			require.NotEmpty(t, result, "dropping all source input cannot satisfy inspection")
			calls := web.callsSince(start)
			require.Len(t, calls, 1, "one native form dispatches exactly one owned command")
			require.Equal(t, "reader", calls[0].caller.Username)
			require.NotEmpty(t, calls[0].caller.RemoteAddr)
			require.NotNil(t, calls[0].caller.Authorizer, "the accepted authentication policy must reach dispatch")
			words, parseErr := cli.ParseCommandWords(calls[0].line)
			require.NoError(t, parseErr)
			tokens := words.Tokens()
			require.GreaterOrEqual(t, len(tokens), 6)
			require.Equal(t, path, strings.Join(tokens[:4], " "))
			arguments, validateErr := command.ValidateModelArgs(path, tokens[4:], nil)
			require.NoError(t, validateErr)
			encoded, present := arguments.Value("content")
			require.True(t, present)
			decoded, decodeErr := command.DecodeContent(encoded)
			require.NoError(t, decodeErr)
			if operation == "diff" {
				var pair map[string]string
				require.NoError(t, json.Unmarshal(decoded, &pair))
				require.Equal(t, map[string]string{"before": suppliedWebSource, "after": suppliedWebAfter}, pair)
			} else {
				require.Equal(t, source, decoded, "the adapter must encode the actual bytes exactly once")
			}
			wantEncoded := base64.StdEncoding.EncodeToString(source)
			response, body = web.post("reader", "/cli/terminal", url.Values{
				"command": {path + " content " + wantEncoded}, "mode": {"operational"}, "history": {"0"},
			}, nil)
			require.NoError(t, response.Body.Close())
			require.Equal(t, http.StatusOK, response.StatusCode, body)
			var terminal struct {
				Output string `json:"output"`
			}
			require.NoError(t, json.Unmarshal([]byte(body), &terminal))
			require.Equal(t, terminal.Output, result, "GUI output must retain the shared formatter, warnings and diagnostics")
			if slices.Contains([]string{"dump", "fmt", "diff"}, operation) {
				require.Contains(t, result, "supplied-reader-sentinel")
			}
			if operation == "diff" {
				require.Contains(t, result, "supplied-comparison-sentinel")
			}
			require.Equal(t, before, web.editors.ContentAtPath("reader", nil))
			after, readErr := web.store.ReadFile("supplied-web.conf")
			require.NoError(t, readErr)
			require.Equal(t, committed, after)
			require.Equal(t, logBefore, env.Get("ze.log"))
		})
	}
	response, body := web.get("/cli?history=1&mode=operational")
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	require.NotContains(t, body, "supplied-reader-sentinel")
	require.NotContains(t, body, base64.StdEncoding.EncodeToString([]byte(suppliedWebSource)))
}

// TestWebSuppliedInspectionFileIsBytesAndBasename proves a native multipart
// selection carries source bytes while its label cannot become a daemon path.
func TestWebSuppliedInspectionFileIsBytesAndBasename(t *testing.T) {
	web := newSuppliedWeb(t)
	response, body := web.post("reader", "/config/inspect", url.Values{"command": {"show config supplied validation"}},
		map[string]suppliedWebFile{"file": {name: `C:\operator\picked.conf`, data: []byte(suppliedWebSource)}})
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	require.NotEmpty(t, suppliedResultText(t, body))
	calls := web.callsSince(0)
	require.Len(t, calls, 1)
	words, err := cli.ParseCommandWords(calls[0].line)
	require.NoError(t, err)
	arguments, err := command.ValidateModelArgs("show config supplied validation", words.Tokens()[4:], nil)
	require.NoError(t, err)
	name, present := arguments.Value("name")
	require.True(t, present)
	require.Equal(t, "picked.conf", name)
	require.NotContains(t, calls[0].line, "operator")
	require.Contains(t, calls[0].line, base64.StdEncoding.EncodeToString([]byte(suppliedWebSource)))
}

// TestWebSuppliedInspectionDeniedCallerUsesDaemonPolicy keeps a successful
// read-only caller beside the denied identity, then proves a real dispatcher
// rejection cannot leak the prior or refused source as a successful answer.
func TestWebSuppliedInspectionDeniedCallerUsesDaemonPolicy(t *testing.T) {
	web := newSuppliedWeb(t)
	values := url.Values{"command": {"show config supplied dump"}, "content": {suppliedWebSource}}
	response, body := web.post("reader", "/config/inspect", values, nil)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	require.Contains(t, suppliedResultText(t, body), "supplied-reader-sentinel")
	response, body = web.post("blocked", "/config/inspect", values, nil)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusForbidden, response.StatusCode, body)
	require.Contains(t, body, plugin.UnauthorizedMessage)
	require.NotContains(t, body, "supplied-reader-sentinel")
	calls := web.callsSince(0)
	require.Len(t, calls, 2, "the authenticated denied identity must be judged by the actual dispatcher")
	require.Equal(t, "blocked", calls[1].caller.Username)
	require.NotNil(t, calls[1].caller.Authorizer)
	require.Zero(t, web.editors.ChangeCount("blocked"))
}

// TestWebSuppliedInspectionCombinedBudget permits each source on its own,
// then refuses their oversized single comparison envelope before dispatch.
func TestWebSuppliedInspectionCombinedBudget(t *testing.T) {
	web := newSuppliedWeb(t)
	source := "bgp {\n    router-id 203.0.113.253;\n    peer peer1 {\n        description supplied-reader-sentinel;\n        connection {\n            remote { ip 127.0.0.1; }\n            local { ip 127.0.0.1; }\n        }\n        session {\n            asn { remote 65533; local 65533; }\n        }\n    }\n}\nsystem {\n host supplied-reader-sentinel;\n # " + strings.Repeat("x", 98_000) + "\n}\n"
	response, body := web.post("reader", "/config/inspect", url.Values{
		"command": {"show config supplied dump"}, "content": {source},
	}, nil)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	require.Contains(t, suppliedResultText(t, body), "supplied-reader-sentinel")
	before := web.callCount()
	response, body = web.post("reader", "/config/inspect", url.Values{
		"command": {"show config supplied diff"}, "before": {source}, "after": {source},
	}, nil)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusRequestEntityTooLarge, response.StatusCode, body)
	require.Contains(t, body, "195,840")
	require.Equal(t, before, web.callCount(), "never dispatch a truncated or independently budgeted pair")
}

// TestWebSuppliedInspectionInvalidValidationKeepsEvidence requires the shared
// diagnostic payload beside its failed verdict, instead of dropping useful
// validation details when the daemon reports an operational error.
func TestWebSuppliedInspectionInvalidValidationKeepsEvidence(t *testing.T) {
	web := newSuppliedWeb(t)
	response, body := web.post("reader", "/config/inspect", url.Values{
		"command": {"show config supplied validation"},
		"content": {"bgp { router-id invalid-supplied-address; }"},
	}, nil)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusBadRequest, response.StatusCode, body)
	result := suppliedResultText(t, body)
	require.Contains(t, result, "invalid-supplied-address")
	require.Contains(t, result, "router-id")
	require.Contains(t, result, "false")
	require.Zero(t, web.editors.ChangeCount("reader"))
}

// TestWebSuppliedInspectionFormUsesDeclaredChoices checks the actual input
// fragment carries the owner's optional flags, without offering file writes.
func TestWebSuppliedInspectionFormUsesDeclaredChoices(t *testing.T) {
	web := newSuppliedWeb(t)
	for _, selection := range []struct {
		operation string
		flags     []string
	}{
		{"dump", []string{"strip-private"}},
		{"fmt", []string{"check", "diff"}},
	} {
		response, body := web.get("/config/inspect/form?command=" + url.QueryEscape("show config supplied "+selection.operation))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusOK, response.StatusCode, body)
		for _, flag := range selection.flags {
			require.Contains(t, body, "name=\""+flag+"\"")
		}
		require.NotContains(t, body, "name=\"write\"")
	}
}

// TestWebSuppliedInspectionDisconnectCancelsProducer observes cancellation at
// the dispatcher dependency, rather than assuming that a closed HTTP request
// stops work that the adapter may have begun in a background context.
func TestWebSuppliedInspectionDisconnectCancelsProducer(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	web := newSuppliedWeb(t, func(ctx context.Context) {
		close(started)
		select {
		case <-ctx.Done():
			close(stopped)
		case <-release:
		}
	})
	request := web.request("reader", http.MethodPost, "/config/inspect", strings.NewReader(url.Values{
		"command": {"show config supplied dump"}, "content": {suppliedWebSource},
	}.Encode()), "application/x-www-form-urlencoded")
	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	request = request.WithContext(ctx)
	finished := make(chan error, 1)
	go func() {
		response, err := web.client.Do(request)
		if response != nil {
			err = errors.Join(err, response.Body.Close())
		}
		finished <- err
	}()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-started:
	case <-timer.C:
		t.Fatal("the actual inspection route did not reach its dispatcher")
	}
	cancel()
	select {
	case <-stopped:
	case <-timer.C:
		t.Fatal("HTTP cancellation did not reach the actual producer context")
	}
	require.ErrorIs(t, <-finished, context.Canceled)
}

// TestWebSuppliedInspectionRejectsAmbiguousForms refuses forged targets,
// lossy UTF-8 conversion, duplicate fields and conflicting file/paste inputs.
func TestWebSuppliedInspectionRejectsAmbiguousForms(t *testing.T) {
	web := newSuppliedWeb(t)
	for _, tc := range []struct {
		name  string
		form  url.Values
		files map[string]suppliedWebFile
	}{
		{"unrelated-command", url.Values{"command": {"show version"}, "content": {suppliedWebSource}}, nil},
		{"injected-command", url.Values{"command": {"show config supplied dump | save /tmp/not-a-target"}, "content": {suppliedWebSource}}, nil},
		{"duplicate-source", url.Values{"command": {"show config supplied dump"}, "content": {suppliedWebSource, suppliedWebAfter}}, nil},
		{"duplicate-command", url.Values{"command": {"show config supplied dump", "show config supplied fmt"}, "content": {suppliedWebSource}}, nil},
		{"unknown-field", url.Values{"command": {"show config supplied dump"}, "content": {suppliedWebSource}, "write": {"true"}}, nil},
		{"missing-second-source", url.Values{"command": {"show config supplied diff"}, "before": {suppliedWebSource}}, nil},
		{"unknown-source", url.Values{"command": {"show config supplied diff"}, "before": {suppliedWebSource}, "after": {suppliedWebAfter}, "third": {suppliedWebSource}}, nil},
		{"invalid-utf8-pair", url.Values{"command": {"show config supplied diff"}, "before": {string([]byte{0xff})}, "after": {suppliedWebAfter}}, nil},
		{"file-and-paste", url.Values{"command": {"show config supplied dump"}, "content": {suppliedWebSource}}, map[string]suppliedWebFile{"file": {name: "picked.conf", data: []byte(suppliedWebAfter)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := web.callCount()
			response, body := web.post("reader", "/config/inspect", tc.form, tc.files)
			require.NoError(t, response.Body.Close())
			require.Equal(t, http.StatusBadRequest, response.StatusCode, body)
			require.Equal(t, before, web.callCount())
			require.NotContains(t, body, "supplied-reader-sentinel")
			require.Zero(t, web.editors.ChangeCount("reader"))
		})
	}
}

// TestWebSuppliedInspectionHTTPGuards exercises the production route wrappers,
// including a useful authenticated same-origin request as the positive control.
func TestWebSuppliedInspectionHTTPGuards(t *testing.T) {
	web := newSuppliedWeb(t)
	values := url.Values{"command": {"show config supplied dump"}, "content": {suppliedWebSource}}
	response, body := web.post("reader", "/config/inspect", values, nil)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	before := web.callCount()
	request := web.request("", http.MethodPost, "/config/inspect", strings.NewReader(values.Encode()), "application/x-www-form-urlencoded")
	response, body = web.do(request)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusUnauthorized, response.StatusCode, body)
	request = web.request("reader", http.MethodPost, "/config/inspect", strings.NewReader(values.Encode()), "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://another-origin.invalid")
	response, body = web.do(request)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusForbidden, response.StatusCode, body)
	request = web.request("reader", http.MethodPost, "/config/inspect", strings.NewReader("content="+strings.Repeat("a", 4*1024*1024)), "application/x-www-form-urlencoded")
	response, body = web.do(request)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusRequestEntityTooLarge, response.StatusCode, body)
	require.Equal(t, before, web.callCount())
}

type suppliedWebFile struct {
	name string
	data []byte
}
type suppliedWebCall struct {
	caller plugin.CallerIdentity
	line   string
}
type suppliedWeb struct {
	t       *testing.T
	base    string
	client  *http.Client
	cookies map[string]*http.Cookie
	editors *zeweb.EditorManager
	store   storage.Storage
	mu      sync.Mutex
	calls   []suppliedWebCall
}

func newSuppliedWeb(t *testing.T, beforeDispatch ...func(context.Context)) *suppliedWeb {
	t.Helper()
	resetAAABundleForTest(t)
	setAPIConfigDir(t, t.TempDir())
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile("supplied-web.conf", []byte(strings.ReplaceAll(suppliedWebSource, "supplied-reader-sentinel", "running-supplied-control")), 0o600))
	policyStore := authz.NewStore()
	for _, profile := range []struct {
		name   string
		action authz.Action
	}{{"reader", authz.Allow}, {"blocked", authz.Deny}} {
		policyStore.AddProfile(authz.Profile{Name: profile.name, Run: authz.Section{Default: profile.action}, Edit: authz.Section{Default: authz.Deny}})
		policyStore.AssignProfiles(profile.name, []string{profile.name})
	}
	policy := authz.StoreAuthorizer{Store: policyStore}
	users := []authz.UserConfig{
		{Name: "reader", Hash: bcryptHash(t, "supplied-test-password"), Profiles: []string{"reader"}},
		{Name: "blocked", Hash: bcryptHash(t, "supplied-test-password"), Profiles: []string{"blocked"}},
	}
	aaaBundle.Store(&aaa.Bundle{Authenticator: &authz.LocalAuthenticator{Users: users}, Authorizer: policy})
	resolved, err := yangloader.DefaultLoader()
	require.NoError(t, err)
	dispatcher := pluginserver.NewDispatcher()
	require.NoError(t, pluginserver.LoadBuiltins(dispatcher, yangloader.WireMethodToPath(resolved),
		yangloader.PathToDescription(resolved), yangloader.PathToHelp(resolved), yangloader.PathToArgDefs(resolved)))
	dispatcher.SetAuthorizer(policy)
	web := &suppliedWeb{t: t, store: store, cookies: make(map[string]*http.Cookie)}
	dispatch := func(ctx context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
		web.mu.Lock()
		web.calls = append(web.calls, suppliedWebCall{caller: caller, line: line})
		web.mu.Unlock()
		for _, hook := range beforeDispatch {
			hook(ctx)
		}
		return dispatcher.Dispatch(&pluginserver.CommandContext{
			Username: caller.Username, RemoteAddr: caller.RemoteAddr, Authorizer: caller.Authorizer,
			RequestContext: ctx, Surface: "web", Sender: plugin.OperatorSender(),
		}, line)
	}
	server, broker, editors := startWebServer(store, "supplied-web.conf", []string{"127.0.0.1:0"}, false, "",
		dispatch, nil, policy, nil, nil, nil, func() ([]authz.UserConfig, error) { return users, nil },
		func() []command.CommandEntry { return dispatcher.Registry().VisibleCommandEntries() }, nil, nil)
	require.NotNil(t, server)
	web.editors = editors
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		require.NoError(t, server.Shutdown(ctx))
		broker.Close()
		require.NoError(t, editors.Close())
	})
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	require.NoError(t, server.WaitReady(ctx))
	web.base = "https://" + server.Address()
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // test-owned self-signed listener
	t.Cleanup(transport.CloseIdleConnections)
	web.client = &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, user := range users {
		response, body := web.post("", "/login", url.Values{"username": {user.Name}, "password": {"supplied-test-password"}}, nil)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusSeeOther, response.StatusCode, body)
		for _, cookie := range response.Cookies() {
			if cookie.Name == "ze-session" {
				web.cookies[user.Name] = cookie
			}
		}
		require.NotNil(t, web.cookies[user.Name])
	}
	return web
}

func (web *suppliedWeb) request(user, method, path string, body io.Reader, contentType string) *http.Request {
	web.t.Helper()
	request, err := http.NewRequestWithContext(web.t.Context(), method, web.base+path, body)
	require.NoError(web.t, err)
	if cookie := web.cookies[user]; cookie != nil {
		request.AddCookie(cookie)
	}
	request.AddCookie(&http.Cookie{Name: "ze-ui-mode", Value: "finder"})
	request.Header.Set("Origin", web.base)
	request.Header.Set("Content-Type", contentType)
	if path != "/login" {
		request.Header.Set("HX-Request", "true")
	}
	return request
}

func (web *suppliedWeb) do(request *http.Request) (*http.Response, string) {
	web.t.Helper()
	response, err := web.client.Do(request)
	require.NoError(web.t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(web.t, err)
	return response, string(body)
}

func (web *suppliedWeb) get(path string) (*http.Response, string) {
	web.t.Helper()
	return web.do(web.request("reader", http.MethodGet, path, nil, ""))
}

func (web *suppliedWeb) post(user, path string, form url.Values, files map[string]suppliedWebFile) (*http.Response, string) {
	web.t.Helper()
	if files == nil {
		return web.do(web.request(user, http.MethodPost, path, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded"))
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, values := range form {
		for _, value := range values {
			require.NoError(web.t, writer.WriteField(name, value))
		}
	}
	for name, file := range files {
		part, err := writer.CreateFormFile(name, file.name)
		require.NoError(web.t, err)
		_, err = part.Write(file.data)
		require.NoError(web.t, err)
	}
	require.NoError(web.t, writer.Close())
	return web.do(web.request(user, http.MethodPost, path, &body, writer.FormDataContentType()))
}

func (web *suppliedWeb) callCount() int {
	web.mu.Lock()
	defer web.mu.Unlock()
	return len(web.calls)
}

func (web *suppliedWeb) callsSince(start int) []suppliedWebCall {
	web.mu.Lock()
	defer web.mu.Unlock()
	return append([]suppliedWebCall(nil), web.calls[start:]...)
}

// suppliedResultText walks only the bounded output from this test's own web
// listener. No user-authored document is parsed as HTML in the product.
func suppliedResultText(t *testing.T, body string) string {
	t.Helper()
	document, err := html.Parse(strings.NewReader(body))
	require.NoError(t, err)
	var found *html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		for _, attribute := range node.Attr {
			if attribute.Key == "id" {
				if attribute.Val == "graphical-supplied-result" {
					found = node
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	require.NotNil(t, found, "the graphical result must be visible at its native output target")
	var result strings.Builder
	var text func(*html.Node)
	text = func(node *html.Node) {
		if node.Type == html.TextNode {
			result.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			text(child)
		}
	}
	text(found)
	return result.String()
}
