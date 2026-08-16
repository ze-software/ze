// Related: handler_golden_test.go -- newWebGoldenEnv and webGoldenRequest, the
// server and the request builder these cases reuse
// Related: auth.go -- serverHandler, the chain under test. It wraps the mux with
// errorfragment.Middleware (internal/core/errorfragment), which is the producer
// these cases drive from this package's entry point
// Related: assets/notification.js -- handleResponseError and handleRequestError,
// the browser side that surfaces a failed action: one for an answer the daemon
// refused, one for htmx 4's merged error event

package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/errorfragment"
)

// webErrorCase is one request a handler answers with a 4xx or a 5xx.
type webErrorCase struct {
	// Name is the subtest name.
	Name string
	// Method, Target and Form are the request.
	Method string
	Target string
	Form   string
	// Origin, when set, replaces the same-origin header a browser sends. It is
	// how a case reaches the cross-origin refusal.
	Origin string
	// Status is the status the handler answers with.
	Status int
	// Producer names the file and the symbol that writes that status.
	Producer string
}

// webErrorCases are two error answers an operator can reach from the UI. One is
// a refused edit, which is the failure an operator meets most; the other is the
// cross-origin refusal every mutating route shares, so it covers the middleware
// rather than one handler.
var webErrorCases = []webErrorCase{
	{
		Name:     "config-rename-missing-new-key",
		Method:   http.MethodPost,
		Target:   "/config/rename/bgp/peer/alpha/",
		Form:     "new-key=",
		Status:   http.StatusBadRequest,
		Producer: "handler_config_entry.go, HandleConfigRenameWithAuthorizer",
	},
	{
		Name:     "config-set-cross-origin",
		Method:   http.MethodPost,
		Target:   "/config/set/bgp/",
		Form:     "leaf=router-id&value=9.9.9.9",
		Origin:   "https://elsewhere.example",
		Status:   http.StatusForbidden,
		Producer: "handler_tools.go, RequireSameOrigin",
	},
}

// VALIDATES: a handler that answers 4xx or 5xx returns markup the browser can
// swap into the target the request named, rather than a bare status line.
// PREVENTS: an operator getting no feedback on a failed action. htmx 4 swaps
// every response except 204 and 304, so a plain-text status line would land in
// the page where the answer was meant to go.
func TestErrorStatusReturnsSwappableFragment(t *testing.T) {
	for _, c := range webErrorCases {
		t.Run(c.Name, func(t *testing.T) {
			env := newWebGoldenEnv(t, false)

			req := webGoldenRequest(t, env, webHandlerCase{
				Method: c.Method,
				Target: c.Target,
				Form:   c.Form,
				HTMX:   true,
			})
			if c.Origin != "" {
				req.Header.Set("Origin", c.Origin)
			}

			rec := httptest.NewRecorder()
			env.handler.ServeHTTP(rec, req)

			if rec.Code != c.Status {
				t.Fatalf("%s answered %d, want %d; this case no longer reaches %s\n%s",
					c.Target, rec.Code, c.Status, c.Producer, rec.Body.String())
			}

			body := strings.TrimSpace(rec.Body.String())
			if body == "" {
				t.Fatalf("%s answered %d with no body at all", c.Target, rec.Code)
			}

			if !strings.HasPrefix(body, "<") || !strings.HasSuffix(body, ">") {
				t.Errorf("%s answered %d with a bare status line, not a fragment (%s writes it): %q",
					c.Target, rec.Code, c.Producer, body)
			}

			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
				t.Errorf("%s answered %d as %q; a fragment the browser swaps is text/html",
					c.Target, rec.Code, ct)
			}
		})
	}
}

// errorMiddlewareCase is one answer a handler writes, beside what the
// middleware owes it.
type errorMiddlewareCase struct {
	// Name is the subtest name, and HTMX sends the header htmx puts on a
	// request whose answer it will swap.
	Name string
	HTMX bool

	// Plain answers the way http.Error does, which is what 142 sites under this
	// package write. Otherwise the handler writes AnswerType, AnswerStatus and
	// AnswerBody itself.
	Plain        bool
	AnswerType   string
	AnswerStatus int
	AnswerBody   string

	// Status and Type are what the client must read.
	Status int
	Type   string

	// Body, when set, is the answer verbatim: the middleware must not have
	// touched it. Message, when set, is what the fragment must carry.
	Body    string
	Message string
}

// handler builds the answer this case's handler writes.
func (c errorMiddlewareCase) handler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, _ *http.Request) {
		if c.Plain {
			http.Error(w, c.AnswerBody, c.AnswerStatus)

			return
		}

		if c.AnswerType != "" {
			w.Header().Set("Content-Type", c.AnswerType)
		}

		w.WriteHeader(c.AnswerStatus)

		if c.AnswerBody == "" {
			return
		}

		if _, err := w.Write([]byte(c.AnswerBody)); err != nil {
			t.Errorf("the case handler could not write its answer: %v", err)
		}
	}
}

// errorMiddlewareCases cover one case per KIND of answer, because the
// middleware decides by kind rather than by route. The three pass-through kinds
// are what a mechanical rewrite of every http.Error site would have broken.
var errorMiddlewareCases = []errorMiddlewareCase{
	{
		Name: "a refused htmx request", HTMX: true,
		Plain: true, AnswerStatus: http.StatusBadRequest, AnswerBody: "missing new-key",
		Status: http.StatusBadRequest, Type: "text/html; charset=utf-8",
		Message: "missing new-key",
	},
	{
		Name: "a failed htmx request", HTMX: true,
		Plain: true, AnswerStatus: http.StatusInternalServerError, AnswerBody: "render: template missing",
		Status: http.StatusInternalServerError, Type: "text/html; charset=utf-8",
		Message: "render: template missing",
	},
	{
		Name: "a refusal to a client that is not htmx", HTMX: false,
		Plain: true, AnswerStatus: http.StatusBadRequest, AnswerBody: "missing new-key",
		Status: http.StatusBadRequest, Type: "text/plain; charset=utf-8",
		Body: "missing new-key\n",
	},
	{
		Name: "a handler that wrote its own fragment", HTMX: true,
		AnswerType: "text/html; charset=utf-8", AnswerStatus: http.StatusBadRequest,
		AnswerBody: `<div class="tool-overlay">refused</div>`,
		Status:     http.StatusBadRequest, Type: "text/html; charset=utf-8",
		Body: `<div class="tool-overlay">refused</div>`,
	},
	{
		Name: "a refusal to a client that negotiated JSON", HTMX: true,
		AnswerType: "application/json", AnswerStatus: http.StatusInternalServerError,
		AnswerBody: `{"error":true}`,
		Status:     http.StatusInternalServerError, Type: "application/json",
		Body: `{"error":true}`,
	},
	{
		Name: "an answer that is not a refusal", HTMX: true,
		AnswerType: "text/plain; charset=utf-8", AnswerStatus: http.StatusOK,
		AnswerBody: "plain but fine",
		Status:     http.StatusOK, Type: "text/plain; charset=utf-8",
		Body: "plain but fine",
	},
	{
		Name: "a refusal with no body", HTMX: true,
		AnswerStatus: http.StatusMethodNotAllowed,
		Status:       http.StatusMethodNotAllowed,
		Body:         "",
	},
}

// VALIDATES: the middleware converts the bare status line http.Error writes,
// and passes every other answer through byte for byte.
// PREVENTS: markup reaching a client that negotiated JSON, a handler's own
// fragment being wrapped in a second one, and a 200 being rewritten. Each is
// what a rewrite of all 142 http.Error sites would have risked, and the
// middleware exists so that rewrite was never needed.
func TestErrorFragmentMiddlewareConvertsOnlyBareStatusLines(t *testing.T) {
	for _, c := range errorMiddlewareCases {
		t.Run(c.Name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.Handle("GET /probe", c.handler(t))

			req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
			if c.HTMX {
				req.Header.Set("HX-Request", htmxRequestTrue)
			}

			rec := httptest.NewRecorder()
			serverHandler(mux).ServeHTTP(rec, req)

			require.Equal(t, c.Status, rec.Code, "status")

			if c.Type != "" {
				assert.Equal(t, c.Type, rec.Header().Get("Content-Type"), "Content-Type")
			}

			if c.Message == "" {
				assert.Equal(t, c.Body, rec.Body.String(), "the answer must reach the client untouched")

				return
			}

			assert.Contains(t, rec.Body.String(), `class="error-fragment"`,
				"a refused htmx request must be answered with the fragment")
			assert.Contains(t, rec.Body.String(), c.Message,
				"the fragment must carry what the handler wrote")
		})
	}
}

// VALIDATES: a message reaches the browser as text, never as markup.
// PREVENTS: an operator value in an error message becoming live nodes. The
// message is built from form input at several sites (handler_config_form.go),
// and this fragment is the first path that puts one inside a document.
func TestErrorFragmentEscapesTheMessage(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /probe", errorMiddlewareCase{
		Plain:        true,
		AnswerStatus: http.StatusBadRequest,
		AnswerBody:   `invalid value: <img src=x onerror="alert(1)">`,
	}.handler(t))

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	req.Header.Set("HX-Request", htmxRequestTrue)

	rec := httptest.NewRecorder()
	serverHandler(mux).ServeHTTP(rec, req)

	body := rec.Body.String()
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.NotContains(t, body, "<img", "the message reached the document as an element")
	assert.NotContains(t, body, `onerror="`, "the message reached the document as an attribute")
	assert.Contains(t, body, "&lt;img", "the renderer must escape the message rather than drop it")
	assert.Contains(t, body, "onerror=&#34;", "the escaped text is still the whole message")
}

// jsQuerySelector finds the class a script reads one element by.
var jsQuerySelector = regexp.MustCompile(`querySelector\('\.([^']+)'\)`)

// VALIDATES: assets/notification.js reads a class the fragment carries, so the
// toast an operator sees is the message rather than the markup around it.
// PREVENTS: AC-7 breaking silently. htmx 2 swaps no 4xx, so this script is the
// ONLY thing that shows a failed action today; it read the response body
// verbatim, and that body has just become markup. No browser runs in this
// package (TestErrorDrawerWiringHoldsTogether says why), so the selector and the
// class are held together here, and proven in a browser by
// test/web/web-error-fragment.wb.
func TestErrorFragmentAndNotificationAgree(t *testing.T) {
	markup := string(errorfragment.Render(http.StatusBadRequest, webErrorFragmentMessage))
	require.Contains(t, markup, webErrorFragmentMessage[:len("invalid uint16")],
		"the shared renderer must carry the message this test reads the classes around")

	script, err := os.ReadFile(filepath.Join("assets", "notification.js"))
	require.NoError(t, err)

	source := string(script)

	reader := jsBlock(source, "function errorMessageFromBody(")
	require.NotEmpty(t, reader, "notification.js must define errorMessageFromBody")

	handler := jsBlock(source, "function handleResponseError(")
	require.NotEmpty(t, handler, "notification.js must define handleResponseError")
	assert.Contains(t, handler, "errorMessageFromBody(",
		"handleResponseError must read the body through the fragment reader, or the toast shows markup")

	selectors := jsQuerySelector.FindAllStringSubmatch(reader, -1)
	require.NotEmpty(t, selectors, "errorMessageFromBody must select the message element")

	for _, match := range selectors {
		assert.True(t, markupHasClass(markup, match[1]),
			"notification.js reads .%s and the error fragment does not carry it", match[1])
	}
}

// VALIDATES: AC-5 -- notification.js answers htmx 4's ONE error event and tells
// its causes apart, so a refusal raises one toast and a request that reached
// nobody raises one too.
// PREVENTS: two toasts for one refusal, and silence when the daemon is
// unreachable. htmx 4 folds htmx 2's sendError into htmx:error, which also fires
// when a swap throws and when a timeout aborts a request: the handler that
// meant "the request never left" now answers three causes it never saw, and the
// ctx is what separates them. A ctx with a response of 400 or more has already
// been reported by handleResponseError.
//
// No browser runs in this package (TestErrorDrawerWiringHoldsTogether says
// why). The same discrimination is proven in one on the chaos dashboard, whose
// listeners read the same two fields: test/web/chaos-error-fragment.wb.
func TestMergedErrorEventIsToldApart(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("assets", "notification.js"))
	require.NoError(t, err)

	source := string(script)

	assert.Contains(t, source, `addEventListener('htmx:error', handleRequestError)`,
		"the merged error event must have a listener, or a request that reached nobody says nothing")
	assert.Contains(t, source, `addEventListener('htmx:response:error', handleResponseError)`,
		"a refused request must still raise the toast carrying what the handler wrote")

	handler := jsBlock(source, "function handleRequestError(")
	require.NotEmpty(t, handler, "notification.js must define handleRequestError")
	assert.Contains(t, handler, "ctx.response",
		"the merged event must be told apart by what the ctx holds, or a refusal is reported twice")
	assert.NotContains(t, handler, "xhr",
		"htmx 4 requests with fetch, so no event it dispatches carries an xhr")
}

// VALIDATES: a rejected value for a leaf the schema marks secret never reaches
// the response, on the route an operator's form post takes.
// PREVENTS: publishing the credential the operator just typed. Every validator
// that names what it refused quotes the value: config.ValidateValue writes
// "invalid uint16: %q" and validateUniqueOnSet writes "duplicate %s %q". That
// message reaches the browser twice now, in the fragment and in the toast built
// from it.
//
// The leaf is built here rather than taken from the shipped YANG because no
// shipped secret leaf is typed anything but a string today, and ValidateValue
// refuses no string. That pairing is one YANG edit away, and the guard has to
// hold when it arrives.
func TestRefusedSecretValueNeverReachesTheBrowser(t *testing.T) {
	const secret = "hunter2-4471bc"

	schema := config.NewSchema()
	token := config.Leaf(config.TypeUint16)
	token.Sensitive = true
	schema.Define("environment", config.Container(
		config.Field("api-server", config.Container(config.Field("token", token))),
	))

	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0o600))

	mgr := NewEditorManager(storage.NewFilesystem(), configPath, schema,
		testEditorFactory(), testEditSessionFactory())

	mux := http.NewServeMux()
	mux.Handle("POST /config/set/", HandleConfigSetWithAuthorizer(mgr, schema, nil, nil))

	req := postConfigRequest(t, "/config/set/environment/api-server/", map[string][]string{
		"leaf":  {"token"},
		"value": {secret},
	}, "alice")
	req.Header.Set("HX-Request", htmxRequestTrue)

	rec := httptest.NewRecorder()
	serverHandler(mux).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code,
		"the value must be refused, or this test proves nothing: %s", rec.Body.String())
	assert.NotContains(t, rec.Body.String(), secret,
		"the refusal published the secret the operator typed")
	assert.Contains(t, rec.Body.String(), config.SecretDataPlaceholder,
		"the message must say a value was refused, with the value masked")
}

// VALIDATES: the mask holds for a secret carrying a character %q escapes.
// PREVENTS: the hole a plain substring replacement leaves. config.ValidateValue
// writes `invalid uint16: %q`, so a value holding a quote reaches the message as
// `pa\"ss` and the raw text appears nowhere in it. A guard that covers the easy
// values and not the awkward ones publishes the credential precisely when it is
// least guessable.
//
// The assertion reads the TAIL of the value rather than the whole of it: the
// fragment escapes its message, so the leaked form is `...\&#34;...` and the raw
// value is absent from a leaking body as well as from a masked one.
func TestRefusedSecretWithAQuoteNeverReachesTheBrowser(t *testing.T) {
	const (
		secret = `hunter2"4471bc`
		tail   = "4471bc"
	)

	schema := config.NewSchema()
	token := config.Leaf(config.TypeUint16)
	token.Sensitive = true
	schema.Define("environment", config.Container(
		config.Field("api-server", config.Container(config.Field("token", token))),
	))

	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0o600))

	mgr := NewEditorManager(storage.NewFilesystem(), configPath, schema,
		testEditorFactory(), testEditSessionFactory())

	mux := http.NewServeMux()
	mux.Handle("POST /config/set/", HandleConfigSetWithAuthorizer(mgr, schema, nil, nil))

	req := postConfigRequest(t, "/config/set/environment/api-server/", map[string][]string{
		"leaf":  {"token"},
		"value": {secret},
	}, "alice")
	req.Header.Set("HX-Request", htmxRequestTrue)

	rec := httptest.NewRecorder()
	serverHandler(mux).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code,
		"the value must be refused, or this test proves nothing: %s", rec.Body.String())
	assert.NotContains(t, rec.Body.String(), tail,
		"the refusal published the escaped form of the secret the operator typed")
	assert.Contains(t, rec.Body.String(), config.SecretDataPlaceholder,
		"the message must say a value was refused, with the value masked")
}
