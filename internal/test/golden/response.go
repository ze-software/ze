// Design: (none -- test utility, no architecture doc)

package golden

import (
	"bytes"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"
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
