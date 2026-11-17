// Design: docs/functional-tests.md -- the seed of the published image the container tests fetch
// Related: register.go -- the le command that runs the seed
// Related: ../../../test/ociregistry/layout.go -- the layout folder the seed writes and the test registry serves

// Package testcontainerseed copies the published Redis image the test tree
// pins into an OCI layout folder of the developer cache, so the container
// tests fetch the real image from a registry on the loopback and need no
// internet when they run. It is the only step of those tests that does.
//
// It speaks the pull subset of the distribution protocol with the standard
// library, and does not use Ze's registry client: that client resolves a
// reference to the one image of the running platform and never hands back the
// index it read, and the layout must hold the pinned index byte for byte.
package testcontainerseed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/test/ociregistry"
)

const (
	// wordFetch is the one action of the command.
	wordFetch = "fetch"
	// seedCommand is the line that runs the seed, as an error names it. The
	// registry fixture names it too, so the test registry package declares it
	// and TestSeedCommandIsTheRegisteredOne holds it to this command.
	seedCommand = ociregistry.SeedCommand

	usageText = "usage: le test container-seed fetch\n\n" +
		"Copies the Redis image pinned in test/interop-container/redis.pin, for\n" +
		"linux/amd64 and linux/arm64, from its public registry into an OCI layout\n" +
		"folder of the developer cache (cache/container-seed/redis in the checkout).\n" +
		"Every object is checked against its digest and size, and an object already\n" +
		"there and correct is not fetched again. It is the only step of the container\n" +
		"tests that needs the network.\n"

	// hubName is how a reference names the Docker Hub, and hubEndpoint is
	// where its registry answers.
	hubName     = "docker.io"
	hubEndpoint = "https://registry-1.docker.io"

	// seedTag is the tag the layout records the pinned index under.
	seedTag = "latest"
	// annotationImageName is the annotation that names the whole reference of
	// a layout entry, as containerd writes it.
	annotationImageName = "io.containerd.image.name"

	osLinux         = "linux"
	digestAlgorithm = "sha256:"
	partialSuffix   = ".partial"
	bearerScheme    = "Bearer "

	// sizeUnknown is the size of the pinned index before it is fetched: the
	// pin names its digest only.
	sizeUnknown int64 = -1

	manifestSizeMax = 4 << 20
	tokenSizeMax    = 1 << 20

	// acceptManifests is the Accept header of a manifest request.
	acceptManifests = ociregistry.MediaOCIIndex + ", " + ociregistry.MediaOCIManifest +
		", application/vnd.docker.distribution.manifest.list.v2+json" +
		", application/vnd.docker.distribution.manifest.v2+json"

	seedTimeout           = 30 * time.Minute
	responseHeaderTimeout = 30 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
)

// seedArchitectures are the platforms the seed copies, each under linux.
var seedArchitectures = [...]string{"amd64", "arm64"}

// helpWords are the words that ask for the usage.
var helpWords = [...]string{"help", "--help", "-h"}

// run is the command: the one word fetch runs the seed.
func run(args []string) int {
	if len(args) != 1 {
		os.Stderr.WriteString(usageText) //nolint:errcheck // stderr is the report
		return 2
	}
	if slices.Contains(helpWords[:], args[0]) {
		os.Stdout.WriteString(usageText) //nolint:errcheck // stdout is the report
		return 0
	}
	if args[0] != wordFetch {
		os.Stderr.WriteString(usageText) //nolint:errcheck // stderr is the report
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, seedTimeout)
	defer cancel()
	if err := seedCheckout(ctx, os.Stdout); err != nil {
		os.Stderr.WriteString("container-seed: " + err.Error() + "\n" + //nolint:errcheck // stderr is the report
			"  The container tests that fetch the published image need this copy.\n" +
			"  Run it again once the cause is gone; it keeps every object already checked:\n" +
			"    " + seedCommand + "\n")
		return 1
	}
	return 0
}

// seedCheckout seeds the layout of the checkout this process works in from
// the pin of its test tree.
func seedCheckout(ctx context.Context, out io.Writer) error {
	root, err := lepath.Root()
	if err != nil {
		return err
	}
	pinPath := filepath.Join(root, "test", "interop-container", "redis.pin")
	text, err := os.ReadFile(pinPath) //nolint:gosec // G304: the pin of this checkout
	if err != nil {
		return err
	}
	pinned, err := parsePin(string(text))
	if err != nil {
		return fmt.Errorf("%s: %w", pinPath, err)
	}
	return seed(ctx, pinned, ociregistry.SeedLayoutDir(root), out)
}

// pin is the image the seed copies: where its registry answers, how a
// reference names that registry, its repository and the digest of its index.
type pin struct {
	endpoint   string
	registry   string
	repository string
	digest     string
}

// parsePin reads the pin file: one line, <registry>/<repository>@sha256:<hex>,
// ended by a newline, with nothing else in the file.
func parsePin(text string) (pin, error) {
	line, ended := strings.CutSuffix(text, "\n")
	if !ended {
		return pin{}, errors.New("the pin does not end with a newline")
	}
	if strings.ContainsAny(line, " \t\r\n") {
		return pin{}, errors.New("the pin holds more than one reference on one line")
	}
	name, digest, found := strings.Cut(line, "@")
	if !found {
		return pin{}, errors.New("the pin names no digest: it is <registry>/<repository>@sha256:<hex>")
	}
	if _, err := ociregistry.LayoutBlobPath(digest); err != nil {
		return pin{}, err
	}
	registry, repository, found := strings.Cut(name, "/")
	if !found {
		return pin{}, errors.New("the pin names no repository: it is <registry>/<repository>@sha256:<hex>")
	}
	if registry == "" {
		return pin{}, errors.New("the pin names no registry")
	}
	if repository == "" {
		return pin{}, errors.New("the pin names an empty repository")
	}
	endpoint := "https://" + registry
	if registry == hubName {
		endpoint = hubEndpoint
	}
	return pin{endpoint: endpoint, registry: registry, repository: repository, digest: digest}, nil
}

// seedManifest reads an index and an image manifest: their fields do not
// overlap.
type seedManifest struct {
	Manifests []ociregistry.Descriptor `json:"manifests"`
	Config    ociregistry.Descriptor   `json:"config"`
	Layers    []ociregistry.Descriptor `json:"layers"`
}

// seeder copies one pinned image into one layout folder. Not safe for
// concurrent use.
type seeder struct {
	endpoint   string
	repository string
	client     *http.Client
	// authorization is the header the registry's realm granted, or empty
	// before the first challenge.
	authorization string
	layout        *os.Root
	out           io.Writer
	fetched       int
	present       int
}

// seed copies the pinned index, the manifests of the seed's platforms, their
// configurations and their layers into the layout folder dir, then records
// the index in the folder's index.json under the latest tag. index.json is
// written last, so a folder that holds one holds the whole seed.
func seed(ctx context.Context, pinned pin, dir string, out io.Writer) error {
	indexName, err := ociregistry.LayoutBlobPath(pinned.digest)
	if err != nil {
		return err
	}
	// The guest and a registry container read the folder under other users.
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: read by the test guest and a registry container
		return err
	}
	layout, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer layout.Close() //nolint:errcheck // every write was closed and checked
	if err := layout.MkdirAll(filepath.Dir(indexName), 0o755); err != nil {
		return err
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		ForceAttemptHTTP2:     true,
	}
	defer transport.CloseIdleConnections()
	s := &seeder{
		endpoint:   pinned.endpoint,
		repository: pinned.repository,
		client:     &http.Client{Transport: transport},
		layout:     layout,
		out:        out,
	}
	if err := s.say("container-seed: layout " + dir); err != nil {
		return err
	}
	index := ociregistry.Descriptor{MediaType: ociregistry.MediaOCIIndex, Digest: pinned.digest, Size: sizeUnknown}
	body, err := s.manifest(ctx, index, "index")
	if err != nil {
		return err
	}
	var listing seedManifest
	if err := json.Unmarshal(body, &listing); err != nil {
		return fmt.Errorf("the index %s: %w", pinned.digest, err)
	}
	members, err := seedPlatforms(listing.Manifests)
	if err != nil {
		return err
	}
	for _, member := range members {
		if err := s.image(ctx, member); err != nil {
			return err
		}
	}
	index.Size = int64(len(body))
	index.Annotations = map[string]string{
		ociregistry.AnnotationRefName: seedTag,
		annotationImageName:           pinned.registry + "/" + pinned.repository + ":" + seedTag,
	}
	if err := s.record(index); err != nil {
		return err
	}
	return s.say("container-seed: " + strconv.Itoa(s.fetched) + " fetched, " + strconv.Itoa(s.present) + " already there")
}

// seedPlatforms answers the entries of an index the seed copies: those of
// linux on a seed architecture. An index that lacks one of the architectures
// is refused, so a seed never holds half of what the tests run on.
func seedPlatforms(entries []ociregistry.Descriptor) ([]ociregistry.Descriptor, error) {
	var members []ociregistry.Descriptor
	for _, entry := range entries {
		if seedsPlatform(entry.Platform) {
			members = append(members, entry)
		}
	}
	for _, architecture := range seedArchitectures {
		held := slices.ContainsFunc(members, func(member ociregistry.Descriptor) bool {
			return member.Platform.Architecture == architecture
		})
		if !held {
			return nil, errors.New("the pinned index names no " + osLinux + "/" + architecture + " image")
		}
	}
	return members, nil
}

// seedsPlatform answers whether the seed copies the image of platform. An
// entry with no platform, and an attestation, which names an unknown one, are
// not copied.
func seedsPlatform(platform *ociregistry.Platform) bool {
	if platform == nil {
		return false
	}
	if platform.OS != osLinux {
		return false
	}
	return slices.Contains(seedArchitectures[:], platform.Architecture)
}

// image copies the manifest member names, its configuration and its layers.
func (s *seeder) image(ctx context.Context, member ociregistry.Descriptor) error {
	what := member.Platform.OS + "/" + member.Platform.Architecture
	body, err := s.manifest(ctx, member, what+" manifest")
	if err != nil {
		return err
	}
	var image seedManifest
	if err := json.Unmarshal(body, &image); err != nil {
		return fmt.Errorf("the manifest %s: %w", member.Digest, err)
	}
	if image.Config.Digest == "" {
		return errors.New("the manifest " + member.Digest + " names no configuration")
	}
	if err := s.blob(ctx, image.Config, what+" configuration"); err != nil {
		return err
	}
	for _, layer := range image.Layers {
		if err := s.blob(ctx, layer, what+" layer"); err != nil {
			return err
		}
	}
	return nil
}

// manifest answers the manifest or the index descriptor names, from the
// layout when a correct copy is there and from the registry otherwise, in
// which case the checked bytes are stored.
func (s *seeder) manifest(ctx context.Context, descriptor ociregistry.Descriptor, what string) ([]byte, error) {
	name, err := ociregistry.LayoutBlobPath(descriptor.Digest)
	if err != nil {
		return nil, err
	}
	held, found, err := s.heldManifest(name, descriptor)
	if err != nil {
		return nil, err
	}
	if found {
		s.present++
		return held, s.report("there  ", descriptor.Digest, int64(len(held)), what)
	}
	var body []byte
	err = s.fetch(ctx, "/manifests/"+descriptor.Digest, acceptManifests, func(answer io.Reader) error {
		data, err := io.ReadAll(io.LimitReader(answer, manifestSizeMax+1))
		body = data
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := checkManifest(body, descriptor); err != nil {
		return nil, fmt.Errorf("from %s: %w", s.endpoint, err)
	}
	if err := s.layout.WriteFile(name+partialSuffix, body, 0o644); err != nil {
		return nil, err
	}
	if err := s.layout.Rename(name+partialSuffix, name); err != nil {
		return nil, err
	}
	s.fetched++
	return body, s.report("fetched", descriptor.Digest, int64(len(body)), what)
}

// heldManifest answers the copy of a manifest the layout holds, and whether
// it holds a correct one. A copy that fails its check is not held: the caller
// fetches the manifest again and replaces it.
func (s *seeder) heldManifest(name string, descriptor ociregistry.Descriptor) ([]byte, bool, error) {
	file, err := s.layout.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer file.Close() //nolint:errcheck // read-only file
	data, err := io.ReadAll(io.LimitReader(file, manifestSizeMax+1))
	if err != nil {
		return nil, false, err
	}
	if checkManifest(data, descriptor) != nil {
		return nil, false, nil
	}
	return data, true, nil
}

// checkManifest refuses bytes that are not the manifest descriptor names: too
// large, of another size than a known one, or of another digest.
func checkManifest(data []byte, descriptor ociregistry.Descriptor) error {
	if len(data) > manifestSizeMax {
		return errors.New("the manifest " + descriptor.Digest + " is larger than the 4 MiB a manifest may hold")
	}
	if descriptor.Size != sizeUnknown {
		if int64(len(data)) != descriptor.Size {
			return errors.New("the manifest " + descriptor.Digest + " holds " + strconv.Itoa(len(data)) +
				" bytes, and its descriptor declares " + textbuf.StringInt(descriptor.Size))
		}
	}
	sum := sha256.Sum256(data)
	if got := digestAlgorithm + hex.EncodeToString(sum[:]); got != descriptor.Digest {
		return errors.New("the manifest " + descriptor.Digest + " hashes to " + got)
	}
	return nil
}

// blob copies the configuration or the layer descriptor names into the
// layout, unless a correct copy is there. The bytes go to a partial file that
// takes the blob's name only once its size and digest are checked.
func (s *seeder) blob(ctx context.Context, descriptor ociregistry.Descriptor, what string) error {
	name, err := ociregistry.LayoutBlobPath(descriptor.Digest)
	if err != nil {
		return err
	}
	held, err := s.heldBlob(name, descriptor)
	if err != nil {
		return err
	}
	if held {
		s.present++
		return s.report("there  ", descriptor.Digest, descriptor.Size, what)
	}
	partial := name + partialSuffix
	err = s.fetch(ctx, "/blobs/"+descriptor.Digest, "", func(answer io.Reader) error {
		return s.copyChecked(partial, answer, descriptor)
	})
	if err != nil {
		return errors.Join(err, s.discard(partial))
	}
	if err := s.layout.Rename(partial, name); err != nil {
		return err
	}
	s.fetched++
	return s.report("fetched", descriptor.Digest, descriptor.Size, what)
}

// heldBlob answers whether the layout holds a correct copy of a blob: of the
// declared size and hashing to the digest. A copy that is not is replaced.
func (s *seeder) heldBlob(name string, descriptor ociregistry.Descriptor) (bool, error) {
	file, err := s.layout.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close() //nolint:errcheck // read-only file
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if info.Size() != descriptor.Size {
		return false, nil
	}
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return false, err
	}
	return digestOf(sum) == descriptor.Digest, nil
}

// copyChecked writes answer into the partial file while it hashes it, and
// refuses bytes of another size or another digest than descriptor declares.
func (s *seeder) copyChecked(partial string, answer io.Reader, descriptor ociregistry.Descriptor) error {
	file, err := s.layout.OpenFile(partial, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	sum := sha256.New()
	// One byte past the declared size is enough to know the answer is longer.
	received, err := io.Copy(io.MultiWriter(file, sum), io.LimitReader(answer, descriptor.Size+1))
	if closeErr := file.Close(); closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if err != nil {
		return fmt.Errorf("the transfer of %s from %s stopped: %w", descriptor.Digest, s.endpoint, err)
	}
	if received != descriptor.Size {
		return errors.New("the blob " + descriptor.Digest + " from " + s.endpoint + " does not hold the " +
			textbuf.StringInt(descriptor.Size) + " bytes its descriptor declares")
	}
	if got := digestOf(sum); got != descriptor.Digest {
		return errors.New("the blob " + descriptor.Digest + " from " + s.endpoint + " hashes to " + got)
	}
	return nil
}

// discard removes a partial file a failed transfer left, if it left one.
func (s *seeder) discard(partial string) error {
	err := s.layout.Remove(partial)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// record writes the layout's marker and its index.json, which names the
// pinned index under the latest tag.
func (s *seeder) record(index ociregistry.Descriptor) error {
	listing, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     ociregistry.MediaOCIIndex,
		"manifests":     []ociregistry.Descriptor{index},
	})
	if err != nil {
		return err
	}
	if err := s.layout.WriteFile(ociregistry.LayoutMarkerFile, []byte(ociregistry.LayoutMarker), 0o644); err != nil {
		return err
	}
	partial := ociregistry.LayoutIndexFile + partialSuffix
	if err := s.layout.WriteFile(partial, listing, 0o644); err != nil {
		return err
	}
	return s.layout.Rename(partial, ociregistry.LayoutIndexFile)
}

// fetch asks the registry for path of the repository and hands the body of a
// 200 answer to consume. It answers one bearer challenge with a token of the
// realm, asked anonymously, and asks again.
func (s *seeder) fetch(ctx context.Context, path, accept string, consume func(answer io.Reader) error) error {
	target := s.endpoint + "/v2/" + s.repository + path
	challenge, err := s.fetchOnce(ctx, target, accept, consume)
	if err != nil {
		return err
	}
	if challenge == "" {
		return nil
	}
	if err := s.authorize(ctx, challenge); err != nil {
		return err
	}
	challenge, err = s.fetchOnce(ctx, target, accept, consume)
	if err != nil {
		return err
	}
	if challenge != "" {
		return errors.New(target + " refused the token its own realm granted")
	}
	return nil
}

// fetchOnce sends one GET of target and hands the body of a 200 answer to
// consume. A 401 answer is no error: it answers the challenge the registry
// sent, which is never empty, and consume was not called.
func (s *seeder) fetchOnce(ctx context.Context, target, accept string, consume func(answer io.Reader) error) (string, error) {
	response, err := s.send(ctx, target, accept)
	if err != nil {
		return "", err
	}
	defer closeBody(response)
	if response.StatusCode == http.StatusUnauthorized {
		challenge := response.Header.Get("WWW-Authenticate")
		if challenge == "" {
			return "", errors.New(target + " answered status 401 with no challenge")
		}
		return challenge, nil
	}
	if response.StatusCode != http.StatusOK {
		return "", errors.New(target + " answered status " + strconv.Itoa(response.StatusCode))
	}
	return "", consume(response.Body)
}

// send sends one GET, with the granted authorization when there is one. The
// caller MUST close the body. The standard client follows a redirect, and
// drops the authorization when the redirect leaves the registry's host.
func (s *seeder) send(ctx context.Context, target, accept string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		request.Header.Set("Accept", accept)
	}
	if s.authorization != "" {
		request.Header.Set("Authorization", s.authorization)
	}
	return s.client.Do(request)
}

// authorize asks the realm a bearer challenge names for an anonymous pull
// token of the repository, and keeps it for the requests that follow.
func (s *seeder) authorize(ctx context.Context, challenge string) error {
	parameters, ok := strings.CutPrefix(challenge, bearerScheme)
	if !ok {
		return errors.New(s.endpoint + " asks for credentials the seed does not hold: its challenge is " + strconv.Quote(challenge))
	}
	realm, ok := challengeValue(parameters, "realm")
	if !ok {
		return errors.New(s.endpoint + " answered a bearer challenge that names no realm")
	}
	query := url.Values{"scope": {"repository:" + s.repository + ":pull"}}
	if service, ok := challengeValue(parameters, "service"); ok {
		query.Set("service", service)
	}
	// A token that ran out is not sent to the realm.
	s.authorization = ""
	response, err := s.send(ctx, realm+"?"+query.Encode(), "")
	if err != nil {
		return err
	}
	defer closeBody(response)
	if response.StatusCode != http.StatusOK {
		return errors.New("the token realm " + realm + " answered status " + strconv.Itoa(response.StatusCode))
	}
	var granted struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, tokenSizeMax)).Decode(&granted); err != nil {
		return fmt.Errorf("the answer of the token realm %s: %w", realm, err)
	}
	token := granted.Token
	if token == "" {
		token = granted.AccessToken
	}
	if token == "" {
		return errors.New("the token realm " + realm + " granted no token")
	}
	s.authorization = bearerScheme + token
	return nil
}

// challengeValue answers the quoted value of key in the parameters of a
// challenge, and whether the challenge holds it.
func challengeValue(parameters, key string) (string, bool) {
	_, rest, found := strings.Cut(parameters, key+`="`)
	if !found {
		return "", false
	}
	value, _, closed := strings.Cut(rest, `"`)
	return value, closed
}

// closeBody closes the body of an answer the seed is done with.
func closeBody(response *http.Response) {
	response.Body.Close() //nolint:errcheck // read-only body
}

// digestOf answers the sha256:<hex> digest a hash holds.
func digestOf(sum hash.Hash) string {
	return digestAlgorithm + hex.EncodeToString(sum.Sum(nil))
}

// report prints one object of the seed: whether it was fetched or already
// there, its digest, its size and what it is.
func (s *seeder) report(state, digest string, size int64, what string) error {
	return s.say(state + "  " + digest + "  " + textbuf.StringInt(size) + " bytes  " + what)
}

// say prints one line of the seed's report.
func (s *seeder) say(line string) error {
	_, err := fmt.Fprintln(s.out, line)
	return err
}
