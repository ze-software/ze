//go:build docker

// Design: docs/guide/gnmi.md -- the gNMI server against a client Ze did not write
// Related: server.go -- the TLS listener and the bearer-token check this drives.
// Related: capabilities.go -- the models and encodings gnmic decodes.
// Related: get.go -- the Get answer gnmic decodes.
// Related: subscribe.go -- the Subscribe ONCE answer gnmic decodes.
// Related: set.go -- the Set answer gnmic decodes.
//
// gNMI specification: https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md
// Client: gnmic, https://github.com/openconfig/gnmic (Apache-2.0).

package gnmi

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	"github.com/ze-software/ze/internal/component/api"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/labtest"
)

const (
	// gnmicImage is gnmic 0.49.1 from the OpenConfig project, its latest
	// stable release (M14), pinned by digest, so the client under the test
	// cannot change between two runs.
	gnmicImage = "ghcr.io/openconfig/gnmic:0.49.1" +
		"@sha256:d148508c7ab3e5fd446759648d529409c1e37c2cc99c8b681078d120ca1a302b"
	// gnmicCertDirectory is where the client reads the test's CA certificate.
	gnmicCertDirectory = "/certs"
	// gnmicToken is the bearer token Ze's server requires.
	gnmicToken = "ze-interop-gnmi-token-0123456789"
	// gnmicRouterID is the value the running tree holds at /bgp/router-id.
	gnmicRouterID = "192.0.2.1"
	// gnmicSetRouterID is the value gnmic's Set writes there.
	gnmicSetRouterID = "198.51.100.7"
	// gnmicTimeout bounds one client run, the image's start included.
	gnmicTimeout = 60 * time.Second
)

// TestGnmicInterop drives Ze's gNMI server, in-process over TLS with a bearer
// token, from gnmic in a container on the host network. Each subtest decodes
// gnmic's protojson output into the gNMI messages and asserts what Ze sent:
// the models and encodings it advertises, the value at a Get path, the
// notification and the sync of a Subscribe ONCE, the result of a Set and the
// commit it made, and the Unauthenticated status of a request with no token
// or a wrong one. The CA gnmic verifies Ze's certificate against is written by
// the test, so a server that served no TLS, or another certificate, fails the
// handshake.
func TestGnmicInterop(t *testing.T) {
	certs := t.TempDir()
	certPEM, keyPEM := writeGnmicCertificates(t, certs)

	tree := zeconfig.NewTree()
	bgp := zeconfig.NewTree()
	bgp.Set("router-id", gnmicRouterID)
	tree.SetContainer("bgp", bgp)
	editor := &configSessionEditor{values: make(map[string]string)}
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) { return editor, nil })
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0", Token: gnmicToken, CertPEM: certPEM, KeyPEM: keyPEM},
		treeFunc(tree), sessions, yangloader.DefaultLoader, nil)
	startTestServer(t, srv)
	address := srv.Address()

	t.Run("capabilities advertise the loaded models and JSON_IETF", func(t *testing.T) {
		result := runGnmic(t, certs, address, gnmicToken, "capabilities")
		requireGnmicSuccess(t, result)
		var answer gpb.CapabilityResponse
		decodeGnmicOutput(t, result.Stdout, &answer)
		if !slices.Equal(answer.GetSupportedEncodings(), []gpb.Encoding{gpb.Encoding_JSON_IETF}) {
			t.Errorf("supported encodings = %v, want [JSON_IETF]", answer.GetSupportedEncodings())
		}
		if answer.GetGNMIVersion() != "0.10.0" {
			t.Errorf("gNMI version = %q, want 0.10.0", answer.GetGNMIVersion())
		}
		got := modelKeys(answer.GetSupportedModels())
		want := loadedModelKeys(t)
		if !slices.Equal(got, want) {
			t.Errorf("gnmic decoded %d models, the loader holds %d:\ngot  %v\nwant %v", len(got), len(want), got, want)
		}
	})

	t.Run("get answers the running value", func(t *testing.T) {
		result := runGnmic(t, certs, address, gnmicToken, "get", "--encoding", "json_ietf", "--path", "/bgp/router-id")
		requireGnmicSuccess(t, result)
		var answer gpb.GetResponse
		decodeGnmicOutput(t, result.Stdout, &answer)
		notifications := answer.GetNotification()
		if len(notifications) != 1 {
			t.Fatalf("Get answered %d notifications, want 1: %s", len(notifications), result.Stdout)
		}
		requireRouterIDUpdate(t, notifications[0].GetUpdate(), gnmicRouterID)
	})

	t.Run("get of an absent path is NotFound", func(t *testing.T) {
		result := runGnmic(t, certs, address, gnmicToken, "get", "--encoding", "json_ietf", "--path", "/bgp/no-such-leaf")
		requireGnmicRefusal(t, result, "code = NotFound desc = path not found: /bgp/no-such-leaf")
	})

	t.Run("subscribe once sends the value then the sync", func(t *testing.T) {
		result := runGnmic(t, certs, address, gnmicToken, "subscribe", "--mode", "once", "--path", "/bgp/router-id")
		requireGnmicSuccess(t, result)
		responses := decodeGnmicSubscribeOutput(t, result.Stdout)
		if len(responses) != 2 {
			t.Fatalf("Subscribe ONCE answered %d responses, want an update and the sync: %s", len(responses), result.Stdout)
		}
		requireRouterIDUpdate(t, responses[0].GetUpdate().GetUpdate(), gnmicRouterID)
		if !responses[1].GetSyncResponse() {
			t.Errorf("second response %v is not sync_response true", responses[1])
		}
	})

	t.Run("set commits the update", func(t *testing.T) {
		result := runGnmic(t, certs, address, gnmicToken, "set", "--update", "/bgp/router-id:::string:::"+gnmicSetRouterID)
		requireGnmicSuccess(t, result)
		var answer gpb.SetResponse
		decodeGnmicOutput(t, result.Stdout, &answer)
		results := answer.GetResponse()
		if len(results) != 1 {
			t.Fatalf("Set answered %d results, want 1: %s", len(results), result.Stdout)
		}
		if results[0].GetOp() != gpb.UpdateResult_UPDATE {
			t.Errorf("Set result op = %v, want UPDATE", results[0].GetOp())
		}
		if path := gnmicPathNames(results[0].GetPath()); !slices.Equal(path, []string{"bgp", "router-id"}) {
			t.Errorf("Set result path = %v, want [bgp router-id]", path)
		}
		if !strings.Contains(editor.committedContent, "bgp.router-id = "+gnmicSetRouterID) {
			t.Errorf("committed content %q does not hold the router-id gnmic set", editor.committedContent)
		}
	})

	t.Run("no token is Unauthenticated", func(t *testing.T) {
		result := runGnmic(t, certs, address, "", "capabilities")
		requireGnmicRefusal(t, result, "code = Unauthenticated desc = missing authorization")
	})

	t.Run("a wrong token is Unauthenticated", func(t *testing.T) {
		result := runGnmic(t, certs, address, "not-"+gnmicToken, "capabilities")
		requireGnmicRefusal(t, result, "code = Unauthenticated desc = invalid token")
	})
}

// runGnmic runs one gnmic command against address, verifying the server
// against the test's CA, with token as the bearer token when it is not empty.
func runGnmic(t *testing.T, certs, address, token string, command ...string) interoplab.CommandResult {
	t.Helper()
	arguments := []string{"--address", address, "--tls-ca", gnmicCertDirectory + "/ca.pem",
		"--format", "protojson", "--timeout", "10s"}
	if token != "" {
		arguments = append(arguments, "--token", token)
	}
	return labtest.Run(t, labtest.Client{
		Name: "gnmic", Image: gnmicImage, Command: append(arguments, command...), HostNetwork: true,
		Mounts:  []interoplab.Mount{{Source: certs, Target: gnmicCertDirectory, ReadOnly: true}},
		Timeout: gnmicTimeout,
	})
}

// requireGnmicSuccess asserts gnmic exited 0.
func requireGnmicSuccess(t *testing.T, result interoplab.CommandResult) {
	t.Helper()
	if result.ExitCode != 0 {
		t.Fatalf("gnmic exited %d, want 0\nstdout: %s\nstderr: %s", result.ExitCode, result.Stdout, result.Stderr)
	}
}

// requireGnmicRefusal asserts gnmic failed with the gRPC status text it
// prints for the status Ze answered.
func requireGnmicRefusal(t *testing.T, result interoplab.CommandResult, status string) {
	t.Helper()
	if result.ExitCode == 0 {
		t.Fatalf("gnmic succeeded, want the refusal %q\nstdout: %s", status, result.Stdout)
	}
	if !strings.Contains(result.Stderr, status) {
		t.Errorf("gnmic stderr does not name %q:\n%s", status, result.Stderr)
	}
}

func requireRouterIDUpdate(t *testing.T, updates []*gpb.Update, value string) {
	t.Helper()
	if len(updates) != 1 {
		t.Fatalf("notification holds %d updates, want 1", len(updates))
	}
	if path := gnmicPathNames(updates[0].GetPath()); !slices.Equal(path, []string{"bgp", "router-id"}) {
		t.Errorf("update path = %v, want [bgp router-id]", path)
	}
	if got := updates[0].GetVal().GetStringVal(); got != value {
		t.Errorf("update value = %q (%v), want %q", got, updates[0].GetVal(), value)
	}
}

func gnmicPathNames(path *gpb.Path) []string {
	names := make([]string, 0, len(path.GetElem()))
	for _, elem := range path.GetElem() {
		names = append(names, elem.GetName())
	}
	return names
}

// decodeGnmicOutput reads gnmic's one protojson message for one target.
func decodeGnmicOutput(t *testing.T, stdout string, message proto.Message) {
	t.Helper()
	if err := protojson.Unmarshal([]byte(stdout), message); err != nil {
		t.Fatalf("gnmic output is not one %T: %v\n%s", message, err, stdout)
	}
}

// decodeGnmicSubscribeOutput reads the protojson SubscribeResponse messages
// gnmic prints one after another.
func decodeGnmicSubscribeOutput(t *testing.T, stdout string) []*gpb.SubscribeResponse {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(stdout))
	var responses []*gpb.SubscribeResponse
	for {
		var raw json.RawMessage
		err := decoder.Decode(&raw)
		if errors.Is(err, io.EOF) {
			return responses
		}
		if err != nil {
			t.Fatalf("gnmic subscribe output is not a JSON sequence: %v\n%s", err, stdout)
		}
		response := &gpb.SubscribeResponse{}
		if err := protojson.Unmarshal(raw, response); err != nil {
			t.Fatalf("gnmic subscribe output holds a value that is not a SubscribeResponse: %v\n%s", err, raw)
		}
		responses = append(responses, response)
	}
}

// modelKeys renders each model as name|organization|version, sorted.
func modelKeys(models []*gpb.ModelData) []string {
	keys := make([]string, 0, len(models))
	for _, model := range models {
		keys = append(keys, model.GetName()+"|"+model.GetOrganization()+"|"+model.GetVersion())
	}
	slices.Sort(keys)
	return keys
}

// loadedModelKeys is what Capabilities owes: every module the loader holds.
func loadedModelKeys(t *testing.T) []string {
	t.Helper()
	loader, err := yangloader.DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader: %v", err)
	}
	names := loader.ModuleNames()
	if len(names) == 0 {
		t.Fatal("the loader holds no module, so the model assertion would prove nothing")
	}
	models := make([]*gpb.ModelData, 0, len(names))
	for _, name := range names {
		module := loader.GetModule(name)
		if module == nil {
			t.Fatalf("the loader names module %s and holds none", name)
		}
		model := &gpb.ModelData{Name: module.Name}
		if module.Organization != nil {
			model.Organization = module.Organization.Name
		}
		if len(module.Revision) > 0 {
			model.Version = module.Revision[0].Name
		}
		models = append(models, model)
	}
	return modelKeys(models)
}

// writeGnmicCertificates writes a CA certificate to directory/ca.pem and
// answers a server certificate and key for 127.0.0.1 that the CA signed.
func writeGnmicCertificates(t *testing.T, directory string) (certPEM, keyPEM []byte) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("CA key: %v", err)
	}
	notBefore := time.Now().Add(-time.Hour)
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "ze gnmic interop CA"},
		NotBefore: notBefore, NotAfter: notBefore.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("CA certificate: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA certificate: %v", err)
	}

	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("server key: %v", err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: notBefore, NotAfter: notBefore.Add(24 * time.Hour),
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
		KeyUsage:    x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("server certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(serverKey)
	if err != nil {
		t.Fatalf("server key encoding: %v", err)
	}

	var caPEM bytes.Buffer
	if err := pem.Encode(&caPEM, &pem.Block{Type: "CERTIFICATE", Bytes: caDER}); err != nil {
		t.Fatalf("CA PEM: %v", err)
	}
	// The client container may run as another user than the test, so the CA
	// file and its directory are world-readable; the CA signs nothing else.
	if err := os.WriteFile(filepath.Join(directory, "ca.pem"), caPEM.Bytes(), 0o644); err != nil { //nolint:gosec // public CA certificate
		t.Fatalf("write CA: %v", err)
	}
	if err := os.Chmod(directory, 0o755); err != nil { //nolint:gosec // holds only the public CA certificate
		t.Fatalf("chmod %s: %v", directory, err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}
