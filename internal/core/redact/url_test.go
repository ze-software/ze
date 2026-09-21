// Design: docs/architecture/ssh/fixit-bcrypt-hash-credential.md -- credential-token redaction for logs
// Related: redact.go -- URL, URLError

package redact

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

// The surfaces that call these two are proven at their own entry points
// (internal/component/config/system, internal/install/disk). These cases pin
// the shapes those tests cannot reach: the fail-closed answer for a string
// that does not parse, and the bare-token userinfo that net/http's own
// stripPassword leaves whole.
func TestURL(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "no userinfo is unchanged", input: "https://example.net/v.json", want: "https://example.net/v.json"},
		{name: "a user and password go whole", input: "https://operator:pw@example.net/v.json", want: "https://<redacted>@example.net/v.json"},
		{name: "a bare token goes too", input: "https://tokenvalue@example.net/v.json", want: "https://<redacted>@example.net/v.json"},
		{name: "a user with an empty password goes", input: "http://operator:@127.0.0.1:8080/v", want: "http://<redacted>@127.0.0.1:8080/v"},
		{name: "a query and fragment survive", input: "https://operator:pw@example.net/v?a=1#b", want: "https://<redacted>@example.net/v?a=1#b"},
		{name: "an unparsable string fails closed", input: "https://exa mple.net/\x7f", want: Placeholder},
		{name: "the empty string is unchanged", input: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := URL(tc.input); got != tc.want {
				t.Errorf("URL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestURLErrorRemovesTheURLTheWrapperCarries drives the two wrappers that put
// a URL into an error: the one net/url builds when a URL does not parse, and
// the one net/http builds for a transport failure.
func TestURLErrorRemovesTheURLTheWrapperCarries(t *testing.T) {
	const credential = "DUMMY-VALUE-NOT-A-CREDENTIAL-4471bc"

	wrapped := &url.Error{
		Op:  "Get",
		URL: "https://" + credential + "@example.net/v.json",
		Err: errors.New("connection refused"),
	}

	cleaned := URLError(wrapped).Error()
	if strings.Contains(cleaned, credential) {
		t.Errorf("the cleaned error still holds the credential: %s", cleaned)
	}
	if !strings.Contains(cleaned, "connection refused") {
		t.Errorf("the cleaned error lost the cause: %s", cleaned)
	}
	if !strings.Contains(cleaned, "example.net") {
		t.Errorf("the cleaned error lost the host: %s", cleaned)
	}
}

// TestURLErrorLeavesAnErrorWithNoURLAlone is the opposite polarity: a helper
// that answered a placeholder for every error would satisfy the test above.
func TestURLErrorLeavesAnErrorWithNoURLAlone(t *testing.T) {
	plain := errors.New("write to /dev/sda: no space left on device")
	if got := URLError(plain); !errors.Is(got, plain) {
		t.Errorf("URLError rewrote an error that carries no URL: %v", got)
	}
}
