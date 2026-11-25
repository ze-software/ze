// VALIDATES: commit secrets refuses newly added API tokens, private-key files
// and PEM blocks outside test fixtures, credential files, live secret env
// values, and local home paths, while honoring line-scoped secret-scan: allow
// markers and ignoring pre-existing unchanged lines.
// PREVENTS: accidental publication of API keys, private keys, credentials, or
// personal home directory paths through `./le commit create`.
package commitsecrets

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

func memSide(files map[string]string) changecheck.Side {
	return func(rel string) (string, bool, error) {
		text, ok := files[rel]
		return text, ok, nil
	}
}

func TestCommitSecretsCheckIsRegistered(t *testing.T) {
	found := false
	for _, check := range changecheck.Registered() {
		if check.Name == area+" check" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("changecheck %q is not registered", area+" check")
	}
}

func TestScanChangeRefusesTokenPrefixesAndHonorsAllowMarker(t *testing.T) {
	fakeAnt := "sk-" + "ant-api03-1234567890abcdefghij12345"
	fakeGH := "gh" + "p_1234567890abcdefghijklmnopqrstuvwxyz"
	change := changecheck.Change{
		Paths:  []string{"docs/example.md"},
		Before: memSide(nil),
		After: memSide(map[string]string{
			"docs/example.md": "clean line\n" +
				"key=" + fakeAnt + "\n" +
				"allowed=" + fakeGH + " // " + allowMarker + "\n",
		}),
	}
	findings, err := ScanChange(change, nil)
	if err != nil {
		t.Fatalf("ScanChange error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if findings[0].Line != 2 {
		t.Errorf("finding line = %d, want 2", findings[0].Line)
	}
	if !strings.Contains(findings[0].Err.Error(), "sk-"+"ant-") {
		t.Errorf("finding error = %q, want prefix mention", findings[0].Err.Error())
	}
}

func TestScanChangeIgnoresUnchangedPreExistingLines(t *testing.T) {
	existing := "token=" + "gh" + "p_1234567890abcdefghijklmnopqrstuvwxyz\n"
	change := changecheck.Change{
		Paths:  []string{"docs/existing.md"},
		Before: memSide(map[string]string{"docs/existing.md": existing}),
		After:  memSide(map[string]string{"docs/existing.md": existing + "new clean line\n"}),
	}
	findings, err := ScanChange(change, nil)
	if err != nil {
		t.Fatalf("ScanChange error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("pre-existing unchanged line should not fire: %+v", findings)
	}
}

func TestScanChangeRefusesForbiddenFilesAndAllowsTestPKIFixtures(t *testing.T) {
	pemBody := "-----BEGIN " + "PRIVATE KEY-----\nMIIBVQIBADANBgkqhkiG9w0BAQEF\n"
	change := changecheck.Change{
		Paths: []string{
			".env.local",
			"internal/component/foo/prod.pem",
			"internal/component/pki/testdata/ed448/leaf-key.pem",
			"test/interop-ipsec/pki/ca-key.pem",
		},
		Before: memSide(nil),
		After: memSide(map[string]string{
			".env.local":                      "FOO=bar\n",
			"internal/component/foo/prod.pem": pemBody,
			"internal/component/pki/testdata/ed448/leaf-key.pem": pemBody,
			"test/interop-ipsec/pki/ca-key.pem":                  pemBody,
		}),
	}
	findings, err := ScanChange(change, nil)
	if err != nil {
		t.Fatalf("ScanChange error: %v", err)
	}
	for _, f := range findings {
		if strings.Contains(f.Path, "testdata/") || strings.HasPrefix(f.Path, "test/") {
			t.Errorf("test PKI fixture path %q should be allowed, got: %+v", f.Path, f)
		}
	}
	sawEnv := false
	sawProdKeyPath := false
	sawProdKeyHeader := false
	for _, f := range findings {
		if f.Path == ".env.local" && f.Line == 0 {
			sawEnv = true
		}
		if f.Path == "internal/component/foo/prod.pem" && f.Line == 0 {
			sawProdKeyPath = true
		}
		if f.Path == "internal/component/foo/prod.pem" && f.Line == 1 {
			sawProdKeyHeader = true
		}
	}
	if !sawEnv || !sawProdKeyPath || !sawProdKeyHeader {
		t.Fatalf("missing expected findings (env=%v path=%v header=%v): %+v",
			sawEnv, sawProdKeyPath, sawProdKeyHeader, findings)
	}
}

func TestScanChangeRefusesLiveSecretEnvValuesAndHomePath(t *testing.T) {
	liveSecret := "supersecretvalue987654"
	fakeHome := "/home/alice-dev"
	environ := []string{
		"HOME=" + fakeHome,
		"MY_API_TOKEN=" + liveSecret,
		"GIT_AUTHOR_NAME=Alice Developer Name",
		"SSH_AUTH_SOCK=/run/user/1000/ssh.sock",
		"SHORT_KEY=abc",
	}
	change := changecheck.Change{
		Paths:  []string{"docs/notes.md"},
		Before: memSide(nil),
		After: memSide(map[string]string{
			"docs/notes.md": "author: Alice Developer Name\n" +
				"path: " + fakeHome + "/ze/main\n" +
				"leaked: " + liveSecret + "\n",
		}),
	}
	findings, err := ScanChange(change, environ)
	if err != nil {
		t.Fatalf("ScanChange error: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2 (home + live env): %+v", len(findings), findings)
	}
	if findings[0].Line != 2 || !strings.Contains(findings[0].Err.Error(), "HOME") {
		t.Errorf("first finding = %+v, want line 2 HOME path", findings[0])
	}
	if findings[1].Line != 3 || !strings.Contains(findings[1].Err.Error(), "MY_API_TOKEN") {
		t.Errorf("second finding = %+v, want line 3 MY_API_TOKEN", findings[1])
	}
}

func TestFormatFindingsMentionsAllowMarkerAndOpererrShape(t *testing.T) {
	if got := FormatFindings(nil); got != "" {
		t.Fatalf("empty findings formatted as %q, want empty", got)
	}
	out := FormatFindings([]Finding{{
		Path: "a.md",
		Line: 4,
		Err:  opererr.New("a.md:4: API token prefix detected", "line contains a token", "remove the token"),
	}})
	if !strings.Contains(out, "a.md:4") || !strings.Contains(out, allowMarker) {
		t.Errorf("unexpected formatted refusal:\n%s", out)
	}
}
