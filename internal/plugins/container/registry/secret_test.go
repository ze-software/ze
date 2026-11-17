// Design: docs/architecture/container/image-store.md -- no credential in an error or a log line
// Related: errors.go -- the error text under test
// Related: client.go -- Credentials, which print as a placeholder

package registry

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// TestCredentialsNeverInErrorOrLog proves no error text, no part of an
// Error, no format of a Config and no log line holds the password, its
// basic encoding or the token, across the failures a credentialed fetch
// meets: credentials the realm refuses, a blob that does not match after
// the token was granted, a redirect that fails after it, and a basic
// registry that refuses the credentials. Method: every log line of the run
// is captured, and each text is searched for each secret.
func TestCredentialsNeverInErrorOrLog(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	const password = "never-shown-pass"
	bearer := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthBearer, Username: "operator", Password: password})
	_, layerDescriptor := putImage(t, bearer, "app", "1", []byte("secret layer"))
	basic := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthBasic, Username: "operator", Password: "other-pass"})
	putImage(t, basic, "app", "1", []byte("basic layer"))
	ref := mustReference(t, bearer.Host()+"/app:1")
	secrets := []string{
		password,
		base64.StdEncoding.EncodeToString([]byte("operator:" + password)),
		bearer.Token(),
	}

	var failures []error
	refusedRealm := newClient(t, Config{Registry: bearer.Host(), Credentials: NewCredentials("operator", password+"-wrong")})
	_, err := refusedRealm.Resolve(t.Context(), ref)
	failures = append(failures, err)

	granted := newClient(t, Config{Registry: bearer.Host(), Credentials: NewCredentials("operator", password)})
	if _, err := granted.Resolve(t.Context(), ref); err != nil {
		t.Fatal(err)
	}
	bearer.SetFault(layerDescriptor.Digest, ociregistry.Fault{Body: []byte("not the layer")})
	_, err = fetchToFile(t, t.Context(), granted, ref, blobOf(t, layerDescriptor))
	failures = append(failures, err)
	bearer.SetFault(layerDescriptor.Digest, ociregistry.Fault{RedirectTo: "http://127.0.0.1:1/v2/app/blobs/" + layerDescriptor.Digest})
	_, err = fetchToFile(t, t.Context(), granted, ref, blobOf(t, layerDescriptor))
	failures = append(failures, err)

	refusedBasic := newClient(t, Config{Registry: basic.Host(), Credentials: NewCredentials("operator", password)})
	_, err = refusedBasic.Resolve(t.Context(), mustReference(t, basic.Host()+"/app:1"))
	failures = append(failures, err)

	texts := []string{
		fmt.Sprint(Config{Credentials: NewCredentials("operator", password)}),
		fmt.Sprintf("%+v", Config{Credentials: NewCredentials("operator", password)}),
		fmt.Sprintf("%#v", Config{Credentials: NewCredentials("operator", password)}),
		logs.String(),
	}
	for i, failure := range failures {
		if failure == nil {
			t.Fatalf("failure %d did not fail, so the test proves nothing", i)
		}
		texts = append(texts, failure.Error())
		if registryError, ok := errors.AsType[*Error](failure); ok {
			texts = append(texts, registryError.Problem(), registryError.Why())
		}
	}
	for _, text := range texts {
		for _, secret := range secrets {
			if strings.Contains(text, secret) {
				t.Errorf("a secret reached the text %q", text)
			}
		}
	}
}
