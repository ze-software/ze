package siterelease

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubForge states the forge's answer for one test: the body of each page by
// its number, or the error every page answers.
func stubForge(t *testing.T, pages []string, failure error) *[]string {
	t.Helper()
	asked := &[]string{}
	original := fetchPage
	fetchPage = func(_ context.Context, pageURL string) ([]byte, error) {
		*asked = append(*asked, pageURL)
		if failure != nil {
			return nil, failure
		}
		if len(*asked) > len(pages) {
			t.Fatalf("the update asked for page %d, the forge stated %d", len(*asked), len(pages))
		}
		return []byte(pages[len(*asked)-1]), nil
	}
	t.Cleanup(func() { fetchPage = original })
	return asked
}

// releaseCheckout answers a checkout holding website/data/, with the release
// file stating content when content is not empty.
func releaseCheckout(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "website", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if content != "" {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(Path)), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// captureStderr answers what one call wrote to stderr.
func captureStderr(t *testing.T, call func()) string {
	t.Helper()
	capture, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stderr
	os.Stderr = capture
	call()
	os.Stderr = original
	written, err := os.ReadFile(capture.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(written)
}

const isoRelease = `[
  {"tag_name":"v26.11.0","name":"Ze 26.11","draft":false,"prerelease":true,
   "published_at":"2026-11-02T10:00:00Z","html_url":"https://github.com/ze-software/ze/releases/tag/v26.11.0",
   "body":"notes the file does not keep",
   "assets":[
     {"name":"ze-20261102-100000.iso","size":128933888,
      "digest":"sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
      "browser_download_url":"https://github.com/ze-software/ze/releases/download/v26.11.0/ze-20261102-100000.iso"},
     {"name":"SHA256SUMS","size":0,"digest":null,
      "browser_download_url":"https://github.com/ze-software/ze/releases/download/v26.11.0/SHA256SUMS"}]},
  {"tag_name":"v26.12.0-draft","name":null,"draft":true,"prerelease":false,"published_at":null,
   "html_url":"https://github.com/ze-software/ze/releases/tag/untagged-1","assets":[]}
]`

const isoReleaseFile = `{
  "endpoint": "https://api.github.com/repos/ze-software/ze/releases",
  "releases": [
    {
      "assets": [
        {
          "browser_download_url": "https://github.com/ze-software/ze/releases/download/v26.11.0/SHA256SUMS",
          "digest": null,
          "name": "SHA256SUMS",
          "size": 0
        },
        {
          "browser_download_url": "https://github.com/ze-software/ze/releases/download/v26.11.0/ze-20261102-100000.iso",
          "digest": "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
          "name": "ze-20261102-100000.iso",
          "size": 128933888
        }
      ],
      "html_url": "https://github.com/ze-software/ze/releases/tag/v26.11.0",
      "name": "Ze 26.11",
      "prerelease": true,
      "published_at": "2026-11-02T10:00:00Z",
      "tag_name": "v26.11.0"
    }
  ]
}
`

// VALIDATES: AC-10, AC-11, wiring. The update writes what the forge's releases
// list states, in the committed form the site build reads.
//
// The forge is stubbed at the page reach, so the test drives the command's
// own update (the body of `./le site release update`) and reads the file back
// byte for byte, then through Read, the site build's reader. The empty list
// is the project's state today, and it commits as "no release yet", not as a
// failure. The release case keeps every field the snapshot reads, keeps a
// zero-sized asset, drops the body and the draft, and two runs over one
// answer write the same bytes.
func TestReleaseUpdateCommitsTheReleasesTheForgeStates(t *testing.T) {
	t.Run("no release yet", func(t *testing.T) {
		root := releaseCheckout(t, "")
		asked := stubForge(t, []string{"[]"}, nil)
		answer, code := update(root)
		if code != 0 {
			t.Fatalf("update answered %d", code)
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Path)))
		if err != nil {
			t.Fatal(err)
		}
		want := "{\n  \"endpoint\": \"" + Endpoint + "\",\n  \"releases\": []\n}\n"
		if string(content) != want {
			t.Errorf("the file reads\n%s\nwant\n%s", content, want)
		}
		if len(*asked) != 1 || !strings.HasPrefix((*asked)[0], Endpoint+"?") {
			t.Errorf("the update asked %v, want one page of %s", *asked, Endpoint)
		}
		if report, isWritten := answer.(written); !isWritten || report.Releases != 0 || report.Latest != "" {
			t.Errorf("the update answered %#v", answer)
		}
		file, err := Read(filepath.Join(root, "website"))
		if err != nil {
			t.Fatal(err)
		}
		if _, published := file.Latest(); published {
			t.Error("an empty list read back with a latest release")
		}
	})

	t.Run("a release with an ISO and another asset", func(t *testing.T) {
		root := releaseCheckout(t, "")
		stubForge(t, []string{isoRelease}, nil)
		if _, code := update(root); code != 0 {
			t.Fatalf("update answered %d", code)
		}
		path := filepath.Join(root, filepath.FromSlash(Path))
		first, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != isoReleaseFile {
			t.Errorf("the file reads\n%s\nwant\n%s", first, isoReleaseFile)
		}

		stubForge(t, []string{isoRelease}, nil)
		if _, code := update(root); code != 0 {
			t.Fatalf("the second update answered %d", code)
		}
		second, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(second, first) {
			t.Error("two updates over one forge answer wrote different bytes")
		}

		file, err := Read(filepath.Join(root, "website"))
		if err != nil {
			t.Fatal(err)
		}
		latest, published := file.Latest()
		if !published || latest.TagName != "v26.11.0" || len(latest.Assets) != 2 {
			t.Fatalf("read back %#v", file)
		}
		if sum, stated := latest.Assets[1].SHA256(); !stated || sum != "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08" {
			t.Errorf("the ISO digest reads %q", sum)
		}
	})

	t.Run("a full page asks for the next", func(t *testing.T) {
		root := releaseCheckout(t, "")
		entries := make([]string, 0, pageEntries)
		for range pageEntries {
			entries = append(entries, `{"tag_name":"v1","draft":true,"assets":[]}`)
		}
		asked := stubForge(t, []string{"[" + strings.Join(entries, ",") + "]", "[]"}, nil)
		if _, code := update(root); code != 0 {
			t.Fatalf("update answered %d", code)
		}
		if len(*asked) != 2 || !strings.HasSuffix((*asked)[1], "&page=2") {
			t.Errorf("the update asked %v, want pages 1 and 2", *asked)
		}
	})
}

// VALIDATES: AC-12, the size boundary and the URL check of the security
// review. Whatever stops the update leaves the committed file byte for byte,
// exits non-zero, and names the endpoint, the cause and the retry command.
//
// Each case starts from a committed file and a forge that fails or answers
// something the file must not hold: an unreachable forge, a negative size (the
// boundary below zero; zero is accepted above), a download URL on another
// host, a page URL over plain http, and a digest that is not sha256.
func TestReleaseUpdateLeavesTheFileWhenTheForgeIsUnreachable(t *testing.T) {
	asset := func(name, size, link, digest string) string {
		return `[{"tag_name":"v1","name":"One","draft":false,"prerelease":false,"published_at":"2026-11-02T10:00:00Z",
			"html_url":"https://github.com/ze-software/ze/releases/tag/v1","assets":[{"name":"` + name + `","size":` + size +
			`,"digest":` + digest + `,"browser_download_url":"` + link + `"}]}]`
	}
	download := "https://github.com/ze-software/ze/releases/download/v1/ze.iso"
	for _, refusal := range []struct {
		what    string
		page    string
		failure error
		cause   string
	}{
		{"an unreachable forge", "", errors.New("dial tcp: lookup api.github.com: no such host"), "no such host"},
		{"a negative asset size", asset("ze.iso", "-1", download, "null"), nil, "size -1 is negative"},
		{"a download on another host", asset("ze.iso", "1", "https://example.com/ze.iso", "null"), nil, "example.com"},
		{"a release page over http", strings.Replace(asset("ze.iso", "1", download, "null"), "https://github.com/ze-software/ze/releases/tag", "http://github.com/ze-software/ze/releases/tag", 1), nil, "not an https URL"},
		{"a digest that is not sha256", asset("ze.iso", "1", download, `"md5:00"`), nil, "not a sha256 digest"},
	} {
		t.Run(refusal.what, func(t *testing.T) {
			root := releaseCheckout(t, isoReleaseFile)
			stubForge(t, []string{refusal.page}, refusal.failure)
			var code int
			said := captureStderr(t, func() { _, code = update(root) })
			if code == 0 {
				t.Fatal("the update answered 0")
			}
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Path)))
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != isoReleaseFile {
				t.Errorf("the committed file changed to\n%s", content)
			}
			for _, phrase := range []string{Endpoint, refusal.cause, "./le site release update", Path + " is unchanged"} {
				if !strings.Contains(said, phrase) {
					t.Errorf("the refusal does not say %q:\n%s", phrase, said)
				}
			}
		})
	}
}
