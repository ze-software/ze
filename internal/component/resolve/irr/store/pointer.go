// Design: docs/architecture/irr-owner.md -- the pointer, binding, interest and generation values
// Related: dataset.go -- the dataset value a pointer names
// Related: notice.go -- the notices sent after a pointer changes
//
// The irr plugin writes the pointer, the binding and the generation, and each
// consumer writes its own interest key. Both sides read them through these
// types, so the JSON spelling is declared once.

package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// InterestNamesMax bounds the names one interest key holds, AS-SETs and AS
// numbers together. A consumer enforcing more is refused at the owner, which
// reads the key as unreadable rather than fetching an unbounded list.
const InterestNamesMax = 65_536

// Named refusals of a stored pointer, binding or interest value. A caller
// reads each one as "nothing stored", never as an empty value.
var (
	ErrPointerMalformed  = errors.New("irr/store: stored AS-SET pointer is malformed")
	ErrBindingMalformed  = errors.New("irr/store: stored ASN binding is malformed")
	ErrInterestMalformed = errors.New("irr/store: stored interest is malformed")
	ErrInterestTooLarge  = errors.New("irr/store: stored interest names too many sets")
)

// ASSetName returns the one spelling of an AS-SET name every key uses: upper
// case, so a consumer that configured "AS-Foo" and an operator who typed
// "AS-FOO" name one dataset and one pointer rather than two copies fetched on
// two schedules. The owner and every consumer pass a configured or typed name
// through this before forming a key. A name the store cannot key is refused.
func ASSetName(name string) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(name))
	if err := validateName(upper); err != nil {
		return "", err
	}
	return upper, nil
}

// Pointer is the value of meta/irr-set/{as-set}: which dataset is enforced for
// the set now. Server is the key form DatasetServer gives, Generation the
// value meta/irr-generation took for this write, Digest the hex SHA-256 the
// dataset ends with. The owner writes it only after the dataset it names.
type Pointer struct {
	Server     string `json:"server"`
	Generation uint32 `json:"generation"`
	Digest     string `json:"digest"`
}

// PointerKey returns the state key of the pointer of asSet, which MUST be an
// ASSetName answer.
func PointerKey(asSet string) string {
	return zefs.KeyIRRSet.Key(asSet)
}

// DatasetKey returns the key of the dataset the pointer names for asSet. The
// pointer already holds the server in its key form, so the key is formed
// without normalising it again.
func (p Pointer) DatasetKey(asSet string) string {
	return zefs.KeyIRRDataset.Key(p.Server, asSet)
}

// EncodePointer returns the stored form of p.
func EncodePointer(p Pointer) ([]byte, error) {
	return json.Marshal(p)
}

// DecodePointer reads a stored pointer. A value with no server, a zero
// generation or a digest that is not 64 hex digits is refused.
func DecodePointer(value []byte) (Pointer, error) {
	var p Pointer
	if err := json.Unmarshal(value, &p); err != nil {
		return Pointer{}, fmt.Errorf("%w: %w", ErrPointerMalformed, err)
	}
	if p.Server == "" {
		return Pointer{}, fmt.Errorf("%w: no server", ErrPointerMalformed)
	}
	if p.Generation == 0 {
		return Pointer{}, fmt.Errorf("%w: generation 0", ErrPointerMalformed)
	}
	if !isHexDigest(p.Digest) {
		return Pointer{}, fmt.Errorf("%w: digest %q", ErrPointerMalformed, p.Digest)
	}
	return p, nil
}

// isHexDigest reports whether digest is a lower-case hex SHA-256.
func isHexDigest(digest string) bool {
	if len(digest) != 2*datasetDigest {
		return false
	}
	for i := range len(digest) {
		c := digest[i]
		if c >= '0' && c <= '9' {
			continue
		}
		if c >= 'a' && c <= 'f' {
			continue
		}
		return false
	}
	return true
}

// Binding is the value of meta/irr-asn/{asn}: the AS-SET the owner queries for
// a bare AS number. PeeringDB named it, or PeeringDB held none, had no client,
// or failed with nothing bound, and the set is the literal AS<asn>.
// PeeringDBURL is the endpoint that answered; a binding whose URL is not
// system peeringdb url reads as absent, so a changed endpoint is asked again.
type Binding struct {
	ASSet        string    `json:"as-set"`
	PeeringDBURL string    `json:"peeringdb-url"`
	DiscoveredAt time.Time `json:"discovered-at"`
}

// BindingKey returns the state key of the binding of asn.
func BindingKey(asn uint32) string {
	return zefs.KeyIRRASN.Key(textbuf.StringUint32(asn))
}

// EncodeBinding returns the stored form of b, its time in UTC.
func EncodeBinding(b Binding) ([]byte, error) {
	b.DiscoveredAt = b.DiscoveredAt.UTC().Truncate(time.Second)
	return json.Marshal(b)
}

// DecodeBinding reads a stored binding. A binding naming a set the store
// cannot key is refused; its AS-SET comes back in ASSetName form.
func DecodeBinding(value []byte) (Binding, error) {
	var b Binding
	if err := json.Unmarshal(value, &b); err != nil {
		return Binding{}, fmt.Errorf("%w: %w", ErrBindingMalformed, err)
	}
	set, err := ASSetName(b.ASSet)
	if err != nil {
		return Binding{}, fmt.Errorf("%w: %w", ErrBindingMalformed, err)
	}
	b.ASSet = set
	return b, nil
}

// Interest is the value of meta/irr-interest/{consumer}: the AS-SETs and bare
// AS numbers one consumer enforces. The owner fetches the union of every
// consumer's interest.
type Interest struct {
	ASSets []string `json:"as-sets"`
	ASNs   []uint32 `json:"asns"`
}

// InterestKey returns the state key of consumer's interest.
func InterestKey(consumer string) string {
	return zefs.KeyIRRInterest.Key(consumer)
}

// InterestPrefix is the listing prefix of every consumer's interest key.
const InterestPrefix = "meta/irr-interest/"

// PointerPrefix and BindingPrefix are the listing prefixes of every pointer and
// every binding.
const (
	PointerPrefix = "meta/irr-set/"
	BindingPrefix = "meta/irr-asn/"
)

// GenerationKey is the state key of the generation counter.
func GenerationKey() string {
	return zefs.KeyIRRGeneration.Key()
}

// EncodeInterest returns the stored form of in, its AS-SETs in ASSetName form.
// More than InterestNamesMax names, or a set the store cannot key, is refused.
func EncodeInterest(in Interest) ([]byte, error) {
	normalised, err := normaliseInterest(in)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalised)
}

// DecodeInterest reads a stored interest, refusing one over InterestNamesMax
// or naming a set the store cannot key. Its AS-SETs come back in ASSetName
// form.
func DecodeInterest(value []byte) (Interest, error) {
	var in Interest
	if err := json.Unmarshal(value, &in); err != nil {
		return Interest{}, fmt.Errorf("%w: %w", ErrInterestMalformed, err)
	}
	return normaliseInterest(in)
}

// normaliseInterest bounds in and writes its sets in ASSetName form.
func normaliseInterest(in Interest) (Interest, error) {
	if len(in.ASSets)+len(in.ASNs) > InterestNamesMax {
		return Interest{}, fmt.Errorf("%w: %d AS-SETs and %d AS numbers, over %d",
			ErrInterestTooLarge, len(in.ASSets), len(in.ASNs), InterestNamesMax)
	}
	out := Interest{ASSets: make([]string, 0, len(in.ASSets)), ASNs: append([]uint32(nil), in.ASNs...)}
	for _, name := range in.ASSets {
		set, err := ASSetName(name)
		if err != nil {
			return Interest{}, fmt.Errorf("%w: %w", ErrInterestMalformed, err)
		}
		out.ASSets = append(out.ASSets, set)
	}
	return out, nil
}
