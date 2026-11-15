// Design: docs/architecture/irr-owner.md -- the stored IRR dataset value
// Related: store.go -- PrefixStore, whose resolved lists the owner stores here
// Related: notice.go -- the notice that names a stored dataset
//
// A dataset is one AS-SET's prefixes as one IRR server answered them, stored
// under meta/irr-data/{server}/{as-set}/{digest} by the irr plugin and read by
// bgp-filter-irr and firewall-irr. The key ends in the value's own digest, so
// every distinct value has a key of its own and the pointer names exactly one
// (docs/architecture/irr-owner.md, "Writing a dataset"). It is compact binary,
// not JSON: 500,000
// prefixes of each family must fit one state value (zefs.IRRDatasetValueMax),
// and a JSON list of a million prefixes is about 20 MB.

package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// The dataset value, version 1. Integers are big-endian. Times are Unix
// seconds, UTC, and 0 means unset.
//
//	offset  size  field
//	0       1     version (1)
//	1       8     refreshed-at  int64
//	9       8     checked-at    int64
//	17      8     stale-since   int64
//	25      2     as-set length L1, then L1 bytes
//	.       2     server length L2, then L2 bytes (the normalised server)
//	.       4     ipv4 count N4, then N4 prefixes
//	.       4     ipv6 count N6, then N6 prefixes
//	.       32    SHA-256 over every byte before it
//
//	prefix: 1 byte of prefix length in bits, then the ceil(bits/8)
//	significant bytes of the masked address, as BGP NLRI writes a prefix.
const (
	datasetVersion  = 1
	datasetTimesLen = 3 * 8
	datasetDigest   = sha256.Size
	// datasetFixedLen is every byte a dataset holds whatever its content.
	datasetFixedLen = 1 + datasetTimesLen + 2 + 2 + 4 + 4 + datasetDigest
	// DatasetFamilyMax bounds the prefixes of one family in one dataset; it is
	// the largest per-family list any consumer accepts (firewall-irr), and with
	// it both families together fit zefs.IRRDatasetValueMax.
	DatasetFamilyMax = 500_000
	// datasetNameMax bounds a stored AS-SET or server name; a state key is at
	// most rpc.StateKeyMax bytes, so neither segment can be longer.
	datasetNameMax = 512
	// whoisPort is the IRR whois port, which a dataset key leaves out.
	whoisPort = 43
)

// The named errors a dataset read can return. A reader treats each one as "no
// stored copy": it never reads a refused value as an empty list.
var (
	ErrDatasetTruncated = errors.New("irr dataset: value is truncated")
	ErrDatasetDigest    = errors.New("irr dataset: digest does not match the content")
	ErrDatasetVersion   = errors.New("irr dataset: unknown format version")
	ErrDatasetMalformed = errors.New("irr dataset: value is malformed")
	ErrDatasetIdentity  = errors.New("irr dataset: value names another AS-SET, server or digest than its key")
	ErrDatasetTooLarge  = errors.New("irr dataset: over the stored size bound")
	ErrDatasetServer    = errors.New("irr dataset: server has no key form")
)

// DatasetInput is what the owner hands NewDataset: the resolved lists and the
// three schedule times of one AS-SET from one server.
type DatasetInput struct {
	ASSet       string
	Server      string
	IPv4        []netip.Prefix
	IPv6        []netip.Prefix
	RefreshedAt time.Time
	CheckedAt   time.Time
	StaleSince  time.Time
}

// Dataset is a validated dataset: only NewDataset and DecodeDataset return one,
// so every Dataset encodes and its prefix lists are within the bounds. Its
// prefixes are stored masked. The slices IPv4 and IPv6 return are the
// dataset's own and MUST NOT be modified. Safe for concurrent reads.
type Dataset struct {
	asSet       string
	server      string
	ipv4        []netip.Prefix
	ipv6        []netip.Prefix
	refreshedAt time.Time
	checkedAt   time.Time
	staleSince  time.Time
	encodedLen  int
}

// NewDataset validates in and returns the dataset it describes. The server is
// normalised (DatasetServer). A list over DatasetFamilyMax, a prefix of the
// wrong family, or a value over zefs.IRRDatasetValueMax is refused whole, with
// the counts in the error: the previous stored copy stays the enforced one.
func NewDataset(in DatasetInput) (Dataset, error) {
	if err := validateName(in.ASSet); err != nil {
		return Dataset{}, err
	}
	server, err := DatasetServer(in.Server)
	if err != nil {
		return Dataset{}, err
	}
	if len(in.ASSet) > datasetNameMax {
		return Dataset{}, fmt.Errorf("%w: AS-SET name is %d bytes", ErrDatasetTooLarge, len(in.ASSet))
	}
	if len(in.IPv4) > DatasetFamilyMax {
		return Dataset{}, fmt.Errorf("%w: %s holds %d IPv4 and %d IPv6 prefixes, over %d per family",
			ErrDatasetTooLarge, in.ASSet, len(in.IPv4), len(in.IPv6), DatasetFamilyMax)
	}
	if len(in.IPv6) > DatasetFamilyMax {
		return Dataset{}, fmt.Errorf("%w: %s holds %d IPv4 and %d IPv6 prefixes, over %d per family",
			ErrDatasetTooLarge, in.ASSet, len(in.IPv4), len(in.IPv6), DatasetFamilyMax)
	}
	size := datasetFixedLen + len(in.ASSet) + len(server)
	for _, prefix := range in.IPv4 {
		if !prefix.IsValid() {
			return Dataset{}, fmt.Errorf("%w: %s IPv4 list holds an invalid prefix", ErrDatasetMalformed, in.ASSet)
		}
		if !prefix.Addr().Is4() {
			return Dataset{}, fmt.Errorf("%w: %s IPv4 list holds %s", ErrDatasetMalformed, in.ASSet, prefix)
		}
		size += 1 + significantBytes(prefix.Bits())
	}
	for _, prefix := range in.IPv6 {
		if !prefix.IsValid() {
			return Dataset{}, fmt.Errorf("%w: %s IPv6 list holds an invalid prefix", ErrDatasetMalformed, in.ASSet)
		}
		if !prefix.Addr().Is6() {
			return Dataset{}, fmt.Errorf("%w: %s IPv6 list holds %s", ErrDatasetMalformed, in.ASSet, prefix)
		}
		size += 1 + significantBytes(prefix.Bits())
	}
	if size > zefs.IRRDatasetValueMax {
		return Dataset{}, fmt.Errorf("%w: %s holds %d IPv4 and %d IPv6 prefixes, %d bytes stored, over %d",
			ErrDatasetTooLarge, in.ASSet, len(in.IPv4), len(in.IPv6), size, zefs.IRRDatasetValueMax)
	}
	for _, at := range []time.Time{in.RefreshedAt, in.CheckedAt, in.StaleSince} {
		if !at.IsZero() && at.Unix() <= 0 {
			return Dataset{}, fmt.Errorf("%w: time %s is not after the Unix epoch", ErrDatasetMalformed, at)
		}
	}
	return Dataset{
		asSet: in.ASSet, server: server, ipv4: in.IPv4, ipv6: in.IPv6,
		refreshedAt: storedTime(in.RefreshedAt), checkedAt: storedTime(in.CheckedAt),
		staleSince: storedTime(in.StaleSince), encodedLen: size,
	}, nil
}

// ASSet returns the AS-SET the dataset holds.
func (d Dataset) ASSet() string { return d.asSet }

// Server returns the normalised server that answered.
func (d Dataset) Server() string { return d.server }

// IPv4 returns the IPv4 prefixes. The caller MUST NOT modify the slice.
func (d Dataset) IPv4() []netip.Prefix { return d.ipv4 }

// IPv6 returns the IPv6 prefixes. The caller MUST NOT modify the slice.
func (d Dataset) IPv6() []netip.Prefix { return d.ipv6 }

// RefreshedAt dates the oldest prefixes the dataset carries (CachedEntry).
func (d Dataset) RefreshedAt() time.Time { return d.refreshedAt }

// CheckedAt is the last time the server answered; the schedule runs from it.
func (d Dataset) CheckedAt() time.Time { return d.checkedAt }

// StaleSince is when a refresh first learned nothing; zero means current.
func (d Dataset) StaleSince() time.Time { return d.staleSince }

// EncodedLen returns the bytes WriteTo writes. It is fixed at construction.
func (d Dataset) EncodedLen() int { return d.encodedLen }

// WriteTo encodes the dataset into buf at off and returns the bytes written.
// buf[off:] MUST hold EncodedLen bytes; the caller sizes it once from
// EncodedLen, and the encoding allocates nothing.
func (d Dataset) WriteTo(buf []byte, off int) int {
	pos := off
	buf[pos] = datasetVersion
	pos++
	pos += writeDatasetTime(buf, pos, d.refreshedAt)
	pos += writeDatasetTime(buf, pos, d.checkedAt)
	pos += writeDatasetTime(buf, pos, d.staleSince)
	pos += writeDatasetName(buf, pos, d.asSet)
	pos += writeDatasetName(buf, pos, d.server)
	pos += writeDatasetFamily(buf, pos, d.ipv4)
	pos += writeDatasetFamily(buf, pos, d.ipv6)
	digest := sha256.Sum256(buf[off:pos])
	pos += copy(buf[pos:], digest[:])
	return pos - off
}

// DatasetDigest returns the digest an encoded dataset ends with: the value the
// pointer and the notice carry. value MUST be one WriteTo produced.
func DatasetDigest(value []byte) [sha256.Size]byte {
	var digest [sha256.Size]byte
	copy(digest[:], value[len(value)-datasetDigest:])
	return digest
}

// DecodeDataset reads the value stored under key. It refuses a value that is
// truncated, fails its digest, carries another version, is malformed, or names
// another AS-SET, server or digest than key, each with its named error; it
// never answers an empty dataset in place of a refused one.
func DecodeDataset(key string, value []byte) (Dataset, error) {
	if len(value) > zefs.IRRDatasetValueMax {
		return Dataset{}, fmt.Errorf("%w: %d bytes", ErrDatasetTooLarge, len(value))
	}
	if len(value) < datasetFixedLen {
		return Dataset{}, fmt.Errorf("%w: %d bytes, under the %d every dataset holds", ErrDatasetTruncated, len(value), datasetFixedLen)
	}
	if value[0] != datasetVersion {
		return Dataset{}, fmt.Errorf("%w: %d", ErrDatasetVersion, value[0])
	}
	reader := datasetReader{body: value[:len(value)-datasetDigest], pos: 1}
	var times [3]time.Time
	for i := range times {
		seconds, err := reader.uint64()
		if err != nil {
			return Dataset{}, err
		}
		times[i] = timeOrZero(int64(seconds))
	}
	asSet, err := reader.name()
	if err != nil {
		return Dataset{}, err
	}
	server, err := reader.name()
	if err != nil {
		return Dataset{}, err
	}
	ipv4, err := reader.family(net.IPv4len)
	if err != nil {
		return Dataset{}, err
	}
	ipv6, err := reader.family(net.IPv6len)
	if err != nil {
		return Dataset{}, err
	}
	if reader.pos != len(reader.body) {
		return Dataset{}, fmt.Errorf("%w: %d bytes after the IPv6 list", ErrDatasetMalformed, len(reader.body)-reader.pos)
	}
	digest := sha256.Sum256(reader.body)
	if !bytes.Equal(digest[:], value[len(reader.body):]) {
		return Dataset{}, ErrDatasetDigest
	}
	named, valid := datasetKeyOf(server, asSet, hex.EncodeToString(digest[:]))
	if !valid {
		return Dataset{}, fmt.Errorf("%w: value holds %q from %q, which names no key", ErrDatasetIdentity, asSet, server)
	}
	if named != key {
		return Dataset{}, fmt.Errorf("%w: value is %s, key is %s", ErrDatasetIdentity, named, key)
	}
	return Dataset{
		asSet: asSet, server: server, ipv4: ipv4, ipv6: ipv6,
		refreshedAt: times[0], checkedAt: times[1], staleSince: times[2], encodedLen: len(value),
	}, nil
}

// DatasetKey returns the state key of the value with digest (lower-case hex,
// as a Pointer carries it) of one AS-SET from one server, after the same
// validation NewDataset applies to the names.
func DatasetKey(server, asSet, digest string) (string, error) {
	if err := validateName(asSet); err != nil {
		return "", err
	}
	normalised, err := DatasetServer(server)
	if err != nil {
		return "", err
	}
	if !isHexDigest(digest) {
		return "", fmt.Errorf("%w: digest %q is not 64 lower-case hex digits", ErrDatasetMalformed, digest)
	}
	return zefs.KeyIRRDataset.Key(normalised, asSet, digest), nil
}

// DatasetKeyASSet returns the AS-SET segment of a dataset key, and false for a
// key that is not server, AS-SET and digest under DatasetPrefix: neither a
// server key form nor an AS-SET name holds a '/', so the three segments are
// unambiguous.
func DatasetKeyASSet(key string) (string, bool) {
	rest, found := strings.CutPrefix(key, DatasetPrefix)
	if !found {
		return "", false
	}
	segments := strings.Split(rest, "/")
	if len(segments) != 3 {
		return "", false
	}
	if slices.Contains(segments, "") {
		return "", false
	}
	return segments[1], true
}

// DatasetServer returns the key form of an IRR server, so that every spelling
// of one server names one dataset: the host is lowercased with no trailing
// dot, the whois port 43 is dropped, and any other port follows as ":port". An
// IPv6 literal is written in its canonical form, with "_port" for a port other
// than 43, because its own colons would make ":port" ambiguous. A server whose
// host holds a character a state key cannot carry is refused.
func DatasetServer(server string) (string, error) {
	host, portText := strings.TrimSpace(server), ""
	if splitHost, splitPort, err := net.SplitHostPort(host); err == nil {
		host, portText = splitHost, splitPort
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" {
		return "", fmt.Errorf("%w: %q has no host", ErrDatasetServer, server)
	}
	// port zero is a guard: it means the key carries no port, which is the
	// case for the whois port and for a server named without one.
	var port uint32
	if portText != "" {
		number, err := strconv.ParseUint(portText, 10, 16)
		if err != nil {
			return "", fmt.Errorf("%w: %q has an invalid port", ErrDatasetServer, server)
		}
		if number == 0 {
			return "", fmt.Errorf("%w: %q names port 0", ErrDatasetServer, server)
		}
		if number != whoisPort {
			port = uint32(number)
		}
	}
	var key textbuf.Buffer
	if addr, err := netip.ParseAddr(host); err == nil && addr.Is6() {
		key.Str(addr.String())
		if port != 0 {
			key.Byte('_').Uint32(port)
		}
		return key.String(), nil
	}
	for _, char := range host {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= '0' && char <= '9':
		case char == '.' || char == '-' || char == '_':
		default:
			return "", fmt.Errorf("%w: %q holds %q", ErrDatasetServer, server, string(char))
		}
	}
	if strings.Contains(host, "..") {
		return "", fmt.Errorf("%w: %q holds an empty label", ErrDatasetServer, server)
	}
	key.Str(host)
	if port != 0 {
		key.Byte(':').Uint32(port)
	}
	return key.String(), nil
}

// datasetKeyOf forms the key a decoded value names: its server, its AS-SET and
// its own digest. A name KeyEntry.Key would panic on (empty, or holding "..")
// names no key, so the second result is false and the identity check refuses
// the value: stored bytes never reach a panic. digest is always 64 hex digits.
func datasetKeyOf(server, asSet, digest string) (string, bool) {
	if server == "" {
		return "", false
	}
	if asSet == "" {
		return "", false
	}
	if strings.Contains(server, "..") {
		return "", false
	}
	if strings.Contains(asSet, "..") {
		return "", false
	}
	return zefs.KeyIRRDataset.Key(server, asSet, digest), true
}

// significantBytes is the address bytes a prefix of bits bits carries.
func significantBytes(bits int) int {
	return (bits + 7) / 8
}

// storedTime is t as the dataset keeps it: whole seconds, UTC.
func storedTime(at time.Time) time.Time {
	if at.IsZero() {
		return time.Time{}
	}
	return time.Unix(at.Unix(), 0).UTC()
}

// timeOrZero reads a stored time: zero seconds is the unset time.
func timeOrZero(seconds int64) time.Time {
	if seconds == 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0).UTC()
}

// writeDatasetTime writes an unset time as zero seconds, NewDataset having
// refused a set time at or before the Unix epoch.
func writeDatasetTime(buf []byte, off int, at time.Time) int {
	var seconds int64
	if !at.IsZero() {
		seconds = at.Unix()
	}
	binary.BigEndian.PutUint64(buf[off:], uint64(seconds))
	return 8
}

func writeDatasetName(buf []byte, off int, name string) int {
	binary.BigEndian.PutUint16(buf[off:], uint16(len(name)))
	return 2 + copy(buf[off+2:], name)
}

func writeDatasetFamily(buf []byte, off int, prefixes []netip.Prefix) int {
	binary.BigEndian.PutUint32(buf[off:], uint32(len(prefixes)))
	pos := off + 4
	for _, prefix := range prefixes {
		bits := prefix.Bits()
		buf[pos] = byte(bits)
		pos++
		masked := prefix.Masked().Addr()
		if masked.Is4() {
			address := masked.As4()
			pos += copy(buf[pos:], address[:significantBytes(bits)])
			continue
		}
		address := masked.As16()
		pos += copy(buf[pos:], address[:significantBytes(bits)])
	}
	return pos - off
}

// datasetReader walks a dataset body, digest excluded. Every read checks the
// bytes it needs against what is left, so a short value ends in
// ErrDatasetTruncated and never in an out-of-range read.
type datasetReader struct {
	body []byte
	pos  int
}

func (r *datasetReader) take(size int) ([]byte, error) {
	if size > len(r.body)-r.pos {
		return nil, fmt.Errorf("%w: %d bytes needed at offset %d, %d left", ErrDatasetTruncated, size, r.pos, len(r.body)-r.pos)
	}
	field := r.body[r.pos : r.pos+size]
	r.pos += size
	return field, nil
}

func (r *datasetReader) uint64() (uint64, error) {
	field, err := r.take(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(field), nil
}

func (r *datasetReader) name() (string, error) {
	field, err := r.take(2)
	if err != nil {
		return "", err
	}
	length := int(binary.BigEndian.Uint16(field))
	if length > datasetNameMax {
		return "", fmt.Errorf("%w: a name of %d bytes", ErrDatasetMalformed, length)
	}
	field, err = r.take(length)
	if err != nil {
		return "", err
	}
	return string(field), nil
}

// family reads one count and its prefixes. The count is bounded by
// DatasetFamilyMax and by the bytes left (each prefix takes at least one)
// before the list is allocated.
func (r *datasetReader) family(addressLen int) ([]netip.Prefix, error) {
	field, err := r.take(4)
	if err != nil {
		return nil, err
	}
	count := int(binary.BigEndian.Uint32(field))
	if count > DatasetFamilyMax {
		return nil, fmt.Errorf("%w: %d prefixes in one family, over %d", ErrDatasetMalformed, count, DatasetFamilyMax)
	}
	if count > len(r.body)-r.pos {
		return nil, fmt.Errorf("%w: %d prefixes announced, %d bytes left", ErrDatasetTruncated, count, len(r.body)-r.pos)
	}
	prefixes := make([]netip.Prefix, 0, count)
	for range count {
		prefix, err := r.prefix(addressLen)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

func (r *datasetReader) prefix(addressLen int) (netip.Prefix, error) {
	field, err := r.take(1)
	if err != nil {
		return netip.Prefix{}, err
	}
	bits := int(field[0])
	if bits > addressLen*8 {
		return netip.Prefix{}, fmt.Errorf("%w: prefix length %d in a %d-byte family", ErrDatasetMalformed, bits, addressLen)
	}
	field, err = r.take(significantBytes(bits))
	if err != nil {
		return netip.Prefix{}, err
	}
	var address [net.IPv6len]byte
	copy(address[:], field)
	addr := netip.AddrFrom16(address)
	if addressLen == net.IPv4len {
		addr = netip.AddrFrom4([4]byte(address[:net.IPv4len]))
	}
	prefix := netip.PrefixFrom(addr, bits)
	if prefix.Masked() != prefix {
		return netip.Prefix{}, fmt.Errorf("%w: %s carries host bits", ErrDatasetMalformed, prefix)
	}
	return prefix, nil
}
