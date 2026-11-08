// Design: docs/features/dns-resolver.md -- the host's resolv.conf as a source of resolvers
// Related: record.go -- the record a resolv.conf's servers become

package dnssource

import (
	"bufio"
	"errors"
	"io"
	"io/fs"
	"net/netip"
	"os"
	"strings"
)

// resolvConfOctetsMax bounds a resolv.conf read. A real one is a few hundred
// bytes; the bound only stops a runaway file.
const resolvConfOctetsMax = 64 << 10

// ParseResolvConf answers the address of every "nameserver" line of a
// resolv.conf, in file order, and the nameserver values that are not IP
// addresses, which cannot be resolvers. It reads at most 64 KiB.
func ParseResolvConf(content io.Reader) (servers []netip.Addr, rejected []string, err error) {
	scanner := bufio.NewScanner(io.LimitReader(content, resolvConfOctetsMax))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		if fields[0] != "nameserver" {
			continue
		}
		addr, parseErr := netip.ParseAddr(fields[1])
		if parseErr != nil {
			rejected = append(rejected, fields[1])
			continue
		}
		servers = append(servers, addr)
	}
	return servers, rejected, scanner.Err()
}

// ErrNoResolvConf reports that the resolv.conf named does not exist, which a
// caller tells apart from one it could not read.
var ErrNoResolvConf = errors.New("resolv.conf does not exist")

// ReadResolvConf parses the resolv.conf at path. A file that does not exist
// answers ErrNoResolvConf; any other failure to read answers its error.
func ReadResolvConf(path string) (servers []netip.Addr, rejected []string, err error) {
	file, err := os.Open(path) //nolint:gosec // the path is the host's resolv.conf, named by Ze
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, ErrNoResolvConf
	}
	if err != nil {
		return nil, nil, err
	}
	defer file.Close() //nolint:errcheck // read-only file, nothing to flush
	return ParseResolvConf(file)
}
