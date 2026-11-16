// Design: docs/architecture/container/image-store.md -- the doctor codes the image store owns

package container

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestStoreDiagnosticCodesRegisterFromThePlugin proves each code of the image
// store's doctor table resolves in the code registry once the plugin is
// linked, with a title and an explanation, and that the six codes a fetch
// raises are declared as event codes.
//
// Method: the plugin's init() ran when this package was loaded, so the test
// looks each code up in the registries the doctor reads.
//
// MUTATION: drop storeDiagnosticCodes from registerContainerDiagnosticCodes.
// Every lookup then answers nil, and the event codes refuse to register.
func TestStoreDiagnosticCodesRegisterFromThePlugin(t *testing.T) {
	owned := []string{
		codeStoreUnwritable, codeStoreFull, codeKernelQuotaUnavailable, codeKernelQuotaUnknown,
		codeStorageFull, codeRegistryUnreachable, codeRegistryAuthRefused, codeImageNotFound,
		codeImageDigestMismatch, codeImageRefused, codeFetchNoSpace,
	}
	if len(storeDiagnosticCodes) != len(owned) {
		t.Fatalf("the store's code table holds %d codes, want the %d of its doctor table", len(storeDiagnosticCodes), len(owned))
	}
	for _, code := range owned {
		meta := diagnostic.Lookup(code)
		if meta == nil {
			t.Errorf("code %s is not in the code registry", code)
			continue
		}
		if meta.Title == "" {
			t.Errorf("code %s has no title", code)
		}
		if meta.Description == "" {
			t.Errorf("code %s has no explanation", code)
		}
	}
	events := diagnostic.EventCodes()
	for _, code := range []string{
		codeRegistryUnreachable, codeRegistryAuthRefused, codeImageNotFound,
		codeImageDigestMismatch, codeImageRefused, codeFetchNoSpace,
	} {
		if !slices.Contains(events, code) {
			t.Errorf("code %s is not declared as an event code", code)
		}
	}
}
