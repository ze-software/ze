// Design: docs/architecture/pki/pki-store.md -- a consumer's certificate or CA name is completed and checked at commit
// Related: reference.go -- certificateReference and caReference, the sources registered here

package pkiref

import "github.com/ze-software/ze/internal/component/config/yang"

func init() {
	// A consumer's certificate and CA name leaves declare
	// `ze:reference "pki-certificate"` and `ze:reference "pki-ca"`: the editor
	// completes them, and commit judges them, through these sources.
	yang.RegisterReferenceSource(CertificateReferenceSource, certificateReference)
	yang.RegisterReferenceSource(CAReferenceSource, caReference)
}
