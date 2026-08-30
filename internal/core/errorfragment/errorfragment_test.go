// Related: errorfragment.go -- Middleware and Render, the producers under test

package errorfragment_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/errorfragment"
)

// answerCase is one answer a handler writes, beside what the middleware owes it.
type answerCase struct {
	// Name is the subtest name, and HTMX sends the header htmx puts on a
	// request whose answer it will swap.
	Name string
	HTMX bool

	// Plain answers the way http.Error does, which is what the 47 refusal sites
	// in the looking glass and the chaos dashboard write. Otherwise the handler
	// writes AnswerType, AnswerStatus and AnswerBody itself.
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
func (c answerCase) handler(t *testing.T) http.HandlerFunc {
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

// answerCases cover one case per KIND of answer, because the middleware decides
// by kind rather than by route. The pass-through kinds are what a mechanical
// rewrite of every http.Error site would have broken.
var answerCases = []answerCase{
	{
		Name: "a refused htmx request", HTMX: true,
		Plain: true, AnswerStatus: http.StatusBadRequest, AnswerBody: "invalid peer id",
		Status: http.StatusBadRequest, Type: "text/html; charset=utf-8",
		Message: "invalid peer id",
	},
	{
		Name: "a failed htmx request", HTMX: true,
		Plain: true, AnswerStatus: http.StatusServiceUnavailable, AnswerBody: "no control channel",
		Status: http.StatusServiceUnavailable, Type: "text/html; charset=utf-8",
		Message: "no control channel",
	},
	{
		Name: "a refusal to a client that is not htmx", HTMX: false,
		Plain: true, AnswerStatus: http.StatusBadRequest, AnswerBody: "invalid peer id",
		Status: http.StatusBadRequest, Type: "text/plain; charset=utf-8",
		Body: "invalid peer id\n",
	},
	{
		Name: "a handler that wrote its own fragment", HTMX: true,
		AnswerType: "text/html; charset=utf-8", AnswerStatus: http.StatusBadRequest,
		AnswerBody: `<div class="graph-empty">no routes</div>`,
		Status:     http.StatusBadRequest, Type: "text/html; charset=utf-8",
		Body: `<div class="graph-empty">no routes</div>`,
	},
	{
		Name: "a refusal to a client that negotiated JSON", HTMX: true,
		AnswerType: "application/json", AnswerStatus: http.StatusServiceUnavailable,
		AnswerBody: `{"error":"engine unavailable"}`,
		Status:     http.StatusServiceUnavailable, Type: "application/json",
		Body: `{"error":"engine unavailable"}`,
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
// what a rewrite of the 47 refusal sites in lg and chaos would have risked, and
// this middleware exists so that rewrite was never needed.
func TestMiddlewareConvertsOnlyBareStatusLines(t *testing.T) {
	for _, c := range answerCases {
		t.Run(c.Name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.Handle("GET /probe", c.handler(t))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/probe", http.NoBody)
			if c.HTMX {
				req.Header.Set("HX-Request", "true")
			}

			rec := httptest.NewRecorder()
			errorfragment.Middleware(mux).ServeHTTP(rec, req)

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
// PREVENTS: an operator value in an error message becoming live nodes. A
// message is built from a query or a form value at several sites (the looking
// glass writes "invalid prefix" beside what the operator typed), and this
// fragment is what puts one inside a document.
func TestRenderEscapesTheMessage(t *testing.T) {
	body := string(errorfragment.Render(http.StatusBadRequest,
		`invalid prefix: <img src=x onerror="alert(1)">`))

	assert.NotContains(t, body, "<img", "the message reached the document as an element")
	assert.NotContains(t, body, `onerror="`, "the message reached the document as an attribute")
	assert.Contains(t, body, "&lt;img", "the message must be escaped rather than dropped")
	assert.Contains(t, body, "onerror=&#34;", "the escaped text is still the whole message")
}

// VALIDATES: the fragment names the status it answers, in the attribute and in
// the sentence beside the message.
// PREVENTS: a fragment an operator cannot tell a refusal from a failure by. A
// status outside net/http's table still gets a fragment.
func TestRenderNamesTheStatus(t *testing.T) {
	refusal := string(errorfragment.Render(http.StatusForbidden, "cross-origin request refused"))
	assert.Contains(t, refusal, `data-error-status="403"`, "the status must be readable by a script")
	assert.Contains(t, refusal, "Forbidden", "the reason comes from http.StatusText")
	assert.Contains(t, refusal, "cross-origin request refused", "the message must survive")

	unknown := string(errorfragment.Render(599, "upstream said nothing"))
	assert.Contains(t, unknown, `data-error-status="599"`)
	assert.Contains(t, unknown, ">599<", "a status net/http does not name repeats the number")
}

// VALIDATES: a streaming handler still flushes through the middleware, and a
// canceled stream is not turned into a fragment.
// PREVENTS: the SSE routes of the looking glass and the chaos dashboard
// stalling. Both write their head, flush, and then write until the client
// leaves; a wrapper that swallows the flush holds the first event forever.
func TestMiddlewareForwardsAFlush(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write([]byte("event: hello\n\n")); err != nil {
			t.Errorf("the stream could not be written: %v", err)
		}

		flusher, ok := w.(http.Flusher)
		require.True(t, ok, "the middleware must keep the writer flushable, or the SSE routes answer 500")
		flusher.Flush()
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/events", http.NoBody)
	req.Header.Set("HX-Request", "true")

	rec := httptest.NewRecorder()
	errorfragment.Middleware(mux).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, rec.Flushed, "the flush must reach the client")
	assert.True(t, strings.HasPrefix(rec.Body.String(), "event: hello"),
		"the stream must reach the client untouched")
}
