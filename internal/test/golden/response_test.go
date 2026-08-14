package golden

import (
	"net/http/httptest"
	"testing"
)

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
