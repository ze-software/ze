// Design: docs/architecture/pki/pki-store.md -- a consumer's certificate or CA name is completed and checked at commit
// Related: register.go -- registers certificateReference and caReference

// Package pkiref declares the two ze:reference sources a PKI name leaf names.
// It imports only the YANG layer, so a package the pki component itself
// depends on (config among them) can link the sources in its tests.
package pkiref

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// CertificateReferenceSource and CAReferenceSource name the ze:reference
// sources a consumer's PKI name leaf declares: `ze:reference
// "pki-certificate"` for a device certificate, `ze:reference "pki-ca"` for a
// trust anchor. The editor completes the entries the candidate's pki section
// defines, and commit refuses a name it does not define, naming the ones it
// does.
//
// The consumers (web, looking glass, managed clients, IPsec, RPKI, geodns,
// AS112) each live in their own module, several of which a build can leave
// out, and a leafref from each would make every one of them import the pki
// module. One source here declares each reference once, and the pki
// component, which is in every build, links this package.
const (
	CertificateReferenceSource = "pki-certificate"
	CAReferenceSource          = "pki-ca"
)

// The pki section and its two named lists (ze-pki-conf.yang).
const (
	referenceRoot            = "pki"
	referenceCertificateList = "certificate"
	referenceCAList          = "ca"
)

// certificateReference answers the names of the certificate entries the
// candidate's pki section defines.
func certificateReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	return yang.ReferenceAnswer{Names: pkiEntryNames(root, referenceCertificateList)}
}

// caReference answers the names of the ca entries the candidate's pki
// section defines.
func caReference(root map[string]any, _ []string) yang.ReferenceAnswer {
	return yang.ReferenceAnswer{Names: pkiEntryNames(root, referenceCAList)}
}

// pkiEntryNames answers the sorted keys of one pki list. root holds one entry
// per top-level section, as Tree.ToPluginMap lowers it, so a list is a map
// keyed by entry name.
func pkiEntryNames(root map[string]any, list string) []string {
	section, _ := root[referenceRoot].(map[string]any) // no pki section: no entry is defined
	entries, _ := section[list].(map[string]any)       // no entry of this list is defined
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
