package golden

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestResponseBodyFindingRefusesAnEmptyAnswer covers the refusal every handler
// capture runs.
//
// VALIDATES: a captured response that carries no body is a finding, unless its
// status is one that carries none, or the case reads a stream.
// PREVENTS: an empty HTTP 200 frozen as the expected answer. The capture
// byte-compares, so a handler that renders nothing captures nothing and passes
// for ever after. webGoldenDiffHandler called a component that no longer
// existed and one -update-golden run would have pinned its empty answer.
func TestResponseBodyFindingRefusesAnEmptyAnswer(t *testing.T) {
	const htmlHeader = "header: Content-Type: text/html; charset=utf-8"

	cases := []struct {
		name     string
		response string
		stream   bool
		refused  bool
	}{
		{
			name:     "an empty 200",
			response: "status: 200\n" + htmlHeader + "\n\n",
			refused:  true,
		},
		{
			name:     "a 200 carrying only whitespace",
			response: "status: 200\n" + htmlHeader + "\n\n  \n\t\n",
			refused:  true,
		},
		{
			name:     "an empty 500",
			response: "status: 500\n" + htmlHeader + "\n\n",
			refused:  true,
		},
		{
			name:     "an empty 404",
			response: "status: 404\n" + htmlHeader + "\n\n",
			refused:  true,
		},
		{
			name:     "a 200 with a body",
			response: "status: 200\n" + htmlHeader + "\n\n<div>a</div>",
		},
		{
			name:     "a redirect",
			response: "status: 303\nheader: Location: /\n\n",
		},
		{
			name:     "no content",
			response: "status: 204\n\n",
		},
		{
			name:     "not modified",
			response: "status: 304\n\n",
		},
		{
			name:     "a stream the client left before the first event",
			response: "status: 200\nheader: Content-Type: text/event-stream\n\n",
			stream:   true,
		},
		{
			name:     "a status this capture cannot read",
			response: "status: none\n\n",
			refused:  true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			finding := responseBodyFinding("case-name", []byte(c.response), c.stream)

			if c.refused && finding == "" {
				t.Fatalf("%q was accepted with no body", c.response)
			}

			if !c.refused && finding != "" {
				t.Fatalf("%q was refused: %s", c.response, finding)
			}

			if c.refused && !strings.Contains(finding, "case-name") {
				t.Errorf("the finding does not name the case: %s", finding)
			}
		})
	}
}

// TestVersionHeaderRequiresAVersion covers the one rewrite every captured
// response carries.
//
// VALIDATES: VersionHeader drops the value of a header version.HTTPHeader
// shaped, and leaves every other spelling of that header alone.
// PREVENTS: a server that stops sending a version, or sends an empty one,
// reading as a normalized version in every fixture of both captures.
func TestVersionHeaderRequiresAVersion(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "release build",
			header: "ze/26.04.05 (ac8f5391; go1.26; darwin/arm64)",
			want:   "header: X-Ze-Version: <VERSION>\n",
		},
		{
			name:   "modified working tree",
			header: "ze/26.04.05 (ac8f5391+; go1.26.1; linux/amd64)",
			want:   "header: X-Ze-Version: <VERSION>\n",
		},
		{
			name:   "no build commit",
			header: "ze/dev (go1.26; darwin/arm64)",
			want:   "header: X-Ze-Version: <VERSION>\n",
		},
		{
			name:   "empty value",
			header: "",
			want:   "header: X-Ze-Version: \n",
		},
		{
			name:   "no release",
			header: "ze/ (go1.26; darwin/arm64)",
			want:   "header: X-Ze-Version: ze/ (go1.26; darwin/arm64)\n",
		},
		{
			name:   "no build block",
			header: "ze/26.04.05",
			want:   "header: X-Ze-Version: ze/26.04.05\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rec.Header().Set("X-Ze-Version", c.header)

			got := string(Response(rec, []Rewrite{VersionHeader}))
			want := "status: 200\n" + c.want + "\n"

			if got != want {
				t.Errorf("Response = %q, want %q", got, want)
			}
		})
	}
}
