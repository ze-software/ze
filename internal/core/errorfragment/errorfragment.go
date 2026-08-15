// Design: docs/architecture/web-interface.md -- the error answer a refused htmx
// request receives
// Related: errorfragment_test.go -- the kind table this middleware is driven by

// Package errorfragment answers a refused htmx request with markup the browser
// can swap into the target the request named.
//
// Ze serves three HTTP user interfaces from three http.Server instances: the
// operator web UI, the looking glass and the chaos dashboard. Each one refuses a
// request with http.Error, which writes a bare status line as text/plain. htmx 2
// swaps no 4xx and no 5xx, so that line reaches nobody. htmx 4 swaps every
// response except 204 and 304, so the same line would land in the element the
// operator was looking at.
//
// The conversion is one answer for all three, held here rather than written
// three times. The markup is one shape as well, so a class an interface's script
// reads is a class every interface renders.
package errorfragment

import (
	"bufio"
	"bytes"
	"errors"
	"html"
	"net"
	"net/http"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// requestHeader is the header htmx puts on every request it issues, and
// requestTrue is the value it sends. A request without it has no target to swap
// an answer into: its reader is curl, a script, or a browser navigating.
const (
	requestHeader = "HX-Request"
	requestTrue   = "true"
)

// plainText is the media type http.Error writes. It is the one body kind this
// package converts.
const plainText = "text/plain"

// fragmentType is what a converted answer is served as.
const fragmentType = "text/html; charset=utf-8"

// The fragment is one div holding two spans: the standard sentence for the
// status, and the message the handler wrote. The class names are a contract with
// the scripts that read a failed action out of the body.
const (
	fragmentOpen    = `<div class="error-fragment" role="alert" data-error-status="`
	fragmentReason  = `"><span class="error-fragment-reason">`
	fragmentMessage = `</span> <span class="error-fragment-message">`
	fragmentClose   = `</span></div>`
)

// errNotHijackable is what a hijack attempt gets while a refusal is buffered. A
// hijacked connection leaves this package nothing to write into, and no route
// both refuses a request and hijacks it.
var errNotHijackable = errors.New("errorfragment: a buffered error response cannot be hijacked")

// Render writes the fragment for one refusal.
//
// The message is the text the handler wrote, and it is escaped rather than
// concatenated: several handlers build it from a query or a form value, so it is
// the one value here an operator controls. The reason beside it comes from
// http.StatusText, a fixed table in net/http.
func Render(status int, message string) []byte {
	var code textbuf.Buffer

	digits := code.Int(int64(status)).String()

	reason := http.StatusText(status)
	if reason == "" {
		// A status outside net/http's table still gets a fragment. The number is
		// the whole answer, so the reason repeats it rather than being blank.
		reason = digits
	}

	escaped := html.EscapeString(message)

	out := make([]byte, 0, len(fragmentOpen)+len(digits)+len(fragmentReason)+len(reason)+
		len(fragmentMessage)+len(escaped)+len(fragmentClose))
	out = append(out, fragmentOpen...)
	out = append(out, digits...)
	out = append(out, fragmentReason...)
	out = append(out, reason...)
	out = append(out, fragmentMessage...)
	out = append(out, escaped...)
	out = append(out, fragmentClose...)

	return out
}

// Middleware answers an htmx request that a handler refused with a fragment.
//
// It converts ONE kind of answer: the plain-text body http.Error writes, at a
// 4xx or a 5xx, to a request carrying HX-Request. Everything else passes through
// untouched, because everything else is already the answer its caller meant:
//
//   - a handler that answered text/html has written its own fragment;
//   - a handler that answered application/json is talking to a client that
//     negotiated JSON, and markup would break it;
//   - a request with no HX-Request header has no target to swap into;
//   - a refusal with no body has no message to render, and inventing one would
//     state something no handler said.
//
// Wrap the mux with it in ONE place, so the daemon and the tests that capture
// responses cannot disagree about the chain.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(requestHeader) != requestTrue {
			next.ServeHTTP(w, r)

			return
		}

		buffered := &writer{ResponseWriter: w}
		next.ServeHTTP(buffered, r)
		buffered.finish()
	})
}

// writer buffers a plain-text refusal so finish can answer the fragment
// instead. It writes through for every other response, and it holds nothing
// until a handler declares a status this package converts.
type writer struct {
	http.ResponseWriter

	// status is the status the handler declared, and wrote records that it
	// declared one. A handler that writes a body without a status is answering
	// 200, which is never converted.
	status int
	wrote  bool

	// capture is set when the declared answer is a plain-text refusal, and body
	// holds what the handler wrote while it is set.
	capture bool
	body    bytes.Buffer
}

// WriteHeader records the status. It withholds a plain-text refusal from the
// underlying writer, because the fragment's Content-Type is only known once the
// body has arrived.
func (w *writer) WriteHeader(status int) {
	if w.wrote {
		return
	}

	w.wrote = true
	w.status = status

	if status >= http.StatusBadRequest && strings.HasPrefix(w.Header().Get("Content-Type"), plainText) {
		w.capture = true

		return
	}

	w.ResponseWriter.WriteHeader(status)
}

// Write buffers the body of a refusal and writes every other body through.
func (w *writer) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}

	if w.capture {
		return w.body.Write(b)
	}

	n, err := w.ResponseWriter.Write(b)
	if err != nil {
		return n, err //nolint:wrapcheck // the caller is a handler writing its own body
	}

	return n, nil
}

// Flush forwards a flush from a streaming handler. A buffered refusal has
// nothing to flush: its body is complete only when the handler returns.
func (w *writer) Flush() {
	if w.capture {
		return
	}

	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack forwards a hijack, so a route that takes the connection over still
// works. It refuses while a refusal is buffered, rather than hand out a
// connection this package still owes a body to.
func (w *writer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.capture {
		return nil, nil, errNotHijackable
	}

	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err //nolint:wrapcheck // the caller is a handler taking the connection
	}

	return conn, rw, nil
}

// Unwrap gives http.ResponseController the writer underneath, so a read or
// write deadline this type does not implement still reaches the connection. A
// ResponseController prefers an implemented method over the unwrapped writer,
// and Flush and Hijack are implemented above, so neither escapes the buffer.
func (w *writer) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// finish writes the buffered refusal as a fragment. It runs after the handler
// returns, which is when the message is complete.
func (w *writer) finish() {
	if !w.capture {
		return
	}

	message := strings.TrimSpace(w.body.String())
	if message == "" {
		// No body, so no message. A fragment carrying only a status would say
		// something no handler said.
		w.ResponseWriter.WriteHeader(w.status)

		return
	}

	header := w.Header()
	header.Set("Content-Type", fragmentType)
	header.Del("Content-Length")

	w.ResponseWriter.WriteHeader(w.status)

	if _, err := w.ResponseWriter.Write(Render(w.status, message)); err != nil {
		// The client has gone. There is nothing to answer and nobody to tell:
		// this package holds no logger, and the handler already returned.
		return
	}
}
