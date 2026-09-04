// Design: docs/architecture/api/process-protocol.md — the certificate a Ze listener serves
// Related: acceptor.go — NewHubAcceptor serves the plugin listener's leaf through one of these
// Related: server/managed_serve.go — managedCertificate does the same for the managed listener
// Related: ipc/tls.go — StartListeners takes the Certificate method as tls.Config.GetCertificate

package plugin

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
)

// errServingLeafNoAuthority refuses a leaf holder with nothing to issue from.
// errServingLeafNoMaterial refuses an authority that answers a certificate
// carrying no DER: a tls.Certificate with an empty chain completes no handshake,
// and crypto/tls reports it one layer away from the authority that produced it.
var (
	errServingLeafNoAuthority = errors.New("plugin: a serving leaf needs a certificate authority")
	errServingLeafNoMaterial  = errors.New("plugin: the certificate authority issued an empty certificate")
)

// renewalFractionNumerator and renewalFractionDenominator set when a leaf is
// reissued: two thirds of the way through its life. The rule is a FRACTION of
// the lifetime rather than a fixed margin, so it holds whatever lifetime the
// authority chose, including the appliance lifetimes measured in years.
const (
	renewalFractionNumerator   = 2
	renewalFractionDenominator = 3
)

// ServingLeaf holds the certificate one of Ze's own listeners presents, and
// reissues it from the daemon's certificate authority before it expires.
//
// A leaf lives 24 hours and a router is not restarted daily, so issuing once at
// construction hands every peer an expired certificate on the second day. The
// operator's config is unchanged when that happens and nothing names the cause,
// which is why the reissue belongs here rather than in an operator procedure.
//
// The stdlib seam is tls.Config.GetCertificate: crypto/tls calls it once per
// handshake, so the expiry is answered at the moment it matters and never from
// a value cached across a boundary. This is the control plane, once per
// connection, so the check costs nothing that matters.
//
// Safe for concurrent use. Certificate serializes on one mutex, so concurrent
// handshakes issue ONE leaf between them rather than one each.
type ServingLeaf struct {
	ca         Authority
	commonName string
	hosts      []string
	clk        clock.Clock

	mu      sync.Mutex
	cert    *tls.Certificate
	renewAt time.Time // The instant Certificate stops answering cert and issues again.
}

// NewServingLeaf issues the first leaf and returns the holder that keeps it
// current. Issuance runs HERE rather than at the first handshake, so an
// authority that cannot sign fails the listener at start, where the operator
// is watching, instead of at the first connection.
//
// clk MAY be nil, which installs clock.RealClock{}. A test passes a fake clock
// to move the leaf's expiry rather than wait for it.
func NewServingLeaf(ca Authority, commonName string, hosts []string, clk clock.Clock) (*ServingLeaf, error) {
	if ca == nil {
		return nil, errServingLeafNoAuthority
	}
	if clk == nil {
		clk = clock.RealClock{}
	}

	leaf := &ServingLeaf{
		ca:         ca,
		commonName: commonName,
		hosts:      hosts,
		clk:        clk,
	}
	if _, err := leaf.Certificate(nil); err != nil {
		return nil, err
	}
	return leaf, nil
}

// Certificate answers the certificate for one handshake, reissuing first when
// the held leaf has spent two thirds of its life. It is
// tls.Config.GetCertificate, and the ClientHelloInfo is unread: every listener
// this serves presents one identity.
//
// Safe for concurrent use.
func (l *ServingLeaf) Certificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cert != nil && l.clk.Now().Before(l.renewAt) {
		return l.cert, nil
	}
	return l.issueLocked()
}

// issueLocked replaces the held leaf. The caller MUST hold l.mu.
func (l *ServingLeaf) issueLocked() (*tls.Certificate, error) {
	cert, err := l.ca.IssueLeaf(l.commonName, l.hosts)
	if err != nil {
		return nil, fmt.Errorf("issue the %s TLS certificate: %w", l.commonName, err)
	}
	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("%w: %s", errServingLeafNoMaterial, l.commonName)
	}

	// crypto/tls parses Leaf itself when it is nil, at every handshake. Parsing
	// it once here removes that work and gives the renewal deadline, which is
	// READ from the certificate rather than assumed: the authority owns the
	// lifetime, and this type owns only when to ask for another.
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse the issued %s certificate: %w", l.commonName, err)
	}
	cert.Leaf = parsed

	l.cert = &cert
	l.renewAt = renewalDeadline(parsed.NotBefore, parsed.NotAfter)
	return l.cert, nil
}

// renewalDeadline returns the instant a leaf is reissued at. A certificate
// whose window is empty or inverted is reissued at once: it is expired the
// moment it exists, so holding it serves nobody.
//
// The division truncates toward zero, which moves the deadline EARLIER, and
// earlier is the safe direction for a renewal. The multiplication bounds the
// lifetime at about 146 years, half what a time.Duration holds. Ze issues
// nothing beyond the 25 years an appliance certificate can ask for.
func renewalDeadline(notBefore, notAfter time.Time) time.Time {
	lifetime := notAfter.Sub(notBefore)
	if lifetime <= 0 {
		return notBefore
	}
	return notBefore.Add(lifetime * renewalFractionNumerator / renewalFractionDenominator)
}
