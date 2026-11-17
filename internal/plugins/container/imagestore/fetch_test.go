//go:build linux

// Design: docs/architecture/container/image-store.md -- the fetch, end to end against the test registry

package imagestore

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/diskspace"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
	"github.com/ze-software/ze/internal/test/ociregistry"
)

// startedName is the container the recorded notices answer as started.
const startedName = "web"

// recordedNotices records the references a fetch reports present, and
// answers that one container started for each.
type recordedNotices struct {
	mu      sync.Mutex
	present []string
}

func (n *recordedNotices) ImagePresent(_ context.Context, reference string) []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.present = append(n.present, reference)
	return []string{startedName}
}

func (n *recordedNotices) RestartOnNewGeneration(context.Context, string) error {
	return errors.New("a fetch sends no restart")
}

func (n *recordedNotices) StopThenStart(context.Context, string) error {
	return errors.New("a fetch sends no stop and start")
}

func (n *recordedNotices) presented() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Clone(n.present)
}

// fetchFixture is a test registry and a store over a fresh data directory.
type fetchFixture struct {
	registry *ociregistry.Registry
	store    *Store
	notices  *recordedNotices
	ctx      context.Context
}

// newFetchFixture starts a test registry with options and a store that
// reaches it over plain HTTP: by its own host, and as the mirror of every
// other registry, so a short name of the public hub is served by it too. The
// store's filesystem reads as 1 TiB free.
func newFetchFixture(t *testing.T, options ociregistry.Options) fetchFixture {
	t.Helper()
	server, err := ociregistry.Start(options)
	if err != nil {
		t.Fatalf("start the test registry: %v", err)
	}
	t.Cleanup(server.Close)
	notices := &recordedNotices{}
	store := openStore(t.TempDir(), notices)
	store.clientConfig = func(reference registry.Reference) (registry.Config, error) {
		config, err := defaultClientConfig(reference)
		if err != nil {
			return registry.Config{}, err
		}
		config.Transport = registry.TransportHTTP
		if reference.Registry() != server.Host() {
			config.Mirror = server.Host()
		}
		return config, nil
	}
	store.space = func(string) (diskspace.Blocks, error) {
		return diskspace.Blocks{Total: 1 << 40, Available: 1 << 40, Size: 1}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return fetchFixture{registry: server, store: store, notices: notices, ctx: ctx}
}

// testImage is one image the fixture stored: the reference that names it,
// the digest of its manifest, the digests of its layer blobs, and the bytes
// a fetch downloads for it.
type testImage struct {
	reference string
	digest    string
	layers    []string
	needed    uint64
}

// put stores an image of one file per body under repository:tag.
func (f fetchFixture) put(t *testing.T, repository, tag string, bodies ...string) testImage {
	t.Helper()
	archives := make([][]byte, 0, len(bodies))
	for i, body := range bodies {
		archives = append(archives, layerTar(t, fileEntry("file-"+strconv.Itoa(i), body, 0o644)))
	}
	return f.putLayers(t, repository, tag, archives...)
}

// putLayers stores an image of the uncompressed layers archives under
// repository:tag, with the configuration declaring their digests. An
// uncompressed layer's blob digest is its decompressed digest too.
func (f fetchFixture) putLayers(t *testing.T, repository, tag string, archives ...[]byte) testImage {
	t.Helper()
	image := testImage{reference: f.registry.Host() + "/" + repository + ":" + tag}
	layers := make([]ociregistry.Layer, 0, len(archives))
	for _, archive := range archives {
		image.layers = append(image.layers, diffDigestOf(t, archive).String())
		image.needed += uint64(len(archive))
		layers = append(layers, ociregistry.Layer{MediaType: ociregistry.MediaOCILayerTar, Content: archive})
	}
	config, err := json.Marshal(map[string]any{
		"architecture": runtime.GOARCH,
		"os":           "linux",
		"rootfs":       map[string]any{"type": "layers", "diff_ids": image.layers},
	})
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := f.registry.PutImage(repository, tag, ociregistry.Image{Architecture: runtime.GOARCH, Layers: layers, Config: config})
	if err != nil {
		t.Fatalf("store %s: %v", image.reference, err)
	}
	image.digest = descriptor.Digest
	image.needed += uint64(descriptor.Size) + uint64(len(config))
	return image
}

// blobRequests counts the requests since the first from that asked for the
// blob digest.
func (f fetchFixture) blobRequests(from int, digest string) int {
	count := 0
	for _, request := range f.registry.Requests()[from:] {
		if strings.HasSuffix(request.Path, "/blobs/"+digest) {
			count++
		}
	}
	return count
}

// rows answers the rows of the listing in state.
func (f fetchFixture) rows(t *testing.T, state container.ImageState) []container.ImageRow {
	t.Helper()
	all, err := f.store.ListImages(f.ctx)
	if err != nil {
		t.Fatalf("list the images: %v", err)
	}
	var rows []container.ImageRow
	for _, row := range all {
		if row.State == state {
			rows = append(rows, row)
		}
	}
	return rows
}

// row answers the row of reference in state, and false when there is none.
func (f fetchFixture) row(t *testing.T, reference string, state container.ImageState) (container.ImageRow, bool) {
	t.Helper()
	for _, row := range f.rows(t, state) {
		if row.Reference == reference {
			return row, true
		}
	}
	return container.ImageRow{}, false
}

// waitFor waits up to ten seconds for condition, because the worker runs on
// its own and the listing is the state the test reads.
func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("waited ten seconds for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// blobNames answers the names blobs/ holds.
func (f fetchFixture) blobNames(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(f.store.path, blobsFolder))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// blobName answers the name blobs/ keeps the blob digest under.
func blobName(t *testing.T, digest string) string {
	t.Helper()
	parsed, err := registry.ParseDigest(digest)
	if err != nil {
		t.Fatal(err)
	}
	return digestFolder(parsed)
}

// requireFailure checks err is the operator error of a failed fetch of
// reference with fix, and the listing holds its failed row with code.
func (f fetchFixture) requireFailure(t *testing.T, reference string, err error, code, fix string) {
	t.Helper()
	failure, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("fetch of %s answered %v, want an operator error", reference, err)
	}
	if failure.Fix() != fix {
		t.Errorf("fetch of %s: fix %q, want %q", reference, failure.Fix(), fix)
	}
	row, found := f.row(t, reference, container.ImageStateFailed)
	if !found {
		t.Fatalf("fetch of %s: no failed row in the listing", reference)
	}
	if row.Code != code {
		t.Errorf("failed row of %s: code %q, want %q", reference, row.Code, code)
	}
	if row.Error != failure.Problem() {
		t.Errorf("failed row of %s: error %q, want the problem %q", reference, row.Error, failure.Problem())
	}
}

// TestFetchCommandAnswersResultAtEnd proves a fetch stores the image under
// the digest of its manifest and answers the reference, the digest, the
// size, that it is new and the containers it started.
//
// Method: a registry with the hub's anonymous bearer challenge serves one
// image. The answer, the runtime's notice, the listing row, the image folder
// (root, manifest, configuration, mark) and the empty staging and blobs are
// each checked.
func TestFetchCommandAnswersResultAtEnd(t *testing.T) {
	f := newFetchFixture(t, ociregistry.Options{Auth: ociregistry.AuthBearer})
	image := f.put(t, "test/app", "v1", "hello\n")

	answer, err := f.store.FetchImage(f.ctx, image.reference)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if answer.Reference != image.reference || answer.Digest != image.digest || !answer.Created {
		t.Fatalf("answer %+v, want %s at %s, created", answer, image.reference, image.digest)
	}
	if !slices.Equal(answer.Started, []string{startedName}) {
		t.Errorf("answer started %v, want [%s]", answer.Started, startedName)
	}
	if !slices.Equal(f.notices.presented(), []string{image.reference}) {
		t.Errorf("notices %v, want [%s]", f.notices.presented(), image.reference)
	}
	row, found := f.row(t, image.reference, container.ImageStatePresent)
	if !found {
		t.Fatal("the listing holds no present row for the fetched image")
	}
	if row.Digest != image.digest || row.Size != answer.Size || row.Size == 0 {
		t.Errorf("row %+v, want digest %s and size %d", row, image.digest, answer.Size)
	}
	folder := filepath.Join(f.store.path, imagesFolder, blobName(t, image.digest))
	requireContent(t, filepath.Join(folder, rootfsFolder), "file-0", "hello\n")
	for _, name := range []string{manifestName, configName, markName} {
		if _, err := os.Stat(filepath.Join(folder, name)); err != nil {
			t.Errorf("the image folder holds no %s: %v", name, err)
		}
	}
	staging, err := os.ReadDir(filepath.Join(f.store.path, stagingFolder))
	if err != nil || len(staging) != 0 {
		t.Errorf("staging holds %v (%v), want nothing", staging, err)
	}
	if names := f.blobNames(t); len(names) != 0 {
		t.Errorf("blobs/ holds %v after publication, want nothing", names)
	}
}

// TestFetchOfPresentDigestDownloadsNothing proves a second fetch of an image
// in the store downloads no layer and answers that it is not new.
func TestFetchOfPresentDigestDownloadsNothing(t *testing.T) {
	f := newFetchFixture(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	image := f.put(t, "test/app", "v1", "once\n")
	if _, err := f.store.FetchImage(f.ctx, image.reference); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	from := len(f.registry.Requests())

	answer, err := f.store.FetchImage(f.ctx, image.reference)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if answer.Created || answer.Digest != image.digest {
		t.Errorf("second answer %+v, want %s, not created", answer, image.digest)
	}
	if got := f.blobRequests(from, image.layers[0]); got != 0 {
		t.Errorf("the second fetch asked for the layer %d times, want 0", got)
	}
}

// TestFetchReusesCheckedBlob proves a fetch that stopped keeps the blobs it
// checked, and the next attempt asks only for the others.
//
// Method: the second layer is served wrong, so the first attempt fails under
// the digest-mismatch code with the first layer kept. With the fault gone,
// the second attempt must ask for the second layer once and the first never.
func TestFetchReusesCheckedBlob(t *testing.T) {
	f := newFetchFixture(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	image := f.put(t, "test/two", "v1", "first\n", "second\n")
	f.registry.SetFault(image.layers[1], ociregistry.Fault{Body: []byte("not the layer")})

	_, err := f.store.FetchImage(f.ctx, image.reference)
	f.requireFailure(t, image.reference, err, container.CodeImageDigestMismatch, container.FixFetch(image.reference))
	if names := f.blobNames(t); !slices.Equal(names, []string{blobName(t, image.layers[0])}) {
		t.Fatalf("blobs/ holds %v, want the first layer only", names)
	}

	f.registry.SetFault(image.layers[1], ociregistry.Fault{})
	from := len(f.registry.Requests())
	answer, err := f.store.FetchImage(f.ctx, image.reference)
	if err != nil {
		t.Fatalf("second attempt: %v", err)
	}
	if !answer.Created {
		t.Errorf("second attempt answered %+v, want created", answer)
	}
	if got := f.blobRequests(from, image.layers[0]); got != 0 {
		t.Errorf("the second attempt asked for the kept layer %d times, want 0", got)
	}
	if got := f.blobRequests(from, image.layers[1]); got != 1 {
		t.Errorf("the second attempt asked for the missing layer %d times, want 1", got)
	}
	if _, found := f.row(t, image.reference, container.ImageStateFailed); found {
		t.Error("the failed row stayed after the fetch of its reference succeeded")
	}
}

// TestFetchOfAnotherImageRemovesLeftoverBlobs proves the blobs a stopped
// fetch kept are removed when a fetch of another image starts.
func TestFetchOfAnotherImageRemovesLeftoverBlobs(t *testing.T) {
	f := newFetchFixture(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	first := f.put(t, "test/first", "v1", "kept\n", "broken\n")
	f.registry.SetFault(first.layers[1], ociregistry.Fault{Body: []byte("not the layer")})
	if _, err := f.store.FetchImage(f.ctx, first.reference); err == nil {
		t.Fatal("the fetch of a wrong layer succeeded")
	}
	if names := f.blobNames(t); len(names) != 1 {
		t.Fatalf("blobs/ holds %v after the stopped fetch, want its one checked layer", names)
	}

	other := f.put(t, "test/other", "v1", "other\n")
	if _, err := f.store.FetchImage(f.ctx, other.reference); err != nil {
		t.Fatalf("fetch of another image: %v", err)
	}
	if names := f.blobNames(t); len(names) != 0 {
		t.Errorf("blobs/ holds %v after another image published, want nothing", names)
	}
}

// TestFreeSpaceRefusalBeforeAnyBlob proves a fetch that would leave less than
// the reserve free is refused before any layer is asked for, and that one
// leaving exactly the reserve proceeds.
//
// Method: the reserve is the larger of 64 MiB and 5% of the filesystem, so
// a 1 GiB filesystem keeps 64 MiB and a 100 GiB one 5 GiB. On a 1 GiB
// filesystem the free space is set one byte under the reserve plus what the
// fetch downloads, then to exactly that.
func TestFreeSpaceRefusalBeforeAnyBlob(t *testing.T) {
	if got := storeReserve(1 << 30); got != reserveMin {
		t.Errorf("reserve of 1 GiB: %d, want %d", got, reserveMin)
	}
	if got := storeReserve(100 << 30); got != 5<<30 {
		t.Errorf("reserve of 100 GiB: %d, want %d", got, uint64(5<<30))
	}
	f := newFetchFixture(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	image := f.put(t, "test/space", "v1", "space\n")
	total := uint64(1 << 30)
	reserve := storeReserve(total)
	f.store.space = func(string) (diskspace.Blocks, error) {
		return diskspace.Blocks{Total: total, Available: reserve + image.needed - 1, Size: 1}, nil
	}
	from := len(f.registry.Requests())

	_, err := f.store.FetchImage(f.ctx, image.reference)
	f.requireFailure(t, image.reference, err, container.CodeFetchNoSpace, container.FixDeleteAllImages)
	for _, number := range []uint64{image.needed, reserve + image.needed - 1, reserve} {
		if !strings.Contains(err.Error(), strconv.FormatUint(number, 10)) {
			t.Errorf("the refusal %q does not name %d", err.Error(), number)
		}
	}
	if got := f.blobRequests(from, image.layers[0]); got != 0 {
		t.Errorf("the refused fetch asked for the layer %d times, want 0", got)
	}

	f.store.space = func(string) (diskspace.Blocks, error) {
		return diskspace.Blocks{Total: total, Available: reserve + image.needed, Size: 1}, nil
	}
	if _, err := f.store.FetchImage(f.ctx, image.reference); err != nil {
		t.Fatalf("a fetch that leaves exactly the reserve was refused: %v", err)
	}
}

// TestListingShowsFetchingWithBytesReceived proves a fetch in flight is a row
// of the listing, fetching, with the bytes it received, and that a fetch that
// outlives its command's answer still ends present.
//
// Method: the layer is held after 1000 bytes, and the command waits one
// millisecond, so it answers that the fetch is still running. The listing
// must show the row fetching with exactly 1000 bytes received; once the
// layer is released, the row must read present under the image's digest.
func TestListingShowsFetchingWithBytesReceived(t *testing.T) {
	const holdAfter = 1000
	f := newFetchFixture(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	image := f.put(t, "test/held", "v1", strings.Repeat("held\n", 2000))
	release := make(chan struct{})
	f.registry.SetFault(image.layers[0], ociregistry.Fault{Hold: release, HoldAfter: holdAfter})
	f.store.answerWait = time.Millisecond

	_, err := f.store.FetchImage(f.ctx, image.reference)
	failure, ok := errors.AsType[*opererr.Error](err)
	if !ok || failure.Fix() != container.FixShowImageList {
		close(release)
		t.Fatalf("a fetch past its answer wait answered %v, want the operator error naming the listing", err)
	}
	waitFor(t, "the row fetching with 1000 bytes received", func() bool {
		row, found := f.row(t, image.reference, container.ImageStateFetching)
		return found && row.Received == holdAfter
	})
	close(release)
	waitFor(t, "the row present", func() bool {
		row, found := f.row(t, image.reference, container.ImageStatePresent)
		return found && row.Digest == image.digest
	})
	if rows := f.rows(t, container.ImageStateFetching); len(rows) != 0 {
		t.Errorf("fetching rows %+v after the fetch ended, want none", rows)
	}
}

// TestFetchQueueBound proves one fetch runs, eight wait, and a ninth that
// would wait is refused.
//
// Method: ten tags name one image whose layer is held, and the command waits
// one millisecond. The first nine fetches are accepted and listed fetching;
// the tenth is refused, naming the queue, with no row. Once released, every
// accepted fetch ends.
func TestFetchQueueBound(t *testing.T) {
	f := newFetchFixture(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	var images []testImage
	for i := range fetchWaitingMax + 2 {
		images = append(images, f.put(t, "test/queue", "t"+strconv.Itoa(i), strings.Repeat("queue\n", 500)))
	}
	release := make(chan struct{})
	f.registry.SetFault(images[0].layers[0], ociregistry.Fault{Hold: release, HoldAfter: 100})
	f.store.answerWait = time.Millisecond
	defer func() {
		close(release)
		waitFor(t, "every accepted fetch to end", func() bool { return len(f.rows(t, container.ImageStateFetching)) == 0 })
	}()

	for _, image := range images[:fetchWaitingMax+1] {
		if _, err := f.store.FetchImage(f.ctx, image.reference); err == nil {
			t.Fatalf("fetch of %s answered before its held layer arrived", image.reference)
		}
	}
	if rows := f.rows(t, container.ImageStateFetching); len(rows) != fetchWaitingMax+1 {
		t.Fatalf("%d fetching rows, want %d", len(rows), fetchWaitingMax+1)
	}
	refused := images[fetchWaitingMax+1]
	_, err := f.store.FetchImage(f.ctx, refused.reference)
	failure, ok := errors.AsType[*opererr.Error](err)
	if !ok || !strings.Contains(failure.Problem(), "eight fetches already wait") {
		t.Fatalf("the ninth waiting fetch answered %v, want the queue's refusal", err)
	}
	if _, found := f.row(t, refused.reference, container.ImageStateFetching); found {
		t.Error("the refused fetch is listed fetching")
	}
}

// TestFetchFailureClassesNameTheirCode proves each class of failure gives
// its doctor code in the failed row and its fix in the error.
func TestFetchFailureClassesNameTheirCode(t *testing.T) {
	closed, err := ociregistry.Start(ociregistry.Options{Auth: ociregistry.AuthNone})
	if err != nil {
		t.Fatal(err)
	}
	closedHost := closed.Host()
	closed.Close()

	tests := []struct {
		name  string
		auth  ociregistry.Auth
		setup func(t *testing.T, f fetchFixture) string
		code  string
		fix   func(f fetchFixture, reference string) string
	}{
		{
			name:  "not found",
			auth:  ociregistry.AuthNone,
			setup: func(_ *testing.T, f fetchFixture) string { return f.registry.Host() + "/test/absent:v1" },
			code:  container.CodeImageNotFound,
			fix:   func(fetchFixture, string) string { return container.FixFetch("REFERENCE") },
		},
		{
			name: "refused entry",
			auth: ociregistry.AuthNone,
			setup: func(t *testing.T, f fetchFixture) string {
				return f.putLayers(t, "test/device", "v1", layerTar(t, deviceEntry("etc/null", tar.TypeChar))).reference
			},
			code: container.CodeImageRefused,
			fix:  func(fetchFixture, string) string { return container.FixFetch("REFERENCE") },
		},
		{
			name:  "auth refused",
			auth:  ociregistry.AuthBasic,
			setup: func(_ *testing.T, f fetchFixture) string { return f.registry.Host() + "/test/private:v1" },
			code:  container.CodeRegistryAuthRefused,
			fix:   func(f fetchFixture, _ string) string { return container.FixRegistryCredentials(f.registry.Host()) },
		},
		{
			name:  "unreachable",
			auth:  ociregistry.AuthNone,
			setup: func(_ *testing.T, _ fetchFixture) string { return closedHost + "/test/app:v1" },
			code:  container.CodeRegistryUnreachable,
			fix:   func(_ fetchFixture, reference string) string { return container.FixFetch(reference) },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFetchFixture(t, ociregistry.Options{Auth: test.auth, Username: "user", Password: "secret"})
			reference := test.setup(t, f)
			_, err := f.store.FetchImage(f.ctx, reference)
			f.requireFailure(t, reference, err, test.code, test.fix(f, reference))
		})
	}
}

// TestFetchShortNameRecordsResolvedDigest proves a short name with no tag
// completes to the hub's library namespace and latest, and that the short
// and the whole spelling name one image.
//
// Method: the test registry serves library/redis:latest as the hub's mirror.
// The binding of the reference waits for the state keys of the generations
// phase; the completion mark records the whole reference.
func TestFetchShortNameRecordsResolvedDigest(t *testing.T) {
	const whole = "docker.io/library/redis:latest"
	f := newFetchFixture(t, ociregistry.Options{Auth: ociregistry.AuthBearer})
	image := f.put(t, "library/redis", "latest", "redis\n")

	answer, err := f.store.FetchImage(f.ctx, "redis")
	if err != nil {
		t.Fatalf("fetch redis: %v", err)
	}
	if answer.Reference != whole || answer.Digest != image.digest {
		t.Fatalf("answer %+v, want %s at %s", answer, whole, image.digest)
	}
	row, found := f.row(t, whole, container.ImageStatePresent)
	if !found || row.Digest != image.digest {
		t.Fatalf("listing row %+v (found %v), want %s at %s", row, found, whole, image.digest)
	}
	again, err := f.store.FetchImage(f.ctx, "docker.io/library/redis")
	if err != nil {
		t.Fatalf("fetch the whole spelling: %v", err)
	}
	if again.Created || again.Digest != image.digest || again.Reference != whole {
		t.Errorf("whole spelling answered %+v, want %s at %s, not created", again, whole, image.digest)
	}
}
