// Design: docs/architecture/system-architecture.md -- ze connect: SSH credential management

package connect

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/ze-software/ze/internal/core/helpfmt"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	defaultPort = "2222"
	flagPort    = "--port"
	flagUser    = "--user"
)

func Run(args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}

	switch args[0] {
	case "add":
		return runAdd(args[1:])
	case "list":
		return runList()
	case "remove":
		return runRemove(args[1:])
	case "default":
		return runDefault(args[1:])
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "error: unknown connect subcommand %q\n", args[0])
		usage()
		return 1
	}
}

func runAdd(args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "error: usage: ze connect add <host> [--port N] [--user name]\n")
		return 1
	}

	host := args[0]
	port := defaultPort
	user := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case flagPort:
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --port requires a value\n")
				return 1
			}
			i++
			port = args[i]
		case flagUser:
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --user requires a value\n")
				return 1
			}
			i++
			user = args[i]
		default:
			fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", args[i])
			return 1
		}
	}

	if user == "" {
		fmt.Fprintf(os.Stderr, "username: ")
		name, err := readLine(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		user = name
		if user == "" {
			fmt.Fprintf(os.Stderr, "error: username is required\n")
			return 1
		}
	}

	password := readPassword()
	if password == "" {
		fmt.Fprintf(os.Stderr, "error: password is required\n")
		return 1
	}

	dbPath := sshclient.ResolveDBPath()
	if dbPath == "" {
		fmt.Fprintf(os.Stderr, "error: cannot determine database location\n")
		return 1
	}

	return AddCredentials(dbPath, host, port, user, password)
}

func runList() int {
	dbPath := sshclient.ResolveDBPath()
	if dbPath == "" {
		fmt.Fprintf(os.Stderr, "error: cannot determine database location\n")
		return 1
	}
	return ListRemotes(dbPath)
}

func runRemove(args []string) int {
	host, port := parseHostPort(args)
	if host == "" {
		fmt.Fprintf(os.Stderr, "error: usage: ze connect remove <host> [--port N]\n")
		return 1
	}

	dbPath := sshclient.ResolveDBPath()
	if dbPath == "" {
		fmt.Fprintf(os.Stderr, "error: cannot determine database location\n")
		return 1
	}
	return RemoveCredentials(dbPath, host, port)
}

func runDefault(args []string) int {
	host, port := parseHostPort(args)
	if host == "" {
		fmt.Fprintf(os.Stderr, "error: usage: ze connect default <host> [--port N]\n")
		return 1
	}

	dbPath := sshclient.ResolveDBPath()
	if dbPath == "" {
		fmt.Fprintf(os.Stderr, "error: cannot determine database location\n")
		return 1
	}
	return SetDefault(dbPath, host, port)
}

func parseHostPort(args []string) (string, string) {
	if len(args) < 1 {
		return "", ""
	}
	host := args[0]
	port := defaultPort
	for i := 1; i < len(args); i++ {
		if args[i] == flagPort && i+1 < len(args) {
			i++
			port = args[i]
		}
	}
	return host, port
}

func validateHostPort(host, port string) error {
	if host == "" {
		return fmt.Errorf("host is required")
	}
	if strings.Contains(host, "/") {
		return fmt.Errorf("host %q must not contain '/'", host)
	}
	if port == "" {
		return fmt.Errorf("port is required")
	}
	if strings.Contains(port, "/") {
		return fmt.Errorf("port %q must not contain '/'", port)
	}
	return nil
}

// AddCredentials stores credentials for a remote daemon.
func AddCredentials(dbPath, host, port, user, password string) int {
	if err := validateHostPort(host, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: hash password: %v\n", err)
		return 1
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open database: %v\n", err)
		return 1
	}
	defer store.Close() //nolint:errcheck // best-effort

	usernameKey := zefs.KeySSHUsername.Key(host, port)
	passwordKey := zefs.KeySSHPassword.Key(host, port)

	if err := store.WriteFile(usernameKey, []byte(user), 0); err != nil {
		fmt.Fprintf(os.Stderr, "error: write username: %v\n", err)
		return 1
	}
	if err := store.WriteFile(passwordKey, hashedPassword, 0); err != nil {
		fmt.Fprintf(os.Stderr, "error: write password: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stdout, "added remote %s:%s (user: %s)\n", host, port, user) //nolint:errcheck // status output
	return 0
}

// ListRemotes prints all stored remote credentials.
func ListRemotes(dbPath string) int {
	store, err := zefs.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open database: %v\n", err)
		return 1
	}
	defer store.Close() //nolint:errcheck // best-effort

	dflt := ""
	if data, err := store.ReadFile(zefs.KeySSHDefault.Pattern); err == nil {
		dflt = string(data)
	}

	entries := store.List("meta/ssh")
	type remoteEntry struct {
		host, port, user string
	}
	seen := make(map[string]*remoteEntry)

	for _, key := range entries {
		if key == "meta/ssh/default" || key == "meta/ssh/authorized-keys" {
			continue
		}
		rest := strings.TrimPrefix(key, "meta/ssh/")
		parts := strings.SplitN(rest, "/", 3)
		if len(parts) != 3 {
			continue
		}
		host, port, leaf := parts[0], parts[1], parts[2]
		id := host + "/" + port
		r, ok := seen[id]
		if !ok {
			r = &remoteEntry{host: host, port: port}
			seen[id] = r
		}
		if leaf == "username" {
			if data, err := store.ReadFile(key); err == nil {
				r.user = string(data)
			}
		}
	}

	if len(seen) == 0 {
		fmt.Fprintf(os.Stdout, "no remotes configured\n") //nolint:errcheck // status output
		return 0
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		r := seen[id]
		marker := "  "
		if id == dflt {
			marker = "* "
		}
		fmt.Fprintf(os.Stdout, "%s%s:%s (user: %s)\n", marker, r.host, r.port, r.user) //nolint:errcheck // status output
	}
	return 0
}

// RemoveCredentials deletes stored credentials for a remote.
func RemoveCredentials(dbPath, host, port string) int {
	if err := validateHostPort(host, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open database: %v\n", err)
		return 1
	}
	defer store.Close() //nolint:errcheck // best-effort

	usernameKey := zefs.KeySSHUsername.Key(host, port)
	passwordKey := zefs.KeySSHPassword.Key(host, port)

	if !store.Has(usernameKey) {
		fmt.Fprintf(os.Stderr, "error: no credentials for %s:%s\n", host, port)
		return 1
	}

	if err := store.Remove(usernameKey); err != nil {
		fmt.Fprintf(os.Stderr, "error: remove username: %v\n", err)
		return 1
	}
	if err := store.Remove(passwordKey); err != nil {
		fmt.Fprintf(os.Stderr, "error: remove password: %v\n", err)
		return 1
	}

	if data, err := store.ReadFile(zefs.KeySSHDefault.Pattern); err == nil && string(data) == host+"/"+port {
		store.Remove(zefs.KeySSHDefault.Pattern) //nolint:errcheck // best-effort cleanup
	}

	fmt.Fprintf(os.Stdout, "removed remote %s:%s\n", host, port) //nolint:errcheck // status output
	return 0
}

// SetDefault sets the default remote target.
func SetDefault(dbPath, host, port string) int {
	if err := validateHostPort(host, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	store, err := zefs.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open database: %v\n", err)
		return 1
	}
	defer store.Close() //nolint:errcheck // best-effort

	usernameKey := zefs.KeySSHUsername.Key(host, port)
	if !store.Has(usernameKey) {
		fmt.Fprintf(os.Stderr, "error: no credentials for %s:%s\n", host, port)
		return 1
	}

	if err := store.WriteFile(zefs.KeySSHDefault.Pattern, []byte(host+"/"+port), 0); err != nil {
		fmt.Fprintf(os.Stderr, "error: write default: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stdout, "default remote set to %s:%s\n", host, port) //nolint:errcheck // status output
	return 0
}

// readLine reads one line from r.
//
// Scan returns false on EOF, on a read error, and on a line above
// bufio.MaxScanTokenSize alike, and a read that fails part way through the line
// still returns the buffered prefix as a successful token. So the error is read
// back even when Scan succeeded: a half-read password stored under the
// operator's host is a credential that fails to authenticate later, with
// nothing in the output pointing at the read.
func readLine(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	ok := scanner.Scan()
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(scanner.Text()), nil
}

// AddCredentialsFromReader reads password from r for testing.
func AddCredentialsFromReader(r io.Reader, dbPath, host, port, user string) int {
	password, err := readLine(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if password == "" {
		fmt.Fprintf(os.Stderr, "error: password is required\n")
		return 1
	}
	return AddCredentials(dbPath, host, port, user, password)
}

func readPassword() string {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "password: ")
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(pw))
	}
	pw, err := readLine(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return ""
	}
	return pw
}

func usage() {
	p := helpfmt.Page{
		Command: "ze connect",
		Summary: "Manage SSH credentials for remote ze daemons",
		Usage: []string{
			"ze connect add <host> [--port N] [--user name]",
			"ze connect list",
			"ze connect remove <host> [--port N]",
			"ze connect default <host> [--port N]",
		},
		Examples: []string{
			"ze connect add 10.0.1.5 --port 2223 --user admin",
			"ze connect list",
			"ze connect default 10.0.1.5 --port 2223",
			"ze connect remove 10.0.1.5 --port 2223",
		},
	}
	p.WriteErr()
}
