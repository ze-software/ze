// Design: docs/architecture/container/image-store.md -- the registry list: how the configuration says a registry is reached
// Related: register.go -- registers parseRegistrySections and the apply as the store's config sibling
// Related: registry_config.go -- engineClientConfig, which reads the applied list at each fetch
// Related: fetch_steps.go -- defaultClientConfig, how a registry with no entry is reached

package imagestore

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// registrySiblingName names the image store in the container plugin's config
// transaction.
const registrySiblingName = "image-store"

// The config words this package reads, as the container schema spells them
// (../yang/ze-container-conf.yang).
const (
	containerRoot      = "container"
	listRegistry       = "registry"
	leafTransport      = "transport"
	nodeMirror         = "mirror"
	leafAddress        = "address"
	leafPort           = "port"
	leafPath           = "path"
	nodeAuthentication = "authentication"
	leafUsername       = "username"
	leafPassword       = "password"
	leafDescription    = "description"
	wordHTTPS          = "https"
	wordHTTP           = "http"
)

// Bounds and defaults the schema states. Each bound is checked again here,
// the second check of the pair, for a delivery that did not pass the schema.
const (
	registryNameLengthMax  = 255
	mirrorAddressLengthMax = 253
	mirrorPortDefault      = 443
	mirrorPortMin          = 1
	mirrorPortMax          = 65535
	mirrorPathDefault      = "/"
	registryTextLengthMax  = 255
	passwordLengthMax      = 4096
)

// registryList is the image store's part of the container config: one entry
// per registry that needs settings. Its zero value holds no entry, so every
// registry is reached by defaultClientConfig. It holds no func and no
// channel: the config transaction compares two lists with reflect.DeepEqual.
type registryList struct {
	// entries are sorted by host. Nil when the config holds none, so the list
	// of an empty config equals the zero list.
	entries []registryEntry
}

// registryEntry is one entry of the registry list. Only parseRegistryEntry
// builds one, so its host is one a reference completes to and its mirror one
// the registry client accepts.
type registryEntry struct {
	// host is the registry as a reference completes it: the list's key.
	host      string
	transport registry.Transport
	// mirror is host:port of the registry's mirror, and empty when the entry
	// names none. mirrorPath is the path the mirror serves under, set with
	// mirror only.
	mirror     string
	mirrorPath string
	// credentials are the authentication node, or the zero value, which is
	// anonymous. They print as a placeholder under every verb.
	credentials registry.Credentials
}

// entry answers the entry of the registry host, and false when the list
// holds none: that registry is reached by the default.
func (l registryList) entry(host string) (registryEntry, bool) {
	for _, entry := range l.entries {
		if entry.host == host {
			return entry, true
		}
	}
	return registryEntry{}, false
}

// clientConfig answers base as the entry changes it. The entry states the
// whole of how its registry is reached: its transport, its mirror or none,
// its credentials or none. The credentials go into the client's registry
// field alone, which the client never sends to the mirror (registry.New).
func (e registryEntry) clientConfig(base registry.Config) registry.Config {
	base.Transport = e.transport
	base.Mirror = e.mirror
	base.MirrorPath = e.mirrorPath
	base.Credentials = e.credentials
	return base
}

// listedClientConfig answers how the registry reference names is reached
// under list, and whether an entry of the list decided it: the entry's
// settings over the default, or the default alone for a registry with no
// entry.
func listedClientConfig(list registryList, reference registry.Reference) (registry.Config, bool, error) {
	config, err := defaultClientConfig(reference)
	if err != nil {
		return registry.Config{}, false, err
	}
	entry, listed := list.entry(reference.Registry())
	if !listed {
		return config, false, nil
	}
	return entry.clientConfig(config), true, nil
}

// appliedRegistryList holds the registry list of the running config. The
// config transaction writes it and each fetch reads it, so a change reaches
// the next fetch with nothing restarted. Safe for concurrent use.
type appliedRegistryList struct {
	mu   sync.Mutex
	list registryList
}

// appliedRegistries is the list of this process, written through the config
// sibling register.go installs.
var appliedRegistries appliedRegistryList

// apply makes list the running registry list. It is the store's apply in the
// config transaction: it only stores, so it is idempotent, cannot fail, and
// a rollback is the same call with the previous list. A fetch in flight
// keeps the client it built.
func (a *appliedRegistryList) apply(_ context.Context, list registryList) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.list = list
	return nil
}

// current answers the running registry list.
func (a *appliedRegistryList) current() registryList {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.list
}

// parseRegistrySections reads and judges the registry list of the delivered
// container root. It is the store's parse in the config transaction, so it
// runs at `ze config validate`, at a reload's verify and at the boot
// configure, and changes no state. Every refusal is an opererr.Error.
// Whether a registry answers is not judged here: the fetch reports it.
func parseRegistrySections(sections []sdk.ConfigSection) (registryList, error) {
	for _, section := range sections {
		if section.Root != containerRoot {
			continue
		}
		var delivered map[string]any
		if err := json.Unmarshal([]byte(section.Data), &delivered); err != nil {
			return registryList{}, opererr.Wrap(err,
				"the registry list could not be read: the delivered container section is not a JSON object",
				"the image store reads how each registry is reached from that section",
				"ze config validate",
			)
		}
		// A removed root unwraps to a nil body, which holds no entry.
		return parseRegistryList(configvalue.Section(containerRoot, delivered))
	}
	return registryList{}, nil
}

// parseRegistryList reads the entries of the container root's body.
func parseRegistryList(body map[string]any) (registryList, error) {
	var list registryList
	for _, entry := range configvalue.ListEntries(body[listRegistry]) {
		parsed, err := parseRegistryEntry(entry)
		if err != nil {
			return registryList{}, err
		}
		list.entries = append(list.entries, parsed)
	}
	return list, nil
}

// parseRegistryEntry reads one entry of the registry list.
func parseRegistryEntry(entry configvalue.ListEntry) (registryEntry, error) {
	if err := checkRegistryName(entry.Key); err != nil {
		return registryEntry{}, err
	}
	parsed := registryEntry{host: entry.Key}
	var err error
	parsed.transport, err = parseTransport(entry)
	if err != nil {
		return registryEntry{}, err
	}
	if raw, present := entry.Fields[nodeMirror]; present {
		parsed.mirror, parsed.mirrorPath, err = parseMirror(entry.Key, raw)
		if err != nil {
			return registryEntry{}, err
		}
	}
	if raw, present := entry.Fields[nodeAuthentication]; present {
		parsed.credentials, err = parseAuthentication(entry.Key, raw)
		if err != nil {
			return registryEntry{}, err
		}
	}
	if _, present := entry.Fields[leafDescription]; present {
		_, err = boundedText(entry.Fields, leafDescription, registryTextLengthMax)
		if err != nil {
			return registryEntry{}, opererr.Wrap(err,
				registryProblem(entry.Key, "the description is not 1 to 255 characters of text"),
				"the description is free text for the operator, shown with the registry",
				setRegistryStatement(entry.Key, leafDescription+" TEXT"),
			)
		}
	}
	return parsed, nil
}

// checkRegistryName refuses a key that no image reference completes to: the
// entry would never apply, and the operator would believe it does. The
// grammar is the reference's own (registry.CheckHost, registry.ParseReference),
// so the list and the references cannot disagree about a host.
func checkRegistryName(name string) error {
	const why = "an entry applies to the image references that name its registry, so its name is that registry as the references write it: a host in lower case with a dot or a colon, or localhost, and docker.io for the public hub"
	fix := setRegistryStatement("HOST", leafTransport+" "+wordHTTPS)
	if name == "" {
		return opererr.New("a container registry entry has an empty name", why, fix)
	}
	if len(name) > registryNameLengthMax {
		return opererr.New("a container registry name is longer than 255 characters", why, fix)
	}
	if err := registry.CheckHost(name); err != nil {
		return opererr.Wrap(err, registryProblem(name, "the name is not a registry host"), why, fix)
	}
	reference, err := registry.ParseReference(name + "/image")
	if err != nil {
		return opererr.Wrap(err, registryProblem(name, "the name is not a registry an image reference can name"), why, fix)
	}
	if reference.Registry() != name {
		return opererr.New(
			registryProblem(name, "no image reference names that registry: a reference that starts with "+name+" names "+reference.Registry()),
			why,
			setRegistryStatement(reference.Registry(), leafTransport+" "+wordHTTPS),
		)
	}
	return nil
}

// parseTransport reads the transport leaf of one entry, or its default.
func parseTransport(entry configvalue.ListEntry) (registry.Transport, error) {
	raw, present := entry.Fields[leafTransport]
	if !present {
		return registry.TransportHTTPS, nil
	}
	word, isWord := raw.(string)
	if !isWord {
		return registry.TransportUnspecified, refuseTransport(entry.Key)
	}
	switch word {
	case wordHTTPS:
		return registry.TransportHTTPS, nil
	case wordHTTP:
		return registry.TransportHTTP, nil
	}
	return registry.TransportUnspecified, refuseTransport(entry.Key)
}

func refuseTransport(name string) error {
	return opererr.New(
		registryProblem(name, "transport is neither https nor http"),
		"the leaf names the protocol Ze speaks to the registry and to its mirror, so Ze refuses a value it cannot read rather than pick one",
		setRegistryStatement(name, leafTransport+" "+wordHTTPS),
	)
}

// parseMirror reads the mirror node of one entry and answers the mirror as
// the registry client takes it: host:port, and the path it serves under.
func parseMirror(name string, raw any) (mirror, path string, err error) {
	const why = "the mirror is asked for every manifest and blob before the registry, so Ze refuses one it could not reach rather than fetch without it"
	fix := setRegistryStatement(name, nodeMirror+" "+leafAddress+" ADDRESS "+leafPort+" PORT")
	fields, isNode := raw.(map[string]any)
	if !isNode {
		return "", "", opererr.New(registryProblem(name, "the mirror is not a group of statements"), why, fix)
	}
	address, err := boundedText(fields, leafAddress, mirrorAddressLengthMax)
	if err != nil {
		return "", "", opererr.Wrap(err, registryProblem(name, "the mirror needs an address of 1 to 253 characters"), why, fix)
	}
	port := mirrorPortDefault
	if rawPort, present := fields[leafPort]; present {
		number, read := configvalue.Int(rawPort)
		if !read {
			return "", "", opererr.New(registryProblem(name, "the mirror's port is not a whole number"), why, fix)
		}
		if number < mirrorPortMin {
			return "", "", opererr.New(registryProblem(name, "the mirror's port is below 1"), why, fix)
		}
		if number > mirrorPortMax {
			return "", "", opererr.New(registryProblem(name, "the mirror's port is above 65535"), why, fix)
		}
		// Between 1 and 65535, checked above.
		port = int(number)
	}
	path = mirrorPathDefault
	if _, present := fields[leafPath]; present {
		path, err = parseMirrorPath(name, fields)
		if err != nil {
			return "", "", err
		}
	}
	// JoinHostPort brackets an IPv6 address, the form a registry host takes.
	mirror = net.JoinHostPort(address, strconv.Itoa(port))
	err = registry.CheckHost(mirror)
	if err != nil {
		return "", "", opererr.Wrap(err,
			registryProblem(name, "the mirror's address is neither an IP address nor a host name in lower case"),
			why, fix,
		)
	}
	return mirror, path, nil
}

// parseMirrorPath reads the path leaf of a mirror node that holds one.
func parseMirrorPath(name string, fields map[string]any) (string, error) {
	const why = "the path is placed before the /v2/ path of every request to the mirror, so it starts with a slash and holds no query, fragment or space"
	fix := setRegistryStatement(name, nodeMirror+" "+leafPath+" /PATH")
	path, err := boundedText(fields, leafPath, registryTextLengthMax)
	if err != nil {
		return "", opererr.Wrap(err, registryProblem(name, "the mirror's path is not 1 to 255 characters of text"), why, fix)
	}
	if !strings.HasPrefix(path, "/") {
		return "", opererr.New(registryProblem(name, "the mirror's path does not start with a slash"), why, fix)
	}
	if strings.ContainsAny(path, "?# ") {
		return "", opererr.New(registryProblem(name, "the mirror's path holds a query, a fragment or a space"), why, fix)
	}
	return path, nil
}

// parseAuthentication reads the authentication node of one entry. Both
// leaves are mandatory: half a credential is refused, never sent.
func parseAuthentication(name string, raw any) (registry.Credentials, error) {
	const why = "the registry is sent a user name and a password together, so Ze refuses an entry that holds one without the other rather than fetch anonymously"
	fix := container.FixRegistryCredentials(name)
	fields, isNode := raw.(map[string]any)
	if !isNode {
		return registry.Credentials{}, opererr.New(registryProblem(name, "the authentication is not a group of statements"), why, fix)
	}
	username, err := boundedText(fields, leafUsername, registryTextLengthMax)
	if err != nil {
		return registry.Credentials{}, opererr.Wrap(err, registryProblem(name, "the authentication needs a username of 1 to 255 characters"), why, fix)
	}
	password, err := boundedText(fields, leafPassword, passwordLengthMax)
	if err != nil {
		// The cause names the leaf and its length only, never its value.
		return registry.Credentials{}, opererr.Wrap(err, registryProblem(name, "the authentication needs a password of 1 to 4096 characters"), why, fix)
	}
	return registry.NewCredentials(username, password), nil
}

// boundedText reads the text leaf named leaf of fields, 1 to lengthMax bytes
// long. The error says what is wrong with the leaf and never repeats its
// value, which may be a password; the caller says whose leaf it is.
func boundedText(fields map[string]any, leaf string, lengthMax int) (string, error) {
	raw, present := fields[leaf]
	if !present {
		return "", errors.New("no " + leaf + " is set")
	}
	text, isText := raw.(string)
	if !isText {
		return "", errors.New("the " + leaf + " is not a string")
	}
	if text == "" {
		return "", errors.New("the " + leaf + " is empty")
	}
	if len(text) > lengthMax {
		return "", errors.New("the " + leaf + " is " + strconv.Itoa(len(text)) + " characters long")
	}
	return text, nil
}

// registryProblem answers the problem sentence of a refused entry.
func registryProblem(name, defect string) string {
	return "container registry " + name + ": " + defect
}

// setRegistryStatement answers the config statement that sets statement on
// the entry name.
func setRegistryStatement(name, statement string) string {
	return "set container registry " + name + " " + statement
}
