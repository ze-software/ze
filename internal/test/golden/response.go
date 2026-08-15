// Design: (none -- test utility, no architecture doc)

package golden

import (
	"bytes"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Rewrite replaces a volatile span of a captured response with a fixed
// placeholder, so the fixture holds the same bytes on every run.
//
// A rewrite is the last resort, not the first. It blinds the capture to
// everything it matches. It MUST therefore match the value alone. It MUST NOT
// match the markup around it.
//
// Where the test can fix the value a handler reads, fix the value instead.
// Every rewrite carries a comment naming the volatile producer.
type Rewrite struct {
	// Pattern selects the volatile span.
	Pattern *regexp.Regexp
	// Replacement is what the span becomes. It can reference capture groups,
	// which is how a rewrite keeps the markup and drops only the value.
	Replacement string
}

// VersionHeader drops the value of the X-Ze-Version response header.
// version.HTTPHeader builds it from the release string, the build commit, the
// Go version, GOOS and GOARCH. Each one is a property of the machine that ran
// the capture. A fixture holding them is red on the next machine, and it says
// nothing about the rendering.
//
// The pattern requires the SHAPE version.HTTPHeader writes, which that function
// states in its own doc comment, and drops the values inside it. Only the header
// name is a capture group. A header the server stops sending, sends empty, or
// sends in another shape matches nothing. The raw line then stays in the fixture
// bytes and the comparison fails, rather than an absent version reading as a
// normalized one.
var VersionHeader = Rewrite{
	Pattern:     regexp.MustCompile(`(?m)^(header: X-Ze-Version: )ze/\S+ \([^()]*go\S+[^()]*\)$`),
	Replacement: "${1}<VERSION>",
}

// Response turns a recorded response into fixture bytes: the status, the
// response headers sorted, a blank line, then the body.
//
// Headers are read from the recorder rather than from Result. A header the
// handler sets after WriteHeader is therefore recorded, although a live server
// would not send it. The recorder also sniffs no content type, which a live
// server does for a handler that sets none. Both gaps are the same before and
// after a rendering change, which is the comparison this capture makes.
func Response(rec *httptest.ResponseRecorder, rewrites []Rewrite) []byte {
	var b bytes.Buffer

	b.WriteString("status: ")
	b.WriteString(strconv.Itoa(rec.Code))
	b.WriteString("\n")

	header := rec.Header()

	names := make([]string, 0, len(header))
	for name := range header {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		values := make([]string, len(header[name]))
		copy(values, header[name])
		sort.Strings(values)

		for _, value := range values {
			b.WriteString("header: ")
			b.WriteString(name)
			b.WriteString(": ")
			b.WriteString(value)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.Write(rec.Body.Bytes())

	out := b.Bytes()
	for _, r := range rewrites {
		out = r.Pattern.ReplaceAll(out, []byte(r.Replacement))
	}

	return out
}

// AssertResponseHasBody refuses a captured response that answers with nothing.
//
// A handler capture byte-compares, so it accepts whatever the handler wrote,
// an empty page included. A component deleted from under a handler renders
// nothing, the handler answers 200 with no body, and one -update-golden run
// freezes that as the expected answer. Nothing later reports it: the fixture
// and the render agree.
//
// webGoldenDiffHandler is the case this exists for. It called a component the
// port had deleted, and only a reader noticed. The TEMPLATE capture has carried
// the same refusal since it was written: webCaptureSet fails a unit that
// renders only whitespace in every variant.
//
// The exemptions are DERIVED, not declared. A redirect, a 204 and a 304 each
// carry no body by their own definition. The stream exemption reads a property
// the case already states. A stream case cancels its request before the
// handler runs, so the capture holds nothing the handler wrote.
//
// It ends the subtest rather than reporting it, and that is load-bearing. A
// capture run writes the fixture after this call, so anything short of Fatal
// leaves the empty page on disk beside a red test. Every case is its own
// subtest, so the rest of the capture still runs.
func AssertResponseHasBody(t *testing.T, name string, got []byte, stream bool) {
	t.Helper()

	if finding := responseBodyFinding(name, got, stream); finding != "" {
		t.Fatal(finding)
	}
}

// responseBodyFinding reports why one captured response is refused, or "".
func responseBodyFinding(name string, got []byte, stream bool) string {
	head, body := splitResponse(got)

	if strings.TrimSpace(body) != "" {
		return ""
	}

	var tb textbuf.Buffer

	status, err := responseStatus(head)
	if err != nil {
		return tb.Str("case ").Str(name).Str(" captures no status line this check can read: ").Err(err).String()
	}

	// 204 and 304 are defined to carry no body, and a redirect carries the
	// Location header rather than a page.
	if status == 204 || status == 304 || (status >= 300 && status < 400) {
		return ""
	}

	if stream {
		return ""
	}

	return tb.Str("case ").Str(name).Str(" answers ").Int(int64(status)).
		Str(" with an empty body, so the capture would freeze a page nobody renders").String()
}

// responseStatus reads the status line Response wrote.
func responseStatus(head string) (int, error) {
	line, _, _ := strings.Cut(head, "\n")

	return strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "status:")))
}
