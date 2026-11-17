// Design: docs/architecture/container/image-store.md -- resolve: the seam's ready and absent answers

//go:build linux

package imagestore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// The references and the container the resolve tests name.
const (
	redisWhole   = "docker.io/library/redis:latest"
	testInstance = "web"
)

// TestResolveAbsentNamesReference proves the store answers a container whose
// image it does not hold with Absent, never an empty value: the whole
// reference the short name completed to, and the reason.
//
// Method: resolve a container that names the short reference "redis" in a
// store that holds nothing: a fresh data directory, and a state store that
// holds no binding and no generation. The short name completes to the
// default registry's library namespace with the tag latest.
//
// Red on the wiring stub, whose Resolve answered an error for every
// container. The store is built over its own data directory and a fake
// engine, because a store whose state cannot be read answers an error, not
// a never-fetched image (TestStoreWithoutEngineRefusesResolve).
func TestResolveAbsentNamesReference(t *testing.T) {
	store := openStore(t.TempDir(), nil)
	store.engine = newFakeEngine().link()
	answer, err := store.Resolve(context.Background(), container.ResolveRequest{Instance: "web", Reference: "redis"})
	if err != nil {
		t.Fatalf("resolve of an image the store does not hold answered an error, want Absent: %v", err)
	}
	absent, ok := answer.(container.Absent)
	if !ok {
		t.Fatalf("resolve answered %T, want container.Absent", answer)
	}
	if absent.Reference != "docker.io/library/redis:latest" {
		t.Errorf("absent reference is %q, want docker.io/library/redis:latest", absent.Reference)
	}
	if absent.Reason != container.AbsentNeverFetched {
		t.Errorf("absent reason is %d, want AbsentNeverFetched", absent.Reason)
	}
}

// resolveFixture is a store over a fresh data directory with a fake engine.
type resolveFixture struct {
	store  *Store
	engine *fakeEngine
}

func newResolveFixture(t *testing.T) resolveFixture {
	t.Helper()
	engine := newFakeEngine()
	store := openStore(t.TempDir(), nil)
	store.engine = engine.link()
	return resolveFixture{store: store, engine: engine}
}

// resolve resolves instance on text, with no volume.
func (f resolveFixture) resolve(t *testing.T, instance, text string) container.Resolution {
	t.Helper()
	answer, err := f.store.Resolve(context.Background(), container.ResolveRequest{Instance: instance, Reference: text})
	if err != nil {
		t.Fatalf("resolve %s on %s: %v", instance, text, err)
	}
	return answer
}

// ready resolves instance on text and fails the test unless it is Ready.
func (f resolveFixture) ready(t *testing.T, instance, text string) container.Ready {
	t.Helper()
	answer := f.resolve(t, instance, text)
	ready, ok := answer.(container.Ready)
	if !ok {
		t.Fatalf("resolve %s on %s answered %#v, want Ready", instance, text, answer)
	}
	return ready
}

// absent resolves testInstance on text and fails the test unless it is Absent
// for reason.
func (f resolveFixture) absent(t *testing.T, text string, reason container.AbsentReason) container.Absent {
	t.Helper()
	answer := f.resolve(t, testInstance, text)
	absent, ok := answer.(container.Absent)
	if !ok {
		t.Fatalf("resolve %s on %s answered %#v, want Absent", testInstance, text, answer)
	}
	if absent.Reason != reason {
		t.Errorf("resolve %s on %s: reason %d, want %d", testInstance, text, absent.Reason, reason)
	}
	return absent
}

// bind binds text to digest in the fake engine, as a fetch does.
func (f resolveFixture) bind(t *testing.T, text string, digest registry.Digest) {
	t.Helper()
	reference := parseTestReference(t, text)
	if err := writeBinding(context.Background(), f.engine, reference, digest); err != nil {
		t.Fatalf("bind %s: %v", text, err)
	}
}

// plant writes a published image folder holding configuration: the root,
// config.json and the completion mark. With marked false the folder holds no
// mark. It answers the digest that names the folder, the SHA-256 of the
// configuration.
func (f resolveFixture) plant(t *testing.T, configuration string, marked bool) registry.Digest {
	t.Helper()
	sum := sha256.Sum256([]byte(configuration))
	digest, err := registry.ParseDigest(digestTextPrefix + hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("digest of the configuration: %v", err)
	}
	folder := filepath.Join(imagesFolder, digestFolder(digest))
	err = f.store.withRoot(func(root *storeRoot) error {
		if err := root.root.Mkdir(folder, storeMode); err != nil {
			return err
		}
		if err := root.root.Mkdir(filepath.Join(folder, rootfsFolder), storeMode); err != nil {
			return err
		}
		if err := root.root.WriteFile(filepath.Join(folder, configName), []byte(configuration), fetchedMode); err != nil {
			return err
		}
		if !marked {
			return nil
		}
		return root.writeMark(folder, completionMark{Digest: digest.String()})
	})
	if err != nil {
		t.Fatalf("plant the image %s: %v", digest, err)
	}
	return digest
}

// remove removes the image folder of digest, as a lost disk would.
func (f resolveFixture) remove(t *testing.T, digest registry.Digest) {
	t.Helper()
	err := f.store.withRoot(func(root *storeRoot) error {
		return root.root.RemoveAll(filepath.Join(imagesFolder, digestFolder(digest)))
	})
	if err != nil {
		t.Fatalf("remove the image %s: %v", digest, err)
	}
}

func parseTestReference(t *testing.T, text string) registry.Reference {
	t.Helper()
	reference, err := registry.ParseReference(text)
	if err != nil {
		t.Fatalf("parse %s: %v", text, err)
	}
	return reference
}

// configurationJSON answers an image configuration of this system and
// architecture whose process is process.
func configurationJSON(t *testing.T, process map[string]any) string {
	t.Helper()
	architecture, err := registry.RunningArchitecture()
	if err != nil {
		t.Fatalf("running architecture: %v", err)
	}
	content, err := json.Marshal(map[string]any{"os": "linux", "architecture": architecture.String(), "config": process})
	if err != nil {
		t.Fatalf("encode the configuration: %v", err)
	}
	return string(content)
}

// redisProcess is the process of the published Redis image's configuration.
func redisProcess() map[string]any {
	return map[string]any{
		"User":         "redis:redis",
		"ExposedPorts": map[string]any{"6379/tcp": map[string]any{}},
		"Env":          []string{"PATH=/usr/local/bin:/usr/bin", "REDIS_VERSION=7.4.1"},
		"Entrypoint":   []string{"docker-entrypoint.sh"},
		"Cmd":          []string{"redis-server"},
		"Volumes":      map[string]any{"/data": map[string]any{}},
		"WorkingDir":   "/data",
		"StopSignal":   "SIGTERM",
	}
}

// TestResolveAbsentReasons proves each absent answer names its reason: a
// fetch running, a fetch waiting, a last fetch that failed with its sentence
// and code, and a bound digest the store lost, gone or unmarked, with the
// fetch that brings it back.
//
// Method: one store; each reference is placed in the fetch queue, the failed
// rows or the bindings, and resolved.
func TestResolveAbsentReasons(t *testing.T) {
	f := newResolveFixture(t)
	f.store.fetches.running = &fetchJob{reference: parseTestReference(t, "redis"), done: make(chan struct{})}
	f.store.fetches.waiting = []*fetchJob{{reference: parseTestReference(t, "nginx"), done: make(chan struct{})}}
	const failure = "image docker.io/library/alpine:latest was not fetched: the registry holds no such tag"
	f.store.fetches.rememberFailure(container.ImageRow{
		Reference: "docker.io/library/alpine:latest", State: container.ImageStateFailed,
		Error: failure, Code: container.CodeImageNotFound,
	})

	if absent := f.absent(t, "redis", container.AbsentFetchRunning); absent.Reference != redisWhole {
		t.Errorf("running: reference %q, want %s", absent.Reference, redisWhole)
	}
	f.absent(t, "nginx", container.AbsentFetchRunning)
	failed := f.absent(t, "alpine", container.AbsentFetchFailed)
	if !strings.Contains(failed.Detail, failure) || !strings.Contains(failed.Detail, container.CodeImageNotFound) {
		t.Errorf("failed: detail %q, want the sentence and the code", failed.Detail)
	}

	gone := f.plant(t, configurationJSON(t, redisProcess()), true)
	f.remove(t, gone)
	f.bind(t, "busybox", gone)
	lost := f.absent(t, "busybox", container.AbsentImageLost)
	if !strings.Contains(lost.Detail, gone.String()) || !strings.Contains(lost.Detail, container.FixFetch("docker.io/library/busybox:latest")) {
		t.Errorf("gone: detail %q, want the digest and the fetch", lost.Detail)
	}

	unmarked := f.plant(t, configurationJSON(t, map[string]any{"Cmd": []string{"sh"}}), false)
	f.bind(t, "debian", unmarked)
	f.absent(t, "debian", container.AbsentImageLost)
	if _, found := f.engine.keys()[generationKey(testInstance)]; found {
		t.Errorf("an absent answer recorded a generation")
	}
}

// TestResolveRefusesRequest proves a request the store cannot answer is an
// operator error naming the container and its fix: no image, a reference
// that does not complete, volumes this build does not create, and a name
// that cannot be a folder.
//
// Method: resolve each request on an empty store.
func TestResolveRefusesRequest(t *testing.T) {
	f := newResolveFixture(t)
	setImage := "set container instance web image REFERENCE"
	cases := []struct {
		name    string
		request container.ResolveRequest
		problem string
		fix     string
	}{
		{"no image", container.ResolveRequest{Instance: testInstance}, "names no image", setImage},
		{"malformed", container.ResolveRequest{Instance: testInstance, Reference: "Redis"}, "is not one the store can complete", setImage},
		{"volumes", container.ResolveRequest{Instance: testInstance, Reference: "redis", Volumes: []container.VolumeRequest{{Name: "data", Destination: "/data", SizeMebibytes: 64}}}, "not implemented in this build", container.FixShowDoctor},
		{"no name", container.ResolveRequest{Reference: "redis"}, "cannot name its writable layer", ""},
		{"separator", container.ResolveRequest{Instance: "a/b", Reference: "redis"}, "cannot name its writable layer", ""},
		{"dots", container.ResolveRequest{Instance: "a..b", Reference: "redis"}, "cannot name its writable layer", ""},
		{"comma", container.ResolveRequest{Instance: "a,b", Reference: "redis"}, "cannot name its writable layer", ""},
	}
	for _, c := range cases {
		answer, err := f.store.Resolve(context.Background(), c.request)
		if err == nil {
			t.Errorf("%s: answered %#v and no error", c.name, answer)
			continue
		}
		refusal, ok := errors.AsType[*opererr.Error](err)
		if !ok {
			t.Errorf("%s: %v is not an operator error", c.name, err)
			continue
		}
		if !strings.Contains(refusal.Problem(), c.problem) {
			t.Errorf("%s: problem %q, want %q", c.name, refusal.Problem(), c.problem)
		}
		if c.fix != "" && refusal.Fix() != c.fix {
			t.Errorf("%s: fix %q, want %q", c.name, refusal.Fix(), c.fix)
		}
	}
}

// TestResolveReadyHandsFoldersAndConfiguration proves the ready answer holds
// the digest, the image's root and the container's writable folder as
// absolute paths beneath the store root, the folder created with the store's
// mode, each field of the image's configuration, and the user as written
// with no account file in the root. The first start records the generation.
//
// Method: a planted image with the Redis configuration, bound to redis, is
// resolved twice for one container.
func TestResolveReadyHandsFoldersAndConfiguration(t *testing.T) {
	f := newResolveFixture(t)
	digest := f.plant(t, configurationJSON(t, redisProcess()), true)
	f.bind(t, "redis", digest)

	ready := f.ready(t, testInstance, "redis")
	wantImage := filepath.Join(f.store.path, imagesFolder, digestFolder(digest), rootfsFolder)
	wantWritable := filepath.Join(f.store.path, layersFolder, testInstance, digestFolder(digest))
	if ready.Digest != digest.String() {
		t.Errorf("digest %s, want %s", ready.Digest, digest)
	}
	if !filepath.IsAbs(f.store.path) || ready.ImageFolder != wantImage || ready.WritableFolder != wantWritable {
		t.Errorf("folders %s and %s, want %s and %s", ready.ImageFolder, ready.WritableFolder, wantImage, wantWritable)
	}
	info, err := os.Lstat(ready.WritableFolder)
	if err != nil {
		t.Fatalf("the writable folder: %v", err)
	}
	if !info.IsDir() || info.Mode().Perm() != storeMode {
		t.Errorf("the writable folder is %v, want a folder of mode %v", info.Mode(), storeMode)
	}
	want := container.Ready{
		Digest: digest.String(), ImageFolder: wantImage, WritableFolder: wantWritable,
		Entrypoint: []string{"docker-entrypoint.sh"}, Command: []string{"redis-server"},
		Environment:      []string{"PATH=/usr/local/bin:/usr/bin", "REDIS_VERSION=7.4.1"},
		WorkingDirectory: "/data", User: "redis:redis", StopSignal: "SIGTERM",
		ExposedPorts: []string{"6379/tcp"}, DeclaredVolumes: []string{"/data"},
	}
	if !readyEqual(ready, want) {
		t.Errorf("ready answer\n %#v\nwant\n %#v", ready, want)
	}
	generation, recorded, err := readGeneration(context.Background(), f.engine, testInstance)
	if err != nil || !recorded || generation.current != digest || generation.reference != redisWhole {
		t.Errorf("generation %+v (recorded %v, %v), want %s for %s", generation, recorded, err, digest, redisWhole)
	}
	if again := f.ready(t, testInstance, "redis"); !readyEqual(again, want) {
		t.Errorf("the second resolve answered %#v", again)
	}
}

// readyEqual compares two ready answers field by field.
func readyEqual(a, b container.Ready) bool {
	return a.Digest == b.Digest && a.ImageFolder == b.ImageFolder && a.WritableFolder == b.WritableFolder &&
		slices.Equal(a.Entrypoint, b.Entrypoint) && slices.Equal(a.Command, b.Command) &&
		slices.Equal(a.Environment, b.Environment) && a.WorkingDirectory == b.WorkingDirectory &&
		a.User == b.User && a.StopSignal == b.StopSignal &&
		slices.Equal(a.ExposedPorts, b.ExposedPorts) && slices.Equal(a.DeclaredVolumes, b.DeclaredVolumes) &&
		len(a.Volumes) == len(b.Volumes)
}

// TestResolveGenerationDecidesOverMovedBinding proves the recorded digest
// decides what a container runs: a re-fetch that moved the binding changes
// nothing for a container that started, while a container's first start
// takes the binding. A recorded digest the store lost is absent, with the
// fetch by that digest, and a config naming another reference starts a new
// generation on its binding.
//
// Method: two planted images A and B; redis is bound to A, then to B.
func TestResolveGenerationDecidesOverMovedBinding(t *testing.T) {
	f := newResolveFixture(t)
	first := f.plant(t, configurationJSON(t, map[string]any{"Cmd": []string{"first"}}), true)
	second := f.plant(t, configurationJSON(t, map[string]any{"Cmd": []string{"second"}}), true)
	f.bind(t, "redis", first)
	if ready := f.ready(t, testInstance, "redis"); ready.Digest != first.String() {
		t.Fatalf("first start on %s, want %s", ready.Digest, first)
	}

	f.bind(t, "redis", second)
	if ready := f.ready(t, testInstance, "redis"); ready.Digest != first.String() || !slices.Equal(ready.Command, []string{"first"}) {
		t.Errorf("after the binding moved, %s runs %s %v, want %s", testInstance, ready.Digest, ready.Command, first)
	}
	if ready := f.ready(t, "cache", "redis"); ready.Digest != second.String() {
		t.Errorf("a first start after the binding moved runs %s, want %s", ready.Digest, second)
	}

	f.remove(t, first)
	lost := f.absent(t, "redis", container.AbsentImageLost)
	if !strings.Contains(lost.Detail, container.FixFetch("docker.io/library/redis@"+first.String())) {
		t.Errorf("lost generation: detail %q, want the fetch by digest", lost.Detail)
	}

	f.bind(t, "nginx", second)
	if ready := f.ready(t, testInstance, "nginx"); ready.Digest != second.String() {
		t.Errorf("another reference runs %s, want %s", ready.Digest, second)
	}
	generation, recorded, err := readGeneration(context.Background(), f.engine, testInstance)
	if err != nil || !recorded || generation.reference != "docker.io/library/nginx:latest" || generation.current != second {
		t.Errorf("generation %+v (recorded %v, %v), want nginx on %s", generation, recorded, err, second)
	}
}

// TestUserIsHandedAsWritten proves the user reaches the answer exactly as
// the configuration wrote it: a name, a number, either with a group, and
// absent. The resolve of TestResolveReadyHandsFoldersAndConfiguration hands
// a name from a root that holds no account file, so none is opened.
//
// Method: parse a configuration holding each user.
func TestUserIsHandedAsWritten(t *testing.T) {
	architecture, err := registry.RunningArchitecture()
	if err != nil {
		t.Fatalf("running architecture: %v", err)
	}
	for _, user := range []string{"redis", "999", "999:999", "redis:redis", "redis:999", ""} {
		ready, err := parseImageConfiguration([]byte(configurationJSON(t, map[string]any{"User": user})), architecture)
		if err != nil {
			t.Errorf("user %q: %v", user, err)
			continue
		}
		if ready.User != user {
			t.Errorf("user %q handed as %q", user, ready.User)
		}
	}
}

// TestImageConfigurationRefusals proves a configuration that is not valid,
// or is of another system or architecture, is refused, never answered as
// empty fields, and that a configuration declaring no process, and lists at
// their bound, are accepted. Through resolve the refusal is an error whose
// fix removes the image folder.
//
// Method: parse each configuration; resolve one refused through a planted
// image.
func TestImageConfigurationRefusals(t *testing.T) {
	architecture, err := registry.RunningArchitecture()
	if err != nil {
		t.Fatalf("running architecture: %v", err)
	}
	ports := map[string]any{}
	for i := range imageConfigListMax + 1 {
		ports[strings.Repeat("1", 1+i%5)+"/"+strings.Repeat("u", 1+i/5)] = map[string]any{}
	}
	refused := map[string]string{
		"not JSON":             `{`,
		"another system":       `{"os":"windows","architecture":"` + architecture.String() + `"}`,
		"no system":            `{"architecture":"` + architecture.String() + `"}`,
		"another architecture": `{"os":"linux","architecture":"s390x"}`,
		"a bare variable":      configurationJSON(t, map[string]any{"Env": []string{"PATH"}}),
		"a nameless variable":  configurationJSON(t, map[string]any{"Env": []string{"=value"}}),
		"a NUL byte":           configurationJSON(t, map[string]any{"Cmd": []string{"a\x00b"}}),
		"too many variables":   configurationJSON(t, map[string]any{"Env": slices.Repeat([]string{"A=1"}, imageConfigListMax+1)}),
		"too many ports":       configurationJSON(t, map[string]any{"ExposedPorts": ports}),
	}
	for name, content := range refused {
		if ready, err := parseImageConfiguration([]byte(content), architecture); err == nil {
			t.Errorf("%s: accepted as %#v", name, ready)
		}
	}
	accepted := map[string]string{
		"no process":       `{"os":"linux","architecture":"` + architecture.String() + `"}`,
		"lists at a bound": configurationJSON(t, map[string]any{"Env": slices.Repeat([]string{"A=1"}, imageConfigListMax)}),
	}
	for name, content := range accepted {
		if _, err := parseImageConfiguration([]byte(content), architecture); err != nil {
			t.Errorf("%s: refused: %v", name, err)
		}
	}

	f := newResolveFixture(t)
	digest := f.plant(t, refused["another architecture"], true)
	f.bind(t, "redis", digest)
	answer, err := f.store.Resolve(context.Background(), container.ResolveRequest{Instance: testInstance, Reference: "redis"})
	refusal, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("resolve of another architecture answered %#v, %v, want an operator error", answer, err)
	}
	if want := "rm -rf " + filepath.Join(f.store.path, imagesFolder, digestFolder(digest)); refusal.Fix() != want {
		t.Errorf("fix %q, want %q", refusal.Fix(), want)
	}
}

// TestImageConfigurationOverBoundRefused proves a configuration of 4 MiB is
// handed to the runtime and one byte more refuses the image.
//
// Method: two planted images whose configuration is padded with spaces to
// the bound and one byte past it.
func TestImageConfigurationOverBoundRefused(t *testing.T) {
	f := newResolveFixture(t)
	base := configurationJSON(t, map[string]any{"Cmd": []string{"sh"}})
	atBound := base + strings.Repeat(" ", imageConfigSizeMax-len(base))
	f.bind(t, "redis", f.plant(t, atBound, true))
	f.bind(t, "nginx", f.plant(t, atBound+" ", true))

	if ready := f.ready(t, testInstance, "redis"); !slices.Equal(ready.Command, []string{"sh"}) {
		t.Errorf("at the bound: command %v, want [sh]", ready.Command)
	}
	if answer, err := f.store.Resolve(context.Background(), container.ResolveRequest{Instance: "cache", Reference: "nginx"}); err == nil {
		t.Errorf("one byte past the bound answered %#v and no error", answer)
	}
}

// TestHoldNamesContainerInListing proves the listing names the containers
// that hold a present image, from the hold to its release, and that the
// release of an earlier hold leaves a later hold on another digest.
//
// Method: resolve a container, hold the answer, list, then hold it again on
// another digest before the earlier release, and release both.
func TestHoldNamesContainerInListing(t *testing.T) {
	f := newResolveFixture(t)
	digest := f.plant(t, configurationJSON(t, redisProcess()), true)
	other := f.plant(t, configurationJSON(t, map[string]any{"Cmd": []string{"other"}}), true)
	f.bind(t, "redis", digest)
	usedBy := func(want registry.Digest) []string {
		t.Helper()
		rows, err := f.store.ListImages(context.Background())
		if err != nil {
			t.Fatalf("list images: %v", err)
		}
		for _, row := range rows {
			if row.Digest == want.String() {
				return row.UsedBy
			}
		}
		t.Fatalf("no row for %s in %+v", want, rows)
		return nil
	}

	release := f.store.Hold(testInstance, f.ready(t, testInstance, "redis"))
	if got := usedBy(digest); !slices.Equal(got, []string{testInstance}) {
		t.Errorf("held: used by %v, want [%s]", got, testInstance)
	}
	later := f.store.Hold(testInstance, container.Ready{Digest: other.String()})
	release()
	if got := usedBy(other); !slices.Equal(got, []string{testInstance}) {
		t.Errorf("an earlier release dropped the later hold: used by %v", got)
	}
	later()
	if got := usedBy(digest); got == nil || len(got) != 0 {
		t.Errorf("released: used by %#v, want an empty list", got)
	}
}
