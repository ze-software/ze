// Design: docs/architecture/system-architecture.md — ze init bootstrap command

// Package init provides the `ze init` command that bootstraps the live store
// or builds an explicit appliance seed artifact.
package init

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/fetch"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/pkg/zefs"

	// Register the netlink backend so iface.LoadBackend("netlink")
	// below resolves. Without this blank import, DiscoverInterfaces
	// returns "no backend loaded" and every detected interface
	// (ethernet, dummy, veth, bridge, tunnel, wireguard) is silently
	// dropped from the initial ze.conf.
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
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
	defaultHost = "127.0.0.1"
	defaultPort = "2222"
)

// Run executes ze init from CLI arguments.
// Returns exit code.
func Run(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	managedFlag := fs.Bool("managed", false, "Enable managed (fleet) mode")
	forceFlag := fs.Bool("force", false, "Replace existing database (moves old to .replaced-<date>)")
	yesFlag := fs.Bool("yes", false, "Skip confirmation prompt (use with --force)")
	webCertFlag := fs.String("web-cert", "", "Generate TLS certificate for web server (listen address, e.g. 0.0.0.0:8080)")
	webCertNameFlag := fs.String("web-cert-name", "", "Extra DNS name for the TLS certificate SAN (e.g. router.example.com)")
	seedFlag := fs.Bool("seed", false, "Create a database.zefs appliance seed artifact without build-host interface discovery")
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
					{Name: "Line 3: host", Desc: "(default: 127.0.0.1)"},
					{Name: "Line 4: port", Desc: "(default: 2222)"},
					{Name: "Line 5: name", Desc: "(default: hostname)"},
				}},
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: flagManaged, Desc: "Enable managed (fleet) mode"},
					{Name: "--force", Desc: "Replace existing database (moves old to .replaced-<date>)"},
					{Name: "--yes", Desc: "Skip confirmation prompt (use with --force)"},
					{Name: "--web-cert <addr>", Desc: "Generate TLS certificate for web server (e.g. 0.0.0.0:8080)"},
					{Name: "--web-cert-name <host>", Desc: "Extra DNS name for TLS certificate SAN (e.g. router.example.com)"},
					{Name: "--seed", Desc: "Create a blob artifact for appliance builders; skip build-host interface discovery"},
					{Name: "--from <source>", Desc: "Import a blob artifact (database.zefs) from a local path or a URL (" + schemes + ") into the live store; a local blob is retired as .replaced-<date>, a fetched copy is removed; reads no credentials"},
					{Name: "--sha256 <hex>", Desc: "Check the --from source against this SHA-256 before a key is written"},
				}},
			},
			Examples: []string{
				`echo -e "admin\nsecret\n127.0.0.1\n2222\nmy-router" | ze init`,
				"ze init --managed  (interactive prompts, managed mode)",
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

	return runInit(inputReader, promptWriter, dbPath, *managedFlag, *webCertFlag, *webCertNameFlag, *seedFlag, *forceFlag)
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
	// Read credentials (with optional prompts)
	scanner := bufio.NewScanner(r)

	username := promptAndRead(scanner, promptW, "username: ")

	var password string
	if promptW != nil && r == os.Stdin && isTerminal(os.Stdin) {
		password = readPassword(promptW, "password: ")
	} else {
		password = promptAndRead(scanner, promptW, "password: ")
	}
	host := promptAndRead(scanner, promptW, "host [127.0.0.1]: ")
	port := promptAndRead(scanner, promptW, "port [2222]: ")
	defaultName, _ := os.Hostname()
	name := promptAndRead(scanner, promptW, fmt.Sprintf("name [%s]: ", defaultName))

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

		for _, e := range entries {
			if err := store.WriteKey(e.key, []byte(e.value)); err != nil {
				return fmt.Errorf("write %s: %w", e.key, err)
			}
		}

		// Discover OS interfaces and generate initial config. LoadBackend
		// activates the netlink backend registered via the blank import
		// above; without it DiscoverInterfaces returns "no backend loaded"
		// and every detected netdev is silently dropped. Backend load
		// failures (e.g., non-Linux platforms with only the stub backend)
		// are non-fatal -- init still completes, the user just gets an
		// empty interface config.
		//
		// --seed skips this entirely: an appliance-image seed DB must NOT bake
		// this build host's interfaces into file/active/<name>.conf. That active
		// config would hold the wrong host's NICs and would shadow any
		// file/template/ze.conf so the appliance never applies it. Instead the
		// appliance boots with no active config and builds one at first boot from
		// the template merged with its own on-device discovery (see
		// cmd/ze/ze_core_start.go bootstrapConfigFromTemplate).
		if seed { //nolint:staticcheck // SA9003: intentional no-op; see comment above
			// appliance seed: nothing baked in; first boot discovers on-device.
		} else if loadErr := iface.LoadBackend("netlink"); loadErr != nil {
			fmt.Fprintf(os.Stderr, "warning: load netlink backend: %v\n", loadErr)
		} else {
			if discovered, discErr := iface.DiscoverInterfaces(); discErr != nil {
				fmt.Fprintf(os.Stderr, "warning: interface discovery: %v\n", discErr)
			} else if len(discovered) > 0 {
				if config := iface.EmitConfig(discovered); config != "" {
					// Named by the instance name written above, the file a bare
					// `ze start` on this store reads.
					configKey := zefs.KeyFileActive.Key(resolve.DefaultConfig(store))
					if wErr := store.WriteKey(configKey, []byte(config)); wErr != nil {
						if closeErr := iface.CloseBackend(); closeErr != nil {
							return fmt.Errorf("write initial config: %w; close backend: %w", wErr, closeErr)
						}
						return fmt.Errorf("write initial config: %w", wErr)
					}
					fmt.Fprintf(os.Stdout, "discovered %d interface(s), wrote initial config\n", len(discovered)) //nolint:errcheck // status output
				}
			}
			if closeErr := iface.CloseBackend(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "warning: close netlink backend: %v\n", closeErr)
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

	var store storage.Storage
	path := filepath.Join(dir, "database")
	switch {
	case seed:
		path = filepath.Join(dir, "database.zefs")
		store, err = storage.CreateBlobPopulated(path, populate, force)
	case force:
		store, err = storage.ReplacePopulated(dir, populate)
	default:
		store, err = storage.CreatePopulated(dir, populate)
	}
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
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// readPassword prompts for a password without echoing input to the terminal.
// Prints "***" after reading to confirm input was received.
func readPassword(w io.Writer, prompt string) string {
	fmt.Fprint(w, prompt) //nolint:errcheck // terminal prompt
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(w, "***") //nolint:errcheck // visual confirmation
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(pw))
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
