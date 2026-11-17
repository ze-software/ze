// Design: docs/architecture/container/image-store.md -- the test registry the registry client is proven against

// Package ociregistry is an in-process container registry for tests. It
// serves the pull subset of the distribution protocol that the container
// registry client speaks: manifests by tag and by digest, blobs by digest, a
// bearer or a basic challenge with its token realm, and scripted faults (a
// wrong body, a stall, server errors, redirects). A unit test starts one
// directly. It is written from the protocol, not from the client: it imports
// nothing of the client, so a constant the client gets wrong is not repeated
// here and passes on both sides.
package ociregistry

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

// Auth is how the registry answers a request that carries no credentials.
type Auth uint8

const (
	// AuthUnspecified is no choice. Start refuses it.
	AuthUnspecified Auth = iota
	// AuthNone serves every request.
	AuthNone
	// AuthBearer answers a bearer challenge naming its token realm.
	AuthBearer
	// AuthBasic answers a basic challenge.
	AuthBasic
)

// Media types the image builder writes. The client's own copies are not
// imported, on purpose (the package comment says why).
const (
	MediaOCIIndex     = "application/vnd.oci.image.index.v1+json"
	MediaOCIManifest  = "application/vnd.oci.image.manifest.v1+json"
	MediaOCIConfig    = "application/vnd.oci.image.config.v1+json"
	MediaOCILayerTar  = "application/vnd.oci.image.layer.v1.tar"
	MediaOCILayerGzip = "application/vnd.oci.image.layer.v1.tar+gzip"
)

// tokenPath is where the registry's own token realm answers.
const tokenPath = "/token"

// Options configure one registry.
type Options struct {
	// Auth MUST be set.
	Auth Auth
	// TLS serves HTTPS with the test certificate Certificate answers.
	TLS bool
	// Username and Password are the credentials the realm (AuthBearer) or
	// the registry (AuthBasic) requires. An empty Username makes the realm
	// grant a token to anyone, as a public registry does.
	Username string
	Password string
	// Realm replaces the realm the bearer challenge names. Empty names the
	// registry's own token path.
	Realm string
	// TokenAnswerSize pads the token answer to exactly this many bytes, for
	// the bound of the token answer. Zero sends it unpadded.
	TokenAnswerSize int
}

// Descriptor is the OCI descriptor of one stored object, in the wire form.
type Descriptor struct {
	MediaType string    `json:"mediaType"`
	Digest    string    `json:"digest"`
	Size      int64     `json:"size"`
	Platform  *Platform `json:"platform,omitempty"`
}

// Platform is the platform of an index entry, in the wire form.
type Platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
}

// Layer is one layer of an image: its media type and its bytes as served.
type Layer struct {
	MediaType string
	Content   []byte
}

// Image is what PutImage stores: one platform's manifest, its configuration
// and its layers.
type Image struct {
	Architecture string
	Variant      string
	Layers       []Layer
	// Config replaces the minimal configuration naming linux and
	// Architecture. Nil writes the minimal one.
	Config []byte
}

// Fault scripts one misbehavior for a request whose last path segment is the
// key it was set under: a digest or a tag.
type Fault struct {
	// Body is served in place of the stored bytes, under the stored
	// object's headers: a wrong digest or a wrong size.
	Body []byte
	// ServerErrors answers this many requests with status 500 first.
	ServerErrors int
	// StallAfter sends this many bytes, then nothing until the client leaves.
	StallAfter int
	// RedirectTo answers a temporary redirect to this URL.
	RedirectTo string
	// RedirectHops answers this many redirects on this host before serving.
	RedirectHops int
}

// Request is one request the registry received.
type Request struct {
	Path          string
	Authorization string
}

// stored is one manifest or blob.
type stored struct {
	mediaType string
	body      []byte
}

// Registry is one running test registry. Safe for concurrent use. Start
// creates it; Close MUST be called to stop it.
type Registry struct {
	server  *httptest.Server
	options Options
	token   string
	done    chan struct{}

	mu        sync.Mutex
	manifests map[string]map[string]stored
	blobs     map[string]stored
	faults    map[string]*Fault
	requests  []Request
}

// Start starts a registry on a loopback port.
func Start(options Options) (*Registry, error) {
	if options.Auth == AuthUnspecified {
		return nil, errors.New("ociregistry: Options.Auth is not set")
	}
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	r := &Registry{
		options:   options,
		token:     "ze-test-token-" + hex.EncodeToString(secret),
		done:      make(chan struct{}),
		manifests: map[string]map[string]stored{},
		blobs:     map[string]stored{},
		faults:    map[string]*Fault{},
	}
	if options.TLS {
		r.server = httptest.NewTLSServer(r)
	} else {
		r.server = httptest.NewServer(r)
	}
	return r, nil
}

// Close ends every stalled answer, then stops the server.
func (r *Registry) Close() {
	close(r.done)
	r.server.Close()
}

// Host answers the address a reference names the registry by: host:port.
func (r *Registry) Host() string {
	return r.server.Listener.Addr().String()
}

// URL answers scheme://host:port.
func (r *Registry) URL() string {
	return r.server.URL
}

// Certificate answers the certificate the registry serves under TLS, which a
// client trusts as its anchor, and nil for a plain registry.
func (r *Registry) Certificate() *x509.Certificate {
	return r.server.Certificate()
}

// Token answers the bearer token the realm grants.
func (r *Registry) Token() string {
	return r.token
}

// BlobURL answers the URL of a blob of this registry, for a redirect another
// registry answers.
func (r *Registry) BlobURL(repository, digest string) string {
	return r.URL() + "/v2/" + repository + "/blobs/" + digest
}

// PutBlob stores content as a blob and answers its descriptor.
func (r *Registry) PutBlob(mediaType string, content []byte) Descriptor {
	digest := digestOf(content)
	r.mu.Lock()
	r.blobs[digest] = stored{mediaType: mediaType, body: content}
	r.mu.Unlock()
	return Descriptor{MediaType: mediaType, Digest: digest, Size: int64(len(content))}
}

// PutManifest stores body as a manifest of repository, under its digest and,
// when tag is not empty, under tag.
func (r *Registry) PutManifest(repository, tag, mediaType string, body []byte) Descriptor {
	digest := digestOf(body)
	r.mu.Lock()
	names := r.manifests[repository]
	if names == nil {
		names = map[string]stored{}
		r.manifests[repository] = names
	}
	names[digest] = stored{mediaType: mediaType, body: body}
	if tag != "" {
		names[tag] = stored{mediaType: mediaType, body: body}
	}
	r.mu.Unlock()
	return Descriptor{MediaType: mediaType, Digest: digest, Size: int64(len(body))}
}

// PutImage stores the configuration, the layers and an OCI manifest of one
// platform, and answers the manifest's descriptor with that platform, ready
// to be an entry of PutIndex.
func (r *Registry) PutImage(repository, tag string, image Image) (Descriptor, error) {
	config := image.Config
	if config == nil {
		minimal, err := json.Marshal(map[string]any{
			"architecture": image.Architecture,
			"os":           "linux",
			"variant":      image.Variant,
			"rootfs":       map[string]any{"type": "layers", "diff_ids": []string{}},
		})
		if err != nil {
			return Descriptor{}, err
		}
		config = minimal
	}
	layers := make([]Descriptor, 0, len(image.Layers))
	for _, layer := range image.Layers {
		layers = append(layers, r.PutBlob(layer.MediaType, layer.Content))
	}
	body, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     MediaOCIManifest,
		"config":        r.PutBlob(MediaOCIConfig, config),
		"layers":        layers,
	})
	if err != nil {
		return Descriptor{}, err
	}
	descriptor := r.PutManifest(repository, tag, MediaOCIManifest, body)
	descriptor.Platform = &Platform{Architecture: image.Architecture, OS: "linux", Variant: image.Variant}
	return descriptor, nil
}

// PutIndex stores an OCI index of entries under tag.
func (r *Registry) PutIndex(repository, tag string, entries []Descriptor) (Descriptor, error) {
	body, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     MediaOCIIndex,
		"manifests":     entries,
	})
	if err != nil {
		return Descriptor{}, err
	}
	return r.PutManifest(repository, tag, MediaOCIIndex, body), nil
}

// SetFault scripts fault for the requests whose last path segment is key.
func (r *Registry) SetFault(key string, fault Fault) {
	r.mu.Lock()
	r.faults[key] = &fault
	r.mu.Unlock()
}

// Requests answers every request received so far, in order.
func (r *Registry) Requests() []Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Request(nil), r.requests...)
}

// ServeHTTP answers one request of the pull subset.
func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.requests = append(r.requests, Request{Path: req.URL.Path, Authorization: req.Header.Get("Authorization")})
	r.mu.Unlock()

	if req.URL.Path == tokenPath {
		r.serveToken(w, req)
		return
	}
	repository, kind, name, ok := splitPath(req.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "NAME_UNKNOWN")
		return
	}
	if !r.authorized(req) {
		r.challenge(w, repository)
		return
	}
	fault := r.takeFault(name)
	if fault.ServerErrors > 0 {
		writeError(w, http.StatusInternalServerError, "UNAVAILABLE")
		return
	}
	if hop, _ := strconv.Atoi(req.URL.Query().Get("hop")); hop < fault.RedirectHops {
		//nolint:gosec // G710: a test registry redirecting to its own path, the scripted same-host hop a client test asks for
		http.Redirect(w, req, req.URL.Path+"?hop="+strconv.Itoa(hop+1), http.StatusTemporaryRedirect)
		return
	}
	if fault.RedirectTo != "" {
		http.Redirect(w, req, fault.RedirectTo, http.StatusTemporaryRedirect)
		return
	}
	object, found := r.lookup(repository, kind, name)
	if !found {
		writeError(w, http.StatusNotFound, strings.ToUpper(kind)+"_UNKNOWN")
		return
	}
	body := object.body
	if fault.Body != nil {
		body = fault.Body
	}
	w.Header().Set("Content-Type", object.mediaType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if kind == "manifest" {
		w.Header().Set("Docker-Content-Digest", digestOf(object.body))
	}
	if fault.StallAfter > 0 && fault.StallAfter < len(body) {
		w.Write(body[:fault.StallAfter]) //nolint:errcheck // the client reports what arrived
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		select {
		case <-req.Context().Done():
		case <-r.done:
		}
		return
	}
	w.Write(body) //nolint:errcheck // the client reports what arrived
}

// takeFault answers a copy of the fault set for name, with one server error
// consumed when any remain. No fault answers the zero Fault, which serves.
func (r *Registry) takeFault(name string) Fault {
	r.mu.Lock()
	defer r.mu.Unlock()
	fault, ok := r.faults[name]
	if !ok {
		return Fault{}
	}
	answer := *fault
	if fault.ServerErrors > 0 {
		fault.ServerErrors--
	}
	return answer
}

// lookup answers the stored manifest or blob a request names.
func (r *Registry) lookup(repository, kind, name string) (stored, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if kind == "blob" {
		object, ok := r.blobs[name]
		return object, ok
	}
	object, ok := r.manifests[repository][name]
	return object, ok
}

// authorized answers whether req carries what the registry's Auth asks for.
func (r *Registry) authorized(req *http.Request) bool {
	header := req.Header.Get("Authorization")
	switch r.options.Auth {
	case AuthNone:
		return true
	case AuthBearer:
		return header == "Bearer "+r.token
	case AuthBasic:
		return header == basicAuthorization(r.options.Username, r.options.Password)
	case AuthUnspecified:
		return false
	}
	return false
}

// challenge answers 401 with the challenge of the registry's Auth.
func (r *Registry) challenge(w http.ResponseWriter, repository string) {
	switch r.options.Auth {
	case AuthBearer:
		realm := r.options.Realm
		if realm == "" {
			realm = r.URL() + tokenPath
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="`+realm+`",service="ociregistry",scope="repository:`+repository+`:pull"`)
	case AuthBasic:
		w.Header().Set("WWW-Authenticate", `Basic realm="ociregistry"`)
	case AuthNone, AuthUnspecified:
	}
	writeError(w, http.StatusUnauthorized, "UNAUTHORIZED")
}

// serveToken is the token realm: it grants the token, to the holder of the
// credentials when the registry has them and to anyone otherwise.
func (r *Registry) serveToken(w http.ResponseWriter, req *http.Request) {
	if r.options.Username != "" && req.Header.Get("Authorization") != basicAuthorization(r.options.Username, r.options.Password) {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	answer := `{"token":"` + r.token + `","padding":""}`
	if pad := r.options.TokenAnswerSize - len(answer); pad > 0 {
		answer = `{"token":"` + r.token + `","padding":"` + strings.Repeat("x", pad) + `"}`
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(answer)) //nolint:errcheck // the client reports what arrived
}

// splitPath reads /v2/<repository>/manifests/<name> and
// /v2/<repository>/blobs/<name>, answering the kind manifest or blob.
func splitPath(path string) (repository, kind, name string, ok bool) {
	rest, found := strings.CutPrefix(path, "/v2/")
	if !found {
		return "", "", "", false
	}
	if i := strings.LastIndex(rest, "/manifests/"); i > 0 {
		return rest[:i], "manifest", rest[i+len("/manifests/"):], true
	}
	if i := strings.LastIndex(rest, "/blobs/"); i > 0 {
		return rest[:i], "blob", rest[i+len("/blobs/"):], true
	}
	return "", "", "", false
}

// writeError answers status with a distribution error body.
func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"errors":[{"code":"` + code + `","message":"test registry"}]}`)) //nolint:errcheck // best effort
}

// basicAuthorization answers the basic authorization header of a pair.
func basicAuthorization(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

// digestOf answers the sha256:<hex> digest of content.
func digestOf(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}
