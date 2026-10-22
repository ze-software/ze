package pin

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recorded answers each URL from a recorded upstream response, and answers
// 404 for every URL it does not hold, so no test reaches the network. A key
// ending in " auth" answers only a request carrying a bearer token.
type recorded map[string]response

func (r recorded) Get(_ context.Context, target string, header http.Header) (response, error) {
	if header.Get("Authorization") != "" {
		if answer, found := r[target+" auth"]; found {
			return answer, nil
		}
	}
	if answer, found := r[target]; found {
		return answer, nil
	}
	return response{Status: http.StatusNotFound}, nil
}

func ok(body string) response { return response{Status: http.StatusOK, Body: []byte(body)} }

// upstreamFixture is the recorded upstream for the fixture tree: Docker Hub
// behind its token challenge, ghcr.io and quay.io open, the Go proxy and the
// GitHub API.
func upstreamFixture() recorded {
	golang := "https://registry-1.docker.io/v2/library/golang/tags/list?n=1000"
	challenge := response{Status: http.StatusUnauthorized, Header: http.Header{"Www-Authenticate": []string{
		`Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/golang:pull"`}}}
	return recorded{
		golang:           challenge,
		golang + " auth": ok(`{"tags":["1.25-alpine","1.26-alpine","1.27-alpine","1.27.1-alpine","1.28rc1-alpine","1.27-trixie"]}`),
		"https://auth.docker.io/token?scope=repository%3Alibrary%2Fgolang%3Apull&service=registry.docker.io": ok(`{"token":"t"}`),
		"https://registry-1.docker.io/v2/library/alpine/tags/list?n=1000": {Status: http.StatusOK,
			Body:   []byte(`{"tags":["3.21.3","3.22.2","edge"]}`),
			Header: http.Header{"Link": []string{`</v2/library/alpine/tags/list?last=edge&n=1000>; rel="next"`}}},
		"https://registry-1.docker.io/v2/library/alpine/tags/list?last=edge&n=1000": ok(`{"tags":["3.24.3","3.24.0_rc1"]}`),
		"https://registry-1.docker.io/v2/ligato/vpp-base/tags/list?n=1000":          ok(`{"tags":["latest","25.10-release","26.06-release","26.06","master"]}`),
		"https://ghcr.io/v2/openconfig/gnmic/tags/list?n=1000":                      ok(`{"tags":["0.42.1","0.49.1","0.50.0-rc1"]}`),
		"https://quay.io/v2/frrouting/frr/tags/list?n=1000":                         ok(`{"tags":["10.3.1","10.7.1"]}`),
		"https://proxy.golang.org/github.com/osrg/gobgp/v3/@latest":                 ok(`{"Version":"v3.37.0"}`),
		"https://proxy.golang.org/github.com/osrg/gobgp/v4/@latest":                 ok(`{"Version":"v4.10.0"}`),
		"https://api.github.com/repos/lance0/rustbgpd/releases/latest":              ok(`{"tag_name":"v0.79.0"}`),
		"https://api.github.com/repos/mc36/freeRtr/commits?per_page=1": ok(`[{"sha":"96368b02da1030148d44fc1821e844deeba0a2f0",` +
			`"commit":{"committer":{"date":"2026-10-22T08:41:13Z"}}}]`),
	}
}

// VALIDATES: the pins are derived from the files (FROM images but not stage
// names or scratch, go install, git --branch, a source commit, Go string
// literals, a scenario image file), and each is judged against upstream's
// latest stable release in the pin's own shape, including a Go major version
// the pinned path's @latest never names.
// PREVENTS: a lab peer left on an old line because its pin was never compared
// with upstream, as GoBGP stayed on v3 after v4 shipped.
func TestCheckFindsEveryPinBehind(t *testing.T) {
	pins, err := Scan(filepath.Join("testdata", "tree"))
	if err != nil {
		t.Fatal(err)
	}
	result := Compare(context.Background(), newUpstream(upstreamFixture()), pins)

	want := map[string][3]string{
		"golang 1.25-alpine":                         {"image", "1.27-alpine", StateBehind},
		"github.com/osrg/gobgp/v3/cmd/gobgp v3.31.0": {"go-module", "v4.10.0", StateBehind},
		"lance0/rustbgpd v0.7.0":                     {"git-tag", "v0.79.0", StateBehind},
		"alpine 3.21.3":                              {"image", "3.24.3", StateBehind},
		"alpine 3.24.3":                              {"image", "3.24.3", StateCurrent},
		"mc36/freeRtr 6c295d8ae79c834ef631d3d21d373c335fb05328": {"source-commit", "96368b02da1030148d44fc1821e844deeba0a2f0", StateBehind},
		"quay.io/frrouting/frr 10.7.1":                          {"image", "10.7.1", StateCurrent},
		"ghcr.io/openconfig/gnmic 0.42.1":                       {"image", "0.49.1", StateBehind},
		"ligato/vpp-base latest":                                {"image", "", StateFloating},
	}
	seen := map[string]bool{}
	for _, row := range result.Rows {
		key := row.Name + " " + row.Version
		expected, found := want[key]
		if !found {
			t.Errorf("unexpected pin %s at %s:%d", key, row.File, row.Line)
			continue
		}
		seen[key] = true
		if row.Kind != expected[0] || row.State != expected[2] {
			t.Errorf("%s: kind %s state %s (%s), want %s %s", key, row.Kind, row.State, row.Detail, expected[0], expected[2])
		}
		if expected[1] != "" && row.Latest != expected[1] {
			t.Errorf("%s: latest %q, want %q", key, row.Latest, expected[1])
		}
	}
	for key := range want {
		if !seen[key] {
			t.Errorf("pin %s was not found", key)
		}
	}
	if result.Behind != 6 {
		t.Errorf("behind %d, want 6", result.Behind)
	}
	text := result.Text()
	if !strings.Contains(text, "[behind] test/interop/Dockerfile.gobgp:2 go-module") || !strings.Contains(text, "Fix: ") {
		t.Errorf("report does not render the behind rows and the fix:\n%s", text)
	}
}

// VALIDATES: an upstream that does not answer leaves the row unresolved with
// the reason, and never reads as current.
// PREVENTS: a network failure reported as a clean tree.
func TestCheckUnreachableUpstreamIsUnresolved(t *testing.T) {
	pins := []Pin{{File: "Dockerfile", Line: 1, Kind: KindImage, Name: "alpine", Version: "3.21.3"}}
	result := Compare(context.Background(), newUpstream(failing{}), pins)
	if result.Rows[0].State != StateUnresolved || !strings.Contains(result.Rows[0].Detail, "no route") {
		t.Fatalf("row %+v, want unresolved with the cause", result.Rows[0])
	}
}

type failing struct{}

func (failing) Get(context.Context, string, http.Header) (response, error) {
	return response{}, errors.New("no route to host")
}

// VALIDATES: bump rewrites the commit and date lines to upstream's latest
// default-branch commit and touches no other line.
// PREVENTS: a bump that leaves the date behind, which the build would then
// judge stale or fresh on the wrong date.
func TestBumpMovesTheSourcePin(t *testing.T) {
	tree := t.TempDir()
	path := "test/interop-rsvpte/Dockerfile.freertr"
	body, err := os.ReadFile(filepath.Join("testdata", "tree", path))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tree, filepath.Dir(path)), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, path), body, 0o600); err != nil {
		t.Fatal(err)
	}
	bumped, err := Bump(context.Background(), newUpstream(upstreamFixture()), tree, path)
	if err != nil {
		t.Fatal(err)
	}
	if !bumped.Changed || bumped.To != "96368b02da1030148d44fc1821e844deeba0a2f0" {
		t.Fatalf("bump answered %+v", bumped)
	}
	after, err := os.ReadFile(filepath.Join(tree, path))
	if err != nil {
		t.Fatal(err)
	}
	wantText := strings.Replace(strings.Replace(string(body),
		"ZE_SOURCE_COMMIT=6c295d8ae79c834ef631d3d21d373c335fb05328", "ZE_SOURCE_COMMIT=96368b02da1030148d44fc1821e844deeba0a2f0", 1),
		"ZE_SOURCE_DATE=2026-09-23T10:00:00Z", "ZE_SOURCE_DATE=2026-10-22T08:41:13Z", 1)
	if string(after) != wantText {
		t.Errorf("bumped Dockerfile:\n%s\nwant:\n%s", after, wantText)
	}

	if _, err := Bump(context.Background(), newUpstream(upstreamFixture()), tree, "test/interop/Dockerfile.none"); err == nil {
		t.Error("bump of a missing Dockerfile was accepted")
	}
}

// VALIDATES: the shape rule: a pin is compared only with tags of its own
// prefix, number count and suffix, and pre-releases never win.
func TestLatestTagKeepsThePinShape(t *testing.T) {
	cases := []struct{ pinned, want string }{
		{"1.25-alpine", "1.27-alpine"},
		{"3.21", "3.24"},
		{"v2.5.1", "v2.6.0"},
		{"latest", "3.24.3"},
	}
	tags := []string{"1.25-alpine", "1.27-alpine", "1.28-rc1-alpine", "3.21", "3.24", "3.24.3", "3.25.0-beta", "v2.5.1", "v2.6.0", "v2.7.0rc1"}
	for _, one := range cases {
		if got := latestTag(one.pinned, tags); got != one.want {
			t.Errorf("latestTag(%q) = %q, want %q", one.pinned, got, one.want)
		}
	}
}

// VALIDATES: a scan of this repository finds the real lab pins (the gnmic
// image, the tac_plus-ng and freeRtr source commits) and reads no path-with-
// a-line literal, such as "docs/one.md:4", as an image.
// PREVENTS: a report padded with rows no upstream can answer.
func TestScanTheRepository(t *testing.T) {
	pins, err := Scan(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, pin := range pins {
		found[pin.Kind.String()+" "+pin.Name] = true
		if pin.Kind == KindImage && (fileSuffix.MatchString(pin.Name) || strings.HasPrefix(pin.Name, ".")) {
			t.Errorf("%s:%d: %s:%s read as an image", pin.File, pin.Line, pin.Name, pin.Version)
		}
	}
	for _, want := range []string{"image ghcr.io/openconfig/gnmic", "source-commit MarcJHuber/event-driven-servers", "source-commit mc36/freeRtr", "image alpine"} {
		if !found[want] {
			t.Errorf("the scan did not find %s", want)
		}
	}
}
