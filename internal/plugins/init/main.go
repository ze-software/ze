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

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/selfcert"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
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

	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze init",
			ShortHelp: "Bootstrap the ze database with SSH credentials",
			Usage:     []string{"ze init [options]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Input (stdin or interactive prompts)", Entries: []helpfmt.HelpEntry{
					{Name: "Line 1: username", Desc: ""},
					{Name: "Line 2: password", Desc: ""},
					{Name: "Line 3: host", Desc: "(default: 127.0.0.1)"},
					{Name: "Line 4: port", Desc: "(default: 2222)"},
					{Name: "Line 5: name", Desc: "(default: hostname)"},
				}},
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--managed", Desc: "Enable managed (fleet) mode"},
					{Name: "--force", Desc: "Replace existing database (moves old to .replaced-<date>)"},
					{Name: "--yes", Desc: "Skip confirmation prompt (use with --force)"},
					{Name: "--web-cert <addr>", Desc: "Generate TLS certificate for web server (e.g. 0.0.0.0:8080)"},
					{Name: "--web-cert-name <host>", Desc: "Extra DNS name for TLS certificate SAN (e.g. router.example.com)"},
					{Name: "--seed", Desc: "Create a blob artifact for appliance builders; skip build-host interface discovery"},
				}},
			},
			Examples: []string{
				`echo -e "admin\nsecret\n127.0.0.1\n2222\nmy-router" | ze init`,
				"ze init --managed  (interactive prompts, managed mode)",
				"ze init --force         (replace existing database)",
				"ze init --force --yes   (replace without confirmation)",
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	dbPath := sshclient.ResolveStoreDir("")
	if dbPath == "" {
		fmt.Fprintf(os.Stderr, "error: cannot determine database location\n")
		return 1
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

	// Confirmation precedes staging; replacement itself holds the storage lock.
	if *forceFlag {
		for _, name := range []string{"database", "database.zefs"} {
			path := filepath.Join(dbPath, name)
			if _, err := os.Lstat(path); err == nil {
				if !*yesFlag {
					if !confirmForceReplace(path) {
						fmt.Fprintf(os.Stderr, "aborted\n")
						return 1
					}
				}
				break
			}
		}
	}

	return runInit(inputReader, promptWriter, dbPath, *managedFlag, *webCertFlag, *webCertNameFlag, *seedFlag, *forceFlag)
}

// RunWithReader creates a live store in dir with SSH credentials read from r.
// Format: one line each for username, password, host, port, name.
// Empty host defaults to 127.0.0.1, empty port defaults to 2222.
func RunWithReader(r io.Reader, dir string, managed bool) int {
	return runInit(r, nil, dir, managed, "", "", false, false)
}

// RunWithReaderForce stages a replacement under exclusive storage ownership.
// Callers MUST obtain confirmation before calling.
func RunWithReaderForce(r io.Reader, dir string, managed bool) (int, error) {
	return runInit(r, nil, dir, managed, "", "", false, true), nil
}

// RunInteractive creates a live store with interactive prompts.
// Prompts are written to w (typically os.Stderr).
func RunInteractive(r io.Reader, w io.Writer, dir string) int {
	return runInit(r, w, dir, false, "", "", false, false)
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
		// this build host's interfaces into file/active/ze.conf. That active
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
					configKey := zefs.KeyFileActive.Key("ze.conf")
					if wErr := store.WriteKey(configKey, []byte(config)); wErr != nil {
						if closeErr := iface.CloseBackend(); closeErr != nil {
							return fmt.Errorf("write initial config: %w; close backend: %v", wErr, closeErr)
						}
						return fmt.Errorf("write initial config: %w", wErr)
					}
					fmt.Fprintf(os.Stdout, "discovered %d interface(s), wrote initial config\n", len(discovered))
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
	if seed {
		path = filepath.Join(dir, "database.zefs")
		store, err = storage.CreateBlobPopulated(path, populate, force)
	} else if force {
		store, err = storage.ReplacePopulated(dir, populate)
	} else {
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
	fmt.Fprintf(os.Stdout, "initialized %s\n", path)
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
