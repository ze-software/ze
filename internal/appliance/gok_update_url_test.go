// Design: docs/architecture/appliance/builder.md -- updater identity handling.

package appliance

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gokrazy/gokapi"
	"github.com/gokrazy/gokapi/ondeviceapi"
)

// These are fixed non-secret fixture values. The first exercises URL authority
// delimiters; the second requires a literal percent escape to survive decoding.
const gokURLFixturePassword = "fixture/+?@:#=token"

// TestGokUpdaterURLPreservesReservedPassword exercises the shared upstream
// BaseURL producer through its existing exported API, without an internal import.
func TestGokUpdaterURLPreservesReservedPassword(t *testing.T) {
	for _, password := range []string{gokURLFixturePassword, "fixture%2Ftoken", "plain-fixture"} {
		t.Run(password, func(t *testing.T) {
			configuration, err := connectGokURLFixture(t, gokapi.ConnectRemotely, password, "2080")
			if err != nil {
				t.Fatalf("updater URL rejected fixture password: %v", err)
			}
			parsed, err := url.Parse(configuration.BasePath)
			if err != nil {
				t.Fatalf("updater API returned an invalid URL: %v", err)
			}
			if parsed.Host != "fixture.example:2080" {
				t.Fatalf("password changed update authority to %q", parsed.Host)
			}
			if parsed.Scheme != "http" {
				t.Fatalf("updater scheme = %q, want http", parsed.Scheme)
			}
			if parsed.User == nil {
				t.Fatal("updater URL omitted authentication")
			}
			if parsed.User.Username() != "gokrazy" {
				t.Fatalf("updater username = %q, want gokrazy", parsed.User.Username())
			}
			got, present := parsed.User.Password()
			if !present {
				t.Fatal("updater URL omitted its password")
			}
			if got != password {
				t.Fatalf("password changed during URL round trip: got %q, want %q", got, password)
			}
			if parsed.RawQuery != "" {
				t.Fatalf("password became a query: %q", parsed.RawQuery)
			}
			if parsed.Fragment != "" {
				t.Fatalf("password became a fragment: %q", parsed.Fragment)
			}
		})
	}
}

// TestGokUpdaterURLFailureOmitsPassword requires host/port validation to fail
// before credentials enter the parse input that net/url includes in errors.
func TestGokUpdaterURLFailureOmitsPassword(t *testing.T) {
	_, err := connectGokURLFixture(t, gokapi.ConnectRemotely, gokURLFixturePassword, "invalid-port")
	if err == nil {
		t.Fatal("invalid update port was accepted")
	}
	if strings.Contains(err.Error(), gokURLFixturePassword) {
		t.Fatalf("updater URL error exposed the fixture password: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid-port") {
		t.Fatalf("updater URL error lost the invalid port: %v", err)
	}
}

// connectGokURLFixture infers the private config type from the exported API.
// JSON is its normal external representation; no forbidden internal import or
// reflection is needed to exercise the real shared URL producer.
func connectGokURLFixture[T any](t *testing.T, connect func(*T) (*ondeviceapi.Configuration, error), password, port string) (*ondeviceapi.Configuration, error) {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"Hostname": "fixture.example",
		"Update": map[string]string{
			"Hostname": "fixture.example", "HTTPPassword": password,
			"HTTPPort": port, "HTTPSPort": "443",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var configuration T
	if err := json.Unmarshal(data, &configuration); err != nil {
		t.Fatal(err)
	}
	return connect(&configuration)
}

// TestGokBuildFailureHidesUpdaterPassword checks the actual CLI diagnostic,
// including opaque upstream errors that no longer expose a typed URL cause.
func TestGokBuildFailureHidesUpdaterPassword(t *testing.T) {
	encoded := strings.TrimPrefix(url.UserPassword("", gokURLFixturePassword).String(), ":")
	for _, test := range []struct {
		name    string
		failure error
		cause   string
	}{
		{"raw URL parse error", &url.Error{Op: "parse", URL: "https://gokrazy:" + gokURLFixturePassword + "@fixture.example/", Err: errors.New("invalid userinfo")}, "invalid userinfo"},
		{"opaque encoded URL", errors.New("cannot contact https://gokrazy:" + encoded + "@fixture.example/: connection refused"), "connection refused"},
		{"ordinary disk failure", errors.New("write image: no space left on device"), "no space left on device"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := writeGokrazyFixture(t)
			cfg := &applianceConfig{Image: ImageConfig{Arch: runtime.GOARCH, SizeBytes: 1073741824}}
			update := &gokUpdate{HTTPPassword: gokURLFixturePassword, HTTPSPort: "443", cert: []byte("fixture-certificate"), key: []byte("fixture-key")}
			previous := gokBuildFn
			t.Cleanup(func() { gokBuildFn = previous })
			called := false
			gokBuildFn = func(args []string) error {
				called = true
				data, err := os.ReadFile(filepath.Join(args[1], "ze", "config.json"))
				if err != nil {
					t.Fatal(err)
				}
				var prepared struct{ Update struct{ HTTPPassword string } }
				if err := json.Unmarshal(data, &prepared); err != nil {
					t.Fatal(err)
				}
				if prepared.Update.HTTPPassword != gokURLFixturePassword {
					t.Fatal("URL escaping changed the raw password stored in Update")
				}
				return test.failure
			}
			var code buildOutcome
			output := captureStderr(t, func() {
				code = runGokBuild(cfg, filepath.Join(root, "fixture.img"), update)
			})
			if !called {
				t.Fatalf("fixture never reached the embedded builder: %s", output)
			}
			if code != exitError {
				t.Fatalf("failed image build returned %d, want failure", code)
			}
			for _, secret := range []string{gokURLFixturePassword, encoded} {
				if strings.Contains(output, secret) {
					t.Fatalf("build diagnostic exposed the fixture password: %s", output)
				}
			}
			if !strings.Contains(output, test.cause) {
				t.Fatalf("build diagnostic lost the failure cause %q: %s", test.cause, output)
			}
			if !strings.HasSuffix(strings.TrimSpace(output), "fix: ./ze-host appliance --dir "+shellArg(filepath.Dir(root))+" build "+shellArg(filepath.Base(root))) {
				t.Fatalf("build diagnostic lost the command to retry this appliance: %s", output)
			}
		})
	}
}
