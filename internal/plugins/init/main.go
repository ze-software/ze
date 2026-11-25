// Design: docs/architecture/system-architecture.md — ze init bootstrap command

// Package init provides the `ze init` command that bootstraps the live store
// or builds an explicit appliance seed artifact.
package init

import (
	"bufio"
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/fetch"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// Key aliases for readability (from zefs key registry).
var (
	keyIdentityName = zefs.KeyInstanceName.Pattern
	keyManaged      = zefs.KeyInstanceManaged.Pattern
)

const (
	// flagManaged is the one flag token the parser, the help page and the
	// completion inventory each spell.
	flagManaged = "--managed"
	// flagHubCA and flagHubTrustFirstUse are the two ways a managed client is
	// told how to authenticate its hub before it holds a config.
	flagHubCA            = "--hub-ca"
	flagHubTrustFirstUse = "--hub-trust-first-use"
	// flagConfirm asks about each discovered interface before the initial
	// config claims it.
	flagConfirm = "--confirm"
	// hubTokenMin is the shortest secret a hub accepts for a client
	// (plugin/hub/server/client/secret in ze-hub-conf.yang).
	hubTokenMin = 32
	defaultHost = "127.0.0.1"
	defaultPort = "2222"
)

// Run executes ze init from CLI arguments.
// Returns exit code.
func Run(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	managedFlag := fs.Bool("managed", false, "Enable managed (fleet) mode")
	hubCAFlag := fs.String("hub-ca", "", "With --managed: file holding the hub's certificate authority root (PEM), from 'ze show pki local-ca pem' on the hub")
	hubTrustFirstUseFlag := fs.Bool("hub-trust-first-use", false, "With --managed: keep the certificate authority the hub presents on the first connection")
	forceFlag := fs.Bool("force", false, "Replace existing database (moves old to .replaced-<date>)")
	yesFlag := fs.Bool("yes", false, "Skip confirmation prompt (use with --force)")
	webCertFlag := fs.String("web-cert", "", "Generate TLS certificate for web server (listen address, e.g. 0.0.0.0:8080)")
	webCertNameFlag := fs.String("web-cert-name", "", "Extra DNS name for the TLS certificate SAN (e.g. router.example.com)")
	seedFlag := fs.Bool("seed", false, "Create a database.zefs appliance seed artifact without build-host interface discovery")
	confirmFlag := fs.Bool("confirm", false, "Ask about each discovered interface and write only the ones answered yes into the initial config")
	fromFlag := fs.String("from", "", "Import a blob artifact from a local path or a URL into the live store instead of prompting for credentials")
	sha256Flag := fs.String("sha256", "", "Refuse the --from source unless its bytes have this SHA-256 (64 hex digits)")
	schemes := strings.Join(fetch.Schemes(), ", ")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze init",
			ShortHelp: "Bootstrap the ze database with SSH credentials",
			Usage:     []string{"ze init [options]", "ze init --from <source> [--sha256 <hex>] [--force [--yes]]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Input (stdin or interactive prompts)", Entries: []helpfmt.HelpEntry{
					{Name: "Line 1: username", Desc: ""},
					{Name: "Line 2: password", Desc: ""},
					{Name: "Line 3: host", Desc: "(default: 127.0.0.1) the IP address the written config's SSH listener binds and the CLI connects to; --seed and --managed write no listener and also take a host name"},
					{Name: "Line 4: port", Desc: "(default: 2222) the SSH listener's port, from 1 through 65535"},
					{Name: "Line 5: name", Desc: "(default: hostname)"},
					{Name: "Line 6: hub address", Desc: "(--managed only) host:port of the hub; empty to name it at the first start"},
					{Name: "Line 7: hub token", Desc: "(--managed only) this client's secret on the hub, 32 characters or more"},
					{Name: "Then: one line per device", Desc: "(--confirm only) for each discovered interface the config can hold, in 'ze interface scan' order: yes or y writes it, no or n leaves it out; any other line, an empty one included, is not accepted and the same interface is asked again"},
				}},
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: flagManaged, Desc: "Enable managed (fleet) mode"},
					{Name: flagHubCA + " <file>", Desc: "With --managed: the hub's certificate authority root (PEM), printed on the hub by 'ze show pki local-ca pem'"},
					{Name: flagHubTrustFirstUse, Desc: "With --managed: keep the certificate authority the hub presents on the first connection, unverified"},
					{Name: "--force", Desc: "Replace existing database (moves old to .replaced-<date>)"},
					{Name: "--yes", Desc: "Skip confirmation prompt (use with --force)"},
					{Name: "--web-cert <addr>", Desc: "Generate TLS certificate for web server (e.g. 0.0.0.0:8080)"},
					{Name: "--web-cert-name <host>", Desc: "Extra DNS name for TLS certificate SAN (e.g. router.example.com)"},
					{Name: "--seed", Desc: "Create a blob artifact for appliance builders; skip build-host interface discovery"},
					{Name: flagConfirm, Desc: "Ask about each discovered interface and write only the ones answered yes; only yes, y, no or n answers, and anything else is asked again; without it every discovered interface is written. Refused with --seed, --managed and --from, which write no discovered interface"},
					{Name: "--from <source>", Desc: "Import a blob artifact (database.zefs) from a local path or a URL (" + schemes + ") into the live store; a local blob is retired as .replaced-<date>, a fetched copy is removed; reads no credentials"},
					{Name: "--sha256 <hex>", Desc: "Check the --from source against this SHA-256 before a key is written"},
				}},
			},
			Examples: []string{
				`echo -e "admin\nsecret\n127.0.0.1\n2222\nmy-router" | ze init`,
				"ze init --confirm  (ask before writing each discovered interface)",
				"ze init --managed  (interactive prompts, managed mode)",
				"ze init --managed --hub-ca hub-root.pem   (authenticate the hub against its root)",
				"ze init --force         (replace existing database)",
				"ze init --force --yes   (replace without confirmation)",
				"ze init --from database.zefs   (import a blob into the live store)",
				"ze init --from https://provision.example.net/install/database.zefs --sha256 <hex>",
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	fleet, fleetErr := managedInitFromFlags(*managedFlag, *hubCAFlag, *hubTrustFirstUseFlag)
	if fleetErr != nil {
		fmt.Fprintln(os.Stderr, fleetErr.Error())
		return 1
	}
	profile, profileErr := profileFromFlags(*confirmFlag, *seedFlag, *managedFlag, *fromFlag)
	if profileErr != nil {
		fmt.Fprintln(os.Stderr, profileErr.Error())
		return 1
	}

	if *sha256Flag != "" && *fromFlag == "" {
		fmt.Fprintf(os.Stderr, "error: --sha256 checks the --from source and needs --from\n")
		return 1
	}
	if *sha256Flag != "" {
		if err := fetch.ValidSHA256(*sha256Flag); err != nil {
			fmt.Fprintf(os.Stderr, "error: --sha256: %v\n", err)
			return 1
		}
	}
	if fetch.IsRemote(*fromFlag) {
		if _, err := fetch.Resolve(*fromFlag); err != nil {
			fmt.Fprintf(os.Stderr, "error: --from %s: %v\n", redact.URL(*fromFlag), err)
			return 1
		}
	}
	if *fromFlag != "" && *seedFlag {
		fmt.Fprintf(os.Stderr, "error: --from imports a blob into the live store; --seed creates one, so the two cannot be combined\n")
		return 1
	}
	// --from publishes the blob as it is: the flags that shape a NEW store
	// would be silently ignored, so each is refused by name.
	if *fromFlag != "" {
		for _, ignored := range []struct {
			set  bool
			name string
		}{{*managedFlag, flagManaged}, {*webCertFlag != "", "--web-cert"}, {*webCertNameFlag != "", "--web-cert-name"}} {
			if ignored.set {
				fmt.Fprintf(os.Stderr, "error: --from imports a blob as it is; %s shapes a new store and cannot be combined with it\n", ignored.name)
				return 1
			}
		}
	}

	dbPath := sshclient.ResolveStoreDir("")
	if dbPath == "" {
		fmt.Fprintf(os.Stderr, "error: cannot determine database location\n")
		return 1
	}
	if *fromFlag != "" {
		if !confirmForce(dbPath, *forceFlag, *yesFlag) {
			return 1
		}
		return runImport(*fromFlag, *sha256Flag, dbPath, *forceFlag)
	}

	// When piped, read all data first so --force can prompt on /dev/tty.
	var inputReader io.Reader = os.Stdin
	var promptWriter io.Writer
	interactive := isTerminal(os.Stdin)
	if interactive {
		promptWriter = os.Stderr
	} else {
		data, readErr := io.ReadAll(os.Stdin)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "error: reading stdin: %v\n", readErr)
			return 1
		}
		inputReader = bytes.NewReader(data)
	}

	if !confirmForce(dbPath, *forceFlag, *yesFlag) {
		return 1
	}

	return runInitStore(inputReader, promptWriter, dbPath, fleet, *webCertFlag, *webCertNameFlag, *seedFlag, *forceFlag, profile)
}

// profileFromFlags answers which discovered interfaces the initial config
// claims: every one by default, and with --confirm only those the operator
// answers yes for. --from, --seed and --managed write no discovered interface,
// so --confirm with any of them would ask nothing; it is refused by name
// rather than silently ignored.
func profileFromFlags(confirm, seed, managed bool, from string) (initProfile, error) {
	if !confirm {
		return initDiscover, nil
	}
	if from != "" {
		return 0, opererr.New(flagConfirm+" was given with --from",
			"--from imports a blob as it is, so ze init discovers no interface to ask about",
			"ze init --from "+redact.URL(from))
	}
	if seed {
		return 0, opererr.New(flagConfirm+" was given with --seed",
			"a seed writes no initial config: the appliance discovers its own interfaces at first boot",
			"ze init --seed")
	}
	if managed {
		return 0, opererr.New(flagConfirm+" was given with "+flagManaged,
			"a managed client writes no initial config: its first config comes from its hub",
			"ze init "+flagManaged)
	}
	return initDiscoverConfirmed, nil
}

// managedInit is what ze init was told about managed (fleet) mode. The zero
// value is an instance that is not managed.
type managedInit struct {
	enabled bool
	// hubCA is the hub's certificate authority root in PEM, already checked
	// to hold one CA certificate. Empty when none was given.
	hubCA []byte
	// trustFirstUse asks the first start to keep the authority the hub
	// presents. It is never set together with hubCA.
	trustFirstUse bool
}

// managedInitFromFlags judges the managed flags together and reads the hub
// root. Each refusal names the flags that fix it.
func managedInitFromFlags(managed bool, hubCAPath string, trustFirstUse bool) (managedInit, error) {
	if !managed {
		if hubCAPath != "" {
			return managedInit{}, opererr.New(flagHubCA+" was given without "+flagManaged,
				"the hub root is only read by a managed client", "ze init --managed --hub-ca "+hubCAPath)
		}
		if trustFirstUse {
			return managedInit{}, opererr.New(flagHubTrustFirstUse+" was given without "+flagManaged,
				"only a managed client connects to a hub", "ze init --managed --hub-trust-first-use")
		}
		return managedInit{}, nil
	}
	fleet := managedInit{enabled: true, trustFirstUse: trustFirstUse}
	if hubCAPath == "" {
		return fleet, nil
	}
	if trustFirstUse {
		return managedInit{}, opererr.New(flagHubCA+" and "+flagHubTrustFirstUse+" were both given",
			"a hub root that is given is the one to trust, so there is nothing left to take from the first connection",
			"ze init --managed --hub-ca "+hubCAPath)
	}
	data, err := os.ReadFile(hubCAPath) //nolint:gosec // cliio:allow ze init reads credentials on stdin, so --hub-ca must name a file
	if err != nil {
		return managedInit{}, opererr.New("the hub root "+hubCAPath+" could not be read",
			"ze init stores the root so the first start can authenticate the hub",
			"ze show pki local-ca pem   (on the hub; save what it prints, then repeat ze init with that file)")
	}
	if !holdsOneCA(data) {
		return managedInit{}, opererr.New("the hub root "+hubCAPath+" is not one PEM certificate of a certificate authority",
			"the first start authenticates the hub against that root",
			"ze show pki local-ca pem   (on the hub; save what it prints, then repeat ze init with that file)")
	}
	fleet.hubCA = data
	return fleet, nil
}

// tokenTooShort reports whether a hub token was given and is shorter than a
// hub accepts. An empty token was not given: the first start names it.
func tokenTooShort(token string) bool {
	if token == "" {
		return false
	}
	return len(token) < hubTokenMin
}

// holdsOneCA reports whether data is one PEM CERTIFICATE block, and nothing
// else, whose certificate may sign others.
func holdsOneCA(data []byte) bool {
	block, rest := pem.Decode(data)
	if block == nil {
		return false
	}
	if block.Type != "CERTIFICATE" {
		return false
	}
	if len(bytes.TrimSpace(rest)) != 0 {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	return cert.IsCA
}

// confirmForce asks before --force replaces a store, unless --yes was given.
// Confirmation precedes staging; replacement itself holds the storage lock.
func confirmForce(dbPath string, force, yes bool) bool {
	if !force {
		return true
	}
	for _, name := range []string{"database", "database.zefs"} {
		path := filepath.Join(dbPath, name)
		if _, err := os.Lstat(path); err != nil {
			continue
		}
		if yes {
			return true
		}
		if !confirmForceReplace(path) {
			fmt.Fprintf(os.Stderr, "aborted\n")
			return false
		}
		return true
	}
	return true
}

// fetchStagePrefix names the private folder a remote --from source is fetched
// into. It sits beside the store rather than in the system temp folder, so
// the copy an unfinished import records survives a reboot and its recovery
// command still finds it.
const fetchStagePrefix = "database.fetch-"

// fetchedName is the fetched copy's name inside its staging folder.
const fetchedName = "fetched.zefs"

// runImport publishes a blob as the live tree. A remote source is fetched
// first, and a local one is read in place; with expectedSHA the bytes are
// checked before storage.ImportBlob runs. ImportBlob checks the blob (zefs.Check)
// before it writes a key, holds the ownership lock (so a running daemon refuses
// it), refuses an existing tree unless force moved it aside, and retires the
// blob.
func runImport(from, expectedSHA, dir string, force bool) int {
	if fetch.IsRemote(from) {
		return runFetchImport(from, expectedSHA, dir, force)
	}
	if expectedSHA != "" {
		if err := fetch.CheckSHA256(from, expectedSHA); err != nil {
			fmt.Fprintf(os.Stderr, "error: import %s: %v\n", from, err)
			return 1
		}
	}
	return importSource(from, from, dir, force)
}

// runFetchImport fetches a remote source into a private staging folder beside
// the store, imports the copy, and removes the folder: the fetched copy, the
// retired name the import leaves for it, and its lock. A failed fetch, digest
// or check imports nothing and removes the folder too. The one exception is an
// import that stopped after recording its intent: the recovery command names
// the fetched copy, so the copy stays and the error says where.
func runFetchImport(from, expectedSHA, dir string, force bool) int {
	shown := redact.URL(from)
	fetcher, err := fetch.Resolve(from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: --from %s: %v\n", shown, err)
		return 1
	}
	// Absolute, because the import intent records its source absolute and the
	// pending check below compares the fetched copy with that record.
	dir, err = filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve %s: %v\n", dir, err)
		return 1
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "error: create %s: %v\n", dir, err)
		return 1
	}
	staging, err := os.MkdirTemp(dir, fetchStagePrefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: create fetch folder in %s: %v\n", dir, err)
		return 1
	}
	fetched := filepath.Join(staging, fetchedName)

	if err := fetcher(from, fetched, expectedSHA); err != nil {
		fmt.Fprintf(os.Stderr, "error: fetch %s: %v\n", shown, err)
		return removeStaging(staging, 1)
	}
	code := importSource(fetched, shown, dir, force)
	if code == 0 {
		return removeStaging(staging, 0)
	}
	source, pending, err := storage.PendingImportSource(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: the fetched copy stays at %s: the import intent cannot be read: %v\n", fetched, err)
		return code
	}
	if pending && source == fetched {
		fmt.Fprintf(os.Stderr, "error: the fetched copy stays at %s: the unfinished import records it\n", fetched)
		return code
	}
	return removeStaging(staging, code)
}

// removeStaging removes a fetch staging folder and returns code, or 1 when the
// removal fails, so a left-behind copy of the credentials is never silent.
func removeStaging(staging string, code int) int {
	if err := os.RemoveAll(staging); err != nil {
		fmt.Fprintf(os.Stderr, "error: remove fetched copy %s: %v\n", staging, err)
		return 1
	}
	return code
}

// importSource runs the import of the blob at path and reports it as shown,
// the operator's own spelling of the source with any URL userinfo redacted.
func importSource(path, shown, dir string, force bool) int {
	importer := storage.ImportBlob
	if force {
		importer = storage.ReplaceImportBlob
	}
	store, err := importer(path, dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: import %s: %v\n", shown, err)
		return 1
	}
	tree := filepath.Join(dir, "database")
	if err := store.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error: close %s: %v\n", tree, err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "imported %s into %s\n", shown, tree) //nolint:errcheck // status output
	return 0
}

func runInit(r io.Reader, promptW io.Writer, dir string, managed bool, webCertAddr, webCertName string, seed, force bool) int {
	return runInitFleet(r, promptW, dir, managedInit{enabled: managed}, webCertAddr, webCertName, seed, force)
}

// runInitFleet is runInit with everything ze init was told about managed mode.
func runInitFleet(r io.Reader, promptW io.Writer, dir string, fleet managedInit, webCertAddr, webCertName string, seed, force bool) int {
	return runInitStore(r, promptW, dir, fleet, webCertAddr, webCertName, seed, force, initDiscover)
}

// runInitStore is runInitFleet with the profile that decides which discovered
// interfaces the initial config claims (profileFromFlags).
func runInitStore(r io.Reader, promptW io.Writer, dir string, fleet managedInit, webCertAddr, webCertName string, seed, force bool, profile initProfile) int {
	path := filepath.Join(dir, "database")
	if seed {
		path = filepath.Join(dir, "database.zefs")
	}
	publish := func(populate func(storage.Storage) error) (storage.Storage, error) {
		if seed {
			return storage.CreateBlobPopulated(path, populate, force)
		}
		if force {
			return storage.ReplacePopulated(dir, populate)
		}
		return storage.CreatePopulated(dir, populate)
	}
	return runInitPublishedProfile(r, promptW, path, fleet, webCertAddr, webCertName, seed, publish, profile)
}

// runInitPublished shares credential population between ordinary init and the
// descriptor-bound session entry. Only the publication operation differs.
func runInitPublished(r io.Reader, promptW io.Writer, path string, managed bool, webCertAddr, webCertName string, seed bool, publish func(func(storage.Storage) error) (storage.Storage, error)) int {
	return runInitPublishedProfile(r, promptW, path, managedInit{enabled: managed}, webCertAddr, webCertName, seed, publish, initDiscover)
}

// initProfile says which of the host's interfaces the initial config claims.
type initProfile uint8

const (
	// initDiscover writes every interface discovery finds: the default.
	initDiscover initProfile = iota + 1
	// initDiscoverConfirmed asks about each discovered interface the config
	// can hold and writes only the ones answered yes (--confirm).
	initDiscoverConfirmed
	// initLocalService writes none: `ze install` claims no host interface.
	initLocalService
)

// runInstallInit initializes the native daemon without claiming host interfaces.
func runInstallInit(r io.Reader, promptW io.Writer, dir string) int {
	publish := func(populate func(storage.Storage) error) (storage.Storage, error) {
		return storage.CreatePopulated(dir, populate)
	}
	return runInitPublishedProfile(r, promptW, filepath.Join(dir, "database"), managedInit{}, "", "", false, publish, initLocalService)
}

func runInitPublishedProfile(r io.Reader, promptW io.Writer, path string, fleet managedInit, webCertAddr, webCertName string, seed bool, publish func(func(storage.Storage) error) (storage.Storage, error), profile initProfile) int {
	// Read credentials (with optional prompts)
	scanner := bufio.NewScanner(r)

	username := promptAndRead(scanner, promptW, "username: ")

	var password string
	if promptW != nil && r == os.Stdin && isTerminal(os.Stdin) {
		typed, err := readPassword(promptW, "password: ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		password = typed
	} else {
		password = promptAndRead(scanner, promptW, "password: ")
	}
	host := promptAndRead(scanner, promptW, "host [127.0.0.1]: ")
	port := promptAndRead(scanner, promptW, "port [2222]: ")
	defaultName, _ := os.Hostname()
	name := promptAndRead(scanner, promptW, fmt.Sprintf("name [%s]: ", defaultName))
	managed := fleet.enabled
	// A managed client is asked where its hub is and what it authenticates
	// with. Both may be left empty: the first start then names them with
	// --server and --token.
	var hubAddress, hubToken string
	if managed {
		hubAddress = promptAndRead(scanner, promptW, "hub address (host:port), empty to give it at the first start: ")
		if promptW != nil && r == os.Stdin && isTerminal(os.Stdin) {
			typed, err := readPassword(promptW, "hub token: ")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			hubToken = typed
		} else {
			hubToken = promptAndRead(scanner, promptW, "hub token: ")
		}
	}

	// Check for I/O errors during reading
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error: read input: %v\n", err)
		return 1
	}

	// Validate required fields
	if username == "" {
		fmt.Fprintf(os.Stderr, "error: username is required\n")
		return 1
	}
	if password == "" {
		fmt.Fprintf(os.Stderr, "error: password is required\n")
		return 1
	}

	if hubAddress != "" {
		if _, _, err := net.SplitHostPort(hubAddress); err != nil {
			fmt.Fprintln(os.Stderr, opererr.New("the hub address "+strconv.Quote(hubAddress)+" is not host:port",
				"the first start connects to that address", "ze init --managed   (then enter the hub as host:port, such as 10.0.0.1:1791)"))
			return 1
		}
	}
	if tokenTooShort(hubToken) {
		fmt.Fprintln(os.Stderr, opererr.New("the hub token is shorter than 32 characters",
			"a hub accepts no shorter secret for a client, so this one could never authenticate",
			"ze init --managed   (then enter the secret of this client's entry under plugin hub server on the hub)"))
		return 1
	}

	// Hash password with bcrypt before storing -- zefs holds the hash,
	// which the CLI sends as an opaque auth token over SSH.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: hash password: %v\n", err)
		return 1
	}

	// Apply defaults
	if host == "" {
		host = defaultHost
	}
	if port == "" {
		port = defaultPort
	}
	if name == "" {
		name = defaultName
	}
	// Every init that writes a config writes the SSH listener on host and
	// port, so both must be what a listener can bind. --seed and --managed
	// write no config, and the CLI target they save may be a host name.
	if !seed && !managed {
		if err := checkListenerEndpoint(host, port, profile); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	populate := func(store storage.Storage) error {
		// Write SSH credentials in deterministic order.
		type entry struct {
			key, value string
		}
		managedValue := "false"
		if managed {
			managedValue = "true"
		}

		entries := []entry{
			{zefs.KeySSHUsername.Key(host, port), username},
			{zefs.KeySSHPassword.Key(host, port), string(hashedPassword)},
			{zefs.KeyLocalAdminUsername.Pattern, username},
			{zefs.KeyLocalAdminPassword.Pattern, string(hashedPassword)},
			{zefs.KeySSHDefault.Pattern, host + "/" + port},
			{keyManaged, managedValue},
		}
		if name != "" {
			entries = append(entries, entry{keyIdentityName, name})
		}
		// What the first start of a managed client needs before it holds a
		// config: where the hub is, what to authenticate with, and how to
		// authenticate the hub. Each is written only when it was given.
		if hubAddress != "" {
			entries = append(entries, entry{zefs.KeyManagedHubAddress.Pattern, hubAddress})
		}
		if hubToken != "" {
			entries = append(entries, entry{zefs.KeyManagedHubToken.Pattern, hubToken})
		}
		if len(fleet.hubCA) != 0 {
			entries = append(entries, entry{zefs.KeyManagedHubCA.Pattern, string(fleet.hubCA)})
		}
		if fleet.trustFirstUse {
			entries = append(entries, entry{zefs.KeyManagedHubTrust.Pattern, zefs.ManagedHubTrustFirstUse})
		}

		for _, e := range entries {
			if err := store.WriteKey(e.key, []byte(e.value)); err != nil {
				return fmt.Errorf("write %s: %w", e.key, err)
			}
		}

		// The initial config, file/active/<name>.conf, is what a bare
		// `ze start` on this store reads.
		switch {
		case seed:
			// --seed writes none: an appliance-image seed DB must NOT bake this
			// build host's interfaces into file/active/<name>.conf. That active
			// config would hold the wrong host's NICs and would shadow any
			// file/template/ze.conf so the appliance never applies it. Instead
			// the appliance boots with no active config and builds one at first
			// boot from the template merged with its own on-device discovery
			// (see cmd/ze/ze_core_start.go bootstrapConfigFromTemplate).
		case managed:
			// --managed writes none: a managed client's config comes from its
			// hub. cmdStartManaged reads file/active/<name>.conf as the hub's
			// cached config, so a file under that name would stand in for it
			// and the first boot would never fetch from the hub.
		default:
			if err := writeInitialConfig(store, host, port, profile, scanner, promptW); err != nil {
				return err
			}
		}

		// Generate and store TLS certificate if requested.
		// --web-cert-name generates a cert with the hostname as DNS SAN (no IP enumeration).
		// --web-cert generates a cert with IP SANs derived from the listen address.
		// Both can be combined.
		if webCertAddr != "" || webCertName != "" {
			var extraNames []string
			if webCertName != "" {
				extraNames = []string{webCertName}
			}
			certPEM, keyPEM, certErr := selfcert.GenerateWebCertWithNames(webCertAddr, extraNames, 0)
			if certErr != nil {
				return fmt.Errorf("generate TLS certificate: %w", certErr)
			}
			if err := store.WriteKey(zefs.KeyWebCert.Pattern, certPEM); err != nil {
				return fmt.Errorf("write TLS cert: %w", err)
			}
			if err := store.WriteKey(zefs.KeyWebKey.Pattern, keyPEM); err != nil {
				return fmt.Errorf("write TLS key: %w", err)
			}
			switch {
			case webCertName != "" && webCertAddr != "":
				fmt.Printf("generated TLS certificate for %s (%s)\n", webCertName, webCertAddr)
			case webCertName != "":
				fmt.Printf("generated TLS certificate for %s\n", webCertName)
			default:
				fmt.Printf("generated TLS certificate for %s\n", webCertAddr)
			}
		}
		return nil
	}

	store, err := publish(populate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: initialize %s: %v\n", path, err)
		return 1
	}
	if err := store.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error: close %s: %v\n", path, err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "initialized %s\n", path) //nolint:errcheck // status output
	return 0
}

// checkListenerEndpoint refuses a host or port the SSH listener in the
// initial config could not bind, so ze init never writes a config that
// `ze start` refuses. The local-service profile also requires a loopback
// address: an installation is reached remotely only after the first login.
// Each refusal names the command that asks again.
func checkListenerEndpoint(host, port string, profile initProfile) error {
	again := "ze init"
	if profile == initLocalService {
		again = "ze install"
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return opererr.New("The SSH address "+strconv.Quote(host)+" is not an IP address",
			"The daemon's SSH listener binds this address, so enter an IP address such as 127.0.0.1 or ::1.", again)
	}
	if profile == initLocalService {
		if !address.IsLoopback() {
			return opererr.New("The bootstrap SSH address is not loopback", "Enter 127.0.0.1 or ::1. Configure remote access after the first login.", again)
		}
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return opererr.New("The SSH port "+strconv.Quote(port)+" is not a number from 1 through 65535",
			"The daemon's SSH listener binds this port, and the CLI connects to it.", again)
	}
	if n == 0 {
		return opererr.New("The SSH port is zero",
			"Port 0 lets the system pick the listener's port, so the CLI could not know it. Enter a number from 1 through 65535.", again)
	}
	return nil
}

// writeInitialConfig writes file/active/<name>.conf, the config a bare
// `ze start` on this store reads. It opens with the SSH listener on the
// endpoint ze init saved as the CLI's default target (meta/ssh/default), so
// `ze cli` reaches the daemon ze init set up. The discover profile adds every
// interface this host has, the confirmed profile only those the operator
// answers yes for on scanner (asked on promptW, nil when stdin is piped), and
// the local-service profile claims none.
func writeInitialConfig(store storage.Storage, host, port string, profile initProfile, scanner *bufio.Scanner, promptW io.Writer) error {
	// Named by the instance name written before, as `ze start` names it.
	configName, err := resolve.DefaultConfig(store)
	if err != nil {
		return fmt.Errorf("name initial config: %w", err)
	}
	var config textbuf.Buffer
	writeSSHListener(&config, host, port)
	if profile == initLocalService {
		return writeInitialConfigKey(store, configName, &config)
	}
	var status textbuf.Buffer
	discovered, discErr := discoverHostInterfaces()
	switch {
	case discErr != nil:
		// The listener is still written: it is how the operator reaches the
		// daemon to configure the interfaces discovery missed. --confirm has
		// nothing to ask, so it asks nothing.
		var warning textbuf.Buffer
		warning.Str("warning: interface discovery failed, so the initial config holds no interface: ").Err(discErr).
			Str(" (after ze start, 'ze interface scan --config' prints them)\n").StdErr() //nolint:errcheck // command diagnostic
		status.Str("wrote initial config")
	case profile == initDiscoverConfirmed:
		// An answer that is not accepted is noted on stderr, beside the
		// prompts and the refusal, on a pipe too: stdout carries the status
		// line a script reads.
		confirmed, err := confirmInterfaces(scanner, promptW, os.Stderr, discovered)
		if err != nil {
			return err
		}
		// Nothing confirmed writes no interface block at all, rather than
		// an empty one.
		if len(confirmed) != 0 {
			config.Str(iface.EmitConfigKept(discovered, func(di *iface.DiscoveredInterface) bool { return confirmed[di.Name] }))
		}
		status.Str("discovered ").Int(int64(len(discovered))).Str(" interface(s), wrote the ").Int(int64(len(confirmed))).
			Str(" confirmed into the initial config")
	default:
		config.Str(iface.EmitConfig(discovered))
		status.Str("discovered ").Int(int64(len(discovered))).Str(" interface(s), wrote initial config")
	}
	if err := writeInitialConfigKey(store, configName, &config); err != nil {
		return err
	}
	status.Str(" with the SSH listener on ").Str(net.JoinHostPort(host, port)).Byte('\n').StdOut() //nolint:errcheck // status output
	return nil
}

// writeInitialConfigKey stores config as file/active/<configName>.
func writeInitialConfigKey(store storage.Storage, configName string, config *textbuf.Buffer) error {
	if err := store.WriteKey(zefs.KeyFileActive.Key(configName), config.Bytes()); err != nil {
		return fmt.Errorf("write initial config: %w", err)
	}
	return nil
}

// writeSSHListener writes the environment block that starts the SSH server
// on host and port. It is the one producer of ze init's listener, for every
// profile that writes a config, so the endpoint the daemon listens on and the
// one the CLI connects to are the same two values.
func writeSSHListener(config *textbuf.Buffer, host, port string) {
	config.Str("environment {\n    ssh {\n        enabled true;\n        server local {\n            ip ").Str(host).
		Str(";\n            port ").Str(port).Str(";\n        }\n    }\n}\n")
}

// discoverHostInterfaces is the discovery writeInitialConfig reads. It is a
// variable so a test supplies a fixed discovered set, because the host's own
// devices differ from one machine to the next. A test replaces it before it
// runs init and restores it after; NOT safe to replace while an init runs.
var discoverHostInterfaces = discoverInterfaces

// discoverInterfaces answers this host's interfaces. LoadBackend activates the
// kernel backend registered via a blank import; without it
// DiscoverInterfaces answers "no backend loaded". A backend that does not
// load (a platform with only the stub) and a discovery that fails are both
// returned as the error.
func discoverInterfaces() ([]iface.DiscoveredInterface, error) {
	if err := iface.LoadBackend("kernel"); err != nil {
		return nil, fmt.Errorf("load kernel backend: %w", err)
	}
	discovered, discErr := iface.DiscoverInterfaces()
	if closeErr := iface.CloseBackend(); closeErr != nil {
		var warning textbuf.Buffer
		warning.Str("warning: close kernel backend: ").Err(closeErr).Byte('\n').StdErr() //nolint:errcheck // command diagnostic
	}
	if discErr != nil {
		return nil, discErr
	}
	return discovered, nil
}

// confirmInterfaces asks, for each discovered device the initial config can
// hold (iface.Emits), whether to write it, and answers the names of the
// devices confirmed. Every device is asked about, the loopback included: a
// device written into the config is one a root ze manages, and the SSH
// listener binds its address without a loopback entry. Devices are asked in
// discovery order, each until it gets an explicit answer (readAnswer). The
// prompts go to promptW, nil when stdin is piped, and the notice for an answer
// not accepted goes to noticeW whether or not stdin is piped, so a script
// feeds the same lines a terminal user types and sees the same notices.
func confirmInterfaces(scanner *bufio.Scanner, promptW, noticeW io.Writer, discovered []iface.DiscoveredInterface) (map[string]bool, error) {
	if promptW != nil {
		fmt.Fprint(promptW, "ze manages each interface written into the config once it starts. Answer yes or y to write it, no or n to leave it out.\n") //nolint:errcheck // terminal prompt
	}
	confirmed := make(map[string]bool, len(discovered))
	for i := range discovered {
		di := &discovered[i]
		if !iface.Emits(di) {
			continue
		}
		var device textbuf.Buffer
		describeDevice(&device, di)
		yes, err := readAnswer(scanner, promptW, noticeW, device.String())
		if err != nil {
			return nil, err
		}
		if yes {
			confirmed[di.Name] = true
		}
	}
	return confirmed, nil
}

// describeDevice writes what the operator needs to decide about di: its kind
// and name, then what discovery knows that hints at an owner, such as the
// bridge or bond it is a member of and its kernel alias.
func describeDevice(b *textbuf.Buffer, di *iface.DiscoveredInterface) {
	b.Str(di.Type).Byte(' ').Str(di.Name)
	separator := " ("
	if di.MAC != "" {
		b.Str(separator).Str("mac ").Str(di.MAC)
		separator = ", "
	}
	if di.Master != "" {
		b.Str(separator).Str("member of ").Str(di.Master)
		separator = ", "
	}
	if di.Alias != "" {
		// Quoted: the alias is free text another program wrote.
		b.Str(separator).Str("alias ").Str(strconv.Quote(di.Alias))
		separator = ", "
	}
	if separator == ", " {
		b.Byte(')')
	}
}

// readAnswer asks about device on promptW (nil when stdin is piped) and reads
// the operator's answer: true for yes or y, false for no or n, compared
// without case. Only an explicit answer decides: any other line, an empty one
// included, is not accepted, so readAnswer writes one notice line to noticeW
// naming the accepted answers and asks about the same device again, reading
// the next line. Input that ends before an accepted answer is refused: a
// device is never decided for the operator.
//
// The end of input is told from an empty line at the scanner: Scan answers
// false only when no line is left (end of file or a read error), while an
// empty line is a successful Scan with empty text. readLine, which answers ""
// for both, is not used here. The loop is bounded by the input: each pass
// consumes one line, and the end of input returns.
func readAnswer(scanner *bufio.Scanner, promptW, noticeW io.Writer, device string) (bool, error) {
	for {
		if promptW != nil {
			fmt.Fprint(promptW, device, ": write it into the config? yes/no: ") //nolint:errcheck // terminal prompt
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return false, fmt.Errorf("read the answer for %s: %w", device, err)
			}
			return false, opererr.New("ze init "+flagConfirm+" got no answer for "+device,
				"the input ended before every discovered interface was answered, and ze init decides none for you: give one line per interface, yes or no",
				"ze init "+flagConfirm)
		}
		answer := strings.TrimSpace(scanner.Text())
		switch strings.ToLower(answer) {
		case "yes", "y":
			return true, nil
		case "no", "n":
			return false, nil
		}
		writeNotAccepted(noticeW, answer, device)
	}
}

// writeNotAccepted writes the one line that tells the operator answer does
// not decide device, and what to type instead.
func writeNotAccepted(noticeW io.Writer, answer, device string) {
	var notice textbuf.Buffer
	if answer == "" {
		notice.Str("an empty line")
	} else {
		notice.Str(strconv.Quote(answer))
	}
	notice.Str(" is not an answer for ").Str(device).Str(": type yes or y to write it, or no or n to leave it out\n")
	noticeW.Write(notice.Bytes()) //nolint:errcheck // command diagnostic
}

// readLine reads a single line from the scanner, trimming whitespace.
func readLine(scanner *bufio.Scanner) string {
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}

// isTerminal returns true if f is a terminal (not a pipe or file).
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	// refuse-by-default: guards runInitPublishedProfile -- a file whose mode cannot be read is not known to be a terminal, so its input is read as plain lines and no no-echo read is tried on it
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// readPassword prompts for a secret without echoing input to the terminal.
// Prints "***" after reading to confirm input was received. A read that fails
// is returned as an error naming what was asked for: an empty secret is a
// valid answer to some prompts (the hub token), so it cannot stand for a
// failed read.
func readPassword(w io.Writer, prompt string) (string, error) {
	fmt.Fprint(w, prompt) //nolint:errcheck // terminal prompt
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(w, "***") //nolint:errcheck // visual confirmation
	if err != nil {
		what := strings.TrimSuffix(prompt, ": ")
		return "", opererr.Wrap(err, "The "+what+" could not be read from the terminal",
			"ze init reads it without echo, and the terminal refused that read, so nothing was stored.",
			"ze init")
	}
	return strings.TrimSpace(string(pw)), nil
}

// promptAndRead optionally writes a prompt to w, then reads a line.
func promptAndRead(scanner *bufio.Scanner, w io.Writer, prompt string) string {
	if w != nil {
		fmt.Fprint(w, prompt) //nolint:errcheck // terminal prompt
	}
	return readLine(scanner)
}

// confirmForceReplace prompts the user for confirmation before replacing an existing database.
// Returns true only if the user types "yes" (case-insensitive).
// When stdin is piped, opens /dev/tty for the confirmation prompt.
func confirmForceReplace(dbPath string) bool {
	var ttyReader io.Reader
	if isTerminal(os.Stdin) {
		ttyReader = os.Stdin
	} else {
		tty, err := os.Open("/dev/tty")
		// refuse-by-default: guards confirmForce -- false is "not confirmed", so with no terminal to ask on confirmForce aborts and the database is kept
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: --force requires a terminal for confirmation\n")
			return false
		}
		defer tty.Close() //nolint:errcheck // read-only
		ttyReader = tty
	}

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  +-------------------------------------------------------+\n")
	fmt.Fprintf(os.Stderr, "  |  WARNING: replacing the existing database              |\n")
	fmt.Fprintf(os.Stderr, "  +-------------------------------------------------------+\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Database : %s\n", dbPath)
	fmt.Fprintf(os.Stderr, "  Backup to: %s.replaced-<date>\n", filepath.Base(dbPath))
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  SSH credentials, instance metadata, and config state\n")
	fmt.Fprintf(os.Stderr, "  in the current database will be replaced.\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Type 'yes' to proceed: ")

	scanner := bufio.NewScanner(ttyReader)
	if !scanner.Scan() {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(scanner.Text()), "yes")
}
