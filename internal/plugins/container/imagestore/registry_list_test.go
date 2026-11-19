// Design: docs/architecture/container/image-store.md -- the registry list: its schema, its rules and the client configuration it gives

package imagestore

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// registrySections is one delivery that carries the container root with data
// as its body, as the engine delivers it: wrapped in the root's name.
func registrySections(data string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: containerRoot, Data: data}}
}

// registryBody wraps the entries of a registry list, written as the JSON
// object of the list, in the delivered container root.
func registryBody(entries string) string {
	return `{"container":{"registry":` + entries + `}}`
}

// mustParseRegistries parses the registry list of data and fails the test on
// a refusal.
func mustParseRegistries(t *testing.T, data string) registryList {
	t.Helper()
	list, err := parseRegistrySections(registrySections(data))
	if err != nil {
		t.Fatalf("parse %s: %v", data, err)
	}
	return list
}

// TestRegistryListIsParsed proves each node of an entry reaches the parsed
// list, and each absent node takes the default the schema states.
//
// Method: one delivery holds an entry with every node, an entry with none,
// a mirror that names its address only and a mirror on an IPv6 address. The
// parsed list is compared whole, in key order.
func TestRegistryListIsParsed(t *testing.T) {
	list := mustParseRegistries(t, registryBody(`{
		"docker.io":{"transport":"http","description":"the hub through the lab mirror",
			"mirror":{"address":"127.0.0.1","port":"5000","path":"/cache"},
			"authentication":{"username":"operator","password":"hub-secret"}},
		"ghcr.io":{},
		"quay.io":{"mirror":{"address":"mirror.example"}},
		"registry.example:5000":{"transport":"https","mirror":{"address":"::1","port":"8443"}}
	}`))
	want := registryList{entries: []registryEntry{
		{
			host: "docker.io", transport: registry.TransportHTTP,
			mirror: "127.0.0.1:5000", mirrorPath: "/cache",
			credentials: registry.NewCredentials("operator", "hub-secret"),
		},
		{host: "ghcr.io", transport: registry.TransportHTTPS},
		{host: "quay.io", transport: registry.TransportHTTPS, mirror: "mirror.example:443", mirrorPath: "/"},
		{host: "registry.example:5000", transport: registry.TransportHTTPS, mirror: "[::1]:8443", mirrorPath: "/"},
	}}
	if !reflect.DeepEqual(list, want) {
		t.Fatalf("parsed %#v, want %#v", list, want)
	}
}

// TestConfigWithoutRegistryIsTheZeroList proves a config that holds no
// registry parses to the zero list, the one the store runs before any config:
// the config transaction compares lists, and a list that differed from the
// zero one would be installed again at every commit.
func TestConfigWithoutRegistryIsTheZeroList(t *testing.T) {
	deliveries := map[string][]sdk.ConfigSection{
		"an empty container root":       registrySections(`{"container":{}}`),
		"a removed container root":      registrySections(`{}`),
		"a root with instances only":    registrySections(`{"container":{"instance":{"web":{"image":"redis"}}}}`),
		"a delivery without the root":   {{Root: "interface", Data: `{"interface":{}}`}},
		"a delivery with no section":    nil,
		"an empty registry list object": registrySections(registryBody(`{}`)),
	}
	for name, sections := range deliveries {
		list, err := parseRegistrySections(sections)
		if err != nil {
			t.Errorf("%s: refused: %v", name, err)
			continue
		}
		if !reflect.DeepEqual(list, registryList{}) {
			t.Errorf("%s: parsed %#v, want the zero list", name, list)
		}
	}
}

// TestRegistryListRefusals covers each rule of the list: every refusal is an
// operator error that names the entry and the defect, and whose fix is the
// statement that sets the refused node.
func TestRegistryListRefusals(t *testing.T) {
	const (
		fixHub       = "set container registry docker.io transport https"
		fixAnyHost   = "set container registry HOST transport https"
		fixMirror    = "set container registry docker.io mirror address ADDRESS port PORT"
		fixPath      = "set container registry docker.io mirror path /PATH"
		fixAuth      = "set container registry docker.io authentication username NAME password PASSWORD"
		fixText      = "set container registry docker.io description TEXT"
		needsAddress = "the mirror needs an address"
		badAddress   = "the mirror's address is neither an IP address nor a host name in lower case"
	)
	cases := []struct {
		name    string
		entries string
		problem string
		fix     string
	}{
		{"a name with no dot or colon", `{"myregistry":{}}`, "a reference that starts with myregistry names docker.io", fixHub},
		{"the hub's other name", `{"index.docker.io":{}}`, "a reference that starts with index.docker.io names docker.io", fixHub},
		{"a name in upper case", `{"Registry.Example":{}}`, "container registry Registry.Example: the name is not a registry host", fixAnyHost},
		{"a name with the port 0", `{"registry.example:0":{}}`, "the name is not a registry host", fixAnyHost},
		{"a name with a path", `{"registry.example/team":{}}`, "the name is not a registry host", fixAnyHost},
		{"a name of 256 characters", `{"` + strings.Repeat("a", 252) + `.com":{}}`, "longer than 255 characters", fixAnyHost},
		{"an unknown transport", `{"docker.io":{"transport":"ftp"}}`, "container registry docker.io: transport is neither https nor http", fixHub},
		{"a transport that is not a word", `{"docker.io":{"transport":{}}}`, "transport is neither https nor http", fixHub},
		{"a mirror with no address", `{"docker.io":{"mirror":{}}}`, needsAddress, fixMirror},
		{"a mirror with an empty address", `{"docker.io":{"mirror":{"address":""}}}`, needsAddress, fixMirror},
		{"a mirror address of 254 characters", `{"docker.io":{"mirror":{"address":"` + strings.Repeat("a", 254) + `"}}}`, needsAddress, fixMirror},
		{"a mirror that is not a node", `{"docker.io":{"mirror":"127.0.0.1"}}`, "the mirror is not a group of statements", fixMirror},
		{"a mirror address in upper case", `{"docker.io":{"mirror":{"address":"Mirror.Example"}}}`, badAddress, fixMirror},
		{"a mirror address that holds its port", `{"docker.io":{"mirror":{"address":"127.0.0.1:5000"}}}`, badAddress, fixMirror},
		{"a mirror address with a scheme", `{"docker.io":{"mirror":{"address":"http://mirror.example"}}}`, badAddress, fixMirror},
		{"a mirror port of 0", `{"docker.io":{"mirror":{"address":"127.0.0.1","port":"0"}}}`, "the mirror's port is below 1", fixMirror},
		{"a mirror port of 65536", `{"docker.io":{"mirror":{"address":"127.0.0.1","port":"65536"}}}`, "the mirror's port is above 65535", fixMirror},
		{"a mirror port that is a word", `{"docker.io":{"mirror":{"address":"127.0.0.1","port":"http"}}}`, "the mirror's port is not a whole number", fixMirror},
		{"a mirror path with no slash", `{"docker.io":{"mirror":{"address":"127.0.0.1","path":"cache"}}}`, "the mirror's path does not start with a slash", fixPath},
		{"a mirror path with two slashes", `{"docker.io":{"mirror":{"address":"127.0.0.1","path":"//evil.example"}}}`, "the mirror's path starts with two slashes", fixPath},
		{"a mirror path with a backslash", `{"docker.io":{"mirror":{"address":"127.0.0.1","path":"/\\evil.example"}}}`, "the mirror's path holds a backslash", fixPath},
		{"a mirror path with a query", `{"docker.io":{"mirror":{"address":"127.0.0.1","path":"/cache?x=1"}}}`, "the mirror's path holds a query, a fragment or a space", fixPath},
		{"an empty mirror path", `{"docker.io":{"mirror":{"address":"127.0.0.1","path":""}}}`, "the mirror's path is not 1 to 255 characters of text", fixPath},
		{"a username with no password", `{"docker.io":{"authentication":{"username":"operator"}}}`, "the authentication needs a password", fixAuth},
		{"a password with no username", `{"docker.io":{"authentication":{"password":"hub-secret"}}}`, "the authentication needs a username", fixAuth},
		{"an empty username", `{"docker.io":{"authentication":{"username":"","password":"hub-secret"}}}`, "the authentication needs a username", fixAuth},
		{"a username of 256 characters", `{"docker.io":{"authentication":{"username":"` + strings.Repeat("u", 256) + `","password":"hub-secret"}}}`, "the authentication needs a username", fixAuth},
		{"an authentication that is not a node", `{"docker.io":{"authentication":"operator"}}`, "the authentication is not a group of statements", fixAuth},
		{"an empty description", `{"docker.io":{"description":""}}`, "the description is not 1 to 255 characters of text", fixText},
		{"a description of 256 characters", `{"docker.io":{"description":"` + strings.Repeat("d", 256) + `"}}`, "the description is not 1 to 255 characters of text", fixText},
	}
	for _, tc := range cases {
		_, err := parseRegistrySections(registrySections(registryBody(tc.entries)))
		if err == nil {
			t.Errorf("%s: accepted", tc.name)
			continue
		}
		refusal, ok := errors.AsType[*opererr.Error](err)
		if !ok {
			t.Errorf("%s: refused with %v, which is not an operator error", tc.name, err)
			continue
		}
		if !strings.Contains(refusal.Problem(), tc.problem) {
			t.Errorf("%s: refused with problem %q, want it to hold %q", tc.name, refusal.Problem(), tc.problem)
		}
		if refusal.Fix() != tc.fix {
			t.Errorf("%s: refused with fix %q, want %q", tc.name, refusal.Fix(), tc.fix)
		}
		if refusal.Why() == "" {
			t.Errorf("%s: the refusal says no why", tc.name)
		}
	}
}

// TestRegistryListBoundsAreInclusive covers the accepted side of each length
// and range bound the refusals cover from outside.
func TestRegistryListBoundsAreInclusive(t *testing.T) {
	accepted := map[string]string{
		"a name of 255 characters":        `{"` + strings.Repeat("a", 251) + `.com":{}}`,
		"a mirror address of 253 bytes":   `{"docker.io":{"mirror":{"address":"` + strings.Repeat("a", 253) + `"}}}`,
		"the mirror ports 1 and 65535":    `{"docker.io":{"mirror":{"address":"127.0.0.1","port":"1"}},"quay.io":{"mirror":{"address":"127.0.0.1","port":"65535"}}}`,
		"a mirror path of one slash":      `{"docker.io":{"mirror":{"address":"127.0.0.1","path":"/"}}}`,
		"a username of 255 characters":    `{"docker.io":{"authentication":{"username":"` + strings.Repeat("u", 255) + `","password":"p"}}}`,
		"a password of 4096 characters":   `{"docker.io":{"authentication":{"username":"u","password":"` + strings.Repeat("p", 4096) + `"}}}`,
		"a description of 255 characters": `{"docker.io":{"description":"` + strings.Repeat("d", 255) + `"}}`,
		"localhost with a port":           `{"localhost:5000":{"transport":"http"}}`,
		"a bracketed IPv6 host":           `{"[::1]:5000":{"transport":"http"}}`,
	}
	for name, entries := range accepted {
		if _, err := parseRegistrySections(registrySections(registryBody(entries))); err != nil {
			t.Errorf("%s: refused: %v", name, err)
		}
	}
}

// TestRefusalNeverRepeatsThePassword proves a refused password is named by
// its leaf and its length only: neither the operator error nor its cause
// holds the value.
func TestRefusalNeverRepeatsThePassword(t *testing.T) {
	password := strings.Repeat("hub-secret-", 400)
	_, err := parseRegistrySections(registrySections(registryBody(
		`{"docker.io":{"authentication":{"username":"operator","password":"` + password + `"}}}`)))
	if err == nil {
		t.Fatal("a password of 4400 characters was accepted")
	}
	refusal, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("refused with %v, which is not an operator error", err)
	}
	if !strings.Contains(refusal.Problem(), "the authentication needs a password of 1 to 4096 characters") {
		t.Errorf("refused with problem %q, want the password bound", refusal.Problem())
	}
	shown := err.Error()
	if cause := errors.Unwrap(err); cause != nil {
		shown += cause.Error()
	}
	if strings.Contains(shown, "hub-secret-") {
		t.Errorf("the refusal repeats the password: %q", shown)
	}
}

// TestRegistryWithNoEntryIsReachedByTheDefault proves a registry the list
// does not name is reached exactly as with no list: verified TLS, anonymous,
// no mirror.
//
// Method: the list holds an entry for another registry. The configuration
// for a short name of the hub equals defaultClientConfig's, under that list
// and under the zero list, and no entry is reported to have decided it.
func TestRegistryWithNoEntryIsReachedByTheDefault(t *testing.T) {
	reference, err := registry.ParseReference("redis")
	if err != nil {
		t.Fatal(err)
	}
	want, err := defaultClientConfig(reference)
	if err != nil {
		t.Fatal(err)
	}
	if want.Transport != registry.TransportHTTPS {
		t.Fatalf("the default transport is %v, want verified TLS", want.Transport)
	}
	if want.Mirror != "" {
		t.Fatalf("the default names the mirror %q, want none", want.Mirror)
	}
	if !want.Credentials.IsZero() {
		t.Fatal("the default holds credentials, want an anonymous client")
	}
	other := mustParseRegistries(t, registryBody(
		`{"registry.example":{"transport":"http","mirror":{"address":"127.0.0.1"},"authentication":{"username":"u","password":"p"}}}`))
	for name, list := range map[string]registryList{"another registry's entry": other, "the zero list": {}} {
		reached, listed, err := listedClientConfig(list, reference)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if listed {
			t.Errorf("%s: an entry is reported to decide how docker.io is reached", name)
		}
		if !reflect.DeepEqual(reached, want) {
			t.Errorf("%s: configuration %#v, want the default %#v", name, reached, want)
		}
	}
}

// TestRegistryEntryDecidesTheClientConfiguration proves the mapping from an
// entry to the registry client's configuration: the transport, the mirror as
// host:port with its path, and the credentials, for the references that name
// the entry's registry and for no other.
//
// Method: an entry named docker.io, the registry a short name completes to.
// The short name redis and the whole spelling get the entry's settings; a
// reference of another registry gets the default.
func TestRegistryEntryDecidesTheClientConfiguration(t *testing.T) {
	list := mustParseRegistries(t, registryBody(`{"docker.io":{"transport":"http",
		"mirror":{"address":"127.0.0.1","port":"5000"},
		"authentication":{"username":"operator","password":"hub-secret"}}}`))
	for _, text := range []string{"redis", "docker.io/library/redis:7", "index.docker.io/library/redis"} {
		reference, err := registry.ParseReference(text)
		if err != nil {
			t.Fatal(err)
		}
		want, err := defaultClientConfig(reference)
		if err != nil {
			t.Fatal(err)
		}
		want.Transport = registry.TransportHTTP
		want.Mirror = "127.0.0.1:5000"
		want.MirrorPath = "/"
		want.Credentials = registry.NewCredentials("operator", "hub-secret")
		reached, listed, err := listedClientConfig(list, reference)
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}
		if !listed {
			t.Errorf("%s: the docker.io entry did not apply", text)
		}
		if !reflect.DeepEqual(reached, want) {
			t.Errorf("%s: configuration %#v, want %#v", text, reached, want)
		}
		client, err := registry.New(reached)
		if err != nil {
			t.Fatalf("%s: the registry client refuses the configuration: %v", text, err)
		}
		client.Close()
	}
	elsewhere, err := registry.ParseReference("ghcr.io/team/app:1")
	if err != nil {
		t.Fatal(err)
	}
	reached, listed, err := listedClientConfig(list, elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	if listed {
		t.Error("the docker.io entry applied to a reference of ghcr.io")
	}
	unlisted, err := defaultClientConfig(elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reached, unlisted) {
		t.Errorf("ghcr.io is reached by %#v, want the default %#v", reached, unlisted)
	}
}

// TestSchemaAcceptsRegistryEntry proves the first check of the pair: the
// config loader accepts an entry that holds every node of the registry list,
// and an entry named by a host with its port.
func TestSchemaAcceptsRegistryEntry(t *testing.T) {
	const whole = `container {
	registry docker.io {
		transport http
		mirror {
			address 127.0.0.1
			port 5000
			path /cache
		}
		authentication {
			username operator
			password hub-secret
		}
		description "the hub through the lab mirror"
	}
	registry registry.example:5000 {
		mirror {
			address mirror.example
		}
	}
}
`
	if _, err := config.LoadConfig(whole, "test.conf", nil); err != nil {
		t.Fatalf("the loader refused a registry entry with every node: %v", err)
	}
}

// TestSchemaRefusesMalformedRegistryEntry proves the schema refuses, before
// any delivery reaches the store's parse, each entry the parse also refuses:
// a transport outside the enumeration, a mirror with no address, half a
// credential, a port of 0 and a path with no leading slash.
//
// Method: each case is the body of one docker.io entry. The control is the
// same entry with the valid value, which loads, so a refusal is the node's
// and not the entry's.
//
// MUTATION: remove "mandatory true" from the mirror's address leaf or from
// the password leaf and that case loads.
func TestSchemaRefusesMalformedRegistryEntry(t *testing.T) {
	entry := func(body string) string {
		return "container {\n\tregistry docker.io {\n" + body + "\t}\n}\n"
	}
	cases := []struct {
		name    string
		refused string
		control string
		path    string
	}{
		{"a transport outside the enumeration", "\t\ttransport ftp\n", "\t\ttransport http\n", ""},
		{
			"a mirror with no address", "\t\tmirror {\n\t\t\tport 5000\n\t\t}\n",
			"\t\tmirror {\n\t\t\taddress 127.0.0.1\n\t\t\tport 5000\n\t\t}\n", "mirror/address",
		},
		{
			"a username with no password", "\t\tauthentication {\n\t\t\tusername operator\n\t\t}\n",
			"\t\tauthentication {\n\t\t\tusername operator\n\t\t\tpassword hub-secret\n\t\t}\n", "authentication/password",
		},
		{
			"a mirror port of 0", "\t\tmirror {\n\t\t\taddress 127.0.0.1\n\t\t\tport 0\n\t\t}\n",
			"\t\tmirror {\n\t\t\taddress 127.0.0.1\n\t\t\tport 1\n\t\t}\n", "",
		},
		{
			"a mirror path with no leading slash", "\t\tmirror {\n\t\t\taddress 127.0.0.1\n\t\t\tpath cache\n\t\t}\n",
			"\t\tmirror {\n\t\t\taddress 127.0.0.1\n\t\t\tpath /cache\n\t\t}\n", "",
		},
	}
	for _, tc := range cases {
		_, err := config.LoadConfig(entry(tc.refused), "test.conf", nil)
		if err == nil {
			t.Errorf("%s: the loader accepted it", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.path) {
			t.Errorf("%s: the refusal %q does not name %s", tc.name, err, tc.path)
		}
		if _, err := config.LoadConfig(entry(tc.control), "test.conf", nil); err != nil {
			t.Errorf("%s: the loader refused the control: %v", tc.name, err)
		}
	}
}
